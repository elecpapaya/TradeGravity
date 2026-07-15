package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"tradegravity/internal/model"
	"tradegravity/internal/strategic"
)

const schemaVersion = "2.0"

type metaFile struct {
	SchemaVersion             string         `json:"schema_version"`
	GeneratedAt               string         `json:"generated_at"`
	Provider                  string         `json:"provider"`
	Partners                  []string       `json:"partners"`
	ReporterCount             int            `json:"reporter_count"`
	ObservationCount          int            `json:"observation_count"`
	ExpectedPartnerBlocks     int            `json:"expected_partner_blocks"`
	AvailablePartnerBlocks    int            `json:"available_partner_blocks"`
	MissingPartnerBlocks      int            `json:"missing_partner_blocks"`
	PeriodCounts              map[string]int `json:"period_counts"`
	DominantPeriod            string         `json:"dominant_period"`
	ComparableReporters       int            `json:"comparable_reporters"`
	IncomparableReporters     int            `json:"incomparable_reporters"`
	StalePartnerBlocks        int            `json:"stale_partner_blocks"`
	SeriesReporterCount       int            `json:"series_reporter_count"`
	SeriesPointCount          int            `json:"series_point_count"`
	ProductProvider           string         `json:"product_provider,omitempty"`
	ProductClassification     string         `json:"product_classification,omitempty"`
	ProductLevel              int            `json:"product_level,omitempty"`
	ProductReporterCount      int            `json:"product_reporter_count"`
	ProductObservationCount   int            `json:"product_observation_count"`
	ContextStatus             string         `json:"context_status"`
	StrategicProvider         string         `json:"strategic_provider,omitempty"`
	StrategicLevel            int            `json:"strategic_level,omitempty"`
	StrategicProductCount     int            `json:"strategic_product_count"`
	StrategicReporterCount    int            `json:"strategic_reporter_count"`
	StrategicPartitionCount   int            `json:"strategic_partition_count"`
	StrategicObservationCount int            `json:"strategic_observation_count"`
	TariffProvider            string         `json:"tariff_provider,omitempty"`
	TariffImporterCount       int            `json:"tariff_importer_count"`
	TariffPartitionCount      int            `json:"tariff_partition_count"`
	TariffObservationCount    int            `json:"tariff_observation_count"`
	MatrixProvider            string         `json:"matrix_provider,omitempty"`
	MatrixReporterCount       int            `json:"matrix_reporter_count"`
	MatrixPartitionCount      int            `json:"matrix_partition_count"`
	MatrixPartnerRowCount     int            `json:"matrix_partner_row_count"`
	MatrixObservationCount    int            `json:"matrix_observation_count"`
}

type latestFile struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   string        `json:"generated_at"`
	Provider      string        `json:"provider"`
	Partners      []string      `json:"partners"`
	Rows          []latestEntry `json:"rows"`
}

type latestEntry struct {
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

type partnerBlock struct {
	Period      string           `json:"period"`
	PeriodType  model.PeriodType `json:"period_type"`
	PrevPeriod  string           `json:"prev_period,omitempty"`
	Export      float64          `json:"export"`
	Import      float64          `json:"import"`
	Trade       float64          `json:"trade"`
	Growth      *growthBlock     `json:"growth,omitempty"`
	GrowthBasis string           `json:"growth_basis,omitempty"`
}

type growthBlock struct {
	Export *float64 `json:"export"`
	Import *float64 `json:"import"`
	Trade  *float64 `json:"trade"`
}

type observationRow struct {
	Provider       string
	ReporterISO    string
	PartnerISO     string
	Flow           model.Flow
	PeriodType     model.PeriodType
	Period         string
	ValueUSD       float64
	Classification string
	ProductCode    string
	ProductLevel   int
}

type latestValue struct {
	PeriodType model.PeriodType
	Period     string
	ValueUSD   float64
	Valid      bool
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "build":
		build(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func build(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	outDir := fs.String("out", "site/data", "output directory")
	dbPath := fs.String("db", "tradegravity.db", "sqlite database path")
	provider := fs.String("provider", "wits", "provider id")
	partnersCSV := fs.String("partners", "USA,CHN", "comma-separated partner ISO3 list (expects USA,CHN)")
	contextPath := fs.String("context", "site/data/context.json", "country context JSON (optional)")
	productProvider := fs.String("product-provider", "comtrade", "HS2 product provider")
	matrixProvider := fs.String("matrix-provider", "comtrade", "bilateral matrix provider")
	productLevel := fs.Int("product-level", 2, "product aggregation level")
	hs2Path := fs.String("hs2", "configs/hs2.csv", "HS2 labels CSV")
	strategicRegistryPath := fs.String("strategic-registry", "configs/strategic_hs6.csv", "strategic HS6 registry CSV")
	seriesYears := fs.Int("series-years", 10, "maximum number of annual periods per reporter")
	fs.Parse(args)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create output dir:", err)
		os.Exit(1)
	}

	partners := parseList(*partnersCSV)
	if err := ensureRequiredPartners(partners, []string{"USA", "CHN"}); err != nil {
		fmt.Fprintln(os.Stderr, "invalid partners:", err)
		os.Exit(1)
	}

	rows, err := loadObservations(*dbPath, *provider, partners)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load observations:", err)
		os.Exit(1)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	latest := buildLatest(rows)
	contextData, err := loadContext(*contextPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load country context:", err)
		os.Exit(1)
	}
	enrichLatest(latest, contextData.Countries)
	seriesOutput := buildSeriesFile(now, *provider, partners, rows, *seriesYears)
	productRows, err := loadProductObservations(*dbPath, *productProvider, *productLevel, partners)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load product observations:", err)
		os.Exit(1)
	}
	hs2Labels, err := loadProductLabels(*hs2Path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load product labels:", err)
		os.Exit(1)
	}
	productIndex, productFiles := buildProductFiles(now, *productProvider, *productLevel, partners, productRows, hs2Labels)
	strategicProducts, err := strategic.LoadCSV(*strategicRegistryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load strategic HS6 registry:", err)
		os.Exit(1)
	}
	strategicRows, err := loadProductObservations(*dbPath, *productProvider, 6, partners)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load strategic HS6 observations:", err)
		os.Exit(1)
	}
	strategicIndex, strategicFiles := buildStrategicFiles(now, *productProvider, partners, strategicRows, strategicProducts)
	tariffRows, err := loadTariffObservations(*dbPath, "trains")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load tariff observations:", err)
		os.Exit(1)
	}
	tariffIndex, tariffFiles := buildTariffFiles(now, "trains", tariffRows, strategicProducts)
	matrixRows, err := loadMatrixObservations(*dbPath, *matrixProvider)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load bilateral matrix observations:", err)
		os.Exit(1)
	}
	matrixIndex, matrixFiles := buildMatrixFiles(now, *matrixProvider, matrixRows)
	runs, err := loadIngestRuns(*dbPath, 20)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to load ingest runs:", err)
		os.Exit(1)
	}
	quality := buildQualityFile(now, *provider, latest, rows, productRows, runs)
	catalog := buildDataCatalog(now, *provider, contextData.Status, seriesOutput, productIndex, strategicIndex, tariffIndex, matrixIndex)
	metadata := buildMeta(now, *provider, partners, rows, latest)
	augmentMeta(&metadata, latest, seriesOutput, productIndex, len(productRows), contextData.Status)
	augmentStrategicMeta(&metadata, strategicIndex)
	augmentTariffMeta(&metadata, tariffIndex)
	augmentMatrixMeta(&metadata, matrixIndex)
	if err := writeJSON(filepath.Join(*outDir, "meta.json"), metadata); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write meta.json:", err)
		os.Exit(1)
	}

	output := latestFile{
		SchemaVersion: schemaVersion,
		GeneratedAt:   now,
		Provider:      strings.ToLower(strings.TrimSpace(*provider)),
		Partners:      partners,
		Rows:          latest,
	}
	if err := writeJSON(filepath.Join(*outDir, "latest.json"), output); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write latest.json:", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(*outDir, "series.json"), seriesOutput); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write series.json:", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(*outDir, "quality.json"), quality); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write quality.json:", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(*outDir, "catalog.json"), catalog); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write catalog.json:", err)
		os.Exit(1)
	}
	productsDir := filepath.Join(*outDir, "products")
	if err := os.MkdirAll(productsDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create products dir:", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(productsDir, "index.json"), productIndex); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write product index:", err)
		os.Exit(1)
	}
	for iso3, file := range productFiles {
		if err := writeJSON(filepath.Join(productsDir, iso3+".json"), file); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write products for %s: %v\n", iso3, err)
			os.Exit(1)
		}
	}
	strategicDir := filepath.Join(*outDir, "strategic-hs6")
	if err := os.MkdirAll(strategicDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create strategic HS6 dir:", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(strategicDir, "index.json"), strategicIndex); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write strategic HS6 index:", err)
		os.Exit(1)
	}
	for relativePath, file := range strategicFiles {
		path := filepath.Join(strategicDir, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create strategic partition directory for %s: %v\n", relativePath, err)
			os.Exit(1)
		}
		if err := writeJSON(path, file); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write strategic partition %s: %v\n", relativePath, err)
			os.Exit(1)
		}
	}
	tariffDir := filepath.Join(*outDir, "tariffs")
	if err := os.MkdirAll(tariffDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create tariff dir:", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(tariffDir, "index.json"), tariffIndex); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write tariff index:", err)
		os.Exit(1)
	}
	for relativePath, file := range tariffFiles {
		path := filepath.Join(tariffDir, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create tariff partition directory for %s: %v\n", relativePath, err)
			os.Exit(1)
		}
		if err := writeJSON(path, file); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write tariff partition %s: %v\n", relativePath, err)
			os.Exit(1)
		}
	}
	matrixDir := filepath.Join(*outDir, "bilateral-matrix")
	if err := os.MkdirAll(matrixDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create bilateral matrix dir:", err)
		os.Exit(1)
	}
	if err := writeJSON(filepath.Join(matrixDir, "index.json"), matrixIndex); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write bilateral matrix index:", err)
		os.Exit(1)
	}
	for relativePath, file := range matrixFiles {
		path := filepath.Join(matrixDir, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "failed to create bilateral matrix partition directory for %s: %v\n", relativePath, err)
			os.Exit(1)
		}
		if err := writeJSON(path, file); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write bilateral matrix partition %s: %v\n", relativePath, err)
			os.Exit(1)
		}
	}

	fmt.Printf("publisher build complete (out=%s)\n", *outDir)
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: publisher build [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "options:")
	fmt.Fprintln(os.Stderr, "  -out   output directory (default: site/data)")
	fmt.Fprintln(os.Stderr, "  -db    sqlite database path (default: tradegravity.db)")
	fmt.Fprintln(os.Stderr, "  -provider   provider id (default: wits)")
	fmt.Fprintln(os.Stderr, "  -partners   comma-separated partner ISO3 list (default: USA,CHN)")
	fmt.Fprintln(os.Stderr, "  -context   country context JSON (default: site/data/context.json)")
	fmt.Fprintln(os.Stderr, "  -product-provider   HS2 provider (default: comtrade)")
	fmt.Fprintln(os.Stderr, "  -matrix-provider   bilateral matrix provider (default: comtrade)")
	fmt.Fprintln(os.Stderr, "  -product-level   product level (default: 2)")
	fmt.Fprintln(os.Stderr, "  -strategic-registry   strategic HS6 registry CSV")
	fmt.Fprintln(os.Stderr, "  -series-years   annual history window (default: 10)")
}

func loadObservations(dbPath, provider string, partners []string) ([]observationRow, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("db path is required")
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ctx := context.Background()
	query := `
		SELECT provider, reporter_iso3, partner_iso3, flow, period_type, period, value_usd
		FROM trade_observations
		WHERE flow IN ('export','import') AND product_level = 0 AND product_code = 'TOTAL'
	`
	args := []any{}
	if strings.TrimSpace(provider) != "" {
		query += " AND provider = ?"
		args = append(args, provider)
	}
	if len(partners) > 0 {
		query += " AND partner_iso3 IN (" + placeholders(len(partners)) + ")"
		for _, partner := range partners {
			args = append(args, partner)
		}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]observationRow, 0)
	for rows.Next() {
		var row observationRow
		var flow string
		var periodType string
		if err := rows.Scan(&row.Provider, &row.ReporterISO, &row.PartnerISO, &flow, &periodType, &row.Period, &row.ValueUSD); err != nil {
			return nil, err
		}
		row.Flow = model.Flow(strings.ToLower(flow))
		row.PeriodType = model.PeriodType(strings.ToUpper(periodType))
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func buildLatest(rows []observationRow) []latestEntry {
	latest := make(map[string]map[string]map[model.Flow]latestValue)
	series := make(map[string]map[string]map[model.Flow]map[string]float64)

	for _, row := range rows {
		reporter := strings.ToUpper(row.ReporterISO)
		partner := strings.ToUpper(row.PartnerISO)
		if reporter == "" || partner == "" {
			continue
		}

		if _, ok := latest[reporter]; !ok {
			latest[reporter] = make(map[string]map[model.Flow]latestValue)
		}
		if _, ok := series[reporter]; !ok {
			series[reporter] = make(map[string]map[model.Flow]map[string]float64)
		}
		if _, ok := latest[reporter][partner]; !ok {
			latest[reporter][partner] = make(map[model.Flow]latestValue)
		}
		if _, ok := series[reporter][partner]; !ok {
			series[reporter][partner] = make(map[model.Flow]map[string]float64)
		}
		if _, ok := series[reporter][partner][row.Flow]; !ok {
			series[reporter][partner][row.Flow] = make(map[string]float64)
		}
		series[reporter][partner][row.Flow][seriesKey(row.PeriodType, row.Period)] = row.ValueUSD

		current := latest[reporter][partner][row.Flow]
		if !current.Valid || comparePeriods(row.PeriodType, row.Period, current.PeriodType, current.Period) > 0 {
			latest[reporter][partner][row.Flow] = latestValue{
				PeriodType: row.PeriodType,
				Period:     row.Period,
				ValueUSD:   row.ValueUSD,
				Valid:      true,
			}
		}
	}

	results := make([]latestEntry, 0, len(latest))
	for reporter, partners := range latest {
		usa := buildPartnerBlock(partners["USA"], series[reporter]["USA"])
		chn := buildPartnerBlock(partners["CHN"], series[reporter]["CHN"])
		if !usa.HasData() && !chn.HasData() {
			continue
		}

		total := usa.Trade + chn.Trade
		shareCN := 0.0
		if total > 0 {
			shareCN = chn.Trade / total
		}

		samePeriod := usa.HasData() && chn.HasData() && usa.PeriodType == chn.PeriodType && usa.Period == chn.Period
		comparisonPeriod := ""
		if samePeriod {
			comparisonPeriod = usa.Period
		}
		results = append(results, latestEntry{
			ISO3:             reporter,
			USA:              usa.partnerBlock,
			CHN:              chn.partnerBlock,
			Total:            total,
			ShareCN:          shareCN,
			SamePeriod:       samePeriod,
			ComparisonPeriod: comparisonPeriod,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ISO3 < results[j].ISO3
	})
	return results
}

func buildMeta(generatedAt, provider string, partners []string, observations []observationRow, latest []latestEntry) metaFile {
	periodCounts := make(map[string]int)
	availableBlocks := 0
	for _, entry := range latest {
		for _, block := range []partnerBlock{entry.USA, entry.CHN} {
			if strings.TrimSpace(block.Period) == "" {
				continue
			}
			availableBlocks++
			key := string(block.PeriodType) + ":" + block.Period
			periodCounts[key]++
		}
	}

	expectedBlocks := len(latest) * len(partners)
	missingBlocks := expectedBlocks - availableBlocks
	if missingBlocks < 0 {
		missingBlocks = 0
	}

	return metaFile{
		SchemaVersion:          schemaVersion,
		GeneratedAt:            generatedAt,
		Provider:               strings.ToLower(strings.TrimSpace(provider)),
		Partners:               append([]string(nil), partners...),
		ReporterCount:          len(latest),
		ObservationCount:       len(observations),
		ExpectedPartnerBlocks:  expectedBlocks,
		AvailablePartnerBlocks: availableBlocks,
		MissingPartnerBlocks:   missingBlocks,
		PeriodCounts:           periodCounts,
	}
}

type partnerSummary struct {
	partnerBlock
	hasData bool
}

func (p partnerSummary) HasData() bool {
	return p.hasData
}

func buildPartnerBlock(values map[model.Flow]latestValue, series map[model.Flow]map[string]float64) partnerSummary {
	if values == nil {
		return partnerSummary{}
	}
	export := values[model.FlowExport]
	imported := values[model.FlowImport]

	periodType, period := selectLatestPeriod(export, imported)
	exportValue, exportOk := seriesValue(series, model.FlowExport, periodType, period)
	importValue, importOk := seriesValue(series, model.FlowImport, periodType, period)
	if !exportOk && export.Valid {
		exportValue = export.ValueUSD
		exportOk = true
	}
	if !importOk && imported.Valid {
		importValue = imported.ValueUSD
		importOk = true
	}

	prevPeriod, growth := buildGrowth(series, periodType, period)

	block := partnerBlock{
		Period:      period,
		PeriodType:  periodType,
		PrevPeriod:  prevPeriod,
		Export:      exportValue,
		Import:      importValue,
		Trade:       exportValue + importValue,
		Growth:      growth,
		GrowthBasis: "yoy",
	}
	if block.Period == "" || block.Growth == nil {
		block.GrowthBasis = ""
	}
	hasData := exportOk || importOk
	return partnerSummary{partnerBlock: block, hasData: hasData}
}

func selectLatestPeriod(export, imported latestValue) (model.PeriodType, string) {
	if export.Valid && !imported.Valid {
		return export.PeriodType, export.Period
	}
	if imported.Valid && !export.Valid {
		return imported.PeriodType, imported.Period
	}
	if export.Valid && imported.Valid {
		if comparePeriods(export.PeriodType, export.Period, imported.PeriodType, imported.Period) >= 0 {
			return export.PeriodType, export.Period
		}
		return imported.PeriodType, imported.Period
	}
	return "", ""
}

func comparePeriods(aType model.PeriodType, aPeriod string, bType model.PeriodType, bPeriod string) int {
	priorityA := periodPriority(aType)
	priorityB := periodPriority(bType)
	if priorityA != priorityB {
		if priorityA > priorityB {
			return 1
		}
		return -1
	}

	keyA := periodKey(aType, aPeriod)
	keyB := periodKey(bType, bPeriod)
	switch {
	case keyA > keyB:
		return 1
	case keyA < keyB:
		return -1
	default:
		return 0
	}
}

func periodPriority(periodType model.PeriodType) int {
	switch periodType {
	case model.PeriodMonth:
		return 3
	case model.PeriodQuarter:
		return 2
	case model.PeriodYear:
		return 1
	default:
		return 0
	}
}

func periodKey(periodType model.PeriodType, period string) int {
	switch periodType {
	case model.PeriodMonth:
		year, month, ok := parseYearMonth(period)
		if !ok {
			return 0
		}
		return year*100 + month
	case model.PeriodQuarter:
		year, quarter, ok := parseYearQuarter(period)
		if !ok {
			return 0
		}
		return year*10 + quarter
	case model.PeriodYear:
		year, ok := parseYear(period)
		if !ok {
			return 0
		}
		return year
	default:
		return 0
	}
}

func parseYearMonth(value string) (int, int, bool) {
	value = strings.TrimSpace(value)
	if len(value) == 6 && isDigits(value) {
		year, _ := strconv.Atoi(value[:4])
		month, _ := strconv.Atoi(value[4:])
		if month >= 1 && month <= 12 {
			return year, month, true
		}
	}

	parts := strings.Split(value, "-")
	if len(parts) == 2 && len(parts[0]) == 4 {
		year, errYear := strconv.Atoi(parts[0])
		month, errMonth := strconv.Atoi(parts[1])
		if errYear == nil && errMonth == nil && month >= 1 && month <= 12 {
			return year, month, true
		}
	}
	return 0, 0, false
}

func parseYearQuarter(value string) (int, int, bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if strings.Contains(value, "-Q") {
		parts := strings.Split(value, "-Q")
		if len(parts) == 2 {
			year, errYear := strconv.Atoi(parts[0])
			quarter, errQuarter := strconv.Atoi(parts[1])
			if errYear == nil && errQuarter == nil && quarter >= 1 && quarter <= 4 {
				return year, quarter, true
			}
		}
	}
	if strings.Contains(value, "Q") {
		parts := strings.Split(value, "Q")
		if len(parts) == 2 {
			year, errYear := strconv.Atoi(parts[0])
			quarter, errQuarter := strconv.Atoi(parts[1])
			if errYear == nil && errQuarter == nil && quarter >= 1 && quarter <= 4 {
				return year, quarter, true
			}
		}
	}
	return 0, 0, false
}

func parseYear(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if len(value) != 4 || !isDigits(value) {
		return 0, false
	}
	year, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return year, true
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func seriesKey(periodType model.PeriodType, period string) string {
	return string(periodType) + "|" + period
}

func seriesValue(series map[model.Flow]map[string]float64, flow model.Flow, periodType model.PeriodType, period string) (float64, bool) {
	if series == nil {
		return 0, false
	}
	flowSeries, ok := series[flow]
	if !ok {
		return 0, false
	}
	value, ok := flowSeries[seriesKey(periodType, period)]
	if !ok {
		return 0, false
	}
	return value, true
}

func buildGrowth(series map[model.Flow]map[string]float64, periodType model.PeriodType, period string) (string, *growthBlock) {
	prev := prevPeriod(periodType, period)
	if prev == "" {
		return "", nil
	}

	currentExport, exportOk := seriesValue(series, model.FlowExport, periodType, period)
	prevExport, prevExportOk := seriesValue(series, model.FlowExport, periodType, prev)
	currentImport, importOk := seriesValue(series, model.FlowImport, periodType, period)
	prevImport, prevImportOk := seriesValue(series, model.FlowImport, periodType, prev)

	exportGrowth := growthForValue(currentExport, prevExport, exportOk, prevExportOk)
	importGrowth := growthForValue(currentImport, prevImport, importOk, prevImportOk)

	currentTrade, tradeOk := tradeValues(series, periodType, period)
	prevTrade, prevTradeOk := tradeValues(series, periodType, prev)
	tradeGrowth := growthForValue(currentTrade, prevTrade, tradeOk, prevTradeOk)

	if exportGrowth == nil && importGrowth == nil && tradeGrowth == nil {
		return "", nil
	}

	return prev, &growthBlock{
		Export: exportGrowth,
		Import: importGrowth,
		Trade:  tradeGrowth,
	}
}

func tradeValues(series map[model.Flow]map[string]float64, periodType model.PeriodType, period string) (float64, bool) {
	exportValue, exportOk := seriesValue(series, model.FlowExport, periodType, period)
	importValue, importOk := seriesValue(series, model.FlowImport, periodType, period)
	if !exportOk || !importOk {
		return 0, false
	}
	return exportValue + importValue, true
}

func growthForValue(current, prev float64, currentOk, prevOk bool) *float64 {
	if !currentOk || !prevOk {
		return nil
	}
	if prev == 0 {
		return nil
	}
	value := (current - prev) / prev
	return &value
}

func prevPeriod(periodType model.PeriodType, period string) string {
	switch periodType {
	case model.PeriodMonth:
		year, month, ok := parseYearMonth(period)
		if !ok {
			return ""
		}
		return fmt.Sprintf("%04d-%02d", year-1, month)
	case model.PeriodQuarter:
		year, quarter, ok := parseYearQuarter(period)
		if !ok {
			return ""
		}
		return fmt.Sprintf("%04d-Q%d", year-1, quarter)
	case model.PeriodYear:
		year, ok := parseYear(period)
		if !ok {
			return ""
		}
		return fmt.Sprintf("%04d", year-1)
	default:
		return ""
	}
}

func parseList(value string) []string {
	raw := strings.Split(value, ",")
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		items = append(items, strings.ToUpper(trimmed))
	}
	return items
}

func ensureRequiredPartners(partners []string, required []string) error {
	set := make(map[string]struct{}, len(partners))
	for _, partner := range partners {
		normalized := strings.ToUpper(partner)
		if _, exists := set[normalized]; exists {
			return fmt.Errorf("duplicate partner %s", normalized)
		}
		set[normalized] = struct{}{}
	}
	requiredSet := make(map[string]struct{}, len(required))
	for _, req := range required {
		normalized := strings.ToUpper(req)
		requiredSet[normalized] = struct{}{}
		if _, ok := set[normalized]; !ok {
			return fmt.Errorf("missing partner %s", normalized)
		}
	}
	for partner := range set {
		if _, ok := requiredSet[partner]; !ok {
			return fmt.Errorf("unsupported partner %s (viewer supports USA,CHN)", partner)
		}
	}
	return nil
}

func placeholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}
