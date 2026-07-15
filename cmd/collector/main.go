package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"tradegravity/internal/model"
	"tradegravity/internal/providers"
	"tradegravity/internal/providers/comtrade"
	"tradegravity/internal/providers/wits"
	"tradegravity/internal/store"
	"tradegravity/internal/store/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		run(os.Args[2:])
	case "products":
		runProducts(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func runProducts(args []string) {
	fs := flag.NewFlagSet("products", flag.ExitOnError)
	provider := fs.String("provider", "comtrade", "product data provider id")
	primaryProvider := fs.String("primary-provider", "wits", "provider used to choose the dominant year when -year=auto")
	year := fs.String("year", "auto", "annual product period or auto")
	level := fs.Int("product-level", 2, "HS product level (currently 2)")
	partners := fs.String("partners", "USA,CHN", "comma-separated partner ISO3 list")
	flows := fs.String("flows", "export,import", "comma-separated flows")
	limit := fs.Int("limit", 0, "limit number of reporters (0 = all)")
	allowlist := fs.String("allowlist", "configs/allowlist.csv", "path to allowlist file")
	dbPath := fs.String("db", "tradegravity.db", "sqlite database path")
	concurrency := fs.Int("concurrency", 6, "maximum reporters collected concurrently")
	verbose := fs.Bool("verbose", false, "print collection progress")
	fs.Parse(args)

	if err := runProductCollector(*provider, *primaryProvider, *year, *level, *partners, *flows, *limit, *allowlist, *dbPath, *concurrency, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "product collector failed:", err)
		os.Exit(1)
	}
}

func run(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	provider := fs.String("provider", "wits", "provider id")
	partners := fs.String("partners", "USA,CHN", "comma-separated partner ISO3 list")
	flows := fs.String("flows", "export,import", "comma-separated flows")
	limit := fs.Int("limit", 0, "limit number of reporters (0 = all)")
	allowlist := fs.String("allowlist", "configs/allowlist.csv", "path to allowlist file (empty = no filter)")
	dbPath := fs.String("db", "tradegravity.db", "sqlite database path (empty disables persistence)")
	historyYears := fs.Int("history-years", 1, "number of previous years to fetch for growth (0 = latest only)")
	concurrency := fs.Int("concurrency", 6, "maximum reporters collected concurrently")
	verbose := fs.Bool("verbose", false, "print each observation")
	fs.Parse(args)

	if err := runCollector(*provider, *partners, *flows, *limit, *allowlist, *dbPath, *historyYears, *concurrency, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "collector run failed:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: collector run [options]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "options:")
	fmt.Fprintln(os.Stderr, "  -provider    provider id (default: wits)")
	fmt.Fprintln(os.Stderr, "  -partners    comma-separated partner ISO3 list (default: USA,CHN)")
	fmt.Fprintln(os.Stderr, "  -flows       comma-separated flows (default: export,import)")
	fmt.Fprintln(os.Stderr, "  -limit       limit number of reporters (default: 0)")
	fmt.Fprintln(os.Stderr, "  -allowlist   path to allowlist file (default: configs/allowlist.csv)")
	fmt.Fprintln(os.Stderr, "  -db          sqlite database path (default: tradegravity.db)")
	fmt.Fprintln(os.Stderr, "  -history-years  number of previous years to fetch (default: 1)")
	fmt.Fprintln(os.Stderr, "  -concurrency maximum concurrent reporters (default: 6)")
	fmt.Fprintln(os.Stderr, "  -verbose     print each observation")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "product breakdown: collector products [options]")
}

func runCollector(providerID, partnersCSV, flowsCSV string, limit int, allowlistPath, dbPath string, historyYears, concurrency int, verbose bool) (runErr error) {
	provider, err := buildProvider(providerID)
	if err != nil {
		return err
	}

	ctx := context.Background()

	st, err := openStore(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	runRecord := model.IngestRun{
		RunID:     newRunID(providerID, "totals"),
		Provider:  providerID,
		Mode:      "totals",
		StartedAt: time.Now().UTC(),
	}
	defer func() {
		runRecord.FinishedAt = time.Now().UTC()
		runRecord.Status = ingestStatus(runRecord, runErr)
		if runErr != nil {
			runRecord.Errors = appendLimited(runRecord.Errors, runErr.Error())
		}
		if err := st.RecordIngestRun(context.Background(), runRecord); err != nil && runErr == nil {
			runErr = err
		}
	}()

	allowed := map[string]struct{}{}
	if strings.TrimSpace(allowlistPath) != "" {
		loaded, err := loadAllowlist(allowlistPath)
		if err != nil {
			return err
		}
		allowed = loaded
	}

	reporters, err := resolveReporters(ctx, provider)
	if err != nil {
		if len(allowed) == 0 {
			return err
		}
		fmt.Fprintf(os.Stderr, "warning: %v (using allowlist only)\n", err)
		reporters = reportersFromAllowlist(allowed)
	} else if len(allowed) > 0 {
		reporters = filterReporters(reporters, allowed)
	}
	if limit > 0 && len(reporters) > limit {
		reporters = reporters[:limit]
	}
	if len(reporters) == 0 {
		return errors.New("no reporters after filtering")
	}
	runRecord.ReporterCount = len(reporters)

	partners := parseList(partnersCSV)
	if len(partners) == 0 {
		return errors.New("no partners provided")
	}

	flowList, err := parseFlows(flowsCSV)
	if err != nil {
		return err
	}

	type totalResult struct {
		reporter, partner string
		flow              model.Flow
		series            []model.Observation
		err               error
		requested         bool
	}
	workerCount := max(1, min(concurrency, len(reporters)))
	reporterJobs := make(chan model.Reporter)
	results := make(chan totalResult, workerCount*2)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for reporter := range reporterJobs {
				for _, partner := range partners {
					for _, flow := range flowList {
						if strings.EqualFold(reporter.ISO3, partner) {
							results <- totalResult{reporter: reporter.ISO3, partner: partner, flow: flow}
							continue
						}
						series, fetchErr := collectObservations(ctx, provider, st, providerID, reporter.ISO3, partner, flow, historyYears)
						results <- totalResult{reporter: reporter.ISO3, partner: partner, flow: flow, series: series, err: fetchErr, requested: true}
					}
				}
			}
		}()
	}
	go func() {
		for _, reporter := range reporters {
			reporterJobs <- reporter
		}
		close(reporterJobs)
		workers.Wait()
		close(results)
	}()
	var quotaErr error
	var persistErr error
	for result := range results {
		if !result.requested {
			runRecord.SkippedCount++
			if verbose {
				fmt.Fprintf(os.Stderr, "skip same-country reporter=%s partner=%s flow=%s\n", result.reporter, result.partner, result.flow)
			}
			continue
		}
		runRecord.RequestCount++
		if result.err != nil {
			if errors.Is(result.err, wits.ErrNoRecords) || errors.Is(result.err, comtrade.ErrNoRecords) {
				runRecord.SkippedCount++
				continue
			}
			if errors.Is(result.err, comtrade.ErrQuotaExceeded) {
				quotaErr = result.err
			}
			runRecord.FailureCount++
			runRecord.Errors = appendLimited(runRecord.Errors, fmt.Sprintf("%s/%s/%s: %v", result.reporter, result.partner, result.flow, result.err))
			fmt.Fprintf(os.Stderr, "fetch failed reporter=%s partner=%s flow=%s: %v\n", result.reporter, result.partner, result.flow, result.err)
			continue
		}
		if len(result.series) == 0 {
			runRecord.SkippedCount++
			continue
		}
		if persistErr != nil {
			continue
		}
		if err := st.UpsertObservations(ctx, result.series); err != nil {
			persistErr = err
			continue
		}
		runRecord.SuccessCount++
		runRecord.StoredCount += len(result.series)
		if verbose {
			for _, observation := range result.series {
				fmt.Printf("%s %s %s %s %s %.2f\n", observation.ReporterISO3, observation.PartnerISO3, observation.Flow, observation.PeriodType, observation.Period, observation.ValueUSD)
			}
		}
	}
	if persistErr != nil {
		return persistErr
	}
	if quotaErr != nil {
		return quotaErr
	}

	if runRecord.StoredCount > 0 {
		fmt.Printf("collector stored observations=%d\n", runRecord.StoredCount)
	}
	fmt.Printf("collector run complete (provider=%s reporters=%d requests=%d success=%d failed=%d)\n",
		providerID, len(reporters), runRecord.RequestCount, runRecord.SuccessCount, runRecord.FailureCount,
	)
	if runRecord.SkippedCount > 0 {
		fmt.Printf("collector run skipped=%d\n", runRecord.SkippedCount)
	}
	return nil
}

func runProductCollector(providerID, primaryProvider, year string, level int, partnersCSV, flowsCSV string, limit int, allowlistPath, dbPath string, concurrency int, verbose bool) (runErr error) {
	provider, err := buildProvider(providerID)
	if err != nil {
		return err
	}
	productProvider, ok := provider.(providers.ProductProvider)
	if !ok {
		return fmt.Errorf("provider %s does not support product breakdowns", providerID)
	}
	ctx := context.Background()
	st, err := openStore(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	runRecord := model.IngestRun{
		RunID:     newRunID(providerID, "products-hs2"),
		Provider:  providerID,
		Mode:      "products-hs2",
		StartedAt: time.Now().UTC(),
	}
	defer func() {
		runRecord.FinishedAt = time.Now().UTC()
		runRecord.Status = ingestStatus(runRecord, runErr)
		if runErr != nil {
			runRecord.Errors = appendLimited(runRecord.Errors, runErr.Error())
		}
		if err := st.RecordIngestRun(context.Background(), runRecord); err != nil && runErr == nil {
			runErr = err
		}
	}()

	selectedYear := strings.TrimSpace(year)
	if strings.EqualFold(selectedYear, "auto") {
		selectedYear, err = st.DominantAnnualPeriod(ctx, primaryProvider)
		if err != nil {
			return err
		}
	}
	if _, ok := parseYear(selectedYear); !ok {
		return fmt.Errorf("product year must be a four-digit annual period, got %q", selectedYear)
	}

	allowed, err := loadAllowlist(allowlistPath)
	if err != nil {
		return err
	}
	reporters, err := resolveReporters(ctx, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v (using allowlist only)\n", err)
		reporters = reportersFromAllowlist(allowed)
	} else {
		reporters = filterReporters(reporters, allowed)
	}
	if limit > 0 && len(reporters) > limit {
		reporters = reporters[:limit]
	}
	if len(reporters) == 0 {
		return errors.New("no reporters after filtering")
	}
	runRecord.ReporterCount = len(reporters)
	partners := parseList(partnersCSV)
	flows, err := parseFlows(flowsCSV)
	if err != nil {
		return err
	}

	type productResult struct {
		reporter, partner string
		flow              model.Flow
		observations      []model.Observation
		err               error
		requested         bool
	}
	workerCount := max(1, min(concurrency, len(reporters)))
	reporterJobs := make(chan model.Reporter)
	results := make(chan productResult, workerCount*2)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for reporter := range reporterJobs {
				for _, partner := range partners {
					for _, flow := range flows {
						if strings.EqualFold(reporter.ISO3, partner) {
							results <- productResult{reporter: reporter.ISO3, partner: partner, flow: flow}
							continue
						}
						observations, fetchErr := productProvider.FetchProducts(ctx, reporter.ISO3, partner, flow, selectedYear, level)
						results <- productResult{reporter: reporter.ISO3, partner: partner, flow: flow, observations: observations, err: fetchErr, requested: true}
					}
				}
			}
		}()
	}
	go func() {
		for _, reporter := range reporters {
			reporterJobs <- reporter
		}
		close(reporterJobs)
		workers.Wait()
		close(results)
	}()
	var persistErr error
	for result := range results {
		if !result.requested {
			runRecord.SkippedCount++
			continue
		}
		runRecord.RequestCount++
		if result.err != nil {
			if errors.Is(result.err, wits.ErrNoRecords) || errors.Is(result.err, comtrade.ErrNoRecords) {
				runRecord.SkippedCount++
				continue
			}
			runRecord.FailureCount++
			runRecord.Errors = appendLimited(runRecord.Errors, fmt.Sprintf("%s/%s/%s: %v", result.reporter, result.partner, result.flow, result.err))
			fmt.Fprintf(os.Stderr, "product fetch failed reporter=%s partner=%s flow=%s: %v\n", result.reporter, result.partner, result.flow, result.err)
			continue
		}
		if persistErr != nil {
			continue
		}
		if err := st.UpsertObservations(ctx, result.observations); err != nil {
			persistErr = err
			continue
		}
		runRecord.SuccessCount++
		runRecord.StoredCount += len(result.observations)
		if verbose {
			fmt.Printf("products reporter=%s partner=%s flow=%s year=%s rows=%d\n", result.reporter, result.partner, result.flow, selectedYear, len(result.observations))
		}
	}
	if persistErr != nil {
		return persistErr
	}
	if runRecord.SuccessCount == 0 {
		return errors.New("no product observations collected")
	}
	fmt.Printf("product collector complete (provider=%s year=%s level=%d reporters=%d requests=%d success=%d failed=%d observations=%d)\n",
		providerID, selectedYear, level, len(reporters), runRecord.RequestCount, runRecord.SuccessCount, runRecord.FailureCount, runRecord.StoredCount)
	return nil
}

func newRunID(provider, mode string) string {
	return fmt.Sprintf("%d-%s-%s", time.Now().UTC().UnixNano(), strings.ToLower(strings.TrimSpace(provider)), mode)
}

func ingestStatus(run model.IngestRun, runErr error) string {
	if runErr != nil || (run.SuccessCount == 0 && run.FailureCount > 0) {
		return "failed"
	}
	if run.FailureCount > 0 {
		return "partial"
	}
	return "success"
}

func appendLimited(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || len(values) >= 50 {
		return values
	}
	return append(values, value)
}

func collectObservations(ctx context.Context, provider providers.Provider, st store.Store, providerID, reporterISO3, partnerISO3 string, flow model.Flow, historyYears int) ([]model.Observation, error) {
	existingKeys, err := existingObservationKeys(ctx, st, providerID, reporterISO3, partnerISO3, flow)
	if err != nil {
		return nil, err
	}

	latest, err := provider.FetchLatest(ctx, reporterISO3, partnerISO3, flow)
	if err != nil {
		return nil, err
	}
	if historyYears <= 0 {
		if _, exists := existingKeys[observationKey(latest.PeriodType, latest.Period)]; exists {
			return nil, nil
		}
		return []model.Observation{latest}, nil
	}

	year, ok := yearFromPeriod(latest.PeriodType, latest.Period)
	if !ok {
		return []model.Observation{latest}, nil
	}
	fromYear := year - historyYears
	if fromYear < 0 {
		fromYear = 0
	}

	fetched, err := provider.FetchSeries(ctx, reporterISO3, partnerISO3, flow, fmt.Sprintf("%04d", fromYear), fmt.Sprintf("%04d", year))
	if err != nil {
		if !errors.Is(err, wits.ErrNoRecords) && !errors.Is(err, comtrade.ErrNoRecords) {
			return nil, err
		}
		fetched = nil
	}
	series := make([]model.Observation, 0, len(fetched))
	for _, observation := range fetched {
		if _, exists := existingKeys[observationKey(observation.PeriodType, observation.Period)]; exists {
			continue
		}
		series = append(series, observation)
	}
	if len(series) == 0 {
		if _, exists := existingKeys[observationKey(latest.PeriodType, latest.Period)]; exists {
			return nil, nil
		}
		return []model.Observation{latest}, nil
	}
	return series, nil
}

func existingObservationKeys(ctx context.Context, st store.Store, providerID, reporterISO3, partnerISO3 string, flow model.Flow) (map[string]struct{}, error) {
	keys := make(map[string]struct{})
	if st == nil {
		return keys, nil
	}
	existing, err := st.ListObservationKeys(ctx, providerID, reporterISO3, partnerISO3, flow)
	if err != nil {
		return nil, err
	}
	for _, key := range existing {
		keys[observationKey(key.PeriodType, key.Period)] = struct{}{}
	}
	return keys, nil
}

func yearFromPeriod(periodType model.PeriodType, period string) (int, bool) {
	switch periodType {
	case model.PeriodMonth:
		year, _, ok := parseYearMonth(period)
		return year, ok
	case model.PeriodQuarter:
		year, _, ok := parseYearQuarter(period)
		return year, ok
	case model.PeriodYear:
		return parseYear(period)
	default:
		return 0, false
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

func observationKey(periodType model.PeriodType, period string) string {
	return string(periodType) + "|" + strings.TrimSpace(period)
}

func buildProvider(providerID string) (providers.Provider, error) {
	switch strings.ToLower(strings.TrimSpace(providerID)) {
	case "wits":
		return wits.New()
	case "comtrade":
		return comtrade.New()
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerID)
	}
}

func openStore(path string) (store.Store, error) {
	if strings.TrimSpace(path) == "" {
		return &store.NopStore{}, nil
	}
	return sqlite.New(path)
}

func resolveReporters(ctx context.Context, provider providers.Provider) ([]model.Reporter, error) {
	reporters, err := provider.ListReporters(ctx)
	if err != nil {
		return nil, err
	}
	return filterActiveReporters(reporters), nil
}

func reportersFromAllowlist(allowed map[string]struct{}) []model.Reporter {
	reporters := make([]model.Reporter, 0, len(allowed))
	for iso3 := range allowed {
		trimmed := strings.TrimSpace(strings.ToUpper(iso3))
		if trimmed == "" || trimmed == "ISO3" {
			continue
		}
		reporters = append(reporters, model.Reporter{
			ISO3:     trimmed,
			NameEN:   trimmed,
			NameKO:   "",
			Region:   "",
			IsActive: true,
		})
	}
	return reporters
}

func loadAllowlist(path string) (map[string]struct{}, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	allowed := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		for _, token := range splitTokens(line) {
			iso3 := strings.ToUpper(strings.TrimSpace(token))
			if iso3 == "" || iso3 == "ISO3" {
				continue
			}
			allowed[iso3] = struct{}{}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(allowed) == 0 {
		return nil, errors.New("allowlist is empty")
	}
	return allowed, nil
}

func splitTokens(line string) []string {
	replacer := strings.NewReplacer(";", ",", "\t", ",")
	line = replacer.Replace(line)
	parts := strings.Split(line, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func filterReporters(reporters []model.Reporter, allowed map[string]struct{}) []model.Reporter {
	if len(allowed) == 0 {
		return reporters
	}
	filtered := make([]model.Reporter, 0, len(reporters))
	for _, reporter := range reporters {
		if _, ok := allowed[strings.ToUpper(reporter.ISO3)]; ok {
			filtered = append(filtered, reporter)
		}
	}
	return filtered
}

func normalizeHeader(header []string) map[string]int {
	result := make(map[string]int, len(header))
	for i, value := range header {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		result[key] = i
	}
	return result
}

func getCell(record []string, header map[string]int, key string) string {
	index, ok := header[key]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func parseBool(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return true
	}
	switch trimmed {
	case "1", "true", "yes", "y":
		return true
	default:
		return false
	}
}

func filterActiveReporters(reporters []model.Reporter) []model.Reporter {
	active := make([]model.Reporter, 0, len(reporters))
	for _, reporter := range reporters {
		if reporter.IsActive {
			active = append(active, reporter)
		}
	}
	return active
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

func parseFlows(value string) ([]model.Flow, error) {
	raw := parseList(value)
	if len(raw) == 0 {
		return nil, errors.New("no flows provided")
	}

	flows := make([]model.Flow, 0, len(raw))
	for _, item := range raw {
		switch strings.ToLower(item) {
		case "export", "exports":
			flows = append(flows, model.FlowExport)
		case "import", "imports":
			flows = append(flows, model.FlowImport)
		default:
			return nil, fmt.Errorf("unknown flow: %s", item)
		}
	}
	return flows, nil
}
