package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"tradegravity/internal/model"
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
			provider, reporter_iso3, partner_iso3, flow, period_type, period,
			value_usd, ingested_at, source_updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, reporter_iso3, partner_iso3, flow, period_type, period)
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

func (s *Store) ListReporters(ctx context.Context, onlyActive bool) ([]model.Reporter, error) {
	_ = ctx
	_ = onlyActive
	return nil, nil
}

func (s *Store) migrate() error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS trade_observations (
			provider TEXT NOT NULL,
			reporter_iso3 TEXT NOT NULL,
			partner_iso3 TEXT NOT NULL,
			flow TEXT NOT NULL,
			period_type TEXT NOT NULL,
			period TEXT NOT NULL,
			value_usd REAL NOT NULL,
			ingested_at TEXT NOT NULL,
			source_updated_at TEXT,
			PRIMARY KEY (provider, reporter_iso3, partner_iso3, flow, period_type, period)
		);`,
	}

	for _, statement := range statements {
		if _, err := s.db.Exec(statement); err != nil {
			return err
		}
	}

	return nil
}
