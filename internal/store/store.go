package store

import (
	"context"

	"tradegravity/internal/model"
)

type Store interface {
	UpsertObservations(ctx context.Context, observations []model.Observation) error
	ListReporters(ctx context.Context, onlyActive bool) ([]model.Reporter, error)
	Close() error
}

type NopStore struct{}

func (s *NopStore) UpsertObservations(ctx context.Context, observations []model.Observation) error {
	_ = ctx
	_ = observations
	return nil
}

func (s *NopStore) ListReporters(ctx context.Context, onlyActive bool) ([]model.Reporter, error) {
	_ = onlyActive
	return nil, nil
}

func (s *NopStore) Close() error {
	return nil
}
