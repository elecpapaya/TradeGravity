package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"tradegravity/internal/model"
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
	default:
		usage()
		os.Exit(2)
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
	verbose := fs.Bool("verbose", false, "print each observation")
	fs.Parse(args)

	if err := runCollector(*provider, *partners, *flows, *limit, *allowlist, *dbPath, *verbose); err != nil {
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
	fmt.Fprintln(os.Stderr, "  -verbose     print each observation")
}

func runCollector(providerID, partnersCSV, flowsCSV string, limit int, allowlistPath, dbPath string, verbose bool) error {
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

	reporters, err := resolveReporters(ctx, provider)
	if err != nil {
		return err
	}
	if strings.TrimSpace(allowlistPath) != "" {
		allowed, err := loadAllowlist(allowlistPath)
		if err != nil {
			return err
		}
		reporters = filterReporters(reporters, allowed)
	}
	if limit > 0 && len(reporters) > limit {
		reporters = reporters[:limit]
	}
	if len(reporters) == 0 {
		return errors.New("no reporters after filtering")
	}

	partners := parseList(partnersCSV)
	if len(partners) == 0 {
		return errors.New("no partners provided")
	}

	flowList, err := parseFlows(flowsCSV)
	if err != nil {
		return err
	}

	requests := 0
	success := 0
	failed := 0
	skipped := 0
	observations := make([]model.Observation, 0)

	for _, reporter := range reporters {
		for _, partner := range partners {
			for _, flow := range flowList {
				if strings.EqualFold(reporter.ISO3, partner) {
					skipped++
					if verbose {
						fmt.Fprintf(os.Stderr, "skip same-country reporter=%s partner=%s flow=%s\n", reporter.ISO3, partner, flow)
					}
					continue
				}
				requests++
				observation, err := provider.FetchLatest(ctx, reporter.ISO3, partner, flow)
				if err != nil {
					if errors.Is(err, wits.ErrNoRecords) {
						skipped++
						if verbose {
							fmt.Fprintf(os.Stderr, "skip no-records reporter=%s partner=%s flow=%s\n", reporter.ISO3, partner, flow)
						}
						continue
					}
					failed++
					fmt.Fprintf(os.Stderr, "fetch failed reporter=%s partner=%s flow=%s: %v\n", reporter.ISO3, partner, flow, err)
					continue
				}
				success++
				observations = append(observations, observation)
				if verbose {
					fmt.Printf("%s %s %s %s %s %.2f\n",
						observation.ReporterISO3,
						observation.PartnerISO3,
						observation.Flow,
						observation.PeriodType,
						observation.Period,
						observation.ValueUSD,
					)
				}
			}
		}
	}

	if err := st.UpsertObservations(ctx, observations); err != nil {
		return err
	}
	if len(observations) > 0 {
		fmt.Printf("collector stored observations=%d\n", len(observations))
	}
	fmt.Printf("collector run complete (provider=%s reporters=%d requests=%d success=%d failed=%d)\n",
		providerID, len(reporters), requests, success, failed,
	)
	if skipped > 0 {
		fmt.Printf("collector run skipped=%d\n", skipped)
	}
	return nil
}

func buildProvider(providerID string) (providers, error) {
	switch strings.ToLower(strings.TrimSpace(providerID)) {
	case "wits":
		return wits.New()
	default:
		return nil, fmt.Errorf("unknown provider: %s", providerID)
	}
}

type providers interface {
	ListReporters(ctx context.Context) ([]model.Reporter, error)
	FetchLatest(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow) (model.Observation, error)
}

func openStore(path string) (store.Store, error) {
	if strings.TrimSpace(path) == "" {
		return &store.NopStore{}, nil
	}
	return sqlite.New(path)
}

func resolveReporters(ctx context.Context, provider providers) ([]model.Reporter, error) {
	reporters, err := provider.ListReporters(ctx)
	if err != nil {
		return nil, err
	}
	return filterActiveReporters(reporters), nil
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
