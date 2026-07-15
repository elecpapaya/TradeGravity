package main

import (
	"sort"
	"strings"

	"tradegravity/internal/model"
	"tradegravity/internal/semiconductor"
	"tradegravity/internal/strategic"
)

type semiconductorMonthlyIndexFile struct {
	SchemaVersion    string                          `json:"schema_version"`
	GeneratedAt      string                          `json:"generated_at"`
	Provider         string                          `json:"provider"`
	Level            int                             `json:"level"`
	Partners         []string                        `json:"partners"`
	Reporters        []string                        `json:"reporters"`
	Periods          []string                        `json:"periods"`
	Partitions       []semiconductorMonthlyPartition `json:"partitions"`
	ObservationCount int                             `json:"observation_count"`
	Scope            string                          `json:"scope"`
}

type semiconductorMonthlyPartition struct {
	ReporterISO3 string `json:"reporter_iso3"`
	Href         string `json:"href"`
	RowCount     int    `json:"row_count"`
	PeriodCount  int    `json:"period_count"`
}

type semiconductorMonthlyFile struct {
	SchemaVersion string                             `json:"schema_version"`
	GeneratedAt   string                             `json:"generated_at"`
	Provider      string                             `json:"provider"`
	Level         int                                `json:"level"`
	Partners      []string                           `json:"partners"`
	ReporterISO3  string                             `json:"reporter_iso3"`
	Periods       []string                           `json:"periods"`
	Rows          []semiconductorMonthlyProductEntry `json:"rows"`
}

type semiconductorMonthlyProductEntry struct {
	Period         string      `json:"period"`
	Classification string      `json:"classification"`
	Code           string      `json:"code"`
	Label          string      `json:"label"`
	USA            seriesBlock `json:"usa"`
	CHN            seriesBlock `json:"chn"`
	Total          float64     `json:"total"`
	ShareCN        float64     `json:"share_cn"`
}

func buildSemiconductorMonthlyFiles(generatedAt, provider string, partners []string, observations []observationRow, products []strategic.Product, reference semiconductor.Reference) (semiconductorMonthlyIndexFile, map[string]semiconductorMonthlyFile) {
	type entryKey struct {
		period, classification, code string
	}
	wanted := make(map[string]string)
	for _, code := range semiconductor.Codes(reference) {
		wanted[code] = code
	}
	for _, product := range products {
		if _, ok := wanted[product.Code]; ok {
			wanted[product.Code] = product.Label
		}
	}
	index := semiconductorMonthlyIndexFile{
		SchemaVersion: schemaVersion, GeneratedAt: generatedAt, Provider: strings.ToLower(strings.TrimSpace(provider)), Level: 6,
		Partners: append([]string(nil), partners...), Reporters: []string{}, Periods: []string{}, Partitions: []semiconductorMonthlyPartition{},
		Scope: "Selected monthly UN Comtrade HS6 observations against USA and China; a turning-point signal, not a complete semiconductor market or physical route",
	}
	grouped := make(map[string]map[entryKey]*semiconductorMonthlyProductEntry)
	periodSet := make(map[string]struct{})
	for _, row := range observations {
		label, ok := wanted[strings.TrimSpace(row.ProductCode)]
		if !ok || row.ProductLevel != 6 || row.PeriodType != model.PeriodMonth {
			continue
		}
		reporter := strings.ToUpper(strings.TrimSpace(row.ReporterISO))
		partner := strings.ToUpper(strings.TrimSpace(row.PartnerISO))
		if !isPublishedISO3(reporter) || (partner != "USA" && partner != "CHN") || len(row.Period) != 7 || row.ValueUSD < 0 {
			continue
		}
		classification := strings.ToUpper(strings.TrimSpace(row.Classification))
		if classification == "" {
			classification = "HS"
		}
		if grouped[reporter] == nil {
			grouped[reporter] = make(map[entryKey]*semiconductorMonthlyProductEntry)
		}
		key := entryKey{period: row.Period, classification: classification, code: row.ProductCode}
		entry := grouped[reporter][key]
		if entry == nil {
			entry = &semiconductorMonthlyProductEntry{Period: row.Period, Classification: classification, Code: row.ProductCode, Label: label}
			grouped[reporter][key] = entry
		}
		block := &entry.USA
		if partner == "CHN" {
			block = &entry.CHN
		}
		block.Available = true
		if row.Flow == model.FlowExport {
			block.Export += row.ValueUSD
		} else if row.Flow == model.FlowImport {
			block.Import += row.ValueUSD
		} else {
			continue
		}
		periodSet[row.Period] = struct{}{}
		index.ObservationCount++
	}

	reporters := make([]string, 0, len(grouped))
	for reporter := range grouped {
		reporters = append(reporters, reporter)
	}
	sort.Strings(reporters)
	files := make(map[string]semiconductorMonthlyFile, len(reporters))
	for _, reporter := range reporters {
		file := semiconductorMonthlyFile{SchemaVersion: schemaVersion, GeneratedAt: generatedAt, Provider: index.Provider, Level: 6, Partners: append([]string(nil), partners...), ReporterISO3: reporter, Periods: []string{}, Rows: []semiconductorMonthlyProductEntry{}}
		filePeriods := make(map[string]struct{})
		for _, entry := range grouped[reporter] {
			entry.USA.Trade = entry.USA.Export + entry.USA.Import
			entry.CHN.Trade = entry.CHN.Export + entry.CHN.Import
			entry.Total = entry.USA.Trade + entry.CHN.Trade
			if entry.Total > 0 {
				entry.ShareCN = entry.CHN.Trade / entry.Total
			}
			file.Rows = append(file.Rows, *entry)
			filePeriods[entry.Period] = struct{}{}
		}
		for period := range filePeriods {
			file.Periods = append(file.Periods, period)
		}
		sort.Strings(file.Periods)
		sort.Slice(file.Rows, func(i, j int) bool {
			if file.Rows[i].Period != file.Rows[j].Period {
				return file.Rows[i].Period < file.Rows[j].Period
			}
			if file.Rows[i].Total != file.Rows[j].Total {
				return file.Rows[i].Total > file.Rows[j].Total
			}
			return file.Rows[i].Code < file.Rows[j].Code
		})
		relativePath := reporter + ".json"
		files[relativePath] = file
		index.Partitions = append(index.Partitions, semiconductorMonthlyPartition{ReporterISO3: reporter, Href: "./" + relativePath, RowCount: len(file.Rows), PeriodCount: len(file.Periods)})
	}
	index.Reporters = reporters
	for period := range periodSet {
		index.Periods = append(index.Periods, period)
	}
	sort.Strings(index.Periods)
	return index, files
}

func augmentSemiconductorMonthlyMeta(meta *metaFile, index semiconductorMonthlyIndexFile) {
	if meta == nil {
		return
	}
	meta.SemiconductorMonthlyProvider = index.Provider
	meta.SemiconductorMonthlyReporterCount = len(index.Reporters)
	meta.SemiconductorMonthlyPeriodCount = len(index.Periods)
	meta.SemiconductorMonthlyObservationCount = index.ObservationCount
}
