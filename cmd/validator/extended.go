package main

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	"tradegravity/internal/semiconductor"
	"tradegravity/internal/strategic"
)

var (
	hs2Pattern = regexp.MustCompile(`^\d{2}$`)
	hs6Pattern = regexp.MustCompile(`^\d{6}$`)
)

type validationSeries struct {
	SchemaVersion string                     `json:"schema_version"`
	GeneratedAt   string                     `json:"generated_at"`
	Provider      string                     `json:"provider"`
	Partners      []string                   `json:"partners"`
	Rows          []validationReporterSeries `json:"rows"`
}

type validationReporterSeries struct {
	ISO3   string                  `json:"iso3"`
	Points []validationSeriesPoint `json:"points"`
}

type validationSeriesPoint struct {
	PeriodType string                `json:"period_type"`
	Period     string                `json:"period"`
	USA        validationSeriesBlock `json:"usa"`
	CHN        validationSeriesBlock `json:"chn"`
	Total      float64               `json:"total"`
	ShareCN    float64               `json:"share_cn"`
	Comparable bool                  `json:"comparable"`
}

type validationSeriesBlock struct {
	Available bool    `json:"available"`
	Export    float64 `json:"export"`
	Import    float64 `json:"import"`
	Trade     float64 `json:"trade"`
}

type validationProductIndex struct {
	SchemaVersion  string   `json:"schema_version"`
	GeneratedAt    string   `json:"generated_at"`
	Provider       string   `json:"provider"`
	Classification string   `json:"classification"`
	Level          int      `json:"level"`
	Partners       []string `json:"partners"`
	Periods        []string `json:"periods"`
	Reporters      []string `json:"reporters"`
}

type validationProductFile struct {
	SchemaVersion  string                   `json:"schema_version"`
	GeneratedAt    string                   `json:"generated_at"`
	Provider       string                   `json:"provider"`
	Classification string                   `json:"classification"`
	Level          int                      `json:"level"`
	ReporterISO3   string                   `json:"reporter_iso3"`
	Periods        []string                 `json:"periods"`
	Rows           []validationProductEntry `json:"rows"`
}

type validationCatalog struct {
	SchemaVersion string                      `json:"schema_version"`
	GeneratedAt   string                      `json:"generated_at"`
	Resources     []validationCatalogResource `json:"resources"`
}

type validationCatalogResource struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Status         string `json:"status"`
	Provider       string `json:"provider,omitempty"`
	Classification string `json:"classification,omitempty"`
	ProductLevel   int    `json:"product_level,omitempty"`
	Grain          string `json:"grain"`
	Partitioning   string `json:"partitioning"`
	Href           string `json:"href,omitempty"`
}

type validationProductEntry struct {
	PeriodType string                `json:"period_type"`
	Period     string                `json:"period"`
	Code       string                `json:"code"`
	Name       string                `json:"name"`
	USA        validationSeriesBlock `json:"usa"`
	CHN        validationSeriesBlock `json:"chn"`
	Total      float64               `json:"total"`
	ShareCN    float64               `json:"share_cn"`
}

type validationStrategicIndex struct {
	SchemaVersion    string                                 `json:"schema_version"`
	GeneratedAt      string                                 `json:"generated_at"`
	Provider         string                                 `json:"provider"`
	Level            int                                    `json:"level"`
	Partners         []string                               `json:"partners"`
	Sectors          []string                               `json:"sectors"`
	Products         []validationStrategicProductDescriptor `json:"products"`
	Reporters        []string                               `json:"reporters"`
	Periods          []string                               `json:"periods"`
	Partitions       []validationStrategicPartition         `json:"partitions"`
	ObservationCount int                                    `json:"observation_count"`
}

type validationStrategicProductDescriptor struct {
	Code         string `json:"code"`
	Sector       string `json:"sector"`
	Label        string `json:"label"`
	RevisionNote string `json:"revision_note"`
	Notes        string `json:"notes,omitempty"`
}

type validationStrategicPartition struct {
	ReporterISO3 string `json:"reporter_iso3"`
	Period       string `json:"period"`
	Href         string `json:"href"`
	RowCount     int    `json:"row_count"`
}

type validationStrategicFile struct {
	SchemaVersion string                            `json:"schema_version"`
	GeneratedAt   string                            `json:"generated_at"`
	Provider      string                            `json:"provider"`
	Level         int                               `json:"level"`
	Partners      []string                          `json:"partners"`
	ReporterISO3  string                            `json:"reporter_iso3"`
	Period        string                            `json:"period"`
	Rows          []validationStrategicProductEntry `json:"rows"`
}

type validationStrategicProductEntry struct {
	Classification string                `json:"classification"`
	Code           string                `json:"code"`
	Sector         string                `json:"sector"`
	Label          string                `json:"label"`
	RevisionNote   string                `json:"revision_note"`
	USA            validationSeriesBlock `json:"usa"`
	CHN            validationSeriesBlock `json:"chn"`
	Total          float64               `json:"total"`
	ShareCN        float64               `json:"share_cn"`
}

type validationTariffIndex struct {
	SchemaVersion    string                                 `json:"schema_version"`
	GeneratedAt      string                                 `json:"generated_at"`
	Provider         string                                 `json:"provider"`
	Level            int                                    `json:"level"`
	Importers        []string                               `json:"importers"`
	Exporters        []string                               `json:"exporters"`
	Years            []string                               `json:"years"`
	DataTypes        []string                               `json:"data_types"`
	RateTypes        []string                               `json:"rate_types"`
	Products         []validationStrategicProductDescriptor `json:"products"`
	Partitions       []validationTariffPartition            `json:"partitions"`
	ObservationCount int                                    `json:"observation_count"`
}

type validationTariffPartition struct {
	ImporterISO3 string `json:"importer_iso3"`
	Year         string `json:"year"`
	Href         string `json:"href"`
	RowCount     int    `json:"row_count"`
}

type validationTariffFile struct {
	SchemaVersion string                `json:"schema_version"`
	GeneratedAt   string                `json:"generated_at"`
	Provider      string                `json:"provider"`
	Level         int                   `json:"level"`
	ImporterISO3  string                `json:"importer_iso3"`
	Year          string                `json:"year"`
	Rows          []validationTariffRow `json:"rows"`
}

type validationTariffRow struct {
	Classification    string   `json:"classification"`
	Nomenclature      string   `json:"nomenclature"`
	Code              string   `json:"code"`
	Sector            string   `json:"sector"`
	Label             string   `json:"label"`
	ExporterISO3      string   `json:"exporter_iso3"`
	ExporterCode      string   `json:"exporter_code,omitempty"`
	DataType          string   `json:"data_type"`
	RateType          string   `json:"rate_type"`
	Regime            string   `json:"regime"`
	RatePercent       float64  `json:"rate_percent"`
	SumRatePercent    *float64 `json:"sum_rate_percent,omitempty"`
	MinRatePercent    *float64 `json:"min_rate_percent,omitempty"`
	MaxRatePercent    *float64 `json:"max_rate_percent,omitempty"`
	TotalLines        int      `json:"total_lines"`
	PreferentialLines int      `json:"preferential_lines"`
	MFNLines          int      `json:"mfn_lines"`
	NonAdValoremLines int      `json:"non_ad_valorem_lines"`
	ExcludedFrom      string   `json:"excluded_from,omitempty"`
	SourceUpdatedAt   string   `json:"source_updated_at,omitempty"`
}

type validationMatrixIndex struct {
	SchemaVersion    string                      `json:"schema_version"`
	GeneratedAt      string                      `json:"generated_at"`
	Provider         string                      `json:"provider"`
	ProductCode      string                      `json:"product_code"`
	ProductLevel     int                         `json:"product_level"`
	Reporters        []string                    `json:"reporters"`
	Partners         []string                    `json:"partners"`
	Periods          []string                    `json:"periods"`
	Partitions       []validationMatrixPartition `json:"partitions"`
	PartnerRowCount  int                         `json:"partner_row_count"`
	ObservationCount int                         `json:"observation_count"`
}

type validationMatrixPartition struct {
	ReporterISO3 string `json:"reporter_iso3"`
	Period       string `json:"period"`
	Href         string `json:"href"`
	RowCount     int    `json:"row_count"`
}

type validationMatrixFile struct {
	SchemaVersion string                    `json:"schema_version"`
	GeneratedAt   string                    `json:"generated_at"`
	Provider      string                    `json:"provider"`
	ProductCode   string                    `json:"product_code"`
	ProductLevel  int                       `json:"product_level"`
	ReporterISO3  string                    `json:"reporter_iso3"`
	Period        string                    `json:"period"`
	Rows          []validationMatrixPartner `json:"rows"`
}

type validationMatrixPartner struct {
	PartnerISO3     string  `json:"partner_iso3"`
	ExportAvailable bool    `json:"export_available"`
	ImportAvailable bool    `json:"import_available"`
	ExportUSD       float64 `json:"export_usd"`
	ImportUSD       float64 `json:"import_usd"`
	TradeUSD        float64 `json:"trade_usd"`
	BalanceUSD      float64 `json:"balance_usd"`
}

type validationMirrorIndex struct {
	SchemaVersion   string                      `json:"schema_version"`
	GeneratedAt     string                      `json:"generated_at"`
	Provider        string                      `json:"provider"`
	Anchors         []string                    `json:"anchors"`
	Reporters       []string                    `json:"reporters"`
	Periods         []string                    `json:"periods"`
	Partitions      []validationMirrorPartition `json:"partitions"`
	ComparisonCount int                         `json:"comparison_count"`
}

type validationMirrorPartition struct {
	ReporterISO3    string `json:"reporter_iso3"`
	Period          string `json:"period"`
	Href            string `json:"href"`
	ComparisonCount int    `json:"comparison_count"`
}

type validationMirrorFile struct {
	SchemaVersion string                       `json:"schema_version"`
	GeneratedAt   string                       `json:"generated_at"`
	Provider      string                       `json:"provider"`
	ReporterISO3  string                       `json:"reporter_iso3"`
	Period        string                       `json:"period"`
	Scope         string                       `json:"scope"`
	Caveats       []string                     `json:"caveats"`
	Rows          []validationMirrorAnchorPair `json:"rows"`
}

type validationMirrorAnchorPair struct {
	AnchorISO3              string   `json:"anchor_iso3"`
	ReporterExportAvailable bool     `json:"reporter_export_available"`
	AnchorImportAvailable   bool     `json:"anchor_import_available"`
	ReporterExportUSD       float64  `json:"reporter_export_usd"`
	AnchorImportUSD         float64  `json:"anchor_import_usd"`
	ExportGapUSD            *float64 `json:"export_gap_usd,omitempty"`
	ExportSymmetricGapRatio *float64 `json:"export_symmetric_gap_ratio,omitempty"`
	ReporterImportAvailable bool     `json:"reporter_import_available"`
	AnchorExportAvailable   bool     `json:"anchor_export_available"`
	ReporterImportUSD       float64  `json:"reporter_import_usd"`
	AnchorExportUSD         float64  `json:"anchor_export_usd"`
	ImportGapUSD            *float64 `json:"import_gap_usd,omitempty"`
	ImportSymmetricGapRatio *float64 `json:"import_symmetric_gap_ratio,omitempty"`
}

type validationSemiconductorMonthlyIndex struct {
	SchemaVersion    string                                    `json:"schema_version"`
	GeneratedAt      string                                    `json:"generated_at"`
	Provider         string                                    `json:"provider"`
	Level            int                                       `json:"level"`
	Partners         []string                                  `json:"partners"`
	Reporters        []string                                  `json:"reporters"`
	Periods          []string                                  `json:"periods"`
	Partitions       []validationSemiconductorMonthlyPartition `json:"partitions"`
	ObservationCount int                                       `json:"observation_count"`
	Scope            string                                    `json:"scope"`
}

type validationSemiconductorMonthlyPartition struct {
	ReporterISO3 string `json:"reporter_iso3"`
	Href         string `json:"href"`
	RowCount     int    `json:"row_count"`
	PeriodCount  int    `json:"period_count"`
}

type validationSemiconductorMonthlyFile struct {
	SchemaVersion string                                       `json:"schema_version"`
	GeneratedAt   string                                       `json:"generated_at"`
	Provider      string                                       `json:"provider"`
	Level         int                                          `json:"level"`
	Partners      []string                                     `json:"partners"`
	ReporterISO3  string                                       `json:"reporter_iso3"`
	Periods       []string                                     `json:"periods"`
	Rows          []validationSemiconductorMonthlyProductEntry `json:"rows"`
}

type validationSemiconductorMonthlyProductEntry struct {
	Period         string                `json:"period"`
	Classification string                `json:"classification"`
	Code           string                `json:"code"`
	Label          string                `json:"label"`
	USA            validationSeriesBlock `json:"usa"`
	CHN            validationSeriesBlock `json:"chn"`
	Total          float64               `json:"total"`
	ShareCN        float64               `json:"share_cn"`
}

type validationPublicationChanges struct {
	SchemaVersion       string                             `json:"schema_version"`
	GeneratedAt         string                             `json:"generated_at"`
	PreviousGeneratedAt string                             `json:"previous_generated_at,omitempty"`
	Status              string                             `json:"status"`
	Scope               string                             `json:"scope"`
	Summary             validationPublicationChangeSummary `json:"summary"`
	CurrentPeriods      []string                           `json:"current_periods"`
	NewPeriods          []string                           `json:"new_periods"`
	RemovedPeriods      []string                           `json:"removed_periods"`
	CurrentReporters    []string                           `json:"current_reporters"`
	NewReporters        []string                           `json:"new_reporters"`
	RemovedReporters    []string                           `json:"removed_reporters"`
	TopRevisions        []validationPublicationRevision    `json:"top_revisions"`
}

type validationPublicationChangeSummary struct {
	CurrentObservationCount  int `json:"current_observation_count"`
	PreviousObservationCount int `json:"previous_observation_count"`
	ObservationDelta         int `json:"observation_delta"`
	AddedRows                int `json:"added_rows"`
	RemovedRows              int `json:"removed_rows"`
	RevisedRows              int `json:"revised_rows"`
}

type validationPublicationRevision struct {
	ReporterISO3          string   `json:"reporter_iso3"`
	Period                string   `json:"period"`
	Classification        string   `json:"classification"`
	Code                  string   `json:"code"`
	Label                 string   `json:"label"`
	PreviousUSATradeUSD   float64  `json:"previous_usa_trade_usd"`
	CurrentUSATradeUSD    float64  `json:"current_usa_trade_usd"`
	PreviousChinaTradeUSD float64  `json:"previous_china_trade_usd"`
	CurrentChinaTradeUSD  float64  `json:"current_china_trade_usd"`
	PreviousTotalUSD      float64  `json:"previous_total_usd"`
	CurrentTotalUSD       float64  `json:"current_total_usd"`
	DeltaTradeUSD         float64  `json:"delta_trade_usd"`
	MagnitudeTradeUSD     float64  `json:"magnitude_trade_usd"`
	ChangeRatio           *float64 `json:"change_ratio,omitempty"`
}

type validationBriefing struct {
	SchemaVersion      string                           `json:"schema_version"`
	GeneratedAt        string                           `json:"generated_at"`
	EditionID          string                           `json:"edition_id"`
	Status             string                           `json:"status"`
	Title              string                           `json:"title"`
	Scope              string                           `json:"scope"`
	LatestPeriod       string                           `json:"latest_period,omitempty"`
	PreviousPeriod     string                           `json:"previous_period,omitempty"`
	PublicationStatus  string                           `json:"publication_status"`
	ReviewRequired     bool                             `json:"review_required"`
	Signals            []validationBriefingSignal       `json:"signals"`
	Email              validationBriefingEmail          `json:"email"`
	SocialCarousel     validationBriefingSocialCarousel `json:"social_carousel"`
	Caveats            []string                         `json:"caveats"`
	EvidenceEntryPoint string                           `json:"evidence_entry_point"`
}

type validationBriefingSignal struct {
	ID               string                          `json:"id"`
	Kind             string                          `json:"kind"`
	Title            string                          `json:"title"`
	Summary          string                          `json:"summary"`
	ReporterISO3     string                          `json:"reporter_iso3"`
	ReporterName     string                          `json:"reporter_name"`
	Classification   string                          `json:"classification,omitempty"`
	Code             string                          `json:"code,omitempty"`
	Label            string                          `json:"label,omitempty"`
	Period           string                          `json:"period"`
	PreviousPeriod   string                          `json:"previous_period"`
	Current          validationBriefingObservedValue `json:"current"`
	Previous         validationBriefingObservedValue `json:"previous"`
	DeltaTradeUSD    float64                         `json:"delta_trade_usd"`
	ChangeRatio      *float64                        `json:"change_ratio,omitempty"`
	ChinaShareDelta  float64                         `json:"china_share_delta"`
	Evidence         []string                        `json:"evidence"`
	Interpretation   string                          `json:"interpretation"`
	MeasurementLimit string                          `json:"measurement_limit"`
}

type validationBriefingObservedValue struct {
	USATradeUSD   float64 `json:"usa_trade_usd"`
	ChinaTradeUSD float64 `json:"china_trade_usd"`
	TotalTradeUSD float64 `json:"total_trade_usd"`
	ChinaShare    float64 `json:"china_share"`
}

type validationBriefingEmail struct {
	Subject     string `json:"subject"`
	Preview     string `json:"preview"`
	Markdown    string `json:"markdown"`
	CTALabel    string `json:"cta_label"`
	CTAPath     string `json:"cta_path"`
	SendPolicy  string `json:"send_policy"`
	PrimaryGoal string `json:"primary_goal"`
}

type validationBriefingSocialCarousel struct {
	Format       string                            `json:"format"`
	AspectRatio  string                            `json:"aspect_ratio"`
	ReviewPolicy string                            `json:"review_policy"`
	Slides       []validationBriefingCarouselSlide `json:"slides"`
}

type validationBriefingCarouselSlide struct {
	Order    int      `json:"order"`
	Role     string   `json:"role"`
	Headline string   `json:"headline"`
	Body     string   `json:"body"`
	Evidence []string `json:"evidence"`
}

type validationQuality struct {
	SchemaVersion      string                         `json:"schema_version"`
	GeneratedAt        string                         `json:"generated_at"`
	PrimaryProvider    string                         `json:"primary_provider"`
	DominantPeriod     string                         `json:"dominant_period"`
	Summary            validationQualitySummary       `json:"summary"`
	ReporterIssues     []validationReporterIssue      `json:"reporter_issues"`
	CollectionRuns     []validationRun                `json:"collection_runs"`
	ProviderComparison []validationProviderComparison `json:"provider_comparison"`
}

type validationQualitySummary struct {
	ReporterCount         int `json:"reporter_count"`
	ComparableReporters   int `json:"comparable_reporters"`
	IncomparableReporters int `json:"incomparable_reporters"`
	MissingPartnerBlocks  int `json:"missing_partner_blocks"`
	StalePartnerBlocks    int `json:"stale_partner_blocks"`
	ComparisonCount       int `json:"provider_comparison_count"`
}

type validationReporterIssue struct {
	ISO3      string   `json:"iso3"`
	USAPeriod string   `json:"usa_period,omitempty"`
	CHNPeriod string   `json:"chn_period,omitempty"`
	Issues    []string `json:"issues"`
}

type validationRun struct {
	RunID         string   `json:"run_id"`
	Provider      string   `json:"provider"`
	Mode          string   `json:"mode"`
	StartedAt     string   `json:"started_at"`
	FinishedAt    string   `json:"finished_at"`
	Status        string   `json:"status"`
	ReporterCount int      `json:"reporter_count"`
	RequestCount  int      `json:"request_count"`
	SuccessCount  int      `json:"success_count"`
	FailureCount  int      `json:"failure_count"`
	SkippedCount  int      `json:"skipped_count"`
	StoredCount   int      `json:"stored_count"`
	Errors        []string `json:"errors"`
}

type validationProviderComparison struct {
	ISO3              string  `json:"iso3"`
	Partner           string  `json:"partner"`
	PeriodType        string  `json:"period_type"`
	Period            string  `json:"period"`
	PrimaryProvider   string  `json:"primary_provider"`
	SecondaryProvider string  `json:"secondary_provider"`
	PrimaryTradeUSD   float64 `json:"primary_trade_usd"`
	SecondaryTradeUSD float64 `json:"secondary_trade_usd"`
	DeltaRatio        float64 `json:"delta_ratio"`
}

type validationContext struct {
	SchemaVersion string                     `json:"schema_version"`
	GeneratedAt   string                     `json:"generated_at"`
	Source        string                     `json:"source"`
	Status        string                     `json:"status"`
	Errors        []string                   `json:"errors,omitempty"`
	Countries     []validationContextCountry `json:"countries"`
}

type validationContextCountry struct {
	ISO3        string        `json:"iso3"`
	ISO2        string        `json:"iso2"`
	Name        string        `json:"name"`
	Region      string        `json:"region"`
	IncomeGroup string        `json:"income_group"`
	Groups      []string      `json:"groups"`
	Population  contextMetric `json:"population"`
	GDP         contextMetric `json:"gdp"`
}

type validationExplanationIndex struct {
	SchemaVersion string   `json:"schema_version"`
	GeneratedAt   string   `json:"generated_at"`
	Reporters     []string `json:"reporters"`
	AICount       int      `json:"ai_count"`
	FallbackCount int      `json:"fallback_count"`
	Model         string   `json:"model"`
}

type validationExplanation struct {
	SchemaVersion string `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	ReporterISO3  string `json:"reporter_iso3"`
	Name          string `json:"name"`
	Generator     struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		Model  string `json:"model"`
	} `json:"generator"`
	Summary    string `json:"summary"`
	Statements []struct {
		Text        string   `json:"text"`
		EvidenceIDs []string `json:"evidence_ids"`
	} `json:"statements"`
	Evidence []struct {
		ID           string  `json:"id"`
		Label        string  `json:"label"`
		Value        float64 `json:"value,omitempty"`
		DisplayValue string  `json:"display_value,omitempty"`
		Unit         string  `json:"unit,omitempty"`
		Period       string  `json:"period,omitempty"`
		Source       string  `json:"source"`
		SourceJSON   string  `json:"source_json"`
	} `json:"evidence"`
}

func validateExtendedDataset(dataDir string, metadata datasetMeta, latest datasetLatest) error {
	if !strings.HasPrefix(metadata.SchemaVersion, "2.") {
		return nil
	}
	var series validationSeries
	if err := readJSON(filepath.Join(dataDir, "series.json"), &series); err != nil {
		return fmt.Errorf("read series.json: %w", err)
	}
	if err := validateSeries(metadata, latest, series); err != nil {
		return err
	}
	var quality validationQuality
	if err := readJSON(filepath.Join(dataDir, "quality.json"), &quality); err != nil {
		return fmt.Errorf("read quality.json: %w", err)
	}
	if err := validateQuality(metadata, quality); err != nil {
		return err
	}
	var products validationProductIndex
	if err := readJSON(filepath.Join(dataDir, "products", "index.json"), &products); err != nil {
		return fmt.Errorf("read product index: %w", err)
	}
	if err := validateProducts(dataDir, metadata, products); err != nil {
		return err
	}
	var strategicIndex validationStrategicIndex
	if err := readJSON(filepath.Join(dataDir, "strategic-hs6", "index.json"), &strategicIndex); err != nil {
		return fmt.Errorf("read strategic HS6 index: %w", err)
	}
	if err := validateStrategic(dataDir, metadata, strategicIndex); err != nil {
		return err
	}
	semiconductorReference, err := semiconductor.Load(filepath.Join(dataDir, "semiconductors", "reference.json"))
	if err != nil {
		return fmt.Errorf("read semiconductor reference: %w", err)
	}
	strategicProducts := make([]strategic.Product, 0, len(strategicIndex.Products))
	for _, product := range strategicIndex.Products {
		strategicProducts = append(strategicProducts, strategic.Product{Code: product.Code, Sector: product.Sector, Label: product.Label, RevisionNote: product.RevisionNote, Notes: product.Notes})
	}
	if err := semiconductor.ValidateStrategicRegistry(semiconductorReference, strategicProducts); err != nil {
		return err
	}
	if err := validateSemiconductor(metadata, semiconductorReference); err != nil {
		return err
	}
	var semiconductorMonthlyIndex validationSemiconductorMonthlyIndex
	if err := readJSON(filepath.Join(dataDir, "semiconductors", "monthly", "index.json"), &semiconductorMonthlyIndex); err != nil {
		return fmt.Errorf("read monthly semiconductor index: %w", err)
	}
	if err := validateSemiconductorMonthly(dataDir, metadata, semiconductorReference, semiconductorMonthlyIndex); err != nil {
		return err
	}
	var publicationChanges validationPublicationChanges
	if err := readJSON(filepath.Join(dataDir, "changes.json"), &publicationChanges); err != nil {
		return fmt.Errorf("read changes.json: %w", err)
	}
	if err := validatePublicationChanges(metadata, semiconductorMonthlyIndex, publicationChanges); err != nil {
		return err
	}
	var briefing validationBriefing
	if err := readJSON(filepath.Join(dataDir, "briefing.json"), &briefing); err != nil {
		return fmt.Errorf("read briefing.json: %w", err)
	}
	if err := validateBriefing(metadata, publicationChanges, briefing); err != nil {
		return err
	}
	var tariffIndex validationTariffIndex
	if err := readJSON(filepath.Join(dataDir, "tariffs", "index.json"), &tariffIndex); err != nil {
		return fmt.Errorf("read tariff index: %w", err)
	}
	if err := validateTariffs(dataDir, metadata, tariffIndex); err != nil {
		return err
	}
	var matrixIndex validationMatrixIndex
	if err := readJSON(filepath.Join(dataDir, "bilateral-matrix", "index.json"), &matrixIndex); err != nil {
		return fmt.Errorf("read bilateral matrix index: %w", err)
	}
	if err := validateMatrix(dataDir, metadata, matrixIndex); err != nil {
		return err
	}
	var mirrorIndex validationMirrorIndex
	if err := readJSON(filepath.Join(dataDir, "mirror", "index.json"), &mirrorIndex); err != nil {
		return fmt.Errorf("read mirror diagnostics index: %w", err)
	}
	if err := validateMirror(dataDir, metadata, mirrorIndex); err != nil {
		return err
	}
	var catalog validationCatalog
	if err := readJSON(filepath.Join(dataDir, "catalog.json"), &catalog); err != nil {
		return fmt.Errorf("read catalog.json: %w", err)
	}
	if err := validateCatalog(metadata, catalog, publicationChanges, briefing); err != nil {
		return err
	}
	if err := validateExplanations(dataDir, metadata, latest); err != nil {
		return err
	}
	var contextData validationContext
	if err := readJSON(filepath.Join(dataDir, "context.json"), &contextData); err != nil {
		return fmt.Errorf("read context.json: %w", err)
	}
	return validateContext(metadata, latest, contextData)
}

func validateCatalog(metadata datasetMeta, catalog validationCatalog, publicationChanges validationPublicationChanges, briefing validationBriefing) error {
	if catalog.SchemaVersion != "1.0" || catalog.GeneratedAt != metadata.GeneratedAt {
		return errorsForExtended("catalog provenance does not match metadata")
	}
	seen := make(map[string]validationCatalogResource, len(catalog.Resources))
	for _, resource := range catalog.Resources {
		if strings.TrimSpace(resource.ID) == "" || strings.TrimSpace(resource.Title) == "" || strings.TrimSpace(resource.Grain) == "" || strings.TrimSpace(resource.Partitioning) == "" {
			return fmt.Errorf("catalog resource is incomplete: %+v", resource)
		}
		if _, exists := seen[resource.ID]; exists {
			return fmt.Errorf("catalog has duplicate resource %q", resource.ID)
		}
		if resource.Status != "ready" && resource.Status != "partial" && resource.Status != "planned" {
			return fmt.Errorf("catalog resource %s has invalid status %q", resource.ID, resource.Status)
		}
		if resource.Status == "planned" && resource.Href != "" {
			return fmt.Errorf("planned catalog resource %s must not claim a published href", resource.ID)
		}
		if resource.Status != "planned" && (!strings.HasPrefix(resource.Href, "./") || strings.Contains(resource.Href, "..")) {
			return fmt.Errorf("published catalog resource %s has invalid relative href %q", resource.ID, resource.Href)
		}
		seen[resource.ID] = resource
	}
	for _, required := range []string{"headline_totals", "time_series", "country_context", "product_chapters", "quality", "strategic_hs6", "tariff_schedules", "bilateral_matrix", "semiconductor_atlas", "semiconductor_monthly", "publication_changes", "distribution_briefing", "mirror_reconciliation", "scenario_runs"} {
		if _, ok := seen[required]; !ok {
			return fmt.Errorf("catalog is missing resource %q", required)
		}
	}
	products := seen["product_chapters"]
	if products.Provider != metadata.ProductProvider || products.Classification != metadata.ProductClassification || products.ProductLevel != metadata.ProductLevel {
		return errorsForExtended("catalog product resource does not match metadata")
	}
	strategicResource := seen["strategic_hs6"]
	wantStrategicStatus := "partial"
	if metadata.StrategicPartitionCount > 0 {
		wantStrategicStatus = "ready"
	}
	if strategicResource.Status != wantStrategicStatus || strategicResource.Provider != metadata.StrategicProvider || strategicResource.ProductLevel != 6 || strategicResource.Href != "./strategic-hs6/index.json" {
		return errorsForExtended("catalog strategic resource does not match metadata")
	}
	tariffResource := seen["tariff_schedules"]
	wantTariffStatus := "partial"
	if metadata.TariffPartitionCount > 0 {
		wantTariffStatus = "ready"
	}
	if tariffResource.Status != wantTariffStatus || tariffResource.Provider != metadata.TariffProvider || tariffResource.ProductLevel != 6 || tariffResource.Href != "./tariffs/index.json" {
		return errorsForExtended("catalog tariff resource does not match metadata")
	}
	matrixResource := seen["bilateral_matrix"]
	wantMatrixStatus := "partial"
	if metadata.MatrixPartitionCount > 0 {
		wantMatrixStatus = "ready"
	}
	if matrixResource.Status != wantMatrixStatus || matrixResource.Provider != metadata.MatrixProvider || matrixResource.ProductLevel != 0 || matrixResource.Href != "./bilateral-matrix/index.json" {
		return errorsForExtended("catalog bilateral matrix resource does not match metadata")
	}
	mirrorResource := seen["mirror_reconciliation"]
	wantMirrorStatus := "partial"
	if metadata.MirrorPartitionCount > 0 {
		wantMirrorStatus = "ready"
	}
	if mirrorResource.Status != wantMirrorStatus || mirrorResource.Provider != metadata.MirrorProvider || mirrorResource.ProductLevel != 0 || mirrorResource.Href != "./mirror/index.json" {
		return errorsForExtended("catalog mirror diagnostics resource does not match metadata")
	}
	monthlyResource := seen["semiconductor_monthly"]
	wantMonthlyStatus := "partial"
	if metadata.SemiconductorMonthlyReporterCount > 0 {
		wantMonthlyStatus = "ready"
	}
	if monthlyResource.Status != wantMonthlyStatus || monthlyResource.Provider != metadata.SemiconductorMonthlyProvider || monthlyResource.ProductLevel != 6 || monthlyResource.Href != "./semiconductors/monthly/index.json" {
		return errorsForExtended("catalog monthly semiconductor resource does not match metadata")
	}
	changesResource := seen["publication_changes"]
	wantChangesStatus := "partial"
	if publicationChanges.Status == "changed" || publicationChanges.Status == "unchanged" {
		wantChangesStatus = "ready"
	}
	if changesResource.Status != wantChangesStatus || changesResource.Provider != "tradegravity" || changesResource.ProductLevel != 6 || changesResource.Href != "./changes.json" {
		return errorsForExtended("catalog publication change resource does not match changes.json")
	}
	briefingResource := seen["distribution_briefing"]
	wantBriefingStatus := "partial"
	if briefing.Status == "ready" {
		wantBriefingStatus = "ready"
	}
	if briefingResource.Status != wantBriefingStatus || briefingResource.Provider != "tradegravity" || briefingResource.ProductLevel != 6 || briefingResource.Href != "./briefing.json" {
		return errorsForExtended("catalog distribution briefing resource does not match briefing.json")
	}
	return nil
}

func validatePublicationChanges(metadata datasetMeta, monthly validationSemiconductorMonthlyIndex, changes validationPublicationChanges) error {
	if changes.SchemaVersion != "1.0" || changes.GeneratedAt != metadata.GeneratedAt || strings.TrimSpace(changes.Scope) == "" {
		return errorsForExtended("publication change provenance does not match metadata")
	}
	if changes.Status != "baseline" && changes.Status != "unchanged" && changes.Status != "changed" {
		return fmt.Errorf("publication changes has invalid status %q", changes.Status)
	}
	if !reflect.DeepEqual(changes.CurrentPeriods, monthly.Periods) || !reflect.DeepEqual(changes.CurrentReporters, monthly.Reporters) || changes.Summary.CurrentObservationCount != monthly.ObservationCount {
		return errorsForExtended("publication changes does not match the current monthly index")
	}
	if !sortedUnique(changes.CurrentPeriods) || !sortedUnique(changes.NewPeriods) || !sortedUnique(changes.RemovedPeriods) || !sortedUnique(changes.CurrentReporters) || !sortedUnique(changes.NewReporters) || !sortedUnique(changes.RemovedReporters) {
		return errorsForExtended("publication change dimensions must be sorted and unique")
	}
	for _, period := range append(append(append([]string{}, changes.CurrentPeriods...), changes.NewPeriods...), changes.RemovedPeriods...) {
		if !monthPattern.MatchString(period) {
			return fmt.Errorf("publication changes has invalid month %q", period)
		}
	}
	for _, reporter := range append(append(append([]string{}, changes.CurrentReporters...), changes.NewReporters...), changes.RemovedReporters...) {
		if !iso3Pattern.MatchString(reporter) {
			return fmt.Errorf("publication changes has invalid reporter %q", reporter)
		}
	}
	if changes.Summary.CurrentObservationCount < 0 || changes.Summary.PreviousObservationCount < 0 || changes.Summary.AddedRows < 0 || changes.Summary.RemovedRows < 0 || changes.Summary.RevisedRows < 0 {
		return errorsForExtended("publication change counts must not be negative")
	}
	if changes.Status == "baseline" {
		if changes.PreviousGeneratedAt != "" || changes.Summary.PreviousObservationCount != 0 || changes.Summary.ObservationDelta != 0 || changes.Summary.AddedRows != 0 || changes.Summary.RemovedRows != 0 || changes.Summary.RevisedRows != 0 || len(changes.NewPeriods)+len(changes.RemovedPeriods)+len(changes.NewReporters)+len(changes.RemovedReporters)+len(changes.TopRevisions) != 0 {
			return errorsForExtended("baseline publication changes must not claim a previous comparison")
		}
	} else {
		if _, err := time.Parse(time.RFC3339, changes.PreviousGeneratedAt); err != nil {
			return fmt.Errorf("publication changes has invalid previous_generated_at: %w", err)
		}
		if changes.Summary.ObservationDelta != changes.Summary.CurrentObservationCount-changes.Summary.PreviousObservationCount {
			return errorsForExtended("publication observation delta is inconsistent")
		}
		claimsChange := changes.Summary.ObservationDelta != 0 || changes.Summary.AddedRows > 0 || changes.Summary.RemovedRows > 0 || changes.Summary.RevisedRows > 0 || len(changes.NewPeriods)+len(changes.RemovedPeriods)+len(changes.NewReporters)+len(changes.RemovedReporters) > 0
		if (changes.Status == "changed") != claimsChange {
			return errorsForExtended("publication change status does not match its summary")
		}
	}
	if len(changes.TopRevisions) > 20 || len(changes.TopRevisions) > changes.Summary.RevisedRows {
		return errorsForExtended("publication change revision list is not bounded by its summary")
	}
	previousMagnitude := math.Inf(1)
	for _, revision := range changes.TopRevisions {
		if !iso3Pattern.MatchString(revision.ReporterISO3) || !monthPattern.MatchString(revision.Period) || strings.TrimSpace(revision.Classification) == "" || !hs6Pattern.MatchString(revision.Code) || strings.TrimSpace(revision.Label) == "" {
			return fmt.Errorf("publication changes has incomplete revision %+v", revision)
		}
		values := []float64{revision.PreviousUSATradeUSD, revision.CurrentUSATradeUSD, revision.PreviousChinaTradeUSD, revision.CurrentChinaTradeUSD, revision.PreviousTotalUSD, revision.CurrentTotalUSD, revision.MagnitudeTradeUSD}
		for _, value := range values {
			if !isFinite(value) || value < 0 {
				return fmt.Errorf("publication revision has invalid nonnegative value %+v", revision)
			}
		}
		if !isFinite(revision.DeltaTradeUSD) || !approximatelyEqual(revision.PreviousTotalUSD, revision.PreviousUSATradeUSD+revision.PreviousChinaTradeUSD) || !approximatelyEqual(revision.CurrentTotalUSD, revision.CurrentUSATradeUSD+revision.CurrentChinaTradeUSD) || !approximatelyEqual(revision.DeltaTradeUSD, revision.CurrentTotalUSD-revision.PreviousTotalUSD) || !approximatelyEqual(revision.MagnitudeTradeUSD, math.Abs(revision.CurrentUSATradeUSD-revision.PreviousUSATradeUSD)+math.Abs(revision.CurrentChinaTradeUSD-revision.PreviousChinaTradeUSD)) {
			return fmt.Errorf("publication revision totals are inconsistent %+v", revision)
		}
		if revision.ChangeRatio != nil && (!isFinite(*revision.ChangeRatio) || revision.PreviousTotalUSD <= 0 || !approximatelyEqual(*revision.ChangeRatio, revision.DeltaTradeUSD/revision.PreviousTotalUSD)) {
			return fmt.Errorf("publication revision ratio is inconsistent %+v", revision)
		}
		if revision.MagnitudeTradeUSD > previousMagnitude {
			return errorsForExtended("publication revisions must be ordered by descending magnitude")
		}
		previousMagnitude = revision.MagnitudeTradeUSD
	}
	return nil
}

func validateBriefing(metadata datasetMeta, changes validationPublicationChanges, briefing validationBriefing) error {
	if briefing.SchemaVersion != "1.0" || briefing.GeneratedAt != metadata.GeneratedAt || strings.TrimSpace(briefing.EditionID) == "" || strings.TrimSpace(briefing.Title) == "" || strings.TrimSpace(briefing.Scope) == "" {
		return errorsForExtended("briefing provenance does not match metadata")
	}
	if briefing.Status != "ready" && briefing.Status != "unavailable" {
		return fmt.Errorf("briefing has invalid status %q", briefing.Status)
	}
	if briefing.PublicationStatus != changes.Status || !briefing.ReviewRequired || len(briefing.Caveats) < 3 || !validBriefingHref(briefing.EvidenceEntryPoint) {
		return errorsForExtended("briefing does not preserve publication status, review, caveat, or evidence requirements")
	}
	for _, caveat := range briefing.Caveats {
		if strings.TrimSpace(caveat) == "" {
			return errorsForExtended("briefing caveats must not be blank")
		}
	}
	email := briefing.Email
	if strings.TrimSpace(email.Subject) == "" || strings.TrimSpace(email.Preview) == "" || strings.TrimSpace(email.Markdown) == "" || strings.TrimSpace(email.CTALabel) == "" || !validBriefingHref(email.CTAPath) || email.SendPolicy != "manual_review_required" || strings.TrimSpace(email.PrimaryGoal) == "" {
		return errorsForExtended("briefing email draft is incomplete or not review-gated")
	}
	carousel := briefing.SocialCarousel
	if carousel.Format != "carousel_copy" || carousel.AspectRatio != "4:5" || carousel.ReviewPolicy != "manual_review_required" {
		return errorsForExtended("briefing carousel contract is invalid")
	}
	if briefing.Status == "unavailable" {
		if len(briefing.Signals) != 0 || len(carousel.Slides) != 0 {
			return errorsForExtended("unavailable briefing must not publish signals or carousel slides")
		}
		return nil
	}
	if !monthPattern.MatchString(briefing.LatestPeriod) || !monthPattern.MatchString(briefing.PreviousPeriod) || briefing.LatestPeriod <= briefing.PreviousPeriod || !strings.Contains(email.Markdown, "{{BASE_URL}}") {
		return errorsForExtended("ready briefing has invalid periods or unresolved delivery template")
	}
	wantKinds := []string{"reporter_total_change", "anchor_share_shift", "product_total_change"}
	if len(briefing.Signals) != len(wantKinds) {
		return fmt.Errorf("ready briefing has %d signals, want %d", len(briefing.Signals), len(wantKinds))
	}
	seenIDs := make(map[string]struct{}, len(briefing.Signals))
	for index, signal := range briefing.Signals {
		if signal.Kind != wantKinds[index] || strings.TrimSpace(signal.ID) == "" || strings.TrimSpace(signal.Title) == "" || strings.TrimSpace(signal.Summary) == "" || !iso3Pattern.MatchString(signal.ReporterISO3) || strings.TrimSpace(signal.ReporterName) == "" || !monthPattern.MatchString(signal.Period) || !monthPattern.MatchString(signal.PreviousPeriod) || signal.Period <= signal.PreviousPeriod || strings.TrimSpace(signal.Interpretation) == "" || strings.TrimSpace(signal.MeasurementLimit) == "" {
			return fmt.Errorf("briefing has incomplete signal %+v", signal)
		}
		if _, exists := seenIDs[signal.ID]; exists {
			return fmt.Errorf("briefing repeats signal id %q", signal.ID)
		}
		seenIDs[signal.ID] = struct{}{}
		if signal.Kind == "product_total_change" {
			if strings.TrimSpace(signal.Classification) == "" || !hs6Pattern.MatchString(signal.Code) || strings.TrimSpace(signal.Label) == "" {
				return fmt.Errorf("briefing product signal is incomplete %+v", signal)
			}
		} else if signal.Classification != "" || signal.Code != "" || signal.Label != "" {
			return fmt.Errorf("briefing aggregate signal claims product identity %+v", signal)
		}
		if err := validateBriefingObservedValue(signal.Current); err != nil {
			return fmt.Errorf("briefing signal %s current value: %w", signal.ID, err)
		}
		if err := validateBriefingObservedValue(signal.Previous); err != nil {
			return fmt.Errorf("briefing signal %s previous value: %w", signal.ID, err)
		}
		if !isFinite(signal.DeltaTradeUSD) || !approximatelyEqual(signal.DeltaTradeUSD, signal.Current.TotalTradeUSD-signal.Previous.TotalTradeUSD) || !isFinite(signal.ChinaShareDelta) || !approximatelyEqual(signal.ChinaShareDelta, signal.Current.ChinaShare-signal.Previous.ChinaShare) {
			return fmt.Errorf("briefing signal %s has inconsistent deltas", signal.ID)
		}
		if signal.Previous.TotalTradeUSD > 0 {
			if signal.ChangeRatio == nil || !isFinite(*signal.ChangeRatio) || !approximatelyEqual(*signal.ChangeRatio, signal.DeltaTradeUSD/signal.Previous.TotalTradeUSD) {
				return fmt.Errorf("briefing signal %s has inconsistent change ratio", signal.ID)
			}
		} else if signal.ChangeRatio != nil {
			return fmt.Errorf("briefing signal %s must omit a ratio with a zero baseline", signal.ID)
		}
		if len(signal.Evidence) < 2 {
			return fmt.Errorf("briefing signal %s has insufficient evidence links", signal.ID)
		}
		for _, href := range signal.Evidence {
			if !validBriefingHref(href) {
				return fmt.Errorf("briefing signal %s has invalid evidence href %q", signal.ID, href)
			}
		}
	}
	if briefing.Signals[0].Period != briefing.LatestPeriod || briefing.Signals[0].PreviousPeriod != briefing.PreviousPeriod {
		return errorsForExtended("briefing edition periods do not match the leading signal")
	}
	wantRoles := []string{"cover", "scale", "anchor_balance", "product", "method", "cta"}
	if len(carousel.Slides) != len(wantRoles) {
		return fmt.Errorf("briefing carousel has %d slides, want %d", len(carousel.Slides), len(wantRoles))
	}
	for index, slide := range carousel.Slides {
		if slide.Order != index+1 || slide.Role != wantRoles[index] || strings.TrimSpace(slide.Headline) == "" || strings.TrimSpace(slide.Body) == "" || len(slide.Evidence) == 0 {
			return fmt.Errorf("briefing carousel has invalid slide %+v", slide)
		}
		for _, href := range slide.Evidence {
			if !validBriefingHref(href) {
				return fmt.Errorf("briefing carousel slide %d has invalid evidence href %q", slide.Order, href)
			}
		}
	}
	return nil
}

func validateBriefingObservedValue(value validationBriefingObservedValue) error {
	values := []float64{value.USATradeUSD, value.ChinaTradeUSD, value.TotalTradeUSD, value.ChinaShare}
	for _, item := range values {
		if !isFinite(item) || item < 0 {
			return fmt.Errorf("contains invalid nonnegative value %v", item)
		}
	}
	if value.ChinaShare > 1 || !approximatelyEqual(value.TotalTradeUSD, value.USATradeUSD+value.ChinaTradeUSD) || (value.TotalTradeUSD > 0 && !approximatelyEqual(value.ChinaShare, value.ChinaTradeUSD/value.TotalTradeUSD)) || (value.TotalTradeUSD == 0 && value.ChinaShare != 0) {
		return errorsForExtended("totals or China share are inconsistent")
	}
	return nil
}

func validBriefingHref(href string) bool {
	return strings.HasPrefix(href, "./") && !strings.Contains(href, "..") && !strings.ContainsAny(href, "\r\n")
}

func sortedUnique(values []string) bool {
	if !sort.StringsAreSorted(values) {
		return false
	}
	for index := 1; index < len(values); index++ {
		if values[index] == values[index-1] {
			return false
		}
	}
	return true
}

func validateSemiconductor(metadata datasetMeta, reference semiconductor.Reference) error {
	publication := reference.Publication
	if reference.GeneratedAt != metadata.GeneratedAt {
		return fmt.Errorf("semiconductor generated_at mismatch: meta=%q reference=%q", metadata.GeneratedAt, reference.GeneratedAt)
	}
	if publication.Status != "reference_only" && publication.Status != "limited" && publication.Status != "research_ready" {
		return fmt.Errorf("semiconductor publication has invalid status %q", publication.Status)
	}
	if publication.RegisteredCodeCount != len(semiconductor.Codes(reference)) || publication.RegisteredCodeCount < 30 {
		return fmt.Errorf("semiconductor registered code count mismatch or below minimum: publication=%d mapped=%d", publication.RegisteredCodeCount, len(semiconductor.Codes(reference)))
	}
	if metadata.SemiconductorStatus != publication.Status || metadata.SemiconductorCodeCount != publication.RegisteredCodeCount || metadata.SemiconductorReporterCount != publication.ObservedReporterCount || metadata.SemiconductorPeriodCount != publication.ObservedPeriodCount {
		return errorsForExtended("semiconductor metadata does not match reference publication")
	}
	if publication.Status == "research_ready" && (publication.ObservedReporterCount < publication.MinimumReporterTarget || publication.ObservedPeriodCount < publication.MinimumPeriodTarget || publication.RegisteredCodeCount < publication.MinimumCodeTarget) {
		return errorsForExtended("semiconductor publication claims research_ready below its coverage gate")
	}
	return nil
}

func validateSemiconductorMonthly(dataDir string, metadata datasetMeta, reference semiconductor.Reference, index validationSemiconductorMonthlyIndex) error {
	if index.SchemaVersion != metadata.SchemaVersion || index.GeneratedAt != metadata.GeneratedAt || index.Provider != metadata.SemiconductorMonthlyProvider || index.Level != 6 || !reflect.DeepEqual(index.Partners, metadata.Partners) || strings.TrimSpace(index.Scope) == "" {
		return errorsForExtended("monthly semiconductor index does not match metadata")
	}
	if len(index.Reporters) != metadata.SemiconductorMonthlyReporterCount || len(index.Periods) != metadata.SemiconductorMonthlyPeriodCount || index.ObservationCount != metadata.SemiconductorMonthlyObservationCount || len(index.Partitions) != len(index.Reporters) {
		return errorsForExtended("monthly semiconductor counts do not match metadata")
	}
	if !sort.StringsAreSorted(index.Reporters) || !sort.StringsAreSorted(index.Periods) {
		return errorsForExtended("monthly semiconductor dimensions must be sorted")
	}
	codeSet := make(map[string]struct{})
	for _, code := range semiconductor.Codes(reference) {
		codeSet[code] = struct{}{}
	}
	for _, period := range index.Periods {
		if !monthPattern.MatchString(period) {
			return fmt.Errorf("monthly semiconductor index has invalid period %q", period)
		}
	}
	reporterSet := make(map[string]struct{})
	periodSet := make(map[string]struct{})
	for _, partition := range index.Partitions {
		if !iso3Pattern.MatchString(partition.ReporterISO3) || partition.Href != "./"+partition.ReporterISO3+".json" || partition.RowCount < 1 || partition.PeriodCount < 1 {
			return fmt.Errorf("monthly semiconductor index has invalid partition %+v", partition)
		}
		if _, exists := reporterSet[partition.ReporterISO3]; exists {
			return fmt.Errorf("monthly semiconductor index repeats reporter %s", partition.ReporterISO3)
		}
		reporterSet[partition.ReporterISO3] = struct{}{}
		var file validationSemiconductorMonthlyFile
		if err := readJSON(filepath.Join(dataDir, "semiconductors", "monthly", partition.ReporterISO3+".json"), &file); err != nil {
			return fmt.Errorf("read monthly semiconductor partition %s: %w", partition.ReporterISO3, err)
		}
		if file.SchemaVersion != index.SchemaVersion || file.GeneratedAt != index.GeneratedAt || file.Provider != index.Provider || file.Level != 6 || !reflect.DeepEqual(file.Partners, index.Partners) || file.ReporterISO3 != partition.ReporterISO3 || len(file.Rows) != partition.RowCount || len(file.Periods) != partition.PeriodCount || !sort.StringsAreSorted(file.Periods) {
			return fmt.Errorf("monthly semiconductor partition %s does not match its index", partition.ReporterISO3)
		}
		filePeriods := make(map[string]struct{})
		previousPeriod := ""
		for _, row := range file.Rows {
			if !monthPattern.MatchString(row.Period) || strings.TrimSpace(row.Classification) == "" || strings.TrimSpace(row.Label) == "" {
				return fmt.Errorf("monthly semiconductor partition %s has incomplete row %+v", partition.ReporterISO3, row)
			}
			if _, ok := codeSet[row.Code]; !ok {
				return fmt.Errorf("monthly semiconductor partition %s has unmapped code %s", partition.ReporterISO3, row.Code)
			}
			if previousPeriod != "" && row.Period < previousPeriod {
				return fmt.Errorf("monthly semiconductor partition %s periods are not ascending", partition.ReporterISO3)
			}
			previousPeriod = row.Period
			if err := validateSeriesBlock(partition.ReporterISO3, "USA", row.USA); err != nil {
				return fmt.Errorf("monthly semiconductor partition %s USA block: %w", partition.ReporterISO3, err)
			}
			if err := validateSeriesBlock(partition.ReporterISO3, "CHN", row.CHN); err != nil {
				return fmt.Errorf("monthly semiconductor partition %s CHN block: %w", partition.ReporterISO3, err)
			}
			if !approximatelyEqual(row.Total, row.USA.Trade+row.CHN.Trade) || row.Total < 0 || !isFinite(row.ShareCN) || row.ShareCN < 0 || row.ShareCN > 1 || (row.Total > 0 && !approximatelyEqual(row.ShareCN, row.CHN.Trade/row.Total)) {
				return fmt.Errorf("monthly semiconductor partition %s has inconsistent totals %+v", partition.ReporterISO3, row)
			}
			filePeriods[row.Period] = struct{}{}
			periodSet[row.Period] = struct{}{}
		}
		if !sameStringSet(file.Periods, filePeriods) {
			return fmt.Errorf("monthly semiconductor partition %s period discovery mismatch", partition.ReporterISO3)
		}
	}
	if index.ObservationCount < 0 || !sameStringSet(index.Reporters, reporterSet) || !sameStringSet(index.Periods, periodSet) {
		return errorsForExtended("monthly semiconductor partition discovery does not match index")
	}
	return nil
}

func validateStrategic(dataDir string, metadata datasetMeta, index validationStrategicIndex) error {
	if index.SchemaVersion != metadata.SchemaVersion || index.GeneratedAt != metadata.GeneratedAt || index.Provider != metadata.StrategicProvider || index.Level != 6 || !reflect.DeepEqual(index.Partners, metadata.Partners) {
		return errorsForExtended("strategic HS6 index does not match metadata")
	}
	if index.ObservationCount != metadata.StrategicObservationCount || len(index.Products) != metadata.StrategicProductCount || len(index.Reporters) != metadata.StrategicReporterCount || len(index.Partitions) != metadata.StrategicPartitionCount {
		return errorsForExtended("strategic HS6 index counts do not match metadata")
	}
	if !sort.StringsAreSorted(index.Reporters) {
		return errorsForExtended("strategic HS6 reporters must be sorted")
	}
	for _, period := range index.Periods {
		if !yearPattern.MatchString(period) {
			return fmt.Errorf("strategic HS6 index has invalid period %q", period)
		}
	}
	productByCode := make(map[string]validationStrategicProductDescriptor, len(index.Products))
	sectorSet := make(map[string]struct{})
	for _, product := range index.Products {
		if !regexp.MustCompile(`^\d{6}$`).MatchString(product.Code) || strings.TrimSpace(product.Sector) == "" || strings.TrimSpace(product.Label) == "" || strings.TrimSpace(product.RevisionNote) == "" {
			return fmt.Errorf("strategic HS6 index has invalid product %+v", product)
		}
		if _, exists := productByCode[product.Code]; exists {
			return fmt.Errorf("strategic HS6 index has duplicate product %s", product.Code)
		}
		productByCode[product.Code] = product
		sectorSet[product.Sector] = struct{}{}
	}
	if len(productByCode) == 0 {
		return errorsForExtended("strategic HS6 registry is empty")
	}
	if len(index.Sectors) != len(sectorSet) || !sort.StringsAreSorted(index.Sectors) {
		return errorsForExtended("strategic HS6 sectors are incomplete or unsorted")
	}
	for _, sector := range index.Sectors {
		if _, ok := sectorSet[sector]; !ok {
			return fmt.Errorf("strategic HS6 index has unknown sector %q", sector)
		}
	}

	partitionSet := make(map[string]struct{}, len(index.Partitions))
	reporterSet := make(map[string]struct{})
	periodSet := make(map[string]struct{})
	for _, partition := range index.Partitions {
		if !iso3Pattern.MatchString(partition.ReporterISO3) || !yearPattern.MatchString(partition.Period) || partition.RowCount < 1 {
			return fmt.Errorf("strategic HS6 index has invalid partition %+v", partition)
		}
		wantHref := "./" + partition.ReporterISO3 + "/" + partition.Period + ".json"
		if partition.Href != wantHref {
			return fmt.Errorf("strategic HS6 partition href %q, want %q", partition.Href, wantHref)
		}
		key := partition.ReporterISO3 + ":" + partition.Period
		if _, exists := partitionSet[key]; exists {
			return fmt.Errorf("strategic HS6 index has duplicate partition %s", key)
		}
		partitionSet[key] = struct{}{}
		reporterSet[partition.ReporterISO3] = struct{}{}
		periodSet[partition.Period] = struct{}{}

		var file validationStrategicFile
		if err := readJSON(filepath.Join(dataDir, "strategic-hs6", partition.ReporterISO3, partition.Period+".json"), &file); err != nil {
			return fmt.Errorf("read strategic HS6 partition %s: %w", key, err)
		}
		if file.SchemaVersion != index.SchemaVersion || file.GeneratedAt != index.GeneratedAt || file.Provider != index.Provider || file.Level != 6 || !reflect.DeepEqual(file.Partners, index.Partners) || file.ReporterISO3 != partition.ReporterISO3 || file.Period != partition.Period || len(file.Rows) != partition.RowCount {
			return fmt.Errorf("strategic HS6 partition %s does not match its index", key)
		}
		seenRows := make(map[string]struct{}, len(file.Rows))
		for _, row := range file.Rows {
			descriptor, ok := productByCode[row.Code]
			if !ok || row.Sector != descriptor.Sector || row.Label != descriptor.Label || row.RevisionNote != descriptor.RevisionNote || strings.TrimSpace(row.Classification) == "" {
				return fmt.Errorf("strategic HS6 partition %s has unregistered row %+v", key, row)
			}
			rowKey := row.Classification + ":" + row.Code
			if _, exists := seenRows[rowKey]; exists {
				return fmt.Errorf("strategic HS6 partition %s has duplicate row %s", key, rowKey)
			}
			seenRows[rowKey] = struct{}{}
			if err := validateSeriesBlock(partition.ReporterISO3, "USA strategic product", row.USA); err != nil {
				return err
			}
			if err := validateSeriesBlock(partition.ReporterISO3, "CHN strategic product", row.CHN); err != nil {
				return err
			}
			if !approximatelyEqual(row.Total, row.USA.Trade+row.CHN.Trade) {
				return fmt.Errorf("strategic HS6 product %s has inconsistent total", row.Code)
			}
			wantShare := 0.0
			if row.Total > 0 {
				wantShare = row.CHN.Trade / row.Total
			}
			if !approximatelyEqual(row.ShareCN, wantShare) {
				return fmt.Errorf("strategic HS6 product %s has inconsistent share", row.Code)
			}
		}
	}
	if len(reporterSet) != len(index.Reporters) || len(periodSet) != len(index.Periods) {
		return errorsForExtended("strategic HS6 reporter or period discovery does not match partitions")
	}
	for _, reporter := range index.Reporters {
		if _, ok := reporterSet[reporter]; !ok {
			return fmt.Errorf("strategic HS6 reporter %s has no partition", reporter)
		}
	}
	for _, period := range index.Periods {
		if _, ok := periodSet[period]; !ok {
			return fmt.Errorf("strategic HS6 period %s has no partition", period)
		}
	}
	return nil
}

func validateTariffs(dataDir string, metadata datasetMeta, index validationTariffIndex) error {
	if index.SchemaVersion != metadata.SchemaVersion || index.GeneratedAt != metadata.GeneratedAt || index.Provider != metadata.TariffProvider || index.Level != 6 {
		return errorsForExtended("tariff index does not match metadata")
	}
	if index.ObservationCount != metadata.TariffObservationCount || len(index.Importers) != metadata.TariffImporterCount || len(index.Partitions) != metadata.TariffPartitionCount {
		return errorsForExtended("tariff index counts do not match metadata")
	}
	if !sort.StringsAreSorted(index.Importers) || !sort.StringsAreSorted(index.Exporters) || !sort.StringsAreSorted(index.DataTypes) || !sort.StringsAreSorted(index.RateTypes) {
		return errorsForExtended("tariff index dimensions must be sorted")
	}
	for position, year := range index.Years {
		if !yearPattern.MatchString(year) || (position > 0 && index.Years[position-1] < year) {
			return fmt.Errorf("tariff index has invalid or unsorted year %q", year)
		}
	}
	for _, importer := range index.Importers {
		if !iso3Pattern.MatchString(importer) {
			return fmt.Errorf("tariff index has invalid importer %q", importer)
		}
	}
	for _, exporter := range index.Exporters {
		if !iso3Pattern.MatchString(exporter) {
			return fmt.Errorf("tariff index has invalid exporter %q", exporter)
		}
	}
	for _, dataType := range index.DataTypes {
		if dataType != "reported" && dataType != "ave_estimated" {
			return fmt.Errorf("tariff index has unsupported data type %q", dataType)
		}
	}
	for _, rateType := range index.RateTypes {
		if !validTariffRateType(rateType) {
			return fmt.Errorf("tariff index has unsupported rate type %q", rateType)
		}
	}
	productByCode := make(map[string]validationStrategicProductDescriptor, len(index.Products))
	for _, product := range index.Products {
		if !hs6Pattern.MatchString(product.Code) || product.Sector == "" || product.Label == "" || product.RevisionNote == "" {
			return fmt.Errorf("tariff index has invalid product %+v", product)
		}
		if _, exists := productByCode[product.Code]; exists {
			return fmt.Errorf("tariff index has duplicate product %s", product.Code)
		}
		productByCode[product.Code] = product
	}
	if len(productByCode) == 0 {
		return errorsForExtended("tariff product registry is empty")
	}

	partitionSet := make(map[string]struct{}, len(index.Partitions))
	importerSet := make(map[string]struct{})
	yearSet := make(map[string]struct{})
	exporterSet := make(map[string]struct{})
	dataTypeSet := make(map[string]struct{})
	rateTypeSet := make(map[string]struct{})
	observationCount := 0
	for _, partition := range index.Partitions {
		if !iso3Pattern.MatchString(partition.ImporterISO3) || !yearPattern.MatchString(partition.Year) || partition.RowCount < 1 {
			return fmt.Errorf("tariff index has invalid partition %+v", partition)
		}
		wantHref := "./" + partition.ImporterISO3 + "/" + partition.Year + ".json"
		if partition.Href != wantHref {
			return fmt.Errorf("tariff partition href %q, want %q", partition.Href, wantHref)
		}
		key := partition.ImporterISO3 + ":" + partition.Year
		if _, exists := partitionSet[key]; exists {
			return fmt.Errorf("tariff index has duplicate partition %s", key)
		}
		partitionSet[key] = struct{}{}
		importerSet[partition.ImporterISO3] = struct{}{}
		yearSet[partition.Year] = struct{}{}

		var file validationTariffFile
		if err := readJSON(filepath.Join(dataDir, "tariffs", partition.ImporterISO3, partition.Year+".json"), &file); err != nil {
			return fmt.Errorf("read tariff partition %s: %w", key, err)
		}
		if file.SchemaVersion != index.SchemaVersion || file.GeneratedAt != index.GeneratedAt || file.Provider != index.Provider || file.Level != 6 || file.ImporterISO3 != partition.ImporterISO3 || file.Year != partition.Year || len(file.Rows) != partition.RowCount {
			return fmt.Errorf("tariff partition %s does not match its index", key)
		}
		seenRows := make(map[string]struct{}, len(file.Rows))
		for _, row := range file.Rows {
			descriptor, ok := productByCode[row.Code]
			if !ok || row.Sector != descriptor.Sector || row.Label != descriptor.Label || strings.TrimSpace(row.Classification) == "" || strings.TrimSpace(row.Nomenclature) == "" {
				return fmt.Errorf("tariff partition %s has unregistered row %+v", key, row)
			}
			if !iso3Pattern.MatchString(row.ExporterISO3) || (row.ExporterCode != "" && len(row.ExporterCode) != 3) {
				return fmt.Errorf("tariff partition %s has invalid exporter %+v", key, row)
			}
			if row.DataType != "reported" && row.DataType != "ave_estimated" {
				return fmt.Errorf("tariff partition %s has invalid data type %q", key, row.DataType)
			}
			if !validTariffRateType(row.RateType) || strings.TrimSpace(row.Regime) == "" {
				return fmt.Errorf("tariff partition %s has invalid rate identity %+v", key, row)
			}
			rowKey := strings.Join([]string{row.Classification, row.Code, row.ExporterISO3, row.DataType, row.RateType, row.Regime}, ":")
			if _, exists := seenRows[rowKey]; exists {
				return fmt.Errorf("tariff partition %s has duplicate row %s", key, rowKey)
			}
			seenRows[rowKey] = struct{}{}
			for label, value := range map[string]*float64{"rate": &row.RatePercent, "sum": row.SumRatePercent, "minimum": row.MinRatePercent, "maximum": row.MaxRatePercent} {
				if value != nil && (!isFinite(*value) || *value < 0) {
					return fmt.Errorf("tariff partition %s has invalid %s rate %v", key, label, *value)
				}
			}
			if row.TotalLines < 0 || row.PreferentialLines < 0 || row.MFNLines < 0 || row.NonAdValoremLines < 0 {
				return fmt.Errorf("tariff partition %s has negative line counts", key)
			}
			if row.SourceUpdatedAt != "" {
				if _, err := time.Parse(time.RFC3339, row.SourceUpdatedAt); err != nil {
					return fmt.Errorf("tariff partition %s has invalid source_updated_at %q", key, row.SourceUpdatedAt)
				}
			}
			exporterSet[row.ExporterISO3] = struct{}{}
			dataTypeSet[row.DataType] = struct{}{}
			rateTypeSet[row.RateType] = struct{}{}
			observationCount++
		}
	}
	if observationCount != index.ObservationCount || !sameStringSet(index.Importers, importerSet) || !sameStringSet(index.Years, yearSet) || !sameStringSet(index.Exporters, exporterSet) || !sameStringSet(index.DataTypes, dataTypeSet) || !sameStringSet(index.RateTypes, rateTypeSet) {
		return errorsForExtended("tariff partition discovery does not match index dimensions")
	}
	return nil
}

func validTariffRateType(value string) bool {
	return value == "mfn_applied" || value == "effectively_applied" || value == "preferential"
}

func validateMatrix(dataDir string, metadata datasetMeta, index validationMatrixIndex) error {
	if index.SchemaVersion != metadata.SchemaVersion || index.GeneratedAt != metadata.GeneratedAt || index.Provider != metadata.MatrixProvider || index.ProductCode != "TOTAL" || index.ProductLevel != 0 {
		return errorsForExtended("bilateral matrix index does not match metadata")
	}
	if len(index.Reporters) != metadata.MatrixReporterCount || len(index.Partitions) != metadata.MatrixPartitionCount || index.PartnerRowCount != metadata.MatrixPartnerRowCount || index.ObservationCount != metadata.MatrixObservationCount {
		return errorsForExtended("bilateral matrix index counts do not match metadata")
	}
	if !sort.StringsAreSorted(index.Reporters) || !sort.StringsAreSorted(index.Partners) {
		return errorsForExtended("bilateral matrix dimensions must be sorted")
	}
	for position, period := range index.Periods {
		if !yearPattern.MatchString(period) || (position > 0 && index.Periods[position-1] < period) {
			return fmt.Errorf("bilateral matrix has invalid or unsorted period %q", period)
		}
	}
	partitionSet := make(map[string]struct{}, len(index.Partitions))
	reporterSet := make(map[string]struct{})
	partnerSet := make(map[string]struct{})
	periodSet := make(map[string]struct{})
	partnerRowCount := 0
	observationCount := 0
	for _, partition := range index.Partitions {
		if !iso3Pattern.MatchString(partition.ReporterISO3) || !yearPattern.MatchString(partition.Period) || partition.RowCount < 1 {
			return fmt.Errorf("bilateral matrix has invalid partition %+v", partition)
		}
		wantHref := "./" + partition.ReporterISO3 + "/" + partition.Period + ".json"
		if partition.Href != wantHref {
			return fmt.Errorf("bilateral matrix partition href %q, want %q", partition.Href, wantHref)
		}
		key := partition.ReporterISO3 + ":" + partition.Period
		if _, exists := partitionSet[key]; exists {
			return fmt.Errorf("bilateral matrix has duplicate partition %s", key)
		}
		partitionSet[key] = struct{}{}
		reporterSet[partition.ReporterISO3] = struct{}{}
		periodSet[partition.Period] = struct{}{}

		var file validationMatrixFile
		if err := readJSON(filepath.Join(dataDir, "bilateral-matrix", partition.ReporterISO3, partition.Period+".json"), &file); err != nil {
			return fmt.Errorf("read bilateral matrix partition %s: %w", key, err)
		}
		if file.SchemaVersion != index.SchemaVersion || file.GeneratedAt != index.GeneratedAt || file.Provider != index.Provider || file.ProductCode != "TOTAL" || file.ProductLevel != 0 || file.ReporterISO3 != partition.ReporterISO3 || file.Period != partition.Period || len(file.Rows) != partition.RowCount {
			return fmt.Errorf("bilateral matrix partition %s does not match its index", key)
		}
		seenPartners := make(map[string]struct{}, len(file.Rows))
		previousTrade := math.Inf(1)
		for _, row := range file.Rows {
			if !iso3Pattern.MatchString(row.PartnerISO3) || row.PartnerISO3 == partition.ReporterISO3 || row.PartnerISO3 == "WLD" || (!row.ExportAvailable && !row.ImportAvailable) {
				return fmt.Errorf("bilateral matrix partition %s has invalid partner row %+v", key, row)
			}
			if _, exists := seenPartners[row.PartnerISO3]; exists {
				return fmt.Errorf("bilateral matrix partition %s has duplicate partner %s", key, row.PartnerISO3)
			}
			seenPartners[row.PartnerISO3] = struct{}{}
			for label, value := range map[string]float64{"export": row.ExportUSD, "import": row.ImportUSD, "trade": row.TradeUSD} {
				if !isFinite(value) || value < 0 {
					return fmt.Errorf("bilateral matrix partition %s has invalid %s value %v", key, label, value)
				}
			}
			if !isFinite(row.BalanceUSD) || !approximatelyEqual(row.TradeUSD, row.ExportUSD+row.ImportUSD) || !approximatelyEqual(row.BalanceUSD, row.ExportUSD-row.ImportUSD) {
				return fmt.Errorf("bilateral matrix partition %s has inconsistent partner row %+v", key, row)
			}
			if !row.ExportAvailable && row.ExportUSD != 0 || !row.ImportAvailable && row.ImportUSD != 0 {
				return fmt.Errorf("bilateral matrix partition %s has values marked unavailable", key)
			}
			if row.TradeUSD > previousTrade {
				return fmt.Errorf("bilateral matrix partition %s rows are not sorted by trade", key)
			}
			previousTrade = row.TradeUSD
			partnerSet[row.PartnerISO3] = struct{}{}
			partnerRowCount++
			if row.ExportAvailable {
				observationCount++
			}
			if row.ImportAvailable {
				observationCount++
			}
		}
	}
	if partnerRowCount != index.PartnerRowCount || observationCount != index.ObservationCount || !sameStringSet(index.Reporters, reporterSet) || !sameStringSet(index.Partners, partnerSet) || !sameStringSet(index.Periods, periodSet) {
		return errorsForExtended("bilateral matrix partition discovery does not match index dimensions")
	}
	return nil
}

func validateMirror(dataDir string, metadata datasetMeta, index validationMirrorIndex) error {
	if index.SchemaVersion != metadata.SchemaVersion || index.GeneratedAt != metadata.GeneratedAt || index.Provider != metadata.MirrorProvider {
		return errorsForExtended("mirror diagnostics index does not match metadata")
	}
	if !reflect.DeepEqual(index.Anchors, []string{"USA", "CHN"}) || len(index.Reporters) != metadata.MirrorReporterCount || len(index.Partitions) != metadata.MirrorPartitionCount || index.ComparisonCount != metadata.MirrorComparisonCount {
		return errorsForExtended("mirror diagnostics dimensions do not match metadata")
	}
	if !sort.StringsAreSorted(index.Reporters) {
		return errorsForExtended("mirror diagnostics reporters must be sorted")
	}
	for position, period := range index.Periods {
		if !yearPattern.MatchString(period) || (position > 0 && index.Periods[position-1] < period) {
			return fmt.Errorf("mirror diagnostics has invalid or unsorted period %q", period)
		}
	}
	reporterSet := make(map[string]struct{})
	periodSet := make(map[string]struct{})
	partitionSet := make(map[string]struct{})
	comparisonCount := 0
	for _, partition := range index.Partitions {
		if !iso3Pattern.MatchString(partition.ReporterISO3) || partition.ReporterISO3 == "USA" || partition.ReporterISO3 == "CHN" || !yearPattern.MatchString(partition.Period) || partition.ComparisonCount < 0 {
			return fmt.Errorf("mirror diagnostics has invalid partition %+v", partition)
		}
		wantHref := "./" + partition.ReporterISO3 + "/" + partition.Period + ".json"
		if partition.Href != wantHref {
			return fmt.Errorf("mirror diagnostics partition href %q, want %q", partition.Href, wantHref)
		}
		key := partition.ReporterISO3 + ":" + partition.Period
		if _, exists := partitionSet[key]; exists {
			return fmt.Errorf("mirror diagnostics has duplicate partition %s", key)
		}
		partitionSet[key] = struct{}{}
		reporterSet[partition.ReporterISO3] = struct{}{}
		periodSet[partition.Period] = struct{}{}

		var file validationMirrorFile
		if err := readJSON(filepath.Join(dataDir, "mirror", partition.ReporterISO3, partition.Period+".json"), &file); err != nil {
			return fmt.Errorf("read mirror diagnostics partition %s: %w", key, err)
		}
		if file.SchemaVersion != index.SchemaVersion || file.GeneratedAt != index.GeneratedAt || file.Provider != index.Provider || file.ReporterISO3 != partition.ReporterISO3 || file.Period != partition.Period || strings.TrimSpace(file.Scope) == "" || len(file.Caveats) < 2 || len(file.Rows) == 0 {
			return fmt.Errorf("mirror diagnostics partition %s does not match its index or lacks disclosure", key)
		}
		seenAnchors := make(map[string]struct{})
		fileComparisons := 0
		for _, row := range file.Rows {
			if row.AnchorISO3 != "USA" && row.AnchorISO3 != "CHN" {
				return fmt.Errorf("mirror diagnostics partition %s has invalid anchor %q", key, row.AnchorISO3)
			}
			if _, exists := seenAnchors[row.AnchorISO3]; exists {
				return fmt.Errorf("mirror diagnostics partition %s repeats anchor %s", key, row.AnchorISO3)
			}
			seenAnchors[row.AnchorISO3] = struct{}{}
			for label, value := range map[string]float64{"reporter export": row.ReporterExportUSD, "anchor import": row.AnchorImportUSD, "reporter import": row.ReporterImportUSD, "anchor export": row.AnchorExportUSD} {
				if !isFinite(value) || value < 0 {
					return fmt.Errorf("mirror diagnostics partition %s has invalid %s value %v", key, label, value)
				}
			}
			if err := validateMirrorPair(row.ReporterExportAvailable, row.AnchorImportAvailable, row.ReporterExportUSD, row.AnchorImportUSD, row.ExportGapUSD, row.ExportSymmetricGapRatio); err != nil {
				return fmt.Errorf("mirror diagnostics partition %s export pair: %w", key, err)
			}
			if row.ReporterExportAvailable && row.AnchorImportAvailable {
				fileComparisons++
			}
			if err := validateMirrorPair(row.ReporterImportAvailable, row.AnchorExportAvailable, row.ReporterImportUSD, row.AnchorExportUSD, row.ImportGapUSD, row.ImportSymmetricGapRatio); err != nil {
				return fmt.Errorf("mirror diagnostics partition %s import pair: %w", key, err)
			}
			if row.ReporterImportAvailable && row.AnchorExportAvailable {
				fileComparisons++
			}
		}
		if fileComparisons != partition.ComparisonCount {
			return fmt.Errorf("mirror diagnostics partition %s comparison count mismatch", key)
		}
		comparisonCount += fileComparisons
	}
	if comparisonCount != index.ComparisonCount || !sameStringSet(index.Reporters, reporterSet) || !sameStringSet(index.Periods, periodSet) {
		return errorsForExtended("mirror diagnostics discovery does not match index dimensions")
	}
	return nil
}

func validateMirrorPair(firstAvailable, secondAvailable bool, first, second float64, gap, ratio *float64) error {
	comparable := firstAvailable && secondAvailable
	if !firstAvailable && first != 0 || !secondAvailable && second != 0 {
		return errors.New("unavailable mirror value is non-zero")
	}
	if !comparable {
		if gap != nil || ratio != nil {
			return errors.New("non-comparable pair publishes a gap")
		}
		return nil
	}
	if gap == nil || ratio == nil || !isFinite(*gap) || !isFinite(*ratio) || !approximatelyEqual(*gap, first-second) {
		return errors.New("comparable pair has inconsistent gap")
	}
	wantRatio := 0.0
	if average := (first + second) / 2; average > 0 {
		wantRatio = (first - second) / average
	}
	if !approximatelyEqual(*ratio, wantRatio) {
		return errors.New("comparable pair has inconsistent symmetric gap ratio")
	}
	return nil
}

func sameStringSet(values []string, set map[string]struct{}) bool {
	if len(values) != len(set) {
		return false
	}
	for _, value := range values {
		if _, ok := set[value]; !ok {
			return false
		}
	}
	return true
}

func validateExplanations(dataDir string, metadata datasetMeta, latest datasetLatest) error {
	var index validationExplanationIndex
	if err := readJSON(filepath.Join(dataDir, "explanations", "index.json"), &index); err != nil {
		return fmt.Errorf("read explanation index: %w", err)
	}
	if index.SchemaVersion != metadata.SchemaVersion || index.GeneratedAt != metadata.GeneratedAt {
		return errorsForExtended("explanation index provenance does not match metadata")
	}
	if index.AICount < 0 || index.FallbackCount < 0 || index.AICount+index.FallbackCount != len(index.Reporters) {
		return errorsForExtended("explanation index generator counts are inconsistent")
	}
	if len(index.Reporters) != len(latest.Rows) || !sort.StringsAreSorted(index.Reporters) {
		return errorsForExtended("explanation reporters must cover and sort all latest reporters")
	}
	latestReporters := make(map[string]struct{}, len(latest.Rows))
	for _, row := range latest.Rows {
		latestReporters[row.ISO3] = struct{}{}
	}
	for _, iso3 := range index.Reporters {
		if _, ok := latestReporters[iso3]; !ok {
			return fmt.Errorf("explanation index has unknown reporter %s", iso3)
		}
		var file validationExplanation
		if err := readJSON(filepath.Join(dataDir, "explanations", iso3+".json"), &file); err != nil {
			return fmt.Errorf("read explanation for %s: %w", iso3, err)
		}
		if file.SchemaVersion != index.SchemaVersion || file.GeneratedAt != index.GeneratedAt || file.ReporterISO3 != iso3 || strings.TrimSpace(file.Summary) == "" {
			return fmt.Errorf("explanation for %s has invalid provenance or empty summary", iso3)
		}
		if (file.Generator.Type != "rules" && file.Generator.Type != "openai") || (file.Generator.Status != "fallback" && file.Generator.Status != "api_fallback" && file.Generator.Status != "success") {
			return fmt.Errorf("explanation for %s has invalid generator metadata", iso3)
		}
		knownEvidence := make(map[string]struct{}, len(file.Evidence))
		for _, item := range file.Evidence {
			if item.ID == "" || item.Label == "" || item.Source == "" || item.SourceJSON == "" {
				return fmt.Errorf("explanation for %s has incomplete evidence", iso3)
			}
			if _, exists := knownEvidence[item.ID]; exists {
				return fmt.Errorf("explanation for %s has duplicate evidence %s", iso3, item.ID)
			}
			knownEvidence[item.ID] = struct{}{}
		}
		if len(file.Statements) < 2 || len(file.Statements) > 6 {
			return fmt.Errorf("explanation for %s has invalid statement count", iso3)
		}
		for _, statement := range file.Statements {
			if strings.TrimSpace(statement.Text) == "" || len(statement.EvidenceIDs) == 0 {
				return fmt.Errorf("explanation for %s has an ungrounded statement", iso3)
			}
			for _, id := range statement.EvidenceIDs {
				if _, ok := knownEvidence[id]; !ok {
					return fmt.Errorf("explanation for %s cites unknown evidence %s", iso3, id)
				}
			}
		}
	}
	return nil
}

func validateSeries(metadata datasetMeta, latest datasetLatest, series validationSeries) error {
	if series.SchemaVersion != metadata.SchemaVersion || series.GeneratedAt != metadata.GeneratedAt || series.Provider != metadata.Provider || !reflect.DeepEqual(series.Partners, metadata.Partners) {
		return errorsForExtended("series provenance does not match metadata")
	}
	seen := make(map[string]struct{})
	pointCount := 0
	for _, reporter := range series.Rows {
		if !iso3Pattern.MatchString(reporter.ISO3) {
			return fmt.Errorf("series has invalid reporter %q", reporter.ISO3)
		}
		if _, exists := seen[reporter.ISO3]; exists {
			return fmt.Errorf("series has duplicate reporter %q", reporter.ISO3)
		}
		seen[reporter.ISO3] = struct{}{}
		periods := make(map[string]struct{})
		for _, point := range reporter.Points {
			pointCount++
			if !validPeriod(point.PeriodType, point.Period) {
				return fmt.Errorf("series %s has invalid period %s/%s", reporter.ISO3, point.PeriodType, point.Period)
			}
			key := point.PeriodType + ":" + point.Period
			if _, exists := periods[key]; exists {
				return fmt.Errorf("series %s has duplicate period %s", reporter.ISO3, key)
			}
			periods[key] = struct{}{}
			if err := validateSeriesBlock(reporter.ISO3, "USA", point.USA); err != nil {
				return err
			}
			if err := validateSeriesBlock(reporter.ISO3, "CHN", point.CHN); err != nil {
				return err
			}
			if point.Comparable != (point.USA.Available && point.CHN.Available) {
				return fmt.Errorf("series %s %s comparable flag is inconsistent", reporter.ISO3, point.Period)
			}
			if !approximatelyEqual(point.Total, point.USA.Trade+point.CHN.Trade) {
				return fmt.Errorf("series %s %s has inconsistent total", reporter.ISO3, point.Period)
			}
			wantShare := 0.0
			if point.Total > 0 {
				wantShare = point.CHN.Trade / point.Total
			}
			if !approximatelyEqual(point.ShareCN, wantShare) {
				return fmt.Errorf("series %s %s has inconsistent China share", reporter.ISO3, point.Period)
			}
		}
	}
	if len(series.Rows) != metadata.SeriesReporterCount || pointCount != metadata.SeriesPointCount {
		return fmt.Errorf("series counts mismatch: meta=%d/%d actual=%d/%d", metadata.SeriesReporterCount, metadata.SeriesPointCount, len(series.Rows), pointCount)
	}
	return nil
}

func validateSeriesBlock(reporter, partner string, block validationSeriesBlock) error {
	for label, value := range map[string]float64{"export": block.Export, "import": block.Import, "trade": block.Trade} {
		if err := finiteNonNegative(partner+" "+label, reporter, value); err != nil {
			return err
		}
	}
	if !approximatelyEqual(block.Trade, block.Export+block.Import) {
		return fmt.Errorf("series %s %s trade is inconsistent", reporter, partner)
	}
	if !block.Available && block.Trade != 0 {
		return fmt.Errorf("series %s %s has values while unavailable", reporter, partner)
	}
	return nil
}

func validateQuality(metadata datasetMeta, quality validationQuality) error {
	if quality.SchemaVersion != metadata.SchemaVersion || quality.GeneratedAt != metadata.GeneratedAt || quality.PrimaryProvider != metadata.Provider || quality.DominantPeriod != metadata.DominantPeriod {
		return errorsForExtended("quality provenance does not match metadata")
	}
	want := validationQualitySummary{
		ReporterCount: metadata.ReporterCount, ComparableReporters: metadata.ComparableReporters,
		IncomparableReporters: metadata.IncomparableReporters, MissingPartnerBlocks: metadata.MissingPartnerBlocks,
		StalePartnerBlocks: metadata.StalePartnerBlocks, ComparisonCount: len(quality.ProviderComparison),
	}
	if !reflect.DeepEqual(quality.Summary, want) {
		return fmt.Errorf("quality summary mismatch: got=%+v want=%+v", quality.Summary, want)
	}
	for _, run := range quality.CollectionRuns {
		if run.RunID == "" || run.Provider == "" || run.Mode == "" {
			return errorsForExtended("collection run is missing identity fields")
		}
		if run.Status != "success" && run.Status != "partial" && run.Status != "failed" {
			return fmt.Errorf("collection run %s has invalid status %q", run.RunID, run.Status)
		}
		for _, value := range []int{run.ReporterCount, run.RequestCount, run.SuccessCount, run.FailureCount, run.SkippedCount, run.StoredCount} {
			if value < 0 {
				return fmt.Errorf("collection run %s has a negative count", run.RunID)
			}
		}
		if run.StartedAt == "" || run.FinishedAt == "" {
			return fmt.Errorf("collection run %s is missing timestamps", run.RunID)
		}
	}
	for _, comparison := range quality.ProviderComparison {
		if !iso3Pattern.MatchString(comparison.ISO3) || !iso3Pattern.MatchString(comparison.Partner) || !validPeriod(comparison.PeriodType, comparison.Period) {
			return fmt.Errorf("invalid provider comparison identity: %+v", comparison)
		}
		if comparison.PrimaryProvider == comparison.SecondaryProvider || comparison.SecondaryProvider == "" {
			return fmt.Errorf("provider comparison for %s does not identify distinct providers", comparison.ISO3)
		}
		if !isFinite(comparison.PrimaryTradeUSD) || comparison.PrimaryTradeUSD < 0 || !isFinite(comparison.SecondaryTradeUSD) || comparison.SecondaryTradeUSD < 0 || !isFinite(comparison.DeltaRatio) {
			return fmt.Errorf("provider comparison for %s has invalid values", comparison.ISO3)
		}
	}
	return nil
}

func validateProducts(dataDir string, metadata datasetMeta, index validationProductIndex) error {
	if index.SchemaVersion != metadata.SchemaVersion || index.GeneratedAt != metadata.GeneratedAt || index.Provider != metadata.ProductProvider || index.Classification != metadata.ProductClassification || index.Level != metadata.ProductLevel || !reflect.DeepEqual(index.Partners, metadata.Partners) {
		return errorsForExtended("product index does not match metadata")
	}
	if len(index.Reporters) != metadata.ProductReporterCount {
		return fmt.Errorf("product reporter count mismatch: meta=%d index=%d", metadata.ProductReporterCount, len(index.Reporters))
	}
	if !sort.StringsAreSorted(index.Reporters) {
		return errorsForExtended("product reporters must be sorted")
	}
	for _, period := range index.Periods {
		if !yearPattern.MatchString(period) {
			return fmt.Errorf("product index has invalid annual period %q", period)
		}
	}
	for _, iso3 := range index.Reporters {
		var file validationProductFile
		if err := readJSON(filepath.Join(dataDir, "products", iso3+".json"), &file); err != nil {
			return fmt.Errorf("read products for %s: %w", iso3, err)
		}
		if file.SchemaVersion != index.SchemaVersion || file.GeneratedAt != index.GeneratedAt || file.Provider != index.Provider || file.Classification != index.Classification || file.Level != index.Level || file.ReporterISO3 != iso3 {
			return fmt.Errorf("product file for %s does not match index", iso3)
		}
		seen := make(map[string]struct{})
		for _, row := range file.Rows {
			if !validPeriod(row.PeriodType, row.Period) || !hs2Pattern.MatchString(row.Code) || strings.TrimSpace(row.Name) == "" {
				return fmt.Errorf("product file %s has invalid row %+v", iso3, row)
			}
			key := row.PeriodType + ":" + row.Period + ":" + row.Code
			if _, exists := seen[key]; exists {
				return fmt.Errorf("product file %s has duplicate %s", iso3, key)
			}
			seen[key] = struct{}{}
			if err := validateSeriesBlock(iso3, "USA product", row.USA); err != nil {
				return err
			}
			if err := validateSeriesBlock(iso3, "CHN product", row.CHN); err != nil {
				return err
			}
			if !approximatelyEqual(row.Total, row.USA.Trade+row.CHN.Trade) {
				return fmt.Errorf("product %s %s has inconsistent total", iso3, row.Code)
			}
			wantShare := 0.0
			if row.Total > 0 {
				wantShare = row.CHN.Trade / row.Total
			}
			if !approximatelyEqual(row.ShareCN, wantShare) {
				return fmt.Errorf("product %s %s has inconsistent share", iso3, row.Code)
			}
		}
	}
	return nil
}

func validateContext(metadata datasetMeta, latest datasetLatest, data validationContext) error {
	if data.Status != metadata.ContextStatus || (data.Status != "success" && data.Status != "partial") {
		return fmt.Errorf("context status mismatch or invalid: meta=%q context=%q", metadata.ContextStatus, data.Status)
	}
	if _, err := time.Parse(time.RFC3339, data.GeneratedAt); err != nil {
		return fmt.Errorf("context has invalid generated_at: %w", err)
	}
	if data.Source == "" || data.SchemaVersion == "" {
		return errorsForExtended("context provenance is incomplete")
	}
	seen := make(map[string]validationContextCountry)
	for _, country := range data.Countries {
		if !iso3Pattern.MatchString(country.ISO3) || len(country.ISO2) != 2 || country.Name == "" {
			return fmt.Errorf("context has invalid country %+v", country)
		}
		if _, exists := seen[country.ISO3]; exists {
			return fmt.Errorf("context has duplicate country %s", country.ISO3)
		}
		if err := validateContextMetric(country.ISO3, "population", country.Population); err != nil {
			return err
		}
		if err := validateContextMetric(country.ISO3, "gdp", country.GDP); err != nil {
			return err
		}
		seen[country.ISO3] = country
	}
	if data.Status == "success" {
		for _, row := range latest.Rows {
			if _, exists := seen[row.ISO3]; !exists {
				return fmt.Errorf("context is missing published reporter %s", row.ISO3)
			}
		}
	}
	return nil
}

func errorsForExtended(message string) error { return fmt.Errorf("%s", message) }
