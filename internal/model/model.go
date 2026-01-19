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
	ReporterISO3    string
	PartnerISO3     string
	Flow            Flow
	PeriodType      PeriodType
	Period          string
	ValueUSD        float64
	IngestedAt      time.Time
	SourceUpdatedAt time.Time
}
