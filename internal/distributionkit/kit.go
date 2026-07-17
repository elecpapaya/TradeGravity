package distributionkit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	kitSchemaVersion = "1.0"
	kitToolVersion   = "tradegravity-distributor/1.1"
	captionRuneLimit = 1800
)

var readySignalKinds = []string{"reporter_total_change", "anchor_share_shift", "product_total_change"}
var readySlideRoles = []string{"cover", "scale", "anchor_balance", "product", "method", "cta"}

type briefing struct {
	SchemaVersion      string     `json:"schema_version"`
	GeneratedAt        string     `json:"generated_at"`
	EditionID          string     `json:"edition_id"`
	Status             string     `json:"status"`
	Title              string     `json:"title"`
	Scope              string     `json:"scope"`
	LatestPeriod       string     `json:"latest_period"`
	PreviousPeriod     string     `json:"previous_period"`
	ReviewRequired     bool       `json:"review_required"`
	Signals            []signal   `json:"signals"`
	Email              emailDraft `json:"email"`
	SocialCarousel     carousel   `json:"social_carousel"`
	Caveats            []string   `json:"caveats"`
	EvidenceEntryPoint string     `json:"evidence_entry_point"`
}

type signal struct {
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Title          string   `json:"title"`
	Summary        string   `json:"summary"`
	Period         string   `json:"period"`
	PreviousPeriod string   `json:"previous_period"`
	Evidence       []string `json:"evidence"`
}

type emailDraft struct {
	Subject     string `json:"subject"`
	Preview     string `json:"preview"`
	Markdown    string `json:"markdown"`
	CTALabel    string `json:"cta_label"`
	CTAPath     string `json:"cta_path"`
	SendPolicy  string `json:"send_policy"`
	PrimaryGoal string `json:"primary_goal"`
}

type carousel struct {
	Format       string  `json:"format"`
	AspectRatio  string  `json:"aspect_ratio"`
	ReviewPolicy string  `json:"review_policy"`
	Slides       []slide `json:"slides"`
}

type slide struct {
	Order    int      `json:"order"`
	Role     string   `json:"role"`
	Headline string   `json:"headline"`
	Body     string   `json:"body"`
	Evidence []string `json:"evidence"`
}

type Manifest struct {
	SchemaVersion           string         `json:"schema_version"`
	Tool                    string         `json:"tool"`
	EditionID               string         `json:"edition_id"`
	SourceGeneratedAt       string         `json:"source_generated_at"`
	DistributionStatus      string         `json:"distribution_status"`
	ReviewRequired          bool           `json:"review_required"`
	SendAuthorized          bool           `json:"send_authorized"`
	SocialPublishAuthorized bool           `json:"social_publish_authorized"`
	BaseURL                 string         `json:"base_url"`
	PrimaryGoal             string         `json:"primary_goal"`
	Email                   ManifestEmail  `json:"email"`
	Carousel                ManifestSocial `json:"carousel"`
	Files                   []ManifestFile `json:"files"`
}

type ManifestEmail struct {
	Subject string `json:"subject"`
	Preview string `json:"preview"`
	CTAURL  string `json:"cta_url"`
}

type ManifestSocial struct {
	Theme       string   `json:"theme"`
	CaptionPath string   `json:"caption_path"`
	AspectRatio string   `json:"aspect_ratio"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	SlideCount  int      `json:"slide_count"`
	Formats     []string `json:"formats"`
}

type ManifestFile struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
	Bytes     int    `json:"bytes"`
	SHA256    string `json:"sha256"`
}

type Bundle struct {
	Manifest Manifest
	Files    map[string][]byte
}

func Build(raw []byte, baseURL string) (Bundle, error) {
	return BuildWithOptions(raw, baseURL, BuildOptions{})
}

func BuildWithOptions(raw []byte, baseURL string, options BuildOptions) (Bundle, error) {
	theme, err := resolveTheme(options.Theme)
	if err != nil {
		return Bundle{}, err
	}
	var source briefing
	if err := json.Unmarshal(raw, &source); err != nil {
		return Bundle{}, fmt.Errorf("decode briefing: %w", err)
	}
	if err := validateBriefing(source); err != nil {
		return Bundle{}, err
	}
	root, err := normalizeBaseURL(baseURL)
	if err != nil {
		return Bundle{}, err
	}

	ctaURL, err := resolveRootReference(root, source.Email.CTAPath)
	if err != nil {
		return Bundle{}, fmt.Errorf("resolve email CTA: %w", err)
	}
	files := map[string][]byte{
		"email/subject.txt": []byte(source.Email.Subject + "\n"),
		"email/preview.txt": []byte(source.Email.Preview + "\n"),
		"email/body.md":     renderEmailMarkdown(source, root),
		"email/body.html":   renderEmailHTML(source, ctaURL),
		"REVIEW.md":         renderReviewChecklist(source, root),
	}
	fonts, err := newSlideFonts()
	if err != nil {
		return Bundle{}, err
	}
	defer fonts.Close()

	for _, item := range source.SocialCarousel.Slides {
		svgPath := fmt.Sprintf("carousel/slide-%02d.svg", item.Order)
		files[svgPath] = renderSlideSVG(source, item, root, theme)
		pngPath := fmt.Sprintf("carousel/slide-%02d.png", item.Order)
		pngContent, renderErr := renderSlidePNG(source, item, compactBaseURL(root), fonts, theme)
		if renderErr != nil {
			return Bundle{}, renderErr
		}
		files[pngPath] = pngContent
	}
	files["carousel/alt-text.md"] = renderAltText(source, root)
	caption, err := renderInstagramCaption(source, root)
	if err != nil {
		return Bundle{}, err
	}
	files["carousel/caption.md"] = caption
	files["carousel/index.html"] = renderCarouselIndex(source)

	manifest := Manifest{
		SchemaVersion:           kitSchemaVersion,
		Tool:                    kitToolVersion,
		EditionID:               source.EditionID,
		SourceGeneratedAt:       source.GeneratedAt,
		DistributionStatus:      "review_pending",
		ReviewRequired:          true,
		SendAuthorized:          false,
		SocialPublishAuthorized: false,
		BaseURL:                 root.String(),
		PrimaryGoal:             source.Email.PrimaryGoal,
		Email: ManifestEmail{
			Subject: source.Email.Subject,
			Preview: source.Email.Preview,
			CTAURL:  ctaURL,
		},
		Carousel: ManifestSocial{
			Theme:       theme.name,
			CaptionPath: "carousel/caption.md",
			AspectRatio: "4:5",
			Width:       1080,
			Height:      1350,
			SlideCount:  len(source.SocialCarousel.Slides),
			Formats:     []string{"png", "svg"},
		},
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		content := files[path]
		digest := sha256.Sum256(content)
		manifest.Files = append(manifest.Files, ManifestFile{
			Path:      path,
			MediaType: mediaType(path),
			Bytes:     len(content),
			SHA256:    hex.EncodeToString(digest[:]),
		})
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Bundle{}, fmt.Errorf("encode manifest: %w", err)
	}
	files["manifest.json"] = append(manifestJSON, '\n')
	return Bundle{Manifest: manifest, Files: files}, nil
}

func Write(outputDir string, bundle Bundle) error {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return errors.New("output directory is required")
	}
	abs, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolve output directory: %w", err)
	}
	if _, err := os.Stat(abs); err == nil {
		return fmt.Errorf("output directory already exists: %s", abs)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect output directory: %w", err)
	}
	parent := filepath.Dir(abs)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create output parent: %w", err)
	}
	tempDir, err := os.MkdirTemp(parent, ".tradegravity-distribution-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	defer os.RemoveAll(tempDir)

	paths := make([]string, 0, len(bundle.Files))
	for path := range bundle.Files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		if !fs.ValidPath(path) || path == "." {
			return fmt.Errorf("invalid bundle path %q", path)
		}
		target := filepath.Join(tempDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create bundle directory for %s: %w", path, err)
		}
		if err := os.WriteFile(target, bundle.Files[path], 0o644); err != nil {
			return fmt.Errorf("write bundle file %s: %w", path, err)
		}
	}
	if err := os.Rename(tempDir, abs); err != nil {
		return fmt.Errorf("publish distribution kit: %w", err)
	}
	return nil
}

func validateBriefing(source briefing) error {
	if source.SchemaVersion != "1.0" || source.Status != "ready" {
		return errors.New("briefing must be a ready schema 1.0 artifact")
	}
	if strings.TrimSpace(source.GeneratedAt) == "" || !safeIdentifier(source.EditionID) {
		return errors.New("briefing has invalid provenance")
	}
	if !source.ReviewRequired || source.Email.SendPolicy != "manual_review_required" || source.SocialCarousel.ReviewPolicy != "manual_review_required" {
		return errors.New("briefing must require manual review for email and social output")
	}
	if source.SocialCarousel.Format != "carousel_copy" || source.SocialCarousel.AspectRatio != "4:5" {
		return errors.New("briefing carousel must use the reviewed 4:5 copy contract")
	}
	if len(source.Signals) != len(readySignalKinds) || len(source.SocialCarousel.Slides) != len(readySlideRoles) {
		return errors.New("briefing must contain three signals and six slides")
	}
	if strings.TrimSpace(source.Email.Subject) == "" || strings.TrimSpace(source.Email.Preview) == "" || strings.TrimSpace(source.Email.Markdown) == "" || strings.TrimSpace(source.Email.CTALabel) == "" || strings.TrimSpace(source.Email.PrimaryGoal) == "" {
		return errors.New("briefing email contract is incomplete")
	}
	if !validRelativeHref(source.Email.CTAPath) || !validRelativeHref(source.EvidenceEntryPoint) {
		return errors.New("briefing contains an invalid evidence entry point")
	}
	for index, item := range source.Signals {
		if item.Kind != readySignalKinds[index] || strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Title) == "" || strings.TrimSpace(item.Summary) == "" {
			return fmt.Errorf("briefing signal %d does not match the ready contract", index+1)
		}
		if item.Period != source.LatestPeriod || item.PreviousPeriod != source.PreviousPeriod || len(item.Evidence) < 2 {
			return fmt.Errorf("briefing signal %s does not share the edition period and evidence", item.ID)
		}
		for _, href := range item.Evidence {
			if !validRelativeHref(href) {
				return fmt.Errorf("briefing signal %s has invalid evidence", item.ID)
			}
		}
	}
	for index, item := range source.SocialCarousel.Slides {
		if item.Order != index+1 || item.Role != readySlideRoles[index] || strings.TrimSpace(item.Headline) == "" || strings.TrimSpace(item.Body) == "" || len(item.Evidence) == 0 {
			return fmt.Errorf("briefing slide %d does not match the ready contract", index+1)
		}
		for _, href := range item.Evidence {
			if !validRelativeHref(href) {
				return fmt.Errorf("briefing slide %d has invalid evidence", item.Order)
			}
		}
	}
	return nil
}

func normalizeBaseURL(value string) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" {
		return nil, errors.New("base URL must be an absolute HTTPS URL")
	}
	loopback := parsed.Scheme == "http" && isLoopbackHost(parsed.Hostname())
	if parsed.Scheme != "https" && !loopback {
		return nil, errors.New("base URL must use HTTPS except for a loopback preview")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("base URL must not contain credentials, query parameters, or a fragment")
	}
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func safeIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func validRelativeHref(value string) bool {
	if !strings.HasPrefix(value, "./") || strings.Contains(value, "\\") || strings.ContainsAny(value, "\r\n") {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return false
	}
	for _, part := range strings.Split(parsed.Path, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

func resolveRootReference(root *url.URL, href string) (string, error) {
	if !validRelativeHref(href) {
		return "", errors.New("reference must be a safe same-origin relative path")
	}
	ref, err := url.Parse(href)
	if err != nil {
		return "", err
	}
	return root.ResolveReference(ref).String(), nil
}

func resolveEvidenceReference(root *url.URL, href string) string {
	if !validRelativeHref(href) {
		return ""
	}
	ref, _ := url.Parse(href)
	if ref.RawQuery != "" && (ref.Path == "." || ref.Path == "./" || ref.Path == "") {
		return root.ResolveReference(ref).String()
	}
	dataRef, _ := url.Parse("data/")
	dataRoot := root.ResolveReference(dataRef)
	return dataRoot.ResolveReference(ref).String()
}

func materializeMarkdown(markdown string, root *url.URL) string {
	base := strings.TrimSuffix(root.String(), "/")
	return strings.ReplaceAll(markdown, "{{BASE_URL}}", base)
}

func renderEmailHTML(source briefing, ctaURL string) []byte {
	var body strings.Builder
	body.WriteString("<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">")
	body.WriteString("<title>" + html.EscapeString(source.Email.Subject) + "</title></head>")
	body.WriteString("<body style=\"margin:0;background:#0b0d12;color:#f4f5f7;font-family:Arial,Helvetica,sans-serif\">")
	body.WriteString("<div style=\"display:none;max-height:0;overflow:hidden;opacity:0;color:transparent\">" + html.EscapeString(source.Email.Preview) + "</div>")
	body.WriteString("<table role=\"presentation\" width=\"100%\" cellspacing=\"0\" cellpadding=\"0\" style=\"background:#0b0d12\"><tr><td align=\"center\" style=\"padding:24px 12px\">")
	body.WriteString("<table role=\"presentation\" width=\"100%\" cellspacing=\"0\" cellpadding=\"0\" style=\"max-width:640px;background:#10141c;border:1px solid #29303b;border-radius:18px\">")
	body.WriteString("<tr><td style=\"padding:32px 32px 16px\"><div style=\"color:#e7d37c;font-size:12px;font-weight:700;letter-spacing:1.4px\">TRADEGRAVITY · REVIEWED DRAFT</div>")
	body.WriteString("<h1 style=\"margin:12px 0 8px;font-size:30px;line-height:1.2;color:#ffffff\">" + html.EscapeString(source.Title) + "</h1>")
	body.WriteString("<p style=\"margin:0;color:#aeb6c3;font-size:15px;line-height:1.6\">Observation window: " + html.EscapeString(source.LatestPeriod) + " vs " + html.EscapeString(source.PreviousPeriod) + "</p></td></tr>")
	for _, item := range source.Signals {
		body.WriteString("<tr><td style=\"padding:12px 32px\"><div style=\"background:#151a23;border:1px solid #29303b;border-radius:14px;padding:20px\">")
		body.WriteString("<h2 style=\"margin:0 0 8px;font-size:19px;line-height:1.35;color:#ffffff\">" + html.EscapeString(item.Title) + "</h2>")
		body.WriteString("<p style=\"margin:0;color:#c7cdd7;font-size:15px;line-height:1.65\">" + html.EscapeString(item.Summary) + "</p>")
		body.WriteString("<p style=\"margin:12px 0 0;color:#7f8998;font-size:12px;line-height:1.5\">" + html.EscapeString(item.PreviousPeriod) + " → " + html.EscapeString(item.Period) + " · cited evidence retained in the kit manifest</p></div></td></tr>")
	}
	body.WriteString("<tr><td align=\"center\" style=\"padding:24px 32px 12px\"><a href=\"" + html.EscapeString(ctaURL) + "\" style=\"display:inline-block;background:#5aa2ff;color:#07101d;text-decoration:none;font-weight:700;border-radius:10px;padding:14px 22px\">" + html.EscapeString(source.Email.CTALabel) + "</a></td></tr>")
	body.WriteString("<tr><td style=\"padding:12px 32px 32px;color:#7f8998;font-size:12px;line-height:1.6\"><p style=\"margin:0 0 8px\">" + html.EscapeString(source.Scope) + "</p><p style=\"margin:0 0 8px\">No tracking pixel is included. Sending remains unauthorized until the review checklist and subscriber-consent controls are completed.</p><p style=\"margin:0\"><a href=\"{{UNSUBSCRIBE_URL}}\" style=\"color:#9aa4b3;text-decoration:underline\">Unsubscribe</a></p></td></tr>")
	body.WriteString("</table></td></tr></table></body></html>\n")
	return []byte(body.String())
}

func renderEmailMarkdown(source briefing, root *url.URL) []byte {
	body := strings.TrimSpace(materializeMarkdown(source.Email.Markdown, root))
	body += "\n\n---\n\nUnsubscribe: {{UNSUBSCRIBE_URL}}\n"
	return []byte(body)
}

func renderSlideSVG(source briefing, item slide, root *url.URL, theme cardTheme) []byte {
	headlineLines := wrapText(item.Headline, 28, 4)
	bodyLines := wrapText(item.Body, 45, 6)
	evidenceLines := make([]string, 0, 3)
	for _, href := range item.Evidence {
		if len(evidenceLines) == 2 {
			break
		}
		evidenceLines = append(evidenceLines, truncateRunes(cardEvidenceLabel(href), 72))
	}
	evidenceLines = append(evidenceLines, truncateRunes("OPEN · "+compactBaseURL(root), 72))
	accent := theme.accent(item.Role)
	role := strings.ToUpper(strings.ReplaceAll(item.Role, "_", " "))

	var svg strings.Builder
	svg.WriteString("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"1080\" height=\"1350\" viewBox=\"0 0 1080 1350\" role=\"img\" aria-labelledby=\"title desc\">\n")
	svg.WriteString("<title id=\"title\">" + html.EscapeString(item.Headline) + "</title><desc id=\"desc\">" + html.EscapeString(item.Body) + "</desc>\n")
	svg.WriteString("<defs><linearGradient id=\"bg\" x1=\"0\" y1=\"0\" x2=\"1\" y2=\"1\"><stop offset=\"0\" stop-color=\"" + theme.backgroundStart + "\"/><stop offset=\"0.56\" stop-color=\"" + theme.backgroundMiddle + "\"/><stop offset=\"1\" stop-color=\"" + theme.backgroundEnd + "\"/></linearGradient></defs>\n")
	svg.WriteString("<rect width=\"1080\" height=\"1350\" fill=\"url(#bg)\"/><circle cx=\"940\" cy=\"120\" r=\"230\" fill=\"" + accent + "\" opacity=\"0.10\"/><circle cx=\"80\" cy=\"1260\" r=\"250\" fill=\"" + theme.decoration + "\" opacity=\"0.07\"/>\n")
	svg.WriteString(fmt.Sprintf("<rect x=\"64\" y=\"62\" width=\"952\" height=\"1226\" rx=\"28\" fill=\"none\" stroke=\"%s\" stroke-opacity=\"%.3f\"/>\n", theme.frame, float64(theme.frameAlpha)/255))
	svg.WriteString("<text x=\"88\" y=\"126\" fill=\"" + theme.header + "\" font-family=\"Segoe UI,Arial,sans-serif\" font-size=\"24\" font-weight=\"700\" letter-spacing=\"3\">TRADEGRAVITY · US–CHINA CHIP LENS</text>\n")
	svg.WriteString(fmt.Sprintf("<text x=\"992\" y=\"126\" text-anchor=\"end\" fill=\"%s\" font-family=\"Segoe UI,Arial,sans-serif\" font-size=\"22\">%02d / %02d</text>\n", theme.muted, item.Order, len(source.SocialCarousel.Slides)))
	svg.WriteString("<rect x=\"88\" y=\"178\" width=\"" + fmt.Sprint(maxInt(210, 40+len([]rune(role))*15)) + "\" height=\"48\" rx=\"24\" fill=\"" + accent + "\" fill-opacity=\"0.16\" stroke=\"" + accent + "\" stroke-opacity=\"0.65\"/>")
	svg.WriteString("<text x=\"112\" y=\"210\" fill=\"" + accent + "\" font-family=\"Segoe UI,Arial,sans-serif\" font-size=\"20\" font-weight=\"700\" letter-spacing=\"2\">" + html.EscapeString(role) + "</text>\n")
	writeSVGLines(&svg, headlineLines, 88, 330, 64, 78, "700", theme.headline)
	writeSVGLines(&svg, bodyLines, 88, 700, 34, 53, "400", theme.body)
	svg.WriteString(fmt.Sprintf("<rect x=\"88\" y=\"1060\" width=\"904\" height=\"154\" rx=\"18\" fill=\"%s\" fill-opacity=\"%.3f\" stroke=\"%s\" stroke-opacity=\"%.3f\"/>\n", theme.panel, float64(theme.panelAlpha)/255, theme.panelBorder, float64(theme.panelBorderAlpha)/255))
	svg.WriteString("<text x=\"116\" y=\"1102\" fill=\"" + theme.evidenceTitle + "\" font-family=\"Segoe UI,Arial,sans-serif\" font-size=\"18\" font-weight=\"700\" letter-spacing=\"2\">EVIDENCE</text>\n")
	writeSVGLines(&svg, evidenceLines, 116, 1144, 20, 29, "400", theme.muted)
	svg.WriteString("<text x=\"88\" y=\"1260\" fill=\"" + theme.footer + "\" font-family=\"Segoe UI,Arial,sans-serif\" font-size=\"18\">Reviewed draft · descriptive customs evidence · not investment advice</text>\n")
	svg.WriteString("</svg>\n")
	return []byte(svg.String())
}

func renderAltText(source briefing, root *url.URL) []byte {
	var result strings.Builder
	result.WriteString("# Carousel alt text\n\n")
	result.WriteString("Review and edit this text with each final visual before publishing.\n\n")
	for _, item := range source.SocialCarousel.Slides {
		result.WriteString(fmt.Sprintf("## Slide %d — %s\n\n", item.Order, item.Headline))
		result.WriteString(item.Headline + ". " + item.Body + "\n\n")
		result.WriteString("Evidence:\n")
		for _, href := range item.Evidence {
			result.WriteString("- " + resolveEvidenceReference(root, href) + "\n")
		}
		result.WriteString("\n")
	}
	return []byte(result.String())
}

func renderInstagramCaption(source briefing, root *url.URL) ([]byte, error) {
	plain := func(value string) string { return strings.Join(strings.Fields(value), " ") }
	var caption strings.Builder
	caption.WriteString(plain(source.Title) + "\n")
	caption.WriteString(plain(source.LatestPeriod) + " vs " + plain(source.PreviousPeriod) + "\n\n")
	for _, item := range source.Signals {
		caption.WriteString("• " + plain(item.Title) + " — " + plain(item.Summary) + "\n")
	}
	caption.WriteString("\nExplore the cited evidence and methodology:\n")
	caption.WriteString(resolveEvidenceReference(root, source.EvidenceEntryPoint) + "\n\n")
	caption.WriteString("Scope note: descriptive customs evidence; not a physical shipment route, causal claim, or investment recommendation.\n\n")
	caption.WriteString("#TradeGravity #Semiconductors #SupplyChain #USChinaTrade\n")
	result := caption.String()
	if len([]rune(result)) > captionRuneLimit {
		return nil, fmt.Errorf("Instagram caption draft exceeds the %d-rune editorial ceiling", captionRuneLimit)
	}
	return []byte(result), nil
}

func renderCarouselIndex(source briefing) []byte {
	var page strings.Builder
	page.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>" + html.EscapeString(source.EditionID) + " carousel review</title>")
	page.WriteString("<style>body{margin:0;background:#0b0d12;color:#fff;font:16px system-ui;padding:24px}main{max-width:1120px;margin:auto}h1{font-size:24px}a{color:#8fc1ff}.notice{color:#e7d37c}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:18px}figure{margin:0}img{display:block;width:100%;height:auto;border:1px solid #29303b;border-radius:12px}figcaption{padding:8px 2px;color:#9aa4b3;font-size:13px}</style></head><body><main>")
	page.WriteString("<p class=\"notice\">Review pending · this page does not publish or send anything.</p><h1>" + html.EscapeString(source.Title) + "</h1><p><a href=\"caption.md\">Open the Instagram caption draft</a> · <a href=\"alt-text.md\">Open alt text</a></p><div class=\"grid\">")
	for _, item := range source.SocialCarousel.Slides {
		page.WriteString(fmt.Sprintf("<figure><img src=\"slide-%02d.png\" width=\"1080\" height=\"1350\" alt=\"%s\"><figcaption>Slide %d · %s · PNG upload asset</figcaption></figure>", item.Order, html.EscapeString(item.Headline+". "+item.Body), item.Order, html.EscapeString(item.Role)))
	}
	page.WriteString("</div></main></body></html>\n")
	return []byte(page.String())
}

func renderReviewChecklist(source briefing, root *url.URL) []byte {
	var review strings.Builder
	review.WriteString("# Distribution review — " + source.EditionID + "\n\n")
	review.WriteString("Status: **review pending**. Generating this kit does not authorize email delivery or social publication.\n\n")
	review.WriteString("- Source publication: `" + source.GeneratedAt + "`\n")
	review.WriteString("- Observation window: `" + source.PreviousPeriod + "` → `" + source.LatestPeriod + "`\n")
	review.WriteString("- Evidence entry point: " + resolveEvidenceReference(root, source.EvidenceEntryPoint) + "\n\n")
	review.WriteString("## Editorial and evidence\n\n- [ ] Periods, values, direction, and units match the cited JSON.\n- [ ] Every evidence URL opens and the source scope is still accurate.\n- [ ] Monthly movement is not described as publication revision, causality, capacity, or a physical route.\n- [ ] Subject, preview, one primary CTA, and mobile rendering were reviewed.\n- [ ] Carousel text and supplied alt text remain legible at feed size.\n- [ ] `carousel/caption.md`, tags, evidence link, and scope note were reviewed together with the final cards.\n- [ ] A named editor approved the final copy and exported images.\n\n")
	review.WriteString("## Delivery and privacy\n\n- [ ] Recipients completed double opt-in for this publication.\n- [ ] The `{{UNSUBSCRIBE_URL}}` placeholder, one-click unsubscribe headers, and suppression handling were tested.\n- [ ] Sender identity, SPF, DKIM, and DMARC are configured.\n- [ ] Bounce, complaint, retention, and deletion procedures are documented.\n- [ ] No subscriber addresses, provider credentials, or tracking secrets are stored in this kit.\n\n")
	review.WriteString("## Approval\n\n- Editor: ____________________\n- Date: ______________________\n- Approved channels: ____________________\n- Non-sensitive audience label: ____________________\n- Final asset hashes recorded: [ ]\n\nAfter every box above is complete, create the content-release record with an explicit UTC time:\n\n```bash\ngo run ./cmd/distribution-approval \\\n  -kit distribution-kit \\\n  -reviewer YOUR_HANDLE \\\n  -audience consented-internal-pilot \\\n  -channels email,instagram \\\n  -approved-at 2026-07-17T12:00:00Z \\\n  -attest-reviewed\n```\n\nThis records content approval only. It does not certify subscriber consent, provider readiness, or automatic publishing permission.\n")
	return []byte(review.String())
}

func wrapText(value string, maxRunes, maxLines int) []string {
	words := strings.Fields(value)
	if len(words) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, maxLines)
	current := ""
	for _, word := range words {
		candidate := word
		if current != "" {
			candidate = current + " " + word
		}
		if len([]rune(candidate)) <= maxRunes {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		current = truncateRunes(word, maxRunes)
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = truncateRunes(strings.TrimSpace(lines[maxLines-1])+" …", maxRunes)
	}
	return lines
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func writeSVGLines(output *strings.Builder, lines []string, x, y, size, lineHeight int, weight, fill string) {
	for index, line := range lines {
		output.WriteString(fmt.Sprintf("<text x=\"%d\" y=\"%d\" fill=\"%s\" font-family=\"Segoe UI,Arial,sans-serif\" font-size=\"%d\" font-weight=\"%s\">%s</text>\n", x, y+index*lineHeight, fill, size, weight, html.EscapeString(line)))
	}
}

func cardEvidenceLabel(href string) string {
	parsed, err := url.Parse(href)
	if err != nil {
		return "EVIDENCE · review manifest"
	}
	if parsed.RawQuery != "" && (parsed.Path == "." || parsed.Path == "./" || parsed.Path == "") {
		return "APP · ?" + parsed.RawQuery
	}
	path := strings.TrimPrefix(parsed.Path, "./")
	return "DATA · " + path
}

func compactBaseURL(root *url.URL) string {
	return root.Host + strings.TrimSuffix(root.EscapedPath(), "/")
}

func mediaType(path string) string {
	switch filepath.Ext(path) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json"
	case ".md":
		return "text/markdown; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	default:
		return "text/plain; charset=utf-8"
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
