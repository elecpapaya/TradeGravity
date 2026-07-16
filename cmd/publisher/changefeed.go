package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxPublicationRevisions = 20

type publicationChangesFile struct {
	SchemaVersion       string                   `json:"schema_version"`
	GeneratedAt         string                   `json:"generated_at"`
	PreviousGeneratedAt string                   `json:"previous_generated_at,omitempty"`
	Status              string                   `json:"status"`
	Scope               string                   `json:"scope"`
	Summary             publicationChangeSummary `json:"summary"`
	CurrentPeriods      []string                 `json:"current_periods"`
	NewPeriods          []string                 `json:"new_periods"`
	RemovedPeriods      []string                 `json:"removed_periods"`
	CurrentReporters    []string                 `json:"current_reporters"`
	NewReporters        []string                 `json:"new_reporters"`
	RemovedReporters    []string                 `json:"removed_reporters"`
	TopRevisions        []publicationRevision    `json:"top_revisions"`
}

type publicationChangeSummary struct {
	CurrentObservationCount  int `json:"current_observation_count"`
	PreviousObservationCount int `json:"previous_observation_count"`
	ObservationDelta         int `json:"observation_delta"`
	AddedRows                int `json:"added_rows"`
	RemovedRows              int `json:"removed_rows"`
	RevisedRows              int `json:"revised_rows"`
}

type publicationRevision struct {
	ReporterISO3          string   `json:"reporter_iso3"`
	Period                string   `json:"period"`
	Classification        string   `json:"classification"`
	Code                  string   `json:"code"`
	Label                 string   `json:"label"`
	PreviousUSATradeUSD   float64  `json:"previous_usa_trade_usd"`
	CurrentUSATradeUSD    float64  `json:"current_usa_trade_usd"`
	PreviousChinaTradeUSD float64  `json:"previous_china_trade_usd"`
	CurrentChinaTradeUSD  float64  `json:"current_china_trade_usd"`
	PreviousTotalUSD      float64  `json:"previous_total_usd"`
	CurrentTotalUSD       float64  `json:"current_total_usd"`
	DeltaTradeUSD         float64  `json:"delta_trade_usd"`
	MagnitudeTradeUSD     float64  `json:"magnitude_trade_usd"`
	ChangeRatio           *float64 `json:"change_ratio,omitempty"`
}

type publicationRow struct {
	reporter string
	row      semiconductorMonthlyProductEntry
}

func buildPublicationChanges(generatedAt, previousDir string, currentIndex semiconductorMonthlyIndexFile, currentFiles map[string]semiconductorMonthlyFile) (publicationChangesFile, error) {
	result := publicationChangesFile{
		SchemaVersion:    "1.0",
		GeneratedAt:      generatedAt,
		Status:           "baseline",
		Scope:            "Publish-to-publish comparison of focused monthly semiconductor observations; separate from month-to-month movement",
		CurrentPeriods:   append([]string{}, currentIndex.Periods...),
		CurrentReporters: append([]string{}, currentIndex.Reporters...),
		NewPeriods:       []string{},
		RemovedPeriods:   []string{},
		NewReporters:     []string{},
		RemovedReporters: []string{},
		TopRevisions:     []publicationRevision{},
		Summary: publicationChangeSummary{
			CurrentObservationCount: currentIndex.ObservationCount,
		},
	}
	sort.Strings(result.CurrentPeriods)
	sort.Strings(result.CurrentReporters)
	if strings.TrimSpace(previousDir) == "" {
		return result, nil
	}

	previousIndex, previousFiles, found, err := loadPreviousSemiconductorMonthly(previousDir)
	if err != nil {
		return publicationChangesFile{}, err
	}
	if !found {
		return result, nil
	}
	result.PreviousGeneratedAt = previousIndex.GeneratedAt
	result.Summary.PreviousObservationCount = previousIndex.ObservationCount
	result.Summary.ObservationDelta = currentIndex.ObservationCount - previousIndex.ObservationCount
	result.NewPeriods, result.RemovedPeriods = stringSetChanges(currentIndex.Periods, previousIndex.Periods)
	result.NewReporters, result.RemovedReporters = stringSetChanges(currentIndex.Reporters, previousIndex.Reporters)

	currentRows := publicationRowsByKey(currentFiles)
	previousRows := publicationRowsByKey(previousFiles)
	for key, current := range currentRows {
		previous, ok := previousRows[key]
		if !ok {
			result.Summary.AddedRows++
			continue
		}
		if monthlyEntryEqual(current.row, previous.row) {
			continue
		}
		result.Summary.RevisedRows++
		result.TopRevisions = append(result.TopRevisions, buildPublicationRevision(current, previous))
	}
	for key := range previousRows {
		if _, ok := currentRows[key]; !ok {
			result.Summary.RemovedRows++
		}
	}

	sort.Slice(result.TopRevisions, func(i, j int) bool {
		left, right := result.TopRevisions[i], result.TopRevisions[j]
		if left.MagnitudeTradeUSD != right.MagnitudeTradeUSD {
			return left.MagnitudeTradeUSD > right.MagnitudeTradeUSD
		}
		if left.ReporterISO3 != right.ReporterISO3 {
			return left.ReporterISO3 < right.ReporterISO3
		}
		if left.Period != right.Period {
			return left.Period > right.Period
		}
		return left.Code < right.Code
	})
	if len(result.TopRevisions) > maxPublicationRevisions {
		result.TopRevisions = result.TopRevisions[:maxPublicationRevisions]
	}
	if len(result.NewPeriods)+len(result.RemovedPeriods)+len(result.NewReporters)+len(result.RemovedReporters) > 0 ||
		result.Summary.ObservationDelta != 0 || result.Summary.AddedRows > 0 || result.Summary.RemovedRows > 0 || result.Summary.RevisedRows > 0 {
		result.Status = "changed"
	} else {
		result.Status = "unchanged"
	}
	return result, nil
}

func loadPreviousSemiconductorMonthly(dataDir string) (semiconductorMonthlyIndexFile, map[string]semiconductorMonthlyFile, bool, error) {
	indexPath := filepath.Join(dataDir, "semiconductors", "monthly", "index.json")
	file, err := os.Open(indexPath)
	if errors.Is(err, os.ErrNotExist) {
		return semiconductorMonthlyIndexFile{}, nil, false, nil
	}
	if err != nil {
		return semiconductorMonthlyIndexFile{}, nil, false, fmt.Errorf("open previous monthly semiconductor index: %w", err)
	}
	defer file.Close()
	var index semiconductorMonthlyIndexFile
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&index); err != nil {
		return semiconductorMonthlyIndexFile{}, nil, false, fmt.Errorf("decode previous monthly semiconductor index: %w", err)
	}
	if strings.TrimSpace(index.GeneratedAt) == "" {
		return semiconductorMonthlyIndexFile{}, nil, false, errors.New("previous monthly semiconductor index has no generated_at")
	}
	files := make(map[string]semiconductorMonthlyFile, len(index.Partitions))
	for _, partition := range index.Partitions {
		reporter := strings.ToUpper(strings.TrimSpace(partition.ReporterISO3))
		if !isPublishedISO3(reporter) {
			return semiconductorMonthlyIndexFile{}, nil, false, fmt.Errorf("previous monthly semiconductor index has invalid reporter %q", partition.ReporterISO3)
		}
		path := filepath.Join(dataDir, "semiconductors", "monthly", reporter+".json")
		partitionFile, err := os.Open(path)
		if err != nil {
			return semiconductorMonthlyIndexFile{}, nil, false, fmt.Errorf("open previous monthly semiconductor partition %s: %w", reporter, err)
		}
		var dataset semiconductorMonthlyFile
		partitionDecoder := json.NewDecoder(partitionFile)
		partitionDecoder.DisallowUnknownFields()
		decodeErr := partitionDecoder.Decode(&dataset)
		closeErr := partitionFile.Close()
		if decodeErr != nil {
			return semiconductorMonthlyIndexFile{}, nil, false, fmt.Errorf("decode previous monthly semiconductor partition %s: %w", reporter, decodeErr)
		}
		if closeErr != nil {
			return semiconductorMonthlyIndexFile{}, nil, false, fmt.Errorf("close previous monthly semiconductor partition %s: %w", reporter, closeErr)
		}
		if dataset.ReporterISO3 != reporter {
			return semiconductorMonthlyIndexFile{}, nil, false, fmt.Errorf("previous monthly semiconductor partition %s has reporter %q", reporter, dataset.ReporterISO3)
		}
		files[reporter+".json"] = dataset
	}
	return index, files, true, nil
}

func publicationRowsByKey(files map[string]semiconductorMonthlyFile) map[string]publicationRow {
	rows := make(map[string]publicationRow)
	for _, file := range files {
		reporter := strings.ToUpper(strings.TrimSpace(file.ReporterISO3))
		for _, row := range file.Rows {
			key := strings.Join([]string{reporter, row.Period, strings.ToUpper(strings.TrimSpace(row.Classification)), row.Code}, "|")
			rows[key] = publicationRow{reporter: reporter, row: row}
		}
	}
	return rows
}

func monthlyEntryEqual(left, right semiconductorMonthlyProductEntry) bool {
	return left.USA == right.USA && left.CHN == right.CHN && left.Total == right.Total && left.ShareCN == right.ShareCN && left.Label == right.Label
}

func buildPublicationRevision(current, previous publicationRow) publicationRevision {
	previousTotal := previous.row.USA.Trade + previous.row.CHN.Trade
	currentTotal := current.row.USA.Trade + current.row.CHN.Trade
	var changeRatio *float64
	if previousTotal > 0 {
		ratio := (currentTotal - previousTotal) / previousTotal
		changeRatio = &ratio
	}
	return publicationRevision{
		ReporterISO3:          current.reporter,
		Period:                current.row.Period,
		Classification:        current.row.Classification,
		Code:                  current.row.Code,
		Label:                 current.row.Label,
		PreviousUSATradeUSD:   previous.row.USA.Trade,
		CurrentUSATradeUSD:    current.row.USA.Trade,
		PreviousChinaTradeUSD: previous.row.CHN.Trade,
		CurrentChinaTradeUSD:  current.row.CHN.Trade,
		PreviousTotalUSD:      previousTotal,
		CurrentTotalUSD:       currentTotal,
		DeltaTradeUSD:         currentTotal - previousTotal,
		MagnitudeTradeUSD:     math.Abs(current.row.USA.Trade-previous.row.USA.Trade) + math.Abs(current.row.CHN.Trade-previous.row.CHN.Trade),
		ChangeRatio:           changeRatio,
	}
}

func stringSetChanges(current, previous []string) ([]string, []string) {
	currentSet := make(map[string]struct{}, len(current))
	previousSet := make(map[string]struct{}, len(previous))
	for _, value := range current {
		currentSet[value] = struct{}{}
	}
	for _, value := range previous {
		previousSet[value] = struct{}{}
	}
	added := make([]string, 0)
	removed := make([]string, 0)
	for value := range currentSet {
		if _, ok := previousSet[value]; !ok {
			added = append(added, value)
		}
	}
	for value := range previousSet {
		if _, ok := currentSet[value]; !ok {
			removed = append(removed, value)
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}
