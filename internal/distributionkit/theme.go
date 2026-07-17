package distributionkit

import (
	"errors"
	"strings"
)

const (
	ThemeIntelligenceDark = "intelligence-dark"
	ThemeEditorialLight   = "editorial-light"
)

type BuildOptions struct {
	Theme string
}

type cardTheme struct {
	name                                             string
	backgroundStart, backgroundMiddle, backgroundEnd string
	decoration, frame, header, muted, headline, body string
	panel, panelBorder, evidenceTitle, footer        string
	frameAlpha, panelAlpha, panelBorderAlpha         uint8
	accents                                          map[string]string
}

func resolveTheme(name string) (cardTheme, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = ThemeIntelligenceDark
	}
	switch name {
	case ThemeIntelligenceDark:
		return cardTheme{
			name: name, backgroundStart: "#111827", backgroundMiddle: "#0b0d12", backgroundEnd: "#161117",
			decoration: "#5aa2ff", frame: "#ffffff", header: "#e7d37c", muted: "#9aa4b3",
			headline: "#ffffff", body: "#c7cdd7", panel: "#ffffff", panelBorder: "#ffffff",
			evidenceTitle: "#e7d37c", footer: "#7f8998", frameAlpha: 33, panelAlpha: 9, panelBorderAlpha: 26,
			accents: map[string]string{"scale": "#5aa2ff", "anchor_balance": "#86e7b0", "product": "#ff8a68", "cta": "#74b3ff", "default": "#e7d37c"},
		}, nil
	case ThemeEditorialLight:
		return cardTheme{
			name: name, backgroundStart: "#f7f3ea", backgroundMiddle: "#ffffff", backgroundEnd: "#eee8dc",
			decoration: "#557da6", frame: "#26323f", header: "#895f25", muted: "#5b6571",
			headline: "#17202a", body: "#44505c", panel: "#ffffff", panelBorder: "#26323f",
			evidenceTitle: "#895f25", footer: "#5b6571", frameAlpha: 46, panelAlpha: 235, panelBorderAlpha: 31,
			accents: map[string]string{"scale": "#356b99", "anchor_balance": "#28745b", "product": "#a34d36", "cta": "#315f86", "default": "#895f25"},
		}, nil
	default:
		return cardTheme{}, errors.New("carousel theme must be intelligence-dark or editorial-light")
	}
}

func (theme cardTheme) accent(role string) string {
	if value := theme.accents[role]; value != "" {
		return value
	}
	return theme.accents["default"]
}
