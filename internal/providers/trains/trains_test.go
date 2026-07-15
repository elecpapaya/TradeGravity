package trains

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"tradegravity/internal/model"
)

const countriesFixture = `<?xml version="1.0" encoding="utf-8"?>
<wits:datasource xmlns:wits="http://wits.worldbank.org"><wits:countries>
<wits:country countrycode="840" isreporter="1" ispartner="1" isgroup="No"><wits:iso3Code>USA</wits:iso3Code><wits:name>United States</wits:name></wits:country>
<wits:country countrycode="000" isreporter="0" ispartner="1" isgroup="Yes"><wits:iso3Code>WLD</wits:iso3Code><wits:name>World</wits:name></wits:country>
</wits:countries></wits:datasource>`

const availabilityFixture = `<?xml version="1.0" encoding="utf-8"?>
<wits:datasource xmlns:wits="http://wits.worldbank.org"><wits:dataavailability>
<wits:reporter countrycode="840" iso3Code="USA"><wits:year>2020</wits:year><wits:reporternernomenclature reporternernomenclaturecode="H5">HS 2017</wits:reporternernomenclature><wits:partnerlist>000;</wits:partnerlist><wits:isspecificdutyexpressionestimatedavailable>Yes</wits:isspecificdutyexpressionestimatedavailable><wits:lastupdateddate>2021/08/30</wits:lastupdateddate></wits:reporter>
<wits:reporter countrycode="840" iso3Code="USA"><wits:year>2021</wits:year><wits:reporternernomenclature reporternernomenclaturecode="H5">HS 2017</wits:reporternernomenclature><wits:partnerlist>000;</wits:partnerlist><wits:isspecificdutyexpressionestimatedavailable>Yes</wits:isspecificdutyexpressionestimatedavailable><wits:lastupdateddate>2025/08/11</wits:lastupdateddate></wits:reporter>
</wits:dataavailability></wits:datasource>`

const tariffFixture = `{
  "dataSets":[{"series":{"0:0:0:0:0":{"observations":{"0":[26.3999996185303,0,null,0,0,0,0,0,0,0,0,0]}}}}],
  "structure":{"dimensions":{"series":[
    {"id":"FREQ","values":[{"id":"A"}]},
    {"id":"REPORTER","values":[{"id":"840"}]},
    {"id":"PARTNER","values":[{"id":"000"}]},
    {"id":"PRODUCTCODE","values":[{"id":"020110"}]},
    {"id":"DATATYPE","values":[{"id":"Reported"}]}
  ],"observation":[{"id":"TIME_PERIOD","values":[{"id":"2021"}]}]},
  "attributes":{"observation":[
    {"id":"NOMENCODE","values":[{"id":"H5"}]},
    {"id":"EXCLUDEDFROM","values":[]},
    {"id":"TARIFFTYPE","values":[{"id":"MFN"}]},
    {"id":"SUM_OF_RATES","values":[{"id":"26.3999996185303"}]},
    {"id":"MIN_RATE","values":[{"id":"26.3999996185303"}]},
    {"id":"MAX_RATE","values":[{"id":"26.3999996185303"}]},
    {"id":"TOTALNOOFLINES","values":[{"id":"3"}]},
    {"id":"NBR_PREF_LINES","values":[{"id":"0"}]},
    {"id":"NBR_MFN_LINES","values":[{"id":"1"}]},
    {"id":"NBR_NA_LINES","values":[{"id":"2"}]},
    {"id":"OBS_VALUE_MEASURE","values":[{"id":"SimpleAverage"}]}
  ]}}}`

func TestFetchTariffsMapsCountriesAvailabilityAndSDMXAttributes(t *testing.T) {
	var dataRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.URL.Path == "/wits/datasource/trn/country/ALL":
			writer.Header().Set("Content-Type", "application/xml")
			_, _ = writer.Write([]byte(countriesFixture))
		case request.URL.Path == "/wits/datasource/trn/dataavailability/country/840/year/all":
			writer.Header().Set("Content-Type", "application/xml")
			_, _ = writer.Write([]byte(availabilityFixture))
		case strings.Contains(request.URL.Path, "/DF_WITS_Tariff_TRAINS/.840.000.020110.aveestimated/"):
			dataRequests.Add(1)
			if request.URL.Query().Get("startperiod") != "2021" || request.URL.Query().Get("endperiod") != "2021" {
				t.Fatalf("unexpected period query: %s", request.URL.RawQuery)
			}
			if request.Header.Get("Accept") != sdmxJSONAccept {
				t.Fatalf("unexpected Accept header: %q", request.Header.Get("Accept"))
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(tariffFixture))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	provider, err := NewWithConfig(Config{BaseURL: server.URL, Timeout: time.Second, Retries: 0, Backoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	year, err := provider.LatestTariffYear(context.Background(), "USA")
	if err != nil || year != "2021" {
		t.Fatalf("LatestTariffYear() = %q, %v", year, err)
	}
	rows, err := provider.FetchTariffs(context.Background(), "USA", "WLD", "2021", []string{"020110"}, model.TariffAVEEstimated)
	if err != nil {
		t.Fatal(err)
	}
	if dataRequests.Load() != 1 || len(rows) != 1 {
		t.Fatalf("requests/rows = %d/%d, want 1/1", dataRequests.Load(), len(rows))
	}
	row := rows[0]
	if row.Classification != "HS2017" || row.Nomenclature != "H5" || row.RateType != model.TariffMFNApplied || row.DataType != model.TariffAVEEstimated {
		t.Fatalf("unexpected tariff identity: %#v", row)
	}
	if row.RatePercent < 26.39 || row.TotalLines != 3 || row.MFNLines != 1 || row.NonAdValoremLines != 2 {
		t.Fatalf("unexpected tariff measures: %#v", row)
	}
	if row.SourceUpdatedAt.Format("2006-01-02") != "2025-08-11" {
		t.Fatalf("source updated = %v", row.SourceUpdatedAt)
	}
}

func TestParseTariffsHandlesWITSPartnerProductSeriesKeyOrder(t *testing.T) {
	payloadText := strings.ReplaceAll(tariffFixture,
		`"0:0:0:0:0":{"observations":{"0":[26.3999996185303,0,null,0,0,0,0,0,0,0,0,0]}}`,
		`"0:0:0:0:0":{"observations":{"0":[26.3999996185303,0,null,0,0,0,0,0,0,0,0,0]}},"0:0:1:0:0":{"observations":{"0":[5,0,null,0,0,0,0,0,0,0,0,0]}}`)
	payloadText = strings.ReplaceAll(payloadText,
		`{"id":"PRODUCTCODE","values":[{"id":"020110"}]}`,
		`{"id":"PRODUCTCODE","values":[{"id":"020110"},{"id":"854231"}]}`)
	var payload sdmxResponse
	if err := json.Unmarshal([]byte(payloadText), &payload); err != nil {
		t.Fatal(err)
	}
	rows, err := parseTariffs(payload, "USA", "WLD", "000", model.TariffAVEEstimated, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ProductCode != "020110" || rows[1].ProductCode != "854231" {
		t.Fatalf("multi-product rows = %#v", rows)
	}
}

func TestFetchTariffsRejectsUnavailablePartnerBeforeDataRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case strings.Contains(request.URL.Path, "/country/ALL"):
			_, _ = writer.Write([]byte(strings.ReplaceAll(countriesFixture, "</wits:countries>", `<wits:country countrycode="156" isreporter="1" ispartner="1" isgroup="No"><wits:iso3Code>CHN</wits:iso3Code><wits:name>China</wits:name></wits:country></wits:countries>`)))
		case strings.Contains(request.URL.Path, "/dataavailability/"):
			_, _ = writer.Write([]byte(availabilityFixture))
		default:
			t.Fatal("data endpoint should not be called")
		}
	}))
	defer server.Close()
	provider, err := NewWithConfig(Config{BaseURL: server.URL, Retries: 0, Backoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.FetchTariffs(context.Background(), "USA", "CHN", "2021", []string{"020110"}, model.TariffReported)
	if !errors.Is(err, ErrPartnerUnavailable) {
		t.Fatalf("error = %v, want ErrPartnerUnavailable", err)
	}
}

func TestDoRequestRetriesRateLimitAndReturnsSafeError(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requests.Add(1) < 3 {
			http.Error(writer, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()
	provider, err := NewWithConfig(Config{BaseURL: server.URL, Retries: 2, Backoff: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	body, err := provider.doRequest(context.Background(), "test", nil, "text/plain")
	if err != nil || string(body) != "ok" || requests.Load() != 3 {
		t.Fatalf("body/error/requests = %q/%v/%d", body, err, requests.Load())
	}
}
