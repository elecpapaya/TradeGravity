package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"tradegravity/internal/emaildelivery"
)

const (
	apiKeyEnvironment       = "RESEND_API_KEY"
	ledgerSecretEnvironment = "TRADEGRAVITY_DELIVERY_SECRET"
)

func main() {
	kitDir := flag.String("kit", "distribution-kit", "approved distribution-kit directory")
	subscribers := flag.String("subscribers", "", "private double-opt-in subscriber CSV")
	suppressions := flag.String("suppressions", "", "private suppression CSV")
	preflight := flag.String("preflight", "delivery-preflight.json", "aggregate preflight bound to the launch authorization")
	authorization := flag.String("authorization", "email-launch-authorization.json", "short-lived private launch authorization")
	ledger := flag.String("ledger", "", "private SQLite delivery ledger")
	sendAtValue := flag.String("send-at", "", "explicit RFC3339 send time inside the authorization window")
	sendLive := flag.Bool("send-live", false, "make live provider requests; required in addition to all other gates")
	flag.Parse()

	sendAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*sendAtValue))
	if err != nil {
		fatal(fmt.Errorf("send-at must be RFC3339: %w", err))
	}
	provider, err := emaildelivery.NewResendProvider(os.Getenv(apiKeyEnvironment), nil)
	if err != nil {
		fatal(err)
	}
	result, err := emaildelivery.Deliver(context.Background(), emaildelivery.DeliveryRequest{
		KitDir:            *kitDir,
		SubscriberCSV:     *subscribers,
		SuppressionCSV:    *suppressions,
		PreflightPath:     *preflight,
		AuthorizationPath: *authorization,
		LedgerPath:        *ledger,
		LedgerSecret:      []byte(os.Getenv(ledgerSecretEnvironment)),
		SendAt:            sendAt,
		Provider:          provider,
		SendLive:          *sendLive,
	})
	if err != nil {
		fatal(err)
	}
	fmt.Printf("email delivery complete (edition=%s audience=%s eligible=%d accepted=%d skipped=%d pending=%d)\n", result.EditionID, result.Audience, result.Eligible, result.Accepted, result.Skipped, result.Pending)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "email delivery failed:", err)
	os.Exit(1)
}
