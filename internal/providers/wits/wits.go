package wits

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"tradegravity/internal/model"
	"tradegravity/internal/providers"
)

const (
	defaultBaseURL           = "https://wits.worldbank.org/API/V1/"
	defaultTradePathTemplate = "SDMX/V21/datasource/tradestats-trade/reporter/{reporter}/year/{year}/partner/{partner}/product/{product}/indicator/{indicator}"
	defaultReportersPath     = "wits/datasource/tradestats-trade/country/ALL"
	defaultDataAvailPath     = "wits/datasource/tradestats-trade/dataavailability/country/{reporter}/indicator/{indicator}"
	defaultAPIKeyParam       = "token"
	defaultFormatParam       = "format"
	defaultFormatValue       = "JSON"
	defaultRateLimitPerSec   = 5
	defaultRateLimitBurst    = 5
	defaultTimeoutSeconds    = 20
	defaultUserAgent         = "TradeGravity/0.1"
	defaultIndicatorExport   = "XPRT-TRD-VL"
	defaultIndicatorImport   = "MPRT-TRD-VL"
	defaultProductCode       = "Total"
	defaultYearAllValue      = "all"
	defaultValueMultiplier   = 1000
	defaultAutoLatestYear    = true
)

var ErrNoRecords = errors.New("wits: no records found")

type Config struct {
	BaseURL           string
	TradePathTemplate string
	ReportersPath     string
	DataAvailPath     string
	APIKey            string
	APIKeyParam       string
	FormatParam       string
	FormatValue       string
	RateLimitPerSec   int
	RateLimitBurst    int
	Timeout           time.Duration
	UserAgent         string
	IndicatorExport   string
	IndicatorImport   string
	ProductCode       string
	YearAllValue      string
	ValueMultiplier   float64
	AutoLatestYear    bool
}

type Provider struct {
	config  Config
	client  *http.Client
	limiter *rateLimiter
	mu      sync.Mutex
	yearMap map[string]string
}

func New() (*Provider, error) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewWithConfig(cfg)
}

func NewWithConfig(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, errors.New("wits base url is required")
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/") + "/"
	if strings.TrimSpace(cfg.TradePathTemplate) == "" {
		cfg.TradePathTemplate = defaultTradePathTemplate
	}
	if strings.TrimSpace(cfg.ReportersPath) == "" {
		cfg.ReportersPath = defaultReportersPath
	}
	if strings.TrimSpace(cfg.DataAvailPath) == "" {
		cfg.DataAvailPath = defaultDataAvailPath
	}
	if cfg.APIKeyParam == "" {
		cfg.APIKeyParam = defaultAPIKeyParam
	}
	if cfg.FormatParam == "" {
		cfg.FormatParam = defaultFormatParam
	}
	if cfg.FormatValue == "" {
		cfg.FormatValue = defaultFormatValue
	}
	if cfg.RateLimitPerSec <= 0 {
		cfg.RateLimitPerSec = defaultRateLimitPerSec
	}
	if cfg.RateLimitBurst <= 0 {
		cfg.RateLimitBurst = defaultRateLimitBurst
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeoutSeconds * time.Second
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.IndicatorExport == "" {
		cfg.IndicatorExport = defaultIndicatorExport
	}
	if cfg.IndicatorImport == "" {
		cfg.IndicatorImport = defaultIndicatorImport
	}
	if cfg.ProductCode == "" {
		cfg.ProductCode = defaultProductCode
	}
	if cfg.YearAllValue == "" {
		cfg.YearAllValue = defaultYearAllValue
	}
	if cfg.ValueMultiplier == 0 {
		cfg.ValueMultiplier = defaultValueMultiplier
	}
	return &Provider{
		config:  cfg,
		client:  &http.Client{Timeout: cfg.Timeout},
		limiter: newRateLimiter(cfg.RateLimitPerSec, cfg.RateLimitBurst),
		yearMap: make(map[string]string),
	}, nil
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		BaseURL:           getenv("WITS_BASE_URL", defaultBaseURL),
		TradePathTemplate: getenv("WITS_TRADE_PATH", defaultTradePathTemplate),
		ReportersPath:     getenv("WITS_REPORTERS_PATH", defaultReportersPath),
		DataAvailPath:     getenv("WITS_DATAAVAIL_PATH", defaultDataAvailPath),
		APIKey:            strings.TrimSpace(os.Getenv("WITS_API_KEY")),
		APIKeyParam:       getenv("WITS_API_KEY_PARAM", defaultAPIKeyParam),
		FormatParam:       getenv("WITS_FORMAT_PARAM", defaultFormatParam),
		FormatValue:       getenv("WITS_FORMAT_VALUE", defaultFormatValue),
		UserAgent:         getenv("WITS_USER_AGENT", defaultUserAgent),
		IndicatorExport:   getenv("WITS_INDICATOR_EXPORT", defaultIndicatorExport),
		IndicatorImport:   getenv("WITS_INDICATOR_IMPORT", defaultIndicatorImport),
		ProductCode:       getenv("WITS_PRODUCT_CODE", defaultProductCode),
		YearAllValue:      getenv("WITS_YEAR_ALL", defaultYearAllValue),
		ValueMultiplier:   getenvFloat("WITS_VALUE_MULTIPLIER", defaultValueMultiplier),
		AutoLatestYear:    getenvBool("WITS_AUTO_LATEST_YEAR", defaultAutoLatestYear),
	}

	cfg.RateLimitPerSec = getenvInt("WITS_RATE_LIMIT_PER_SEC", defaultRateLimitPerSec)
	cfg.RateLimitBurst = getenvInt("WITS_RATE_LIMIT_BURST", defaultRateLimitBurst)
	cfg.Timeout = time.Duration(getenvInt("WITS_TIMEOUT_SECONDS", defaultTimeoutSeconds)) * time.Second

	return cfg, nil
}

func (p *Provider) Name() string {
	return "wits"
}

func (p *Provider) ListReporters(ctx context.Context) ([]model.Reporter, error) {
	body, err := p.doRequest(ctx, p.config.ReportersPath, nil, "application/xml")
	if err != nil {
		return nil, err
	}
	reporters, err := parseReportersXML(body)
	if err != nil {
		return nil, err
	}

	if len(reporters) == 0 {
		return nil, errors.New("wits: no reporters parsed")
	}
	return reporters, nil
}

func (p *Provider) FetchLatest(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow) (model.Observation, error) {
	series, err := p.FetchSeries(ctx, reporterISO3, partnerISO3, flow, "", "")
	if err != nil {
		return model.Observation{}, err
	}
	if len(series) == 0 {
		return model.Observation{}, errors.New("wits: no observations returned")
	}

	latest, ok := pickLatest(series)
	if !ok {
		return model.Observation{}, errors.New("wits: unable to select latest observation")
	}
	return latest, nil
}

func (p *Provider) FetchSeries(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow, from, to string) ([]model.Observation, error) {
	indicator := p.indicatorForFlow(flow)
	yearValue, err := p.resolveYear(ctx, reporterISO3, indicator, from, to)
	if err != nil {
		return nil, err
	}
	path, params := p.tradePath(reporterISO3, partnerISO3, indicator, yearValue)
	var payload sdmxResponse
	if err := p.doJSON(ctx, path, params, &payload); err != nil {
		return nil, err
	}

	observations, err := parseSDMXObservations(payload, flow, reporterISO3, partnerISO3, p.config.ValueMultiplier)
	if err != nil {
		return nil, err
	}
	for i := range observations {
		observations[i].Provider = p.Name()
	}
	return observations, nil
}

func (p *Provider) tradePath(reporterISO3, partnerISO3, indicator, yearValue string) (string, url.Values) {
	path := p.config.TradePathTemplate
	params := url.Values{}

	product := p.config.ProductCode
	if strings.Contains(path, "{reporter}") {
		path = strings.ReplaceAll(path, "{reporter}", url.PathEscape(reporterISO3))
	} else {
		params.Set("reporter", reporterISO3)
	}
	if strings.Contains(path, "{partner}") {
		path = strings.ReplaceAll(path, "{partner}", url.PathEscape(partnerISO3))
	} else {
		params.Set("partner", partnerISO3)
	}
	if strings.Contains(path, "{indicator}") {
		path = strings.ReplaceAll(path, "{indicator}", url.PathEscape(indicator))
	} else {
		params.Set("indicator", indicator)
	}
	if strings.Contains(path, "{product}") {
		path = strings.ReplaceAll(path, "{product}", url.PathEscape(product))
	} else if product != "" {
		params.Set("product", product)
	}
	if strings.Contains(path, "{year}") {
		path = strings.ReplaceAll(path, "{year}", url.PathEscape(yearValue))
	} else if yearValue != "" {
		params.Set("year", yearValue)
	}

	return path, params
}

func (p *Provider) indicatorForFlow(flow model.Flow) string {
	switch flow {
	case model.FlowExport:
		return p.config.IndicatorExport
	case model.FlowImport:
		return p.config.IndicatorImport
	default:
		return string(flow)
	}
}

func (p *Provider) resolveYear(ctx context.Context, reporterISO3, indicator, from, to string) (string, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)

	if from == "" && to == "" {
		if p.config.AutoLatestYear {
			year, err := p.latestYear(ctx, reporterISO3, indicator)
			if err == nil && year != "" {
				return year, nil
			}
		}
		return p.config.YearAllValue, nil
	}
	if from != "" && to != "" && from != to {
		return from + ";" + to, nil
	}
	if from != "" {
		return from, nil
	}
	return to, nil
}

func (p *Provider) doJSON(ctx context.Context, path string, params url.Values, dest any) error {
	body, err := p.doRequest(ctx, path, params, "application/json")
	if err != nil {
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	return nil
}

func (p *Provider) doRequest(ctx context.Context, path string, params url.Values, accept string) ([]byte, error) {
	endpoint, err := p.buildURL(path, params)
	if err != nil {
		return nil, err
	}

	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if p.config.UserAgent != "" {
		req.Header.Set("User-Agent", p.config.UserAgent)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusNotFound && strings.Contains(string(body), "NoRecordsFound") {
		return nil, ErrNoRecords
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("wits: request failed (%s): %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return body, nil
}

func (p *Provider) buildURL(path string, params url.Values) (string, error) {
	base := strings.TrimRight(p.config.BaseURL, "/")
	path = strings.TrimLeft(path, "/")
	endpoint := base + "/" + path

	query := url.Values{}
	for key, values := range params {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	if p.config.APIKey != "" && p.config.APIKeyParam != "" {
		query.Set(p.config.APIKeyParam, p.config.APIKey)
	}
	if p.config.FormatParam != "" && p.config.FormatValue != "" {
		query.Set(p.config.FormatParam, p.config.FormatValue)
	}
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	return endpoint, nil
}

type rateLimiter struct {
	tokens chan struct{}
}

func newRateLimiter(ratePerSec, burst int) *rateLimiter {
	if ratePerSec <= 0 {
		return nil
	}
	if burst <= 0 {
		burst = 1
	}

	limiter := &rateLimiter{
		tokens: make(chan struct{}, burst),
	}
	for i := 0; i < burst; i++ {
		limiter.tokens <- struct{}{}
	}

	interval := time.Second / time.Duration(ratePerSec)
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			select {
			case limiter.tokens <- struct{}{}:
			default:
			}
		}
	}()

	return limiter
}

func (l *rateLimiter) Wait(ctx context.Context) error {
	if l == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.tokens:
		return nil
	}
}

type dataAvailabilityResponse struct {
	Reporters []dataAvailabilityReporter `xml:"dataavailability>reporter"`
}

type dataAvailabilityReporter struct {
	Year string `xml:"year"`
}

func (p *Provider) latestYear(ctx context.Context, reporterISO3, indicator string) (string, error) {
	cacheKey := strings.ToUpper(strings.TrimSpace(reporterISO3)) + "|" + strings.ToUpper(strings.TrimSpace(indicator))
	p.mu.Lock()
	if year, ok := p.yearMap[cacheKey]; ok {
		p.mu.Unlock()
		return year, nil
	}
	p.mu.Unlock()

	path := p.dataAvailabilityPath(reporterISO3, indicator)
	body, err := p.doRequest(ctx, path, nil, "application/xml")
	if err != nil {
		return "", err
	}

	var response dataAvailabilityResponse
	if err := xml.Unmarshal(body, &response); err != nil {
		return "", err
	}

	maxYear := 0
	for _, entry := range response.Reporters {
		yearValue := strings.TrimSpace(entry.Year)
		if yearValue == "" {
			continue
		}
		year, err := strconv.Atoi(yearValue)
		if err != nil {
			continue
		}
		if year > maxYear {
			maxYear = year
		}
	}

	if maxYear == 0 {
		return "", errors.New("wits: no data availability years")
	}
	latest := strconv.Itoa(maxYear)

	p.mu.Lock()
	p.yearMap[cacheKey] = latest
	p.mu.Unlock()

	return latest, nil
}

func (p *Provider) dataAvailabilityPath(reporterISO3, indicator string) string {
	path := p.config.DataAvailPath
	if strings.Contains(path, "{reporter}") {
		path = strings.ReplaceAll(path, "{reporter}", url.PathEscape(reporterISO3))
	}
	if strings.Contains(path, "{indicator}") {
		path = strings.ReplaceAll(path, "{indicator}", url.PathEscape(indicator))
	}
	return path
}

type witsCountryList struct {
	Countries []witsCountry `xml:"countries>country"`
}

type witsCountry struct {
	ISO3       string `xml:"iso3Code"`
	Name       string `xml:"name"`
	IsReporter string `xml:"isreporter,attr"`
	IsGroup    string `xml:"isgroup,attr"`
}

func parseReportersXML(payload []byte) ([]model.Reporter, error) {
	var response witsCountryList
	if err := xml.Unmarshal(payload, &response); err != nil {
		return nil, err
	}

	reporters := make([]model.Reporter, 0, len(response.Countries))
	for _, country := range response.Countries {
		if strings.TrimSpace(country.ISO3) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(country.IsReporter), "1") {
			if strings.EqualFold(strings.TrimSpace(country.IsGroup), "yes") {
				continue
			}
			reporters = append(reporters, model.Reporter{
				ISO3:     strings.ToUpper(strings.TrimSpace(country.ISO3)),
				NameEN:   strings.TrimSpace(country.Name),
				NameKO:   "",
				Region:   "",
				IsActive: true,
			})
		}
	}

	return reporters, nil
}

type sdmxResponse struct {
	DataSets  []sdmxDataSet `json:"dataSets"`
	Structure sdmxStructure `json:"structure"`
}

type sdmxDataSet struct {
	Series       map[string]sdmxSeries `json:"series"`
	Observations map[string][]any      `json:"observations"`
}

type sdmxSeries struct {
	Observations map[string][]any `json:"observations"`
}

type sdmxStructure struct {
	Dimensions sdmxDimensions `json:"dimensions"`
}

type sdmxDimensions struct {
	Series      []sdmxDimension `json:"series"`
	Observation []sdmxDimension `json:"observation"`
}

type sdmxDimension struct {
	ID     string      `json:"id"`
	Values []sdmxValue `json:"values"`
}

type sdmxValue struct {
	ID string `json:"id"`
}

func parseSDMXObservations(payload sdmxResponse, fallbackFlow model.Flow, reporterISO3, partnerISO3 string, multiplier float64) ([]model.Observation, error) {
	if len(payload.DataSets) == 0 {
		return nil, errors.New("wits: missing dataset")
	}
	if len(payload.Structure.Dimensions.Observation) == 0 {
		return nil, errors.New("wits: missing observation dimension")
	}

	seriesDims := payload.Structure.Dimensions.Series
	seriesValues := make([][]string, len(seriesDims))
	for i, dim := range seriesDims {
		values := make([]string, len(dim.Values))
		for j, value := range dim.Values {
			values[j] = value.ID
		}
		seriesValues[i] = values
	}

	timeDim := payload.Structure.Dimensions.Observation[0]
	timeValues := make([]string, len(timeDim.Values))
	for i, value := range timeDim.Values {
		timeValues[i] = value.ID
	}

	dataSet := payload.DataSets[0]
	if len(dataSet.Series) == 0 {
		return nil, errors.New("wits: empty series response")
	}

	observations := make([]model.Observation, 0)
	for seriesKey, series := range dataSet.Series {
		indices, ok := parseSeriesKey(seriesKey, len(seriesDims))
		if !ok {
			continue
		}

		dimensionValues := map[string]string{}
		for i, dim := range seriesDims {
			if i >= len(indices) || indices[i] < 0 || indices[i] >= len(seriesValues[i]) {
				continue
			}
			dimensionValues[dim.ID] = seriesValues[i][indices[i]]
		}

		reporter := reporterISO3
		if value, ok := dimensionValues["REPORTER"]; ok && value != "" {
			reporter = value
		}
		partner := partnerISO3
		if value, ok := dimensionValues["PARTNER"]; ok && value != "" {
			partner = value
		}

		flow := fallbackFlow
		if indicator, ok := dimensionValues["INDICATOR"]; ok {
			if mappedFlow, ok := flowFromIndicator(indicator); ok {
				flow = mappedFlow
			}
		}

		for obsKey, obsValue := range series.Observations {
			index, err := strconv.Atoi(obsKey)
			if err != nil || index < 0 || index >= len(timeValues) {
				continue
			}
			periodType, period, ok := normalizePeriod(timeValues[index])
			if !ok {
				continue
			}
			value, ok := parseSDMXValue(obsValue)
			if !ok {
				continue
			}

			observations = append(observations, model.Observation{
				ReporterISO3: strings.ToUpper(reporter),
				PartnerISO3:  strings.ToUpper(partner),
				Flow:         flow,
				PeriodType:   periodType,
				Period:       period,
				ValueUSD:     value * multiplier,
			})
		}
	}

	if len(observations) == 0 {
		return nil, errors.New("wits: no observations parsed")
	}
	return observations, nil
}

func parseSeriesKey(key string, expected int) ([]int, bool) {
	parts := strings.Split(key, ":")
	if expected > 0 && len(parts) != expected {
		return nil, false
	}
	indices := make([]int, len(parts))
	for i, part := range parts {
		index, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		indices[i] = index
	}
	return indices, true
}

func parseSDMXValue(values []any) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}
	switch typed := values[0].(type) {
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func flowFromIndicator(indicator string) (model.Flow, bool) {
	upper := strings.ToUpper(strings.TrimSpace(indicator))
	switch {
	case strings.HasPrefix(upper, "XPRT"):
		return model.FlowExport, true
	case strings.HasPrefix(upper, "MPRT"):
		return model.FlowImport, true
	default:
		return "", false
	}
}

func extractRows(payload any) ([]map[string]any, error) {
	switch typed := payload.(type) {
	case []any:
		return toRowList(typed), nil
	case map[string]any:
		for _, key := range []string{"dataset", "Dataset", "data", "Data", "results", "Results", "value", "Value"} {
			if raw, ok := typed[key]; ok {
				return extractRows(raw)
			}
		}
		return nil, errors.New("wits: unexpected response shape")
	default:
		return nil, errors.New("wits: unexpected response type")
	}
}

func toRowList(items []any) []map[string]any {
	rows := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rows = append(rows, row)
	}
	return rows
}

func rowToObservation(row map[string]any, reporterISO3, partnerISO3 string, flow model.Flow, multiplier float64) (model.Observation, error) {
	value, ok := getFloat(row,
		"TradeValue", "tradeValue", "TradeValueUSD", "TradeValueUS$", "TradeValueUS",
		"TradeValue1000USD", "Value", "value",
	)
	if !ok {
		return model.Observation{}, errors.New("wits: missing trade value")
	}
	value *= multiplier

	periodType, period, ok := periodFromRow(row)
	if !ok {
		return model.Observation{}, errors.New("wits: missing period")
	}

	if rowFlow, ok := flowFromRow(row); ok {
		flow = rowFlow
	}

	reporter := reporterISO3
	if rowReporter, ok := getString(row, "ReporterISO3", "Reporter", "reporter", "ReporterCode"); ok && rowReporter != "" {
		reporter = rowReporter
	}
	partner := partnerISO3
	if rowPartner, ok := getString(row, "PartnerISO3", "Partner", "partner", "PartnerCode"); ok && rowPartner != "" {
		partner = rowPartner
	}

	return model.Observation{
		ReporterISO3: strings.ToUpper(reporter),
		PartnerISO3:  strings.ToUpper(partner),
		Flow:         flow,
		PeriodType:   periodType,
		Period:       period,
		ValueUSD:     value,
	}, nil
}

func periodFromRow(row map[string]any) (model.PeriodType, string, bool) {
	if raw, ok := getString(row, "Period", "period", "Time", "time"); ok {
		if periodType, period, ok := normalizePeriod(raw); ok {
			return periodType, period, ok
		}
	}

	year, _ := getString(row, "Year", "year")
	month, _ := getString(row, "Month", "month")
	quarter, _ := getString(row, "Quarter", "quarter")

	if year != "" {
		if parsedYear, ok := parseYear(year); ok {
			year = fmt.Sprintf("%04d", parsedYear)
		}
		if month != "" {
			monthNumber, err := strconv.Atoi(month)
			if err == nil && monthNumber >= 1 && monthNumber <= 12 {
				return model.PeriodMonth, fmt.Sprintf("%s-%02d", year, monthNumber), true
			}
		}
		if quarter != "" {
			quarterNumber, err := strconv.Atoi(quarter)
			if err == nil && quarterNumber >= 1 && quarterNumber <= 4 {
				return model.PeriodQuarter, fmt.Sprintf("%s-Q%d", year, quarterNumber), true
			}
		}
		return model.PeriodYear, year, true
	}

	return "", "", false
}

func normalizePeriod(raw string) (model.PeriodType, string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", false
	}

	if year, month, ok := parseYearMonth(trimmed); ok {
		return model.PeriodMonth, fmt.Sprintf("%04d-%02d", year, month), true
	}
	if year, quarter, ok := parseYearQuarter(trimmed); ok {
		return model.PeriodQuarter, fmt.Sprintf("%04d-Q%d", year, quarter), true
	}
	if year, ok := parseYear(trimmed); ok {
		return model.PeriodYear, fmt.Sprintf("%04d", year), true
	}
	return "", "", false
}

func parseYearMonth(value string) (int, int, bool) {
	value = strings.TrimSpace(value)
	if len(value) == 6 && isDigits(value) {
		year, _ := strconv.Atoi(value[:4])
		month, _ := strconv.Atoi(value[4:])
		if month >= 1 && month <= 12 {
			return year, month, true
		}
	}

	parts := strings.Split(value, "-")
	if len(parts) == 2 && len(parts[0]) == 4 {
		year, errYear := strconv.Atoi(parts[0])
		month, errMonth := strconv.Atoi(parts[1])
		if errYear == nil && errMonth == nil && month >= 1 && month <= 12 {
			return year, month, true
		}
	}
	return 0, 0, false
}

func parseYearQuarter(value string) (int, int, bool) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if strings.Contains(value, "-Q") {
		parts := strings.Split(value, "-Q")
		if len(parts) == 2 {
			year, errYear := strconv.Atoi(parts[0])
			quarter, errQuarter := strconv.Atoi(parts[1])
			if errYear == nil && errQuarter == nil && quarter >= 1 && quarter <= 4 {
				return year, quarter, true
			}
		}
	}
	if strings.Contains(value, "Q") {
		parts := strings.Split(value, "Q")
		if len(parts) == 2 {
			year, errYear := strconv.Atoi(parts[0])
			quarter, errQuarter := strconv.Atoi(parts[1])
			if errYear == nil && errQuarter == nil && quarter >= 1 && quarter <= 4 {
				return year, quarter, true
			}
		}
	}
	return 0, 0, false
}

func parseYear(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if len(value) != 4 || !isDigits(value) {
		return 0, false
	}
	year, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return year, true
}

func isDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func flowFromRow(row map[string]any) (model.Flow, bool) {
	if raw, ok := getString(row, "TradeFlow", "tradeFlow", "Flow", "flow"); ok {
		return normalizeFlow(raw)
	}
	return "", false
}

func normalizeFlow(value string) (model.Flow, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "export", "exports", "exp":
		return model.FlowExport, true
	case "import", "imports", "imp":
		return model.FlowImport, true
	default:
		return "", false
	}
}

func getString(row map[string]any, keys ...string) (string, bool) {
	value, ok := getValue(row, keys...)
	if !ok {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	case json.Number:
		return typed.String(), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case int:
		return strconv.Itoa(typed), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	default:
		return "", false
	}
}

func getFloat(row map[string]any, keys ...string) (float64, bool) {
	value, ok := getValue(row, keys...)
	if !ok {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func getValue(row map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		if value, ok := row[key]; ok {
			return value, ok
		}
	}
	for rowKey, value := range row {
		for _, key := range keys {
			if strings.EqualFold(rowKey, key) {
				return value, true
			}
		}
	}
	return nil, false
}

func pickLatest(observations []model.Observation) (model.Observation, bool) {
	selectedIndex := -1
	for i := range observations {
		if selectedIndex == -1 || compareObservation(observations[i], observations[selectedIndex]) > 0 {
			selectedIndex = i
		}
	}
	if selectedIndex == -1 {
		return model.Observation{}, false
	}
	return observations[selectedIndex], true
}

func compareObservation(a, b model.Observation) int {
	priorityA := periodPriority(a.PeriodType)
	priorityB := periodPriority(b.PeriodType)
	if priorityA != priorityB {
		if priorityA > priorityB {
			return 1
		}
		return -1
	}

	keyA := periodKey(a.PeriodType, a.Period)
	keyB := periodKey(b.PeriodType, b.Period)
	switch {
	case keyA > keyB:
		return 1
	case keyA < keyB:
		return -1
	default:
		return 0
	}
}

func periodPriority(periodType model.PeriodType) int {
	switch periodType {
	case model.PeriodMonth:
		return 3
	case model.PeriodQuarter:
		return 2
	case model.PeriodYear:
		return 1
	default:
		return 0
	}
}

func periodKey(periodType model.PeriodType, period string) int {
	switch periodType {
	case model.PeriodMonth:
		year, month, ok := parseYearMonth(period)
		if !ok {
			return 0
		}
		return year*100 + month
	case model.PeriodQuarter:
		year, quarter, ok := parseYearQuarter(period)
		if !ok {
			return 0
		}
		return year*10 + quarter
	case model.PeriodYear:
		year, ok := parseYear(period)
		if !ok {
			return 0
		}
		return year
	default:
		return 0
	}
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func getenvBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "y":
		return true
	case "0", "false", "no", "n":
		return false
	default:
		return fallback
	}
}

var _ providers.Provider = (*Provider)(nil)
