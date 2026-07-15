package wits

import (
	"testing"

	"tradegravity/internal/model"
)

func TestParseReportersXMLFiltersGroupsAndNonReporters(t *testing.T) {
	payload := []byte(`<root><countries>
		<country isreporter="1" isgroup="No"><iso3Code>kor</iso3Code><name>Korea, Rep.</name></country>
		<country isreporter="0" isgroup="No"><iso3Code>usa</iso3Code><name>United States</name></country>
		<country isreporter="1" isgroup="yes"><iso3Code>wld</iso3Code><name>World</name></country>
	</countries></root>`)

	got, err := parseReportersXML(payload)
	if err != nil {
		t.Fatalf("parseReportersXML() error = %v", err)
	}
	if len(got) != 1 || got[0].ISO3 != "KOR" || !got[0].IsActive {
		t.Fatalf("parseReportersXML() = %#v, want active KOR reporter", got)
	}
}

func TestNormalizePeriod(t *testing.T) {
	tests := []struct {
		input      string
		wantType   model.PeriodType
		wantPeriod string
		wantOK     bool
	}{
		{input: "202401", wantType: model.PeriodMonth, wantPeriod: "2024-01", wantOK: true},
		{input: "2024-Q3", wantType: model.PeriodQuarter, wantPeriod: "2024-Q3", wantOK: true},
		{input: "2024", wantType: model.PeriodYear, wantPeriod: "2024", wantOK: true},
		{input: "2024-13", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotType, gotPeriod, gotOK := normalizePeriod(tt.input)
			if gotType != tt.wantType || gotPeriod != tt.wantPeriod || gotOK != tt.wantOK {
				t.Fatalf("normalizePeriod(%q) = (%q, %q, %v), want (%q, %q, %v)", tt.input, gotType, gotPeriod, gotOK, tt.wantType, tt.wantPeriod, tt.wantOK)
			}
		})
	}
}
