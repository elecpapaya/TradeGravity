package subscriptions

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	_ "modernc.org/sqlite"
)

const (
	tokenVersion       = 1
	minimumSecretBytes = 32
)

var consentHeader = []string{"email", "audience", "status", "consented_at", "consent_method", "consent_source", "privacy_notice_version"}
var deliveryHeader = []string{"email", "audience", "status", "consented_at", "consent_method", "consent_source", "privacy_notice_version", "unsubscribe_url"}
var suppressionHeader = []string{"email", "reason", "suppressed_at"}

var ErrInvalidToken = errors.New("invalid unsubscribe token")

type Registry struct {
	db              *sql.DB
	secret          []byte
	unsubscribeBase *url.URL
}

type ImportResult struct {
	Inserted          int
	Updated           int
	SuppressedSkipped int
}

type UnsubscribeResult struct {
	Changed        bool
	AlreadyStopped bool
}

type FeedbackResult struct {
	Duplicate            bool
	SubscriptionsStopped int64
}

type consentRecord struct {
	Email                string
	Audience             string
	ConsentedAt          time.Time
	ConsentSource        string
	PrivacyNoticeVersion string
}

type storedSubscription struct {
	ID                   string
	Email                string
	Audience             string
	Status               string
	ConsentedAt          string
	ConsentSource        string
	PrivacyNoticeVersion string
}

type tokenPayload struct {
	Version int    `json:"v"`
	ID      string `json:"id"`
}

func Open(databasePath string, secret []byte, publicBaseURL string) (*Registry, error) {
	databasePath = strings.TrimSpace(databasePath)
	if databasePath == "" {
		return nil, errors.New("subscription database path is required")
	}
	if len(secret) < minimumSecretBytes {
		return nil, fmt.Errorf("unsubscribe secret must contain at least %d bytes", minimumSecretBytes)
	}
	base, err := normalizePublicBaseURL(publicBaseURL)
	if err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, fmt.Errorf("resolve subscription database: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return nil, fmt.Errorf("create subscription database directory: %w", err)
	}
	db, err := sql.Open("sqlite", absolute)
	if err != nil {
		return nil, fmt.Errorf("open subscription database: %w", err)
	}
	db.SetMaxOpenConns(1)
	registry := &Registry{db: db, secret: append([]byte(nil), secret...), unsubscribeBase: base}
	if err := registry.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(absolute, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("restrict subscription database permissions: %w", err)
	}
	return registry, nil
}

func (registry *Registry) Close() error {
	if registry == nil || registry.db == nil {
		return nil
	}
	for index := range registry.secret {
		registry.secret[index] = 0
	}
	return registry.db.Close()
}

func (registry *Registry) ImportConsents(ctx context.Context, raw []byte, importedAt time.Time) (ImportResult, error) {
	if importedAt.IsZero() {
		return ImportResult{}, errors.New("consent import time is required")
	}
	records, err := parseConsentCSV(raw, importedAt)
	if err != nil {
		return ImportResult{}, err
	}
	tx, err := registry.db.BeginTx(ctx, nil)
	if err != nil {
		return ImportResult{}, fmt.Errorf("begin consent import: %w", err)
	}
	defer tx.Rollback()
	result := ImportResult{}
	for _, record := range records {
		var globallySuppressed int
		err := tx.QueryRowContext(ctx, `
			SELECT 1 FROM address_suppressions WHERE email_normalized = ?
		`, record.Email).Scan(&globallySuppressed)
		if err == nil {
			result.SuppressedSkipped++
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return ImportResult{}, fmt.Errorf("check address suppression: %w", err)
		}
		var existingID, existingStatus, existingConsentedAt string
		err = tx.QueryRowContext(ctx, `
			SELECT id, status, consented_at
			FROM subscriptions
			WHERE email_normalized = ? AND audience = ?
		`, record.Email, record.Audience).Scan(&existingID, &existingStatus, &existingConsentedAt)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			id, idErr := randomID()
			if idErr != nil {
				return ImportResult{}, idErr
			}
			_, err = tx.ExecContext(ctx, `
				INSERT INTO subscriptions (
					id, email, email_normalized, audience, status, consented_at,
					consent_method, consent_source, privacy_notice_version, created_at
				) VALUES (?, ?, ?, ?, 'active', ?, 'double_opt_in', ?, ?, ?)
			`, id, record.Email, record.Email, record.Audience, record.ConsentedAt.UTC().Format(time.RFC3339), record.ConsentSource, record.PrivacyNoticeVersion, importedAt.UTC().Format(time.RFC3339))
			if err != nil {
				return ImportResult{}, fmt.Errorf("insert consent record: %w", err)
			}
			result.Inserted++
		case err != nil:
			return ImportResult{}, fmt.Errorf("find consent record: %w", err)
		case existingStatus == "suppressed":
			result.SuppressedSkipped++
		case existingStatus == "active":
			existingTime, parseErr := time.Parse(time.RFC3339, existingConsentedAt)
			if parseErr != nil {
				return ImportResult{}, errors.New("stored consent timestamp is invalid")
			}
			if record.ConsentedAt.Before(existingTime) {
				return ImportResult{}, fmt.Errorf("consent import for %s is older than the stored record", record.Audience)
			}
			_, err = tx.ExecContext(ctx, `
				UPDATE subscriptions
				SET email = ?, consented_at = ?, consent_source = ?, privacy_notice_version = ?, updated_at = ?
				WHERE id = ?
			`, record.Email, record.ConsentedAt.UTC().Format(time.RFC3339), record.ConsentSource, record.PrivacyNoticeVersion, importedAt.UTC().Format(time.RFC3339), existingID)
			if err != nil {
				return ImportResult{}, fmt.Errorf("update consent record: %w", err)
			}
			result.Updated++
		default:
			return ImportResult{}, fmt.Errorf("stored subscription has unsupported status %q", existingStatus)
		}
	}
	if err := tx.Commit(); err != nil {
		return ImportResult{}, fmt.Errorf("commit consent import: %w", err)
	}
	return result, nil
}

// SuppressAddress records a verified provider feedback event and suppresses
// every active audience membership for that address. The separate address
// suppression also prevents a later consent import from silently reactivating
// an address that hard-bounced, complained, or was provider-suppressed.
func (registry *Registry) SuppressAddress(ctx context.Context, email, reason, eventID, eventType string, occurredAt, processedAt time.Time) (FeedbackResult, error) {
	address, err := canonicalEmail(email)
	if err != nil {
		return FeedbackResult{}, err
	}
	if reason != "bounced" && reason != "complaint" && reason != "invalid" {
		return FeedbackResult{}, errors.New("provider feedback reason is unsupported")
	}
	eventID = strings.TrimSpace(eventID)
	eventType = strings.TrimSpace(eventType)
	if eventID == "" || len(eventID) > 256 || strings.ContainsAny(eventID, "\r\n") || eventType == "" || len(eventType) > 120 || strings.ContainsAny(eventType, "\r\n") {
		return FeedbackResult{}, errors.New("provider feedback identity is invalid")
	}
	if occurredAt.IsZero() || processedAt.IsZero() || occurredAt.After(processedAt.Add(5*time.Minute)) {
		return FeedbackResult{}, errors.New("provider feedback timestamps are invalid")
	}
	tx, err := registry.db.BeginTx(ctx, nil)
	if err != nil {
		return FeedbackResult{}, fmt.Errorf("begin provider feedback: %w", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO provider_events (event_id, event_type, occurred_at, processed_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(event_id) DO NOTHING
	`, eventID, eventType, occurredAt.UTC().Format(time.RFC3339), processedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return FeedbackResult{}, fmt.Errorf("record provider feedback event: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return FeedbackResult{}, fmt.Errorf("inspect provider feedback insert: %w", err)
	}
	if inserted == 0 {
		if err := tx.Commit(); err != nil {
			return FeedbackResult{}, err
		}
		return FeedbackResult{Duplicate: true}, nil
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO address_suppressions (email_normalized, reason, suppressed_at, event_id)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(email_normalized) DO NOTHING
	`, address, reason, occurredAt.UTC().Format(time.RFC3339), eventID)
	if err != nil {
		return FeedbackResult{}, fmt.Errorf("record address suppression: %w", err)
	}
	updated, err := tx.ExecContext(ctx, `
		UPDATE subscriptions
		SET status = 'suppressed', suppression_reason = ?, suppressed_at = ?, updated_at = ?
		WHERE email_normalized = ? AND status = 'active'
	`, reason, occurredAt.UTC().Format(time.RFC3339), processedAt.UTC().Format(time.RFC3339), address)
	if err != nil {
		return FeedbackResult{}, fmt.Errorf("apply provider feedback suppression: %w", err)
	}
	stopped, err := updated.RowsAffected()
	if err != nil {
		return FeedbackResult{}, fmt.Errorf("count provider feedback suppressions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return FeedbackResult{}, fmt.Errorf("commit provider feedback: %w", err)
	}
	return FeedbackResult{SubscriptionsStopped: stopped}, nil
}

func (registry *Registry) ExportAudience(ctx context.Context, audience string) ([]byte, []byte, error) {
	if err := validateAudience(audience); err != nil {
		return nil, nil, err
	}
	rows, err := registry.db.QueryContext(ctx, `
		SELECT id, email, audience, status, consented_at, consent_source, privacy_notice_version,
		       COALESCE(suppression_reason, ''), COALESCE(suppressed_at, '')
		FROM subscriptions
		WHERE audience = ?
		ORDER BY email_normalized
	`, audience)
	if err != nil {
		return nil, nil, fmt.Errorf("list audience subscriptions: %w", err)
	}
	defer rows.Close()
	var active [][]string
	suppressedByAddress := map[string][]string{}
	for rows.Next() {
		var record storedSubscription
		var suppressionReason, suppressedAt string
		if err := rows.Scan(&record.ID, &record.Email, &record.Audience, &record.Status, &record.ConsentedAt, &record.ConsentSource, &record.PrivacyNoticeVersion, &suppressionReason, &suppressedAt); err != nil {
			return nil, nil, err
		}
		switch record.Status {
		case "active":
			token, err := registry.tokenFor(record.ID)
			if err != nil {
				return nil, nil, err
			}
			unsubscribeURL := *registry.unsubscribeBase
			query := unsubscribeURL.Query()
			query.Set("token", token)
			unsubscribeURL.RawQuery = query.Encode()
			active = append(active, []string{record.Email, record.Audience, "active", record.ConsentedAt, "double_opt_in", record.ConsentSource, record.PrivacyNoticeVersion, unsubscribeURL.String()})
		case "suppressed":
			suppressedByAddress[record.Email] = []string{record.Email, suppressionReason, suppressedAt}
		default:
			return nil, nil, fmt.Errorf("stored subscription has unsupported status %q", record.Status)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	globalRows, err := registry.db.QueryContext(ctx, `
		SELECT email_normalized, reason, suppressed_at
		FROM address_suppressions
		ORDER BY email_normalized
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("list global address suppressions: %w", err)
	}
	defer globalRows.Close()
	globallySuppressed := map[string]bool{}
	for globalRows.Next() {
		var email, reason, suppressedAt string
		if err := globalRows.Scan(&email, &reason, &suppressedAt); err != nil {
			return nil, nil, err
		}
		globallySuppressed[email] = true
		suppressedByAddress[email] = []string{email, reason, suppressedAt}
	}
	if err := globalRows.Err(); err != nil {
		return nil, nil, err
	}
	if len(globallySuppressed) > 0 {
		filtered := active[:0]
		for _, row := range active {
			if !globallySuppressed[row[0]] {
				filtered = append(filtered, row)
			}
		}
		active = filtered
	}
	suppressedEmails := make([]string, 0, len(suppressedByAddress))
	for email := range suppressedByAddress {
		suppressedEmails = append(suppressedEmails, email)
	}
	sort.Strings(suppressedEmails)
	suppressed := make([][]string, 0, len(suppressedEmails))
	for _, email := range suppressedEmails {
		suppressed = append(suppressed, suppressedByAddress[email])
	}
	subscriberCSV, err := encodeCSV(deliveryHeader, active)
	if err != nil {
		return nil, nil, err
	}
	suppressionCSV, err := encodeCSV(suppressionHeader, suppressed)
	if err != nil {
		return nil, nil, err
	}
	return subscriberCSV, suppressionCSV, nil
}

func WritePrivateExports(subscriberPath string, subscriberCSV []byte, suppressionPath string, suppressionCSV []byte) error {
	subscriberTarget, err := preparePrivateTarget(subscriberPath)
	if err != nil {
		return fmt.Errorf("subscriber export: %w", err)
	}
	suppressionTarget, err := preparePrivateTarget(suppressionPath)
	if err != nil {
		return fmt.Errorf("suppression export: %w", err)
	}
	if strings.EqualFold(subscriberTarget, suppressionTarget) {
		return errors.New("subscriber and suppression exports must use different paths")
	}
	if err := writePrivateFile(subscriberTarget, subscriberCSV); err != nil {
		return fmt.Errorf("write subscriber export: %w", err)
	}
	if err := writePrivateFile(suppressionTarget, suppressionCSV); err != nil {
		_ = os.Remove(subscriberTarget)
		return fmt.Errorf("write suppression export: %w", err)
	}
	return nil
}

func (registry *Registry) Unsubscribe(ctx context.Context, token string, stoppedAt time.Time) (UnsubscribeResult, error) {
	if stoppedAt.IsZero() {
		return UnsubscribeResult{}, errors.New("unsubscribe time is required")
	}
	payload, err := registry.verifyToken(token)
	if err != nil {
		return UnsubscribeResult{}, err
	}
	tx, err := registry.db.BeginTx(ctx, nil)
	if err != nil {
		return UnsubscribeResult{}, err
	}
	defer tx.Rollback()
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM subscriptions WHERE id = ?`, payload.ID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return UnsubscribeResult{}, ErrInvalidToken
	}
	if err != nil {
		return UnsubscribeResult{}, fmt.Errorf("lookup unsubscribe token: %w", err)
	}
	if status == "suppressed" {
		if err := tx.Commit(); err != nil {
			return UnsubscribeResult{}, err
		}
		return UnsubscribeResult{AlreadyStopped: true}, nil
	}
	if status != "active" {
		return UnsubscribeResult{}, ErrInvalidToken
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE subscriptions
		SET status = 'suppressed', suppression_reason = 'unsubscribed', suppressed_at = ?, updated_at = ?
		WHERE id = ? AND status = 'active'
	`, stoppedAt.UTC().Format(time.RFC3339), stoppedAt.UTC().Format(time.RFC3339), payload.ID)
	if err != nil {
		return UnsubscribeResult{}, fmt.Errorf("record unsubscribe: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return UnsubscribeResult{}, err
	}
	return UnsubscribeResult{Changed: true}, nil
}

func (registry *Registry) ValidateToken(token string) error {
	payload, err := registry.verifyToken(token)
	if err != nil {
		return err
	}
	var exists int
	err = registry.db.QueryRow(`SELECT 1 FROM subscriptions WHERE id = ?`, payload.ID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrInvalidToken
	}
	return err
}

func (registry *Registry) tokenFor(id string) (string, error) {
	return registry.purposeTokenFor(id, "tradegravity-unsubscribe-v1:")
}

func (registry *Registry) purposeTokenFor(id, domain string) (string, error) {
	payloadRaw, err := json.Marshal(tokenPayload{Version: tokenVersion, ID: id})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadRaw)
	mac := hmac.New(sha256.New, registry.secret)
	_, _ = mac.Write([]byte(domain + payload))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + signature, nil
}

func (registry *Registry) verifyToken(token string) (tokenPayload, error) {
	return registry.verifyPurposeToken(token, "tradegravity-unsubscribe-v1:")
}

func (registry *Registry) verifyPurposeToken(token, domain string) (tokenPayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || len(token) > 1024 {
		return tokenPayload{}, ErrInvalidToken
	}
	mac := hmac.New(sha256.New, registry.secret)
	_, _ = mac.Write([]byte(domain + parts[0]))
	provided, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || base64.RawURLEncoding.EncodeToString(provided) != parts[1] || !hmac.Equal(provided, mac.Sum(nil)) {
		return tokenPayload{}, ErrInvalidToken
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || base64.RawURLEncoding.EncodeToString(payloadRaw) != parts[0] {
		return tokenPayload{}, ErrInvalidToken
	}
	var payload tokenPayload
	decoder := json.NewDecoder(bytes.NewReader(payloadRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return tokenPayload{}, ErrInvalidToken
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return tokenPayload{}, ErrInvalidToken
	}
	if payload.Version != tokenVersion || len(payload.ID) != 32 {
		return tokenPayload{}, ErrInvalidToken
	}
	if _, err := hex.DecodeString(payload.ID); err != nil {
		return tokenPayload{}, ErrInvalidToken
	}
	return payload, nil
}

func (registry *Registry) migrate() error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
		`CREATE TABLE IF NOT EXISTS subscriptions (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			email_normalized TEXT NOT NULL,
			audience TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('active', 'suppressed')),
			consented_at TEXT NOT NULL,
			consent_method TEXT NOT NULL CHECK (consent_method = 'double_opt_in'),
			consent_source TEXT NOT NULL,
			privacy_notice_version TEXT NOT NULL,
			suppression_reason TEXT,
			suppressed_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT,
			UNIQUE(email_normalized, audience),
			CHECK ((status = 'active' AND suppression_reason IS NULL AND suppressed_at IS NULL)
			    OR (status = 'suppressed' AND suppression_reason IS NOT NULL AND suppressed_at IS NOT NULL))
		);`,
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_audience_status ON subscriptions(audience, status, email_normalized);`,
		`CREATE TABLE IF NOT EXISTS provider_events (
			event_id TEXT PRIMARY KEY,
			event_type TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			processed_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS address_suppressions (
			email_normalized TEXT PRIMARY KEY,
			reason TEXT NOT NULL CHECK (reason IN ('bounced', 'complaint', 'invalid')),
			suppressed_at TEXT NOT NULL,
			event_id TEXT NOT NULL UNIQUE REFERENCES provider_events(event_id)
		);`,
		`CREATE TABLE IF NOT EXISTS pending_subscriptions (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL,
			email_normalized TEXT NOT NULL,
			audience TEXT NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('dispatch_pending', 'sent', 'confirmed')),
			consent_source TEXT NOT NULL,
			privacy_notice_version TEXT NOT NULL,
			requested_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			last_dispatch_at TEXT,
			provider_message_id TEXT,
			confirmed_at TEXT,
			UNIQUE(email_normalized, audience),
			CHECK ((status = 'dispatch_pending' AND provider_message_id IS NULL AND confirmed_at IS NULL)
			    OR (status = 'sent' AND provider_message_id IS NOT NULL AND confirmed_at IS NULL)
			    OR (status = 'confirmed' AND confirmed_at IS NOT NULL))
		);`,
		`CREATE INDEX IF NOT EXISTS idx_pending_subscriptions_expiry ON pending_subscriptions(status, expires_at);`,
	}
	for _, statement := range statements {
		if _, err := registry.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate subscription database: %w", err)
		}
	}
	return nil
}

func parseConsentCSV(raw []byte, importedAt time.Time) ([]consentRecord, error) {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = len(consentHeader)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse consent CSV: %w", err)
	}
	if len(rows) < 2 || !equalStrings(rows[0], consentHeader) {
		return nil, fmt.Errorf("consent CSV header must be exactly %s and contain at least one row", strings.Join(consentHeader, ","))
	}
	seen := map[string]bool{}
	records := make([]consentRecord, 0, len(rows)-1)
	for index, row := range rows[1:] {
		line := index + 2
		for _, value := range row {
			if value != strings.TrimSpace(value) {
				return nil, fmt.Errorf("consent CSV line %d contains leading or trailing whitespace", line)
			}
		}
		email, err := canonicalEmail(row[0])
		if err != nil {
			return nil, fmt.Errorf("consent CSV line %d: %w", line, err)
		}
		if err := validateAudience(row[1]); err != nil {
			return nil, fmt.Errorf("consent CSV line %d: %w", line, err)
		}
		key := email + "\x00" + row[1]
		if seen[key] {
			return nil, fmt.Errorf("consent CSV line %d duplicates an email and audience", line)
		}
		seen[key] = true
		if row[2] != "active" || row[4] != "double_opt_in" {
			return nil, fmt.Errorf("consent CSV line %d is not active double opt-in consent", line)
		}
		consentedAt, err := time.Parse(time.RFC3339, row[3])
		if err != nil || consentedAt.After(importedAt) {
			return nil, fmt.Errorf("consent CSV line %d has invalid or future consent time", line)
		}
		if err := validateLabel(row[5]); err != nil {
			return nil, fmt.Errorf("consent CSV line %d has invalid consent source", line)
		}
		if err := validateLabel(row[6]); err != nil {
			return nil, fmt.Errorf("consent CSV line %d has invalid privacy notice version", line)
		}
		records = append(records, consentRecord{Email: email, Audience: row[1], ConsentedAt: consentedAt, ConsentSource: row[5], PrivacyNoticeVersion: row[6]})
	}
	sort.Slice(records, func(first, second int) bool {
		if records[first].Audience == records[second].Audience {
			return records[first].Email < records[second].Email
		}
		return records[first].Audience < records[second].Audience
	})
	return records, nil
}

func encodeCSV(header []string, rows [][]string) ([]byte, error) {
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	writer.WriteAll(rows)
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func normalizePublicBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("public subscription base URL must be absolute HTTPS without credentials, query, or fragment")
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed.ResolveReference(&url.URL{Path: "unsubscribe"}), nil
}

func preparePrivateTarget(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("output path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	parent := filepath.Dir(absolute)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedParent, filepath.Base(absolute)), nil
}

func writePrivateFile(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("output already exists")
		}
		return err
	}
	writeErr := func() error {
		if _, err := file.Write(content); err != nil {
			return err
		}
		return file.Sync()
	}()
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}

func canonicalEmail(value string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || strings.Count(value, "@") != 1 {
		return "", errors.New("email address is empty or malformed")
	}
	for _, char := range value {
		if char > unicode.MaxASCII || unicode.IsSpace(char) || unicode.IsControl(char) {
			return "", errors.New("email address must use a plain ASCII addr-spec")
		}
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", errors.New("email address must not contain a display name or invalid syntax")
	}
	parts := strings.Split(value, "@")
	if parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") {
		return "", errors.New("email address must contain a valid-looking domain")
	}
	return strings.ToLower(value), nil
}

func validateAudience(value string) error {
	if value == "" || value != strings.TrimSpace(value) || len([]rune(value)) > 120 || strings.ContainsAny(value, "@/\\") || strings.Contains(value, "://") {
		return errors.New("audience must be a non-sensitive label")
	}
	return validateLabel(value)
}

func validateLabel(value string) error {
	if value == "" || len([]rune(value)) > 120 {
		return errors.New("label is empty or too long")
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return errors.New("label contains a control character")
		}
	}
	return nil
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate subscription ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func equalStrings(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}
