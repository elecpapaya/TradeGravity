package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"tradegravity/internal/deliverypreflight"
)

func main() {
	kitDir := flag.String("kit", "distribution-kit", "approved distribution-kit directory")
	subscribers := flag.String("subscribers", "", "local double-opt-in subscriber CSV outside the kit")
	suppressions := flag.String("suppressions", "", "local unsubscribe/bounce/complaint suppression CSV outside the kit")
	output := flag.String("out", "delivery-preflight.json", "new aggregate-only preflight JSON outside the kit")
	generatedAtValue := flag.String("generated-at", "", "explicit RFC3339 preflight time")
	maxRecipients := flag.Int("max-recipients", 25, "fail when the consented unsuppressed pilot audience exceeds this limit (1-1000)")
	flag.Parse()

	generatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*generatedAtValue))
	if err != nil {
		fatal(fmt.Errorf("generated-at must be RFC3339: %w", err))
	}
	result, err := deliverypreflight.Build(deliverypreflight.Request{
		KitDir:         *kitDir,
		SubscriberCSV:  *subscribers,
		SuppressionCSV: *suppressions,
		GeneratedAt:    generatedAt,
		MaxRecipients:  *maxRecipients,
	})
	if err != nil {
		fatal(err)
	}
	if err := deliverypreflight.Write(*output, *kitDir, result.JSON); err != nil {
		fatal(err)
	}
	fmt.Printf("email preflight passed (edition=%s audience=%s consented=%d suppressed=%d eligible=%d delivery_authorized=%t)\n", result.Plan.EditionID, result.Plan.Audience, result.Plan.Counts.Consented, result.Plan.Counts.Suppressed, result.Plan.Counts.Eligible, result.Plan.DeliveryAuthorized)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "distribution preflight failed:", err)
	os.Exit(1)
}
