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

// SelectedProductProvider fetches a bounded list of product codes. It supports
// curated high-detail collections without requesting every product in a
// classification revision.
type SelectedProductProvider interface {
	FetchProductCodes(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow, year string, level int, codes []string) ([]model.Observation, error)
}

// TariffProvider exposes detailed HS6 tariff schedules separately from trade
// values. Implementations must preserve the source nomenclature and data type
// so reported rates are never silently mixed with estimated AVEs.
type TariffProvider interface {
	Name() string
	ListTariffImporters(ctx context.Context) ([]model.Reporter, error)
	LatestTariffYear(ctx context.Context, importerISO3 string) (string, error)
	FetchTariffs(ctx context.Context, importerISO3, exporterISO3, year string, codes []string, dataType model.TariffDataType) ([]model.TariffObservation, error)
}

// PartnerMatrixProvider returns total trade with every individually reported
// partner for a reporter/year/flow. World aggregates and country groups must
// not be emitted as if they were bilateral country links.
type PartnerMatrixProvider interface {
	Name() string
	ListReporters(ctx context.Context) ([]model.Reporter, error)
	FetchPartnerMatrix(ctx context.Context, reporterISO3 string, flow model.Flow, year string) ([]model.Observation, error)
}
