package main

import (
	"math"
	"testing"

	"tradegravity/internal/model"
)

func TestBuildLatestCalculatesGrowthAndShare(t *testing.T) {
	rows := []observationRow{
		{ReporterISO: "kor", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 100},
		{ReporterISO: "kor", PartnerISO: "USA", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 100},
		{ReporterISO: "kor", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2024", ValueUSD: 120},
		{ReporterISO: "kor", PartnerISO: "USA", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2024", ValueUSD: 80},
		{ReporterISO: "kor", PartnerISO: "CHN", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 40},
		{ReporterISO: "kor", PartnerISO: "CHN", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 100},
		{ReporterISO: "kor", PartnerISO: "CHN", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2024", ValueUSD: 50},
		{ReporterISO: "kor", PartnerISO: "CHN", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2024", ValueUSD: 150},
	}

	got := buildLatest(rows)
	if len(got) != 1 {
		t.Fatalf("buildLatest() returned %d rows, want 1", len(got))
	}

	entry := got[0]
	if entry.ISO3 != "KOR" {
		t.Fatalf("ISO3 = %q, want KOR", entry.ISO3)
	}
	if entry.USA.Trade != 200 || entry.CHN.Trade != 200 || entry.Total != 400 {
		t.Fatalf("unexpected trade totals: USA=%v CHN=%v total=%v", entry.USA.Trade, entry.CHN.Trade, entry.Total)
	}
	assertFloat(t, "share_cn", entry.ShareCN, 0.5)

	if entry.USA.PrevPeriod != "2023" || entry.USA.Growth == nil {
		t.Fatalf("USA growth metadata = %#v, prev=%q", entry.USA.Growth, entry.USA.PrevPeriod)
	}
	assertFloatPtr(t, "USA export growth", entry.USA.Growth.Export, 0.2)
	assertFloatPtr(t, "USA import growth", entry.USA.Growth.Import, -0.2)
	assertFloatPtr(t, "USA trade growth", entry.USA.Growth.Trade, 0)

	if entry.CHN.Growth == nil {
		t.Fatal("CHN growth is nil")
	}
	assertFloatPtr(t, "CHN trade growth", entry.CHN.Growth.Trade, 60.0/140.0)
}

func TestComparePeriodsUsesGranularityThenRecency(t *testing.T) {
	tests := []struct {
		name             string
		aType, bType     model.PeriodType
		aPeriod, bPeriod string
		want             int
	}{
		{name: "newer year", aType: model.PeriodYear, aPeriod: "2024", bType: model.PeriodYear, bPeriod: "2023", want: 1},
		{name: "older month", aType: model.PeriodMonth, aPeriod: "2024-01", bType: model.PeriodMonth, bPeriod: "2024-02", want: -1},
		{name: "month preferred to year", aType: model.PeriodMonth, aPeriod: "2023-01", bType: model.PeriodYear, bPeriod: "2024", want: 1},
		{name: "same quarter", aType: model.PeriodQuarter, aPeriod: "2024-Q2", bType: model.PeriodQuarter, bPeriod: "2024Q2", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := comparePeriods(tt.aType, tt.aPeriod, tt.bType, tt.bPeriod); got != tt.want {
				t.Fatalf("comparePeriods() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestGrowthForValueRejectsMissingOrZeroBaseline(t *testing.T) {
	if got := growthForValue(10, 0, true, true); got != nil {
		t.Fatalf("zero baseline returned %v, want nil", *got)
	}
	if got := growthForValue(10, 5, false, true); got != nil {
		t.Fatalf("missing current value returned %v, want nil", *got)
	}
}

func TestBuildMetaSummarizesCoverageAndPeriods(t *testing.T) {
	latest := []latestEntry{
		{
			ISO3: "JPN",
			USA:  partnerBlock{PeriodType: model.PeriodYear, Period: "2023"},
			CHN:  partnerBlock{PeriodType: model.PeriodYear, Period: "2023"},
		},
		{
			ISO3: "KOR",
			USA:  partnerBlock{PeriodType: model.PeriodYear, Period: "2021"},
		},
	}
	observations := []observationRow{{}, {}, {}, {}}

	got := buildMeta("2026-07-15T00:00:00Z", " WITS ", []string{"USA", "CHN"}, observations, latest)
	if got.SchemaVersion != schemaVersion || got.Provider != "wits" {
		t.Fatalf("schema/provider = %q/%q", got.SchemaVersion, got.Provider)
	}
	if got.ReporterCount != 2 || got.ObservationCount != 4 {
		t.Fatalf("reporter/observation counts = %d/%d", got.ReporterCount, got.ObservationCount)
	}
	if got.ExpectedPartnerBlocks != 4 || got.AvailablePartnerBlocks != 3 || got.MissingPartnerBlocks != 1 {
		t.Fatalf("coverage = expected %d available %d missing %d", got.ExpectedPartnerBlocks, got.AvailablePartnerBlocks, got.MissingPartnerBlocks)
	}
	if got.PeriodCounts["Y:2023"] != 2 || got.PeriodCounts["Y:2021"] != 1 {
		t.Fatalf("period counts = %#v", got.PeriodCounts)
	}
}

func TestBuildLatestSortsReporters(t *testing.T) {
	rows := []observationRow{
		{ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2024", ValueUSD: 1},
		{ReporterISO: "JPN", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2024", ValueUSD: 1},
	}

	got := buildLatest(rows)
	if len(got) != 2 || got[0].ISO3 != "JPN" || got[1].ISO3 != "KOR" {
		t.Fatalf("reporter order = %#v, want JPN then KOR", got)
	}
}

func TestEnsureRequiredPartnersRejectsDuplicatesAndUnsupportedPartners(t *testing.T) {
	if err := ensureRequiredPartners([]string{"USA", "CHN"}, []string{"USA", "CHN"}); err != nil {
		t.Fatalf("valid partners rejected: %v", err)
	}
	for _, partners := range [][]string{
		{"USA"},
		{"USA", "CHN", "CAN"},
		{"USA", "CHN", "USA"},
	} {
		if err := ensureRequiredPartners(partners, []string{"USA", "CHN"}); err == nil {
			t.Fatalf("ensureRequiredPartners(%v) returned nil error", partners)
		}
	}
}

func assertFloatPtr(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s is nil, want %v", name, want)
	}
	assertFloat(t, name, *got, want)
}

func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
