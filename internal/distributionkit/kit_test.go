package distributionkit

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildProducesDeterministicReviewGatedKit(t *testing.T) {
	raw := readSampleBriefing(t)
	first, err := Build(raw, "https://example.org/TradeGravity/")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	second, err := Build(raw, "https://example.org/TradeGravity/")
	if err != nil {
		t.Fatalf("second Build() error = %v", err)
	}
	if !bytes.Equal(first.Files["manifest.json"], second.Files["manifest.json"]) {
		t.Fatal("manifest is not deterministic")
	}
	if !bytes.Equal(first.Files["carousel/slide-01.png"], second.Files["carousel/slide-01.png"]) {
		t.Fatal("PNG rendering is not deterministic")
	}
	if first.Manifest.DistributionStatus != "review_pending" || first.Manifest.SendAuthorized || first.Manifest.SocialPublishAuthorized || !first.Manifest.ReviewRequired {
		t.Fatalf("unsafe manifest gates: %+v", first.Manifest)
	}
	if first.Manifest.Carousel.Width != 1080 || first.Manifest.Carousel.Height != 1350 || first.Manifest.Carousel.SlideCount != 6 || strings.Join(first.Manifest.Carousel.Formats, ",") != "png,svg" {
		t.Fatalf("unexpected social dimensions: %+v", first.Manifest.Carousel)
	}
	if first.Manifest.Carousel.Theme != ThemeIntelligenceDark {
		t.Fatalf("default carousel theme = %q", first.Manifest.Carousel.Theme)
	}
	if len(first.Files) != 21 || len(first.Manifest.Files) != 20 {
		t.Fatalf("file counts = %d/%d, want 21/20", len(first.Files), len(first.Manifest.Files))
	}
	if first.Manifest.Carousel.CaptionPath != "carousel/caption.md" {
		t.Fatalf("caption path = %q", first.Manifest.Carousel.CaptionPath)
	}

	htmlBody := string(first.Files["email/body.html"])
	if strings.Count(htmlBody, "<a href=") != 2 || strings.Count(htmlBody, "{{UNSUBSCRIBE_URL}}") != 1 || strings.Contains(htmlBody, "<script") || strings.Contains(htmlBody, "<img") {
		t.Fatalf("email HTML does not preserve the one-primary-CTA/unsubscribe/no-script contract: %s", htmlBody)
	}
	if !strings.Contains(htmlBody, "https://example.org/TradeGravity/?tab=semiconductors") {
		t.Fatal("email HTML does not resolve the evidence CTA")
	}
	if strings.Count(string(first.Files["email/body.md"]), "{{UNSUBSCRIBE_URL}}") != 1 {
		t.Fatal("email Markdown does not retain the required unsubscribe placeholder")
	}
	for index := 1; index <= 6; index++ {
		svgPath := filepath.ToSlash(filepath.Join("carousel", "slide-0"+string(rune('0'+index))+".svg"))
		svg := string(first.Files[svgPath])
		if !strings.Contains(svg, "width=\"1080\" height=\"1350\"") || !strings.Contains(svg, "Reviewed draft") || strings.Contains(svg, "<script") {
			t.Fatalf("invalid slide %d SVG", index)
		}
		if err := xml.Unmarshal(first.Files[svgPath], &struct{}{}); err != nil {
			t.Fatalf("slide %d is not valid XML: %v", index, err)
		}
		pngPath := filepath.ToSlash(filepath.Join("carousel", "slide-0"+string(rune('0'+index))+".png"))
		decoded, err := png.Decode(bytes.NewReader(first.Files[pngPath]))
		if err != nil {
			t.Fatalf("slide %d is not a valid PNG: %v", index, err)
		}
		if decoded.Bounds().Dx() != 1080 || decoded.Bounds().Dy() != 1350 {
			t.Fatalf("slide %d PNG dimensions = %v", index, decoded.Bounds())
		}
	}
	if !strings.Contains(string(first.Files["carousel/slide-02.svg"]), "DATA · semiconductors/monthly/KOR.json") || !strings.Contains(string(first.Files["carousel/slide-02.svg"]), "OPEN · example.org/TradeGravity") {
		t.Fatal("card evidence labels are not legible and traceable")
	}
	if !strings.Contains(string(first.Files["carousel/index.html"]), "slide-01.png") || strings.Contains(string(first.Files["carousel/index.html"]), "slide-01.svg") {
		t.Fatal("carousel review page does not preview the upload-ready PNG")
	}
	if !strings.Contains(string(first.Files["carousel/index.html"]), "caption.md") {
		t.Fatal("carousel review page does not link the caption draft")
	}
	if !strings.Contains(string(first.Files["REVIEW.md"]), "double opt-in") || !strings.Contains(string(first.Files["REVIEW.md"]), "carousel/caption.md") {
		t.Fatal("review checklist omits consent gate")
	}
	caption := string(first.Files["carousel/caption.md"])
	if !strings.Contains(caption, "2023-12 vs 2023-11") || !strings.Contains(caption, "https://example.org/TradeGravity/?tab=semiconductors") || !strings.Contains(caption, "not a physical shipment route") {
		t.Fatalf("caption omitted period, evidence, or scope note: %s", caption)
	}
	for _, signal := range []string{"Korea, Rep. selected chip trade increased", "Japan two-anchor balance shifted toward USA", "Korea, Rep. · Processors and controllers increased"} {
		if !strings.Contains(caption, signal) {
			t.Fatalf("caption omitted a validated signal %q: %s", signal, caption)
		}
	}
	if len([]rune(caption)) > captionRuneLimit || strings.Contains(caption, "{{") {
		t.Fatal("caption exceeded the editorial contract or retained a placeholder")
	}
}

func TestBuildEditorialThemeIsDeterministicDistinctAndReviewGated(t *testing.T) {
	raw := readSampleBriefing(t)
	options := BuildOptions{Theme: ThemeEditorialLight}
	first, err := BuildWithOptions(raw, "https://example.org/TradeGravity/", options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildWithOptions(raw, "https://example.org/TradeGravity/", options)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := Build(raw, "https://example.org/TradeGravity/")
	if err != nil {
		t.Fatal(err)
	}
	if first.Manifest.Carousel.Theme != ThemeEditorialLight || first.Manifest.SocialPublishAuthorized || !first.Manifest.ReviewRequired {
		t.Fatalf("unsafe or missing editorial theme manifest: %+v", first.Manifest)
	}
	if !bytes.Equal(first.Files["manifest.json"], second.Files["manifest.json"]) || !bytes.Equal(first.Files["carousel/slide-01.png"], second.Files["carousel/slide-01.png"]) {
		t.Fatal("editorial theme output is not deterministic")
	}
	if bytes.Equal(first.Files["carousel/slide-01.png"], baseline.Files["carousel/slide-01.png"]) || bytes.Equal(first.Files["carousel/slide-01.svg"], baseline.Files["carousel/slide-01.svg"]) {
		t.Fatal("editorial theme did not produce distinct review assets")
	}
	for index := 1; index <= 6; index++ {
		path := fmt.Sprintf("carousel/slide-%02d.png", index)
		decoded, err := png.Decode(bytes.NewReader(first.Files[path]))
		if err != nil || decoded.Bounds().Dx() != 1080 || decoded.Bounds().Dy() != 1350 {
			t.Fatalf("editorial asset %s is invalid: bounds=%v err=%v", path, decoded.Bounds(), err)
		}
	}
	kitDir := filepath.Join(t.TempDir(), "editorial-kit")
	if err := Write(kitDir, first); err != nil {
		t.Fatal(err)
	}
	_, approvalRaw, err := Approve(kitDir, ApprovalRequest{Reviewer: "reviewer", Audience: "editorial-pilot", Channels: []string{"instagram"}, ApprovedAt: time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC), Attested: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteApproval(kitDir, approvalRaw); err != nil {
		t.Fatal(err)
	}
	if _, _, err := VerifyApproved(kitDir, "instagram"); err != nil {
		t.Fatalf("editorial theme could not cross the existing approval seam: %v", err)
	}
}

func TestBuildRejectsUnknownTheme(t *testing.T) {
	if _, err := BuildWithOptions(readSampleBriefing(t), "https://example.org/", BuildOptions{Theme: "remote-html"}); err == nil {
		t.Fatal("BuildWithOptions() accepted an unknown renderer theme")
	}
}

func TestBuildRejectsAutomaticOrUnavailableBriefing(t *testing.T) {
	raw := readSampleBriefing(t)
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	value["review_required"] = false
	unsafe, _ := json.Marshal(value)
	if _, err := Build(unsafe, "https://example.org/"); err == nil {
		t.Fatal("Build() accepted a briefing without manual review")
	}
	value["review_required"] = true
	value["status"] = "unavailable"
	unavailable, _ := json.Marshal(value)
	if _, err := Build(unavailable, "https://example.org/"); err == nil {
		t.Fatal("Build() accepted an unavailable briefing")
	}
	if _, err := Build(raw, "http://example.org/"); err == nil {
		t.Fatal("Build() accepted an insecure public base URL")
	}
}

func TestWriteCreatesNewDirectoryAndRefusesOverwrite(t *testing.T) {
	bundle, err := Build(readSampleBriefing(t), "http://127.0.0.1:8080/")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "kit")
	if err := Write(out, bundle); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "carousel", "slide-01.png")); err != nil {
		t.Fatalf("written kit is incomplete: %v", err)
	}
	if err := Write(out, bundle); err == nil {
		t.Fatal("Write() overwrote an existing directory")
	}
}

func readSampleBriefing(t *testing.T) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "examples", "sample-data", "briefing.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sample briefing: %v", err)
	}
	return raw
}
