package semiconductor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tradegravity/internal/strategic"
)

func TestRepositoryReferenceIsValidAndMapped(t *testing.T) {
	reference, err := Load(filepath.Join("..", "..", "configs", "semiconductor_reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	products, err := strategic.LoadCSV(filepath.Join("..", "..", "configs", "strategic_hs6.csv"))
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateStrategicRegistry(reference, products); err != nil {
		t.Fatal(err)
	}
	if len(Codes(reference)) < 30 {
		t.Fatalf("semiconductor atlas maps %d unique codes, want at least 30", len(Codes(reference)))
	}
}

func TestReferenceRejectsUnknownSource(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "configs", "semiconductor_reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(data), `"source_id":"eu_chips_act"`, `"source_id":"missing_source"`, 1)
	if _, err := Parse(strings.NewReader(broken)); err == nil {
		t.Fatal("Parse accepted an unknown policy source")
	}
}

func TestReferenceRejectsPaidOrProprietaryMetricInput(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "configs", "semiconductor_reference.json"))
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(data), `"access":"official_open_data"`, `"access":"paid_proprietary"`, 1)
	if _, err := Parse(strings.NewReader(broken)); err == nil {
		t.Fatal("Parse accepted a paid/proprietary semiconductor input")
	}
}
