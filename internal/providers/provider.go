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

// ProductProvider is implemented by sources that can return a commodity
// breakdown. Product observations must carry Classification, ProductCode, and
// ProductLevel so they never mix silently with total-trade observations.
type ProductProvider interface {
	FetchProducts(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow, year string, level int) ([]model.Observation, error)
}
