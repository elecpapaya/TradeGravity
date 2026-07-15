package main

import (
	"fmt"
	"testing"

	"tradegravity/internal/model"
)

func TestBuildSeriesFileLimitsAnnualWindowAndMarksComparability(t *testing.T) {
	var rows []observationRow
	for year := 2013; year <= 2023; year++ {
		for _, partner := range []string{"USA", "CHN"} {
			if year == 2014 && partner == "CHN" {
				continue
			}
			for _, flow := range []model.Flow{model.FlowExport, model.FlowImport} {
				rows = append(rows, observationRow{Provider: "wits", ReporterISO: "KOR", PartnerISO: partner, Flow: flow, PeriodType: model.PeriodYear, Period: fmt.Sprint(year), ValueUSD: float64(year)})
			}
		}
	}
	series := buildSeriesFile("2026-01-01T00:00:00Z", "wits", []string{"USA", "CHN"}, rows, 10)
	if len(series.Rows) != 1 || len(series.Rows[0].Points) != 10 {
		t.Fatalf("unexpected series shape: %#v", series.Rows)
	}
	points := series.Rows[0].Points
	if points[0].Period != "2014" || points[len(points)-1].Period != "2023" {
		t.Fatalf("unexpected retained window: %s..%s", points[0].Period, points[len(points)-1].Period)
	}
	if points[0].Comparable || !points[1].Comparable {
		t.Fatalf("comparability flags do not reflect partner availability: %#v", points[:2])
	}
}

func TestBuildProductFilesAggregatesFlowsWithoutChangingProvider(t *testing.T) {
	rows := []observationRow{
		{Provider: "comtrade", Classification: "H6", ProductCode: "85", ProductLevel: 2, ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 60},
		{Provider: "comtrade", Classification: "H6", ProductCode: "85", ProductLevel: 2, ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 40},
		{Provider: "comtrade", Classification: "H6", ProductCode: "85", ProductLevel: 2, ReporterISO: "KOR", PartnerISO: "CHN", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 30},
		{Provider: "comtrade", Classification: "H6", ProductCode: "85", ProductLevel: 2, ReporterISO: "KOR", PartnerISO: "CHN", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 70},
	}
	index, files := buildProductFiles("2026-01-01T00:00:00Z", "comtrade", 2, []string{"USA", "CHN"}, rows, map[string]string{"85": "Electrical machinery"})
	file := files["KOR"]
	if index.Provider != "comtrade" || file.Provider != "comtrade" || len(file.Rows) != 1 {
		t.Fatalf("unexpected product provenance or rows: %+v %+v", index, file)
	}
	row := file.Rows[0]
	if row.USA.Trade != 100 || row.CHN.Trade != 100 || row.Total != 200 || row.ShareCN != 0.5 {
		t.Fatalf("unexpected product aggregation: %+v", row)
	}
}

func TestBuildQualityFileFlagsMixedAndStalePeriods(t *testing.T) {
	latest := []latestEntry{
		{ISO3: "KOR", SamePeriod: true, USA: partnerBlock{PeriodType: model.PeriodYear, Period: "2023"}, CHN: partnerBlock{PeriodType: model.PeriodYear, Period: "2023"}},
		{ISO3: "BGD", SamePeriod: false, USA: partnerBlock{PeriodType: model.PeriodYear, Period: "2015"}, CHN: partnerBlock{}},
	}
	quality := buildQualityFile("2026-01-01T00:00:00Z", "wits", latest, nil, nil, nil)
	if quality.DominantPeriod != "Y:2023" || quality.Summary.ComparableReporters != 1 || quality.Summary.IncomparableReporters != 1 || quality.Summary.MissingPartnerBlocks != 1 || quality.Summary.StalePartnerBlocks != 1 {
		t.Fatalf("unexpected quality summary: %+v", quality)
	}
	if len(quality.ReporterIssues) != 1 || quality.ReporterIssues[0].ISO3 != "BGD" {
		t.Fatalf("unexpected reporter issues: %+v", quality.ReporterIssues)
	}
}
