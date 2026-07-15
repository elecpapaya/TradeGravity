package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"tradegravity/internal/model"
)

func TestUpsertObservationsAndListKeys(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tradegravity.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx := context.Background()
	observation := model.Observation{
		Provider:     "wits",
		ReporterISO3: "KOR",
		PartnerISO3:  "USA",
		Flow:         model.FlowExport,
		PeriodType:   model.PeriodYear,
		Period:       "2024",
		ValueUSD:     100,
	}
	if err := store.UpsertObservations(ctx, []model.Observation{observation}); err != nil {
		t.Fatalf("first UpsertObservations() error = %v", err)
	}

	observation.ValueUSD = 125
	if err := store.UpsertObservations(ctx, []model.Observation{observation}); err != nil {
		t.Fatalf("second UpsertObservations() error = %v", err)
	}

	keys, err := store.ListObservationKeys(ctx, "wits", "KOR", "USA", model.FlowExport)
	if err != nil {
		t.Fatalf("ListObservationKeys() error = %v", err)
	}
	if len(keys) != 1 || keys[0].PeriodType != model.PeriodYear || keys[0].Period != "2024" {
		t.Fatalf("ListObservationKeys() = %#v, want one 2024 annual key", keys)
	}

	var count int
	var value float64
	if err := store.db.QueryRow(`
		SELECT COUNT(*), MAX(value_usd)
		FROM trade_observations
		WHERE provider = 'wits' AND reporter_iso3 = 'KOR' AND partner_iso3 = 'USA'
	`).Scan(&count, &value); err != nil {
		t.Fatalf("query persisted observation: %v", err)
	}
	if count != 1 || value != 125 {
		t.Fatalf("persisted count/value = %d/%v, want 1/125", count, value)
	}
}

func TestNewRequiresPath(t *testing.T) {
	if _, err := New(""); err == nil {
		t.Fatal("New(\"\") returned nil error")
	}
}

func TestDominantAnnualPeriodUsesLatestPeriodPerSeries(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tradegravity.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var observations []model.Observation
	for _, reporter := range []string{"ARG", "AUS", "BRA"} {
		for _, year := range []string{"2014", "2015", "2016", "2017", "2018", "2019", "2020", "2021", "2022", "2023"} {
			observations = append(observations, model.Observation{Provider: "wits", ReporterISO3: reporter, PartnerISO3: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: year, ValueUSD: 1})
		}
	}
	for _, reporter := range []string{"BGD", "PAK"} {
		for _, year := range []string{"2006", "2007", "2008", "2009", "2010", "2011", "2012", "2013", "2014", "2015"} {
			observations = append(observations, model.Observation{Provider: "wits", ReporterISO3: reporter, PartnerISO3: "USA", Flow: model.FlowExport, PeriodType: model.PeriodYear, Period: year, ValueUSD: 1})
		}
	}
	if err := store.UpsertObservations(context.Background(), observations); err != nil {
		t.Fatal(err)
	}
	period, err := store.DominantAnnualPeriod(context.Background(), "wits")
	if err != nil {
		t.Fatal(err)
	}
	if period != "2023" {
		t.Fatalf("DominantAnnualPeriod() = %q, want latest-series mode 2023", period)
	}
}
