package main

import "testing"

func TestValidateBriefingAcceptsReviewGatedCitedDraft(t *testing.T) {
	metadata := datasetMeta{GeneratedAt: "2026-07-17T00:00:00Z"}
	briefing := validValidationBriefing(metadata.GeneratedAt)
	if err := validateBriefing(metadata, validationPublicationChanges{Status: "changed"}, briefing); err != nil {
		t.Fatalf("validateBriefing() error = %v", err)
	}
}

func TestValidateBriefingRejectsAutomaticSendAndBrokenArithmetic(t *testing.T) {
	metadata := datasetMeta{GeneratedAt: "2026-07-17T00:00:00Z"}
	briefing := validValidationBriefing(metadata.GeneratedAt)
	briefing.Email.SendPolicy = "automatic"
	if err := validateBriefing(metadata, validationPublicationChanges{Status: "changed"}, briefing); err == nil {
		t.Fatal("validateBriefing() accepted an automatic send policy")
	}
	briefing = validValidationBriefing(metadata.GeneratedAt)
	briefing.Signals[0].DeltaTradeUSD = 999
	if err := validateBriefing(metadata, validationPublicationChanges{Status: "changed"}, briefing); err == nil {
		t.Fatal("validateBriefing() accepted inconsistent signal arithmetic")
	}
}

func validValidationBriefing(generatedAt string) validationBriefing {
	current := validationBriefingObservedValue{USATradeUSD: 120, ChinaTradeUSD: 80, TotalTradeUSD: 200, ChinaShare: 0.4}
	previous := validationBriefingObservedValue{USATradeUSD: 80, ChinaTradeUSD: 80, TotalTradeUSD: 160, ChinaShare: 0.5}
	ratio := 0.25
	signals := []validationBriefingSignal{
		{ID: "monthly-total-kor", Kind: "reporter_total_change", Title: "Korea total increased", Summary: "Summary", ReporterISO3: "KOR", ReporterName: "Korea, Rep.", Period: "2026-05", PreviousPeriod: "2026-04", Current: current, Previous: previous, DeltaTradeUSD: 40, ChangeRatio: &ratio, ChinaShareDelta: -0.1, Evidence: []string{"./semiconductors/monthly/KOR.json", "./semiconductors/monthly/index.json"}, Interpretation: "Boundary", MeasurementLimit: "Limit"},
		{ID: "anchor-share-kor", Kind: "anchor_share_shift", Title: "Korea share shifted", Summary: "Summary", ReporterISO3: "KOR", ReporterName: "Korea, Rep.", Period: "2026-05", PreviousPeriod: "2026-04", Current: current, Previous: previous, DeltaTradeUSD: 40, ChangeRatio: &ratio, ChinaShareDelta: -0.1, Evidence: []string{"./semiconductors/monthly/KOR.json", "./semiconductors/monthly/index.json"}, Interpretation: "Boundary", MeasurementLimit: "Limit"},
		{ID: "product-kor-854232", Kind: "product_total_change", Title: "Korea memories increased", Summary: "Summary", ReporterISO3: "KOR", ReporterName: "Korea, Rep.", Classification: "H6", Code: "854232", Label: "Memories", Period: "2026-05", PreviousPeriod: "2026-04", Current: current, Previous: previous, DeltaTradeUSD: 40, ChangeRatio: &ratio, ChinaShareDelta: -0.1, Evidence: []string{"./semiconductors/monthly/KOR.json", "./semiconductors/reference.json"}, Interpretation: "Boundary", MeasurementLimit: "Limit"},
	}
	roles := []string{"cover", "scale", "anchor_balance", "product", "method", "cta"}
	slides := make([]validationBriefingCarouselSlide, 0, len(roles))
	for index, role := range roles {
		slides = append(slides, validationBriefingCarouselSlide{Order: index + 1, Role: role, Headline: "Headline", Body: "Body", Evidence: []string{"./semiconductors/monthly/index.json"}})
	}
	return validationBriefing{
		SchemaVersion: "1.0", GeneratedAt: generatedAt, EditionID: "semiconductor-pulse-2026-05-20260717T000000Z", Status: "ready",
		Title: "TradeGravity Semiconductor Pulse", Scope: "Scope", LatestPeriod: "2026-05", PreviousPeriod: "2026-04", PublicationStatus: "changed", ReviewRequired: true,
		Signals:        signals,
		Email:          validationBriefingEmail{Subject: "Subject", Preview: "Preview", Markdown: "[Evidence]({{BASE_URL}})", CTALabel: "Inspect", CTAPath: "./?tab=semiconductors", SendPolicy: "manual_review_required", PrimaryGoal: "Inspect evidence"},
		SocialCarousel: validationBriefingSocialCarousel{Format: "carousel_copy", AspectRatio: "4:5", ReviewPolicy: "manual_review_required", Slides: slides},
		Caveats:        []string{"One", "Two", "Three"}, EvidenceEntryPoint: "./?tab=semiconductors",
	}
}
