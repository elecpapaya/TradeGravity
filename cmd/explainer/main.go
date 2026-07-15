package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const schemaVersion = "2.0"

var numberPattern = regexp.MustCompile(`\b\d[\d,.]*%?\b`)
var unsupportedCausalPattern = regexp.MustCompile(`(?i)\b(because|caused?|due to|driven by|as a result)\b`)

type metric struct {
	Value *float64 `json:"value"`
	Year  string   `json:"year"`
}

type partnerBlock struct {
	Period     string  `json:"period"`
	PeriodType string  `json:"period_type"`
	Export     float64 `json:"export"`
	Import     float64 `json:"import"`
	Trade      float64 `json:"trade"`
}

type latestEntry struct {
	ISO3             string       `json:"iso3"`
	Name             string       `json:"name"`
	Population       metric       `json:"population"`
	GDP              metric       `json:"gdp"`
	USA              partnerBlock `json:"usa"`
	CHN              partnerBlock `json:"chn"`
	Total            float64      `json:"total"`
	ShareCN          float64      `json:"share_cn"`
	SamePeriod       bool         `json:"same_period"`
	ComparisonPeriod string       `json:"comparison_period"`
}

type latestFile struct {
	SchemaVersion string        `json:"schema_version"`
	GeneratedAt   string        `json:"generated_at"`
	Provider      string        `json:"provider"`
	Rows          []latestEntry `json:"rows"`
}

type seriesBlock struct {
	Available bool    `json:"available"`
	Trade     float64 `json:"trade"`
}

type seriesPoint struct {
	PeriodType string      `json:"period_type"`
	Period     string      `json:"period"`
	USA        seriesBlock `json:"usa"`
	CHN        seriesBlock `json:"chn"`
	Comparable bool        `json:"comparable"`
}

type reporterSeries struct {
	ISO3   string        `json:"iso3"`
	Points []seriesPoint `json:"points"`
}

type seriesFile struct {
	Rows []reporterSeries `json:"rows"`
}

type productBlock struct {
	Trade float64 `json:"trade"`
}

type productEntry struct {
	Period string       `json:"period"`
	Code   string       `json:"code"`
	Name   string       `json:"name"`
	USA    productBlock `json:"usa"`
	CHN    productBlock `json:"chn"`
	Total  float64      `json:"total"`
}

type productFile struct {
	Provider       string         `json:"provider"`
	Classification string         `json:"classification"`
	Level          int            `json:"level"`
	Periods        []string       `json:"periods"`
	Rows           []productEntry `json:"rows"`
}

type evidence struct {
	ID           string  `json:"id"`
	Label        string  `json:"label"`
	Value        float64 `json:"value,omitempty"`
	DisplayValue string  `json:"display_value,omitempty"`
	Unit         string  `json:"unit,omitempty"`
	Period       string  `json:"period,omitempty"`
	Source       string  `json:"source"`
	SourceJSON   string  `json:"source_json"`
}

type statement struct {
	Text        string   `json:"text"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type generator struct {
	Type   string `json:"type"`
	Status string `json:"status"`
	Model  string `json:"model"`
}

type explanation struct {
	SchemaVersion string      `json:"schema_version"`
	GeneratedAt   string      `json:"generated_at"`
	ReporterISO3  string      `json:"reporter_iso3"`
	Name          string      `json:"name"`
	Generator     generator   `json:"generator"`
	Summary       string      `json:"summary"`
	Statements    []statement `json:"statements"`
	Evidence      []evidence  `json:"evidence"`
}

type explanationIndex struct {
	SchemaVersion string   `json:"schema_version"`
	GeneratedAt   string   `json:"generated_at"`
	Reporters     []string `json:"reporters"`
	AICount       int      `json:"ai_count"`
	FallbackCount int      `json:"fallback_count"`
	Model         string   `json:"model"`
}

type aiContent struct {
	Summary    string      `json:"summary"`
	Statements []statement `json:"statements"`
}

type responseEnvelope struct {
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func main() {
	dataDir := flag.String("dir", "site/data", "published data directory")
	outDir := flag.String("out", "", "explanation output directory (default: <dir>/explanations)")
	useAI := flag.Bool("ai", false, "use the OpenAI Responses API when OPENAI_API_KEY is set")
	model := flag.String("model", envOr("OPENAI_MODEL", "gpt-5.6-luna"), "OpenAI model")
	maxAI := flag.Int("max-ai-reporters", 10, "maximum reporters sent to the API; remaining reporters use deterministic output")
	timeout := flag.Duration("timeout", 45*time.Second, "timeout per API request")
	flag.Parse()

	if *outDir == "" {
		*outDir = filepath.Join(*dataDir, "explanations")
	}
	var latest latestFile
	if err := readJSON(filepath.Join(*dataDir, "latest.json"), &latest); err != nil {
		fatalf("read latest dataset: %v", err)
	}
	if latest.SchemaVersion != schemaVersion {
		fatalf("unsupported latest schema %q; expected %s", latest.SchemaVersion, schemaVersion)
	}
	var series seriesFile
	_ = readJSON(filepath.Join(*dataDir, "series.json"), &series)
	seriesByISO := make(map[string]reporterSeries, len(series.Rows))
	for _, item := range series.Rows {
		seriesByISO[item.ISO3] = item
	}

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	client := &http.Client{Timeout: *timeout}
	index := explanationIndex{SchemaVersion: schemaVersion, GeneratedAt: latest.GeneratedAt, Model: "none", Reporters: []string{}}
	if *useAI && apiKey != "" {
		index.Model = *model
	}
	sort.Slice(latest.Rows, func(i, j int) bool { return latest.Rows[i].ISO3 < latest.Rows[j].ISO3 })
	for rowIndex, row := range latest.Rows {
		product := loadProduct(*dataDir, row.ISO3)
		evidenceBundle := buildEvidence(row, latest.Provider, seriesByISO[row.ISO3], product)
		item := deterministicExplanation(row, latest.GeneratedAt, evidenceBundle)
		if *useAI && apiKey != "" && rowIndex < *maxAI {
			ctx, cancel := context.WithTimeout(context.Background(), *timeout)
			generated, err := requestAI(ctx, client, apiKey, *model, row, evidenceBundle)
			cancel()
			if err == nil {
				item.Summary = generated.Summary
				item.Statements = generated.Statements
				item.Generator = generator{Type: "openai", Status: "success", Model: *model}
				index.AICount++
			} else {
				fmt.Fprintf(os.Stderr, "AI explanation fallback for %s: %v\n", row.ISO3, err)
				item.Generator.Status = "api_fallback"
				index.FallbackCount++
			}
		} else {
			index.FallbackCount++
		}
		if err := writeJSON(filepath.Join(*outDir, row.ISO3+".json"), item); err != nil {
			fatalf("write %s explanation: %v", row.ISO3, err)
		}
		index.Reporters = append(index.Reporters, row.ISO3)
	}
	if err := writeJSON(filepath.Join(*outDir, "index.json"), index); err != nil {
		fatalf("write explanation index: %v", err)
	}
	fmt.Printf("published %d explanations (%d OpenAI, %d deterministic)\n", len(index.Reporters), index.AICount, index.FallbackCount)
}

func buildEvidence(row latestEntry, provider string, series reporterSeries, product productFile) []evidence {
	items := []evidence{
		{ID: "TOTAL-USA", Label: "Headline trade with USA", Value: row.USA.Trade, DisplayValue: formatUSD(row.USA.Trade), Unit: "current USD", Period: row.USA.Period, Source: strings.ToUpper(provider), SourceJSON: "../latest.json"},
		{ID: "TOTAL-CHN", Label: "Headline trade with China", Value: row.CHN.Trade, DisplayValue: formatUSD(row.CHN.Trade), Unit: "current USD", Period: row.CHN.Period, Source: strings.ToUpper(provider), SourceJSON: "../latest.json"},
		{ID: "SHARE-CHN", Label: "China share of USA-plus-China trade", Value: row.ShareCN, DisplayValue: fmt.Sprintf("%.1f%%", row.ShareCN*100), Unit: "share", Period: row.ComparisonPeriod, Source: "TradeGravity calculation", SourceJSON: "../latest.json"},
	}
	qualityLabel := "Partner periods differ or one is missing"
	qualityValue := "not comparable"
	if row.SamePeriod {
		qualityLabel = "USA and China observations use the same period"
		qualityValue = "comparable"
	}
	items = append(items, evidence{ID: "QUALITY-PERIOD", Label: qualityLabel, DisplayValue: qualityValue, Period: row.ComparisonPeriod, Source: "TradeGravity quality check", SourceJSON: "../quality.json"})

	annual := make([]seriesPoint, 0)
	for _, point := range series.Points {
		if point.PeriodType == "Y" && point.Comparable {
			annual = append(annual, point)
		}
	}
	if len(annual) >= 2 {
		first, last := annual[0], annual[len(annual)-1]
		items = append(items,
			evidence{ID: "TREND-USA-FIRST", Label: "First comparable annual USA trade", Value: first.USA.Trade, DisplayValue: formatUSD(first.USA.Trade), Unit: "current USD", Period: first.Period, Source: strings.ToUpper(provider), SourceJSON: "../series.json"},
			evidence{ID: "TREND-USA-LAST", Label: "Latest comparable annual USA trade", Value: last.USA.Trade, DisplayValue: formatUSD(last.USA.Trade), Unit: "current USD", Period: last.Period, Source: strings.ToUpper(provider), SourceJSON: "../series.json"},
			evidence{ID: "TREND-CHN-FIRST", Label: "First comparable annual China trade", Value: first.CHN.Trade, DisplayValue: formatUSD(first.CHN.Trade), Unit: "current USD", Period: first.Period, Source: strings.ToUpper(provider), SourceJSON: "../series.json"},
			evidence{ID: "TREND-CHN-LAST", Label: "Latest comparable annual China trade", Value: last.CHN.Trade, DisplayValue: formatUSD(last.CHN.Trade), Unit: "current USD", Period: last.Period, Source: strings.ToUpper(provider), SourceJSON: "../series.json"},
		)
	}
	if len(product.Rows) > 0 {
		period := ""
		if len(product.Periods) > 0 {
			period = product.Periods[0]
		}
		products := append([]productEntry(nil), product.Rows...)
		sort.Slice(products, func(i, j int) bool { return products[i].Total > products[j].Total })
		for _, productRow := range products {
			if period != "" && productRow.Period != period {
				continue
			}
			items = append(items, evidence{ID: "PRODUCT-TOP", Label: "Top HS2 chapter: HS " + productRow.Code + " " + productRow.Name, Value: productRow.Total, DisplayValue: formatUSD(productRow.Total), Unit: "current USD", Period: productRow.Period, Source: strings.ToUpper(product.Provider), SourceJSON: "../products/" + row.ISO3 + ".json"})
			break
		}
	}
	return items
}

func deterministicExplanation(row latestEntry, generatedAt string, evidenceBundle []evidence) explanation {
	usa := evidenceByID(evidenceBundle, "TOTAL-USA")
	china := evidenceByID(evidenceBundle, "TOTAL-CHN")
	leader := "USA"
	if row.CHN.Trade > row.USA.Trade {
		leader = "China"
	}
	periodStatement := "The partner periods differ or one is missing, so the two totals are not a same-period comparison."
	if row.SamePeriod {
		periodStatement = "Both partner totals use the same observation period."
	}
	statements := []statement{
		{Text: fmt.Sprintf("Trade with USA is %s for %s.", usa.DisplayValue, displayPeriod(usa.Period)), EvidenceIDs: []string{"TOTAL-USA"}},
		{Text: fmt.Sprintf("Trade with China is %s for %s.", china.DisplayValue, displayPeriod(china.Period)), EvidenceIDs: []string{"TOTAL-CHN"}},
		{Text: periodStatement, EvidenceIDs: []string{"QUALITY-PERIOD"}},
	}
	if top := evidenceByID(evidenceBundle, "PRODUCT-TOP"); top.ID != "" {
		statements = append(statements, statement{Text: fmt.Sprintf("The largest published HS2 chapter is %s at %s for %s.", top.Label, top.DisplayValue, displayPeriod(top.Period)), EvidenceIDs: []string{"PRODUCT-TOP"}})
	}
	return explanation{
		SchemaVersion: schemaVersion, GeneratedAt: generatedAt, ReporterISO3: row.ISO3, Name: row.Name,
		Generator:  generator{Type: "rules", Status: "fallback", Model: "none"},
		Summary:    fmt.Sprintf("%s has a larger published USA-plus-China trade relationship with %s in the current view.", fallbackName(row.Name, row.ISO3), leader),
		Statements: statements,
		Evidence:   evidenceBundle,
	}
}

func requestAI(ctx context.Context, client *http.Client, apiKey, model string, row latestEntry, evidenceBundle []evidence) (aiContent, error) {
	schema := map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []string{"summary", "statements"},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string", "maxLength": 300},
			"statements": map[string]any{
				"type": "array", "minItems": 2, "maxItems": 6,
				"items": map[string]any{
					"type": "object", "additionalProperties": false,
					"required": []string{"text", "evidence_ids"},
					"properties": map[string]any{
						"text":         map[string]any{"type": "string", "maxLength": 300},
						"evidence_ids": map[string]any{"type": "array", "minItems": 1, "items": map[string]any{"type": "string"}},
					},
				},
			},
		},
	}
	input, _ := json.Marshal(map[string]any{"reporter_iso3": row.ISO3, "name": row.Name, "evidence": evidenceBundle})
	body, _ := json.Marshal(map[string]any{
		"model":        model,
		"instructions": "Explain the trade evidence for a general research audience. Use only the supplied evidence. Write a qualitative summary without numbers. Every statement must cite one or more exact evidence IDs. Reproduce numeric display_value strings exactly; do not calculate, round, infer causation, or use outside facts. Explicitly warn when QUALITY-PERIOD is not comparable.",
		"input":        string(input), "store": false,
		"text": map[string]any{"format": map[string]any{"type": "json_schema", "name": "trade_explanation", "strict": true, "schema": schema}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(body))
	if err != nil {
		return aiContent{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	response, err := client.Do(req)
	if err != nil {
		return aiContent{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return aiContent{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return aiContent{}, fmt.Errorf("Responses API returned %s", response.Status)
	}
	generated, err := parseResponse(responseBody)
	if err != nil {
		return aiContent{}, err
	}
	if err := validateAIContent(generated, evidenceBundle); err != nil {
		return aiContent{}, err
	}
	return generated, nil
}

func parseResponse(data []byte) (aiContent, error) {
	var envelope responseEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return aiContent{}, fmt.Errorf("decode Responses API envelope: %w", err)
	}
	for _, output := range envelope.Output {
		for _, content := range output.Content {
			if content.Type != "output_text" || strings.TrimSpace(content.Text) == "" {
				continue
			}
			var generated aiContent
			if err := json.Unmarshal([]byte(content.Text), &generated); err != nil {
				return aiContent{}, fmt.Errorf("decode structured output: %w", err)
			}
			return generated, nil
		}
	}
	return aiContent{}, errors.New("Responses API contained no output_text")
}

func validateAIContent(content aiContent, evidenceBundle []evidence) error {
	if strings.TrimSpace(content.Summary) == "" || len(content.Statements) < 2 || len(content.Statements) > 6 {
		return errors.New("structured explanation is empty or has an invalid statement count")
	}
	if numberPattern.MatchString(content.Summary) || unsupportedCausalPattern.MatchString(content.Summary) {
		return errors.New("summary contains unsupported numeric or causal language")
	}
	known := make(map[string]evidence, len(evidenceBundle))
	allowedNumbers := make(map[string]struct{})
	for _, item := range evidenceBundle {
		known[item.ID] = item
		for _, token := range numberPattern.FindAllString(item.DisplayValue+" "+item.Period, -1) {
			allowedNumbers[normalizeNumberToken(token)] = struct{}{}
		}
	}
	for _, item := range content.Statements {
		if strings.TrimSpace(item.Text) == "" || len(item.EvidenceIDs) == 0 {
			return errors.New("every explanation statement must have text and evidence IDs")
		}
		if unsupportedCausalPattern.MatchString(item.Text) {
			return errors.New("explanation statement contains unsupported causal language")
		}
		for _, id := range item.EvidenceIDs {
			if _, ok := known[id]; !ok {
				return fmt.Errorf("unknown evidence ID %q", id)
			}
		}
		for _, token := range numberPattern.FindAllString(item.Text, -1) {
			if _, ok := allowedNumbers[normalizeNumberToken(token)]; !ok {
				return fmt.Errorf("unsupported numeric token %q", token)
			}
		}
	}
	return nil
}

func normalizeNumberToken(value string) string {
	return strings.Trim(strings.ReplaceAll(value, ",", ""), ".")
}

func loadProduct(dataDir, iso3 string) productFile {
	var product productFile
	_ = readJSON(filepath.Join(dataDir, "products", iso3+".json"), &product)
	return product
}

func evidenceByID(items []evidence, id string) evidence {
	for _, item := range items {
		if item.ID == id {
			return item
		}
	}
	return evidence{}
}

func displayPeriod(value string) string {
	if strings.TrimSpace(value) == "" {
		return "an unavailable period"
	}
	return value
}

func fallbackName(name, iso3 string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return iso3
}

func formatUSD(value float64) string {
	return "$" + commaInteger(value)
}

func commaInteger(value float64) string {
	raw := strconv.FormatInt(int64(value+0.5), 10)
	start := 0
	if strings.HasPrefix(raw, "-") {
		start = 1
	}
	for index := len(raw) - 3; index > start; index -= 3 {
		raw = raw[:index] + "," + raw[index:]
	}
	return raw
}

func readJSON(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewDecoder(file).Decode(target)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func fatalf(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
