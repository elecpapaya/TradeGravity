package emaildelivery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"time"

	"tradegravity/internal/deliverypreflight"
)

const (
	authorizationSchemaVersion = "1.0"
	authorizationToolVersion   = "tradegravity-email-launch-approval/1.0"
	maximumAuthorizationAge    = time.Hour
	maximumPrivateFileSize     = 5 << 20
)

// Attestations records the operator controls that cannot be inferred from the
// repository or the provider API.
type Attestations struct {
	SenderDomainAuthenticated bool `json:"sender_domain_authenticated"`
	BounceComplaintReady      bool `json:"bounce_complaint_process_ready"`
	PrivacyControlsReviewed   bool `json:"privacy_controls_reviewed"`
	PilotRecipientsConfirmed  bool `json:"pilot_recipients_confirmed"`
}

// Authorization binds one short-lived live-send decision to a specific
// aggregate preflight and sender identity. It intentionally contains no
// recipient addresses or unsubscribe URLs.
type Authorization struct {
	SchemaVersion      string       `json:"schema_version"`
	Tool               string       `json:"tool"`
	Status             string       `json:"status"`
	Channel            string       `json:"channel"`
	Provider           string       `json:"provider"`
	EditionID          string       `json:"edition_id"`
	Audience           string       `json:"audience"`
	Sender             string       `json:"sender"`
	ReplyTo            string       `json:"reply_to,omitempty"`
	AuthorizedBy       string       `json:"authorized_by"`
	AuthorizedAt       string       `json:"authorized_at"`
	ExpiresAt          string       `json:"expires_at"`
	PreflightSHA256    string       `json:"preflight_sha256"`
	ManifestSHA256     string       `json:"manifest_sha256"`
	ApprovalSHA256     string       `json:"approval_sha256"`
	SubscriberSHA256   string       `json:"subscriber_csv_sha256"`
	SuppressionSHA256  string       `json:"suppression_csv_sha256"`
	EmailSHA256        string       `json:"email_template_sha256"`
	EligibleRecipients int          `json:"eligible_recipients"`
	MaxRecipients      int          `json:"max_recipients"`
	Attestations       Attestations `json:"attestations"`
	DeliveryAuthorized bool         `json:"delivery_authorized"`
	FileSHA256         string       `json:"-"`
}

type AuthorizationRequest struct {
	PreflightPath string
	Provider      string
	Sender        string
	ReplyTo       string
	AuthorizedBy  string
	AuthorizedAt  time.Time
	ExpiresAt     time.Time
	Attestations  Attestations
}

func Authorize(request AuthorizationRequest) (Authorization, []byte, error) {
	preflightRaw, err := readPrivateRegularFile(request.PreflightPath, "preflight")
	if err != nil {
		return Authorization{}, nil, err
	}
	plan, err := decodePreflight(preflightRaw)
	if err != nil {
		return Authorization{}, nil, err
	}
	if err := validatePreflightForAuthorization(plan); err != nil {
		return Authorization{}, nil, err
	}
	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if provider != "resend" {
		return Authorization{}, nil, errors.New("provider must be resend for the current pilot adapter")
	}
	sender, err := canonicalMailbox(request.Sender, "sender")
	if err != nil {
		return Authorization{}, nil, err
	}
	replyTo := ""
	if strings.TrimSpace(request.ReplyTo) != "" {
		replyTo, err = canonicalMailbox(request.ReplyTo, "reply-to")
		if err != nil {
			return Authorization{}, nil, err
		}
	}
	operator := strings.TrimSpace(request.AuthorizedBy)
	if operator == "" || len([]rune(operator)) > 120 || strings.ContainsAny(operator, "\r\n") {
		return Authorization{}, nil, errors.New("authorized-by is required and must be a single bounded label")
	}
	if request.AuthorizedAt.IsZero() || request.ExpiresAt.IsZero() {
		return Authorization{}, nil, errors.New("authorization and expiry times are required")
	}
	authorizedAt := request.AuthorizedAt.UTC()
	expiresAt := request.ExpiresAt.UTC()
	if !expiresAt.After(authorizedAt) || expiresAt.Sub(authorizedAt) > maximumAuthorizationAge {
		return Authorization{}, nil, errors.New("launch authorization must expire after authorization and within one hour")
	}
	preflightAt, err := time.Parse(time.RFC3339, plan.GeneratedAt)
	if err != nil || authorizedAt.Before(preflightAt) {
		return Authorization{}, nil, errors.New("launch authorization cannot predate the delivery preflight")
	}
	if !allAttested(request.Attestations) {
		return Authorization{}, nil, errors.New("all launch attestations are required")
	}
	digest := sha256.Sum256(preflightRaw)
	authorization := Authorization{
		SchemaVersion:      authorizationSchemaVersion,
		Tool:               authorizationToolVersion,
		Status:             "live_send_authorized",
		Channel:            "email",
		Provider:           provider,
		EditionID:          plan.EditionID,
		Audience:           plan.Audience,
		Sender:             sender,
		ReplyTo:            replyTo,
		AuthorizedBy:       operator,
		AuthorizedAt:       authorizedAt.Format(time.RFC3339),
		ExpiresAt:          expiresAt.Format(time.RFC3339),
		PreflightSHA256:    hex.EncodeToString(digest[:]),
		ManifestSHA256:     plan.ManifestSHA256,
		ApprovalSHA256:     plan.ApprovalSHA256,
		SubscriberSHA256:   plan.Sources.SubscriberCSVSHA256,
		SuppressionSHA256:  plan.Sources.SuppressionCSVSHA256,
		EmailSHA256:        plan.EmailTemplateSHA256,
		EligibleRecipients: plan.Counts.Eligible,
		MaxRecipients:      plan.MaxRecipients,
		Attestations:       request.Attestations,
		DeliveryAuthorized: true,
	}
	raw, err := json.MarshalIndent(authorization, "", "  ")
	if err != nil {
		return Authorization{}, nil, fmt.Errorf("encode launch authorization: %w", err)
	}
	return authorization, append(raw, '\n'), nil
}

func WriteAuthorization(path string, raw []byte) error {
	return writePrivateExclusive(path, raw, "launch authorization")
}

func LoadAuthorization(path string, preflightRaw []byte, sendAt time.Time) (Authorization, error) {
	raw, err := readPrivateRegularFile(path, "launch authorization")
	if err != nil {
		return Authorization{}, err
	}
	var authorization Authorization
	if err := decodeStrictJSON(raw, &authorization); err != nil {
		return Authorization{}, fmt.Errorf("decode launch authorization: %w", err)
	}
	if authorization.SchemaVersion != authorizationSchemaVersion || authorization.Tool != authorizationToolVersion || authorization.Status != "live_send_authorized" || authorization.Channel != "email" || authorization.Provider != "resend" || !authorization.DeliveryAuthorized {
		return Authorization{}, errors.New("launch authorization has an unsupported contract or inactive status")
	}
	if !allAttested(authorization.Attestations) {
		return Authorization{}, errors.New("launch authorization is missing required attestations")
	}
	if _, err := canonicalMailbox(authorization.Sender, "sender"); err != nil {
		return Authorization{}, err
	}
	if authorization.ReplyTo != "" {
		if _, err := canonicalMailbox(authorization.ReplyTo, "reply-to"); err != nil {
			return Authorization{}, err
		}
	}
	authorizedAt, err := time.Parse(time.RFC3339, authorization.AuthorizedAt)
	if err != nil {
		return Authorization{}, errors.New("launch authorization time is invalid")
	}
	expiresAt, err := time.Parse(time.RFC3339, authorization.ExpiresAt)
	if err != nil || !expiresAt.After(authorizedAt) || expiresAt.Sub(authorizedAt) > maximumAuthorizationAge {
		return Authorization{}, errors.New("launch authorization expiry is invalid")
	}
	if sendAt.IsZero() || sendAt.UTC().Before(authorizedAt) || sendAt.UTC().After(expiresAt) {
		return Authorization{}, errors.New("send time is outside the launch authorization window")
	}
	digest := sha256.Sum256(preflightRaw)
	if authorization.PreflightSHA256 != hex.EncodeToString(digest[:]) {
		return Authorization{}, errors.New("launch authorization does not match the supplied preflight")
	}
	authorizationDigest := sha256.Sum256(raw)
	authorization.FileSHA256 = hex.EncodeToString(authorizationDigest[:])
	return authorization, nil
}

func VerifyLivePlan(authorization Authorization, original, live deliverypreflight.Plan) error {
	if original.EditionID != authorization.EditionID || live.EditionID != authorization.EditionID || original.Audience != authorization.Audience || live.Audience != authorization.Audience {
		return errors.New("live preflight edition or audience differs from launch authorization")
	}
	if original.ManifestSHA256 != authorization.ManifestSHA256 || live.ManifestSHA256 != authorization.ManifestSHA256 || original.ApprovalSHA256 != authorization.ApprovalSHA256 || live.ApprovalSHA256 != authorization.ApprovalSHA256 || original.EmailTemplateSHA256 != authorization.EmailSHA256 || live.EmailTemplateSHA256 != authorization.EmailSHA256 {
		return errors.New("live content digests differ from launch authorization")
	}
	if original.Sources.SubscriberCSVSHA256 != authorization.SubscriberSHA256 || live.Sources.SubscriberCSVSHA256 != authorization.SubscriberSHA256 || original.Sources.SuppressionCSVSHA256 != authorization.SuppressionSHA256 || live.Sources.SuppressionCSVSHA256 != authorization.SuppressionSHA256 {
		return errors.New("live consent or suppression inputs differ from launch authorization")
	}
	if original.Counts.Eligible != authorization.EligibleRecipients || live.Counts.Eligible != authorization.EligibleRecipients || original.MaxRecipients != authorization.MaxRecipients || live.MaxRecipients != authorization.MaxRecipients {
		return errors.New("live recipient counts or pilot ceiling differ from launch authorization")
	}
	if !live.ConsentValidated || !live.SuppressionApplied || !live.UnsubscribeURLsValidated || live.ContainsRecipientAddresses || live.DeliveryAuthorized {
		return errors.New("live preflight safety invariants are not satisfied")
	}
	return nil
}

func ReadPreflight(path string) (deliverypreflight.Plan, []byte, error) {
	raw, err := readPrivateRegularFile(path, "preflight")
	if err != nil {
		return deliverypreflight.Plan{}, nil, err
	}
	plan, err := decodePreflight(raw)
	if err != nil {
		return deliverypreflight.Plan{}, nil, err
	}
	return plan, raw, nil
}

func decodePreflight(raw []byte) (deliverypreflight.Plan, error) {
	var plan deliverypreflight.Plan
	if err := decodeStrictJSON(raw, &plan); err != nil {
		return deliverypreflight.Plan{}, fmt.Errorf("decode delivery preflight: %w", err)
	}
	return plan, nil
}

func validatePreflightForAuthorization(plan deliverypreflight.Plan) error {
	if plan.SchemaVersion != "1.0" || plan.Status != "consent_preflight_passed" || plan.Channel != "email" {
		return errors.New("delivery preflight has an unsupported contract or status")
	}
	if !plan.ConsentValidated || !plan.SuppressionApplied || !plan.UnsubscribeURLsValidated || plan.ContainsRecipientAddresses || plan.ProviderConfigured || plan.DeliveryAuthorized {
		return errors.New("delivery preflight safety invariants are not satisfied")
	}
	if plan.Counts.Eligible < 1 || plan.Counts.Eligible > plan.MaxRecipients || plan.MaxRecipients < 1 {
		return errors.New("delivery preflight recipient counts are invalid")
	}
	if plan.EditionID == "" || plan.Audience == "" || plan.ManifestSHA256 == "" || plan.ApprovalSHA256 == "" || plan.Sources.SubscriberCSVSHA256 == "" || plan.Sources.SuppressionCSVSHA256 == "" || plan.EmailTemplateSHA256 == "" {
		return errors.New("delivery preflight is missing required identities or digests")
	}
	if _, err := time.Parse(time.RFC3339, plan.GeneratedAt); err != nil {
		return errors.New("delivery preflight generation time is invalid")
	}
	return nil
}

func canonicalMailbox(value, label string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 320 || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s mailbox is empty or malformed", label)
	}
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address == "" {
		return "", fmt.Errorf("%s mailbox is malformed", label)
	}
	return parsed.String(), nil
}

func allAttested(value Attestations) bool {
	return value.SenderDomainAuthenticated && value.BounceComplaintReady && value.PrivacyControlsReviewed && value.PilotRecipientsConfirmed
}

func decodeStrictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains trailing content")
	}
	return nil
}

func readPrivateRegularFile(path, label string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("%s path is required", label)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s must be a regular non-symlink file", label)
	}
	if info.Size() > maximumPrivateFileSize {
		return nil, fmt.Errorf("%s exceeds the private file size limit", label)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	return raw, nil
}

func writePrivateExclusive(path string, raw []byte, label string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("%s output path is required", label)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve %s output: %w", label, err)
	}
	parent := filepath.Dir(absolute)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create %s output directory: %w", label, err)
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve %s output directory: %w", label, err)
	}
	target := filepath.Join(resolvedParent, filepath.Base(absolute))
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s output already exists", label)
		}
		return fmt.Errorf("create %s output: %w", label, err)
	}
	writeErr := func() error {
		if _, err := file.Write(raw); err != nil {
			return err
		}
		return file.Sync()
	}()
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(target)
		if writeErr != nil {
			return fmt.Errorf("write %s: %w", label, writeErr)
		}
		return fmt.Errorf("close %s: %w", label, closeErr)
	}
	return nil
}
