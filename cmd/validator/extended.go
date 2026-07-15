package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

var hs2Pattern = regexp.MustCompile(`^\d{2}$`)

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
	if err := validateExplanations(dataDir, metadata, latest); err != nil {
		return err
	}
	var contextData validationContext
	if err := readJSON(filepath.Join(dataDir, "context.json"), &contextData); err != nil {
		return fmt.Errorf("read context.json: %w", err)
	}
	return validateContext(metadata, latest, contextData)
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
