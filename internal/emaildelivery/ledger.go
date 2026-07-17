package emaildelivery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const minimumLedgerSecretBytes = 32

var (
	ErrDeliveryPending          = errors.New("delivery has an unresolved pending attempt")
	ErrNewAuthorizationRequired = errors.New("a reconciled non-accepted delivery requires a new launch authorization")
)

type Ledger struct {
	db     *sql.DB
	secret []byte
}

type DeliveryAttempt struct {
	EditionID           string
	Audience            string
	Email               string
	Provider            string
	ContentSHA256       string
	AuthorizationSHA256 string
	AttemptedAt         time.Time
}

type PrepareResult struct {
	DeliveryKey     string
	IdempotencyKey  string
	AlreadyAccepted bool
}

type ReconciliationRequest struct {
	EditionID         string
	Audience          string
	Email             string
	Outcome           string
	ProviderMessageID string
	ResolvedBy        string
	Evidence          string
	ResolvedAt        time.Time
}

type ReconciliationResult struct {
	Changed         bool
	AlreadyResolved bool
}

func OpenLedger(databasePath string, secret []byte) (*Ledger, error) {
	databasePath = strings.TrimSpace(databasePath)
	if databasePath == "" {
		return nil, errors.New("delivery ledger path is required")
	}
	if len(secret) < minimumLedgerSecretBytes {
		return nil, fmt.Errorf("delivery ledger secret must contain at least %d bytes", minimumLedgerSecretBytes)
	}
	absolute, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, fmt.Errorf("resolve delivery ledger: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o700); err != nil {
		return nil, fmt.Errorf("create delivery ledger directory: %w", err)
	}
	db, err := sql.Open("sqlite", absolute)
	if err != nil {
		return nil, fmt.Errorf("open delivery ledger: %w", err)
	}
	db.SetMaxOpenConns(1)
	ledger := &Ledger{db: db, secret: append([]byte(nil), secret...)}
	if err := ledger.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := os.Chmod(absolute, 0o600); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("restrict delivery ledger permissions: %w", err)
	}
	return ledger, nil
}

func (ledger *Ledger) Close() error {
	if ledger == nil || ledger.db == nil {
		return nil
	}
	for index := range ledger.secret {
		ledger.secret[index] = 0
	}
	return ledger.db.Close()
}

func (ledger *Ledger) Prepare(ctx context.Context, attempt DeliveryAttempt) (PrepareResult, error) {
	if attempt.AttemptedAt.IsZero() || attempt.EditionID == "" || attempt.Audience == "" || attempt.Provider == "" || attempt.ContentSHA256 == "" || !validSHA256(attempt.AuthorizationSHA256) {
		return PrepareResult{}, errors.New("delivery attempt is missing required fields")
	}
	address, err := canonicalRecipient(attempt.Email)
	if err != nil {
		return PrepareResult{}, err
	}
	deliveryKey := ledger.key("delivery", attempt.EditionID, attempt.Audience, address)
	recipientKey := ledger.key("recipient", address)
	idempotencyKey := "tradegravity/" + sanitizeKeyPart(attempt.EditionID) + "/" + deliveryKey[:32]
	tx, err := ledger.db.BeginTx(ctx, nil)
	if err != nil {
		return PrepareResult{}, fmt.Errorf("begin delivery ledger transaction: %w", err)
	}
	defer tx.Rollback()
	var status, storedContent, storedProvider, storedIdempotency, storedAuthorization, resolution, resolvedAt string
	err = tx.QueryRowContext(ctx, `
		SELECT status, content_sha256, provider, idempotency_key,
		       authorization_sha256, COALESCE(resolution, ''), COALESCE(resolved_at, '')
		FROM deliveries WHERE delivery_key = ?
	`, deliveryKey).Scan(&status, &storedContent, &storedProvider, &storedIdempotency, &storedAuthorization, &resolution, &resolvedAt)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		_, err = tx.ExecContext(ctx, `
			INSERT INTO deliveries (
				delivery_key, edition_id, audience, recipient_key, provider,
				content_sha256, idempotency_key, authorization_sha256, status, attempted_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 'pending', ?)
		`, deliveryKey, attempt.EditionID, attempt.Audience, recipientKey, attempt.Provider, attempt.ContentSHA256, idempotencyKey, attempt.AuthorizationSHA256, attempt.AttemptedAt.UTC().Format(time.RFC3339))
		if err != nil {
			return PrepareResult{}, fmt.Errorf("insert delivery ledger attempt: %w", err)
		}
	case err != nil:
		return PrepareResult{}, fmt.Errorf("read delivery ledger attempt: %w", err)
	case storedContent != attempt.ContentSHA256 || storedProvider != attempt.Provider || storedIdempotency != idempotencyKey:
		return PrepareResult{}, errors.New("existing delivery ledger entry does not match the current content or provider")
	case status == "accepted":
		if err := tx.Commit(); err != nil {
			return PrepareResult{}, err
		}
		return PrepareResult{DeliveryKey: deliveryKey, IdempotencyKey: idempotencyKey, AlreadyAccepted: true}, nil
	case status == "pending" && resolution == "":
		return PrepareResult{}, ErrDeliveryPending
	case status == "pending" && resolution == "not_accepted" && storedAuthorization == attempt.AuthorizationSHA256:
		return PrepareResult{}, ErrNewAuthorizationRequired
	case status == "pending" && resolution == "not_accepted":
		resolvedTime, parseErr := time.Parse(time.RFC3339, resolvedAt)
		if parseErr != nil || !attempt.AttemptedAt.UTC().After(resolvedTime) {
			return PrepareResult{}, errors.New("reconciled delivery retry must occur after the recorded resolution")
		}
		update, updateErr := tx.ExecContext(ctx, `
			UPDATE deliveries
			SET authorization_sha256 = ?, attempted_at = ?, resolution = NULL,
			    resolved_at = NULL, resolved_by = NULL, resolution_evidence = NULL
			WHERE delivery_key = ? AND status = 'pending' AND resolution = 'not_accepted'
		`, attempt.AuthorizationSHA256, attempt.AttemptedAt.UTC().Format(time.RFC3339), deliveryKey)
		if updateErr != nil {
			return PrepareResult{}, fmt.Errorf("prepare reconciled delivery retry: %w", updateErr)
		}
		changed, updateErr := update.RowsAffected()
		if updateErr != nil || changed != 1 {
			return PrepareResult{}, errors.New("reconciled delivery retry did not update one pending entry")
		}
	default:
		return PrepareResult{}, fmt.Errorf("delivery ledger contains unsupported status %q or resolution %q", status, resolution)
	}
	if err := tx.Commit(); err != nil {
		return PrepareResult{}, fmt.Errorf("commit delivery ledger attempt: %w", err)
	}
	return PrepareResult{DeliveryKey: deliveryKey, IdempotencyKey: idempotencyKey}, nil
}

func (ledger *Ledger) Reconcile(ctx context.Context, request ReconciliationRequest) (ReconciliationResult, error) {
	if request.EditionID == "" || request.Audience == "" || request.ResolvedAt.IsZero() {
		return ReconciliationResult{}, errors.New("reconciliation is missing edition, audience, or time")
	}
	address, err := canonicalRecipient(request.Email)
	if err != nil {
		return ReconciliationResult{}, err
	}
	if request.Outcome != "accepted" && request.Outcome != "not_accepted" {
		return ReconciliationResult{}, errors.New("reconciliation outcome must be accepted or not_accepted")
	}
	if err := validateAuditLabel(request.ResolvedBy, "resolved-by", 120); err != nil {
		return ReconciliationResult{}, err
	}
	if err := validateAuditLabel(request.Evidence, "evidence", 256); err != nil {
		return ReconciliationResult{}, err
	}
	providerMessageID := strings.TrimSpace(request.ProviderMessageID)
	if request.Outcome == "accepted" {
		if !validProviderMessageID(providerMessageID) {
			return ReconciliationResult{}, errors.New("accepted reconciliation requires a valid provider message ID")
		}
	} else if providerMessageID != "" {
		return ReconciliationResult{}, errors.New("not_accepted reconciliation must not include a provider message ID")
	}
	deliveryKey := ledger.key("delivery", request.EditionID, request.Audience, address)
	tx, err := ledger.db.BeginTx(ctx, nil)
	if err != nil {
		return ReconciliationResult{}, fmt.Errorf("begin delivery reconciliation: %w", err)
	}
	defer tx.Rollback()
	var status, resolution, storedMessageID, attemptedAt string
	err = tx.QueryRowContext(ctx, `
		SELECT status, COALESCE(resolution, ''), COALESCE(provider_message_id, ''), attempted_at
		FROM deliveries WHERE delivery_key = ?
	`, deliveryKey).Scan(&status, &resolution, &storedMessageID, &attemptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ReconciliationResult{}, errors.New("no delivery ledger entry matches the private recipient identity")
	}
	if err != nil {
		return ReconciliationResult{}, fmt.Errorf("read pending delivery: %w", err)
	}
	attemptedTime, err := time.Parse(time.RFC3339, attemptedAt)
	if err != nil || request.ResolvedAt.UTC().Before(attemptedTime) {
		return ReconciliationResult{}, errors.New("reconciliation time cannot predate the delivery attempt")
	}
	if status == "accepted" {
		if request.Outcome == "accepted" && providerMessageID == storedMessageID {
			if err := tx.Commit(); err != nil {
				return ReconciliationResult{}, err
			}
			return ReconciliationResult{AlreadyResolved: true}, nil
		}
		return ReconciliationResult{}, errors.New("an accepted delivery cannot be reconciled as not accepted or to a different provider message")
	}
	if status != "pending" {
		return ReconciliationResult{}, fmt.Errorf("delivery ledger contains unsupported status %q", status)
	}
	if resolution != "" {
		if resolution == request.Outcome {
			if err := tx.Commit(); err != nil {
				return ReconciliationResult{}, err
			}
			return ReconciliationResult{AlreadyResolved: true}, nil
		}
		return ReconciliationResult{}, errors.New("pending delivery already has a different reconciliation outcome")
	}
	resolvedAt := request.ResolvedAt.UTC().Format(time.RFC3339)
	var update sql.Result
	if request.Outcome == "accepted" {
		update, err = tx.ExecContext(ctx, `
			UPDATE deliveries
			SET status = 'accepted', provider_message_id = ?, accepted_at = ?,
			    resolution = 'accepted', resolved_at = ?, resolved_by = ?, resolution_evidence = ?
			WHERE delivery_key = ? AND status = 'pending' AND resolution IS NULL
		`, providerMessageID, resolvedAt, resolvedAt, request.ResolvedBy, request.Evidence, deliveryKey)
	} else {
		update, err = tx.ExecContext(ctx, `
			UPDATE deliveries
			SET resolution = 'not_accepted', resolved_at = ?, resolved_by = ?, resolution_evidence = ?
			WHERE delivery_key = ? AND status = 'pending' AND resolution IS NULL
		`, resolvedAt, request.ResolvedBy, request.Evidence, deliveryKey)
	}
	if err != nil {
		return ReconciliationResult{}, fmt.Errorf("record delivery reconciliation: %w", err)
	}
	changed, err := update.RowsAffected()
	if err != nil || changed != 1 {
		return ReconciliationResult{}, errors.New("delivery reconciliation did not update one pending entry")
	}
	if err := tx.Commit(); err != nil {
		return ReconciliationResult{}, fmt.Errorf("commit delivery reconciliation: %w", err)
	}
	return ReconciliationResult{Changed: true}, nil
}

func (ledger *Ledger) MarkAccepted(ctx context.Context, deliveryKey, providerMessageID string, acceptedAt time.Time) error {
	providerMessageID = strings.TrimSpace(providerMessageID)
	if deliveryKey == "" || !validProviderMessageID(providerMessageID) || acceptedAt.IsZero() {
		return errors.New("accepted delivery record is invalid")
	}
	result, err := ledger.db.ExecContext(ctx, `
		UPDATE deliveries
		SET status = 'accepted', provider_message_id = ?, accepted_at = ?
		WHERE delivery_key = ? AND status = 'pending'
	`, providerMessageID, acceptedAt.UTC().Format(time.RFC3339), deliveryKey)
	if err != nil {
		return fmt.Errorf("mark delivery accepted: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return errors.New("delivery ledger did not contain one pending attempt")
	}
	return nil
}

func (ledger *Ledger) Counts(ctx context.Context, editionID, audience string) (pending, accepted int, err error) {
	rows, err := ledger.db.QueryContext(ctx, `
		SELECT status, COUNT(*) FROM deliveries
		WHERE edition_id = ? AND audience = ? GROUP BY status
	`, editionID, audience)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return 0, 0, err
		}
		switch status {
		case "pending":
			pending = count
		case "accepted":
			accepted = count
		default:
			return 0, 0, fmt.Errorf("delivery ledger contains unsupported status %q", status)
		}
	}
	return pending, accepted, rows.Err()
}

func (ledger *Ledger) key(purpose string, values ...string) string {
	mac := hmac.New(sha256.New, ledger.secret)
	_, _ = mac.Write([]byte(purpose))
	for _, value := range values {
		_, _ = mac.Write([]byte{0})
		_, _ = mac.Write([]byte(value))
	}
	return hex.EncodeToString(mac.Sum(nil))
}

func (ledger *Ledger) migrate() error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`PRAGMA journal_mode = WAL;`,
		`PRAGMA busy_timeout = 5000;`,
		`CREATE TABLE IF NOT EXISTS deliveries (
			delivery_key TEXT PRIMARY KEY,
			edition_id TEXT NOT NULL,
			audience TEXT NOT NULL,
			recipient_key TEXT NOT NULL,
			provider TEXT NOT NULL,
			content_sha256 TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			authorization_sha256 TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL CHECK (status IN ('pending', 'accepted')),
			attempted_at TEXT NOT NULL,
			accepted_at TEXT,
			provider_message_id TEXT,
			resolution TEXT CHECK (resolution IN ('accepted', 'not_accepted')),
			resolved_at TEXT,
			resolved_by TEXT,
			resolution_evidence TEXT,
			UNIQUE(edition_id, audience, recipient_key),
			CHECK ((status = 'pending' AND accepted_at IS NULL AND provider_message_id IS NULL)
			    OR (status = 'accepted' AND accepted_at IS NOT NULL AND provider_message_id IS NOT NULL))
		);`,
		`CREATE INDEX IF NOT EXISTS idx_deliveries_edition_status ON deliveries(edition_id, audience, status);`,
	}
	for _, statement := range statements {
		if _, err := ledger.db.Exec(statement); err != nil {
			return fmt.Errorf("migrate delivery ledger: %w", err)
		}
	}
	columns := []struct {
		name       string
		definition string
	}{
		{"authorization_sha256", "TEXT NOT NULL DEFAULT ''"},
		{"resolution", "TEXT"},
		{"resolved_at", "TEXT"},
		{"resolved_by", "TEXT"},
		{"resolution_evidence", "TEXT"},
	}
	for _, column := range columns {
		if err := ledger.ensureColumn(column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

func (ledger *Ledger) ensureColumn(name, definition string) error {
	rows, err := ledger.db.Query(`PRAGMA table_info(deliveries)`)
	if err != nil {
		return fmt.Errorf("inspect delivery ledger columns: %w", err)
	}
	found := false
	for rows.Next() {
		var cid int
		var columnName, columnType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			return err
		}
		if columnName == name {
			found = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if found {
		return nil
	}
	if _, err := ledger.db.Exec("ALTER TABLE deliveries ADD COLUMN " + name + " " + definition); err != nil {
		return fmt.Errorf("add delivery ledger column %s: %w", name, err)
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validateAuditLabel(value, name string, limit int) error {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || trimmed == "" || len([]rune(trimmed)) > limit || strings.Contains(trimmed, "@") || strings.Contains(trimmed, "://") {
		return fmt.Errorf("%s must be a non-sensitive bounded label", name)
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return fmt.Errorf("%s contains a control character", name)
		}
	}
	return nil
}

func validProviderMessageID(value string) bool {
	return value != "" && len(value) <= 256 && !strings.ContainsAny(value, "\r\n@") && !strings.Contains(value, "://")
}

func canonicalRecipient(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || strings.Count(value, "@") != 1 || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("recipient must be one plain email addr-spec")
	}
	return strings.ToLower(value), nil
}

func sanitizeKeyPart(value string) string {
	var result strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			result.WriteRune(char)
		}
	}
	if result.Len() == 0 {
		return "edition"
	}
	if result.Len() > 80 {
		return result.String()[:80]
	}
	return result.String()
}
