package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"tradegravity/internal/socialpreflight"
)

func main() {
	kit := flag.String("kit", "distribution-kit", "Instagram-approved distribution-kit directory")
	out := flag.String("out", "instagram-preflight.json", "new aggregate-only JSON outside the kit")
	generated := flag.String("generated-at", "", "explicit RFC3339 preflight time")
	flag.Parse()
	generatedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(*generated))
	if err != nil {
		fatal(fmt.Errorf("generated-at must be RFC3339: %w", err))
	}
	result, err := socialpreflight.Build(*kit, generatedAt)
	if err != nil {
		fatal(err)
	}
	if err := socialpreflight.Write(*out, *kit, result.JSON); err != nil {
		fatal(err)
	}
	fmt.Printf("Instagram preflight passed (edition=%s theme=%s slides=%d caption_runes=%d hashtags=%d manual_upload=%t automatic_publish=%t)\n", result.Plan.EditionID, result.Plan.Theme, result.Plan.SlideCount, result.Plan.CaptionRunes, result.Plan.HashtagCount, result.Plan.ManualUploadRequired, result.Plan.AutomaticPublishAuthorized)
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "Instagram preflight failed:", err); os.Exit(1) }
