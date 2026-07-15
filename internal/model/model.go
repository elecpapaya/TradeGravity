package model

import "time"

type Flow string

const (
	FlowExport Flow = "export"
	FlowImport Flow = "import"
)

type PeriodType string

const (
	PeriodMonth   PeriodType = "M"
	PeriodQuarter PeriodType = "Q"
	PeriodYear    PeriodType = "Y"
)

type Reporter struct {
	ISO3     string
	NameEN   string
	NameKO   string
	Region   string
	IsActive bool
}

type Observation struct {
	Provider        string
	Classification  string
	ProductCode     string
	ProductLevel    int
	ReporterISO3    string
	PartnerISO3     string
	Flow            Flow
	PeriodType      PeriodType
	Period          string
	ValueUSD        float64
	IngestedAt      time.Time
	SourceUpdatedAt time.Time
}

type TariffRateType string

const (
	TariffMFNApplied         TariffRateType = "mfn_applied"
	TariffEffectivelyApplied TariffRateType = "effectively_applied"
	TariffPreferential       TariffRateType = "preferential"
)

type TariffDataType string

const (
	// TariffReported contains only rates reported as ad valorem by the source.
	TariffReported TariffDataType = "reported"
	// TariffAVEEstimated also includes specific duties converted to ad valorem
	// equivalents by the source methodology.
	TariffAVEEstimated TariffDataType = "ave_estimated"
)

// TariffObservation is deliberately separate from trade observations. Rates
// describe an importer/product/regime/year and must never be interpreted as a
// trade value or silently joined across classification revisions.
type TariffObservation struct {
	Provider          string
	Classification    string
	ProductCode       string
	ProductLevel      int
	ImporterISO3      string
	ExporterISO3      string
	ExporterCode      string
	DataType          TariffDataType
	RateType          TariffRateType
	Regime            string
	Year              string
	RatePercent       float64
	SumRatePercent    *float64
	MinRatePercent    *float64
	MaxRatePercent    *float64
	TotalLines        int
	PreferentialLines int
	MFNLines          int
	NonAdValoremLines int
	Nomenclature      string
	ExcludedFrom      string
	IngestedAt        time.Time
	SourceUpdatedAt   time.Time
}

// IngestRun records one collector invocation so published quality metadata can
// distinguish complete, partial, and failed refreshes.
type IngestRun struct {
	RunID         string
	Provider      string
	Mode          string
	StartedAt     time.Time
	FinishedAt    time.Time
	Status        string
	ReporterCount int
	RequestCount  int
	SuccessCount  int
	FailureCount  int
	SkippedCount  int
	StoredCount   int
	Errors        []string
}
