package main

import (
	"flag"
	"fmt"
	"os"

	"tradegravity/internal/distributionkit"
)

func main() {
	briefingPath := flag.String("briefing", "site/data/briefing.json", "review-gated briefing.json input")
	outputDir := flag.String("out", "distribution-kit", "new output directory; existing paths are never overwritten")
	baseURL := flag.String("base-url", "https://elecpapaya.github.io/TradeGravity/", "public evidence base URL")
	theme := flag.String("theme", distributionkit.ThemeIntelligenceDark, "carousel theme: intelligence-dark or editorial-light")
	flag.Parse()

	raw, err := os.ReadFile(*briefingPath)
	if err != nil {
		fatal(fmt.Errorf("read briefing: %w", err))
	}
	bundle, err := distributionkit.BuildWithOptions(raw, *baseURL, distributionkit.BuildOptions{Theme: *theme})
	if err != nil {
		fatal(err)
	}
	if err := distributionkit.Write(*outputDir, bundle); err != nil {
		fatal(err)
	}
	fmt.Printf("distribution kit built (edition=%s theme=%s files=%d status=%s out=%s)\n", bundle.Manifest.EditionID, bundle.Manifest.Carousel.Theme, len(bundle.Files), bundle.Manifest.DistributionStatus, *outputDir)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "distribution kit failed:", err)
	os.Exit(1)
}
