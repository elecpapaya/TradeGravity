package distributionkit

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	approvalSchemaVersion = "1.0"
	approvalScope         = "content_release"
)

var allowedApprovalChannels = map[string]bool{
	"email":     true,
	"instagram": true,
}

var contentApprovalAttestations = []string{
	"evidence_periods_values_and_units_reviewed",
	"claims_caveats_links_and_rights_reviewed",
	"final_channel_assets_and_alt_text_reviewed",
}

type ApprovalRequest struct {
	Reviewer   string
	Audience   string
	Channels   []string
	ApprovedAt time.Time
	Attested   bool
}

type Approval struct {
	SchemaVersion          string   `json:"schema_version"`
	Tool                   string   `json:"tool"`
	Scope                  string   `json:"scope"`
	Status                 string   `json:"status"`
	EditionID              string   `json:"edition_id"`
	ManifestSHA256         string   `json:"manifest_sha256"`
	ManifestFileCount      int      `json:"manifest_file_count"`
	ApprovedAt             string   `json:"approved_at"`
	Reviewer               string   `json:"reviewer"`
	Audience               string   `json:"audience"`
	Channels               []string `json:"channels"`
	Attestations           []string `json:"attestations"`
	ProviderDeliveryReady  bool     `json:"provider_delivery_ready"`
	SubscriberConsentReady bool     `json:"subscriber_consent_ready"`
	AutomaticPublishReady  bool     `json:"automatic_publish_ready"`
}

func Approve(kitDir string, request ApprovalRequest) (Approval, []byte, error) {
	if err := validateApprovalRequest(request); err != nil {
		return Approval{}, nil, err
	}
	manifest, manifestRaw, err := Verify(kitDir)
	if err != nil {
		return Approval{}, nil, err
	}
	digest := sha256.Sum256(manifestRaw)
	channels := append([]string(nil), request.Channels...)
	sort.Strings(channels)
	approval := Approval{
		SchemaVersion:          approvalSchemaVersion,
		Tool:                   kitToolVersion,
		Scope:                  approvalScope,
		Status:                 "approved",
		EditionID:              manifest.EditionID,
		ManifestSHA256:         hex.EncodeToString(digest[:]),
		ManifestFileCount:      len(manifest.Files),
		ApprovedAt:             request.ApprovedAt.UTC().Format(time.RFC3339),
		Reviewer:               strings.TrimSpace(request.Reviewer),
		Audience:               strings.TrimSpace(request.Audience),
		Channels:               channels,
		Attestations:           append([]string(nil), contentApprovalAttestations...),
		ProviderDeliveryReady:  false,
		SubscriberConsentReady: false,
		AutomaticPublishReady:  false,
	}
	raw, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return Approval{}, nil, fmt.Errorf("encode approval: %w", err)
	}
	return approval, append(raw, '\n'), nil
}

func Verify(kitDir string) (Manifest, []byte, error) {
	return verifyKit(kitDir, false)
}

func VerifyApproved(kitDir, requiredChannel string) (Approval, Manifest, error) {
	requiredChannel = strings.TrimSpace(requiredChannel)
	if !allowedApprovalChannels[requiredChannel] {
		return Approval{}, Manifest{}, fmt.Errorf("unsupported required channel %q", requiredChannel)
	}
	root, err := cleanKitRoot(kitDir)
	if err != nil {
		return Approval{}, Manifest{}, err
	}
	manifest, manifestRaw, err := verifyKit(root, true)
	if err != nil {
		return Approval{}, Manifest{}, err
	}
	approvalRaw, err := os.ReadFile(filepath.Join(root, "approval.json"))
	if err != nil {
		return Approval{}, Manifest{}, fmt.Errorf("read approval.json: %w", err)
	}
	var approval Approval
	decoder := json.NewDecoder(bytes.NewReader(approvalRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&approval); err != nil {
		return Approval{}, Manifest{}, fmt.Errorf("decode approval.json: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Approval{}, Manifest{}, errors.New("approval.json contains trailing JSON values")
	}
	canonicalApproval, err := marshalCanonicalJSON(approval)
	if err != nil {
		return Approval{}, Manifest{}, fmt.Errorf("canonicalize approval.json: %w", err)
	}
	if !bytes.Equal(approvalRaw, canonicalApproval) {
		return Approval{}, Manifest{}, errors.New("approval.json must use the canonical generated encoding")
	}
	if err := validateStoredApproval(approval, manifest, manifestRaw, requiredChannel); err != nil {
		return Approval{}, Manifest{}, err
	}
	return approval, manifest, nil
}

func verifyKit(kitDir string, allowApproval bool) (Manifest, []byte, error) {
	root, err := cleanKitRoot(kitDir)
	if err != nil {
		return Manifest{}, nil, err
	}
	manifestPath := filepath.Join(root, "manifest.json")
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("inspect manifest: %w", err)
	}
	if !manifestInfo.Mode().IsRegular() {
		return Manifest{}, nil, errors.New("manifest.json must be a regular file")
	}
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("read manifest: %w", err)
	}
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(manifestRaw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Manifest{}, nil, errors.New("manifest contains trailing JSON values")
	}
	canonicalManifest, err := marshalCanonicalJSON(manifest)
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("canonicalize manifest: %w", err)
	}
	if !bytes.Equal(manifestRaw, canonicalManifest) {
		return Manifest{}, nil, errors.New("manifest.json must use the canonical generated encoding")
	}
	if err := validateApprovalManifest(manifest); err != nil {
		return Manifest{}, nil, err
	}

	expected := map[string]ManifestFile{"manifest.json": {Path: "manifest.json"}}
	if allowApproval {
		expected["approval.json"] = ManifestFile{Path: "approval.json"}
	}
	previousPath := ""
	for _, item := range manifest.Files {
		if !fs.ValidPath(item.Path) || strings.Contains(item.Path, "\\") || item.Path == "." || item.Path == "manifest.json" || item.Path == "approval.json" {
			return Manifest{}, nil, fmt.Errorf("manifest contains invalid file path %q", item.Path)
		}
		if item.Path <= previousPath {
			return Manifest{}, nil, errors.New("manifest files must be unique and sorted by path")
		}
		previousPath = item.Path
		if item.MediaType != mediaType(item.Path) || item.Bytes < 0 || len(item.SHA256) != sha256.Size*2 {
			return Manifest{}, nil, fmt.Errorf("manifest metadata is invalid for %s", item.Path)
		}
		if _, err := hex.DecodeString(item.SHA256); err != nil {
			return Manifest{}, nil, fmt.Errorf("manifest digest is invalid for %s", item.Path)
		}
		expected[item.Path] = item
	}

	seen := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("distribution kit must not contain symlinks: %s", entry.Name())
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("distribution kit contains a non-regular file: %s", entry.Name())
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		item, ok := expected[relative]
		if !ok {
			return fmt.Errorf("distribution kit contains untracked file %q", relative)
		}
		seen[relative] = true
		if relative == "manifest.json" || relative == "approval.json" {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if len(content) != item.Bytes {
			return fmt.Errorf("distribution file size changed: %s", relative)
		}
		digest := sha256.Sum256(content)
		if !strings.EqualFold(hex.EncodeToString(digest[:]), item.SHA256) {
			return fmt.Errorf("distribution file digest changed: %s", relative)
		}
		return nil
	})
	if err != nil {
		return Manifest{}, nil, fmt.Errorf("verify distribution kit: %w", err)
	}
	if len(seen) != len(expected) {
		missing := make([]string, 0)
		for path := range expected {
			if !seen[path] {
				missing = append(missing, path)
			}
		}
		sort.Strings(missing)
		return Manifest{}, nil, fmt.Errorf("distribution kit is missing files: %s", strings.Join(missing, ", "))
	}
	return manifest, manifestRaw, nil
}

func WriteApproval(kitDir string, content []byte) error {
	root, err := cleanKitRoot(kitDir)
	if err != nil {
		return err
	}
	target := filepath.Join(root, "approval.json")
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return errors.New("approval.json already exists; rebuild the kit for a new approval")
		}
		return fmt.Errorf("create approval.json: %w", err)
	}
	writeErr := func() error {
		if _, err := file.Write(content); err != nil {
			return err
		}
		return file.Sync()
	}()
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(target)
		return fmt.Errorf("write approval.json: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(target)
		return fmt.Errorf("close approval.json: %w", closeErr)
	}
	return nil
}

func validateApprovalRequest(request ApprovalRequest) error {
	if !request.Attested {
		return errors.New("explicit review attestation is required")
	}
	if err := validateApprovalLabel("reviewer", request.Reviewer, 100); err != nil {
		return err
	}
	if err := validateApprovalAudience(request.Audience); err != nil {
		return err
	}
	if request.ApprovedAt.IsZero() {
		return errors.New("approval time is required")
	}
	if len(request.Channels) == 0 {
		return errors.New("at least one approval channel is required")
	}
	seen := map[string]bool{}
	for _, channel := range request.Channels {
		if channel != strings.TrimSpace(channel) || !allowedApprovalChannels[channel] {
			return fmt.Errorf("unsupported approval channel %q", channel)
		}
		if seen[channel] {
			return fmt.Errorf("duplicate approval channel %q", channel)
		}
		seen[channel] = true
	}
	return nil
}

func validateApprovalLabel(name, value string, limit int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || len([]rune(trimmed)) > limit {
		return fmt.Errorf("%s must contain 1-%d characters", name, limit)
	}
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%s must not contain control characters", name)
		}
	}
	return nil
}

func validateApprovalAudience(value string) error {
	if err := validateApprovalLabel("audience", value, 120); err != nil {
		return err
	}
	if strings.ContainsAny(value, "@/\\") || strings.Contains(value, "://") {
		return errors.New("audience must be a non-sensitive label, not an address or path")
	}
	return nil
}

func validateApprovalManifest(manifest Manifest) error {
	if manifest.SchemaVersion != kitSchemaVersion || manifest.Tool != kitToolVersion || !safeIdentifier(manifest.EditionID) {
		return errors.New("manifest provenance does not match the distribution-kit contract")
	}
	if manifest.DistributionStatus != "review_pending" || !manifest.ReviewRequired || manifest.SendAuthorized || manifest.SocialPublishAuthorized {
		return errors.New("only an unchanged review-pending kit can be approved")
	}
	if len(manifest.Files) == 0 || manifest.Carousel.Width != cardWidth || manifest.Carousel.Height != cardHeight || manifest.Carousel.SlideCount != 6 {
		return errors.New("manifest content contract is incomplete")
	}
	if _, err := resolveTheme(manifest.Carousel.Theme); err != nil || strings.TrimSpace(manifest.Carousel.Theme) == "" {
		return errors.New("manifest carousel theme is unsupported")
	}
	if manifest.Carousel.CaptionPath != "carousel/caption.md" {
		return errors.New("manifest carousel caption contract is incomplete")
	}
	if strings.Join(manifest.Carousel.Formats, ",") != "png,svg" {
		return errors.New("manifest must contain matched PNG and SVG carousel assets")
	}
	return nil
}

func validateStoredApproval(approval Approval, manifest Manifest, manifestRaw []byte, requiredChannel string) error {
	if approval.SchemaVersion != approvalSchemaVersion || approval.Tool != kitToolVersion || approval.Scope != approvalScope || approval.Status != "approved" {
		return errors.New("approval.json does not match the content-release contract")
	}
	if approval.EditionID != manifest.EditionID || approval.ManifestFileCount != len(manifest.Files) {
		return errors.New("approval.json does not match the manifest edition")
	}
	digest := sha256.Sum256(manifestRaw)
	if approval.ManifestSHA256 != hex.EncodeToString(digest[:]) {
		return errors.New("approval.json manifest digest does not match")
	}
	if err := validateApprovalLabel("reviewer", approval.Reviewer, 100); err != nil {
		return fmt.Errorf("invalid stored approval: %w", err)
	}
	if err := validateApprovalAudience(approval.Audience); err != nil {
		return fmt.Errorf("invalid stored approval: %w", err)
	}
	approvedAt, err := time.Parse(time.RFC3339, approval.ApprovedAt)
	if err != nil || approvedAt.UTC().Format(time.RFC3339) != approval.ApprovedAt {
		return errors.New("approval.json must contain a canonical UTC approval time")
	}
	if strings.Join(approval.Attestations, "\n") != strings.Join(contentApprovalAttestations, "\n") {
		return errors.New("approval.json attestations do not match the content-release contract")
	}
	if approval.ProviderDeliveryReady || approval.SubscriberConsentReady || approval.AutomaticPublishReady {
		return errors.New("content approval must not claim provider, consent, or automatic-publish readiness")
	}
	foundRequired := false
	previous := ""
	for _, channel := range approval.Channels {
		if !allowedApprovalChannels[channel] || channel <= previous {
			return errors.New("approval.json channels must be unique, supported, and sorted")
		}
		previous = channel
		if channel == requiredChannel {
			foundRequired = true
		}
	}
	if !foundRequired {
		return fmt.Errorf("content is not approved for channel %q", requiredChannel)
	}
	return nil
}

func cleanKitRoot(kitDir string) (string, error) {
	kitDir = strings.TrimSpace(kitDir)
	if kitDir == "" {
		return "", errors.New("distribution-kit directory is required")
	}
	root, err := filepath.Abs(kitDir)
	if err != nil {
		return "", fmt.Errorf("resolve distribution-kit directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("inspect distribution-kit directory: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("distribution-kit path must be a directory")
	}
	return root, nil
}

func marshalCanonicalJSON(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}
