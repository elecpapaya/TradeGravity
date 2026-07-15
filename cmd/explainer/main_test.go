package main

import (
	"encoding/json"
	"testing"
)

func floatPointer(value float64) *float64 { return &value }

func TestDeterministicExplanationCitesPublishedEvidence(t *testing.T) {
	row := latestEntry{
		ISO3: "VNM", Name: "Viet Nam", SamePeriod: true, ComparisonPeriod: "2023", ShareCN: 0.6,
		USA:        partnerBlock{Period: "2023", PeriodType: "Y", Trade: 400},
		CHN:        partnerBlock{Period: "2023", PeriodType: "Y", Trade: 600},
		Population: metric{Value: floatPointer(100)},
	}
	product := productFile{Provider: "comtrade", Periods: []string{"2023"}, Rows: []productEntry{{Period: "2023", Code: "85", Name: "Electrical machinery", Total: 300}}}
	evidenceBundle := buildEvidence(row, "wits", reporterSeries{}, product)
	result := deterministicExplanation(row, "2026-07-15T00:00:00Z", evidenceBundle)
	if result.ReporterISO3 != "VNM" || len(result.Statements) != 4 {
		t.Fatalf("unexpected deterministic explanation: %#v", result)
	}
	known := map[string]bool{}
	for _, item := range result.Evidence {
		known[item.ID] = true
	}
	for _, item := range result.Statements {
		for _, id := range item.EvidenceIDs {
			if !known[id] {
				t.Fatalf("statement cites unknown evidence %q", id)
			}
		}
	}
}

func TestParseResponseAndValidateGrounding(t *testing.T) {
	structured := aiContent{
		Summary: "Published evidence summary.",
		Statements: []statement{
			{Text: "USA trade is $400 in 2023.", EvidenceIDs: []string{"TOTAL-USA"}},
			{Text: "China trade is $600 in 2023.", EvidenceIDs: []string{"TOTAL-CHN"}},
		},
	}
	text, _ := json.Marshal(structured)
	envelope, _ := json.Marshal(map[string]any{"output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": string(text)}}}}})
	parsed, err := parseResponse(envelope)
	if err != nil {
		t.Fatal(err)
	}
	evidenceBundle := []evidence{
		{ID: "TOTAL-USA", DisplayValue: "$400", Period: "2023"},
		{ID: "TOTAL-CHN", DisplayValue: "$600", Period: "2023"},
	}
	if err := validateAIContent(parsed, evidenceBundle); err != nil {
		t.Fatal(err)
	}
	parsed.Statements[0].Text = "USA trade is $999 in 2023."
	if err := validateAIContent(parsed, evidenceBundle); err == nil {
		t.Fatal("expected unsupported numeric claim to be rejected")
	}
	parsed.Statements[0].Text = "USA trade is $400 in 2023 because demand increased."
	if err := validateAIContent(parsed, evidenceBundle); err == nil {
		t.Fatal("expected unsupported causal claim to be rejected")
	}
}
