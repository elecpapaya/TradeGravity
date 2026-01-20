package comtrade

import (
	"context"
	"encoding/json"
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
	defaultBaseURL           = "https://comtradeapi.un.org/"
	defaultDataPath          = "data/v1/get/{type}/{freq}/{cl}"
	defaultReportersURL      = "https://comtradeapi.un.org/files/v1/app/reference/Reporters.json"
	defaultPartnersURL       = "https://comtradeapi.un.org/files/v1/app/reference/partnerAreas.json"
	defaultAPIKeyParam       = "subscription-key"
	defaultType              = "C"
	defaultFrequency         = "A"
	defaultClassification    = "HS"
	defaultCommodity         = "TOTAL"
	defaultFlowExport        = "X"
	defaultFlowImport        = "M"
	defaultFormat            = "json"
	defaultMaxRecords        = 50000
	defaultLookbackYears     = 5
	defaultRateLimitPerSec   = 2
	defaultRateLimitBurst    = 2
	defaultTimeoutSeconds    = 30
	defaultUserAgent         = "TradeGravity/0.1"
	defaultValueMultiplier   = 1.0
	defaultAllowISO3Fallback = true
	defaultMaxRetries        = 3
)

var ErrNoRecords = errors.New("comtrade: no records found")
var ErrQuotaExceeded = errors.New("comtrade: quota exceeded")

type Config struct {
	BaseURL           string
	DataPath          string
	Dataset           string
	ReportersURL      string
	PartnersURL       string
	APIKeyPrimary     string
	APIKeySecondary   string
	APIKeyParam       string
	Type              string
	Frequency         string
	Classification    string
	Commodity         string
	FlowExport        string
	FlowImport        string
	Format            string
	MaxRecords        int
	LookbackYears     int
	Timeout           time.Duration
	UserAgent         string
	ValueMultiplier   float64
	AllowISO3Fallback bool
	RateLimitPerSec   int
	RateLimitBurst    int
	MaxRetries        int
}

type Provider struct {
	config       Config
	client       *http.Client
	limiter      *rateLimiter
	mu           sync.Mutex
	refsLoaded   bool
	reporters    []model.Reporter
	reporterCode map[string]string
	partnerCode  map[string]string
}

type referenceEntry struct {
	Code        string
	ISO3        string
	Name        string
	IsReporter  bool
	HasReporter bool
	IsPartner   bool
	HasPartner  bool
	IsGroup     bool
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
		cfg.BaseURL = defaultBaseURL
	}
	if strings.TrimSpace(cfg.DataPath) == "" {
		cfg.DataPath = defaultDataPath
	}
	if strings.TrimSpace(cfg.ReportersURL) == "" {
		cfg.ReportersURL = defaultReportersURL
	}
	if strings.TrimSpace(cfg.PartnersURL) == "" {
		cfg.PartnersURL = defaultPartnersURL
	}
	if strings.TrimSpace(cfg.APIKeyParam) == "" {
		cfg.APIKeyParam = defaultAPIKeyParam
	}
	if strings.TrimSpace(cfg.Type) == "" {
		cfg.Type = defaultType
	}
	if strings.TrimSpace(cfg.Frequency) == "" {
		cfg.Frequency = defaultFrequency
	}
	if strings.TrimSpace(cfg.Classification) == "" {
		cfg.Classification = defaultClassification
	}
	if strings.TrimSpace(cfg.Commodity) == "" {
		cfg.Commodity = defaultCommodity
	}
	if strings.TrimSpace(cfg.FlowExport) == "" {
		cfg.FlowExport = defaultFlowExport
	}
	if strings.TrimSpace(cfg.FlowImport) == "" {
		cfg.FlowImport = defaultFlowImport
	}
	if strings.TrimSpace(cfg.Format) == "" {
		cfg.Format = defaultFormat
	}
	if cfg.MaxRecords <= 0 {
		cfg.MaxRecords = defaultMaxRecords
	}
	if cfg.LookbackYears <= 0 {
		cfg.LookbackYears = defaultLookbackYears
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeoutSeconds * time.Second
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUserAgent
	}
	if cfg.ValueMultiplier == 0 {
		cfg.ValueMultiplier = defaultValueMultiplier
	}
	if cfg.RateLimitPerSec <= 0 {
		cfg.RateLimitPerSec = defaultRateLimitPerSec
	}
	if cfg.RateLimitBurst <= 0 {
		cfg.RateLimitBurst = defaultRateLimitBurst
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaultMaxRetries
	}

	return &Provider{
		config:       cfg,
		client:       &http.Client{Timeout: cfg.Timeout},
		limiter:      newRateLimiter(cfg.RateLimitPerSec, cfg.RateLimitBurst),
		reporterCode: make(map[string]string),
		partnerCode:  make(map[string]string),
	}, nil
}

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		BaseURL:           getenv("COMTRADE_BASE_URL", defaultBaseURL),
		DataPath:          getenv("COMTRADE_DATA_PATH", defaultDataPath),
		Dataset:           strings.TrimSpace(os.Getenv("COMTRADE_DATASET")),
		ReportersURL:      getenv("COMTRADE_REPORTERS_URL", defaultReportersURL),
		PartnersURL:       getenv("COMTRADE_PARTNERS_URL", defaultPartnersURL),
		APIKeyPrimary:     strings.TrimSpace(os.Getenv("COMTRADE_PRIMARY_KEY")),
		APIKeySecondary:   strings.TrimSpace(os.Getenv("COMTRADE_SECONDARY_KEY")),
		APIKeyParam:       getenv("COMTRADE_API_KEY_PARAM", defaultAPIKeyParam),
		Type:              getenv("COMTRADE_TYPE", defaultType),
		Frequency:         getenv("COMTRADE_FREQUENCY", defaultFrequency),
		Classification:    getenv("COMTRADE_CLASSIFICATION", defaultClassification),
		Commodity:         getenv("COMTRADE_COMMODITY", defaultCommodity),
		FlowExport:        getenv("COMTRADE_FLOW_EXPORT", defaultFlowExport),
		FlowImport:        getenv("COMTRADE_FLOW_IMPORT", defaultFlowImport),
		Format:            getenv("COMTRADE_FORMAT", defaultFormat),
		ValueMultiplier:   getenvFloat("COMTRADE_VALUE_MULTIPLIER", defaultValueMultiplier),
		AllowISO3Fallback: getenvBool("COMTRADE_ALLOW_ISO3_FALLBACK", defaultAllowISO3Fallback),
	}

	cfg.MaxRecords = getenvInt("COMTRADE_MAX_RECORDS", defaultMaxRecords)
	cfg.LookbackYears = getenvInt("COMTRADE_LOOKBACK_YEARS", defaultLookbackYears)
	cfg.Timeout = time.Duration(getenvInt("COMTRADE_TIMEOUT_SECONDS", defaultTimeoutSeconds)) * time.Second
	cfg.RateLimitPerSec = getenvInt("COMTRADE_RATE_LIMIT_PER_SEC", defaultRateLimitPerSec)
	cfg.RateLimitBurst = getenvInt("COMTRADE_RATE_LIMIT_BURST", defaultRateLimitBurst)
	cfg.MaxRetries = getenvInt("COMTRADE_MAX_RETRIES", defaultMaxRetries)

	return cfg, nil
}

func (p *Provider) Name() string {
	return "comtrade"
}

func (p *Provider) ListReporters(ctx context.Context) ([]model.Reporter, error) {
	if err := p.ensureReferences(ctx); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	copied := make([]model.Reporter, len(p.reporters))
	copy(copied, p.reporters)
	return copied, nil
}

func (p *Provider) FetchLatest(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow) (model.Observation, error) {
	series, err := p.FetchSeries(ctx, reporterISO3, partnerISO3, flow, "", "")
	if err != nil {
		return model.Observation{}, err
	}
	if len(series) == 0 {
		return model.Observation{}, ErrNoRecords
	}
	latest, ok := pickLatest(series)
	if !ok {
		return model.Observation{}, errors.New("comtrade: unable to select latest observation")
	}
	return latest, nil
}

func (p *Provider) FetchSeries(ctx context.Context, reporterISO3, partnerISO3 string, flow model.Flow, from, to string) ([]model.Observation, error) {
	refsErr := p.ensureReferences(ctx)

	reporterISO3 = strings.ToUpper(strings.TrimSpace(reporterISO3))
	partnerISO3 = strings.ToUpper(strings.TrimSpace(partnerISO3))

	reporterCode := reporterISO3
	partnerCode := partnerISO3
	if refsErr == nil {
		code, err := p.resolveReporterCode(reporterISO3)
		if err != nil {
			return nil, err
		}
		reporterCode = code

		code, err = p.resolvePartnerCode(partnerISO3)
		if err != nil {
			return nil, err
		}
		partnerCode = code
	} else if !p.config.AllowISO3Fallback {
		return nil, refsErr
	}

	years, err := buildYearRange(from, to, p.config.LookbackYears)
	if err != nil {
		return nil, err
	}

	flowCode := p.flowCode(flow)
	observations := make([]model.Observation, 0)
	for _, year := range years {
		rows, err := p.fetchYear(ctx, reporterISO3, partnerISO3, reporterCode, partnerCode, flow, flowCode, year)
		if err != nil {
			if errors.Is(err, ErrNoRecords) {
				continue
			}
			return nil, err
		}
		observations = append(observations, rows...)
	}

	if len(observations) == 0 {
		return nil, ErrNoRecords
	}
	return observations, nil
}

func (p *Provider) ensureReferences(ctx context.Context) error {
	p.mu.Lock()
	if p.refsLoaded {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	reporters, reporterCodes, err := p.fetchReferences(ctx, p.config.ReportersURL, true)
	if err != nil {
		return err
	}
	_, partnerCodes, err := p.fetchReferences(ctx, p.config.PartnersURL, false)
	if err != nil {
		return err
	}

	p.mu.Lock()
	p.reporters = reporters
	p.reporterCode = reporterCodes
	p.partnerCode = partnerCodes
	p.refsLoaded = true
	p.mu.Unlock()

	return nil
}

func (p *Provider) fetchReferences(ctx context.Context, endpoint string, filterReporter bool) ([]model.Reporter, map[string]string, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, nil, errors.New("comtrade: reference url is required")
	}

	body, err := p.doRequest(ctx, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	entries, err := parseReferenceEntries(body)
	if err != nil {
		return nil, nil, err
	}

	reporters := make([]model.Reporter, 0)
	codes := make(map[string]string)
	for _, entry := range entries {
		iso3 := strings.ToUpper(strings.TrimSpace(entry.ISO3))
		if iso3 == "" {
			continue
		}
		if entry.IsGroup {
			continue
		}
		if filterReporter && entry.HasReporter && !entry.IsReporter {
			continue
		}

		code := strings.TrimSpace(entry.Code)
		if code == "" {
			code = iso3
		}
		codes[iso3] = code

		if filterReporter {
			reporters = append(reporters, model.Reporter{
				ISO3:     iso3,
				NameEN:   strings.TrimSpace(entry.Name),
				NameKO:   "",
				Region:   "",
				IsActive: true,
			})
		}
	}

	if filterReporter && len(reporters) == 0 {
		return nil, nil, errors.New("comtrade: no reporters parsed")
	}
	return reporters, codes, nil
}

func (p *Provider) resolveReporterCode(iso3 string) (string, error) {
	return p.resolveCode("reporter", iso3, p.reporterCode)
}

func (p *Provider) resolvePartnerCode(iso3 string) (string, error) {
	return p.resolveCode("partner", iso3, p.partnerCode)
}

func (p *Provider) resolveCode(kind, iso3 string, codes map[string]string) (string, error) {
	iso3 = strings.ToUpper(strings.TrimSpace(iso3))
	if iso3 == "" {
		return "", fmt.Errorf("comtrade: %s iso3 is required", kind)
	}
	if code, ok := codes[iso3]; ok && code != "" {
		return code, nil
	}
	if p.config.AllowISO3Fallback {
		return iso3, nil
	}
	return "", fmt.Errorf("comtrade: missing %s code for %s", kind, iso3)
}

func (p *Provider) fetchYear(ctx context.Context, reporterISO3, partnerISO3, reporterCode, partnerCode string, flow model.Flow, flowCode string, year int) ([]model.Observation, error) {
	params := url.Values{}
	params.Set("reportercode", reporterCode)
	params.Set("flowCode", flowCode)
	params.Set("period", strconv.Itoa(year))
	params.Set("cmdCode", p.config.Commodity)
	params.Set("partnerCode", partnerCode)
	params.Set("format", p.config.Format)
	if p.config.MaxRecords > 0 {
		params.Set("maxRecords", strconv.Itoa(p.config.MaxRecords))
	}

	body, err := p.doRequest(ctx, p.dataURL(), params)
	if err != nil {
		return nil, err
	}

	observations, err := parseObservations(body, flow, reporterISO3, partnerISO3, p.config.ValueMultiplier)
	if err != nil {
		return nil, err
	}
	if len(observations) == 0 {
		return nil, ErrNoRecords
	}
	for i := range observations {
		observations[i].Provider = p.Name()
	}
	return observations, nil
}

func (p *Provider) dataURL() string {
	path := strings.TrimLeft(p.config.DataPath, "/")
	path = strings.ReplaceAll(path, "{type}", url.PathEscape(p.config.Type))
	path = strings.ReplaceAll(path, "{freq}", url.PathEscape(p.config.Frequency))
	path = strings.ReplaceAll(path, "{cl}", url.PathEscape(p.config.Classification))

	endpoint := strings.TrimRight(p.config.BaseURL, "/") + "/" + path
	if strings.TrimSpace(p.config.Dataset) != "" {
		endpoint = strings.TrimRight(endpoint, "/") + "/" + url.PathEscape(strings.TrimSpace(p.config.Dataset))
	}
	return endpoint
}

func (p *Provider) flowCode(flow model.Flow) string {
	switch flow {
	case model.FlowExport:
		return p.config.FlowExport
	case model.FlowImport:
		return p.config.FlowImport
	default:
		return string(flow)
	}
}

func (p *Provider) doRequest(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	keys := []string{}
	if strings.TrimSpace(p.config.APIKeyPrimary) != "" {
		keys = append(keys, p.config.APIKeyPrimary)
	}
	if strings.TrimSpace(p.config.APIKeySecondary) != "" && p.config.APIKeySecondary != p.config.APIKeyPrimary {
		keys = append(keys, p.config.APIKeySecondary)
	}
	if len(keys) == 0 {
		return nil, errors.New("comtrade: api key is required (COMTRADE_PRIMARY_KEY)")
	}

	var lastErr error
	for _, key := range keys {
		attempts := p.config.MaxRetries + 1
		if attempts < 1 {
			attempts = 1
		}
		for attempt := 0; attempt < attempts; attempt++ {
			body, status, retryAfter, err := p.doRequestWithKey(ctx, endpoint, params, key)
			if err == nil {
				return body, nil
			}
			lastErr = err
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				break
			}
			if status == http.StatusTooManyRequests {
				if attempt < attempts-1 {
					if retryAfter <= 0 {
						retryAfter = time.Second
					}
					if err := sleepWithContext(ctx, retryAfter); err != nil {
						return nil, err
					}
					continue
				}
			}
			return nil, err
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, errors.New("comtrade: request failed")
}

func (p *Provider) doRequestWithKey(ctx context.Context, endpoint string, params url.Values, apiKey string) ([]byte, int, time.Duration, error) {
	if p.limiter != nil {
		if err := p.limiter.Wait(ctx); err != nil {
			return nil, 0, 0, err
		}
	}

	uri, err := p.buildURL(endpoint, params, apiKey)
	if err != nil {
		return nil, 0, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, uri, nil)
	if err != nil {
		return nil, 0, 0, err
	}
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		req.Header.Set("Ocp-Apim-Subscription-Key", apiKey)
	}
	if p.config.UserAgent != "" {
		req.Header.Set("User-Agent", p.config.UserAgent)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, 0, err
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		retryAfter := parseRetryAfter(resp, body)
		if resp.StatusCode == http.StatusForbidden && isQuotaExceeded(body) {
			return nil, resp.StatusCode, retryAfter, fmt.Errorf("%w: %s", ErrQuotaExceeded, strings.TrimSpace(string(body)))
		}
		return nil, resp.StatusCode, retryAfter, fmt.Errorf("comtrade: request failed (%s): %s", resp.Status, strings.TrimSpace(string(body)))
	}

	return body, resp.StatusCode, 0, nil
}

func (p *Provider) buildURL(endpoint string, params url.Values, apiKey string) (string, error) {
	query := url.Values{}
	for key, values := range params {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	if strings.TrimSpace(apiKey) != "" && strings.TrimSpace(p.config.APIKeyParam) != "" {
		query.Set(p.config.APIKeyParam, apiKey)
	}
	if len(query) > 0 {
		return endpoint + "?" + query.Encode(), nil
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

func parseReferenceEntries(body []byte) ([]referenceEntry, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	rows, err := extractRows(payload)
	if err != nil {
		return nil, err
	}

	entries := make([]referenceEntry, 0, len(rows))
	for _, row := range rows {
		code, _ := getString(row, "id", "code", "reporterCode", "partnerCode", "PartnerCode", "areaCode")
		iso3, _ := getString(row, "iso3", "ISO3", "iso3Code", "iso3code", "iso3ISO", "rt3ISO", "pt3ISO", "reporterCodeIsoAlpha3", "ReporterCodeIsoAlpha3", "PartnerCodeIsoAlpha3", "partnerCodeIsoAlpha3")
		name, _ := getString(row, "text", "name", "label", "description")
		entry := referenceEntry{
			Code: strings.TrimSpace(code),
			ISO3: strings.TrimSpace(iso3),
			Name: strings.TrimSpace(name),
		}

		if value, ok := getValue(row, "isReporter", "isreporter", "reporter"); ok {
			entry.IsReporter = parseBool(value)
			entry.HasReporter = true
		}
		if value, ok := getValue(row, "isPartner", "ispartner", "partner"); ok {
			entry.IsPartner = parseBool(value)
			entry.HasPartner = true
		}
		if value, ok := getValue(row, "isGroup", "isgroup", "group"); ok {
			entry.IsGroup = parseBool(value)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func parseObservations(body []byte, fallbackFlow model.Flow, reporterISO3, partnerISO3 string, multiplier float64) ([]model.Observation, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	rows, err := extractRows(payload)
	if err != nil {
		return nil, err
	}

	observations := make([]model.Observation, 0, len(rows))
	for _, row := range rows {
		observation, err := rowToObservation(row, reporterISO3, partnerISO3, fallbackFlow, multiplier)
		if err != nil {
			continue
		}
		observations = append(observations, observation)
	}

	return observations, nil
}

func rowToObservation(row map[string]any, reporterISO3, partnerISO3 string, flow model.Flow, multiplier float64) (model.Observation, error) {
	value, ok := getFloat(row, "TradeValue", "tradeValue", "TradeValueUSD", "TradeValueUS$", "Value", "value", "primaryValue")
	if !ok {
		return model.Observation{}, errors.New("comtrade: missing trade value")
	}
	value *= multiplier

	periodType, period, ok := periodFromRow(row)
	if !ok {
		return model.Observation{}, errors.New("comtrade: missing period")
	}

	reporter := reporterISO3
	if value, ok := getString(row, "rt3ISO", "ReporterISO3", "reporterISO3", "Reporter", "reporter"); ok {
		reporter = value
	}
	partner := partnerISO3
	if value, ok := getString(row, "pt3ISO", "PartnerISO3", "partnerISO3", "Partner", "partner"); ok {
		partner = value
	}

	return model.Observation{
		ReporterISO3: strings.ToUpper(strings.TrimSpace(reporter)),
		PartnerISO3:  strings.ToUpper(strings.TrimSpace(partner)),
		Flow:         flow,
		PeriodType:   periodType,
		Period:       period,
		ValueUSD:     value,
	}, nil
}

func periodFromRow(row map[string]any) (model.PeriodType, string, bool) {
	if value, ok := getString(row, "Period", "period", "Time", "time"); ok {
		if periodType, period, ok := normalizePeriod(value); ok {
			return periodType, period, ok
		}
	}

	if value, ok := getString(row, "yr", "year", "Year"); ok {
		if year, ok := parseYear(value); ok {
			return model.PeriodYear, fmt.Sprintf("%04d", year), true
		}
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

func extractRows(payload any) ([]map[string]any, error) {
	switch typed := payload.(type) {
	case []any:
		return toRowList(typed), nil
	case map[string]any:
		for _, key := range []string{"dataset", "Dataset", "data", "Data", "results", "Results", "value", "Value", "items", "Items"} {
			if raw, ok := typed[key]; ok {
				return extractRows(raw)
			}
		}
		return nil, errors.New("comtrade: unexpected response shape")
	default:
		return nil, errors.New("comtrade: unexpected response type")
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

func parseBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "y":
			return true
		default:
			return false
		}
	case json.Number:
		return typed.String() != "0"
	case float64:
		return typed != 0
	case float32:
		return typed != 0
	case int:
		return typed != 0
	case int64:
		return typed != 0
	default:
		return false
	}
}

func parseRetryAfter(resp *http.Response, body []byte) time.Duration {
	if resp != nil {
		if value := strings.TrimSpace(resp.Header.Get("Retry-After")); value != "" {
			if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
			if when, err := time.Parse(http.TimeFormat, value); err == nil {
				wait := time.Until(when)
				if wait > 0 {
					return wait
				}
			}
		}
	}

	if len(body) == 0 {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0
	}
	message, _ := payload["message"].(string)
	seconds := parseRetrySeconds(message)
	if seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func isQuotaExceeded(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err == nil {
		if message, ok := payload["message"].(string); ok {
			return strings.Contains(strings.ToLower(message), "quota")
		}
	}
	return strings.Contains(strings.ToLower(string(body)), "quota")
}

func parseRetrySeconds(message string) int {
	msg := strings.ToLower(message)
	marker := "try again in"
	idx := strings.Index(msg, marker)
	if idx == -1 {
		return 0
	}
	fragment := msg[idx+len(marker):]
	for _, part := range strings.Fields(fragment) {
		if value, err := strconv.Atoi(part); err == nil && value > 0 {
			return value
		}
	}
	return 0
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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

func buildYearRange(from, to string, lookback int) ([]int, error) {
	current := time.Now().UTC().Year()

	if from == "" && to == "" {
		start := current - lookback
		if start < 0 {
			start = 0
		}
		return yearsBetween(start, current), nil
	}

	if from == "" {
		from = to
	}
	if to == "" {
		to = from
	}
	start, ok := parseYear(from)
	if !ok {
		return nil, fmt.Errorf("comtrade: invalid from year %q", from)
	}
	end, ok := parseYear(to)
	if !ok {
		return nil, fmt.Errorf("comtrade: invalid to year %q", to)
	}
	if start > end {
		start, end = end, start
	}
	return yearsBetween(start, end), nil
}

func yearsBetween(start, end int) []int {
	count := end - start + 1
	if count <= 0 {
		return []int{}
	}
	years := make([]int, 0, count)
	for year := start; year <= end; year++ {
		years = append(years, year)
	}
	return years
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
