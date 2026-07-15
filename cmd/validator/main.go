package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	iso3Pattern    = regexp.MustCompile(`^[A-Z]{3}$`)
	yearPattern    = regexp.MustCompile(`^\d{4}$`)
	quarterPattern = regexp.MustCompile(`^\d{4}-Q[1-4]$`)
	monthPattern   = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])$`)
)

type datasetMeta struct {
	SchemaVersion           string         `json:"schema_version"`
	GeneratedAt             string         `json:"generated_at"`
	Provider                string         `json:"provider"`
	Partners                []string       `json:"partners"`
	ReporterCount           int            `json:"reporter_count"`
	ObservationCount        int            `json:"observation_count"`
	ExpectedPartnerBlocks   int            `json:"expected_partner_blocks"`
	AvailablePartnerBlocks  int            `json:"available_partner_blocks"`
	MissingPartnerBlocks    int            `json:"missing_partner_blocks"`
	PeriodCounts            map[string]int `json:"period_counts"`
	DominantPeriod          string         `json:"dominant_period"`
	ComparableReporters     int            `json:"comparable_reporters"`
	IncomparableReporters   int            `json:"incomparable_reporters"`
	StalePartnerBlocks      int            `json:"stale_partner_blocks"`
	SeriesReporterCount     int            `json:"series_reporter_count"`
	SeriesPointCount        int            `json:"series_point_count"`
	ProductProvider         string         `json:"product_provider,omitempty"`
	ProductClassification   string         `json:"product_classification,omitempty"`
	ProductLevel            int            `json:"product_level,omitempty"`
	ProductReporterCount    int            `json:"product_reporter_count"`
	ProductObservationCount int            `json:"product_observation_count"`
	ContextStatus           string         `json:"context_status"`
}

type datasetLatest struct {
	SchemaVersion string       `json:"schema_version"`
	GeneratedAt   string       `json:"generated_at"`
	Provider      string       `json:"provider"`
	Partners      []string     `json:"partners"`
	Rows          []datasetRow `json:"rows"`
}

type datasetRow struct {
	ISO3             string        `json:"iso3"`
	ISO2             string        `json:"iso2,omitempty"`
	Name             string        `json:"name,omitempty"`
	Region           string        `json:"region,omitempty"`
	IncomeGroup      string        `json:"income_group,omitempty"`
	Groups           []string      `json:"groups,omitempty"`
	Population       contextMetric `json:"population"`
	GDP              contextMetric `json:"gdp"`
	USA              partnerBlock  `json:"usa"`
	CHN              partnerBlock  `json:"chn"`
	Total            float64       `json:"total"`
	ShareCN          float64       `json:"share_cn"`
	SamePeriod       bool          `json:"same_period"`
	ComparisonPeriod string        `json:"comparison_period,omitempty"`
}

type contextMetric struct {
	Value *float64 `json:"value"`
	Year  string   `json:"year"`
}

type partnerBlock struct {
	Period      string       `json:"period"`
	PeriodType  string       `json:"period_type"`
	PrevPeriod  string       `json:"prev_period,omitempty"`
	Export      float64      `json:"export"`
	Import      float64      `json:"import"`
	Trade       float64      `json:"trade"`
	Growth      *growthBlock `json:"growth,omitempty"`
	GrowthBasis string       `json:"growth_basis,omitempty"`
}

type growthBlock struct {
	Export *float64 `json:"export"`
	Import *float64 `json:"import"`
	Trade  *float64 `json:"trade"`
}

func main() {
	dataDir := flag.String("dir", "site/data", "directory containing meta.json and latest.json")
	minReporters := flag.Int("min-reporters", 1, "minimum expected number of reporter rows")
	flag.Parse()

	metadata, latest, err := loadDataset(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dataset validation failed:", err)
		os.Exit(1)
	}
	if err := validateDataset(metadata, latest, *minReporters); err != nil {
		fmt.Fprintln(os.Stderr, "dataset validation failed:", err)
		os.Exit(1)
	}
	if err := validateExtendedDataset(*dataDir, metadata, latest); err != nil {
		fmt.Fprintln(os.Stderr, "dataset validation failed:", err)
		os.Exit(1)
	}

	fmt.Printf(
		"dataset validation complete (provider=%s reporters=%d coverage=%d/%d periods=%s)\n",
		metadata.Provider,
		metadata.ReporterCount,
		metadata.AvailablePartnerBlocks,
		metadata.ExpectedPartnerBlocks,
		formatPeriodCounts(metadata.PeriodCounts),
	)
}

func loadDataset(dataDir string) (datasetMeta, datasetLatest, error) {
	var metadata datasetMeta
	var latest datasetLatest
	if strings.TrimSpace(dataDir) == "" {
		return metadata, latest, errors.New("data directory is required")
	}
	if err := readJSON(filepath.Join(dataDir, "meta.json"), &metadata); err != nil {
		return metadata, latest, fmt.Errorf("read meta.json: %w", err)
	}
	if err := readJSON(filepath.Join(dataDir, "latest.json"), &latest); err != nil {
		return metadata, latest, fmt.Errorf("read latest.json: %w", err)
	}
	return metadata, latest, nil
}

func readJSON(path string, value any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return errors.New("unexpected trailing JSON content")
		}
		return fmt.Errorf("unexpected trailing JSON content: %w", err)
	}
	return nil
}

func validateDataset(metadata datasetMeta, latest datasetLatest, minReporters int) error {
	if minReporters < 1 {
		return errors.New("min-reporters must be positive")
	}
	if metadata.SchemaVersion == "" || metadata.SchemaVersion != latest.SchemaVersion {
		return fmt.Errorf("schema version mismatch: meta=%q latest=%q", metadata.SchemaVersion, latest.SchemaVersion)
	}
	if _, err := time.Parse(time.RFC3339, metadata.GeneratedAt); err != nil {
		return fmt.Errorf("invalid generated_at in metadata: %w", err)
	}
	if metadata.GeneratedAt != latest.GeneratedAt {
		return fmt.Errorf("generated_at mismatch: meta=%q latest=%q", metadata.GeneratedAt, latest.GeneratedAt)
	}
	if metadata.Provider == "" || metadata.Provider != latest.Provider {
		return fmt.Errorf("provider mismatch: meta=%q latest=%q", metadata.Provider, latest.Provider)
	}
	if !reflect.DeepEqual(metadata.Partners, latest.Partners) {
		return fmt.Errorf("partner mismatch: meta=%v latest=%v", metadata.Partners, latest.Partners)
	}
	if !containsAll(metadata.Partners, "USA", "CHN") {
		return fmt.Errorf("partners must include USA and CHN: %v", metadata.Partners)
	}
	if len(latest.Rows) < minReporters {
		return fmt.Errorf("reporter count %d is below minimum %d", len(latest.Rows), minReporters)
	}
	if metadata.ReporterCount != len(latest.Rows) {
		return fmt.Errorf("reporter count mismatch: meta=%d latest=%d", metadata.ReporterCount, len(latest.Rows))
	}
	if metadata.ObservationCount < metadata.ReporterCount {
		return fmt.Errorf("observation count %d is lower than reporter count %d", metadata.ObservationCount, metadata.ReporterCount)
	}

	seen := make(map[string]struct{}, len(latest.Rows))
	periodCounts := make(map[string]int)
	availableBlocks := 0
	comparableReporters := 0
	for index, row := range latest.Rows {
		if !iso3Pattern.MatchString(row.ISO3) {
			return fmt.Errorf("row %d has invalid ISO3 %q", index, row.ISO3)
		}
		if _, exists := seen[row.ISO3]; exists {
			return fmt.Errorf("duplicate reporter %q", row.ISO3)
		}
		seen[row.ISO3] = struct{}{}
		if row.ISO2 != "" && !regexp.MustCompile(`^[A-Z]{2}$`).MatchString(row.ISO2) {
			return fmt.Errorf("%s has invalid ISO2 %q", row.ISO3, row.ISO2)
		}
		if err := validateContextMetric(row.ISO3, "population", row.Population); err != nil {
			return err
		}
		if err := validateContextMetric(row.ISO3, "gdp", row.GDP); err != nil {
			return err
		}

		for label, block := range map[string]partnerBlock{"USA": row.USA, "CHN": row.CHN} {
			if err := validateBlock(row.ISO3, label, block); err != nil {
				return err
			}
			if block.Period != "" {
				availableBlocks++
				periodCounts[block.PeriodType+":"+block.Period]++
			}
		}
		samePeriod := row.USA.Period != "" && row.CHN.Period != "" && row.USA.PeriodType == row.CHN.PeriodType && row.USA.Period == row.CHN.Period
		if samePeriod {
			comparableReporters++
		}
		if strings.HasPrefix(metadata.SchemaVersion, "2.") {
			if row.SamePeriod != samePeriod {
				return fmt.Errorf("%s same_period=%v does not match partner periods", row.ISO3, row.SamePeriod)
			}
			if samePeriod && row.ComparisonPeriod != row.USA.Period {
				return fmt.Errorf("%s comparison_period=%q, want %q", row.ISO3, row.ComparisonPeriod, row.USA.Period)
			}
			if !samePeriod && row.ComparisonPeriod != "" {
				return fmt.Errorf("%s has comparison_period without comparable partners", row.ISO3)
			}
		}

		if err := finiteNonNegative("total", row.ISO3, row.Total); err != nil {
			return err
		}
		if !approximatelyEqual(row.Total, row.USA.Trade+row.CHN.Trade) {
			return fmt.Errorf("%s total %v does not equal USA+CHN trade %v", row.ISO3, row.Total, row.USA.Trade+row.CHN.Trade)
		}
		if !isFinite(row.ShareCN) || row.ShareCN < 0 || row.ShareCN > 1 {
			return fmt.Errorf("%s share_cn %v is outside [0,1]", row.ISO3, row.ShareCN)
		}
		wantShare := 0.0
		if row.Total > 0 {
			wantShare = row.CHN.Trade / row.Total
		}
		if !approximatelyEqual(row.ShareCN, wantShare) {
			return fmt.Errorf("%s share_cn %v does not equal calculated value %v", row.ISO3, row.ShareCN, wantShare)
		}
	}

	expectedBlocks := len(latest.Rows) * len(latest.Partners)
	missingBlocks := expectedBlocks - availableBlocks
	if missingBlocks < 0 {
		missingBlocks = 0
	}
	if metadata.ExpectedPartnerBlocks != expectedBlocks || metadata.AvailablePartnerBlocks != availableBlocks || metadata.MissingPartnerBlocks != missingBlocks {
		return fmt.Errorf(
			"coverage mismatch: meta=%d/%d missing=%d calculated=%d/%d missing=%d",
			metadata.AvailablePartnerBlocks,
			metadata.ExpectedPartnerBlocks,
			metadata.MissingPartnerBlocks,
			availableBlocks,
			expectedBlocks,
			missingBlocks,
		)
	}
	if !reflect.DeepEqual(metadata.PeriodCounts, periodCounts) {
		return fmt.Errorf("period counts mismatch: meta=%v calculated=%v", metadata.PeriodCounts, periodCounts)
	}
	if strings.HasPrefix(metadata.SchemaVersion, "2.") {
		dominant := dominantPeriod(periodCounts)
		stale := 0
		for _, row := range latest.Rows {
			for _, block := range []partnerBlock{row.USA, row.CHN} {
				if block.Period != "" && block.PeriodType+":"+block.Period != dominant {
					stale++
				}
			}
		}
		if metadata.DominantPeriod != dominant {
			return fmt.Errorf("dominant period mismatch: meta=%q calculated=%q", metadata.DominantPeriod, dominant)
		}
		if metadata.ComparableReporters != comparableReporters || metadata.IncomparableReporters != len(latest.Rows)-comparableReporters {
			return fmt.Errorf("comparable reporter mismatch: meta=%d/%d calculated=%d/%d", metadata.ComparableReporters, metadata.IncomparableReporters, comparableReporters, len(latest.Rows)-comparableReporters)
		}
		if metadata.StalePartnerBlocks != stale {
			return fmt.Errorf("stale partner block mismatch: meta=%d calculated=%d", metadata.StalePartnerBlocks, stale)
		}
	}

	return nil
}

func validateContextMetric(reporter, label string, metric contextMetric) error {
	if metric.Value != nil {
		if err := finiteNonNegative(label, reporter, *metric.Value); err != nil {
			return err
		}
		if !yearPattern.MatchString(metric.Year) {
			return fmt.Errorf("%s %s has invalid year %q", reporter, label, metric.Year)
		}
	} else if metric.Year != "" {
		return fmt.Errorf("%s %s has year without value", reporter, label)
	}
	return nil
}

func dominantPeriod(counts map[string]int) string {
	best := ""
	bestCount := -1
	for key, count := range counts {
		if count > bestCount || (count == bestCount && key > best) {
			best, bestCount = key, count
		}
	}
	return best
}

func validateBlock(reporter, partner string, block partnerBlock) error {
	for label, value := range map[string]float64{"export": block.Export, "import": block.Import, "trade": block.Trade} {
		if err := finiteNonNegative(partner+" "+label, reporter, value); err != nil {
			return err
		}
	}
	if !approximatelyEqual(block.Trade, block.Export+block.Import) {
		return fmt.Errorf("%s %s trade %v does not equal export+import %v", reporter, partner, block.Trade, block.Export+block.Import)
	}
	if block.Growth != nil {
		for label, value := range map[string]*float64{"export": block.Growth.Export, "import": block.Growth.Import, "trade": block.Growth.Trade} {
			if value != nil && !isFinite(*value) {
				return fmt.Errorf("%s %s growth %s must be finite, got %v", reporter, partner, label, *value)
			}
		}
		if strings.ToLower(block.GrowthBasis) != "yoy" {
			return fmt.Errorf("%s %s has unsupported growth basis %q", reporter, partner, block.GrowthBasis)
		}
	}
	if block.Period == "" {
		if block.PeriodType != "" {
			return fmt.Errorf("%s %s has period type %q without a period", reporter, partner, block.PeriodType)
		}
		return nil
	}

	if !validPeriod(block.PeriodType, block.Period) {
		return fmt.Errorf("%s %s has invalid period %q/%q", reporter, partner, block.PeriodType, block.Period)
	}
	if block.PrevPeriod != "" && !validPeriod(block.PeriodType, block.PrevPeriod) {
		return fmt.Errorf("%s %s has invalid previous period %q/%q", reporter, partner, block.PeriodType, block.PrevPeriod)
	}
	return nil
}

func validPeriod(periodType, period string) bool {
	switch periodType {
	case "Y":
		return yearPattern.MatchString(period)
	case "Q":
		return quarterPattern.MatchString(period)
	case "M":
		return monthPattern.MatchString(period)
	default:
		return false
	}
}

func finiteNonNegative(field, reporter string, value float64) error {
	if !isFinite(value) || value < 0 {
		return fmt.Errorf("%s %s must be finite and non-negative, got %v", reporter, field, value)
	}
	return nil
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func approximatelyEqual(a, b float64) bool {
	scale := math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
	return math.Abs(a-b) <= scale*1e-9
}

func containsAll(values []string, required ...string) bool {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	for _, value := range required {
		if _, ok := set[value]; !ok {
			return false
		}
	}
	return true
}

func formatPeriodCounts(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys)))
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ",")
}
