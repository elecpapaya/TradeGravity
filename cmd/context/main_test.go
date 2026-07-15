package main

import "testing"

func TestLatestIndicatorValuesSelectsNewestNonNullValue(t *testing.T) {
	oldValue := 10.0
	newValue := 12.0
	rows := []wbIndicator{
		{CountryISO3: "kor", Date: "2021", Value: &oldValue},
		{CountryISO3: "KOR", Date: "2023", Value: &newValue},
		{CountryISO3: "KOR", Date: "2024", Value: nil},
	}
	got := latestIndicatorValues(rows)["KOR"]
	if got.Value == nil || *got.Value != 12 || got.Year != "2023" {
		t.Fatalf("latest value = %#v, want 12 in 2023", got)
	}
}

func TestSplitGroupsNormalizesAndSorts(t *testing.T) {
	got := splitGroups(" eu;ASEAN ")
	if len(got) != 2 || got[0] != "ASEAN" || got[1] != "EU" {
		t.Fatalf("splitGroups() = %#v", got)
	}
}
