package comtrade

import (
	"testing"

	"tradegravity/internal/model"
)

func TestParseObservationsNormalizesProviderRows(t *testing.T) {
	body := []byte(`{
		"data": [
			{"period": "2024", "primaryValue": 12.5, "rt3ISO": "kor", "pt3ISO": "usa"},
			{"period": "invalid", "primaryValue": 99}
		]
	}`)

	got, err := parseObservations(body, model.FlowExport, "FALLBACK", "CHN", 1_000_000)
	if err != nil {
		t.Fatalf("parseObservations() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("parseObservations() returned %d rows, want 1", len(got))
	}
	observation := got[0]
	if observation.ReporterISO3 != "KOR" || observation.PartnerISO3 != "USA" {
		t.Fatalf("normalized reporter/partner = %s/%s, want KOR/USA", observation.ReporterISO3, observation.PartnerISO3)
	}
	if observation.PeriodType != model.PeriodYear || observation.Period != "2024" {
		t.Fatalf("normalized period = %s/%s, want Y/2024", observation.PeriodType, observation.Period)
	}
	if observation.ValueUSD != 12_500_000 {
		t.Fatalf("ValueUSD = %v, want 12500000", observation.ValueUSD)
	}
}

func TestQuotaAndRetryParsing(t *testing.T) {
	body := []byte(`{"message":"Daily quota exceeded; try again in 42 seconds"}`)
	if !isQuotaExceeded(body) {
		t.Fatal("isQuotaExceeded() = false, want true")
	}
	if got := parseRetrySeconds("Daily quota exceeded; try again in 42 seconds"); got != 42 {
		t.Fatalf("parseRetrySeconds() = %d, want 42", got)
	}
}
