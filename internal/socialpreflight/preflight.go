package socialpreflight

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"tradegravity/internal/distributionkit"
)

const (
	schemaVersion       = "1.0"
	toolVersion         = "tradegravity-instagram-preflight/1.0"
	maximumTextFileSize = 64 << 10
	maximumPNGFileSize  = 20 << 20
	captionRuneLimit    = 1800
)

var hashtagPattern = regexp.MustCompile(`^#[A-Za-z0-9]+$`)

type Plan struct {
	SchemaVersion              string `json:"schema_version"`
	Tool                       string `json:"tool"`
	Status                     string `json:"status"`
	Channel                    string `json:"channel"`
	EditionID                  string `json:"edition_id"`
	Theme                      string `json:"theme"`
	GeneratedAt                string `json:"generated_at"`
	ManifestSHA256             string `json:"manifest_sha256"`
	ApprovalSHA256             string `json:"approval_sha256"`
	SlideCount                 int    `json:"slide_count"`
	Width                      int    `json:"width"`
	Height                     int    `json:"height"`
	CaptionRunes               int    `json:"caption_runes"`
	HashtagCount               int    `json:"hashtag_count"`
	AltTextSections            int    `json:"alt_text_sections"`
	Checks                     Checks `json:"checks"`
	ContainsCaptionText        bool   `json:"contains_caption_text"`
	ContainsCredentials        bool   `json:"contains_credentials"`
	ManualUploadRequired       bool   `json:"manual_upload_required"`
	AutomaticPublishAuthorized bool   `json:"automatic_publish_authorized"`
}

type Checks struct {
	ContentApproved         bool `json:"content_approved"`
	ManifestIntegrity       bool `json:"manifest_integrity"`
	PNGDimensions           bool `json:"png_dimensions"`
	CaptionEvidenceAndScope bool `json:"caption_evidence_and_scope"`
	AltTextComplete         bool `json:"alt_text_complete"`
}

type Result struct {
	Plan Plan
	JSON []byte
}

func Build(kitDir string, generatedAt time.Time) (Result, error) {
	if generatedAt.IsZero() {
		return Result{}, errors.New("Instagram preflight generation time is required")
	}
	root, err := canonicalDirectory(kitDir)
	if err != nil {
		return Result{}, err
	}
	approval, manifest, err := distributionkit.VerifyApproved(root, "instagram")
	if err != nil {
		return Result{}, fmt.Errorf("verify approved Instagram kit: %w", err)
	}
	caption, err := readBoundedRegular(filepath.Join(root, filepath.FromSlash(manifest.Carousel.CaptionPath)), "caption")
	if err != nil {
		return Result{}, err
	}
	captionRunes, hashtags, err := validateCaption(caption, manifest.BaseURL)
	if err != nil {
		return Result{}, err
	}
	altText, err := readBoundedRegular(filepath.Join(root, "carousel", "alt-text.md"), "alt text")
	if err != nil {
		return Result{}, err
	}
	sections, err := validateAltText(altText, manifest.Carousel.SlideCount)
	if err != nil {
		return Result{}, err
	}
	for index := 1; index <= manifest.Carousel.SlideCount; index++ {
		path := filepath.Join(root, "carousel", fmt.Sprintf("slide-%02d.png", index))
		raw, readErr := readBoundedRegular(path, "carousel PNG")
		if readErr != nil {
			return Result{}, readErr
		}
		configuration, configErr := png.DecodeConfig(bytes.NewReader(raw))
		if configErr != nil {
			return Result{}, fmt.Errorf("inspect carousel PNG %d: %w", index, configErr)
		}
		if configuration.Width != manifest.Carousel.Width || configuration.Height != manifest.Carousel.Height {
			return Result{}, fmt.Errorf("carousel PNG %d dimensions do not match the manifest", index)
		}
		image, decodeErr := png.Decode(bytes.NewReader(raw))
		if decodeErr != nil {
			return Result{}, fmt.Errorf("decode carousel PNG %d: %w", index, decodeErr)
		}
		if image.Bounds().Dx() != configuration.Width || image.Bounds().Dy() != configuration.Height {
			return Result{}, fmt.Errorf("carousel PNG %d dimensions do not match the manifest", index)
		}
	}
	approvalRaw, err := readBoundedRegular(filepath.Join(root, "approval.json"), "approval")
	if err != nil {
		return Result{}, err
	}
	approvalDigest := sha256.Sum256(approvalRaw)
	plan := Plan{
		SchemaVersion: schemaVersion, Tool: toolVersion, Status: "ready_for_manual_preview", Channel: "instagram",
		EditionID: manifest.EditionID, Theme: manifest.Carousel.Theme, GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		ManifestSHA256: approval.ManifestSHA256, ApprovalSHA256: hex.EncodeToString(approvalDigest[:]),
		SlideCount: manifest.Carousel.SlideCount, Width: manifest.Carousel.Width, Height: manifest.Carousel.Height,
		CaptionRunes: captionRunes, HashtagCount: hashtags, AltTextSections: sections,
		Checks:              Checks{ContentApproved: true, ManifestIntegrity: true, PNGDimensions: true, CaptionEvidenceAndScope: true, AltTextComplete: true},
		ContainsCaptionText: false, ContainsCredentials: false, ManualUploadRequired: true, AutomaticPublishAuthorized: false,
	}
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("encode Instagram preflight: %w", err)
	}
	return Result{Plan: plan, JSON: append(raw, '\n')}, nil
}

func validateCaption(raw []byte, baseURL string) (int, int, error) {
	if !utf8.Valid(raw) {
		return 0, 0, errors.New("Instagram caption is not valid UTF-8")
	}
	value := string(raw)
	runes := len([]rune(value))
	if runes == 0 || runes > captionRuneLimit {
		return 0, 0, fmt.Errorf("Instagram caption must contain 1-%d runes", captionRuneLimit)
	}
	if strings.Contains(value, "{{") || strings.ContainsRune(value, '\x00') {
		return 0, 0, errors.New("Instagram caption contains an unresolved or invalid value")
	}
	if !strings.Contains(value, strings.TrimSpace(baseURL)) || !strings.Contains(value, "Scope note:") || !strings.Contains(value, "not a physical shipment route") {
		return 0, 0, errors.New("Instagram caption must retain the evidence URL and scope note")
	}
	seen := map[string]bool{}
	count := 0
	for _, field := range strings.Fields(value) {
		if !strings.HasPrefix(field, "#") {
			continue
		}
		if !hashtagPattern.MatchString(field) {
			return 0, 0, errors.New("Instagram caption contains an invalid hashtag")
		}
		key := strings.ToLower(field)
		if seen[key] {
			return 0, 0, errors.New("Instagram caption contains a duplicate hashtag")
		}
		seen[key] = true
		count++
	}
	if count < 1 || count > 8 {
		return 0, 0, errors.New("Instagram caption must contain 1-8 restrained hashtags")
	}
	return runes, count, nil
}

func validateAltText(raw []byte, want int) (int, error) {
	if !utf8.Valid(raw) || strings.Contains(string(raw), "{{") {
		return 0, errors.New("carousel alt text is invalid")
	}
	sections := strings.Count(string(raw), "\n## Slide ")
	evidence := strings.Count(string(raw), "\nEvidence:\n")
	if sections != want || evidence != want {
		return 0, fmt.Errorf("carousel alt text must contain %d complete slide sections", want)
	}
	return sections, nil
}

func readBoundedRegular(path, label string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s must be a regular file", label)
	}
	if (filepath.Ext(path) == ".png" && info.Size() > maximumPNGFileSize) || (filepath.Ext(path) != ".png" && info.Size() > maximumTextFileSize) {
		return nil, fmt.Errorf("%s exceeds the safety limit", label)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", label, err)
	}
	return raw, nil
}

func Write(path, kitDir string, raw []byte) error {
	target, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil || strings.TrimSpace(path) == "" {
		return errors.New("Instagram preflight output path is required")
	}
	root, err := canonicalDirectory(kitDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(root, target)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("Instagram preflight output must remain outside the distribution kit")
	}
	if filepath.Ext(target) != ".json" {
		return errors.New("Instagram preflight output must use a .json extension")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return errors.New("Instagram preflight output already exists")
		}
		return err
	}
	if _, err = file.Write(raw); err != nil {
		_ = file.Close()
		_ = os.Remove(target)
		return err
	}
	if err = file.Close(); err != nil {
		_ = os.Remove(target)
		return err
	}
	return nil
}

func canonicalDirectory(value string) (string, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(value))
	if err != nil || strings.TrimSpace(value) == "" {
		return "", errors.New("distribution-kit directory is required")
	}
	info, err := os.Stat(absolute)
	if err != nil || !info.IsDir() {
		return "", errors.New("distribution-kit directory is unavailable")
	}
	return absolute, nil
}
