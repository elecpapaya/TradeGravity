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
	monthlyProvider, supportsSingleReporter := provider.(providers.SelectedProductPeriodsProvider)
	batchProvider, supportsBatch := provider.(providers.SelectedProductPeriodBatchProvider)
	if !supportsSingleReporter && !supportsBatch {
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

	type request struct {
		reporters []string
		partners  []string
		flow      model.Flow
		periods   []string
		label     string
	}
	type result struct {
		label string
		rows  []model.Observation
		err   error
	}

	requests := make([]request, 0)
	if supportsBatch {
		// Six reporters × two partners × thirty reference codes stays below
		// the public preview response ceiling for a single flow and month.
		const reporterBatchSize = 6
		for start := 0; start < len(reporters); start += reporterBatchSize {
			end := min(start+reporterBatchSize, len(reporters))
			reporterBatch := make([]string, 0, end-start)
			for _, reporter := range reporters[start:end] {
				reporterBatch = append(reporterBatch, reporter.ISO3)
			}
			for _, period := range periods {
				for _, flow := range flows {
					requests = append(requests, request{
						reporters: reporterBatch,
						partners:  partners,
						flow:      flow,
						periods:   []string{period},
						label:     fmt.Sprintf("reporters=%s/partners=%s/%s/%s", strings.Join(reporterBatch, ","), strings.Join(partners, ","), flow, period),
					})
				}
			}
		}
	} else {
		for _, reporter := range reporters {
			for _, partner := range partners {
				if strings.EqualFold(reporter.ISO3, partner) {
					runRecord.SkippedCount += len(flows)
					continue
				}
				for _, flow := range flows {
					requests = append(requests, request{
						reporters: []string{reporter.ISO3},
						partners:  []string{partner},
						flow:      flow,
						periods:   periods,
						label:     fmt.Sprintf("%s/%s/%s", reporter.ISO3, partner, flow),
					})
				}
			}
		}
	}
	if len(requests) == 0 {
		return errors.New("no monthly semiconductor requests after filtering")
	}

	workerCount := max(1, min(concurrency, len(requests)))
	jobs := make(chan request)
	results := make(chan result, workerCount*2)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for request := range jobs {
				var rows []model.Observation
				var fetchErr error
				if supportsBatch {
					rows, fetchErr = batchProvider.FetchProductPeriodBatch(ctx, request.reporters, request.partners, request.flow, request.periods[0], 6, codes)
					if fetchErr == nil {
						filtered := rows[:0]
						for _, row := range rows {
							if !strings.EqualFold(row.ReporterISO3, row.PartnerISO3) {
								filtered = append(filtered, row)
							}
						}
						rows = filtered
						if len(rows) == 0 {
							fetchErr = comtrade.ErrNoRecords
						}
					}
				} else {
					rows, fetchErr = monthlyProvider.FetchProductPeriods(ctx, request.reporters[0], request.partners[0], request.flow, request.periods, 6, codes)
				}
				results <- result{label: request.label, rows: rows, err: fetchErr}
			}
		}()
	}
	go func() {
		for _, request := range requests {
			jobs <- request
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()
	var persistErr, quotaErr error
	for item := range results {
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
			runRecord.Errors = appendLimited(runRecord.Errors, fmt.Sprintf("%s: %v", item.label, item.err))
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
			fmt.Printf("chip-monthly %s rows=%d\n", item.label, len(item.rows))
		}
	}
	if persistErr != nil {
		return persistErr
	}
	if quotaErr != nil {
		return quotaErr
	}
	if runRecord.SuccessCount == 0 {
		if len(runRecord.Errors) > 0 {
			return fmt.Errorf("no monthly semiconductor observations collected; first request error: %s", runRecord.Errors[0])
		}
		return errors.New("no monthly semiconductor observations collected")
	}
	fmt.Printf("monthly semiconductor collector complete (periods=%s..%s reporters=%d requests=%d observations=%d)\n", periods[0], periods[len(periods)-1], len(reporters), runRecord.RequestCount, runRecord.StoredCount)
	return nil
}
