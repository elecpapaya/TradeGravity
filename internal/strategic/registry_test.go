package strategic

import (
	"strings"
	"testing"
)

func TestParseFilterAndCodes(t *testing.T) {
	products, err := ParseCSV(strings.NewReader("code,sector,label,revision_note,notes\n854231,semiconductors,Processors,HS 2017+,test\n850760,ev_batteries,Lithium-ion batteries,Multiple HS revisions,test\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 2 || products[0].Code != "850760" {
		t.Fatalf("products not sorted by sector/code: %+v", products)
	}
	filtered, err := Filter(products, []string{"semiconductors"})
	if err != nil {
		t.Fatal(err)
	}
	if got := Codes(filtered); len(got) != 1 || got[0] != "854231" {
		t.Fatalf("Codes() = %v", got)
	}
	if _, err := Filter(products, []string{"unknown"}); err == nil {
		t.Fatal("Filter() accepted unknown sector")
	}
}

func TestParseRejectsDuplicateAndNonHS6Codes(t *testing.T) {
	for _, input := range []string{
		"code,sector,label,revision_note,notes\n8542,semiconductors,ICs,all,test\n",
		"code,sector,label,revision_note,notes\n854231,semiconductors,Processors,all,test\n854231,semiconductors,Processors,all,test\n",
	} {
		if _, err := ParseCSV(strings.NewReader(input)); err == nil {
			t.Fatalf("ParseCSV() accepted invalid registry: %q", input)
		}
	}
}
