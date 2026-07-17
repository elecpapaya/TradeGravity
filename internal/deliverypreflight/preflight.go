package deliverypreflight

import (
	"bytes"
	"crypto/sha256"
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

	"tradegravity/internal/distributionkit"
)

const (
	schemaVersion = "1.0"
	toolVersion   = "tradegravity-delivery-preflight/1.0"
	maxSourceSize = 5 << 20
)

var subscriberHeader = []string{"email", "audience", "status", "consented_at", "consent_method", "consent_source", "privacy_notice_version", "unsubscribe_url"}
var suppressionHeader = []string{"email", "reason", "suppressed_at"}
var allowedSuppressionReasons = map[string]bool{
	"bounced":      true,
	"complaint":    true,
	"invalid":      true,
	"manual":       true,
	"unsubscribed": true,
}

type Request struct {
	KitDir         string
	SubscriberCSV  string
	SuppressionCSV string
	GeneratedAt    time.Time
	MaxRecipients  int
}

type Plan struct {
	SchemaVersion              string      `json:"schema_version"`
	Tool                       string      `json:"tool"`
	Status                     string      `json:"status"`
	Channel                    string      `json:"channel"`
	EditionID                  string      `json:"edition_id"`
	ManifestSHA256             string      `json:"manifest_sha256"`
	ApprovalSHA256             string      `json:"approval_sha256"`
	Audience                   string      `json:"audience"`
	GeneratedAt                string      `json:"generated_at"`
	Sources                    SourceProof `json:"sources"`
	Counts                     Counts      `json:"counts"`
	MaxRecipients              int         `json:"max_recipients"`
	ConsentValidated           bool        `json:"consent_validated"`
	SuppressionApplied         bool        `json:"suppression_applied"`
	UnsubscribeURLsValidated   bool        `json:"unsubscribe_urls_validated"`
	ContainsRecipientAddresses bool        `json:"contains_recipient_addresses"`
	ProviderConfigured         bool        `json:"provider_configured"`
	DeliveryAuthorized         bool        `json:"delivery_authorized"`
	UnsubscribePlaceholder     string      `json:"unsubscribe_placeholder"`
	RequiredProviderHeaders    []string    `json:"required_provider_headers"`
	ListUnsubscribePostValue   string      `json:"list_unsubscribe_post_value"`
	UnsubscribeHTTPSRequired   bool        `json:"unsubscribe_https_required"`
	RequiredDKIMCoveredHeaders []string    `json:"required_dkim_covered_headers"`
	EmailTemplateSHA256        string      `json:"email_template_sha256"`
}

type SourceProof struct {
	SubscriberCSVSHA256  string `json:"subscriber_csv_sha256"`
	SuppressionCSVSHA256 string `json:"suppression_csv_sha256"`
}

type Counts struct {
	Consented       int `json:"consented"`
	Suppressed      int `json:"suppressed"`
	SuppressionRows int `json:"suppression_rows"`
	Eligible        int `json:"eligible"`
}

type Result struct {
	Plan               Plan
	EligibleRecipients []Recipient
	JSON               []byte
}

type Recipient struct {
	Email          string
	UnsubscribeURL string
}

func Build(request Request) (Result, error) {
	if request.GeneratedAt.IsZero() {
		return Result{}, errors.New("preflight generation time is required")
	}
	if request.MaxRecipients < 1 || request.MaxRecipients > 1000 {
		return Result{}, errors.New("max recipients must be between 1 and 1000")
	}
	kitRoot, err := canonicalDirectory(request.KitDir)
	if err != nil {
		return Result{}, err
	}
	approval, manifest, err := distributionkit.VerifyApproved(kitRoot, "email")
	if err != nil {
		return Result{}, fmt.Errorf("verify approved email kit: %w", err)
	}

	subscriberRaw, subscriberPath, err := readExternalSource(kitRoot, request.SubscriberCSV, "subscriber CSV")
	if err != nil {
		return Result{}, err
	}
	suppressionRaw, suppressionPath, err := readExternalSource(kitRoot, request.SuppressionCSV, "suppression CSV")
	if err != nil {
		return Result{}, err
	}
	if samePath(subscriberPath, suppressionPath) {
		return Result{}, errors.New("subscriber and suppression CSVs must be different files")
	}

	consented, err := parseSubscribers(subscriberRaw, approval.Audience, request.GeneratedAt)
	if err != nil {
		return Result{}, err
	}
	suppressed, suppressionRows, err := parseSuppressions(suppressionRaw, request.GeneratedAt)
	if err != nil {
		return Result{}, err
	}
	eligible := make([]Recipient, 0, len(consented))
	matchedSuppressions := 0
	for _, recipient := range consented {
		if suppressed[recipient.Email] {
			matchedSuppressions++
			continue
		}
		eligible = append(eligible, recipient)
	}
	if len(eligible) == 0 {
		return Result{}, errors.New("no eligible recipients remain after suppression")
	}
	if len(eligible) > request.MaxRecipients {
		return Result{}, fmt.Errorf("eligible recipient count %d exceeds pilot limit %d", len(eligible), request.MaxRecipients)
	}
	sort.Slice(eligible, func(first, second int) bool {
		return eligible[first].Email < eligible[second].Email
	})

	templateRaw, templateDigest, err := verifiedEmailTemplate(kitRoot, manifest)
	if err != nil {
		return Result{}, err
	}
	if bytes.Count(templateRaw, []byte("{{UNSUBSCRIBE_URL}}")) != 1 {
		return Result{}, errors.New("approved email template must contain exactly one unsubscribe placeholder")
	}
	markdownRaw, err := os.ReadFile(filepath.Join(kitRoot, "email", "body.md"))
	if err != nil {
		return Result{}, fmt.Errorf("read approved email Markdown: %w", err)
	}
	if bytes.Count(markdownRaw, []byte("{{UNSUBSCRIBE_URL}}")) != 1 {
		return Result{}, errors.New("approved email Markdown must contain exactly one unsubscribe placeholder")
	}
	lowerTemplate := bytes.ToLower(templateRaw)
	if bytes.Contains(lowerTemplate, []byte("<script")) || bytes.Contains(lowerTemplate, []byte("<img")) {
		return Result{}, errors.New("approved email template contains a forbidden active or tracking element")
	}

	manifestRaw, err := os.ReadFile(filepath.Join(kitRoot, "manifest.json"))
	if err != nil {
		return Result{}, fmt.Errorf("read approved manifest: %w", err)
	}
	approvalRaw, err := os.ReadFile(filepath.Join(kitRoot, "approval.json"))
	if err != nil {
		return Result{}, fmt.Errorf("read content approval: %w", err)
	}
	manifestDigest := sha256.Sum256(manifestRaw)
	approvalDigest := sha256.Sum256(approvalRaw)
	subscriberDigest := sha256.Sum256(subscriberRaw)
	suppressionDigest := sha256.Sum256(suppressionRaw)
	plan := Plan{
		SchemaVersion:  schemaVersion,
		Tool:           toolVersion,
		Status:         "consent_preflight_passed",
		Channel:        "email",
		EditionID:      manifest.EditionID,
		ManifestSHA256: hex.EncodeToString(manifestDigest[:]),
		ApprovalSHA256: hex.EncodeToString(approvalDigest[:]),
		Audience:       approval.Audience,
		GeneratedAt:    request.GeneratedAt.UTC().Format(time.RFC3339),
		Sources: SourceProof{
			SubscriberCSVSHA256:  hex.EncodeToString(subscriberDigest[:]),
			SuppressionCSVSHA256: hex.EncodeToString(suppressionDigest[:]),
		},
		Counts: Counts{
			Consented:       len(consented),
			Suppressed:      matchedSuppressions,
			SuppressionRows: suppressionRows,
			Eligible:        len(eligible),
		},
		MaxRecipients:              request.MaxRecipients,
		ConsentValidated:           true,
		SuppressionApplied:         true,
		UnsubscribeURLsValidated:   true,
		ContainsRecipientAddresses: false,
		ProviderConfigured:         false,
		DeliveryAuthorized:         false,
		UnsubscribePlaceholder:     "{{UNSUBSCRIBE_URL}}",
		RequiredProviderHeaders:    []string{"List-Unsubscribe", "List-Unsubscribe-Post"},
		ListUnsubscribePostValue:   "List-Unsubscribe=One-Click",
		UnsubscribeHTTPSRequired:   true,
		RequiredDKIMCoveredHeaders: []string{"List-Unsubscribe", "List-Unsubscribe-Post"},
		EmailTemplateSHA256:        templateDigest,
	}
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("encode delivery preflight: %w", err)
	}
	return Result{Plan: plan, EligibleRecipients: eligible, JSON: append(raw, '\n')}, nil
}

func Write(outputPath, kitDir string, content []byte) error {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return errors.New("preflight output path is required")
	}
	root, err := canonicalDirectory(kitDir)
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve preflight output: %w", err)
	}
	parent := filepath.Dir(absolute)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create preflight output directory: %w", err)
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve preflight output directory links: %w", err)
	}
	target := filepath.Join(resolvedParent, filepath.Base(absolute))
	if pathWithin(root, target) {
		return errors.New("preflight output must stay outside the distribution kit")
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return errors.New("preflight output already exists")
		}
		return fmt.Errorf("create preflight output: %w", err)
	}
	writeErr := func() error {
		if _, err := file.Write(content); err != nil {
			return err
		}
		return file.Sync()
	}()
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(target)
		if writeErr != nil {
			return fmt.Errorf("write preflight output: %w", writeErr)
		}
		return fmt.Errorf("close preflight output: %w", closeErr)
	}
	return nil
}

func parseSubscribers(raw []byte, audience string, generatedAt time.Time) ([]Recipient, error) {
	records, err := parseCSV(raw, subscriberHeader, "subscriber CSV")
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, errors.New("subscriber CSV contains no consented records")
	}
	seenAddresses := map[string]bool{}
	seenUnsubscribeURLs := map[string]bool{}
	recipients := make([]Recipient, 0, len(records))
	for index, row := range records {
		line := index + 2
		address, err := canonicalEmail(row[0])
		if err != nil {
			return nil, fmt.Errorf("subscriber CSV line %d: %w", line, err)
		}
		if seenAddresses[address] {
			return nil, fmt.Errorf("subscriber CSV line %d duplicates an email address", line)
		}
		seenAddresses[address] = true
		if row[1] != audience {
			return nil, fmt.Errorf("subscriber CSV line %d audience does not match approved audience", line)
		}
		if row[2] != "active" || row[4] != "double_opt_in" {
			return nil, fmt.Errorf("subscriber CSV line %d is not active double opt-in consent", line)
		}
		if err := validateTimestamp(row[3], generatedAt, "consent", line); err != nil {
			return nil, err
		}
		if err := validateLabel(row[5], "consent source", line); err != nil {
			return nil, err
		}
		if err := validateLabel(row[6], "privacy notice version", line); err != nil {
			return nil, err
		}
		unsubscribeURL, err := validateUnsubscribeURL(row[7], address, line)
		if err != nil {
			return nil, err
		}
		if seenUnsubscribeURLs[unsubscribeURL] {
			return nil, fmt.Errorf("subscriber CSV line %d duplicates an unsubscribe URL", line)
		}
		seenUnsubscribeURLs[unsubscribeURL] = true
		recipients = append(recipients, Recipient{Email: address, UnsubscribeURL: unsubscribeURL})
	}
	return recipients, nil
}

func validateUnsubscribeURL(value, address string, line int) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("subscriber CSV line %d has an invalid unsubscribe URL", line)
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return "", fmt.Errorf("subscriber CSV line %d unsubscribe URL must be absolute HTTPS without credentials or a fragment", line)
	}
	if (parsed.Path == "" || parsed.Path == "/") && parsed.RawQuery == "" {
		return "", fmt.Errorf("subscriber CSV line %d unsubscribe URL must contain an opaque recipient token", line)
	}
	decoded, err := url.QueryUnescape(value)
	if err != nil {
		return "", fmt.Errorf("subscriber CSV line %d has invalid unsubscribe URL escaping", line)
	}
	if strings.Contains(strings.ToLower(decoded), strings.ToLower(address)) || strings.Contains(decoded, "@") {
		return "", fmt.Errorf("subscriber CSV line %d unsubscribe URL must not expose an email address", line)
	}
	return parsed.String(), nil
}

func parseSuppressions(raw []byte, generatedAt time.Time) (map[string]bool, int, error) {
	records, err := parseCSV(raw, suppressionHeader, "suppression CSV")
	if err != nil {
		return nil, 0, err
	}
	result := map[string]bool{}
	for index, row := range records {
		line := index + 2
		address, err := canonicalEmail(row[0])
		if err != nil {
			return nil, 0, fmt.Errorf("suppression CSV line %d: %w", line, err)
		}
		if result[address] {
			return nil, 0, fmt.Errorf("suppression CSV line %d duplicates an email address", line)
		}
		if !allowedSuppressionReasons[row[1]] {
			return nil, 0, fmt.Errorf("suppression CSV line %d has unsupported reason %q", line, row[1])
		}
		if err := validateTimestamp(row[2], generatedAt, "suppression", line); err != nil {
			return nil, 0, err
		}
		result[address] = true
	}
	return result, len(records), nil
}

func parseCSV(raw []byte, header []string, label string) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = len(header)
	records := make([][]string, 0)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", label, err)
		}
		for _, value := range record {
			if value != strings.TrimSpace(value) {
				return nil, fmt.Errorf("%s contains leading or trailing whitespace", label)
			}
		}
		records = append(records, record)
	}
	if len(records) == 0 || !equalStrings(records[0], header) {
		return nil, fmt.Errorf("%s header must be exactly %s", label, strings.Join(header, ","))
	}
	return records[1:], nil
}

func canonicalEmail(value string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || strings.Count(value, "@") != 1 {
		return "", errors.New("email address is empty or malformed")
	}
	for _, r := range value {
		if r > unicode.MaxASCII || unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", errors.New("email address must use a plain ASCII addr-spec")
		}
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value {
		return "", errors.New("email address must not contain a display name or invalid syntax")
	}
	parts := strings.Split(value, "@")
	if parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") || strings.HasPrefix(parts[1], ".") || strings.HasSuffix(parts[1], ".") {
		return "", errors.New("email address must contain a valid-looking domain")
	}
	return strings.ToLower(value), nil
}

func validateTimestamp(value string, generatedAt time.Time, kind string, line int) error {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return fmt.Errorf("CSV line %d has invalid %s timestamp", line, kind)
	}
	if parsed.After(generatedAt) {
		return fmt.Errorf("CSV line %d has future %s timestamp", line, kind)
	}
	return nil
}

func validateLabel(value, name string, line int) error {
	if value == "" || len([]rune(value)) > 120 {
		return fmt.Errorf("CSV line %d has invalid %s", line, name)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("CSV line %d has invalid %s", line, name)
		}
	}
	return nil
}

func verifiedEmailTemplate(kitRoot string, manifest distributionkit.Manifest) ([]byte, string, error) {
	var declared *distributionkit.ManifestFile
	for index := range manifest.Files {
		if manifest.Files[index].Path == "email/body.html" {
			declared = &manifest.Files[index]
			break
		}
	}
	if declared == nil {
		return nil, "", errors.New("approved manifest does not contain email/body.html")
	}
	raw, err := os.ReadFile(filepath.Join(kitRoot, "email", "body.html"))
	if err != nil {
		return nil, "", fmt.Errorf("read approved email template: %w", err)
	}
	digest := sha256.Sum256(raw)
	value := hex.EncodeToString(digest[:])
	if value != declared.SHA256 {
		return nil, "", errors.New("approved email template digest does not match manifest")
	}
	return raw, value, nil
}

func readExternalSource(kitRoot, path, label string) ([]byte, string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, "", fmt.Errorf("%s path is required", label)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve %s: %w", label, err)
	}
	originalInfo, err := os.Lstat(absolute)
	if err != nil {
		return nil, "", fmt.Errorf("inspect %s: %w", label, err)
	}
	if originalInfo.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmt.Errorf("%s must be a regular non-symlink file", label)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, "", fmt.Errorf("resolve %s links: %w", label, err)
	}
	if pathWithin(kitRoot, resolved) {
		return nil, "", fmt.Errorf("%s must stay outside the distribution kit", label)
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, "", fmt.Errorf("%s must be a regular non-symlink file", label)
	}
	if info.Size() > maxSourceSize {
		return nil, "", fmt.Errorf("%s exceeds the %d-byte limit", label, maxSourceSize)
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", label, err)
	}
	return raw, resolved, nil
}

func canonicalDirectory(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("distribution-kit directory is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve distribution-kit directory: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve distribution-kit links: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("distribution-kit path must be a directory")
	}
	return resolved, nil
}

func pathWithin(root, candidate string) bool {
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func samePath(first, second string) bool {
	return strings.EqualFold(filepath.Clean(first), filepath.Clean(second))
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
