package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultWorldBankURL = "https://api.worldbank.org/v2"

type countryConfig struct {
	ISO3   string
	ISO2   string
	Name   string
	Groups []string
}

type indicatorValue struct {
	Value *float64 `json:"value"`
	Year  string   `json:"year"`
}

type countryContext struct {
	ISO3        string         `json:"iso3"`
	ISO2        string         `json:"iso2"`
	Name        string         `json:"name"`
	Region      string         `json:"region"`
	IncomeGroup string         `json:"income_group"`
	Groups      []string       `json:"groups"`
	Population  indicatorValue `json:"population"`
	GDP         indicatorValue `json:"gdp"`
}

type contextFile struct {
	SchemaVersion string           `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Source        string           `json:"source"`
	Status        string           `json:"status"`
	Errors        []string         `json:"errors,omitempty"`
	Countries     []countryContext `json:"countries"`
}

type wbCountry struct {
	ID          string  `json:"id"`
	ISO2Code    string  `json:"iso2Code"`
	Name        string  `json:"name"`
	Region      wbLabel `json:"region"`
	IncomeLevel wbLabel `json:"incomeLevel"`
}

type wbLabel struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type wbIndicator struct {
	CountryISO3 string   `json:"countryiso3code"`
	Date        string   `json:"date"`
	Value       *float64 `json:"value"`
}

func main() {
	countriesPath := flag.String("countries", "configs/countries.csv", "country metadata CSV")
	outPath := flag.String("out", "site/data/context.json", "output JSON path")
	baseURL := flag.String("base-url", defaultWorldBankURL, "World Bank API base URL")
	timeout := flag.Duration("timeout", 2*time.Minute, "overall HTTP request timeout")
	flag.Parse()

	if err := run(*countriesPath, *outPath, *baseURL, *timeout); err != nil {
		fmt.Fprintln(os.Stderr, "context build failed:", err)
		os.Exit(1)
	}
}

func run(countriesPath, outPath, baseURL string, timeout time.Duration) error {
	configs, err := loadCountries(countriesPath)
	if err != nil {
		return err
	}
	if timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}

	output := contextFile{
		SchemaVersion: "1.0",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Source:        "World Bank Open Data",
		Status:        "success",
		Countries:     make([]countryContext, 0, len(configs)),
	}
	byISO := make(map[string]*countryContext, len(configs))
	iso2Codes := make([]string, 0, len(configs))
	for _, config := range configs {
		entry := countryContext{
			ISO3: config.ISO3, ISO2: config.ISO2, Name: config.Name,
			Groups: append([]string{}, config.Groups...),
		}
		output.Countries = append(output.Countries, entry)
		byISO[config.ISO3] = &output.Countries[len(output.Countries)-1]
		iso2Codes = append(iso2Codes, config.ISO2)
	}

	if rows, fetchErr := fetchCountries(ctx, client, baseURL, iso2Codes); fetchErr != nil {
		output.Status = "partial"
		output.Errors = append(output.Errors, "country metadata: "+fetchErr.Error())
	} else {
		applyCountryMetadata(byISO, rows)
	}
	for _, indicator := range []struct {
		ID    string
		Apply func(*countryContext, indicatorValue)
	}{
		{ID: "SP.POP.TOTL", Apply: func(country *countryContext, value indicatorValue) { country.Population = value }},
		{ID: "NY.GDP.MKTP.CD", Apply: func(country *countryContext, value indicatorValue) { country.GDP = value }},
	} {
		rows, fetchErr := fetchIndicator(ctx, client, baseURL, iso2Codes, indicator.ID)
		if fetchErr != nil {
			output.Status = "partial"
			output.Errors = append(output.Errors, indicator.ID+": "+fetchErr.Error())
			continue
		}
		for iso3, value := range latestIndicatorValues(rows) {
			if country := byISO[iso3]; country != nil {
				indicator.Apply(country, value)
			}
		}
	}

	sort.Slice(output.Countries, func(i, j int) bool { return output.Countries[i].ISO3 < output.Countries[j].ISO3 })
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	file, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		return err
	}
	fmt.Printf("context build complete (countries=%d status=%s out=%s)\n", len(output.Countries), output.Status, outPath)
	return nil
}

func loadCountries(path string) ([]countryConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, errors.New("country metadata CSV is empty")
	}
	header := make(map[string]int)
	for index, value := range records[0] {
		header[strings.ToLower(strings.TrimSpace(value))] = index
	}
	cell := func(record []string, name string) string {
		index, ok := header[name]
		if !ok || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	seen := make(map[string]struct{})
	items := make([]countryConfig, 0, len(records)-1)
	for _, record := range records[1:] {
		iso3 := strings.ToUpper(cell(record, "iso3"))
		iso2 := strings.ToUpper(cell(record, "iso2"))
		name := cell(record, "name")
		if len(iso3) != 3 || len(iso2) != 2 || name == "" {
			return nil, fmt.Errorf("invalid country metadata row: %v", record)
		}
		if _, exists := seen[iso3]; exists {
			return nil, fmt.Errorf("duplicate country %s", iso3)
		}
		seen[iso3] = struct{}{}
		items = append(items, countryConfig{ISO3: iso3, ISO2: iso2, Name: name, Groups: splitGroups(cell(record, "groups"))})
	}
	return items, nil
}

func splitGroups(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool { return r == ';' || r == '|' })
	groups := make([]string, 0, len(parts))
	for _, part := range parts {
		if group := strings.ToUpper(strings.TrimSpace(part)); group != "" {
			groups = append(groups, group)
		}
	}
	sort.Strings(groups)
	return groups
}

func fetchCountries(ctx context.Context, client *http.Client, baseURL string, codes []string) ([]wbCountry, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/country/" + strings.Join(codes, ";") + "?format=json&per_page=1000"
	var rows []wbCountry
	if err := fetchWorldBank(ctx, client, endpoint, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func fetchIndicator(ctx context.Context, client *http.Client, baseURL string, codes []string, indicator string) ([]wbIndicator, error) {
	currentYear := time.Now().UTC().Year()
	type result struct {
		rows []wbIndicator
		err  error
	}
	chunks := chunkStrings(codes, 10)
	results := make(chan result, len(chunks))
	semaphore := make(chan struct{}, 4)
	var wait sync.WaitGroup
	for _, chunk := range chunks {
		chunk := append([]string(nil), chunk...)
		wait.Add(1)
		go func() {
			defer wait.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				results <- result{err: ctx.Err()}
				return
			}
			query := url.Values{}
			query.Set("format", "json")
			query.Set("per_page", "5000")
			query.Set("date", strconv.Itoa(currentYear-12)+":"+strconv.Itoa(currentYear))
			endpoint := strings.TrimRight(baseURL, "/") + "/country/" + strings.Join(chunk, ";") + "/indicator/" + url.PathEscape(indicator) + "?" + query.Encode()
			var rows []wbIndicator
			err := fetchWorldBank(ctx, client, endpoint, &rows)
			results <- result{rows: rows, err: err}
		}()
	}
	wait.Wait()
	close(results)
	var rows []wbIndicator
	for item := range results {
		if item.err != nil {
			return nil, item.err
		}
		rows = append(rows, item.rows...)
	}
	return rows, nil
}

func chunkStrings(values []string, size int) [][]string {
	if size < 1 {
		size = 1
	}
	chunks := make([][]string, 0, (len(values)+size-1)/size)
	for start := 0; start < len(values); start += size {
		end := start + size
		if end > len(values) {
			end = len(values)
		}
		chunks = append(chunks, values[start:end])
	}
	return chunks
}

func fetchWorldBank(ctx context.Context, client *http.Client, endpoint string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "TradeGravity/0.2")
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("request failed: %s", response.Status)
	}
	var envelope []json.RawMessage
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope) < 2 || string(envelope[1]) == "null" {
		return errors.New("response did not include data rows")
	}
	return json.Unmarshal(envelope[1], destination)
}

func applyCountryMetadata(countries map[string]*countryContext, rows []wbCountry) {
	for _, row := range rows {
		iso3 := strings.ToUpper(strings.TrimSpace(row.ID))
		country := countries[iso3]
		if country == nil {
			continue
		}
		if value := strings.TrimSpace(row.Name); value != "" {
			country.Name = value
		}
		if value := strings.ToUpper(strings.TrimSpace(row.ISO2Code)); value != "" {
			country.ISO2 = value
		}
		country.Region = strings.TrimSpace(row.Region.Value)
		country.IncomeGroup = strings.TrimSpace(row.IncomeLevel.Value)
	}
}

func latestIndicatorValues(rows []wbIndicator) map[string]indicatorValue {
	values := make(map[string]indicatorValue)
	for _, row := range rows {
		iso3 := strings.ToUpper(strings.TrimSpace(row.CountryISO3))
		if iso3 == "" || row.Value == nil {
			continue
		}
		current, exists := values[iso3]
		if !exists || row.Date > current.Year {
			value := *row.Value
			values[iso3] = indicatorValue{Value: &value, Year: row.Date}
		}
	}
	return values
}
