package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"tradegravity/internal/model"
	"tradegravity/internal/store"
)

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite: path is required")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) UpsertObservations(ctx context.Context, observations []model.Observation) error {
	if len(observations) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO trade_observations (
			provider, classification, product_code, product_level,
			reporter_iso3, partner_iso3, flow, period_type, period,
			value_usd, ingested_at, source_updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, classification, product_code, reporter_iso3, partner_iso3, flow, period_type, period)
		DO UPDATE SET
			value_usd = excluded.value_usd,
			ingested_at = excluded.ingested_at,
			source_updated_at = excluded.source_updated_at
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for i := range observations {
		observation := observations[i]
		observation.Provider = strings.ToLower(strings.TrimSpace(observation.Provider))
		observation.Classification = strings.ToUpper(strings.TrimSpace(observation.Classification))
		observation.ProductCode = strings.ToUpper(strings.TrimSpace(observation.ProductCode))
		if observation.ProductCode == "" {
			observation.ProductCode = "TOTAL"
		}
		if observation.ProductCode == "TOTAL" {
			observation.ProductLevel = 0
		}
		if observation.IngestedAt.IsZero() {
			observation.IngestedAt = now
		}
		var sourceUpdatedAt any
		if !observation.SourceUpdatedAt.IsZero() {
			sourceUpdatedAt = observation.SourceUpdatedAt.UTC()
		}
		_, err = stmt.ExecContext(
			ctx,
			observation.Provider,
			observation.Classification,
			observation.ProductCode,
			observation.ProductLevel,
			observation.ReporterISO3,
			observation.PartnerISO3,
			string(observation.Flow),
			string(observation.PeriodType),
			observation.Period,
			observation.ValueUSD,
			observation.IngestedAt.UTC(),
			sourceUpdatedAt,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *Store) UpsertTariffObservations(ctx context.Context, observations []model.TariffObservation) error {
	if len(observations) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO tariff_observations (
			provider, classification, product_code, product_level,
			importer_iso3, exporter_iso3, exporter_code, data_type,
			rate_type, regime, year, rate_percent,
			sum_rate_percent, min_rate_percent, max_rate_percent,
			total_lines, preferential_lines, mfn_lines, non_ad_valorem_lines,
			nomenclature, excluded_from, ingested_at, source_updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, classification, product_code, importer_iso3, exporter_iso3, data_type, rate_type, regime, year)
		DO UPDATE SET
			exporter_code = excluded.exporter_code,
			rate_percent = excluded.rate_percent,
			sum_rate_percent = excluded.sum_rate_percent,
			min_rate_percent = excluded.min_rate_percent,
			max_rate_percent = excluded.max_rate_percent,
			total_lines = excluded.total_lines,
			preferential_lines = excluded.preferential_lines,
			mfn_lines = excluded.mfn_lines,
			non_ad_valorem_lines = excluded.non_ad_valorem_lines,
			nomenclature = excluded.nomenclature,
			excluded_from = excluded.excluded_from,
			ingested_at = excluded.ingested_at,
			source_updated_at = excluded.source_updated_at
	`)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	defer stmt.Close()
	now := time.Now().UTC()
	for _, observation := range observations {
		observation.Provider = strings.ToLower(strings.TrimSpace(observation.Provider))
		observation.Classification = strings.ToUpper(strings.TrimSpace(observation.Classification))
		observation.ProductCode = strings.TrimSpace(observation.ProductCode)
		observation.ImporterISO3 = strings.ToUpper(strings.TrimSpace(observation.ImporterISO3))
		observation.ExporterISO3 = strings.ToUpper(strings.TrimSpace(observation.ExporterISO3))
		observation.ExporterCode = strings.ToUpper(strings.TrimSpace(observation.ExporterCode))
		if observation.DataType == "" {
			observation.DataType = model.TariffReported
		}
		observation.Regime = strings.ToLower(strings.TrimSpace(observation.Regime))
		observation.Year = strings.TrimSpace(observation.Year)
		observation.Nomenclature = strings.ToUpper(strings.TrimSpace(observation.Nomenclature))
		observation.ExcludedFrom = strings.ToUpper(strings.TrimSpace(observation.ExcludedFrom))
		if err = validateTariffObservation(observation); err != nil {
			_ = tx.Rollback()
			return err
		}
		if observation.IngestedAt.IsZero() {
			observation.IngestedAt = now
		}
		var sourceUpdatedAt any
		if !observation.SourceUpdatedAt.IsZero() {
			sourceUpdatedAt = observation.SourceUpdatedAt.UTC()
		}
		_, err = stmt.ExecContext(ctx,
			observation.Provider, observation.Classification, observation.ProductCode, observation.ProductLevel,
			observation.ImporterISO3, observation.ExporterISO3, observation.ExporterCode, string(observation.DataType),
			string(observation.RateType), observation.Regime, observation.Year, observation.RatePercent,
			observation.SumRatePercent, observation.MinRatePercent, observation.MaxRatePercent,
			observation.TotalLines, observation.PreferentialLines, observation.MFNLines, observation.NonAdValoremLines,
			observation.Nomenclature, observation.ExcludedFrom, observation.IngestedAt.UTC(), sourceUpdatedAt,
		)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func validateTariffObservation(observation model.TariffObservation) error {
	if observation.Provider == "" || observation.Classification == "" {
		return errors.New("tariff provider and classification are required")
	}
	if observation.ProductLevel != 6 || len(observation.ProductCode) != 6 || !digitsOnly(observation.ProductCode) {
		return fmt.Errorf("tariff product code %q must be HS6", observation.ProductCode)
	}
	if len(observation.ImporterISO3) != 3 || len(observation.ExporterISO3) != 3 {
		return errors.New("tariff importer and exporter must be ISO3-compatible codes")
	}
	if observation.RateType != model.TariffMFNApplied && observation.RateType != model.TariffEffectivelyApplied && observation.RateType != model.TariffPreferential {
		return fmt.Errorf("unsupported tariff rate type %q", observation.RateType)
	}
	if observation.DataType != model.TariffReported && observation.DataType != model.TariffAVEEstimated {
		return fmt.Errorf("unsupported tariff data type %q", observation.DataType)
	}
	if observation.Regime == "" || len(observation.Year) != 4 || !digitsOnly(observation.Year) {
		return errors.New("tariff regime and four-digit year are required")
	}
	if math.IsNaN(observation.RatePercent) || math.IsInf(observation.RatePercent, 0) || observation.RatePercent < 0 {
		return fmt.Errorf("tariff rate must be finite and non-negative, got %v", observation.RatePercent)
	}
	for label, value := range map[string]*float64{
		"sum": observation.SumRatePercent, "minimum": observation.MinRatePercent, "maximum": observation.MaxRatePercent,
	} {
		if value != nil && (math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0) {
			return fmt.Errorf("tariff %s rate must be finite and non-negative, got %v", label, *value)
		}
	}
	if observation.TotalLines < 0 || observation.PreferentialLines < 0 || observation.MFNLines < 0 || observation.NonAdValoremLines < 0 {
		return errors.New("tariff line counts must be non-negative")
	}
	return nil
}

func digitsOnly(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return value != ""
}

func (s *Store) RecordIngestRun(ctx context.Context, run model.IngestRun) error {
	if s == nil || s.db == nil {
		return nil
	}
	errorsJSON, err := json.Marshal(run.Errors)
	if err != nil {
		return fmt.Errorf("encode ingest errors: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO ingest_runs (
			run_id, provider, mode, started_at, finished_at, status,
			reporter_count, request_count, success_count, failure_count,
			skipped_count, stored_count, errors_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET
			finished_at = excluded.finished_at,
			status = excluded.status,
			reporter_count = excluded.reporter_count,
			request_count = excluded.request_count,
			success_count = excluded.success_count,
			failure_count = excluded.failure_count,
			skipped_count = excluded.skipped_count,
			stored_count = excluded.stored_count,
			errors_json = excluded.errors_json
	`, run.RunID, strings.ToLower(strings.TrimSpace(run.Provider)), run.Mode,
		run.StartedAt.UTC().Format(time.RFC3339Nano), run.FinishedAt.UTC().Format(time.RFC3339Nano), run.Status,
		run.ReporterCount, run.RequestCount, run.SuccessCount, run.FailureCount,
		run.SkippedCount, run.StoredCount, string(errorsJSON))
	if err != nil {
		return fmt.Errorf("record ingest run: %w", err)
	}
	return nil
}

func (s *Store) DominantAnnualPeriod(ctx context.Context, provider string) (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("sqlite store is not open")
	}
	var period string
	err := s.db.QueryRowContext(ctx, `
		WITH latest AS (
			SELECT reporter_iso3, partner_iso3, flow, MAX(period) AS period
			FROM trade_observations
			WHERE provider = ? AND product_level = 0 AND product_code = 'TOTAL' AND period_type = 'Y'
			GROUP BY reporter_iso3, partner_iso3, flow
		)
		SELECT period FROM latest
		GROUP BY period
		ORDER BY COUNT(*) DESC, period DESC
		LIMIT 1
	`, strings.ToLower(strings.TrimSpace(provider))).Scan(&period)
	if err != nil {
		return "", fmt.Errorf("find dominant annual period for %s: %w", provider, err)
	}
	return period, nil
}

func (s *Store) ListReporters(ctx context.Context, onlyActive bool) ([]model.Reporter, error) {
	_ = ctx
	_ = onlyActive
	return nil, nil
}

func (s *Store) ListObservationKeys(ctx context.Context, provider, reporterISO3, partnerISO3 string, flow model.Flow) ([]store.ObservationKey, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT period_type, period
		FROM trade_observations
		WHERE provider = ? AND product_level = 0 AND product_code = 'TOTAL'
		  AND reporter_iso3 = ? AND partner_iso3 = ? AND flow = ?
	`, provider, reporterISO3, partnerISO3, string(flow))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]store.ObservationKey, 0)
	for rows.Next() {
		var periodType string
		var period string
		if err := rows.Scan(&periodType, &period); err != nil {
			return nil, err
		}
		keys = append(keys, store.ObservationKey{
			PeriodType: model.PeriodType(strings.ToUpper(strings.TrimSpace(periodType))),
			Period:     strings.TrimSpace(period),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`PRAGMA foreign_keys = ON;`); err != nil {
		return err
	}
	columns, err := s.tableColumns("trade_observations")
	if err != nil {
		return err
	}
	if len(columns) > 0 {
		if _, ok := columns["product_code"]; !ok {
			if err := s.migrateObservationsV1(); err != nil {
				return err
			}
		}
	}
	tariffColumns, err := s.tableColumns("tariff_observations")
	if err != nil {
		return err
	}
	if len(tariffColumns) > 0 {
		if _, ok := tariffColumns["data_type"]; !ok {
			if err := s.migrateTariffsV2(); err != nil {
				return err
			}
		}
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS trade_observations (
			provider TEXT NOT NULL,
			classification TEXT NOT NULL DEFAULT '',
			product_code TEXT NOT NULL DEFAULT 'TOTAL',
			product_level INTEGER NOT NULL DEFAULT 0,
			reporter_iso3 TEXT NOT NULL,
			partner_iso3 TEXT NOT NULL,
			flow TEXT NOT NULL,
			period_type TEXT NOT NULL,
			period TEXT NOT NULL,
			value_usd REAL NOT NULL,
			ingested_at TEXT NOT NULL,
			source_updated_at TEXT,
			PRIMARY KEY (provider, classification, product_code, reporter_iso3, partner_iso3, flow, period_type, period)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_trade_observations_totals
		 ON trade_observations(provider, product_level, reporter_iso3, partner_iso3, period_type, period);`,
		`CREATE TABLE IF NOT EXISTS tariff_observations (
			provider TEXT NOT NULL,
			classification TEXT NOT NULL,
			product_code TEXT NOT NULL,
			product_level INTEGER NOT NULL,
			importer_iso3 TEXT NOT NULL,
			exporter_iso3 TEXT NOT NULL,
			exporter_code TEXT NOT NULL DEFAULT '',
			data_type TEXT NOT NULL,
			rate_type TEXT NOT NULL,
			regime TEXT NOT NULL,
			year TEXT NOT NULL,
			rate_percent REAL NOT NULL,
			sum_rate_percent REAL,
			min_rate_percent REAL,
			max_rate_percent REAL,
			total_lines INTEGER NOT NULL DEFAULT 0,
			preferential_lines INTEGER NOT NULL DEFAULT 0,
			mfn_lines INTEGER NOT NULL DEFAULT 0,
			non_ad_valorem_lines INTEGER NOT NULL DEFAULT 0,
			nomenclature TEXT NOT NULL DEFAULT '',
			excluded_from TEXT NOT NULL DEFAULT '',
			ingested_at TEXT NOT NULL,
			source_updated_at TEXT,
			PRIMARY KEY (provider, classification, product_code, importer_iso3, exporter_iso3, data_type, rate_type, regime, year)
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tariff_observations_lookup
		 ON tariff_observations(importer_iso3, exporter_iso3, year, product_code, data_type, rate_type);`,
		`CREATE TABLE IF NOT EXISTS ingest_runs (
			run_id TEXT PRIMARY KEY,
			provider TEXT NOT NULL,
			mode TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL,
			status TEXT NOT NULL,
			reporter_count INTEGER NOT NULL,
			request_count INTEGER NOT NULL,
			success_count INTEGER NOT NULL,
			failure_count INTEGER NOT NULL,
			skipped_count INTEGER NOT NULL,
			stored_count INTEGER NOT NULL,
			errors_json TEXT NOT NULL DEFAULT '[]'
		);`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) tableColumns(table string) (map[string]struct{}, error) {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make(map[string]struct{})
	for rows.Next() {
		var cid, notNull, pk int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, err
		}
		columns[strings.ToLower(name)] = struct{}{}
	}
	return columns, rows.Err()
}

func (s *Store) migrateObservationsV1() (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	statements := []string{
		`ALTER TABLE trade_observations RENAME TO trade_observations_v1;`,
		`CREATE TABLE trade_observations (
			provider TEXT NOT NULL,
			classification TEXT NOT NULL DEFAULT '',
			product_code TEXT NOT NULL DEFAULT 'TOTAL',
			product_level INTEGER NOT NULL DEFAULT 0,
			reporter_iso3 TEXT NOT NULL,
			partner_iso3 TEXT NOT NULL,
			flow TEXT NOT NULL,
			period_type TEXT NOT NULL,
			period TEXT NOT NULL,
			value_usd REAL NOT NULL,
			ingested_at TEXT NOT NULL,
			source_updated_at TEXT,
			PRIMARY KEY (provider, classification, product_code, reporter_iso3, partner_iso3, flow, period_type, period)
		);`,
		`INSERT INTO trade_observations (
			provider, classification, product_code, product_level, reporter_iso3,
			partner_iso3, flow, period_type, period, value_usd, ingested_at, source_updated_at
		) SELECT provider, '', 'TOTAL', 0, reporter_iso3, partner_iso3, flow,
			period_type, period, value_usd, ingested_at, source_updated_at
		  FROM trade_observations_v1;`,
		`DROP TABLE trade_observations_v1;`,
	}
	for _, statement := range statements {
		if _, err = tx.Exec(statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) migrateTariffsV2() (err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	statements := []string{
		`ALTER TABLE tariff_observations RENAME TO tariff_observations_v1;`,
		`CREATE TABLE tariff_observations (
			provider TEXT NOT NULL,
			classification TEXT NOT NULL,
			product_code TEXT NOT NULL,
			product_level INTEGER NOT NULL,
			importer_iso3 TEXT NOT NULL,
			exporter_iso3 TEXT NOT NULL,
			exporter_code TEXT NOT NULL DEFAULT '',
			data_type TEXT NOT NULL,
			rate_type TEXT NOT NULL,
			regime TEXT NOT NULL,
			year TEXT NOT NULL,
			rate_percent REAL NOT NULL,
			sum_rate_percent REAL,
			min_rate_percent REAL,
			max_rate_percent REAL,
			total_lines INTEGER NOT NULL DEFAULT 0,
			preferential_lines INTEGER NOT NULL DEFAULT 0,
			mfn_lines INTEGER NOT NULL DEFAULT 0,
			non_ad_valorem_lines INTEGER NOT NULL DEFAULT 0,
			nomenclature TEXT NOT NULL DEFAULT '',
			excluded_from TEXT NOT NULL DEFAULT '',
			ingested_at TEXT NOT NULL,
			source_updated_at TEXT,
			PRIMARY KEY (provider, classification, product_code, importer_iso3, exporter_iso3, data_type, rate_type, regime, year)
		);`,
		`INSERT INTO tariff_observations (
			provider, classification, product_code, product_level, importer_iso3,
			exporter_iso3, exporter_code, data_type, rate_type, regime, year,
			rate_percent, ingested_at, source_updated_at
		) SELECT provider, classification, product_code, product_level, importer_iso3,
			exporter_iso3, '', 'reported', rate_type, regime, year,
			rate_percent, ingested_at, source_updated_at
		  FROM tariff_observations_v1;`,
		`DROP TABLE tariff_observations_v1;`,
	}
	for _, statement := range statements {
		if _, err = tx.Exec(statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}
