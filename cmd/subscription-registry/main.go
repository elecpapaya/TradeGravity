package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"tradegravity/internal/subscriptions"
)

const secretEnvironment = "TRADEGRAVITY_UNSUBSCRIBE_SECRET"

func main() {
	databasePath := flag.String("db", "", "private SQLite subscription database path")
	publicBaseURL := flag.String("base-url", "", "public HTTPS base URL for the unsubscribe service")
	consentPath := flag.String("consents", "", "private active double-opt-in consent CSV")
	audience := flag.String("audience", "", "non-sensitive audience label to export")
	subscriberOutput := flag.String("out-subscribers", "", "new private delivery subscriber CSV")
	suppressionOutput := flag.String("out-suppressions", "", "new private suppression CSV")
	importedAtValue := flag.String("imported-at", "", "explicit RFC3339 import time")
	flag.Parse()

	importedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*importedAtValue))
	if err != nil {
		fatal(fmt.Errorf("imported-at must be RFC3339: %w", err))
	}
	secret := []byte(os.Getenv(secretEnvironment))
	if len(secret) == 0 {
		fatal(fmt.Errorf("%s is required", secretEnvironment))
	}
	consents, err := readPrivateInput(*consentPath)
	if err != nil {
		fatal(err)
	}
	registry, err := subscriptions.Open(*databasePath, secret, *publicBaseURL)
	if err != nil {
		fatal(err)
	}
	defer registry.Close()

	result, err := registry.ImportConsents(context.Background(), consents, importedAt)
	if err != nil {
		fatal(err)
	}
	subscribersCSV, suppressionsCSV, err := registry.ExportAudience(context.Background(), *audience)
	if err != nil {
		fatal(err)
	}
	if err := subscriptions.WritePrivateExports(*subscriberOutput, subscribersCSV, *suppressionOutput, suppressionsCSV); err != nil {
		fatal(err)
	}
	fmt.Printf("subscription registry updated (inserted=%d updated=%d suppressed_skipped=%d exports=2)\n", result.Inserted, result.Updated, result.SuppressedSkipped)
}

func readPrivateInput(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("consent CSV path is required")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect consent CSV: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("consent CSV must be a regular non-symlink file")
	}
	if info.Size() > 5<<20 {
		return nil, errors.New("consent CSV exceeds the 5 MiB limit")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read consent CSV: %w", err)
	}
	return raw, nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "subscription registry failed:", err)
	os.Exit(1)
}
