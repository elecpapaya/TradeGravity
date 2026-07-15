package main

import (
	"strings"
	"testing"
)

func TestValidateDatasetAcceptsConsistentData(t *testing.T) {
	metadata, latest := validDataset()
	if err := validateDataset(metadata, latest, 1); err != nil {
		t.Fatalf("validateDataset() error = %v", err)
	}
}

func TestLoadDatasetReadsValidFixture(t *testing.T) {
	metadata, latest, err := loadDataset("testdata/valid")
	if err != nil {
		t.Fatalf("loadDataset() error = %v", err)
	}
	if err := validateDataset(metadata, latest, 1); err != nil {
		t.Fatalf("fixture validation error = %v", err)
	}
}

func TestValidateDatasetRejectsUnsafeOrInconsistentData(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*datasetMeta, *datasetLatest)
		message string
	}{
		{
			name: "duplicate reporter",
			mutate: func(meta *datasetMeta, latest *datasetLatest) {
				latest.Rows = append(latest.Rows, latest.Rows[0])
				meta.ReporterCount++
				meta.ExpectedPartnerBlocks += 2
				meta.AvailablePartnerBlocks += 2
				meta.PeriodCounts["Y:2023"] += 2
			},
			message: "duplicate reporter",
		},
		{
			name: "bad total",
			mutate: func(_ *datasetMeta, latest *datasetLatest) {
				latest.Rows[0].Total = 999
			},
			message: "does not equal USA+CHN",
		},
		{
			name: "invalid period",
			mutate: func(_ *datasetMeta, latest *datasetLatest) {
				latest.Rows[0].USA.Period = "2023-99"
			},
			message: "invalid period",
		},
		{
			name: "negative value",
			mutate: func(_ *datasetMeta, latest *datasetLatest) {
				latest.Rows[0].USA.Export = -1
			},
			message: "non-negative",
		},
		{
			name: "coverage mismatch",
			mutate: func(meta *datasetMeta, _ *datasetLatest) {
				meta.AvailablePartnerBlocks = 1
			},
			message: "coverage mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata, latest := validDataset()
			tt.mutate(&metadata, &latest)
			err := validateDataset(metadata, latest, 1)
			if err == nil || !strings.Contains(err.Error(), tt.message) {
				t.Fatalf("validateDataset() error = %v, want message containing %q", err, tt.message)
			}
		})
	}
}

func validDataset() (datasetMeta, datasetLatest) {
	usa := partnerBlock{Period: "2023", PeriodType: "Y", Export: 40, Import: 60, Trade: 100}
	chn := partnerBlock{Period: "2023", PeriodType: "Y", Export: 20, Import: 80, Trade: 100}
	latest := datasetLatest{
		SchemaVersion: "1.0",
		GeneratedAt:   "2026-07-15T00:00:00Z",
		Provider:      "wits",
		Partners:      []string{"USA", "CHN"},
		Rows: []datasetRow{{
			ISO3:    "KOR",
			USA:     usa,
			CHN:     chn,
			Total:   200,
			ShareCN: 0.5,
		}},
	}
	metadata := datasetMeta{
		SchemaVersion:          "1.0",
		GeneratedAt:            latest.GeneratedAt,
		Provider:               latest.Provider,
		Partners:               append([]string(nil), latest.Partners...),
		ReporterCount:          1,
		ObservationCount:       4,
		ExpectedPartnerBlocks:  2,
		AvailablePartnerBlocks: 2,
		MissingPartnerBlocks:   0,
		PeriodCounts:           map[string]int{"Y:2023": 2},
	}
	return metadata, latest
}
