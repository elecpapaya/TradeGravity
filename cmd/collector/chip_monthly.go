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
	"tradegravity/internal/semiconductor"
)

func runChipMonthly(args []string) {
	fs := flag.NewFlagSet("chip-monthly", flag.ExitOnError)
	providerID := fs.String("provider", "comtrade", "monthly semiconductor trade provider id")
	through := fs.String("through", "auto", "last complete month (YYYY-MM) or auto")
	months := fs.Int("months", 12, "number of monthly periods to collect")
	referencePath := fs.String("reference", "configs/semiconductor_reference.json", "semiconductor reference JSON")
	partners := fs.String("partners", "USA,CHN", "comma-separated anchor partners")
	flowsCSV := fs.String("flows", "export,import", "comma-separated flows")
	allowlist := fs.String("allowlist", "configs/chip_connectors.csv", "focused monthly reporter allowlist")
	dbPath := fs.String("db", "tradegravity.db", "sqlite database path")
	concurrency := fs.Int("concurrency", 2, "maximum reporters collected concurrently")
	verbose := fs.Bool("verbose", false, "print collection progress")
	fs.Parse(args)

	reference, err := semiconductor.Load(*referencePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "monthly semiconductor collector failed:", err)
		os.Exit(1)
	}
	periods, err := monthlyWindow(*through, *months, time.Now().UTC())
	if err != nil {
		fmt.Fprintln(os.Stderr, "monthly semiconductor collector failed:", err)
		os.Exit(1)
	}
	if err := runChipMonthlyCollector(*providerID, periods, semiconductor.Codes(reference), *partners, *flowsCSV, *allowlist, *dbPath, *concurrency, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "monthly semiconductor collector failed:", err)
		os.Exit(1)
	}
}

func monthlyWindow(through string, months int, now time.Time) ([]string, error) {
	if months < 1 || months > 36 {
		return nil, fmt.Errorf("months must be between 1 and 36, got %d", months)
	}
	var end time.Time
	if strings.EqualFold(strings.TrimSpace(through), "auto") {
		end = time.Date(now.UTC().Year(), now.UTC().Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	} else {
		year, month, ok := parseYearMonth(through)
		if !ok {
			return nil, fmt.Errorf("through must be YYYY-MM or auto, got %q", through)
		}
		end = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	}
	periods := make([]string, months)
	for index := range months {
		period := end.AddDate(0, index-(months-1), 0)
		periods[index] = period.Format("2006-01")
	}
	return periods, nil
}

func runChipMonthlyCollector(providerID string, periods, codes []string, partnersCSV, flowsCSV, allowlistPath, dbPath string, concurrency int, verbose bool) (runErr error) {
	provider, err := buildProvider(providerID)
	if err != nil {
		return err
	}
	monthlyProvider, ok := provider.(providers.SelectedProductPeriodsProvider)
	if !ok {
		return fmt.Errorf("provider %s does not support selected monthly product periods", providerID)
	}
	allowed, err := loadAllowlist(allowlistPath)
	if err != nil {
		return err
	}
	ctx := context.Background()
	reporters, err := resolveReporters(ctx, provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v (using focused allowlist only)\n", err)
		reporters = reportersFromAllowlist(allowed)
	} else {
		reporters = filterReporters(reporters, allowed)
	}
	if len(reporters) == 0 {
		return errors.New("no monthly semiconductor reporters after filtering")
	}
	partners := parseList(partnersCSV)
	flows, err := parseFlows(flowsCSV)
	if err != nil {
		return err
	}
	st, err := openStore(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	runRecord := model.IngestRun{
		RunID: newRunID(providerID, "products-semiconductor-monthly-hs6"), Provider: providerID,
		Mode: "products-semiconductor-monthly-hs6", StartedAt: time.Now().UTC(), ReporterCount: len(reporters),
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

	type result struct {
		reporter, partner string
		flow              model.Flow
		rows              []model.Observation
		err               error
		requested         bool
	}
	workerCount := max(1, min(concurrency, len(reporters)))
	jobs := make(chan model.Reporter)
	results := make(chan result, workerCount*2)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for reporter := range jobs {
				for _, partner := range partners {
					for _, flow := range flows {
						if strings.EqualFold(reporter.ISO3, partner) {
							results <- result{reporter: reporter.ISO3, partner: partner, flow: flow}
							continue
						}
						rows, fetchErr := monthlyProvider.FetchProductPeriods(ctx, reporter.ISO3, partner, flow, periods, 6, codes)
						results <- result{reporter: reporter.ISO3, partner: partner, flow: flow, rows: rows, err: fetchErr, requested: true}
					}
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
	var persistErr, quotaErr error
	for item := range results {
		if !item.requested {
			runRecord.SkippedCount++
			continue
		}
		runRecord.RequestCount++
		if item.err != nil {
			if errors.Is(item.err, comtrade.ErrNoRecords) {
				runRecord.SkippedCount++
				continue
			}
			if errors.Is(item.err, comtrade.ErrQuotaExceeded) {
				quotaErr = item.err
			}
			runRecord.FailureCount++
			runRecord.Errors = appendLimited(runRecord.Errors, fmt.Sprintf("%s/%s/%s: %v", item.reporter, item.partner, item.flow, item.err))
			continue
		}
		if persistErr != nil {
			continue
		}
		if err := st.UpsertObservations(ctx, item.rows); err != nil {
			persistErr = err
			continue
		}
		runRecord.SuccessCount++
		runRecord.StoredCount += len(item.rows)
		if verbose {
			fmt.Printf("chip-monthly reporter=%s partner=%s flow=%s rows=%d\n", item.reporter, item.partner, item.flow, len(item.rows))
		}
	}
	if persistErr != nil {
		return persistErr
	}
	if quotaErr != nil {
		return quotaErr
	}
	if runRecord.SuccessCount == 0 {
		return errors.New("no monthly semiconductor observations collected")
	}
	fmt.Printf("monthly semiconductor collector complete (periods=%s..%s reporters=%d requests=%d observations=%d)\n", periods[0], periods[len(periods)-1], len(reporters), runRecord.RequestCount, runRecord.StoredCount)
	return nil
}
