package main

import (
	"strings"
	"testing"
)

func TestBuildBriefingCreatesDeterministicEmailAndCarouselDrafts(t *testing.T) {
	generatedAt := "2026-07-17T00:00:00Z"
	index := semiconductorMonthlyIndexFile{Periods: []string{"2026-04", "2026-05"}, Reporters: []string{"JPN", "KOR"}}
	files := map[string]semiconductorMonthlyFile{
		"KOR.json": briefingMonthlyFile("KOR", []semiconductorMonthlyProductEntry{
			briefingMonthlyRow("2026-04", "854232", "Memories", 100, 100),
			briefingMonthlyRow("2026-04", "848620", "Semiconductor manufacturing machinery", 50, 50),
			briefingMonthlyRow("2026-05", "854232", "Memories", 180, 120),
			briefingMonthlyRow("2026-05", "848620", "Semiconductor manufacturing machinery", 55, 45),
		}),
		"JPN.json": briefingMonthlyFile("JPN", []semiconductorMonthlyProductEntry{
			briefingMonthlyRow("2026-04", "854232", "Memories", 80, 120),
			briefingMonthlyRow("2026-05", "854232", "Memories", 120, 80),
		}),
	}
	latest := []latestEntry{{ISO3: "KOR", Name: "Korea, Rep."}, {ISO3: "JPN", Name: "Japan"}}
	changes := publicationChangesFile{Status: "changed"}

	got := buildBriefing(generatedAt, latest, index, files, changes)
	if got.Status != "ready" || got.SchemaVersion != "1.0" || got.EditionID != "semiconductor-pulse-2026-05-20260717T000000Z" {
		t.Fatalf("unexpected briefing identity: %+v", got)
	}
	if !got.ReviewRequired || got.PublicationStatus != "changed" || got.LatestPeriod != "2026-05" || got.PreviousPeriod != "2026-04" {
		t.Fatalf("unexpected briefing provenance: %+v", got)
	}
	if len(got.Signals) != 3 {
		t.Fatalf("signals = %d, want 3", len(got.Signals))
	}
	if got.Signals[0].Kind != "reporter_total_change" || got.Signals[0].ReporterISO3 != "KOR" || got.Signals[0].DeltaTradeUSD != 100 {
		t.Fatalf("unexpected scale signal: %+v", got.Signals[0])
	}
	if got.Signals[1].Kind != "anchor_share_shift" || got.Signals[1].ReporterISO3 != "JPN" {
		t.Fatalf("unexpected anchor signal: %+v", got.Signals[1])
	}
	assertFloat(t, "anchor share shift", got.Signals[1].ChinaShareDelta, -0.2)
	if got.Signals[2].Kind != "product_total_change" || got.Signals[2].ReporterISO3 != "KOR" || got.Signals[2].Code != "854232" || got.Signals[2].DeltaTradeUSD != 100 {
		t.Fatalf("unexpected product signal: %+v", got.Signals[2])
	}
	if got.Email.SendPolicy != "manual_review_required" || !strings.Contains(got.Email.Markdown, "{{BASE_URL}}") || !strings.Contains(got.Email.Markdown, got.Signals[0].Title) {
		t.Fatalf("email draft does not preserve review and evidence handoff: %+v", got.Email)
	}
	if got.SocialCarousel.AspectRatio != "4:5" || got.SocialCarousel.ReviewPolicy != "manual_review_required" || len(got.SocialCarousel.Slides) != 6 {
		t.Fatalf("unexpected carousel draft: %+v", got.SocialCarousel)
	}
	for index, slide := range got.SocialCarousel.Slides {
		if slide.Order != index+1 || len(slide.Evidence) == 0 {
			t.Fatalf("slide %d is not ordered or cited: %+v", index, slide)
		}
	}
}

func TestBuildBriefingFailsClosedWithoutTwoComparableMonths(t *testing.T) {
	index := semiconductorMonthlyIndexFile{Periods: []string{"2026-05"}, Reporters: []string{"KOR"}}
	files := map[string]semiconductorMonthlyFile{
		"KOR.json": briefingMonthlyFile("KOR", []semiconductorMonthlyProductEntry{briefingMonthlyRow("2026-05", "854232", "Memories", 100, 100)}),
	}
	got := buildBriefing("2026-07-17T00:00:00Z", []latestEntry{{ISO3: "KOR", Name: "Korea, Rep."}}, index, files, publicationChangesFile{Status: "baseline"})
	if got.Status != "unavailable" || len(got.Signals) != 0 || len(got.SocialCarousel.Slides) != 0 {
		t.Fatalf("briefing should fail closed: %+v", got)
	}
	if !strings.Contains(got.Email.Markdown, "not available") || got.Email.SendPolicy != "manual_review_required" {
		t.Fatalf("unavailable email state is not explicit: %+v", got.Email)
	}
}

func TestBuildBriefingUsesOnePublicationWindowAcrossReporters(t *testing.T) {
	index := semiconductorMonthlyIndexFile{Periods: []string{"2026-03", "2026-04", "2026-05"}, Reporters: []string{"JPN", "KOR"}}
	files := map[string]semiconductorMonthlyFile{
		"JPN.json": briefingMonthlyFile("JPN", []semiconductorMonthlyProductEntry{
			briefingMonthlyRow("2026-04", "854232", "Memories", 100, 100),
			briefingMonthlyRow("2026-05", "854232", "Memories", 120, 80),
		}),
		"KOR.json": {
			ReporterISO3: "KOR",
			Periods:      []string{"2026-03", "2026-04"},
			Rows: []semiconductorMonthlyProductEntry{
				briefingMonthlyRow("2026-03", "854232", "Memories", 100, 100),
				briefingMonthlyRow("2026-04", "854232", "Memories", 1000, 0),
			},
		},
	}
	latest := []latestEntry{{ISO3: "JPN", Name: "Japan"}, {ISO3: "KOR", Name: "Korea, Rep."}}

	got := buildBriefing("2026-07-17T00:00:00Z", latest, index, files, publicationChangesFile{Status: "changed"})
	if got.Status != "ready" || got.LatestPeriod != "2026-05" || got.PreviousPeriod != "2026-04" {
		t.Fatalf("unexpected common publication window: %+v", got)
	}
	for _, signal := range got.Signals {
		if signal.ReporterISO3 != "JPN" || signal.Period != "2026-05" || signal.PreviousPeriod != "2026-04" {
			t.Fatalf("signal escaped the common publication window: %+v", signal)
		}
	}
}

func briefingMonthlyFile(reporter string, rows []semiconductorMonthlyProductEntry) semiconductorMonthlyFile {
	return semiconductorMonthlyFile{ReporterISO3: reporter, Periods: []string{"2026-04", "2026-05"}, Rows: rows}
}

func briefingMonthlyRow(period, code, label string, usa, china float64) semiconductorMonthlyProductEntry {
	total := usa + china
	share := 0.0
	if total > 0 {
		share = china / total
	}
	return semiconductorMonthlyProductEntry{
		Period: period, Classification: "H6", Code: code, Label: label,
		USA: seriesBlock{Available: true, Trade: usa}, CHN: seriesBlock{Available: true, Trade: china}, Total: total, ShareCN: share,
	}
}
