package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPublicationChangesStartsWithExplicitBaseline(t *testing.T) {
	index := semiconductorMonthlyIndexFile{GeneratedAt: "2026-07-16T00:00:00Z", Reporters: []string{"KOR"}, Periods: []string{"2026-06"}, ObservationCount: 4}
	changes, err := buildPublicationChanges(index.GeneratedAt, "", index, map[string]semiconductorMonthlyFile{})
	if err != nil {
		t.Fatal(err)
	}
	if changes.Status != "baseline" || changes.PreviousGeneratedAt != "" || changes.Summary.CurrentObservationCount != 4 {
		t.Fatalf("unexpected baseline: %+v", changes)
	}
}

func TestBuildPublicationChangesKeepsEmptyDimensionsAsArrays(t *testing.T) {
	changes, err := buildPublicationChanges("2026-07-16T00:00:00Z", "", semiconductorMonthlyIndexFile{Reporters: []string{}, Periods: []string{}}, map[string]semiconductorMonthlyFile{})
	if err != nil {
		t.Fatal(err)
	}
	if changes.CurrentPeriods == nil || changes.CurrentReporters == nil {
		t.Fatalf("empty dimensions must encode as arrays: %+v", changes)
	}
}

func TestBuildPublicationChangesDetectsCoverageRowsAndRevisions(t *testing.T) {
	previousDir := t.TempDir()
	previousIndex := semiconductorMonthlyIndexFile{
		SchemaVersion: "2.0", GeneratedAt: "2026-07-09T00:00:00Z", Provider: "comtrade", Level: 6,
		Reporters: []string{"JPN", "KOR"}, Periods: []string{"2026-05"}, ObservationCount: 8,
		Partitions: []semiconductorMonthlyPartition{{ReporterISO3: "JPN"}, {ReporterISO3: "KOR"}},
	}
	previousFiles := map[string]semiconductorMonthlyFile{
		"JPN.json": monthlyTestFile("2026-07-09T00:00:00Z", "JPN", []semiconductorMonthlyProductEntry{
			monthlyTestRow("2026-05", "854231", "Processors", 40, 60),
		}),
		"KOR.json": monthlyTestFile("2026-07-09T00:00:00Z", "KOR", []semiconductorMonthlyProductEntry{
			monthlyTestRow("2026-05", "854231", "Processors", 100, 100),
			monthlyTestRow("2026-05", "854232", "Memories", 50, 50),
		}),
	}
	writePreviousMonthlyFixture(t, previousDir, previousIndex, previousFiles)

	currentIndex := semiconductorMonthlyIndexFile{
		SchemaVersion: "2.0", GeneratedAt: "2026-07-16T00:00:00Z", Provider: "comtrade", Level: 6,
		Reporters: []string{"KOR", "TWN"}, Periods: []string{"2026-05", "2026-06"}, ObservationCount: 12,
	}
	currentFiles := map[string]semiconductorMonthlyFile{
		"KOR.json": monthlyTestFile(currentIndex.GeneratedAt, "KOR", []semiconductorMonthlyProductEntry{
			monthlyTestRow("2026-05", "854231", "Processors", 130, 80),
			monthlyTestRow("2026-06", "854231", "Processors", 90, 110),
		}),
		"TWN.json": monthlyTestFile(currentIndex.GeneratedAt, "TWN", []semiconductorMonthlyProductEntry{
			monthlyTestRow("2026-06", "854231", "Processors", 120, 80),
		}),
	}
	changes, err := buildPublicationChanges(currentIndex.GeneratedAt, previousDir, currentIndex, currentFiles)
	if err != nil {
		t.Fatal(err)
	}
	if changes.Status != "changed" || changes.PreviousGeneratedAt != previousIndex.GeneratedAt {
		t.Fatalf("unexpected comparison status: %+v", changes)
	}
	if changes.Summary.AddedRows != 2 || changes.Summary.RemovedRows != 2 || changes.Summary.RevisedRows != 1 || changes.Summary.ObservationDelta != 4 {
		t.Fatalf("unexpected change counts: %+v", changes.Summary)
	}
	if len(changes.NewPeriods) != 1 || changes.NewPeriods[0] != "2026-06" || len(changes.NewReporters) != 1 || changes.NewReporters[0] != "TWN" || len(changes.RemovedReporters) != 1 || changes.RemovedReporters[0] != "JPN" {
		t.Fatalf("unexpected dimension changes: %+v", changes)
	}
	if len(changes.TopRevisions) != 1 || changes.TopRevisions[0].ReporterISO3 != "KOR" || changes.TopRevisions[0].MagnitudeTradeUSD != 50 || changes.TopRevisions[0].DeltaTradeUSD != 10 {
		t.Fatalf("unexpected revisions: %+v", changes.TopRevisions)
	}
}

func TestBuildPublicationChangesMarksComparablePublicationUnchanged(t *testing.T) {
	previousDir := t.TempDir()
	index := semiconductorMonthlyIndexFile{
		SchemaVersion: "2.0", GeneratedAt: "2026-07-09T00:00:00Z", Provider: "comtrade", Level: 6,
		Reporters: []string{"KOR"}, Periods: []string{"2026-05"}, ObservationCount: 4,
		Partitions: []semiconductorMonthlyPartition{{ReporterISO3: "KOR"}},
	}
	file := monthlyTestFile(index.GeneratedAt, "KOR", []semiconductorMonthlyProductEntry{monthlyTestRow("2026-05", "854231", "Processors", 100, 80)})
	writePreviousMonthlyFixture(t, previousDir, index, map[string]semiconductorMonthlyFile{"KOR.json": file})
	index.GeneratedAt = "2026-07-16T00:00:00Z"
	file.GeneratedAt = index.GeneratedAt
	changes, err := buildPublicationChanges(index.GeneratedAt, previousDir, index, map[string]semiconductorMonthlyFile{"KOR.json": file})
	if err != nil {
		t.Fatal(err)
	}
	if changes.Status != "unchanged" || changes.Summary.RevisedRows != 0 || len(changes.TopRevisions) != 0 {
		t.Fatalf("expected unchanged publication: %+v", changes)
	}
}

func monthlyTestRow(period, code, label string, usaTrade, chinaTrade float64) semiconductorMonthlyProductEntry {
	row := semiconductorMonthlyProductEntry{
		Period: period, Classification: "H6", Code: code, Label: label,
		USA:   seriesBlock{Available: true, Export: usaTrade, Trade: usaTrade},
		CHN:   seriesBlock{Available: true, Export: chinaTrade, Trade: chinaTrade},
		Total: usaTrade + chinaTrade,
	}
	if row.Total > 0 {
		row.ShareCN = chinaTrade / row.Total
	}
	return row
}

func monthlyTestFile(generatedAt, reporter string, rows []semiconductorMonthlyProductEntry) semiconductorMonthlyFile {
	return semiconductorMonthlyFile{SchemaVersion: "2.0", GeneratedAt: generatedAt, Provider: "comtrade", Level: 6, Partners: []string{"USA", "CHN"}, ReporterISO3: reporter, Periods: []string{"2026-05"}, Rows: rows}
}

func writePreviousMonthlyFixture(t *testing.T, dataDir string, index semiconductorMonthlyIndexFile, files map[string]semiconductorMonthlyFile) {
	t.Helper()
	dir := filepath.Join(dataDir, "semiconductors", "monthly")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(dir, "index.json"), index); err != nil {
		t.Fatal(err)
	}
	for name, file := range files {
		if err := writeJSON(filepath.Join(dir, name), file); err != nil {
			t.Fatal(err)
		}
	}
}
