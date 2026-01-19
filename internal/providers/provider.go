package providers

import (
	"context"

	"tradegravity/internal/model"
)

type Provider interface {
	Name() string
	ListReporters(ctx context.Context) ([]model.Reporter, error)
	FetchLatest(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow) (model.Observation, error)
	FetchSeries(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow, from, to string) ([]model.Observation, error)
}
