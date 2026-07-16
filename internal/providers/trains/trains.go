package trains

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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"tradegravity/internal/model"
	"tradegravity/internal/providers"
)

const (
	defaultBaseURL          = "https://wits.worldbank.org/API/V1/"
	defaultCountriesPath    = "wits/datasource/trn/country/ALL"
	defaultAvailabilityPath = "wits/datasource/trn/dataavailability/country/{reporter}/year/all"
	defaultDataPath         = "SDMX/V21/rest/data/DF_WITS_Tariff_TRAINS/A.{reporter}.{partner}.{products}.{datatype}/"
	defaultTimeout          = 45 * time.Second
	defaultUserAgent        = "TradeGravity/0.1"
	defaultRetries          = 2
	defaultBackoff          = 500 * time.Millisecond
	maxProductCodes         = 50
	maxProductsPerRequest   = 20
	sdmxJSONAccept          = "application/vnd.sdmx.data+json;version=1.0.0-wd"
)

var (
	ErrNoRecords          = errors.New("trains: no records found")
	ErrRateLimited        = errors.New("trains: rate limited")
	ErrPartnerUnavailable = errors.New("trains: partner unavailable")
	ErrAVEUnavailable     = errors.New("trains: AVE estimates unavailable")
)

type Config struct {
	BaseURL          string
	CountriesPath    string
	AvailabilityPath string
	DataPath         string
	Timeout          time.Duration
	UserAgent        string
	Retries          int
	Backoff          time.Duration
	Client           *http.Client
}

type Provider struct {
	config Config
	client *http.Client

	mu           sync.Mutex
	countries    []country
	byISO3       map[string]country
	availability map[string][]availabilityEntry
}

type country struct {
	Code       string
	ISO3       string
	Name       string
	IsReporter bool
	IsPartner  bool
	IsGroup    bool
}

type availabilityEntry struct {
	Year         string
	Nomenclature string
	PartnerCodes map[string]struct{}
	AVEAvailable bool
	UpdatedAt    time.Time
}

func New() (*Provider, error) {
	return NewWithConfig(ConfigFromEnv())
}

func ConfigFromEnv() Config {
	return Config{
		BaseURL:          env("TRAINS_BASE_URL", defaultBaseURL),
		CountriesPath:    env("TRAINS_COUNTRIES_PATH", defaultCountriesPath),
		AvailabilityPath: env("TRAINS_AVAILABILITY_PATH", defaultAvailabilityPath),
		DataPath:         env("TRAINS_DATA_PATH", defaultDataPath),
		Timeout:          time.Duration(envInt("TRAINS_TIMEOUT_SECONDS", int(defaultTimeout/time.Second))) * time.Second,
		UserAgent:        env("TRAINS_USER_AGENT", defaultUserAgent),
		Retries:          envInt("TRAINS_RETRIES", defaultRetries),
		Backoff:          time.Duration(envInt("TRAINS_BACKOFF_MILLISECONDS", int(defaultBackoff/time.Millisecond))) * time.Millisecond,
	}
}

func NewWithConfig(config Config) (*Provider, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, errors.New("trains base URL is required")
	}
	config.BaseURL = strings.TrimRight(config.BaseURL, "/") + "/"
	if config.CountriesPath == "" {
		config.CountriesPath = defaultCountriesPath
	}
	if config.AvailabilityPath == "" {
		config.AvailabilityPath = defaultAvailabilityPath
	}
	if config.DataPath == "" {
		config.DataPath = defaultDataPath
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultTimeout
	}
	if config.UserAgent == "" {
		config.UserAgent = defaultUserAgent
	}
	if config.Retries < 0 {
		config.Retries = 0
	}
	if config.Backoff <= 0 {
		config.Backoff = defaultBackoff
	}
	client := config.Client
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	return &Provider{
		config:       config,
		client:       client,
		byISO3:       make(map[string]country),
		availability: make(map[string][]availabilityEntry),
	}, nil
}

func (p *Provider) Name() string { return "trains" }

func (p *Provider) ListTariffImporters(ctx context.Context) ([]model.Reporter, error) {
	if err := p.ensureCountries(ctx); err != nil {
		return nil, err
	}
	p.mu.Lock()
	countries := append([]country(nil), p.countries...)
	p.mu.Unlock()
	reporters := make([]model.Reporter, 0, len(countries))
	for _, item := range countries {
		if !item.IsReporter || item.IsGroup || !isISO3(item.ISO3) {
			continue
		}
		reporters = append(reporters, model.Reporter{ISO3: item.ISO3, NameEN: item.Name, IsActive: true})
	}
	sort.Slice(reporters, func(i, j int) bool { return reporters[i].ISO3 < reporters[j].ISO3 })
	if len(reporters) == 0 {
		return nil, errors.New("trains: no tariff importers parsed")
	}
	return reporters, nil
}

func (p *Provider) LatestTariffYear(ctx context.Context, importerISO3 string) (string, error) {
	entries, err := p.availabilityFor(ctx, importerISO3)
	if err != nil {
		return "", err
	}
	latest := ""
	for _, entry := range entries {
		if validYear(entry.Year) && entry.Year > latest {
			latest = entry.Year
		}
	}
	if latest == "" {
		return "", ErrNoRecords
	}
	return latest, nil
}

func (p *Provider) FetchTariffs(ctx context.Context, importerISO3, exporterISO3, year string, codes []string, dataType model.TariffDataType) ([]model.TariffObservation, error) {
	importerISO3 = strings.ToUpper(strings.TrimSpace(importerISO3))
	exporterISO3 = strings.ToUpper(strings.TrimSpace(exporterISO3))
	if !isISO3(importerISO3) || !isISO3(exporterISO3) {
		return nil, errors.New("trains: importer and exporter must be ISO3 codes")
	}
	if !validYear(year) {
		return nil, fmt.Errorf("trains: invalid tariff year %q", year)
	}
	normalizedCodes, err := normalizeCodes(codes)
	if err != nil {
		return nil, err
	}
	dataTypePath, err := dataTypeSegment(dataType)
	if err != nil {
		return nil, err
	}
	if err := p.ensureCountries(ctx); err != nil {
		return nil, err
	}
	importer, ok := p.countryForISO3(importerISO3)
	if !ok || !importer.IsReporter {
		return nil, fmt.Errorf("trains: unknown importer %s", importerISO3)
	}
	exporter, ok := p.countryForISO3(exporterISO3)
	if !ok || !exporter.IsPartner {
		return nil, fmt.Errorf("trains: unknown exporter %s", exporterISO3)
	}
	entries, err := p.availabilityFor(ctx, importerISO3)
	if err != nil {
		return nil, err
	}
	available, ok := availabilityForYear(entries, year)
	if !ok {
		return nil, fmt.Errorf("%w: importer=%s year=%s", ErrNoRecords, importerISO3, year)
	}
	if _, ok := available.PartnerCodes[exporter.Code]; !ok {
		return nil, fmt.Errorf("%w: importer=%s exporter=%s year=%s", ErrPartnerUnavailable, importerISO3, exporterISO3, year)
	}
	if dataType == model.TariffAVEEstimated && !available.AVEAvailable {
		return nil, fmt.Errorf("%w for importer=%s year=%s", ErrAVEUnavailable, importerISO3, year)
	}

	query := url.Values{"startperiod": {year}, "endperiod": {year}}
	observations := make([]model.TariffObservation, 0)
	for start := 0; start < len(normalizedCodes); start += maxProductsPerRequest {
		end := min(start+maxProductsPerRequest, len(normalizedCodes))
		path := p.config.DataPath
		replacements := map[string]string{
			"{reporter}": importer.Code,
			"{partner}":  exporter.Code,
			"{products}": strings.Join(normalizedCodes[start:end], "+"),
			"{datatype}": dataTypePath,
		}
		for placeholder, value := range replacements {
			path = strings.ReplaceAll(path, placeholder, value)
		}
		body, requestErr := p.doRequest(ctx, path, query, sdmxJSONAccept)
		if errors.Is(requestErr, ErrNoRecords) {
			continue
		}
		if requestErr != nil {
			return nil, requestErr
		}
		var payload sdmxResponse
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			return nil, fmt.Errorf("trains: decode tariff response: %w", err)
		}
		rows, parseErr := parseTariffs(payload, importerISO3, exporterISO3, exporter.Code, dataType, available.UpdatedAt)
		if parseErr != nil {
			return nil, parseErr
		}
		observations = append(observations, rows...)
	}
	if len(observations) == 0 {
		return nil, ErrNoRecords
	}
	return observations, nil
}

func (p *Provider) ensureCountries(ctx context.Context) error {
	p.mu.Lock()
	loaded := len(p.countries) > 0
	p.mu.Unlock()
	if loaded {
		return nil
	}
	body, err := p.doRequest(ctx, p.config.CountriesPath, nil, "application/xml")
	if err != nil {
		return err
	}
	parsed, err := parseCountries(body)
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.countries) == 0 {
		p.countries = parsed
		for _, item := range parsed {
			p.byISO3[item.ISO3] = item
		}
		// WITS uses partner code 000 for the world/MFN schedule, but does not
		// consistently include that aggregate in the country metadata response.
		if _, ok := p.byISO3["WLD"]; !ok {
			p.byISO3["WLD"] = country{Code: "000", ISO3: "WLD", Name: "World", IsPartner: true, IsGroup: true}
		}
	}
	return nil
}

func (p *Provider) countryForISO3(iso3 string) (country, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.byISO3[strings.ToUpper(strings.TrimSpace(iso3))]
	return item, ok
}

func (p *Provider) availabilityFor(ctx context.Context, importerISO3 string) ([]availabilityEntry, error) {
	importerISO3 = strings.ToUpper(strings.TrimSpace(importerISO3))
	if err := p.ensureCountries(ctx); err != nil {
		return nil, err
	}
	importer, ok := p.countryForISO3(importerISO3)
	if !ok || !importer.IsReporter {
		return nil, fmt.Errorf("trains: unknown importer %s", importerISO3)
	}
	p.mu.Lock()
	if cached, ok := p.availability[importerISO3]; ok {
		result := append([]availabilityEntry(nil), cached...)
		p.mu.Unlock()
		return result, nil
	}
	p.mu.Unlock()
	path := strings.ReplaceAll(p.config.AvailabilityPath, "{reporter}", importer.Code)
	body, err := p.doRequest(ctx, path, nil, "application/xml")
	if err != nil {
		return nil, err
	}
	entries, err := parseAvailability(body)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, ErrNoRecords
	}
	p.mu.Lock()
	p.availability[importerISO3] = append([]availabilityEntry(nil), entries...)
	p.mu.Unlock()
	return entries, nil
}

func (p *Provider) doRequest(ctx context.Context, path string, query url.Values, accept string) ([]byte, error) {
	endpoint := strings.TrimRight(p.config.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	var lastErr error
	for attempt := 0; attempt <= p.config.Retries; attempt++ {
		if attempt > 0 {
			delay := p.config.Backoff * time.Duration(1<<(attempt-1))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", accept)
		req.Header.Set("User-Agent", p.config.UserAgent)
		response, err := p.client.Do(req)
		if err != nil {
			lastErr = safeTransportError(err)
			if attempt < p.config.Retries && retryableTransport(err) {
				continue
			}
			return nil, lastErr
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 20<<20))
		response.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("trains: read response: %w", readErr)
		}
		if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
			if len(bytes.TrimSpace(body)) == 0 {
				lastErr = errors.New("trains: empty response")
				if attempt < p.config.Retries {
					continue
				}
				return nil, lastErr
			}
			return body, nil
		}
		message := strings.TrimSpace(string(body))
		if len(message) > 500 {
			message = message[:500]
		}
		if response.StatusCode == http.StatusNotFound || strings.Contains(strings.ToLower(message), "data not found") || strings.Contains(strings.ToLower(message), "norecord") {
			return nil, ErrNoRecords
		}
		if response.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("%w: HTTP %d", ErrRateLimited, response.StatusCode)
		} else {
			lastErr = fmt.Errorf("trains: request failed (HTTP %d): %s", response.StatusCode, message)
		}
		if attempt < p.config.Retries && retryableStatus(response.StatusCode) {
			continue
		}
		return nil, lastErr
	}
	return nil, lastErr
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func retryableTransport(err error) bool {
	var netErr interface{ Timeout() bool }
	return errors.As(err, &netErr) && netErr.Timeout()
}

func safeTransportError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return fmt.Errorf("trains: request failed: %w", urlErr.Err)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("trains: request failed: %w", err)
	}
	return errors.New("trains: request failed")
}

type countryXML struct {
	Countries []struct {
		Code       string `xml:"countrycode,attr"`
		IsReporter string `xml:"isreporter,attr"`
		IsPartner  string `xml:"ispartner,attr"`
		IsGroup    string `xml:"isgroup,attr"`
		ISO3       string `xml:"iso3Code"`
		Name       string `xml:"name"`
	} `xml:"countries>country"`
}

func parseCountries(body []byte) ([]country, error) {
	var payload countryXML
	if err := xml.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("trains: decode countries: %w", err)
	}
	result := make([]country, 0, len(payload.Countries))
	for _, item := range payload.Countries {
		iso3 := strings.ToUpper(strings.TrimSpace(item.ISO3))
		code := strings.ToUpper(strings.TrimSpace(item.Code))
		if !isISO3(iso3) || len(code) != 3 {
			continue
		}
		result = append(result, country{
			Code: code, ISO3: iso3, Name: strings.TrimSpace(item.Name),
			IsReporter: item.IsReporter == "1", IsPartner: item.IsPartner == "1",
			IsGroup: strings.EqualFold(strings.TrimSpace(item.IsGroup), "yes"),
		})
	}
	if len(result) == 0 {
		return nil, errors.New("trains: empty country metadata")
	}
	return result, nil
}

type availabilityXML struct {
	Reporters []struct {
		Year         string `xml:"year"`
		Nomenclature struct {
			Code string `xml:"reporternernomenclaturecode,attr"`
		} `xml:"reporternernomenclature"`
		Partners     string `xml:"partnerlist"`
		AVEAvailable string `xml:"isspecificdutyexpressionestimatedavailable"`
		Updated      string `xml:"lastupdateddate"`
	} `xml:"dataavailability>reporter"`
}

func parseAvailability(body []byte) ([]availabilityEntry, error) {
	var payload availabilityXML
	if err := xml.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("trains: decode availability: %w", err)
	}
	entries := make([]availabilityEntry, 0, len(payload.Reporters))
	for _, item := range payload.Reporters {
		if !validYear(strings.TrimSpace(item.Year)) {
			continue
		}
		partners := make(map[string]struct{})
		for _, partner := range strings.Split(item.Partners, ";") {
			partner = strings.ToUpper(strings.TrimSpace(partner))
			if partner != "" {
				partners[partner] = struct{}{}
			}
		}
		updated, _ := time.Parse("2006/01/02", strings.TrimSpace(item.Updated))
		entries = append(entries, availabilityEntry{
			Year: strings.TrimSpace(item.Year), Nomenclature: strings.ToUpper(strings.TrimSpace(item.Nomenclature.Code)),
			PartnerCodes: partners, AVEAvailable: strings.EqualFold(strings.TrimSpace(item.AVEAvailable), "yes"), UpdatedAt: updated.UTC(),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Year < entries[j].Year })
	return entries, nil
}

func availabilityForYear(entries []availabilityEntry, year string) (availabilityEntry, bool) {
	for _, entry := range entries {
		if entry.Year == year {
			return entry, true
		}
	}
	return availabilityEntry{}, false
}

type sdmxResponse struct {
	DataSets []struct {
		Series map[string]struct {
			Observations map[string][]any `json:"observations"`
		} `json:"series"`
	} `json:"dataSets"`
	Structure struct {
		Dimensions struct {
			Series      []sdmxDimension `json:"series"`
			Observation []sdmxDimension `json:"observation"`
		} `json:"dimensions"`
		Attributes struct {
			Observation []sdmxDimension `json:"observation"`
		} `json:"attributes"`
	} `json:"structure"`
}

type sdmxDimension struct {
	ID     string      `json:"id"`
	Values []sdmxValue `json:"values"`
}

type sdmxValue struct {
	ID string `json:"id"`
}

func parseTariffs(payload sdmxResponse, importerISO3, exporterISO3, exporterCode string, dataType model.TariffDataType, updatedAt time.Time) ([]model.TariffObservation, error) {
	if len(payload.DataSets) == 0 {
		return nil, errors.New("trains: missing dataset")
	}
	seriesDims := payload.Structure.Dimensions.Series
	timeDims := payload.Structure.Dimensions.Observation
	if len(seriesDims) == 0 || len(timeDims) == 0 {
		return nil, errors.New("trains: missing SDMX dimensions")
	}
	positions := seriesDimensionPositions(seriesDims, payload.DataSets[0].Series)
	result := make([]model.TariffObservation, 0)
	for seriesKey, series := range payload.DataSets[0].Series {
		indices, ok := parseKey(seriesKey, len(seriesDims))
		if !ok {
			continue
		}
		dimensions := make(map[string]string)
		for index, dimension := range seriesDims {
			position := positions[index]
			if position < 0 || position >= len(indices) || indices[position] < 0 || indices[position] >= len(dimension.Values) {
				continue
			}
			dimensions[strings.ToUpper(dimension.ID)] = dimension.Values[indices[position]].ID
		}
		product := dimensions["PRODUCTCODE"]
		if !isHS6(product) {
			continue
		}
		for observationKey, values := range series.Observations {
			if len(values) == 0 {
				continue
			}
			timeIndex, err := strconv.Atoi(observationKey)
			if err != nil || timeIndex < 0 || timeIndex >= len(timeDims[0].Values) {
				continue
			}
			year := timeDims[0].Values[timeIndex].ID
			rate, ok := number(values[0])
			if !ok || rate < 0 {
				continue
			}
			attrs := observationAttributes(payload.Structure.Attributes.Observation, values[1:])
			tariffType := strings.ToUpper(attrs["TARIFFTYPE"])
			rateType := model.TariffEffectivelyApplied
			switch tariffType {
			case "MFN":
				rateType = model.TariffMFNApplied
			case "PREF":
				rateType = model.TariffPreferential
			}
			nomenclature := strings.ToUpper(attrs["NOMENCODE"])
			observation := model.TariffObservation{
				Provider: "trains", Classification: classificationForNomenclature(nomenclature), ProductCode: product, ProductLevel: 6,
				ImporterISO3: importerISO3, ExporterISO3: exporterISO3, ExporterCode: exporterCode,
				DataType: dataType, RateType: rateType, Regime: strings.ToLower(tariffType), Year: year, RatePercent: rate,
				Nomenclature: nomenclature, ExcludedFrom: attrs["EXCLUDEDFROM"], SourceUpdatedAt: updatedAt,
			}
			observation.SumRatePercent = optionalFloat(attrs["SUM_OF_RATES"])
			observation.MinRatePercent = optionalFloat(attrs["MIN_RATE"])
			observation.MaxRatePercent = optionalFloat(attrs["MAX_RATE"])
			observation.TotalLines = integer(attrs["TOTALNOOFLINES"])
			observation.PreferentialLines = integer(attrs["NBR_PREF_LINES"])
			observation.MFNLines = integer(attrs["NBR_MFN_LINES"])
			observation.NonAdValoremLines = integer(attrs["NBR_NA_LINES"])
			result = append(result, observation)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ProductCode != result[j].ProductCode {
			return result[i].ProductCode < result[j].ProductCode
		}
		return result[i].RateType < result[j].RateType
	})
	return result, nil
}

// The WITS TRAINS JSON response currently declares PARTNER before PRODUCTCODE
// but emits their series-key indexes in the opposite order for multi-product
// requests. Detect the inconsistency from index bounds and apply the narrow
// swap only when it makes every returned key valid.
func seriesDimensionPositions(dimensions []sdmxDimension, series map[string]struct {
	Observations map[string][]any `json:"observations"`
}) []int {
	positions := make([]int, len(dimensions))
	for index := range positions {
		positions[index] = index
	}
	if seriesPositionsValid(dimensions, positions, series) {
		return positions
	}
	partnerIndex, productIndex := -1, -1
	for index, dimension := range dimensions {
		switch strings.ToUpper(dimension.ID) {
		case "PARTNER":
			partnerIndex = index
		case "PRODUCTCODE":
			productIndex = index
		}
	}
	if partnerIndex >= 0 && productIndex >= 0 {
		positions[partnerIndex], positions[productIndex] = positions[productIndex], positions[partnerIndex]
		if seriesPositionsValid(dimensions, positions, series) {
			return positions
		}
		positions[partnerIndex], positions[productIndex] = positions[productIndex], positions[partnerIndex]
	}
	return positions
}

func seriesPositionsValid(dimensions []sdmxDimension, positions []int, series map[string]struct {
	Observations map[string][]any `json:"observations"`
}) bool {
	for key := range series {
		indices, ok := parseKey(key, len(dimensions))
		if !ok {
			return false
		}
		for dimensionIndex, dimension := range dimensions {
			position := positions[dimensionIndex]
			if position < 0 || position >= len(indices) || indices[position] < 0 || indices[position] >= len(dimension.Values) {
				return false
			}
		}
	}
	return true
}

func observationAttributes(dimensions []sdmxDimension, raw []any) map[string]string {
	result := make(map[string]string)
	for index, dimension := range dimensions {
		if index >= len(raw) || raw[index] == nil {
			continue
		}
		valueIndex, ok := integerValue(raw[index])
		if !ok || valueIndex < 0 || valueIndex >= len(dimension.Values) {
			continue
		}
		result[strings.ToUpper(dimension.ID)] = dimension.Values[valueIndex].ID
	}
	return result
}

func parseKey(value string, want int) ([]int, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != want {
		return nil, false
	}
	result := make([]int, len(parts))
	for index, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return nil, false
		}
		result[index] = parsed
	}
	return result, true
}

func classificationForNomenclature(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "H0":
		return "HS1988/92"
	case "H1":
		return "HS1996"
	case "H2":
		return "HS2002"
	case "H3":
		return "HS2007"
	case "H4":
		return "HS2012"
	case "H5":
		return "HS2017"
	case "H6":
		return "HS2022"
	default:
		return "HS-" + strings.ToUpper(strings.TrimSpace(value))
	}
}

func dataTypeSegment(value model.TariffDataType) (string, error) {
	switch value {
	case model.TariffReported:
		return "reported", nil
	case model.TariffAVEEstimated:
		return "aveestimated", nil
	default:
		return "", fmt.Errorf("trains: unsupported tariff data type %q", value)
	}
}

func normalizeCodes(codes []string) ([]string, error) {
	if len(codes) == 0 {
		return nil, errors.New("trains: at least one HS6 product code is required")
	}
	if len(codes) > maxProductCodes {
		return nil, fmt.Errorf("trains: at most %d product codes may be requested", maxProductCodes)
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if !isHS6(code) {
			return nil, fmt.Errorf("trains: invalid HS6 product code %q", code)
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	sort.Strings(result)
	return result, nil
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case float64:
		return typed, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func integerValue(value any) (int, bool) {
	number, ok := number(value)
	if !ok || number != float64(int(number)) {
		return 0, false
	}
	return int(number), true
}

func optionalFloat(value string) *float64 {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return nil
	}
	return &parsed
}

func integer(value string) int {
	parsed, _ := strconv.Atoi(strings.TrimSpace(value))
	return parsed
}

func isISO3(value string) bool {
	if len(value) != 3 {
		return false
	}
	for _, char := range value {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}

func isHS6(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func validYear(value string) bool {
	if len(value) != 4 {
		return false
	}
	_, err := strconv.Atoi(value)
	return err == nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
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

var _ providers.TariffProvider = (*Provider)(nil)
