package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"tradegravity/internal/distributionkit"
)

func main() {
	kitDir := flag.String("kit", "distribution-kit", "reviewed distribution-kit directory")
	reviewer := flag.String("reviewer", "", "human reviewer name or stable handle; do not include subscriber data")
	audience := flag.String("audience", "", "non-sensitive audience label, for example internal-pilot")
	channelsValue := flag.String("channels", "", "comma-separated content channels: email,instagram")
	approvedAtValue := flag.String("approved-at", "", "explicit RFC3339 approval time")
	attested := flag.Bool("attest-reviewed", false, "confirm evidence, copy, rights, assets, and alt text were reviewed")
	flag.Parse()

	approvedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*approvedAtValue))
	if err != nil {
		fatal(fmt.Errorf("approved-at must be RFC3339: %w", err))
	}
	channels := splitChannels(*channelsValue)
	approval, content, err := distributionkit.Approve(*kitDir, distributionkit.ApprovalRequest{
		Reviewer:   *reviewer,
		Audience:   *audience,
		Channels:   channels,
		ApprovedAt: approvedAt,
		Attested:   *attested,
	})
	if err != nil {
		fatal(err)
	}
	if err := distributionkit.WriteApproval(*kitDir, content); err != nil {
		fatal(err)
	}
	fmt.Printf("content approval recorded (edition=%s channels=%s scope=%s delivery_ready=%t)\n", approval.EditionID, strings.Join(approval.Channels, ","), approval.Scope, approval.ProviderDeliveryReady)
}

func splitChannels(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "distribution approval failed:", err)
	os.Exit(1)
}
