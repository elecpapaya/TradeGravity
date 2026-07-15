package semiconductor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"tradegravity/internal/strategic"
)

type Reference struct {
	SchemaVersion    string            `json:"schema_version"`
	UpdatedAt        string            `json:"updated_at"`
	GeneratedAt      string            `json:"generated_at,omitempty"`
	Title            string            `json:"title"`
	Scope            string            `json:"scope"`
	Perspective      Perspective       `json:"perspective"`
	DataPolicy       DataPolicy        `json:"data_policy"`
	OpenDatasets     []OpenDataset     `json:"open_datasets"`
	Caveats          []string          `json:"caveats"`
	Stages           []Stage           `json:"stages"`
	CountryRoles     []CountryRole     `json:"country_roles"`
	Trends           []Trend           `json:"trends"`
	PolicyEvents     []PolicyEvent     `json:"policy_events"`
	CapacitySignals  []CapacitySignal  `json:"capacity_signals"`
	Sources          []Source          `json:"sources"`
	ScenarioDefaults ScenarioDefaults  `json:"scenario_defaults"`
	Publication      PublicationStatus `json:"publication,omitempty"`
}

type Stage struct {
	ID              string   `json:"id"`
	Order           int      `json:"order"`
	Label           string   `json:"label"`
	ShortLabel      string   `json:"short_label"`
	Description     string   `json:"description"`
	ObservationType string   `json:"observation_type"`
	Codes           []string `json:"codes"`
	Gap             string   `json:"gap"`
}

type CountryRole struct {
	ISO3     string   `json:"iso3"`
	Name     string   `json:"name"`
	Roles    []string `json:"roles"`
	Note     string   `json:"note"`
	Evidence string   `json:"evidence"`
}

type Trend struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Summary   string   `json:"summary"`
	AsOf      string   `json:"as_of"`
	SourceIDs []string `json:"source_ids"`
}

type PolicyEvent struct {
	Date         string   `json:"date"`
	Jurisdiction string   `json:"jurisdiction"`
	Title        string   `json:"title"`
	Kind         string   `json:"kind"`
	Stages       []string `json:"stages"`
	Status       string   `json:"status"`
	SourceID     string   `json:"source_id"`
}

type CapacitySignal struct {
	AnnouncedAt       string `json:"announced_at"`
	Country           string `json:"country"`
	ISO3              string `json:"iso3"`
	Title             string `json:"title"`
	Stage             string `json:"stage"`
	Status            string `json:"status"`
	ExpectedOperation string `json:"expected_operation"`
	Claim             string `json:"claim"`
	SourceID          string `json:"source_id"`
}

type Source struct {
	ID          string `json:"id"`
	Publisher   string `json:"publisher"`
	Title       string `json:"title"`
	PublishedAt string `json:"published_at"`
	URL         string `json:"url"`
	SourceType  string `json:"source_type"`
	Access      string `json:"access"`
	ReuseNote   string `json:"reuse_note"`
}

type Perspective struct {
	Lens      string   `json:"lens"`
	Principle string   `json:"principle"`
	Anchors   []string `json:"anchors"`
	Questions []string `json:"questions"`
}

type DataPolicy struct {
	Mode       string   `json:"mode"`
	Rule       string   `json:"rule"`
	Exclusions []string `json:"exclusions"`
}

type OpenDataset struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Granularity string `json:"granularity"`
	Role        string `json:"role"`
	Access      string `json:"access"`
	ReuseNote   string `json:"reuse_note"`
}

type ScenarioDefaults struct {
	DisruptionPercent   float64 `json:"disruption_percent"`
	SubstitutionPercent float64 `json:"substitution_percent"`
	Warning             string  `json:"warning"`
}

type PublicationStatus struct {
	Status                 string   `json:"status,omitempty"`
	Scope                  string   `json:"scope,omitempty"`
	RegisteredCodeCount    int      `json:"registered_code_count,omitempty"`
	ObservedReporterCount  int      `json:"observed_reporter_count,omitempty"`
	ObservedPeriodCount    int      `json:"observed_period_count,omitempty"`
	ObservedRowCount       int      `json:"observed_row_count,omitempty"`
	ObservedReporters      []string `json:"observed_reporters,omitempty"`
	ObservedPeriods        []string `json:"observed_periods,omitempty"`
	MinimumReporterTarget  int      `json:"minimum_reporter_target,omitempty"`
	MinimumPeriodTarget    int      `json:"minimum_period_target,omitempty"`
	MinimumCodeTarget      int      `json:"minimum_code_target,omitempty"`
	MeasurementDescription string   `json:"measurement_description,omitempty"`
}

func Load(path string) (Reference, error) {
	file, err := os.Open(path)
	if err != nil {
		return Reference{}, err
	}
	defer file.Close()
	return Parse(file)
}

func Parse(reader io.Reader) (Reference, error) {
	var reference Reference
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reference); err != nil {
		return Reference{}, err
	}
	if err := validate(reference); err != nil {
		return Reference{}, err
	}
	sort.Slice(reference.Stages, func(i, j int) bool { return reference.Stages[i].Order < reference.Stages[j].Order })
	sort.Slice(reference.PolicyEvents, func(i, j int) bool { return reference.PolicyEvents[i].Date < reference.PolicyEvents[j].Date })
	sort.Slice(reference.CountryRoles, func(i, j int) bool { return reference.CountryRoles[i].ISO3 < reference.CountryRoles[j].ISO3 })
	return reference, nil
}

func ValidateStrategicRegistry(reference Reference, products []strategic.Product) error {
	known := make(map[string]struct{}, len(products))
	for _, product := range products {
		known[product.Code] = struct{}{}
	}
	missing := []string{}
	for _, code := range Codes(reference) {
		if _, ok := known[code]; !ok {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("semiconductor reference codes missing from strategic registry: %s", strings.Join(missing, ","))
	}
	return nil
}

func Codes(reference Reference) []string {
	set := make(map[string]struct{})
	for _, stage := range reference.Stages {
		for _, code := range stage.Codes {
			set[code] = struct{}{}
		}
	}
	codes := make([]string, 0, len(set))
	for code := range set {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func validate(reference Reference) error {
	if reference.SchemaVersion != "1.0" {
		return fmt.Errorf("semiconductor reference schema_version is %q, want 1.0", reference.SchemaVersion)
	}
	if _, err := time.Parse("2006-01-02", reference.UpdatedAt); err != nil {
		return fmt.Errorf("invalid semiconductor reference updated_at: %w", err)
	}
	if strings.TrimSpace(reference.Title) == "" || strings.TrimSpace(reference.Scope) == "" {
		return errors.New("semiconductor reference requires title and scope")
	}
	if reference.Perspective.Lens != "us_china" || reference.Perspective.Principle == "" || !equalStrings(reference.Perspective.Anchors, []string{"USA", "CHN"}) || len(reference.Perspective.Questions) < 4 {
		return errors.New("semiconductor perspective requires the explicit USA/China lens and core questions")
	}
	if reference.DataPolicy.Mode != "free_public_only" || strings.TrimSpace(reference.DataPolicy.Rule) == "" || len(reference.DataPolicy.Exclusions) == 0 {
		return errors.New("semiconductor data policy must require free_public_only sources")
	}
	if len(reference.OpenDatasets) < 4 {
		return errors.New("semiconductor reference requires at least four open dataset declarations")
	}
	openDatasetIDs := make(map[string]struct{}, len(reference.OpenDatasets))
	for _, dataset := range reference.OpenDatasets {
		if !isSlug(dataset.ID) || strings.TrimSpace(dataset.Provider) == "" || strings.TrimSpace(dataset.Title) == "" || strings.TrimSpace(dataset.Granularity) == "" || strings.TrimSpace(dataset.Role) == "" || !isFreeAccess(dataset.Access) || strings.TrimSpace(dataset.ReuseNote) == "" {
			return fmt.Errorf("invalid open semiconductor dataset %+v", dataset)
		}
		if _, exists := openDatasetIDs[dataset.ID]; exists {
			return fmt.Errorf("duplicate open semiconductor dataset %s", dataset.ID)
		}
		parsed, err := url.Parse(dataset.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("open dataset %s requires an HTTPS URL", dataset.ID)
		}
		openDatasetIDs[dataset.ID] = struct{}{}
	}
	if len(reference.Caveats) < 3 || len(reference.Stages) < 7 {
		return errors.New("semiconductor reference requires at least three caveats and seven value-chain stages")
	}
	stages := make(map[string]struct{}, len(reference.Stages))
	orders := make(map[int]struct{}, len(reference.Stages))
	for _, stage := range reference.Stages {
		if !isSlug(stage.ID) || stage.Order <= 0 || strings.TrimSpace(stage.Label) == "" || strings.TrimSpace(stage.Gap) == "" || strings.TrimSpace(stage.ObservationType) == "" {
			return fmt.Errorf("invalid semiconductor stage %+v", stage)
		}
		if _, exists := stages[stage.ID]; exists {
			return fmt.Errorf("duplicate semiconductor stage %s", stage.ID)
		}
		if _, exists := orders[stage.Order]; exists {
			return fmt.Errorf("duplicate semiconductor stage order %d", stage.Order)
		}
		stages[stage.ID], orders[stage.Order] = struct{}{}, struct{}{}
		seenCodes := make(map[string]struct{}, len(stage.Codes))
		for _, code := range stage.Codes {
			if !isSixDigits(code) {
				return fmt.Errorf("stage %s has invalid HS6 code %q", stage.ID, code)
			}
			if _, exists := seenCodes[code]; exists {
				return fmt.Errorf("stage %s repeats HS6 code %s", stage.ID, code)
			}
			seenCodes[code] = struct{}{}
		}
	}

	sources := make(map[string]struct{}, len(reference.Sources))
	for _, source := range reference.Sources {
		if !isSlug(source.ID) || strings.TrimSpace(source.Publisher) == "" || strings.TrimSpace(source.Title) == "" || !isFreeAccess(source.Access) || strings.TrimSpace(source.ReuseNote) == "" {
			return fmt.Errorf("invalid semiconductor source %+v", source)
		}
		if _, exists := sources[source.ID]; exists {
			return fmt.Errorf("duplicate semiconductor source %s", source.ID)
		}
		parsed, err := url.Parse(source.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("source %s requires an HTTPS URL", source.ID)
		}
		if _, err := time.Parse("2006-01-02", source.PublishedAt); err != nil {
			return fmt.Errorf("source %s has invalid published_at", source.ID)
		}
		sources[source.ID] = struct{}{}
	}
	if len(sources) == 0 {
		return errors.New("semiconductor reference requires sources")
	}

	for _, role := range reference.CountryRoles {
		if len(role.ISO3) != 3 || strings.ToUpper(role.ISO3) != role.ISO3 || strings.TrimSpace(role.Name) == "" || len(role.Roles) == 0 || role.Evidence != "contextual" {
			return fmt.Errorf("invalid semiconductor country role %+v", role)
		}
		for _, stage := range role.Roles {
			if _, ok := stages[stage]; !ok {
				return fmt.Errorf("country %s references unknown stage %s", role.ISO3, stage)
			}
		}
	}
	for _, trend := range reference.Trends {
		if !isSlug(trend.ID) || strings.TrimSpace(trend.Label) == "" || strings.TrimSpace(trend.Summary) == "" || len(trend.SourceIDs) == 0 {
			return fmt.Errorf("invalid semiconductor trend %+v", trend)
		}
		if _, err := time.Parse("2006-01-02", trend.AsOf); err != nil {
			return fmt.Errorf("trend %s has invalid as_of", trend.ID)
		}
		for _, sourceID := range trend.SourceIDs {
			if _, ok := sources[sourceID]; !ok {
				return fmt.Errorf("trend %s references unknown source %s", trend.ID, sourceID)
			}
		}
	}
	for _, event := range reference.PolicyEvents {
		if _, err := time.Parse("2006-01-02", event.Date); err != nil {
			return fmt.Errorf("policy event %q has invalid date", event.Title)
		}
		if strings.TrimSpace(event.Jurisdiction) == "" || strings.TrimSpace(event.Title) == "" || !isSlug(event.Kind) || !isSlug(event.Status) {
			return fmt.Errorf("invalid semiconductor policy event %+v", event)
		}
		if _, ok := sources[event.SourceID]; !ok {
			return fmt.Errorf("policy event %q references unknown source %s", event.Title, event.SourceID)
		}
		for _, stage := range event.Stages {
			if _, ok := stages[stage]; !ok {
				return fmt.Errorf("policy event %q references unknown stage %s", event.Title, stage)
			}
		}
	}
	for _, signal := range reference.CapacitySignals {
		if _, err := time.Parse("2006-01-02", signal.AnnouncedAt); err != nil {
			return fmt.Errorf("capacity signal %q has invalid announced_at", signal.Title)
		}
		if strings.TrimSpace(signal.Country) == "" || (signal.ISO3 != "WLD" && (len(signal.ISO3) != 3 || strings.ToUpper(signal.ISO3) != signal.ISO3)) || strings.TrimSpace(signal.Title) == "" || strings.TrimSpace(signal.Claim) == "" || strings.TrimSpace(signal.ExpectedOperation) == "" || !isSlug(signal.Status) {
			return fmt.Errorf("invalid semiconductor capacity signal %+v", signal)
		}
		if _, ok := stages[signal.Stage]; !ok {
			return fmt.Errorf("capacity signal %q references unknown stage %s", signal.Title, signal.Stage)
		}
		if _, ok := sources[signal.SourceID]; !ok {
			return fmt.Errorf("capacity signal %q references unknown source %s", signal.Title, signal.SourceID)
		}
	}
	if reference.ScenarioDefaults.DisruptionPercent < 0 || reference.ScenarioDefaults.DisruptionPercent > 100 || reference.ScenarioDefaults.SubstitutionPercent < 0 || reference.ScenarioDefaults.SubstitutionPercent > 100 || strings.TrimSpace(reference.ScenarioDefaults.Warning) == "" {
		return errors.New("semiconductor scenario defaults require bounded percentages and a warning")
	}
	return nil
}

func isFreeAccess(value string) bool {
	switch value {
	case "free_public_web", "official_open_data", "public_download":
		return true
	default:
		return false
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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
