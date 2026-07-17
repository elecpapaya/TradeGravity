package distributionkit

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApproveBindsReviewedContentWithoutClaimingDeliveryReadiness(t *testing.T) {
	kitDir := buildWrittenKit(t)
	request := ApprovalRequest{
		Reviewer:   "maintainer-handle",
		Audience:   "consented-internal-pilot",
		Channels:   []string{"instagram", "email"},
		ApprovedAt: time.Date(2026, 7, 17, 12, 34, 56, 0, time.FixedZone("KST", 9*60*60)),
		Attested:   true,
	}
	approval, raw, err := Approve(kitDir, request)
	if err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if approval.Scope != "content_release" || approval.Status != "approved" || strings.Join(approval.Channels, ",") != "email,instagram" {
		t.Fatalf("unexpected approval contract: %+v", approval)
	}
	if approval.ProviderDeliveryReady || approval.SubscriberConsentReady || approval.AutomaticPublishReady {
		t.Fatalf("content approval overclaims delivery readiness: %+v", approval)
	}
	if approval.ApprovedAt != "2026-07-17T03:34:56Z" || len(approval.ManifestSHA256) != 64 || approval.ManifestFileCount != 20 {
		t.Fatalf("approval provenance is incomplete: %+v", approval)
	}
	if len(approval.Attestations) != 3 || !bytes.HasSuffix(raw, []byte("\n")) {
		t.Fatalf("approval attestation serialization is incomplete: %s", raw)
	}
	if err := WriteApproval(kitDir, raw); err != nil {
		t.Fatalf("WriteApproval() error = %v", err)
	}
	verified, verifiedManifest, err := VerifyApproved(kitDir, "email")
	if err != nil {
		t.Fatalf("VerifyApproved() error = %v", err)
	}
	if verified.ManifestSHA256 != approval.ManifestSHA256 || verifiedManifest.EditionID != approval.EditionID {
		t.Fatal("VerifyApproved() did not preserve the approval-manifest binding")
	}
	if _, _, err := VerifyApproved(kitDir, "instagram"); err != nil {
		t.Fatalf("VerifyApproved() rejected an approved channel: %v", err)
	}
	if err := WriteApproval(kitDir, raw); err == nil {
		t.Fatal("WriteApproval() overwrote an existing approval")
	}
	stored, err := os.ReadFile(filepath.Join(kitDir, "approval.json"))
	if err != nil || !bytes.Equal(stored, raw) {
		t.Fatalf("stored approval mismatch: err=%v", err)
	}
}

func TestVerifyApprovedRejectsUnapprovedChannelAndManifestChange(t *testing.T) {
	kitDir := buildWrittenKit(t)
	approval, raw, err := Approve(kitDir, validApprovalRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteApproval(kitDir, raw); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyApproved(kitDir, "instagram"); err == nil || !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("VerifyApproved() unapproved-channel error = %v", err)
	}

	manifestPath := filepath.Join(kitDir, "manifest.json")
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.PrimaryGoal = "modified after approval"
	changed, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(manifestPath, append(changed, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyApproved(kitDir, approval.Channels[0]); err == nil || !strings.Contains(err.Error(), "digest does not match") {
		t.Fatalf("VerifyApproved() changed-manifest error = %v", err)
	}
}

func TestApproveRejectsTamperedMissingAndUntrackedFiles(t *testing.T) {
	t.Run("tampered", func(t *testing.T) {
		kitDir := buildWrittenKit(t)
		path := filepath.Join(kitDir, "email", "body.html")
		if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Approve(kitDir, validApprovalRequest()); err == nil || !strings.Contains(err.Error(), "size changed") {
			t.Fatalf("Approve() tamper error = %v", err)
		}
	})

	t.Run("caption-tampered", func(t *testing.T) {
		kitDir := buildWrittenKit(t)
		path := filepath.Join(kitDir, "carousel", "caption.md")
		if err := os.WriteFile(path, []byte("uncited replacement caption\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Approve(kitDir, validApprovalRequest()); err == nil || (!strings.Contains(err.Error(), "size changed") && !strings.Contains(err.Error(), "digest changed")) {
			t.Fatalf("Approve() caption tamper error = %v", err)
		}
	})

	t.Run("missing", func(t *testing.T) {
		kitDir := buildWrittenKit(t)
		if err := os.Remove(filepath.Join(kitDir, "carousel", "slide-06.png")); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Approve(kitDir, validApprovalRequest()); err == nil || !strings.Contains(err.Error(), "missing files") {
			t.Fatalf("Approve() missing-file error = %v", err)
		}
	})

	t.Run("untracked", func(t *testing.T) {
		kitDir := buildWrittenKit(t)
		if err := os.WriteFile(filepath.Join(kitDir, "recipient-list.csv"), []byte("must not be stored here"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, _, err := Approve(kitDir, validApprovalRequest()); err == nil || !strings.Contains(err.Error(), "untracked file") {
			t.Fatalf("Approve() untracked-file error = %v", err)
		}
	})
}

func TestApproveRejectsMissingAttestationAndUnsafeManifest(t *testing.T) {
	kitDir := buildWrittenKit(t)
	request := validApprovalRequest()
	request.Attested = false
	if _, _, err := Approve(kitDir, request); err == nil || !strings.Contains(err.Error(), "attestation") {
		t.Fatalf("Approve() attestation error = %v", err)
	}
	request = validApprovalRequest()
	request.Audience = "reader@example.invalid"
	if _, _, err := Approve(kitDir, request); err == nil || !strings.Contains(err.Error(), "non-sensitive label") {
		t.Fatalf("Approve() sensitive-audience error = %v", err)
	}

	manifestPath := filepath.Join(kitDir, "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.SendAuthorized = true
	unsafeRaw, _ := marshalCanonicalJSON(manifest)
	if err := os.WriteFile(manifestPath, unsafeRaw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Approve(kitDir, validApprovalRequest()); err == nil || !strings.Contains(err.Error(), "review-pending") {
		t.Fatalf("Approve() unsafe-manifest error = %v", err)
	}
}

func buildWrittenKit(t *testing.T) string {
	t.Helper()
	bundle, err := Build(readSampleBriefing(t), "https://example.org/TradeGravity/")
	if err != nil {
		t.Fatal(err)
	}
	kitDir := filepath.Join(t.TempDir(), "kit")
	if err := Write(kitDir, bundle); err != nil {
		t.Fatal(err)
	}
	return kitDir
}

func validApprovalRequest() ApprovalRequest {
	return ApprovalRequest{
		Reviewer:   "reviewer",
		Audience:   "internal-pilot",
		Channels:   []string{"email"},
		ApprovedAt: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
		Attested:   true,
	}
}
