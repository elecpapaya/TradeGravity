package store

import (
	"context"
	"errors"

	"tradegravity/internal/model"
)

type Store interface {
	UpsertObservations(ctx context.Context, observations []model.Observation) error
	RecordIngestRun(ctx context.Context, run model.IngestRun) error
	DominantAnnualPeriod(ctx context.Context, provider string) (string, error)
	ListReporters(ctx context.Context, onlyActive bool) ([]model.Reporter, error)
	ListObservationKeys(ctx context.Context, provider, reporterISO3, partnerISO3 string, flow model.Flow) ([]ObservationKey, error)
	Close() error
}

type NopStore struct{}

func (s *NopStore) UpsertObservations(ctx context.Context, observations []model.Observation) error {
	_ = ctx
	_ = observations
	return nil
}

func (s *NopStore) RecordIngestRun(ctx context.Context, run model.IngestRun) error {
	_ = ctx
	_ = run
	return nil
}

func (s *NopStore) DominantAnnualPeriod(ctx context.Context, provider string) (string, error) {
	_ = ctx
	_ = provider
	return "", errors.New("dominant period requires persistent storage")
}

func (s *NopStore) ListReporters(ctx context.Context, onlyActive bool) ([]model.Reporter, error) {
	_ = onlyActive
	return nil, nil
}

func (s *NopStore) ListObservationKeys(ctx context.Context, provider, reporterISO3, partnerISO3 string, flow model.Flow) ([]ObservationKey, error) {
	_ = ctx
	_ = provider
	_ = reporterISO3
	_ = partnerISO3
	_ = flow
	return nil, nil
}

func (s *NopStore) Close() error {
	return nil
}

type ObservationKey struct {
	PeriodType model.PeriodType
	Period     string
}
