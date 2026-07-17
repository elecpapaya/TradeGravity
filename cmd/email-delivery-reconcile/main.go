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

const ledgerSecretEnvironment = "TRADEGRAVITY_DELIVERY_SECRET"

func main() {
	ledgerPath := flag.String("ledger", "", "private SQLite delivery ledger")
	edition := flag.String("edition", "", "edition ID from the stopped delivery")
	audience := flag.String("audience", "", "non-sensitive audience label")
	recipient := flag.String("recipient", "", "private recipient address used only to derive the ledger key")
	outcome := flag.String("outcome", "", "provider-confirmed outcome: accepted or not_accepted")
	providerMessageID := flag.String("provider-message-id", "", "required only for an accepted outcome")
	resolvedBy := flag.String("resolved-by", "", "operator identity recorded in the private ledger")
	evidence := flag.String("evidence", "", "non-sensitive provider-dashboard or support evidence label")
	resolvedAtValue := flag.String("resolved-at", "", "explicit RFC3339 reconciliation time")
	attest := flag.Bool("attest-provider-checked", false, "attest that the provider dashboard or support record was checked")
	flag.Parse()

	if !*attest {
		fatal(fmt.Errorf("attest-provider-checked is required"))
	}
	resolvedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*resolvedAtValue))
	if err != nil {
		fatal(fmt.Errorf("resolved-at must be RFC3339: %w", err))
	}
	ledger, err := emaildelivery.OpenLedger(*ledgerPath, []byte(os.Getenv(ledgerSecretEnvironment)))
	if err != nil {
		fatal(err)
	}
	defer ledger.Close()
	result, err := ledger.Reconcile(context.Background(), emaildelivery.ReconciliationRequest{
		EditionID:         *edition,
		Audience:          *audience,
		Email:             *recipient,
		Outcome:           strings.TrimSpace(*outcome),
		ProviderMessageID: *providerMessageID,
		ResolvedBy:        *resolvedBy,
		Evidence:          *evidence,
		ResolvedAt:        resolvedAt,
	})
	if err != nil {
		fatal(err)
	}
	fmt.Printf("delivery reconciliation recorded (edition=%s audience=%s outcome=%s changed=%t already_resolved=%t)\n", *edition, *audience, *outcome, result.Changed, result.AlreadyResolved)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "email delivery reconciliation failed:", err)
	os.Exit(1)
}
