package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"tradegravity/internal/model"
	"tradegravity/internal/providers"
	"tradegravity/internal/providers/trains"
	"tradegravity/internal/strategic"
)

func runTariffs(args []string) {
	fs := flag.NewFlagSet("tariffs", flag.ExitOnError)
	providerID := fs.String("provider", "trains", "tariff provider id")
	year := fs.String("year", "auto", "tariff year per importer or auto")
	registryPath := fs.String("registry", "configs/strategic_hs6.csv", "strategic HS6 registry CSV")
	sectorsCSV := fs.String("sectors", "all", "comma-separated strategic sectors or all")
	partnersCSV := fs.String("partners", "WLD", "comma-separated exporter/regime ISO3 codes")
	dataTypeText := fs.String("data-type", "aveestimated", "reported or aveestimated")
	limit := fs.Int("limit", 0, "limit number of importers (0 = all)")
	allowlistPath := fs.String("allowlist", "configs/allowlist.csv", "path to importer allowlist")
	dbPath := fs.String("db", "tradegravity.db", "sqlite database path")
	concurrency := fs.Int("concurrency", 3, "maximum importers collected concurrently")
	verbose := fs.Bool("verbose", false, "print collection progress")
	fs.Parse(args)

	registry, err := strategic.LoadCSV(*registryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tariff collector failed:", err)
		os.Exit(1)
	}
	selected, err := strategic.Filter(registry, strings.Split(*sectorsCSV, ","))
	if err != nil {
		fmt.Fprintln(os.Stderr, "tariff collector failed:", err)
		os.Exit(1)
	}
	dataType, err := parseTariffDataType(*dataTypeText)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tariff collector failed:", err)
		os.Exit(1)
	}
	if err := runTariffCollector(*providerID, *year, strategic.Codes(selected), *partnersCSV, dataType, *limit, *allowlistPath, *dbPath, *concurrency, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "tariff collector failed:", err)
		os.Exit(1)
	}
	fmt.Printf("tariff product selection complete (sectors=%s codes=%d)\n", strings.Join(strategic.Sectors(selected), ","), len(selected))
}

func runTariffCollector(providerID, year string, codes []string, partnersCSV string, dataType model.TariffDataType, limit int, allowlistPath, dbPath string, concurrency int, verbose bool) (runErr error) {
	provider, err := buildTariffProvider(providerID)
	if err != nil {
		return err
	}
	if len(codes) == 0 {
		return errors.New("no tariff product codes selected")
	}
	partners := parseList(partnersCSV)
	if len(partners) == 0 {
		return errors.New("no tariff partners provided")
	}
	requestedYear := strings.TrimSpace(year)
	if !strings.EqualFold(requestedYear, "auto") {
		if _, ok := parseYear(requestedYear); !ok {
			return fmt.Errorf("tariff year must be auto or four digits, got %q", requestedYear)
		}
	}

	ctx := context.Background()
	st, err := openStore(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	runRecord := model.IngestRun{
		RunID: newRunID(provider.Name(), "tariffs-strategic-hs6"), Provider: provider.Name(),
		Mode: "tariffs-strategic-hs6", StartedAt: time.Now().UTC(),
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

	allowed, err := loadAllowlist(allowlistPath)
	if err != nil {
		return err
	}
	reporters, err := provider.ListTariffImporters(ctx)
	if err != nil {
		if len(allowed) == 0 {
			return err
		}
		fmt.Fprintf(os.Stderr, "warning: %v (using allowlist only)\n", err)
		reporters = reportersFromAllowlist(allowed)
	} else {
		reporters = filterReporters(reporters, allowed)
	}
	if limit > 0 && len(reporters) > limit {
		reporters = reporters[:limit]
	}
	if len(reporters) == 0 {
		return errors.New("no tariff importers after filtering")
	}
	runRecord.ReporterCount = len(reporters)

	type tariffResult struct {
		importer, exporter, year string
		observations             []model.TariffObservation
		err                      error
		requested                bool
	}
	workerCount := max(1, min(concurrency, len(reporters)))
	jobs := make(chan model.Reporter)
	results := make(chan tariffResult, workerCount*2)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for reporter := range jobs {
				selectedYear := requestedYear
				if strings.EqualFold(selectedYear, "auto") {
					resolved, resolveErr := provider.LatestTariffYear(ctx, reporter.ISO3)
					if resolveErr != nil {
						results <- tariffResult{importer: reporter.ISO3, err: resolveErr}
						continue
					}
					selectedYear = resolved
				}
				for _, partner := range partners {
					if strings.EqualFold(reporter.ISO3, partner) {
						results <- tariffResult{importer: reporter.ISO3, exporter: partner, year: selectedYear}
						continue
					}
					observations, fetchErr := provider.FetchTariffs(ctx, reporter.ISO3, partner, selectedYear, codes, dataType)
					if errors.Is(fetchErr, trains.ErrAVEUnavailable) && dataType == model.TariffAVEEstimated {
						observations, fetchErr = provider.FetchTariffs(ctx, reporter.ISO3, partner, selectedYear, codes, model.TariffReported)
					}
					results <- tariffResult{importer: reporter.ISO3, exporter: partner, year: selectedYear, observations: observations, err: fetchErr, requested: true}
				}
			}
		}()
	}
	go func() {
		for _, reporter := range reporters {
			jobs <- reporter
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	var persistErr error
	var rateLimitErr error
	for result := range results {
		if !result.requested {
			if result.err != nil {
				runRecord.FailureCount++
				runRecord.Errors = appendLimited(runRecord.Errors, fmt.Sprintf("%s/year: %v", result.importer, result.err))
			} else {
				runRecord.SkippedCount++
			}
			continue
		}
		runRecord.RequestCount++
		if result.err != nil {
			if errors.Is(result.err, trains.ErrNoRecords) || errors.Is(result.err, trains.ErrPartnerUnavailable) {
				runRecord.SkippedCount++
				if verbose {
					fmt.Fprintf(os.Stderr, "tariff unavailable importer=%s exporter=%s year=%s: %v\n", result.importer, result.exporter, result.year, result.err)
				}
				continue
			}
			if errors.Is(result.err, trains.ErrRateLimited) {
				rateLimitErr = result.err
			}
			runRecord.FailureCount++
			runRecord.Errors = appendLimited(runRecord.Errors, fmt.Sprintf("%s/%s/%s: %v", result.importer, result.exporter, result.year, result.err))
			fmt.Fprintf(os.Stderr, "tariff fetch failed importer=%s exporter=%s year=%s: %v\n", result.importer, result.exporter, result.year, result.err)
			continue
		}
		if len(result.observations) == 0 {
			runRecord.SkippedCount++
			continue
		}
		if persistErr != nil {
			continue
		}
		if err := st.UpsertTariffObservations(ctx, result.observations); err != nil {
			persistErr = err
			continue
		}
		runRecord.SuccessCount++
		runRecord.StoredCount += len(result.observations)
		if verbose {
			fmt.Printf("tariffs importer=%s exporter=%s year=%s rows=%d data_type=%s\n", result.importer, result.exporter, result.year, len(result.observations), dataType)
		}
	}
	if persistErr != nil {
		return persistErr
	}
	if rateLimitErr != nil && runRecord.SuccessCount == 0 {
		return rateLimitErr
	}
	if runRecord.SuccessCount == 0 {
		return errors.New("no tariff observations collected")
	}
	fmt.Printf("tariff collector complete (provider=%s importers=%d requests=%d success=%d failed=%d observations=%d)\n",
		provider.Name(), len(reporters), runRecord.RequestCount, runRecord.SuccessCount, runRecord.FailureCount, runRecord.StoredCount)
	return nil
}

func buildTariffProvider(providerID string) (providers.TariffProvider, error) {
	switch strings.ToLower(strings.TrimSpace(providerID)) {
	case "trains", "wits-trains":
		return trains.New()
	default:
		return nil, fmt.Errorf("unknown tariff provider: %s", providerID)
	}
}

func parseTariffDataType(value string) (model.TariffDataType, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "reported":
		return model.TariffReported, nil
	case "aveestimated", "ave_estimated", "ave-estimated":
		return model.TariffAVEEstimated, nil
	default:
		return "", fmt.Errorf("unknown tariff data type %q", value)
	}
}
