package main

import (
	"sort"
	"strings"

	"tradegravity/internal/semiconductor"
)

const (
	semiconductorMinimumReporters = 15
	semiconductorMinimumPeriods   = 5
	semiconductorMinimumCodes     = 30
)

func buildSemiconductorPublication(reference semiconductor.Reference, files map[string]strategicFile) semiconductor.PublicationStatus {
	wanted := make(map[string]struct{})
	for _, code := range semiconductor.Codes(reference) {
		wanted[code] = struct{}{}
	}
	reporters := make(map[string]struct{})
	periods := make(map[string]struct{})
	rowCount := 0
	for _, file := range files {
		observed := false
		for _, row := range file.Rows {
			if _, ok := wanted[row.Code]; !ok {
				continue
			}
			if !row.USA.Available && !row.CHN.Available {
				continue
			}
			rowCount++
			observed = true
		}
		if observed {
			reporters[strings.ToUpper(file.ReporterISO3)] = struct{}{}
			periods[file.Period] = struct{}{}
		}
	}
	reporterList := sortedKeys(reporters)
	periodList := sortedKeys(periods)
	sort.Sort(sort.Reverse(sort.StringSlice(periodList)))
	status := "reference_only"
	if len(reporterList) > 0 {
		status = "limited"
	}
	if len(reporterList) >= semiconductorMinimumReporters && len(periodList) >= semiconductorMinimumPeriods && len(wanted) >= semiconductorMinimumCodes {
		status = "research_ready"
	}
	return semiconductor.PublicationStatus{
		Status:                 status,
		Scope:                  "UN Comtrade reporter observations against USA and China partners; not total world semiconductor trade or physical routes",
		RegisteredCodeCount:    len(wanted),
		ObservedReporterCount:  len(reporterList),
		ObservedPeriodCount:    len(periodList),
		ObservedRowCount:       rowCount,
		ObservedReporters:      reporterList,
		ObservedPeriods:        periodList,
		MinimumReporterTarget:  semiconductorMinimumReporters,
		MinimumPeriodTarget:    semiconductorMinimumPeriods,
		MinimumCodeTarget:      semiconductorMinimumCodes,
		MeasurementDescription: "Coverage gates describe published stage-mapped HS6 observations. Passing them does not reveal capacity, process node, firms, services or shipment routes.",
	}
}

func augmentSemiconductorMeta(metadata *metaFile, reference semiconductor.Reference) {
	metadata.SemiconductorStatus = reference.Publication.Status
	metadata.SemiconductorCodeCount = reference.Publication.RegisteredCodeCount
	metadata.SemiconductorReporterCount = reference.Publication.ObservedReporterCount
	metadata.SemiconductorPeriodCount = reference.Publication.ObservedPeriodCount
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}
