package strategic

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Product is a curated HS6 product used for bounded strategic-sector
// collection. Label is a project-facing description, while RevisionNote keeps
// classification caveats visible to publishers and users.
type Product struct {
	Code         string
	Sector       string
	Label        string
	RevisionNote string
	Notes        string
}

func LoadCSV(path string) ([]Product, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("strategic registry path is required")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return ParseCSV(file)
}

func ParseCSV(reader io.Reader) ([]Product, error) {
	rows, err := csv.NewReader(reader).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, errors.New("strategic registry must include a header and at least one product")
	}
	wantHeader := []string{"code", "sector", "label", "revision_note", "notes"}
	if len(rows[0]) != len(wantHeader) {
		return nil, fmt.Errorf("strategic registry header has %d columns, want %d", len(rows[0]), len(wantHeader))
	}
	for index, want := range wantHeader {
		if strings.TrimSpace(strings.ToLower(rows[0][index])) != want {
			return nil, fmt.Errorf("strategic registry column %d is %q, want %q", index+1, rows[0][index], want)
		}
	}

	products := make([]Product, 0, len(rows)-1)
	seen := make(map[string]struct{}, len(rows)-1)
	for index, row := range rows[1:] {
		line := index + 2
		if len(row) != len(wantHeader) {
			return nil, fmt.Errorf("strategic registry line %d has %d columns, want %d", line, len(row), len(wantHeader))
		}
		product := Product{
			Code:         strings.TrimSpace(row[0]),
			Sector:       strings.ToLower(strings.TrimSpace(row[1])),
			Label:        strings.TrimSpace(row[2]),
			RevisionNote: strings.TrimSpace(row[3]),
			Notes:        strings.TrimSpace(row[4]),
		}
		if !isSixDigits(product.Code) {
			return nil, fmt.Errorf("strategic registry line %d has invalid HS6 code %q", line, product.Code)
		}
		if !isSlug(product.Sector) {
			return nil, fmt.Errorf("strategic registry line %d has invalid sector %q", line, product.Sector)
		}
		if product.Label == "" || product.RevisionNote == "" {
			return nil, fmt.Errorf("strategic registry line %d requires label and revision_note", line)
		}
		if _, exists := seen[product.Code]; exists {
			return nil, fmt.Errorf("strategic registry has duplicate HS6 code %s", product.Code)
		}
		seen[product.Code] = struct{}{}
		products = append(products, product)
	}
	sort.Slice(products, func(i, j int) bool {
		if products[i].Sector != products[j].Sector {
			return products[i].Sector < products[j].Sector
		}
		return products[i].Code < products[j].Code
	})
	return products, nil
}

func Filter(products []Product, sectors []string) ([]Product, error) {
	if len(sectors) == 0 {
		return append([]Product(nil), products...), nil
	}
	wanted := make(map[string]struct{}, len(sectors))
	for _, sector := range sectors {
		normalized := strings.ToLower(strings.TrimSpace(sector))
		if normalized == "" || normalized == "all" {
			continue
		}
		wanted[normalized] = struct{}{}
	}
	if len(wanted) == 0 {
		return append([]Product(nil), products...), nil
	}
	known := make(map[string]struct{})
	filtered := make([]Product, 0, len(products))
	for _, product := range products {
		known[product.Sector] = struct{}{}
		if _, ok := wanted[product.Sector]; ok {
			filtered = append(filtered, product)
		}
	}
	for sector := range wanted {
		if _, ok := known[sector]; !ok {
			return nil, fmt.Errorf("unknown strategic sector %q", sector)
		}
	}
	if len(filtered) == 0 {
		return nil, errors.New("strategic sector filter selected no products")
	}
	return filtered, nil
}

func Codes(products []Product) []string {
	codes := make([]string, 0, len(products))
	for _, product := range products {
		codes = append(codes, product.Code)
	}
	return codes
}

func Sectors(products []Product) []string {
	set := make(map[string]struct{})
	for _, product := range products {
		set[product.Sector] = struct{}{}
	}
	sectors := make([]string, 0, len(set))
	for sector := range set {
		sectors = append(sectors, sector)
	}
	sort.Strings(sectors)
	return sectors
}

func isSixDigits(value string) bool {
	if len(value) != 6 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func isSlug(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}
