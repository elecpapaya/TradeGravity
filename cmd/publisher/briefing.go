package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const briefingSchemaVersion = "1.0"

type briefingFile struct {
	SchemaVersion      string                 `json:"schema_version"`
	GeneratedAt        string                 `json:"generated_at"`
	EditionID          string                 `json:"edition_id"`
	Status             string                 `json:"status"`
	Title              string                 `json:"title"`
	Scope              string                 `json:"scope"`
	LatestPeriod       string                 `json:"latest_period,omitempty"`
	PreviousPeriod     string                 `json:"previous_period,omitempty"`
	PublicationStatus  string                 `json:"publication_status"`
	ReviewRequired     bool                   `json:"review_required"`
	Signals            []briefingSignal       `json:"signals"`
	Email              briefingEmail          `json:"email"`
	SocialCarousel     briefingSocialCarousel `json:"social_carousel"`
	Caveats            []string               `json:"caveats"`
	EvidenceEntryPoint string                 `json:"evidence_entry_point"`
}

type briefingSignal struct {
	ID               string                `json:"id"`
	Kind             string                `json:"kind"`
	Title            string                `json:"title"`
	Summary          string                `json:"summary"`
	ReporterISO3     string                `json:"reporter_iso3"`
	ReporterName     string                `json:"reporter_name"`
	Classification   string                `json:"classification,omitempty"`
	Code             string                `json:"code,omitempty"`
	Label            string                `json:"label,omitempty"`
	Period           string                `json:"period"`
	PreviousPeriod   string                `json:"previous_period"`
	Current          briefingObservedValue `json:"current"`
	Previous         briefingObservedValue `json:"previous"`
	DeltaTradeUSD    float64               `json:"delta_trade_usd"`
	ChangeRatio      *float64              `json:"change_ratio,omitempty"`
	ChinaShareDelta  float64               `json:"china_share_delta"`
	Evidence         []string              `json:"evidence"`
	Interpretation   string                `json:"interpretation"`
	MeasurementLimit string                `json:"measurement_limit"`
}

type briefingObservedValue struct {
	USATradeUSD   float64 `json:"usa_trade_usd"`
	ChinaTradeUSD float64 `json:"china_trade_usd"`
	TotalTradeUSD float64 `json:"total_trade_usd"`
	ChinaShare    float64 `json:"china_share"`
}

type briefingEmail struct {
	Subject     string `json:"subject"`
	Preview     string `json:"preview"`
	Markdown    string `json:"markdown"`
	CTALabel    string `json:"cta_label"`
	CTAPath     string `json:"cta_path"`
	SendPolicy  string `json:"send_policy"`
	PrimaryGoal string `json:"primary_goal"`
}

type briefingSocialCarousel struct {
	Format       string                  `json:"format"`
	AspectRatio  string                  `json:"aspect_ratio"`
	ReviewPolicy string                  `json:"review_policy"`
	Slides       []briefingCarouselSlide `json:"slides"`
}

type briefingCarouselSlide struct {
	Order    int      `json:"order"`
	Role     string   `json:"role"`
	Headline string   `json:"headline"`
	Body     string   `json:"body"`
	Evidence []string `json:"evidence"`
}

type briefingCandidate struct {
	signal    briefingSignal
	magnitude float64
}

func buildBriefing(generatedAt string, latest []latestEntry, monthlyIndex semiconductorMonthlyIndexFile, monthlyFiles map[string]semiconductorMonthlyFile, publicationChanges publicationChangesFile) briefingFile {
	briefing := briefingFile{
		SchemaVersion:      briefingSchemaVersion,
		GeneratedAt:        generatedAt,
		EditionID:          briefingEditionID(generatedAt, monthlyIndex.Periods),
		Status:             "unavailable",
		Title:              "TradeGravity Semiconductor Pulse",
		Scope:              "Deterministic distribution brief from selected monthly UN Comtrade HS6 observations against USA and China; not a complete semiconductor market, causal claim, or investment recommendation",
		PublicationStatus:  publicationChanges.Status,
		ReviewRequired:     true,
		Signals:            []briefingSignal{},
		EvidenceEntryPoint: "./?tab=semiconductors",
		Caveats: []string{
			"Monthly customs observations can be volatile, incomplete, and revised.",
			"USA and China values are the two published anchor relationships, not world totals or physical shipment routes.",
			"Publication-to-publication revisions are separate from economic month-to-month movement.",
		},
		Email: briefingEmail{
			CTALabel:    "Inspect the evidence",
			CTAPath:     "./?tab=semiconductors",
			SendPolicy:  "manual_review_required",
			PrimaryGoal: "Return the reader to the cited TradeGravity evidence",
		},
		SocialCarousel: briefingSocialCarousel{
			Format:       "carousel_copy",
			AspectRatio:  "4:5",
			ReviewPolicy: "manual_review_required",
			Slides:       []briefingCarouselSlide{},
		},
	}
	if len(monthlyIndex.Periods) > 0 {
		briefing.LatestPeriod = monthlyIndex.Periods[len(monthlyIndex.Periods)-1]
	}
	if len(monthlyIndex.Periods) > 1 {
		briefing.PreviousPeriod = monthlyIndex.Periods[len(monthlyIndex.Periods)-2]
	}

	names := make(map[string]string, len(latest))
	for _, row := range latest {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			name = row.ISO3
		}
		names[strings.ToUpper(row.ISO3)] = name
	}
	reporterCandidates := make([]briefingCandidate, 0, len(monthlyFiles))
	shareCandidates := make([]briefingCandidate, 0, len(monthlyFiles))
	productCandidates := make([]briefingCandidate, 0)
	currentPeriod, previousPeriod := briefing.LatestPeriod, briefing.PreviousPeriod
	if currentPeriod == "" || previousPeriod == "" {
		briefing.Email.Subject = "TradeGravity Semiconductor Pulse · data unavailable"
		briefing.Email.Preview = "The current publication does not contain enough comparable monthly observations to produce a distribution brief."
		briefing.Email.Markdown = "# TradeGravity Semiconductor Pulse\n\nNo distribution brief was generated because two comparable monthly observations were not available. This is not interpreted as no change.\n"
		return briefing
	}
	for _, file := range monthlyFiles {
		currentAggregate, currentOK := aggregateBriefingRows(file.Rows, currentPeriod)
		previousAggregate, previousOK := aggregateBriefingRows(file.Rows, previousPeriod)
		if !currentOK || !previousOK {
			continue
		}
		reporter := strings.ToUpper(strings.TrimSpace(file.ReporterISO3))
		name := names[reporter]
		if name == "" {
			name = reporter
		}
		reporterSignal := makeBriefingSignal("reporter_total_change", reporter, name, "", "", "", currentPeriod, previousPeriod, currentAggregate, previousAggregate)
		reporterSignal.ID = "monthly-total-" + strings.ToLower(reporter)
		reporterSignal.Title = fmt.Sprintf("%s selected chip trade %s", name, movementWord(reporterSignal.DeltaTradeUSD))
		reporterSignal.Summary = fmt.Sprintf("Selected monthly HS6 trade with USA and China moved from %s to %s (%s).", formatBriefingUSD(previousAggregate.TotalTradeUSD), formatBriefingUSD(currentAggregate.TotalTradeUSD), formatBriefingPercent(reporterSignal.ChangeRatio))
		reporterSignal.Interpretation = "A change in the selected two-anchor customs observations worth investigating; it does not establish production, demand, or causality."
		reporterSignal.Evidence = []string{"./semiconductors/monthly/" + reporter + ".json", "./semiconductors/monthly/index.json"}
		reporterCandidates = append(reporterCandidates, briefingCandidate{signal: reporterSignal, magnitude: math.Abs(reporterSignal.DeltaTradeUSD)})

		shareSignal := reporterSignal
		shareSignal.ID = "anchor-share-" + strings.ToLower(reporter)
		shareSignal.Kind = "anchor_share_shift"
		shareSignal.Title = fmt.Sprintf("%s two-anchor balance shifted %s", name, anchorDirection(shareSignal.ChinaShareDelta))
		shareSignal.Summary = fmt.Sprintf("China's share of the selected USA-plus-China total moved from %.1f%% to %.1f%% (%+.1f percentage points).", previousAggregate.ChinaShare*100, currentAggregate.ChinaShare*100, shareSignal.ChinaShareDelta*100)
		shareSignal.Interpretation = "The sign describes movement within the published USA-China anchor pair, not political alignment or global market share."
		shareCandidates = append(shareCandidates, briefingCandidate{signal: shareSignal, magnitude: math.Abs(shareSignal.ChinaShareDelta)})

		currentProducts := briefingProductsByKey(file.Rows, currentPeriod)
		previousProducts := briefingProductsByKey(file.Rows, previousPeriod)
		for key, current := range currentProducts {
			previous, ok := previousProducts[key]
			if !ok {
				continue
			}
			productSignal := makeBriefingSignal("product_total_change", reporter, name, current.Classification, current.Code, current.Label, currentPeriod, previousPeriod, observedValueFromMonthly(current), observedValueFromMonthly(previous))
			productSignal.ID = "product-" + strings.ToLower(reporter) + "-" + current.Code
			productSignal.Title = fmt.Sprintf("%s · %s %s", name, current.Label, movementWord(productSignal.DeltaTradeUSD))
			productSignal.Summary = fmt.Sprintf("HS6 %s selected trade moved from %s to %s (%s).", current.Code, formatBriefingUSD(productSignal.Previous.TotalTradeUSD), formatBriefingUSD(productSignal.Current.TotalTradeUSD), formatBriefingPercent(productSignal.ChangeRatio))
			productSignal.Interpretation = "This is a product-level customs observation against USA and China, not company revenue, capacity, or a shipment route."
			productSignal.Evidence = []string{"./semiconductors/monthly/" + reporter + ".json", "./semiconductors/reference.json"}
			productCandidates = append(productCandidates, briefingCandidate{signal: productSignal, magnitude: math.Abs(productSignal.DeltaTradeUSD)})
		}
	}

	sortBriefingCandidates(reporterCandidates)
	sortBriefingCandidates(shareCandidates)
	sortBriefingCandidates(productCandidates)
	for _, candidates := range [][]briefingCandidate{reporterCandidates, shareCandidates, productCandidates} {
		if len(candidates) > 0 {
			briefing.Signals = append(briefing.Signals, candidates[0].signal)
		}
	}
	if len(briefing.Signals) != 3 {
		briefing.Email.Subject = "TradeGravity Semiconductor Pulse · data unavailable"
		briefing.Email.Preview = "The current publication does not contain enough comparable monthly observations to produce a distribution brief."
		briefing.Email.Markdown = "# TradeGravity Semiconductor Pulse\n\nNo distribution brief was generated because two comparable monthly observations were not available. This is not interpreted as no change.\n"
		return briefing
	}

	briefing.Status = "ready"
	briefing.Email = buildBriefingEmail(briefing)
	briefing.SocialCarousel = buildBriefingCarousel(briefing)
	return briefing
}

func aggregateBriefingRows(rows []semiconductorMonthlyProductEntry, period string) (briefingObservedValue, bool) {
	value := briefingObservedValue{}
	found := false
	for _, row := range rows {
		if row.Period != period {
			continue
		}
		value.USATradeUSD += row.USA.Trade
		value.ChinaTradeUSD += row.CHN.Trade
		found = true
	}
	value.TotalTradeUSD = value.USATradeUSD + value.ChinaTradeUSD
	if value.TotalTradeUSD > 0 {
		value.ChinaShare = value.ChinaTradeUSD / value.TotalTradeUSD
	}
	return value, found
}

func briefingProductsByKey(rows []semiconductorMonthlyProductEntry, period string) map[string]semiconductorMonthlyProductEntry {
	result := make(map[string]semiconductorMonthlyProductEntry)
	for _, row := range rows {
		if row.Period != period {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(row.Classification)) + "|" + row.Code
		result[key] = row
	}
	return result
}

func observedValueFromMonthly(row semiconductorMonthlyProductEntry) briefingObservedValue {
	return briefingObservedValue{USATradeUSD: row.USA.Trade, ChinaTradeUSD: row.CHN.Trade, TotalTradeUSD: row.Total, ChinaShare: row.ShareCN}
}

func makeBriefingSignal(kind, reporter, name, classification, code, label, period, previousPeriod string, current, previous briefingObservedValue) briefingSignal {
	delta := current.TotalTradeUSD - previous.TotalTradeUSD
	var ratio *float64
	if previous.TotalTradeUSD > 0 {
		value := delta / previous.TotalTradeUSD
		ratio = &value
	}
	return briefingSignal{
		Kind:             kind,
		ReporterISO3:     reporter,
		ReporterName:     name,
		Classification:   classification,
		Code:             code,
		Label:            label,
		Period:           period,
		PreviousPeriod:   previousPeriod,
		Current:          current,
		Previous:         previous,
		DeltaTradeUSD:    delta,
		ChangeRatio:      ratio,
		ChinaShareDelta:  current.ChinaShare - previous.ChinaShare,
		Evidence:         []string{},
		MeasurementLimit: "Selected monthly HS6 observations against USA and China only; subject to source revisions and coverage limits.",
	}
}

func sortBriefingCandidates(candidates []briefingCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].magnitude != candidates[j].magnitude {
			return candidates[i].magnitude > candidates[j].magnitude
		}
		left, right := candidates[i].signal, candidates[j].signal
		if left.ReporterISO3 != right.ReporterISO3 {
			return left.ReporterISO3 < right.ReporterISO3
		}
		return left.Code < right.Code
	})
}

func buildBriefingEmail(briefing briefingFile) briefingEmail {
	lines := []string{
		"# " + briefing.Title,
		"",
		fmt.Sprintf("Observation window: %s vs %s · publication status: %s", briefing.LatestPeriod, briefing.PreviousPeriod, briefing.PublicationStatus),
		"",
	}
	for _, signal := range briefing.Signals {
		lines = append(lines, "## "+signal.Title, "", signal.Summary, "", "Interpretation boundary: "+signal.Interpretation, "")
	}
	lines = append(lines,
		"Review the cited evidence before forwarding or publishing this draft.",
		"",
		"[Inspect the evidence]({{BASE_URL}}/?tab=semiconductors)",
		"",
		"Data scope: selected monthly UN Comtrade HS6 observations against USA and China. Not investment, legal, or policy advice.",
	)
	return briefingEmail{
		Subject:     fmt.Sprintf("TradeGravity Semiconductor Pulse · %s", briefing.LatestPeriod),
		Preview:     fmt.Sprintf("Three cited USA-China semiconductor observations for %s; monthly movement is kept separate from publication revisions.", briefing.LatestPeriod),
		Markdown:    strings.Join(lines, "\n"),
		CTALabel:    "Inspect the evidence",
		CTAPath:     "./?tab=semiconductors",
		SendPolicy:  "manual_review_required",
		PrimaryGoal: "Return the reader to the cited TradeGravity evidence",
	}
}

func buildBriefingCarousel(briefing briefingFile) briefingSocialCarousel {
	slides := []briefingCarouselSlide{{
		Order: 1, Role: "cover", Headline: briefing.Title,
		Body:     fmt.Sprintf("Three USA-China semiconductor observations · %s vs %s", briefing.LatestPeriod, briefing.PreviousPeriod),
		Evidence: []string{"./semiconductors/monthly/index.json"},
	}}
	roles := []string{"scale", "anchor_balance", "product"}
	for index, signal := range briefing.Signals {
		slides = append(slides, briefingCarouselSlide{Order: index + 2, Role: roles[index], Headline: signal.Title, Body: signal.Summary, Evidence: append([]string(nil), signal.Evidence...)})
	}
	slides = append(slides,
		briefingCarouselSlide{Order: 5, Role: "method", Headline: "Read the clocks separately", Body: "Month-to-month customs movement and publish-to-publish revisions answer different questions. Neither proves causality or a physical route.", Evidence: []string{"./changes.json", "./semiconductors/monthly/index.json"}},
		briefingCarouselSlide{Order: 6, Role: "cta", Headline: "Inspect the evidence", Body: "Open TradeGravity's Chip Lens for periods, values, sources, coverage, and limitations.", Evidence: []string{"./?tab=semiconductors"}},
	)
	return briefingSocialCarousel{Format: "carousel_copy", AspectRatio: "4:5", ReviewPolicy: "manual_review_required", Slides: slides}
}

func briefingEditionID(generatedAt string, periods []string) string {
	period := "no-period"
	if len(periods) > 0 {
		period = periods[len(periods)-1]
	}
	timestamp := "unknown"
	if parsed, err := time.Parse(time.RFC3339, generatedAt); err == nil {
		timestamp = parsed.UTC().Format("20060102T150405Z")
	}
	return "semiconductor-pulse-" + period + "-" + timestamp
}

func movementWord(delta float64) string {
	if delta > 0 {
		return "increased"
	}
	if delta < 0 {
		return "decreased"
	}
	return "was unchanged"
}

func anchorDirection(chinaShareDelta float64) string {
	if chinaShareDelta > 0 {
		return "toward China"
	}
	if chinaShareDelta < 0 {
		return "toward USA"
	}
	return "without a change"
}

func formatBriefingUSD(value float64) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1e12:
		return fmt.Sprintf("US$%.2fT", value/1e12)
	case abs >= 1e9:
		return fmt.Sprintf("US$%.2fB", value/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("US$%.2fM", value/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("US$%.2fK", value/1e3)
	default:
		return fmt.Sprintf("US$%.0f", value)
	}
}

func formatBriefingPercent(value *float64) string {
	if value == nil {
		return "no comparable percentage"
	}
	return fmt.Sprintf("%+.1f%%", *value*100)
}
