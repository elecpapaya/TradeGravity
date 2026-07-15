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
	"tradegravity/internal/providers/comtrade"
)

func runMatrix(args []string) {
	fs := flag.NewFlagSet("matrix", flag.ExitOnError)
	providerID := fs.String("provider", "comtrade", "matrix provider id")
	primaryProvider := fs.String("primary-provider", "wits", "provider used to choose the dominant year when -year=auto")
	year := fs.String("year", "auto", "annual matrix period or auto")
	flowsCSV := fs.String("flows", "export,import", "comma-separated flows")
	limit := fs.Int("limit", 0, "limit number of reporters (0 = all)")
	allowlistPath := fs.String("allowlist", "configs/allowlist.csv", "path to reporter allowlist")
	dbPath := fs.String("db", "tradegravity.db", "sqlite database path")
	concurrency := fs.Int("concurrency", 2, "maximum reporters collected concurrently")
	verbose := fs.Bool("verbose", false, "print collection progress")
	fs.Parse(args)
	if err := runMatrixCollector(*providerID, *primaryProvider, *year, *flowsCSV, *limit, *allowlistPath, *dbPath, *concurrency, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "matrix collector failed:", err)
		os.Exit(1)
	}
}

func runMatrixCollector(providerID, primaryProvider, year, flowsCSV string, limit int, allowlistPath, dbPath string, concurrency int, verbose bool) (runErr error) {
	baseProvider, err := buildProvider(providerID)
	if err != nil {
		return err
	}
	provider, ok := baseProvider.(providers.PartnerMatrixProvider)
	if !ok {
		return fmt.Errorf("provider %s does not support partner matrices", providerID)
	}
	flows, err := parseFlows(flowsCSV)
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
		RunID: newRunID(provider.Name(), "bilateral-matrix"), Provider: provider.Name(),
		Mode: "bilateral-matrix", StartedAt: time.Now().UTC(),
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
		return fmt.Errorf("matrix year must be auto or four digits, got %q", selectedYear)
	}
	allowed, err := loadAllowlist(allowlistPath)
	if err != nil {
		return err
	}
	reporters, err := provider.ListReporters(ctx)
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
		return errors.New("no matrix reporters after filtering")
	}
	runRecord.ReporterCount = len(reporters)

	type matrixResult struct {
		reporter     string
		flow         model.Flow
		observations []model.Observation
		err          error
	}
	workerCount := max(1, min(concurrency, len(reporters)))
	jobs := make(chan model.Reporter)
	results := make(chan matrixResult, workerCount*2)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for reporter := range jobs {
				for _, flow := range flows {
					observations, fetchErr := provider.FetchPartnerMatrix(ctx, reporter.ISO3, flow, selectedYear)
					results <- matrixResult{reporter: reporter.ISO3, flow: flow, observations: observations, err: fetchErr}
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
	var quotaErr error
	for result := range results {
		runRecord.RequestCount++
		if result.err != nil {
			if errors.Is(result.err, comtrade.ErrNoRecords) {
				runRecord.SkippedCount++
				continue
			}
			if errors.Is(result.err, comtrade.ErrQuotaExceeded) {
				quotaErr = result.err
			}
			runRecord.FailureCount++
			runRecord.Errors = appendLimited(runRecord.Errors, fmt.Sprintf("%s/%s/%s: %v", result.reporter, result.flow, selectedYear, result.err))
			fmt.Fprintf(os.Stderr, "matrix fetch failed reporter=%s flow=%s year=%s: %v\n", result.reporter, result.flow, selectedYear, result.err)
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
			fmt.Printf("matrix reporter=%s flow=%s year=%s partners=%d\n", result.reporter, result.flow, selectedYear, len(result.observations))
		}
	}
	if persistErr != nil {
		return persistErr
	}
	if quotaErr != nil && runRecord.SuccessCount == 0 {
		return quotaErr
	}
	if runRecord.SuccessCount == 0 {
		return errors.New("no matrix observations collected")
	}
	fmt.Printf("matrix collector complete (provider=%s year=%s reporters=%d requests=%d success=%d failed=%d observations=%d)\n",
		provider.Name(), selectedYear, len(reporters), runRecord.RequestCount, runRecord.SuccessCount, runRecord.FailureCount, runRecord.StoredCount)
	return nil
}
