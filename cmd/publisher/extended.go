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

	"tradegravity/internal/model"
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
