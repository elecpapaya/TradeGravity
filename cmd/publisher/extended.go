package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"time"

	"tradegravity/internal/model"
	"tradegravity/internal/semiconductor"
	"tradegravity/internal/strategic"
)

type contextMetric struct {
	Value *float64 `json:"value"`
	Year  string   `json:"year"`
}

type contextCountry struct {
	ISO3        string        `json:"iso3"`
	ISO2        string        `json:"iso2"`
	Name        string        `json:"name"`
	Region      string        `json:"region"`
	IncomeGroup string        `json:"income_group"`
	Groups      []string      `json:"groups"`
	Population  contextMetric `json:"population"`
	GDP         contextMetric `json:"gdp"`
}

type contextDataset struct {
	Status    string           `json:"status"`
	Countries []contextCountry `json:"countries"`
}

type seriesFile struct {
	SchemaVersion string           `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Provider      string           `json:"provider"`
	Partners      []string         `json:"partners"`
	Rows          []reporterSeries `json:"rows"`
}

type reporterSeries struct {
	ISO3   string        `json:"iso3"`
	Points []seriesPoint `json:"points"`
}

type seriesPoint struct {
	PeriodType model.PeriodType `json:"period_type"`
	Period     string           `json:"period"`
	USA        seriesBlock      `json:"usa"`
	CHN        seriesBlock      `json:"chn"`
	Total      float64          `json:"total"`
	ShareCN    float64          `json:"share_cn"`
	Comparable bool             `json:"comparable"`
}

type seriesBlock struct {
	Available bool    `json:"available"`
	Export    float64 `json:"export"`
	Import    float64 `json:"import"`
	Trade     float64 `json:"trade"`
}

type productIndexFile struct {
	SchemaVersion  string   `json:"schema_version"`
	GeneratedAt    string   `json:"generated_at"`
	Provider       string   `json:"provider"`
	Classification string   `json:"classification"`
	Level          int      `json:"level"`
	Partners       []string `json:"partners"`
	Periods        []string `json:"periods"`
	Reporters      []string `json:"reporters"`
}

type productFile struct {
	SchemaVersion  string         `json:"schema_version"`
	GeneratedAt    string         `json:"generated_at"`
	Provider       string         `json:"provider"`
	Classification string         `json:"classification"`
	Level          int            `json:"level"`
	ReporterISO3   string         `json:"reporter_iso3"`
	Periods        []string       `json:"periods"`
	Rows           []productEntry `json:"rows"`
}

type strategicIndexFile struct {
	SchemaVersion    string                       `json:"schema_version"`
	GeneratedAt      string                       `json:"generated_at"`
	Provider         string                       `json:"provider"`
	Level            int                          `json:"level"`
	Partners         []string                     `json:"partners"`
	Sectors          []string                     `json:"sectors"`
	Products         []strategicProductDescriptor `json:"products"`
	Reporters        []string                     `json:"reporters"`
	Periods          []string                     `json:"periods"`
	Partitions       []strategicPartition         `json:"partitions"`
	ObservationCount int                          `json:"observation_count"`
}

type strategicProductDescriptor struct {
	Code         string `json:"code"`
	Sector       string `json:"sector"`
	Label        string `json:"label"`
	RevisionNote string `json:"revision_note"`
	Notes        string `json:"notes,omitempty"`
}

type strategicPartition struct {
	ReporterISO3 string `json:"reporter_iso3"`
	Period       string `json:"period"`
	Href         string `json:"href"`
	RowCount     int    `json:"row_count"`
}

type strategicFile struct {
	SchemaVersion string                  `json:"schema_version"`
	GeneratedAt   string                  `json:"generated_at"`
	Provider      string                  `json:"provider"`
	Level         int                     `json:"level"`
	Partners      []string                `json:"partners"`
	ReporterISO3  string                  `json:"reporter_iso3"`
	Period        string                  `json:"period"`
	Rows          []strategicProductEntry `json:"rows"`
}

type strategicProductEntry struct {
	Classification string      `json:"classification"`
	Code           string      `json:"code"`
	Sector         string      `json:"sector"`
	Label          string      `json:"label"`
	RevisionNote   string      `json:"revision_note"`
	USA            seriesBlock `json:"usa"`
	CHN            seriesBlock `json:"chn"`
	Total          float64     `json:"total"`
	ShareCN        float64     `json:"share_cn"`
}

type tariffIndexFile struct {
	SchemaVersion    string                       `json:"schema_version"`
	GeneratedAt      string                       `json:"generated_at"`
	Provider         string                       `json:"provider"`
	Level            int                          `json:"level"`
	Importers        []string                     `json:"importers"`
	Exporters        []string                     `json:"exporters"`
	Years            []string                     `json:"years"`
	DataTypes        []string                     `json:"data_types"`
	RateTypes        []string                     `json:"rate_types"`
	Products         []strategicProductDescriptor `json:"products"`
	Partitions       []tariffPartition            `json:"partitions"`
	ObservationCount int                          `json:"observation_count"`
}

type tariffPartition struct {
	ImporterISO3 string `json:"importer_iso3"`
	Year         string `json:"year"`
	Href         string `json:"href"`
	RowCount     int    `json:"row_count"`
}

type tariffFile struct {
	SchemaVersion string               `json:"schema_version"`
	GeneratedAt   string               `json:"generated_at"`
	Provider      string               `json:"provider"`
	Level         int                  `json:"level"`
	ImporterISO3  string               `json:"importer_iso3"`
	Year          string               `json:"year"`
	Rows          []tariffPublishedRow `json:"rows"`
}

type tariffPublishedRow struct {
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

type tariffObservationRow struct {
	Provider          string
	Classification    string
	ProductCode       string
	ProductLevel      int
	ImporterISO3      string
	ExporterISO3      string
	ExporterCode      string
	DataType          string
	RateType          string
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
	SourceUpdatedAt   string
}

type matrixIndexFile struct {
	SchemaVersion    string            `json:"schema_version"`
	GeneratedAt      string            `json:"generated_at"`
	Provider         string            `json:"provider"`
	ProductCode      string            `json:"product_code"`
	ProductLevel     int               `json:"product_level"`
	Reporters        []string          `json:"reporters"`
	Partners         []string          `json:"partners"`
	Periods          []string          `json:"periods"`
	Partitions       []matrixPartition `json:"partitions"`
	PartnerRowCount  int               `json:"partner_row_count"`
	ObservationCount int               `json:"observation_count"`
}

type matrixPartition struct {
	ReporterISO3 string `json:"reporter_iso3"`
	Period       string `json:"period"`
	Href         string `json:"href"`
	RowCount     int    `json:"row_count"`
}

type matrixFile struct {
	SchemaVersion string          `json:"schema_version"`
	GeneratedAt   string          `json:"generated_at"`
	Provider      string          `json:"provider"`
	ProductCode   string          `json:"product_code"`
	ProductLevel  int             `json:"product_level"`
	ReporterISO3  string          `json:"reporter_iso3"`
	Period        string          `json:"period"`
	Rows          []matrixPartner `json:"rows"`
}

type matrixPartner struct {
	PartnerISO3     string  `json:"partner_iso3"`
	ExportAvailable bool    `json:"export_available"`
	ImportAvailable bool    `json:"import_available"`
	ExportUSD       float64 `json:"export_usd"`
	ImportUSD       float64 `json:"import_usd"`
	TradeUSD        float64 `json:"trade_usd"`
	BalanceUSD      float64 `json:"balance_usd"`
}

type dataCatalogFile struct {
	SchemaVersion string            `json:"schema_version"`
	GeneratedAt   string            `json:"generated_at"`
	Resources     []catalogResource `json:"resources"`
}

type catalogResource struct {
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

type productEntry struct {
	PeriodType model.PeriodType `json:"period_type"`
	Period     string           `json:"period"`
	Code       string           `json:"code"`
	Name       string           `json:"name"`
	USA        seriesBlock      `json:"usa"`
	CHN        seriesBlock      `json:"chn"`
	Total      float64          `json:"total"`
	ShareCN    float64          `json:"share_cn"`
}

type ingestRunRecord struct {
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

type qualityFile struct {
	SchemaVersion      string               `json:"schema_version"`
	GeneratedAt        string               `json:"generated_at"`
	PrimaryProvider    string               `json:"primary_provider"`
	DominantPeriod     string               `json:"dominant_period"`
	Summary            qualitySummary       `json:"summary"`
	ReporterIssues     []reporterIssue      `json:"reporter_issues"`
	CollectionRuns     []ingestRunRecord    `json:"collection_runs"`
	ProviderComparison []providerComparison `json:"provider_comparison"`
}

type qualitySummary struct {
	ReporterCount         int `json:"reporter_count"`
	ComparableReporters   int `json:"comparable_reporters"`
	IncomparableReporters int `json:"incomparable_reporters"`
	MissingPartnerBlocks  int `json:"missing_partner_blocks"`
	StalePartnerBlocks    int `json:"stale_partner_blocks"`
	ComparisonCount       int `json:"provider_comparison_count"`
}

type reporterIssue struct {
	ISO3      string   `json:"iso3"`
	USAPeriod string   `json:"usa_period,omitempty"`
	CHNPeriod string   `json:"chn_period,omitempty"`
	Issues    []string `json:"issues"`
}

type providerComparison struct {
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

func buildDataCatalog(generatedAt, provider, contextStatus string, series seriesFile, products productIndexFile, strategicIndex strategicIndexFile, tariffIndex tariffIndexFile, matrixIndex matrixIndexFile, mirrorIndex mirrorIndexFile, semiconductorMonthlyIndex semiconductorMonthlyIndexFile, publicationChanges publicationChangesFile, briefing briefingFile, semiconductorReferences ...semiconductor.Reference) dataCatalogFile {
	semiconductorReference := semiconductor.Reference{}
	if len(semiconductorReferences) > 0 {
		semiconductorReference = semiconductorReferences[0]
	}
	primaryProvider := strings.ToLower(strings.TrimSpace(provider))
	productStatus := "partial"
	if len(products.Reporters) > 0 {
		productStatus = "ready"
	}
	countryContextStatus := "partial"
	if strings.EqualFold(contextStatus, "success") {
		countryContextStatus = "ready"
	}
	strategicStatus := "partial"
	if len(strategicIndex.Partitions) > 0 {
		strategicStatus = "ready"
	}
	tariffStatus := "partial"
	if len(tariffIndex.Partitions) > 0 {
		tariffStatus = "ready"
	}
	matrixStatus := "partial"
	if len(matrixIndex.Partitions) > 0 {
		matrixStatus = "ready"
	}
	mirrorStatus := "partial"
	if len(mirrorIndex.Partitions) > 0 {
		mirrorStatus = "ready"
	}
	semiconductorStatus := "partial"
	if semiconductorReference.Publication.Status == "research_ready" {
		semiconductorStatus = "ready"
	}
	semiconductorMonthlyStatus := "partial"
	if len(semiconductorMonthlyIndex.Partitions) > 0 {
		semiconductorMonthlyStatus = "ready"
	}
	publicationChangesStatus := "partial"
	if publicationChanges.Status == "changed" || publicationChanges.Status == "unchanged" {
		publicationChangesStatus = "ready"
	}
	briefingStatus := "partial"
	if briefing.Status == "ready" {
		briefingStatus = "ready"
	}
	return dataCatalogFile{
		SchemaVersion: "1.0",
		GeneratedAt:   generatedAt,
		Resources: []catalogResource{
			{ID: "headline_totals", Title: "Headline bilateral totals", Status: "ready", Provider: primaryProvider, Grain: "reporter × USA/CHN partner × flow × latest period", Partitioning: "single publication", Href: "./latest.json"},
			{ID: "time_series", Title: "Headline time series", Status: statusForCount(len(series.Rows)), Provider: primaryProvider, Grain: "reporter × USA/CHN partner × flow × period", Partitioning: "single publication", Href: "./series.json"},
			{ID: "country_context", Title: "Country economic context", Status: countryContextStatus, Provider: "world_bank", Grain: "reporter × indicator × year", Partitioning: "single publication", Href: "./context.json"},
			{ID: "product_chapters", Title: "Product chapter observations", Status: productStatus, Provider: strings.ToLower(products.Provider), Classification: products.Classification, ProductLevel: products.Level, Grain: "reporter × partner × flow × HS2 × period", Partitioning: "index + one file per reporter", Href: "./products/index.json"},
			{ID: "quality", Title: "Quality and provenance signals", Status: "ready", Provider: "tradegravity", Grain: "publication + reporter/provider issue", Partitioning: "single publication", Href: "./quality.json"},
			{ID: "strategic_hs6", Title: "Curated strategic HS6 products", Status: strategicStatus, Provider: strategicIndex.Provider, Classification: "source HS revision", ProductLevel: 6, Grain: "reporter × partner × flow × HS6 × period × source classification", Partitioning: "index + reporter/year chunks", Href: "./strategic-hs6/index.json"},
			{ID: "tariff_schedules", Title: "Tariff schedules", Status: tariffStatus, Provider: tariffIndex.Provider, Classification: "source HS revision", ProductLevel: 6, Grain: "importer × exporter/regime × HS6 × year × data type", Partitioning: "index + importer/year chunks", Href: "./tariffs/index.json"},
			{ID: "bilateral_matrix", Title: "Multi-partner bilateral matrix", Status: matrixStatus, Provider: matrixIndex.Provider, ProductLevel: 0, Grain: "reporter × partner × flow × TOTAL × annual period", Partitioning: "index + reporter/year chunks", Href: "./bilateral-matrix/index.json"},
			{ID: "semiconductor_atlas", Title: "Semiconductor value-chain atlas", Status: semiconductorStatus, Provider: "tradegravity + cited official sources", Classification: "stage-mapped source HS revision", ProductLevel: 6, Grain: "stage taxonomy + country role context + policy event + published HS6 coverage", Partitioning: "reference publication + strategic HS6 reporter/year chunks", Href: "./semiconductors/reference.json"},
			{ID: "semiconductor_monthly", Title: "Focused US-China semiconductor turning points", Status: semiconductorMonthlyStatus, Provider: semiconductorMonthlyIndex.Provider, Classification: "source HS revision", ProductLevel: 6, Grain: "focused reporter × USA/CHN partner × flow × selected HS6 × month", Partitioning: "index + one file per reporter", Href: "./semiconductors/monthly/index.json"},
			{ID: "publication_changes", Title: "Observed publication changes", Status: publicationChangesStatus, Provider: "tradegravity", Classification: "source HS revision", ProductLevel: 6, Grain: "publication × focused reporter × month × selected HS6", Partitioning: "single bounded change feed", Href: "./changes.json"},
			{ID: "distribution_briefing", Title: "Deterministic email and social briefing", Status: briefingStatus, Provider: "tradegravity", Classification: "source HS revision", ProductLevel: 6, Grain: "edition × selected monthly observation × distribution channel", Partitioning: "single reviewed-draft publication", Href: "./briefing.json"},
			{ID: "mirror_reconciliation", Title: "Unadjusted mirror-reporting diagnostics", Status: mirrorStatus, Provider: mirrorIndex.Provider, ProductLevel: 0, Grain: "third-country reporter × USA/CHN anchor × mirrored flow × TOTAL × annual period", Partitioning: "index + reporter/year chunks", Href: "./mirror/index.json"},
			{ID: "value_added_network", Title: "Value-added supply-chain exposure", Status: "planned", Grain: "origin × destination × industry × year", Partitioning: "year/industry chunks"},
			{ID: "scenario_runs", Title: "Versioned scenario outputs", Status: "planned", Grain: "scenario × market × product × partner", Partitioning: "one manifest and result set per run"},
		},
	}
}

func statusForCount(count int) string {
	if count > 0 {
		return "ready"
	}
	return "partial"
}

func loadContext(path string) (contextDataset, error) {
	var dataset contextDataset
	if strings.TrimSpace(path) == "" {
		dataset.Status = "missing"
		return dataset, nil
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		dataset.Status = "missing"
		return dataset, nil
	}
	if err != nil {
		return dataset, err
	}
	defer file.Close()
	if err := json.NewDecoder(file).Decode(&dataset); err != nil {
		return dataset, err
	}
	if dataset.Status == "" {
		dataset.Status = "unknown"
	}
	return dataset, nil
}

func enrichLatest(rows []latestEntry, countries []contextCountry) {
	byISO := make(map[string]contextCountry, len(countries))
	for _, country := range countries {
		byISO[strings.ToUpper(country.ISO3)] = country
	}
	for index := range rows {
		country, ok := byISO[rows[index].ISO3]
		if !ok {
			continue
		}
		rows[index].ISO2 = country.ISO2
		rows[index].Name = country.Name
		rows[index].Region = country.Region
		rows[index].IncomeGroup = country.IncomeGroup
		rows[index].Groups = append([]string(nil), country.Groups...)
		rows[index].Population = country.Population
		rows[index].GDP = country.GDP
	}
}

func buildSeriesFile(generatedAt, provider string, partners []string, observations []observationRow, maxYears int) seriesFile {
	grouped := make(map[string]map[string]*seriesPoint)
	for _, row := range observations {
		reporter := strings.ToUpper(strings.TrimSpace(row.ReporterISO))
		if reporter == "" {
			continue
		}
		if grouped[reporter] == nil {
			grouped[reporter] = make(map[string]*seriesPoint)
		}
		key := seriesKey(row.PeriodType, row.Period)
		point := grouped[reporter][key]
		if point == nil {
			point = &seriesPoint{PeriodType: row.PeriodType, Period: row.Period}
			grouped[reporter][key] = point
		}
		var block *seriesBlock
		switch strings.ToUpper(row.PartnerISO) {
		case "USA":
			block = &point.USA
		case "CHN":
			block = &point.CHN
		default:
			continue
		}
		block.Available = true
		switch row.Flow {
		case model.FlowExport:
			block.Export = row.ValueUSD
		case model.FlowImport:
			block.Import = row.ValueUSD
		}
	}

	output := seriesFile{
		SchemaVersion: schemaVersion,
		GeneratedAt:   generatedAt,
		Provider:      strings.ToLower(strings.TrimSpace(provider)),
		Partners:      append([]string(nil), partners...),
		Rows:          []reporterSeries{},
	}
	for reporter, pointsByPeriod := range grouped {
		points := make([]seriesPoint, 0, len(pointsByPeriod))
		maxYear := 0
		for _, point := range pointsByPeriod {
			point.USA.Trade = point.USA.Export + point.USA.Import
			point.CHN.Trade = point.CHN.Export + point.CHN.Import
			point.Total = point.USA.Trade + point.CHN.Trade
			if point.Total > 0 {
				point.ShareCN = point.CHN.Trade / point.Total
			}
			point.Comparable = point.USA.Available && point.CHN.Available
			if year := yearForPeriod(point.PeriodType, point.Period); year > maxYear {
				maxYear = year
			}
			points = append(points, *point)
		}
		if maxYears > 0 && maxYear > 0 {
			minimumYear := maxYear - maxYears + 1
			filtered := points[:0]
			for _, point := range points {
				if year := yearForPeriod(point.PeriodType, point.Period); year == 0 || year >= minimumYear {
					filtered = append(filtered, point)
				}
			}
			points = filtered
		}
		sort.Slice(points, func(i, j int) bool {
			return comparePeriods(points[i].PeriodType, points[i].Period, points[j].PeriodType, points[j].Period) < 0
		})
		output.Rows = append(output.Rows, reporterSeries{ISO3: reporter, Points: points})
	}
	sort.Slice(output.Rows, func(i, j int) bool { return output.Rows[i].ISO3 < output.Rows[j].ISO3 })
	return output
}

func yearForPeriod(periodType model.PeriodType, period string) int {
	switch periodType {
	case model.PeriodYear:
		year, _ := parseYear(period)
		return year
	case model.PeriodQuarter:
		year, _, _ := parseYearQuarter(period)
		return year
	case model.PeriodMonth:
		year, _, _ := parseYearMonth(period)
		return year
	default:
		return 0
	}
}

func loadProductObservations(dbPath, provider string, level int, partners []string) ([]observationRow, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	query := `SELECT provider, classification, product_code, product_level,
		reporter_iso3, partner_iso3, flow, period_type, period, value_usd
		FROM trade_observations
		WHERE provider = ? AND product_level = ? AND flow IN ('export','import')`
	args := []any{strings.ToLower(strings.TrimSpace(provider)), level}
	if len(partners) > 0 {
		query += " AND partner_iso3 IN (" + placeholders(len(partners)) + ")"
		for _, partner := range partners {
			args = append(args, partner)
		}
	}
	rows, err := db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []observationRow
	for rows.Next() {
		var row observationRow
		var flow, periodType string
		if err := rows.Scan(&row.Provider, &row.Classification, &row.ProductCode, &row.ProductLevel,
			&row.ReporterISO, &row.PartnerISO, &flow, &periodType, &row.Period, &row.ValueUSD); err != nil {
			return nil, err
		}
		row.Flow = model.Flow(strings.ToLower(flow))
		row.PeriodType = model.PeriodType(strings.ToUpper(periodType))
		results = append(results, row)
	}
	return results, rows.Err()
}

func loadProductLabels(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, err
	}
	labels := make(map[string]string)
	for index, record := range records {
		if index == 0 || len(record) < 2 {
			continue
		}
		code := strings.TrimSpace(record[0])
		if len(code) == 1 {
			code = "0" + code
		}
		if len(code) == 2 {
			labels[code] = strings.TrimSpace(record[1])
		}
	}
	if len(labels) == 0 {
		return nil, errors.New("HS2 label file is empty")
	}
	return labels, nil
}

func buildProductFiles(generatedAt, provider string, level int, partners []string, observations []observationRow, labels map[string]string) (productIndexFile, map[string]productFile) {
	type productKey struct{ periodKey, code string }
	grouped := make(map[string]map[productKey]*productEntry)
	classification := "HS"
	periodSet := make(map[string]struct{})
	for _, row := range observations {
		reporter := strings.ToUpper(row.ReporterISO)
		if reporter == "" || row.ProductCode == "" {
			continue
		}
		if row.Classification != "" {
			classification = strings.ToUpper(row.Classification)
		}
		if grouped[reporter] == nil {
			grouped[reporter] = make(map[productKey]*productEntry)
		}
		key := productKey{periodKey: seriesKey(row.PeriodType, row.Period), code: row.ProductCode}
		entry := grouped[reporter][key]
		if entry == nil {
			entry = &productEntry{PeriodType: row.PeriodType, Period: row.Period, Code: row.ProductCode, Name: labels[row.ProductCode]}
			if entry.Name == "" {
				entry.Name = "HS " + row.ProductCode
			}
			grouped[reporter][key] = entry
		}
		block := &entry.USA
		if strings.ToUpper(row.PartnerISO) == "CHN" {
			block = &entry.CHN
		} else if strings.ToUpper(row.PartnerISO) != "USA" {
			continue
		}
		block.Available = true
		if row.Flow == model.FlowExport {
			block.Export += row.ValueUSD
		} else if row.Flow == model.FlowImport {
			block.Import += row.ValueUSD
		}
		periodSet[row.Period] = struct{}{}
	}

	index := productIndexFile{
		SchemaVersion: schemaVersion, GeneratedAt: generatedAt,
		Provider: strings.ToLower(strings.TrimSpace(provider)), Classification: classification,
		Level: level, Partners: append([]string(nil), partners...), Periods: []string{}, Reporters: []string{},
	}
	files := make(map[string]productFile)
	for reporter, entriesByKey := range grouped {
		file := productFile{
			SchemaVersion: schemaVersion, GeneratedAt: generatedAt, Provider: index.Provider,
			Classification: classification, Level: level, ReporterISO3: reporter,
			Periods: []string{}, Rows: []productEntry{},
		}
		filePeriodSet := make(map[string]struct{})
		for _, entry := range entriesByKey {
			entry.USA.Trade = entry.USA.Export + entry.USA.Import
			entry.CHN.Trade = entry.CHN.Export + entry.CHN.Import
			entry.Total = entry.USA.Trade + entry.CHN.Trade
			if entry.Total > 0 {
				entry.ShareCN = entry.CHN.Trade / entry.Total
			}
			file.Rows = append(file.Rows, *entry)
			filePeriodSet[entry.Period] = struct{}{}
		}
		for period := range filePeriodSet {
			file.Periods = append(file.Periods, period)
		}
		sort.Sort(sort.Reverse(sort.StringSlice(file.Periods)))
		sort.Slice(file.Rows, func(i, j int) bool {
			if file.Rows[i].Period != file.Rows[j].Period {
				return file.Rows[i].Period > file.Rows[j].Period
			}
			if file.Rows[i].Total != file.Rows[j].Total {
				return file.Rows[i].Total > file.Rows[j].Total
			}
			return file.Rows[i].Code < file.Rows[j].Code
		})
		files[reporter] = file
		index.Reporters = append(index.Reporters, reporter)
	}
	for period := range periodSet {
		index.Periods = append(index.Periods, period)
	}
	sort.Strings(index.Reporters)
	sort.Sort(sort.Reverse(sort.StringSlice(index.Periods)))
	return index, files
}

func buildStrategicFiles(generatedAt, provider string, partners []string, observations []observationRow, products []strategic.Product) (strategicIndexFile, map[string]strategicFile) {
	type entryKey struct {
		classification string
		code           string
	}
	type partitionKey struct {
		reporter string
		period   string
	}

	descriptors := make(map[string]strategicProductDescriptor, len(products))
	index := strategicIndexFile{
		SchemaVersion: schemaVersion,
		GeneratedAt:   generatedAt,
		Provider:      strings.ToLower(strings.TrimSpace(provider)),
		Level:         6,
		Partners:      append([]string(nil), partners...),
		Sectors:       strategic.Sectors(products),
		Products:      make([]strategicProductDescriptor, 0, len(products)),
		Reporters:     []string{},
		Periods:       []string{},
		Partitions:    []strategicPartition{},
	}
	for _, product := range products {
		descriptor := strategicProductDescriptor{
			Code:         product.Code,
			Sector:       product.Sector,
			Label:        product.Label,
			RevisionNote: product.RevisionNote,
			Notes:        product.Notes,
		}
		descriptors[product.Code] = descriptor
		index.Products = append(index.Products, descriptor)
	}

	grouped := make(map[partitionKey]map[entryKey]*strategicProductEntry)
	for _, row := range observations {
		descriptor, ok := descriptors[row.ProductCode]
		if !ok || row.ProductLevel != 6 || row.PeriodType != model.PeriodYear {
			continue
		}
		reporter := strings.ToUpper(strings.TrimSpace(row.ReporterISO))
		if reporter == "" || row.Period == "" {
			continue
		}
		partner := strings.ToUpper(strings.TrimSpace(row.PartnerISO))
		if partner != "USA" && partner != "CHN" {
			continue
		}
		classification := strings.ToUpper(strings.TrimSpace(row.Classification))
		if classification == "" {
			classification = "HS"
		}
		pkey := partitionKey{reporter: reporter, period: row.Period}
		if grouped[pkey] == nil {
			grouped[pkey] = make(map[entryKey]*strategicProductEntry)
		}
		ekey := entryKey{classification: classification, code: row.ProductCode}
		entry := grouped[pkey][ekey]
		if entry == nil {
			entry = &strategicProductEntry{
				Classification: classification,
				Code:           descriptor.Code,
				Sector:         descriptor.Sector,
				Label:          descriptor.Label,
				RevisionNote:   descriptor.RevisionNote,
			}
			grouped[pkey][ekey] = entry
		}
		block := &entry.USA
		if partner == "CHN" {
			block = &entry.CHN
		}
		block.Available = true
		if row.Flow == model.FlowExport {
			block.Export += row.ValueUSD
		} else if row.Flow == model.FlowImport {
			block.Import += row.ValueUSD
		}
		index.ObservationCount++
	}

	files := make(map[string]strategicFile, len(grouped))
	reporterSet := make(map[string]struct{})
	periodSet := make(map[string]struct{})
	keys := make([]partitionKey, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].reporter != keys[j].reporter {
			return keys[i].reporter < keys[j].reporter
		}
		return keys[i].period > keys[j].period
	})
	for _, key := range keys {
		file := strategicFile{
			SchemaVersion: schemaVersion,
			GeneratedAt:   generatedAt,
			Provider:      index.Provider,
			Level:         6,
			Partners:      append([]string(nil), partners...),
			ReporterISO3:  key.reporter,
			Period:        key.period,
			Rows:          []strategicProductEntry{},
		}
		for _, entry := range grouped[key] {
			entry.USA.Trade = entry.USA.Export + entry.USA.Import
			entry.CHN.Trade = entry.CHN.Export + entry.CHN.Import
			entry.Total = entry.USA.Trade + entry.CHN.Trade
			if entry.Total > 0 {
				entry.ShareCN = entry.CHN.Trade / entry.Total
			}
			file.Rows = append(file.Rows, *entry)
		}
		sort.Slice(file.Rows, func(i, j int) bool {
			if file.Rows[i].Sector != file.Rows[j].Sector {
				return file.Rows[i].Sector < file.Rows[j].Sector
			}
			if file.Rows[i].Total != file.Rows[j].Total {
				return file.Rows[i].Total > file.Rows[j].Total
			}
			if file.Rows[i].Code != file.Rows[j].Code {
				return file.Rows[i].Code < file.Rows[j].Code
			}
			return file.Rows[i].Classification < file.Rows[j].Classification
		})
		relativePath := key.reporter + "/" + key.period + ".json"
		files[relativePath] = file
		index.Partitions = append(index.Partitions, strategicPartition{
			ReporterISO3: key.reporter,
			Period:       key.period,
			Href:         "./" + relativePath,
			RowCount:     len(file.Rows),
		})
		reporterSet[key.reporter] = struct{}{}
		periodSet[key.period] = struct{}{}
	}
	for reporter := range reporterSet {
		index.Reporters = append(index.Reporters, reporter)
	}
	for period := range periodSet {
		index.Periods = append(index.Periods, period)
	}
	sort.Strings(index.Reporters)
	sort.Sort(sort.Reverse(sort.StringSlice(index.Periods)))
	return index, files
}

func loadTariffObservations(dbPath, provider string) ([]tariffObservationRow, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	columns, err := sqliteTableColumns(db, "tariff_observations")
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return []tariffObservationRow{}, nil
	}
	column := func(name, fallback string) string {
		if _, ok := columns[name]; ok {
			return name
		}
		return fallback + " AS " + name
	}
	query := `SELECT provider, classification, product_code, product_level,
		importer_iso3, exporter_iso3, ` + column("exporter_code", "''") + `,
		` + column("data_type", "'reported'") + `, rate_type, regime, year, rate_percent,
		` + column("sum_rate_percent", "NULL") + `, ` + column("min_rate_percent", "NULL") + `,
		` + column("max_rate_percent", "NULL") + `, ` + column("total_lines", "0") + `,
		` + column("preferential_lines", "0") + `, ` + column("mfn_lines", "0") + `,
		` + column("non_ad_valorem_lines", "0") + `, ` + column("nomenclature", "''") + `,
		` + column("excluded_from", "''") + `, ` + column("source_updated_at", "NULL") + `
		FROM tariff_observations WHERE product_level = 6`
	args := []any{}
	if strings.TrimSpace(provider) != "" {
		query += " AND provider = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(provider)))
	}
	rows, err := db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]tariffObservationRow, 0)
	for rows.Next() {
		var row tariffObservationRow
		var sumRate, minRate, maxRate sql.NullFloat64
		var sourceUpdated sql.NullString
		if err := rows.Scan(
			&row.Provider, &row.Classification, &row.ProductCode, &row.ProductLevel,
			&row.ImporterISO3, &row.ExporterISO3, &row.ExporterCode, &row.DataType,
			&row.RateType, &row.Regime, &row.Year, &row.RatePercent,
			&sumRate, &minRate, &maxRate, &row.TotalLines, &row.PreferentialLines,
			&row.MFNLines, &row.NonAdValoremLines, &row.Nomenclature, &row.ExcludedFrom,
			&sourceUpdated,
		); err != nil {
			return nil, err
		}
		row.SumRatePercent = nullableFloat(sumRate)
		row.MinRatePercent = nullableFloat(minRate)
		row.MaxRatePercent = nullableFloat(maxRate)
		if sourceUpdated.Valid {
			row.SourceUpdatedAt = sourceUpdated.String
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func sqliteTableColumns(db *sql.DB, table string) (map[string]struct{}, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make(map[string]struct{})
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[strings.ToLower(name)] = struct{}{}
	}
	return columns, rows.Err()
}

func nullableFloat(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	result := value.Float64
	return &result
}

func buildTariffFiles(generatedAt, provider string, observations []tariffObservationRow, products []strategic.Product) (tariffIndexFile, map[string]tariffFile) {
	type partitionKey struct {
		importer string
		year     string
	}
	descriptors := make(map[string]strategicProductDescriptor, len(products))
	index := tariffIndexFile{
		SchemaVersion: schemaVersion, GeneratedAt: generatedAt, Provider: strings.ToLower(strings.TrimSpace(provider)), Level: 6,
		Importers: []string{}, Exporters: []string{}, Years: []string{}, DataTypes: []string{}, RateTypes: []string{},
		Products: []strategicProductDescriptor{}, Partitions: []tariffPartition{},
	}
	for _, product := range products {
		descriptor := strategicProductDescriptor{Code: product.Code, Sector: product.Sector, Label: product.Label, RevisionNote: product.RevisionNote, Notes: product.Notes}
		descriptors[product.Code] = descriptor
		index.Products = append(index.Products, descriptor)
	}
	grouped := make(map[partitionKey][]tariffPublishedRow)
	importers := make(map[string]struct{})
	exporters := make(map[string]struct{})
	years := make(map[string]struct{})
	dataTypes := make(map[string]struct{})
	rateTypes := make(map[string]struct{})
	for _, observation := range observations {
		descriptor, ok := descriptors[strings.TrimSpace(observation.ProductCode)]
		if !ok || observation.ProductLevel != 6 {
			continue
		}
		importer := strings.ToUpper(strings.TrimSpace(observation.ImporterISO3))
		exporter := strings.ToUpper(strings.TrimSpace(observation.ExporterISO3))
		year := strings.TrimSpace(observation.Year)
		if len(importer) != 3 || len(exporter) != 3 || len(year) != 4 {
			continue
		}
		row := tariffPublishedRow{
			Classification: strings.ToUpper(strings.TrimSpace(observation.Classification)),
			Nomenclature:   strings.ToUpper(strings.TrimSpace(observation.Nomenclature)),
			Code:           descriptor.Code, Sector: descriptor.Sector, Label: descriptor.Label,
			ExporterISO3: exporter, ExporterCode: strings.ToUpper(strings.TrimSpace(observation.ExporterCode)),
			DataType: strings.ToLower(strings.TrimSpace(observation.DataType)), RateType: strings.ToLower(strings.TrimSpace(observation.RateType)),
			Regime: strings.ToLower(strings.TrimSpace(observation.Regime)), RatePercent: observation.RatePercent,
			SumRatePercent: observation.SumRatePercent, MinRatePercent: observation.MinRatePercent, MaxRatePercent: observation.MaxRatePercent,
			TotalLines: observation.TotalLines, PreferentialLines: observation.PreferentialLines, MFNLines: observation.MFNLines,
			NonAdValoremLines: observation.NonAdValoremLines, ExcludedFrom: strings.ToUpper(strings.TrimSpace(observation.ExcludedFrom)),
			SourceUpdatedAt: normalizePublishedTime(observation.SourceUpdatedAt),
		}
		key := partitionKey{importer: importer, year: year}
		grouped[key] = append(grouped[key], row)
		importers[importer] = struct{}{}
		exporters[exporter] = struct{}{}
		years[year] = struct{}{}
		dataTypes[row.DataType] = struct{}{}
		rateTypes[row.RateType] = struct{}{}
		index.ObservationCount++
	}
	keys := make([]partitionKey, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].importer != keys[j].importer {
			return keys[i].importer < keys[j].importer
		}
		return keys[i].year > keys[j].year
	})
	files := make(map[string]tariffFile, len(keys))
	for _, key := range keys {
		partitionRows := grouped[key]
		sort.Slice(partitionRows, func(i, j int) bool {
			if partitionRows[i].Sector != partitionRows[j].Sector {
				return partitionRows[i].Sector < partitionRows[j].Sector
			}
			if partitionRows[i].Code != partitionRows[j].Code {
				return partitionRows[i].Code < partitionRows[j].Code
			}
			if partitionRows[i].ExporterISO3 != partitionRows[j].ExporterISO3 {
				return partitionRows[i].ExporterISO3 < partitionRows[j].ExporterISO3
			}
			if partitionRows[i].DataType != partitionRows[j].DataType {
				return partitionRows[i].DataType < partitionRows[j].DataType
			}
			return partitionRows[i].RateType < partitionRows[j].RateType
		})
		relativePath := key.importer + "/" + key.year + ".json"
		files[relativePath] = tariffFile{
			SchemaVersion: schemaVersion, GeneratedAt: generatedAt, Provider: index.Provider, Level: 6,
			ImporterISO3: key.importer, Year: key.year, Rows: partitionRows,
		}
		index.Partitions = append(index.Partitions, tariffPartition{ImporterISO3: key.importer, Year: key.year, Href: "./" + relativePath, RowCount: len(partitionRows)})
	}
	for value := range importers {
		index.Importers = append(index.Importers, value)
	}
	for value := range exporters {
		index.Exporters = append(index.Exporters, value)
	}
	for value := range years {
		index.Years = append(index.Years, value)
	}
	for value := range dataTypes {
		index.DataTypes = append(index.DataTypes, value)
	}
	for value := range rateTypes {
		index.RateTypes = append(index.RateTypes, value)
	}
	sort.Strings(index.Importers)
	sort.Strings(index.Exporters)
	sort.Sort(sort.Reverse(sort.StringSlice(index.Years)))
	sort.Strings(index.DataTypes)
	sort.Strings(index.RateTypes)
	return index, files
}

func normalizePublishedTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05.999999999 -0700 MST"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format(time.RFC3339)
		}
	}
	return value
}

func loadMatrixObservations(dbPath, provider string) ([]observationRow, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	query := `SELECT provider, reporter_iso3, partner_iso3, flow, period_type, period,
		MAX(value_usd), MAX(classification), 'TOTAL', 0
		FROM trade_observations
		WHERE product_level = 0 AND product_code = 'TOTAL' AND period_type = 'Y'
			AND flow IN ('export','import') AND partner_iso3 <> 'WLD' AND partner_iso3 <> reporter_iso3`
	args := []any{}
	if strings.TrimSpace(provider) != "" {
		query += " AND provider = ?"
		args = append(args, strings.ToLower(strings.TrimSpace(provider)))
	}
	query += ` GROUP BY provider, reporter_iso3, partner_iso3, flow, period_type, period`
	rows, err := db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]observationRow, 0)
	for rows.Next() {
		var row observationRow
		if err := rows.Scan(&row.Provider, &row.ReporterISO, &row.PartnerISO, &row.Flow, &row.PeriodType, &row.Period, &row.ValueUSD, &row.Classification, &row.ProductCode, &row.ProductLevel); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func buildMatrixFiles(generatedAt, provider string, observations []observationRow) (matrixIndexFile, map[string]matrixFile) {
	type partitionKey struct {
		reporter string
		period   string
	}
	index := matrixIndexFile{
		SchemaVersion: schemaVersion, GeneratedAt: generatedAt, Provider: strings.ToLower(strings.TrimSpace(provider)),
		ProductCode: "TOTAL", ProductLevel: 0, Reporters: []string{}, Partners: []string{}, Periods: []string{}, Partitions: []matrixPartition{},
	}
	grouped := make(map[partitionKey]map[string]*matrixPartner)
	partnerSet := make(map[string]struct{})
	for _, observation := range observations {
		if observation.ProductLevel != 0 || strings.ToUpper(strings.TrimSpace(observation.ProductCode)) != "TOTAL" || observation.PeriodType != model.PeriodYear {
			continue
		}
		reporter := strings.ToUpper(strings.TrimSpace(observation.ReporterISO))
		partner := strings.ToUpper(strings.TrimSpace(observation.PartnerISO))
		period := strings.TrimSpace(observation.Period)
		if !isPublishedISO3(reporter) || !isPublishedISO3(partner) || partner == "WLD" || partner == reporter || len(period) != 4 || observation.ValueUSD < 0 {
			continue
		}
		key := partitionKey{reporter: reporter, period: period}
		if grouped[key] == nil {
			grouped[key] = make(map[string]*matrixPartner)
		}
		entry := grouped[key][partner]
		if entry == nil {
			entry = &matrixPartner{PartnerISO3: partner}
			grouped[key][partner] = entry
		}
		switch observation.Flow {
		case model.FlowExport:
			entry.ExportAvailable = true
			entry.ExportUSD = observation.ValueUSD
		case model.FlowImport:
			entry.ImportAvailable = true
			entry.ImportUSD = observation.ValueUSD
		default:
			continue
		}
		partnerSet[partner] = struct{}{}
		index.ObservationCount++
	}

	keys := make([]partitionKey, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].reporter != keys[j].reporter {
			return keys[i].reporter < keys[j].reporter
		}
		return keys[i].period > keys[j].period
	})
	files := make(map[string]matrixFile, len(keys))
	reporterSet := make(map[string]struct{})
	periodSet := make(map[string]struct{})
	for _, key := range keys {
		file := matrixFile{
			SchemaVersion: schemaVersion, GeneratedAt: generatedAt, Provider: index.Provider, ProductCode: "TOTAL", ProductLevel: 0,
			ReporterISO3: key.reporter, Period: key.period, Rows: []matrixPartner{},
		}
		for _, entry := range grouped[key] {
			entry.TradeUSD = entry.ExportUSD + entry.ImportUSD
			entry.BalanceUSD = entry.ExportUSD - entry.ImportUSD
			file.Rows = append(file.Rows, *entry)
		}
		sort.Slice(file.Rows, func(i, j int) bool {
			if file.Rows[i].TradeUSD != file.Rows[j].TradeUSD {
				return file.Rows[i].TradeUSD > file.Rows[j].TradeUSD
			}
			return file.Rows[i].PartnerISO3 < file.Rows[j].PartnerISO3
		})
		relativePath := key.reporter + "/" + key.period + ".json"
		files[relativePath] = file
		index.Partitions = append(index.Partitions, matrixPartition{ReporterISO3: key.reporter, Period: key.period, Href: "./" + relativePath, RowCount: len(file.Rows)})
		index.PartnerRowCount += len(file.Rows)
		reporterSet[key.reporter] = struct{}{}
		periodSet[key.period] = struct{}{}
	}
	for value := range reporterSet {
		index.Reporters = append(index.Reporters, value)
	}
	for value := range partnerSet {
		index.Partners = append(index.Partners, value)
	}
	for value := range periodSet {
		index.Periods = append(index.Periods, value)
	}
	sort.Strings(index.Reporters)
	sort.Strings(index.Partners)
	sort.Sort(sort.Reverse(sort.StringSlice(index.Periods)))
	return index, files
}

func isPublishedISO3(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return false
		}
	}
	return true
}

func loadIngestRuns(dbPath string, limit int) ([]ingestRunRecord, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT run_id, provider, mode, started_at, finished_at, status,
		reporter_count, request_count, success_count, failure_count, skipped_count, stored_count, errors_json
		FROM ingest_runs ORDER BY finished_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []ingestRunRecord
	for rows.Next() {
		var item ingestRunRecord
		var errorsJSON string
		if err := rows.Scan(&item.RunID, &item.Provider, &item.Mode, &item.StartedAt, &item.FinishedAt, &item.Status,
			&item.ReporterCount, &item.RequestCount, &item.SuccessCount, &item.FailureCount,
			&item.SkippedCount, &item.StoredCount, &errorsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(errorsJSON), &item.Errors)
		if item.Errors == nil {
			item.Errors = []string{}
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func buildQualityFile(generatedAt, primaryProvider string, latest []latestEntry, primaryRows, productRows []observationRow, runs []ingestRunRecord) qualityFile {
	dominant := dominantLatestPeriod(latest)
	output := qualityFile{
		SchemaVersion: schemaVersion, GeneratedAt: generatedAt,
		PrimaryProvider: strings.ToLower(strings.TrimSpace(primaryProvider)),
		DominantPeriod:  dominant, CollectionRuns: runs,
		ReporterIssues: []reporterIssue{}, ProviderComparison: []providerComparison{},
	}
	if output.CollectionRuns == nil {
		output.CollectionRuns = []ingestRunRecord{}
	}
	output.Summary.ReporterCount = len(latest)
	for _, row := range latest {
		issue := reporterIssue{ISO3: row.ISO3, USAPeriod: row.USA.Period, CHNPeriod: row.CHN.Period}
		if row.USA.Period == "" {
			issue.Issues = append(issue.Issues, "missing_usa")
			output.Summary.MissingPartnerBlocks++
		}
		if row.CHN.Period == "" {
			issue.Issues = append(issue.Issues, "missing_chn")
			output.Summary.MissingPartnerBlocks++
		}
		if row.SamePeriod {
			output.Summary.ComparableReporters++
		} else {
			output.Summary.IncomparableReporters++
			issue.Issues = append(issue.Issues, "mixed_or_missing_periods")
		}
		for label, block := range map[string]partnerBlock{"usa": row.USA, "chn": row.CHN} {
			if block.Period != "" && string(block.PeriodType)+":"+block.Period != dominant {
				issue.Issues = append(issue.Issues, "stale_"+label)
				output.Summary.StalePartnerBlocks++
			}
		}
		if len(issue.Issues) > 0 {
			output.ReporterIssues = append(output.ReporterIssues, issue)
		}
	}
	output.ProviderComparison = compareProviders(primaryProvider, primaryRows, productRows)
	output.Summary.ComparisonCount = len(output.ProviderComparison)
	return output
}

func dominantLatestPeriod(latest []latestEntry) string {
	counts := make(map[string]int)
	for _, row := range latest {
		for _, block := range []partnerBlock{row.USA, row.CHN} {
			if block.Period != "" {
				counts[string(block.PeriodType)+":"+block.Period]++
			}
		}
	}
	best := ""
	bestCount := -1
	for key, count := range counts {
		if count > bestCount || (count == bestCount && key > best) {
			best, bestCount = key, count
		}
	}
	return best
}

type flowTotal struct {
	export, imported     float64
	hasExport, hasImport bool
	provider             string
	reporter             string
	partner              string
	periodType           model.PeriodType
	period               string
}

func compareProviders(primaryProvider string, primaryRows, productRows []observationRow) []providerComparison {
	primary := aggregateFlows(primaryRows, false)
	secondary := aggregateFlows(productRows, true)
	var comparisons []providerComparison
	for key, left := range primary {
		right, ok := secondary[key]
		if !ok || !left.hasExport || !left.hasImport || !right.hasExport || !right.hasImport {
			continue
		}
		primaryTrade := left.export + left.imported
		secondaryTrade := right.export + right.imported
		if primaryTrade <= 0 {
			continue
		}
		comparisons = append(comparisons, providerComparison{
			ISO3: left.reporter, Partner: left.partner, PeriodType: string(left.periodType), Period: left.period,
			PrimaryProvider: strings.ToLower(strings.TrimSpace(primaryProvider)), SecondaryProvider: right.provider,
			PrimaryTradeUSD: primaryTrade, SecondaryTradeUSD: secondaryTrade,
			DeltaRatio: (secondaryTrade - primaryTrade) / primaryTrade,
		})
	}
	sort.Slice(comparisons, func(i, j int) bool {
		if comparisons[i].ISO3 != comparisons[j].ISO3 {
			return comparisons[i].ISO3 < comparisons[j].ISO3
		}
		if comparisons[i].Partner != comparisons[j].Partner {
			return comparisons[i].Partner < comparisons[j].Partner
		}
		return comparisons[i].Period < comparisons[j].Period
	})
	return comparisons
}

func aggregateFlows(rows []observationRow, sumProducts bool) map[string]*flowTotal {
	values := make(map[string]*flowTotal)
	for _, row := range rows {
		key := strings.Join([]string{strings.ToUpper(row.ReporterISO), strings.ToUpper(row.PartnerISO), string(row.PeriodType), row.Period}, "|")
		item := values[key]
		if item == nil {
			item = &flowTotal{provider: strings.ToLower(row.Provider), reporter: strings.ToUpper(row.ReporterISO), partner: strings.ToUpper(row.PartnerISO), periodType: row.PeriodType, period: row.Period}
			values[key] = item
		}
		if row.Flow == model.FlowExport {
			if sumProducts {
				item.export += row.ValueUSD
			} else {
				item.export = row.ValueUSD
			}
			item.hasExport = true
		} else if row.Flow == model.FlowImport {
			if sumProducts {
				item.imported += row.ValueUSD
			} else {
				item.imported = row.ValueUSD
			}
			item.hasImport = true
		}
	}
	return values
}

func augmentMeta(meta *metaFile, latest []latestEntry, series seriesFile, products productIndexFile, productObservationCount int, contextStatus string) {
	if meta == nil {
		return
	}
	meta.DominantPeriod = dominantLatestPeriod(latest)
	for _, row := range latest {
		if row.SamePeriod {
			meta.ComparableReporters++
		} else {
			meta.IncomparableReporters++
		}
		for _, block := range []partnerBlock{row.USA, row.CHN} {
			if block.Period != "" && string(block.PeriodType)+":"+block.Period != meta.DominantPeriod {
				meta.StalePartnerBlocks++
			}
		}
	}
	meta.SeriesReporterCount = len(series.Rows)
	for _, row := range series.Rows {
		meta.SeriesPointCount += len(row.Points)
	}
	meta.ProductProvider = products.Provider
	meta.ProductClassification = products.Classification
	meta.ProductLevel = products.Level
	meta.ProductReporterCount = len(products.Reporters)
	meta.ProductObservationCount = productObservationCount
	meta.ContextStatus = contextStatus
}

func augmentStrategicMeta(meta *metaFile, index strategicIndexFile) {
	if meta == nil {
		return
	}
	meta.StrategicProvider = index.Provider
	meta.StrategicLevel = index.Level
	meta.StrategicProductCount = len(index.Products)
	meta.StrategicReporterCount = len(index.Reporters)
	meta.StrategicPartitionCount = len(index.Partitions)
	meta.StrategicObservationCount = index.ObservationCount
}

func augmentTariffMeta(meta *metaFile, index tariffIndexFile) {
	if meta == nil {
		return
	}
	meta.TariffProvider = index.Provider
	meta.TariffImporterCount = len(index.Importers)
	meta.TariffPartitionCount = len(index.Partitions)
	meta.TariffObservationCount = index.ObservationCount
}

func augmentMatrixMeta(meta *metaFile, index matrixIndexFile) {
	if meta == nil {
		return
	}
	meta.MatrixProvider = index.Provider
	meta.MatrixReporterCount = len(index.Reporters)
	meta.MatrixPartitionCount = len(index.Partitions)
	meta.MatrixPartnerRowCount = index.PartnerRowCount
	meta.MatrixObservationCount = index.ObservationCount
}

func augmentMirrorMeta(meta *metaFile, index mirrorIndexFile) {
	if meta == nil {
		return
	}
	meta.MirrorProvider = index.Provider
	meta.MirrorReporterCount = len(index.Reporters)
	meta.MirrorPartitionCount = len(index.Partitions)
	meta.MirrorComparisonCount = index.ComparisonCount
}
