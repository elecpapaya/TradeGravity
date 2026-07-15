package main

import (
	"reflect"
	"testing"
	"time"
)

func TestAnnualHistoryIncludesSelectedYearAndRequestedWindow(t *testing.T) {
	want := []string{"2019", "2020", "2021", "2022", "2023"}
	if got := annualHistory("2023", 4); !reflect.DeepEqual(got, want) {
		t.Fatalf("annualHistory() = %v, want %v", got, want)
	}
	if got := annualHistory("latest", 4); got != nil {
		t.Fatalf("annualHistory invalid year = %v, want nil", got)
	}
}

func TestMonthlyWindowUsesCompleteMonthsInAscendingOrder(t *testing.T) {
	want := []string{"2025-11", "2025-12", "2026-01"}
	got, err := monthlyWindow("auto", 3, time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("monthlyWindow() = %v, want %v", got, want)
	}
	if _, err := monthlyWindow("2026-13", 3, time.Now()); err == nil {
		t.Fatal("monthlyWindow accepted an invalid month")
	}
}
