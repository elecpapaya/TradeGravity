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
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"tradegravity/internal/model"
)

type metaFile struct {
	GeneratedAt string `json:"generated_at"`
}

type latestFile struct {
	GeneratedAt string        `json:"generated_at"`
	Rows        []latestEntry `json:"rows"`
}

type latestEntry struct {
	ISO3    string       `json:"iso3"`
	USA     partnerBlock `json:"usa"`
	CHN     partnerBlock `json:"chn"`
	Total   float64      `json:"total"`
	ShareCN float64      `json:"share_cn"`
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
	Provider    string
	ReporterISO string
	PartnerISO  string
	Flow        model.Flow
	PeriodType  model.PeriodType
	Period      string
	ValueUSD    float64
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
	fs.Parse(args)

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "failed to create output dir:", err)
		os.Exit(1)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := writeJSON(filepath.Join(*outDir, "meta.json"), metaFile{GeneratedAt: now}); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write meta.json:", err)
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

	latest := buildLatest(rows)
	if err := writeJSON(filepath.Join(*outDir, "latest.json"), latestFile{GeneratedAt: now, Rows: latest}); err != nil {
		fmt.Fprintln(os.Stderr, "failed to write latest.json:", err)
		os.Exit(1)
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
		WHERE flow IN ('export','import')
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

		results = append(results, latestEntry{
			ISO3:    reporter,
			USA:     usa.partnerBlock,
			CHN:     chn.partnerBlock,
			Total:   total,
			ShareCN: shareCN,
		})
	}

	return results
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
		set[strings.ToUpper(partner)] = struct{}{}
	}
	for _, req := range required {
		if _, ok := set[req]; !ok {
			return fmt.Errorf("missing partner %s", req)
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
