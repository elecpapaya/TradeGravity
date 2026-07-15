package main

import (
	"fmt"
	"path/filepath"
	"testing"

	"tradegravity/internal/model"
	"tradegravity/internal/semiconductor"
	"tradegravity/internal/strategic"
)

func TestBuildSemiconductorPublicationExposesCoverageGate(t *testing.T) {
	reference, err := semiconductor.Load(filepath.Join("..", "..", "configs", "semiconductor_reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	limited := buildSemiconductorPublication(reference, map[string]strategicFile{
		"ARG/2023.json": {
			ReporterISO3: "ARG",
			Period:       "2023",
			Rows: []strategicProductEntry{{
				Code: "854232", USA: seriesBlock{Available: true, Trade: 1}, CHN: seriesBlock{Available: true, Trade: 2},
			}},
		},
	})
	if limited.Status != "limited" || limited.ObservedReporterCount != 1 || limited.ObservedPeriodCount != 1 || limited.RegisteredCodeCount < semiconductorMinimumCodes {
		t.Fatalf("unexpected limited coverage: %+v", limited)
	}

	files := make(map[string]strategicFile)
	for reporter := 0; reporter < semiconductorMinimumReporters; reporter++ {
		for year := 2019; year < 2019+semiconductorMinimumPeriods; year++ {
			iso3 := fmt.Sprintf("X%02d", reporter)
			key := fmt.Sprintf("%s/%d.json", iso3, year)
			files[key] = strategicFile{
				ReporterISO3: iso3,
				Period:       fmt.Sprint(year),
				Rows: []strategicProductEntry{{
					Code: "854231", USA: seriesBlock{Available: true, Trade: 1},
				}},
			}
		}
	}
	ready := buildSemiconductorPublication(reference, files)
	if ready.Status != "research_ready" {
		t.Fatalf("coverage gate status = %q, want research_ready: %+v", ready.Status, ready)
	}
}

func TestBuildSemiconductorMonthlyFilesKeepsAnchorFlowsAndFiltersTheRegistry(t *testing.T) {
	reference := semiconductor.Reference{Stages: []semiconductor.Stage{{ID: "memory", Codes: []string{"854232"}}}}
	products := []strategic.Product{{Code: "854232", Label: "Memories"}}
	rows := []observationRow{
		{Provider: "comtrade", Classification: "H6", ProductCode: "854232", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodMonth, Period: "2026-05", ValueUSD: 60},
		{Provider: "comtrade", Classification: "H6", ProductCode: "854232", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowImport, PeriodType: model.PeriodMonth, Period: "2026-05", ValueUSD: 40},
		{Provider: "comtrade", Classification: "H6", ProductCode: "854232", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "CHN", Flow: model.FlowExport, PeriodType: model.PeriodMonth, Period: "2026-05", ValueUSD: 30},
		{Provider: "comtrade", Classification: "H6", ProductCode: "854232", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "CHN", Flow: model.FlowImport, PeriodType: model.PeriodMonth, Period: "2026-05", ValueUSD: 70},
		{Provider: "comtrade", Classification: "H6", ProductCode: "854232", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: "2026", ValueUSD: 999},
		{Provider: "comtrade", Classification: "H6", ProductCode: "999999", ProductLevel: 6, ReporterISO: "KOR", PartnerISO: "USA", Flow: model.FlowExport, PeriodType: model.PeriodMonth, Period: "2026-05", ValueUSD: 999},
	}

	index, files := buildSemiconductorMonthlyFiles("2026-07-16T00:00:00Z", "comtrade", []string{"USA", "CHN"}, rows, products, reference)
	if index.ObservationCount != 4 || len(index.Partitions) != 1 || index.Partitions[0].Href != "./KOR.json" || index.Partitions[0].PeriodCount != 1 {
		t.Fatalf("unexpected monthly index: %+v", index)
	}
	file := files["KOR.json"]
	if len(file.Rows) != 1 || file.Rows[0].Label != "Memories" || file.Rows[0].USA.Trade != 100 || file.Rows[0].CHN.Trade != 100 || file.Rows[0].Total != 200 || file.Rows[0].ShareCN != 0.5 {
		t.Fatalf("unexpected monthly partition: %+v", file)
	}
}
