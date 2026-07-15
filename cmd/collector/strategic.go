package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"tradegravity/internal/strategic"
)

func runStrategic(args []string) {
	fs := flag.NewFlagSet("strategic", flag.ExitOnError)
	provider := fs.String("provider", "comtrade", "strategic product data provider id")
	primaryProvider := fs.String("primary-provider", "wits", "provider used to choose the dominant year when -year=auto")
	year := fs.String("year", "auto", "annual strategic-product period or auto")
	registryPath := fs.String("registry", "configs/strategic_hs6.csv", "strategic HS6 registry CSV")
	sectorsCSV := fs.String("sectors", "all", "comma-separated strategic sectors or all")
	partners := fs.String("partners", "USA,CHN", "comma-separated partner ISO3 list")
	flows := fs.String("flows", "export,import", "comma-separated flows")
	limit := fs.Int("limit", 0, "limit number of reporters (0 = all)")
	allowlist := fs.String("allowlist", "configs/allowlist.csv", "path to allowlist file")
	dbPath := fs.String("db", "tradegravity.db", "sqlite database path")
	concurrency := fs.Int("concurrency", 6, "maximum reporters collected concurrently")
	verbose := fs.Bool("verbose", false, "print collection progress")
	fs.Parse(args)

	registry, err := strategic.LoadCSV(*registryPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "strategic collector failed:", err)
		os.Exit(1)
	}
	selected, err := strategic.Filter(registry, strings.Split(*sectorsCSV, ","))
	if err != nil {
		fmt.Fprintln(os.Stderr, "strategic collector failed:", err)
		os.Exit(1)
	}
	if err := runProductCollector(*provider, *primaryProvider, *year, 6, strategic.Codes(selected), *partners, *flows, *limit, *allowlist, *dbPath, *concurrency, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "strategic collector failed:", err)
		os.Exit(1)
	}
	fmt.Printf("strategic product selection complete (sectors=%s codes=%d)\n", strings.Join(strategic.Sectors(selected), ","), len(selected))
}
