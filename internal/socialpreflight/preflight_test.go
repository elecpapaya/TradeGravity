package socialpreflight

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradegravity/internal/distributionkit"
)

func TestBuildValidatesApprovedManualInstagramPackageWithoutContentLeak(t *testing.T) {
	kit := approvedKit(t, []string{"instagram"})
	result, err := Build(kit, time.Date(2026, 7, 17, 14, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	plan := result.Plan
	if plan.Status != "ready_for_manual_preview" || plan.Channel != "instagram" || plan.Theme != distributionkit.ThemeEditorialLight || plan.SlideCount != 6 || plan.Width != 1080 || plan.Height != 1350 {
		t.Fatalf("unexpected plan identity: %+v", plan)
	}
	if !plan.Checks.ContentApproved || !plan.Checks.ManifestIntegrity || !plan.Checks.PNGDimensions || !plan.Checks.CaptionEvidenceAndScope || !plan.Checks.AltTextComplete {
		t.Fatalf("preflight checks are incomplete: %+v", plan.Checks)
	}
	if plan.CaptionRunes < 100 || plan.CaptionRunes > captionRuneLimit || plan.HashtagCount != 4 || plan.AltTextSections != 6 {
		t.Fatalf("unexpected content aggregates: %+v", plan)
	}
	if !plan.ManualUploadRequired || plan.AutomaticPublishAuthorized || plan.ContainsCaptionText || plan.ContainsCredentials {
		t.Fatalf("unsafe publication flags: %+v", plan)
	}
	if bytes.Contains(result.JSON, []byte("Korea")) || bytes.Contains(result.JSON, []byte("example.org")) || bytes.Contains(result.JSON, []byte("#TradeGravity")) {
		t.Fatal("aggregate preflight leaked caption text, evidence URL, or hashtags")
	}
	var decoded Plan
	if err := json.Unmarshal(result.JSON, &decoded); err != nil || decoded.EditionID != plan.EditionID {
		t.Fatalf("preflight JSON is invalid: %v", err)
	}
}

func TestBuildRequiresInstagramApprovalAndRejectsTampering(t *testing.T) {
	if _, err := Build(approvedKit(t, []string{"email"}), time.Now()); err == nil || !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("email-only approval error = %v", err)
	}
	kit := approvedKit(t, []string{"instagram"})
	caption := filepath.Join(kit, "carousel", "caption.md")
	if err := os.WriteFile(caption, []byte("replacement\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Build(kit, time.Now()); err == nil || (!strings.Contains(err.Error(), "size changed") && !strings.Contains(err.Error(), "digest changed")) {
		t.Fatalf("tampered caption error = %v", err)
	}
}

func TestCaptionAndAltTextContractsFailClosed(t *testing.T) {
	valid := []byte("Title\n\nEvidence https://example.org/TradeGravity/\nScope note: descriptive evidence; not a physical shipment route.\n#TradeGravity\n")
	if _, count, err := validateCaption(valid, "https://example.org/TradeGravity/"); err != nil || count != 1 {
		t.Fatalf("valid caption = count %d err %v", count, err)
	}
	for name, raw := range map[string][]byte{
		"missing evidence": []byte("Scope note: not a physical shipment route. #TradeGravity"),
		"missing scope":    []byte("https://example.org/TradeGravity/ #TradeGravity"),
		"duplicate tag":    []byte("https://example.org/TradeGravity/ Scope note: not a physical shipment route. #TradeGravity #tradegravity"),
		"placeholder":      []byte("https://example.org/TradeGravity/ Scope note: not a physical shipment route. {{value}} #TradeGravity"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, _, err := validateCaption(raw, "https://example.org/TradeGravity/"); err == nil {
				t.Fatal("invalid caption was accepted")
			}
		})
	}
	if _, err := validateAltText([]byte("# Carousel alt text\n\n## Slide 1\n\nEvidence:\n- source\n"), 6); err == nil {
		t.Fatal("incomplete alt text was accepted")
	}
}

func TestWriteRefusesKitPathAndOverwrite(t *testing.T) {
	kit := approvedKit(t, []string{"instagram"})
	if err := Write(filepath.Join(kit, "preflight.json"), kit, []byte("{}\n")); err == nil {
		t.Fatal("Write() placed preflight inside the approved kit")
	}
	out := filepath.Join(t.TempDir(), "instagram-preflight.json")
	if err := Write(out, kit, []byte("{}\n")); err != nil {
		t.Fatal(err)
	}
	if err := Write(out, kit, []byte("changed\n")); err == nil {
		t.Fatal("Write() overwrote an existing preflight")
	}
}

func approvedKit(t *testing.T, channels []string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "sample-data", "briefing.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := distributionkit.BuildWithOptions(raw, "https://example.org/TradeGravity/", distributionkit.BuildOptions{Theme: distributionkit.ThemeEditorialLight})
	if err != nil {
		t.Fatal(err)
	}
	kit := filepath.Join(t.TempDir(), "kit")
	if err := distributionkit.Write(kit, bundle); err != nil {
		t.Fatal(err)
	}
	_, approvalRaw, err := distributionkit.Approve(kit, distributionkit.ApprovalRequest{Reviewer: "reviewer", Audience: "social-pilot", Channels: channels, ApprovedAt: time.Date(2026, 7, 17, 13, 0, 0, 0, time.UTC), Attested: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := distributionkit.WriteApproval(kit, approvalRaw); err != nil {
		t.Fatal(err)
	}
	return kit
}
