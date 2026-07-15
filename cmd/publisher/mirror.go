package main

import (
	"sort"
	"strings"
)

var mirrorAnchors = []string{"USA", "CHN"}

type mirrorIndexFile struct {
	SchemaVersion   string            `json:"schema_version"`
	GeneratedAt     string            `json:"generated_at"`
	Provider        string            `json:"provider"`
	Anchors         []string          `json:"anchors"`
	Reporters       []string          `json:"reporters"`
	Periods         []string          `json:"periods"`
	Partitions      []mirrorPartition `json:"partitions"`
	ComparisonCount int               `json:"comparison_count"`
}

type mirrorPartition struct {
	ReporterISO3    string `json:"reporter_iso3"`
	Period          string `json:"period"`
	Href            string `json:"href"`
	ComparisonCount int    `json:"comparison_count"`
}

type mirrorFile struct {
	SchemaVersion string             `json:"schema_version"`
	GeneratedAt   string             `json:"generated_at"`
	Provider      string             `json:"provider"`
	ReporterISO3  string             `json:"reporter_iso3"`
	Period        string             `json:"period"`
	Scope         string             `json:"scope"`
	Caveats       []string           `json:"caveats"`
	Rows          []mirrorAnchorPair `json:"rows"`
}

type mirrorAnchorPair struct {
	AnchorISO3              string   `json:"anchor_iso3"`
	ReporterExportAvailable bool     `json:"reporter_export_available"`
	AnchorImportAvailable   bool     `json:"anchor_import_available"`
	ReporterExportUSD       float64  `json:"reporter_export_usd"`
	AnchorImportUSD         float64  `json:"anchor_import_usd"`
	ExportGapUSD            *float64 `json:"export_gap_usd,omitempty"`
	ExportSymmetricGapRatio *float64 `json:"export_symmetric_gap_ratio,omitempty"`
	ReporterImportAvailable bool     `json:"reporter_import_available"`
	AnchorExportAvailable   bool     `json:"anchor_export_available"`
	ReporterImportUSD       float64  `json:"reporter_import_usd"`
	AnchorExportUSD         float64  `json:"anchor_export_usd"`
	ImportGapUSD            *float64 `json:"import_gap_usd,omitempty"`
	ImportSymmetricGapRatio *float64 `json:"import_symmetric_gap_ratio,omitempty"`
}

func buildMirrorFiles(generatedAt, provider string, matrixFiles map[string]matrixFile) (mirrorIndexFile, map[string]mirrorFile) {
	index := mirrorIndexFile{
		SchemaVersion: schemaVersion,
		GeneratedAt:   generatedAt,
		Provider:      strings.ToLower(strings.TrimSpace(provider)),
		Anchors:       append([]string(nil), mirrorAnchors...),
		Reporters:     []string{},
		Periods:       []string{},
		Partitions:    []mirrorPartition{},
	}
	files := make(map[string]mirrorFile)
	reporterSet := make(map[string]struct{})
	periodSet := make(map[string]struct{})

	keys := make([]string, 0, len(matrixFiles))
	for key := range matrixFiles {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		reporterFile := matrixFiles[key]
		if reporterFile.ReporterISO3 == "USA" || reporterFile.ReporterISO3 == "CHN" {
			continue
		}
		output := mirrorFile{
			SchemaVersion: schemaVersion,
			GeneratedAt:   generatedAt,
			Provider:      index.Provider,
			ReporterISO3:  reporterFile.ReporterISO3,
			Period:        reporterFile.Period,
			Scope:         "Unadjusted bilateral mirror-reporting diagnostics; neither reporter is treated as ground truth",
			Caveats:       []string{"Differences can reflect CIF/FOB valuation, timing, classification, partner attribution, re-exports, and revisions.", "These diagnostics are not fraud, evasion, transshipment, or physical-route estimates."},
			Rows:          []mirrorAnchorPair{},
		}
		comparisonCount := 0
		for _, anchor := range mirrorAnchors {
			anchorFile, ok := matrixFiles[anchor+"/"+reporterFile.Period+".json"]
			if !ok {
				continue
			}
			reporterSide, reporterOK := matrixPartnerByISO(reporterFile.Rows, anchor)
			anchorSide, anchorOK := matrixPartnerByISO(anchorFile.Rows, reporterFile.ReporterISO3)
			if !reporterOK && !anchorOK {
				continue
			}
			row := mirrorAnchorPair{AnchorISO3: anchor}
			if reporterOK {
				row.ReporterExportAvailable = reporterSide.ExportAvailable
				row.ReporterImportAvailable = reporterSide.ImportAvailable
				row.ReporterExportUSD = reporterSide.ExportUSD
				row.ReporterImportUSD = reporterSide.ImportUSD
			}
			if anchorOK {
				row.AnchorImportAvailable = anchorSide.ImportAvailable
				row.AnchorExportAvailable = anchorSide.ExportAvailable
				row.AnchorImportUSD = anchorSide.ImportUSD
				row.AnchorExportUSD = anchorSide.ExportUSD
			}
			if row.ReporterExportAvailable && row.AnchorImportAvailable {
				row.ExportGapUSD, row.ExportSymmetricGapRatio = mirrorGap(row.ReporterExportUSD, row.AnchorImportUSD)
				comparisonCount++
			}
			if row.ReporterImportAvailable && row.AnchorExportAvailable {
				row.ImportGapUSD, row.ImportSymmetricGapRatio = mirrorGap(row.ReporterImportUSD, row.AnchorExportUSD)
				comparisonCount++
			}
			output.Rows = append(output.Rows, row)
		}
		if len(output.Rows) == 0 {
			continue
		}
		relativePath := output.ReporterISO3 + "/" + output.Period + ".json"
		files[relativePath] = output
		index.Partitions = append(index.Partitions, mirrorPartition{ReporterISO3: output.ReporterISO3, Period: output.Period, Href: "./" + relativePath, ComparisonCount: comparisonCount})
		index.ComparisonCount += comparisonCount
		reporterSet[output.ReporterISO3] = struct{}{}
		periodSet[output.Period] = struct{}{}
	}
	for reporter := range reporterSet {
		index.Reporters = append(index.Reporters, reporter)
	}
	for period := range periodSet {
		index.Periods = append(index.Periods, period)
	}
	sort.Strings(index.Reporters)
	sort.Sort(sort.Reverse(sort.StringSlice(index.Periods)))
	return index, files
}

func matrixPartnerByISO(rows []matrixPartner, partner string) (matrixPartner, bool) {
	for _, row := range rows {
		if row.PartnerISO3 == partner {
			return row, true
		}
	}
	return matrixPartner{}, false
}

func mirrorGap(reporterValue, mirrorValue float64) (*float64, *float64) {
	gap := reporterValue - mirrorValue
	average := (reporterValue + mirrorValue) / 2
	ratio := 0.0
	if average > 0 {
		ratio = gap / average
	}
	return &gap, &ratio
}
