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

func TestUpsertTariffObservationsKeepsRateTypesSeparate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tradegravity.db")
	store, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	base := model.TariffObservation{
		Provider: "example", Classification: "H6", ProductCode: "854231", ProductLevel: 6,
		ImporterISO3: "USA", ExporterISO3: "CHN", Regime: "general", Year: "2023", RatePercent: 10,
	}
	base.RateType = model.TariffMFNApplied
	preferential := base
	preferential.RateType = model.TariffPreferential
	preferential.Regime = "agreement-x"
	preferential.RatePercent = 5
	if err := store.UpsertTariffObservations(context.Background(), []model.TariffObservation{base, preferential}); err != nil {
		t.Fatal(err)
	}
	base.RatePercent = 12
	if err := store.UpsertTariffObservations(context.Background(), []model.TariffObservation{base}); err != nil {
		t.Fatal(err)
	}
	estimated := base
	estimated.DataType = model.TariffAVEEstimated
	estimated.RatePercent = 13
	estimated.Nomenclature = "H5"
	estimated.TotalLines = 2
	if err := store.UpsertTariffObservations(context.Background(), []model.TariffObservation{estimated}); err != nil {
		t.Fatal(err)
	}
	var count int
	var total float64
	if err := store.db.QueryRow(`SELECT COUNT(*), SUM(rate_percent) FROM tariff_observations`).Scan(&count, &total); err != nil {
		t.Fatal(err)
	}
	if count != 3 || total != 30 {
		t.Fatalf("tariff rows/count = %d/%v, want 3/30", count, total)
	}
	invalid := base
	invalid.ProductCode = "8542"
	if err := store.UpsertTariffObservations(context.Background(), []model.TariffObservation{invalid}); err == nil {
		t.Fatal("UpsertTariffObservations() accepted a non-HS6 product")
	}
}

func TestMigrateTariffObservationsAddsDataTypeWithoutDroppingRows(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	legacy, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.db.Exec(`DROP TABLE tariff_observations;
		CREATE TABLE tariff_observations (
			provider TEXT NOT NULL, classification TEXT NOT NULL, product_code TEXT NOT NULL,
			product_level INTEGER NOT NULL, importer_iso3 TEXT NOT NULL, exporter_iso3 TEXT NOT NULL,
			rate_type TEXT NOT NULL, regime TEXT NOT NULL, year TEXT NOT NULL, rate_percent REAL NOT NULL,
			ingested_at TEXT NOT NULL, source_updated_at TEXT,
			PRIMARY KEY (provider, classification, product_code, importer_iso3, exporter_iso3, rate_type, regime, year)
		);
		INSERT INTO tariff_observations VALUES ('legacy','H5','854231',6,'USA','WLD','mfn_applied','mfn','2021',2.5,'2026-01-01T00:00:00Z',NULL);`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}
	migrated, err := New(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = migrated.Close() })
	var count int
	var dataType string
	if err := migrated.db.QueryRow(`SELECT COUNT(*), MAX(data_type) FROM tariff_observations`).Scan(&count, &dataType); err != nil {
		t.Fatal(err)
	}
	if count != 1 || dataType != "reported" {
		t.Fatalf("migrated count/data_type = %d/%q", count, dataType)
	}
}
