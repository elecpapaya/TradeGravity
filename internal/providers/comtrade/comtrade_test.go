package comtrade

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestFetchPartnerMatrixOmitsPartnerCodeAndFiltersWorldAggregate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/files/reporters":
			_, _ = writer.Write([]byte(`{"results":[{"id":"410","iso3":"KOR","text":"Korea","isReporter":true,"isGroup":false}]}`))
		case "/files/partners":
			_, _ = writer.Write([]byte(`{"results":[{"id":"842","iso3":"USA","text":"United States","isPartner":true,"isGroup":false},{"id":"156","iso3":"CHN","text":"China","isPartner":true,"isGroup":false},{"id":"899","iso3":"S19","text":"Special category","isPartner":true,"isGroup":false}]}`))
		case "/preview":
			if request.URL.Query().Has("partnerCode") {
				t.Fatalf("matrix request must omit partnerCode, got %s", request.URL.RawQuery)
			}
			if request.URL.Query().Get("cmdCode") != "TOTAL" || request.URL.Query().Get("breakdownMode") != "classic" {
				t.Fatalf("unexpected matrix query %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"data":[
				{"period":"2023","primaryValue":100,"reporterCode":410,"reporterISO":null,"partnerCode":842,"partnerISO":null,"cmdCode":"TOTAL","classificationSearchCode":"H6"},
				{"period":"2023","primaryValue":80,"reporterCode":410,"reporterISO":null,"partnerCode":156,"partnerISO":null,"cmdCode":"TOTAL","classificationSearchCode":"H6"},
				{"period":"2023","primaryValue":999,"reporterCode":410,"reporterISO":null,"partnerCode":0,"partnerISO":null,"cmdCode":"TOTAL","classificationSearchCode":"H6"},
				{"period":"2023","primaryValue":20,"reporterCode":410,"reporterISO":null,"partnerCode":899,"partnerISO":null,"cmdCode":"TOTAL","classificationSearchCode":"H6"}
			]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider, err := NewWithConfig(Config{
		BaseURL: server.URL, DataPath: "data", PreviewDataPath: "preview",
		ReportersURL: server.URL + "/files/reporters", PartnersURL: server.URL + "/files/partners",
		MaxRecords: 500, Timeout: time.Second, RateLimitPerSec: 100, RateLimitBurst: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := provider.FetchPartnerMatrix(context.Background(), "KOR", model.FlowExport, "2023")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].PartnerISO3 != "USA" || rows[1].PartnerISO3 != "CHN" {
		t.Fatalf("matrix rows = %#v", rows)
	}
	for _, row := range rows {
		if row.ProductCode != "TOTAL" || row.ProductLevel != 0 || strings.TrimSpace(row.Provider) != "comtrade" {
			t.Fatalf("invalid matrix row = %#v", row)
		}
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

func TestNormalizeProductCodesValidatesAndDeduplicatesHS6(t *testing.T) {
	got, err := normalizeProductCodes([]string{"854231", " 850760 ", "854231"}, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "854231" || got[1] != "850760" {
		t.Fatalf("normalizeProductCodes() = %v", got)
	}
	if _, err := normalizeProductCodes([]string{"8542"}, 6); err == nil {
		t.Fatal("normalizeProductCodes() accepted an HS4 code for HS6 collection")
	}
}

func TestFetchProductPeriodsBatchesMonthlyPeriodsAndFiltersExactCodes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/files/reporters":
			_, _ = writer.Write([]byte(`{"results":[{"id":"410","iso3":"KOR","text":"Korea","isReporter":true,"isGroup":false}]}`))
		case "/files/partners":
			_, _ = writer.Write([]byte(`{"results":[{"id":"842","iso3":"USA","text":"United States","isPartner":true,"isGroup":false}]}`))
		case "/data/M", "/data/M/":
			if request.URL.Query().Get("period") != "202401,202402" || request.URL.Query().Get("cmdCode") != "854231,854232" {
				t.Fatalf("unexpected monthly query %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"data":[
				{"period":"202401","primaryValue":100,"cmdCode":"854231","classificationSearchCode":"H6"},
				{"period":"202402","primaryValue":120,"cmdCode":"854232","classificationSearchCode":"H6"},
				{"period":"202402","primaryValue":999,"cmdCode":"TOTAL","classificationSearchCode":"H6"}
			]}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	provider, err := NewWithConfig(Config{
		BaseURL: server.URL, DataPath: "data/{freq}", PreviewDataPath: "data/{freq}", Frequency: "M",
		ReportersURL: server.URL + "/files/reporters", PartnersURL: server.URL + "/files/partners",
		MaxRecords: 500, Timeout: time.Second, RateLimitPerSec: 100, RateLimitBurst: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := provider.FetchProductPeriods(context.Background(), "KOR", "USA", model.FlowExport, []string{"2024-01", "202402"}, 6, []string{"854231", "854232"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Period != "2024-01" || rows[1].Period != "2024-02" || rows[0].Provider != "comtrade" {
		t.Fatalf("unexpected monthly rows: %#v", rows)
	}
}
