package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"tradegravity/internal/emaildelivery"
)

func main() {
	preflight := flag.String("preflight", "delivery-preflight.json", "aggregate delivery preflight JSON")
	output := flag.String("out", "email-launch-authorization.json", "new private launch authorization JSON")
	provider := flag.String("provider", "resend", "email provider for this pilot authorization")
	sender := flag.String("from", "", "authenticated sender mailbox")
	replyTo := flag.String("reply-to", "", "optional reply-to mailbox")
	authorizedBy := flag.String("authorized-by", "", "operator identity recorded in the launch authorization")
	authorizedAtValue := flag.String("authorized-at", "", "explicit RFC3339 authorization time")
	expiresAtValue := flag.String("expires-at", "", "explicit RFC3339 expiry no more than one hour later")
	domainReady := flag.Bool("attest-domain-authenticated", false, "attest that SPF, DKIM, and DMARC were verified for the sender")
	feedbackReady := flag.Bool("attest-feedback-ready", false, "attest that bounce and complaint handling is operational")
	privacyReady := flag.Bool("attest-privacy-reviewed", false, "attest that retention, deletion, and operator access were reviewed")
	pilotReady := flag.Bool("attest-pilot-recipients", false, "attest that every eligible pilot recipient was intentionally confirmed")
	flag.Parse()

	authorizedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*authorizedAtValue))
	if err != nil {
		fatal(fmt.Errorf("authorized-at must be RFC3339: %w", err))
	}
	expiresAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*expiresAtValue))
	if err != nil {
		fatal(fmt.Errorf("expires-at must be RFC3339: %w", err))
	}
	authorization, raw, err := emaildelivery.Authorize(emaildelivery.AuthorizationRequest{
		PreflightPath: *preflight,
		Provider:      *provider,
		Sender:        *sender,
		ReplyTo:       *replyTo,
		AuthorizedBy:  *authorizedBy,
		AuthorizedAt:  authorizedAt,
		ExpiresAt:     expiresAt,
		Attestations: emaildelivery.Attestations{
			SenderDomainAuthenticated: *domainReady,
			BounceComplaintReady:      *feedbackReady,
			PrivacyControlsReviewed:   *privacyReady,
			PilotRecipientsConfirmed:  *pilotReady,
		},
	})
	if err != nil {
		fatal(err)
	}
	if err := emaildelivery.WriteAuthorization(*output, raw); err != nil {
		fatal(err)
	}
	fmt.Printf("email launch authorized (edition=%s audience=%s provider=%s eligible=%d expires_at=%s)\n", authorization.EditionID, authorization.Audience, authorization.Provider, authorization.EligibleRecipients, authorization.ExpiresAt)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "email launch approval failed:", err)
	os.Exit(1)
}
