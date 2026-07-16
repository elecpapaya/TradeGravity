package main

import "testing"

func TestValidatePublicationChangesAcceptsComparableFeed(t *testing.T) {
	metadata := datasetMeta{GeneratedAt: "2026-07-16T00:00:00Z"}
	monthly := validationSemiconductorMonthlyIndex{
		GeneratedAt: metadata.GeneratedAt,
		Reporters:   []string{"KOR"}, Periods: []string{"2026-05", "2026-06"}, ObservationCount: 12,
	}
	ratio := 0.25
	changes := validationPublicationChanges{
		SchemaVersion: "1.0", GeneratedAt: metadata.GeneratedAt, PreviousGeneratedAt: "2026-07-09T00:00:00Z", Status: "changed", Scope: "publish-to-publish",
		Summary:        validationPublicationChangeSummary{CurrentObservationCount: 12, PreviousObservationCount: 8, ObservationDelta: 4, AddedRows: 2, RevisedRows: 1},
		CurrentPeriods: []string{"2026-05", "2026-06"}, NewPeriods: []string{"2026-06"}, RemovedPeriods: []string{},
		CurrentReporters: []string{"KOR"}, NewReporters: []string{}, RemovedReporters: []string{},
		TopRevisions: []validationPublicationRevision{{
			ReporterISO3: "KOR", Period: "2026-05", Classification: "H6", Code: "854231", Label: "Processors",
			PreviousUSATradeUSD: 40, CurrentUSATradeUSD: 55, PreviousChinaTradeUSD: 60, CurrentChinaTradeUSD: 70,
			PreviousTotalUSD: 100, CurrentTotalUSD: 125, DeltaTradeUSD: 25, MagnitudeTradeUSD: 25, ChangeRatio: &ratio,
		}},
	}
	if err := validatePublicationChanges(metadata, monthly, changes); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePublicationChangesRejectsFalseUnchangedClaim(t *testing.T) {
	metadata := datasetMeta{GeneratedAt: "2026-07-16T00:00:00Z"}
	monthly := validationSemiconductorMonthlyIndex{Reporters: []string{"KOR"}, Periods: []string{"2026-06"}, ObservationCount: 12}
	changes := validationPublicationChanges{
		SchemaVersion: "1.0", GeneratedAt: metadata.GeneratedAt, PreviousGeneratedAt: "2026-07-09T00:00:00Z", Status: "unchanged", Scope: "publish-to-publish",
		Summary:        validationPublicationChangeSummary{CurrentObservationCount: 12, PreviousObservationCount: 8, ObservationDelta: 4},
		CurrentPeriods: []string{"2026-06"}, CurrentReporters: []string{"KOR"},
		NewPeriods: []string{}, RemovedPeriods: []string{}, NewReporters: []string{}, RemovedReporters: []string{}, TopRevisions: []validationPublicationRevision{},
	}
	if err := validatePublicationChanges(metadata, monthly, changes); err == nil {
		t.Fatal("expected inconsistent unchanged status to be rejected")
	}
}
