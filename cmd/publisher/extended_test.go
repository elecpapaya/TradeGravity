package main

import (
	"fmt"
	"testing"

	"tradegravity/internal/model"
	"tradegravity/internal/strategic"
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

func TestBuildDataCatalogSeparatesReadyAndPlannedResources(t *testing.T) {
	catalog := buildDataCatalog(
		"2026-01-01T00:00:00Z",
		"wits",
		"success",
		seriesFile{Rows: []reporterSeries{{ISO3: "KOR"}}},
		productIndexFile{Provider: "comtrade", Classification: "H6", Level: 2, Reporters: []string{"KOR"}},
		strategicIndexFile{Provider: "comtrade", Level: 6, Partitions: []strategicPartition{{ReporterISO3: "KOR", Period: "2023"}}},
		tariffIndexFile{Provider: "trains", Level: 6, Partitions: []tariffPartition{{ImporterISO3: "KOR", Year: "2023"}}},
		matrixIndexFile{Provider: "comtrade", ProductCode: "TOTAL", Partitions: []matrixPartition{{ReporterISO3: "KOR", Period: "2023"}}},
		mirrorIndexFile{Provider: "comtrade", Partitions: []mirrorPartition{{ReporterISO3: "KOR", Period: "2023"}}},
		semiconductorMonthlyIndexFile{Provider: "comtrade", Partitions: []semiconductorMonthlyPartition{{ReporterISO3: "KOR"}}},
		publicationChangesFile{Status: "changed"},
		briefingFile{Status: "ready"},
	)
	if catalog.SchemaVersion != "1.0" || len(catalog.Resources) < 10 {
		t.Fatalf("unexpected catalog shape: %+v", catalog)
	}
	byID := make(map[string]catalogResource)
	for _, resource := range catalog.Resources {
		byID[resource.ID] = resource
	}
	if byID["product_chapters"].Status != "ready" || byID["product_chapters"].ProductLevel != 2 || byID["product_chapters"].Partitioning == "" {
		t.Fatalf("product resource does not describe the current partition: %+v", byID["product_chapters"])
	}
	if byID["strategic_hs6"].Status != "ready" || byID["strategic_hs6"].Href != "./strategic-hs6/index.json" {
		t.Fatalf("strategic resource does not expose the partition index: %+v", byID["strategic_hs6"])
	}
	if byID["tariff_schedules"].Status != "ready" || byID["tariff_schedules"].Href != "./tariffs/index.json" || byID["scenario_runs"].Href != "" {
		t.Fatalf("tariff and planned resource status is wrong: %+v %+v", byID["tariff_schedules"], byID["scenario_runs"])
	}
	if byID["bilateral_matrix"].Status != "ready" || byID["bilateral_matrix"].Href != "./bilateral-matrix/index.json" || byID["bilateral_matrix"].Provider != "comtrade" {
		t.Fatalf("matrix resource is not published: %+v", byID["bilateral_matrix"])
	}
	if byID["mirror_reconciliation"].Status != "ready" || byID["mirror_reconciliation"].Href != "./mirror/index.json" {
		t.Fatalf("mirror diagnostics resource is not published: %+v", byID["mirror_reconciliation"])
	}
	if byID["publication_changes"].Status != "ready" || byID["publication_changes"].Href != "./changes.json" {
		t.Fatalf("publication change feed is not published: %+v", byID["publication_changes"])
	}
	if byID["distribution_briefing"].Status != "ready" || byID["distribution_briefing"].Href != "./briefing.json" {
		t.Fatalf("distribution briefing is not published: %+v", byID["distribution_briefing"])
	}
}

func TestBuildMirrorFilesComparesBothReportedDirectionsWithoutChoosingTruth(t *testing.T) {
	matrixFiles := map[string]matrixFile{
		"KOR/2023.json": {ReporterISO3: "KOR", Period: "2023", Rows: []matrixPartner{{PartnerISO3: "USA", ExportAvailable: true, ImportAvailable: true, ExportUSD: 100, ImportUSD: 80}}},
		"USA/2023.json": {ReporterISO3: "USA", Period: "2023", Rows: []matrixPartner{{PartnerISO3: "KOR", ExportAvailable: true, ImportAvailable: true, ExportUSD: 70, ImportUSD: 110}}},
	}
	index, files := buildMirrorFiles("2026-01-01T00:00:00Z", "comtrade", matrixFiles)
	if len(index.Partitions) != 1 || index.ComparisonCount != 2 || index.Partitions[0].Href != "./KOR/2023.json" {
		t.Fatalf("unexpected mirror index: %+v", index)
	}
	row := files["KOR/2023.json"].Rows[0]
	if row.AnchorISO3 != "USA" || row.ExportGapUSD == nil || *row.ExportGapUSD != -10 || row.ImportGapUSD == nil || *row.ImportGapUSD != 10 {
		t.Fatalf("unexpected mirror row: %+v", row)
	}
	if files["KOR/2023.json"].Scope == "" || len(files["KOR/2023.json"].Caveats) < 2 {
		t.Fatal("mirror diagnostics must disclose scope and caveats")
	}
}

func TestBuildMatrixFilesAggregatesPartnerFlows(t *testing.T) {
	rows := []observationRow{
		{Provider: "comtrade", ProductCode: "TOTAL", ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 60},
		{Provider: "comtrade", ProductCode: "TOTAL", ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 40},
		{Provider: "comtrade", ProductCode: "TOTAL", ReporterISO: "KOR", PartnerISO: "CHN", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 80},
		{Provider: "comtrade", ProductCode: "TOTAL", ReporterISO: "KOR", PartnerISO: "WLD", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 999},
	}
	index, files := buildMatrixFiles("2026-01-01T00:00:00Z", "comtrade", rows)
	if index.ObservationCount != 3 || index.PartnerRowCount != 2 || len(index.Partitions) != 1 || index.Partitions[0].Href != "./KOR/2023.json" {
		t.Fatalf("unexpected matrix index: %+v", index)
	}
	file := files["KOR/2023.json"]
	if len(file.Rows) != 2 || file.Rows[0].PartnerISO3 != "USA" || file.Rows[0].TradeUSD != 100 || file.Rows[0].BalanceUSD != 20 {
		t.Fatalf("unexpected matrix file: %+v", file)
	}
	if file.Rows[1].ExportAvailable || !file.Rows[1].ImportAvailable || file.Rows[1].BalanceUSD != -80 {
		t.Fatalf("matrix availability is wrong: %+v", file.Rows[1])
	}
}

func TestBuildTariffFilesPartitionsByImporterAndYear(t *testing.T) {
	registry := []strategic.Product{{Code: "854231", Sector: "semiconductors", Label: "Processors", RevisionNote: "HS 2007+"}}
	minRate := 2.0
	rows := []tariffObservationRow{
		{Provider: "trains", Classification: "HS2017", ProductCode: "854231", ProductLevel: 6, ImporterISO3: "KOR", ExporterISO3: "WLD", ExporterCode: "000", DataType: "ave_estimated", RateType: "mfn_applied", Regime: "mfn", Year: "2023", RatePercent: 4.5, MinRatePercent: &minRate, TotalLines: 3, MFNLines: 3, Nomenclature: "H5", SourceUpdatedAt: "2025-01-02T00:00:00Z"},
		{Provider: "trains", Classification: "HS2017", ProductCode: "999999", ProductLevel: 6, ImporterISO3: "KOR", ExporterISO3: "WLD", DataType: "reported", RateType: "mfn_applied", Regime: "mfn", Year: "2023", RatePercent: 1},
	}
	index, files := buildTariffFiles("2026-01-01T00:00:00Z", "trains", rows, registry)
	if index.ObservationCount != 1 || len(index.Partitions) != 1 || index.Partitions[0].Href != "./KOR/2023.json" {
		t.Fatalf("unexpected tariff index: %+v", index)
	}
	file, ok := files["KOR/2023.json"]
	if !ok || len(file.Rows) != 1 {
		t.Fatalf("unexpected tariff file: %+v", files)
	}
	row := file.Rows[0]
	if row.Code != "854231" || row.Sector != "semiconductors" || row.RatePercent != 4.5 || row.MinRatePercent == nil || *row.MinRatePercent != 2 {
		t.Fatalf("unexpected published tariff row: %+v", row)
	}
}

func TestBuildStrategicFilesPartitionsByReporterAndYear(t *testing.T) {
	registry := []strategic.Product{
		{Code: "854231", Sector: "semiconductors", Label: "Processors", RevisionNote: "HS 2007+"},
		{Code: "850760", Sector: "ev_batteries", Label: "Lithium-ion accumulators", RevisionNote: "compatible"},
	}
	rows := []observationRow{
		{Provider: "comtrade", Classification: "H6", ProductCode: "854231", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 60},
		{Provider: "comtrade", Classification: "H6", ProductCode: "854231", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 40},
		{Provider: "comtrade", Classification: "H6", ProductCode: "850760", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "CHN", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 100},
		{Provider: "comtrade", Classification: "H6", ProductCode: "999999", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "CHN", Flow: model.FlowImport, PeriodType: model.PeriodYear, Period: "2023", ValueUSD: 500},
	}
	index, files := buildStrategicFiles("2026-01-01T00:00:00Z", "comtrade", []string{"USA", "CHN"}, rows, registry)
	if index.ObservationCount != 3 || len(index.Partitions) != 1 || index.Partitions[0].Href != "./KOR/2023.json" {
		t.Fatalf("unexpected strategic index: %+v", index)
	}
	file := files["KOR/2023.json"]
	if len(file.Rows) != 2 || file.Rows[0].Sector != "ev_batteries" || file.Rows[1].Total != 100 {
		t.Fatalf("unexpected strategic partition: %+v", file)
	}
}
