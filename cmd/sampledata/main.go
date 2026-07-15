// Command sampledata creates a deterministic, offline SQLite fixture and
// country-context file used to regenerate examples/sample-data.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tradegravity/internal/model"
	"tradegravity/internal/store/sqlite"
)

type contextMetric struct {
	Value float64 `json:"value"`
	Year  string  `json:"year"`
}

type contextCountry struct {
	ISO3        string        `json:"iso3"`
	ISO2        string        `json:"iso2"`
	Name        string        `json:"name"`
	Region      string        `json:"region"`
	IncomeGroup string        `json:"income_group"`
	Groups      []string      `json:"groups"`
	Population  contextMetric `json:"population"`
	GDP         contextMetric `json:"gdp"`
}

type contextFile struct {
	SchemaVersion string           `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Source        string           `json:"source"`
	Status        string           `json:"status"`
	Errors        []string         `json:"errors"`
	Countries     []contextCountry `json:"countries"`
}

func main() {
	dbPath := flag.String("db", "sample-fixture.db", "new SQLite fixture path (must not already exist)")
	contextPath := flag.String("context", "examples/sample-data/context.json", "context JSON output path")
	flag.Parse()
	if _, err := os.Stat(*dbPath); err == nil {
		fmt.Fprintf(os.Stderr, "refusing to overwrite existing fixture %s\n", *dbPath)
		os.Exit(1)
	} else if !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	store, err := sqlite.New(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer store.Close()
	ctx := context.Background()
	observations := totalObservations()
	observations = append(observations, productObservations()...)
	matrix := matrixObservations()
	observations = append(observations, matrix...)
	if err := store.UpsertObservations(ctx, observations); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	tariffs := tariffObservations()
	if err := store.UpsertTariffObservations(ctx, tariffs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := store.RecordIngestRun(ctx, model.IngestRun{
		RunID: "sample-wits-totals", Provider: "wits", Mode: "totals", StartedAt: fixed,
		FinishedAt: fixed.Add(time.Minute), Status: "success", ReporterCount: 3,
		RequestCount: 12, SuccessCount: 12, StoredCount: 60, Errors: []string{},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := store.RecordIngestRun(ctx, model.IngestRun{
		RunID: "sample-trains-tariffs", Provider: "trains", Mode: "tariffs-strategic-hs6", StartedAt: fixed.Add(4 * time.Minute),
		FinishedAt: fixed.Add(5 * time.Minute), Status: "success", ReporterCount: 3,
		RequestCount: 3, SuccessCount: 3, StoredCount: len(tariffs), Errors: []string{},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := store.RecordIngestRun(ctx, model.IngestRun{
		RunID: "sample-comtrade-products", Provider: "comtrade", Mode: "products-hs2", StartedAt: fixed.Add(2 * time.Minute),
		FinishedAt: fixed.Add(3 * time.Minute), Status: "success", ReporterCount: 3,
		RequestCount: 12, SuccessCount: 12, StoredCount: 36, Errors: []string{},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := store.RecordIngestRun(ctx, model.IngestRun{
		RunID: "sample-comtrade-matrix", Provider: "comtrade", Mode: "bilateral-matrix", StartedAt: fixed.Add(6 * time.Minute),
		FinishedAt: fixed.Add(7 * time.Minute), Status: "success", ReporterCount: 3,
		RequestCount: 6, SuccessCount: 6, StoredCount: len(matrix), Errors: []string{},
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := writeContext(*contextPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("sample fixture created (db=%s observations=%d context=%s)\n", *dbPath, len(observations), *contextPath)
}

func tariffObservations() []model.TariffObservation {
	products := []struct {
		code string
		base float64
	}{{"260300", 1.5}, {"850760", 3.2}, {"854231", 2.4}}
	fixed := time.Date(2025, 8, 11, 0, 0, 0, 0, time.UTC)
	var observations []model.TariffObservation
	for importerIndex, importer := range []string{"DEU", "JPN", "KOR"} {
		for _, product := range products {
			rate := product.base + float64(importerIndex)*0.4
			minRate, maxRate, sumRate := rate, rate+1, rate*2+1
			observations = append(observations, model.TariffObservation{
				Provider: "trains", Classification: "HS2017", ProductCode: product.code, ProductLevel: 6,
				ImporterISO3: importer, ExporterISO3: "WLD", ExporterCode: "000", DataType: model.TariffAVEEstimated,
				RateType: model.TariffMFNApplied, Regime: "mfn", Year: "2023", RatePercent: rate,
				SumRatePercent: &sumRate, MinRatePercent: &minRate, MaxRatePercent: &maxRate,
				TotalLines: 2, MFNLines: 2, Nomenclature: "H5", SourceUpdatedAt: fixed,
			})
		}
	}
	return observations
}

func totalObservations() []model.Observation {
	type reporterBase struct {
		iso3 string
		usa  float64
		chn  float64
	}
	reporters := []reporterBase{{"DEU", 210e9, 205e9}, {"JPN", 195e9, 260e9}, {"KOR", 165e9, 245e9}}
	var observations []model.Observation
	for _, reporter := range reporters {
		for year := 2019; year <= 2023; year++ {
			factor := 1 + float64(year-2019)*0.025
			for _, partner := range []struct {
				iso3 string
				base float64
			}{{"USA", reporter.usa}, {"CHN", reporter.chn}} {
				for _, flow := range []struct {
					name  model.Flow
					share float64
				}{{model.FlowExport, 0.58}, {model.FlowImport, 0.42}} {
					observations = append(observations, model.Observation{
						Provider: "wits", ReporterISO3: reporter.iso3, PartnerISO3: partner.iso3,
						Flow: flow.name, PeriodType: model.PeriodYear, Period: fmt.Sprintf("%d", year),
						ValueUSD: partner.base * factor * flow.share,
					})
				}
			}
		}
	}
	return observations
}

func productObservations() []model.Observation {
	chapters := []struct {
		code  string
		value float64
	}{{"85", 28e9}, {"87", 17e9}, {"90", 9e9}}
	var observations []model.Observation
	for reporterIndex, reporter := range []string{"DEU", "JPN", "KOR"} {
		for _, chapter := range chapters {
			for partnerIndex, partner := range []string{"USA", "CHN"} {
				for _, flow := range []struct {
					name  model.Flow
					share float64
				}{{model.FlowExport, 0.6}, {model.FlowImport, 0.4}} {
					observations = append(observations, model.Observation{
						Provider: "comtrade", Classification: "H6", ProductCode: chapter.code, ProductLevel: 2,
						ReporterISO3: reporter, PartnerISO3: partner, Flow: flow.name,
						PeriodType: model.PeriodYear, Period: "2023",
						ValueUSD: chapter.value * (1 + float64(reporterIndex)*0.1) * (1 + float64(partnerIndex)*0.15) * flow.share,
					})
				}
			}
		}
	}
	return observations
}

func matrixObservations() []model.Observation {
	partners := []struct {
		iso3  string
		value float64
	}{{"USA", 160e9}, {"CHN", 210e9}, {"VNM", 55e9}, {"MEX", 42e9}, {"AUS", 36e9}}
	var observations []model.Observation
	for reporterIndex, reporter := range []string{"DEU", "JPN", "KOR"} {
		for partnerIndex, partner := range partners {
			for _, flow := range []struct {
				name  model.Flow
				share float64
			}{{model.FlowExport, 0.57}, {model.FlowImport, 0.43}} {
				observations = append(observations, model.Observation{
					Provider: "comtrade", Classification: "H6", ProductCode: "TOTAL", ProductLevel: 0,
					ReporterISO3: reporter, PartnerISO3: partner.iso3, Flow: flow.name,
					PeriodType: model.PeriodYear, Period: "2023",
					ValueUSD: partner.value * (1 + float64(reporterIndex)*0.08) * (1 + float64(partnerIndex)*0.01) * flow.share,
				})
			}
		}
	}
	return observations
}

func writeContext(path string) error {
	data := contextFile{
		SchemaVersion: "1.0", GeneratedAt: "2026-01-01T00:00:00Z",
		Source: "Deterministic TradeGravity sample fixture", Status: "success", Errors: []string{},
		Countries: []contextCountry{
			{ISO3: "DEU", ISO2: "DE", Name: "Germany", Region: "Europe & Central Asia", IncomeGroup: "High income", Groups: []string{"EU"}, Population: contextMetric{84_480_000, "2023"}, GDP: contextMetric{4.53e12, "2023"}},
			{ISO3: "JPN", ISO2: "JP", Name: "Japan", Region: "East Asia & Pacific", IncomeGroup: "High income", Groups: []string{}, Population: contextMetric{124_520_000, "2023"}, GDP: contextMetric{4.21e12, "2023"}},
			{ISO3: "KOR", ISO2: "KR", Name: "Korea, Rep.", Region: "East Asia & Pacific", IncomeGroup: "High income", Groups: []string{}, Population: contextMetric{51_710_000, "2023"}, GDP: contextMetric{1.71e12, "2023"}},
		},
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}
