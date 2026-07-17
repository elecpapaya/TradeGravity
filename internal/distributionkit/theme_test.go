package distributionkit

import (
	"fmt"
	"math"
	"testing"
)

func TestCardThemesMeetTextContrastContract(t *testing.T) {
	for _, name := range []string{ThemeIntelligenceDark, ThemeEditorialLight} {
		t.Run(name, func(t *testing.T) {
			theme, err := resolveTheme(name)
			if err != nil {
				t.Fatal(err)
			}
			backgrounds := []string{theme.backgroundStart, theme.backgroundMiddle, theme.backgroundEnd}
			foregrounds := map[string]string{
				"header": theme.header, "counter/evidence": theme.muted, "headline": theme.headline,
				"body": theme.body, "evidence title": theme.evidenceTitle, "footer": theme.footer,
			}
			for role, foreground := range foregrounds {
				for _, background := range backgrounds {
					if ratio := contrastRatio(foreground, background); ratio < 4.5 {
						t.Errorf("%s on %s contrast %.2f:1, want at least 4.5:1", role, background, ratio)
					}
				}
			}
			for role, foreground := range theme.accents {
				for _, background := range backgrounds {
					pill := blendHex(foreground, background, 0.16)
					if ratio := contrastRatio(foreground, pill); ratio < 3.0 {
						t.Errorf("role label %s on blended pill %s contrast %.2f:1, want at least 3:1", role, pill, ratio)
					}
				}
			}
		})
	}
}

func contrastRatio(first, second string) float64 {
	a, b := relativeLuminance(parseTestHex(first)), relativeLuminance(parseTestHex(second))
	if a < b {
		a, b = b, a
	}
	return (a + 0.05) / (b + 0.05)
}

func relativeLuminance(value [3]float64) float64 {
	linear := func(channel float64) float64 {
		channel /= 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(value[0]) + 0.7152*linear(value[1]) + 0.0722*linear(value[2])
}

func blendHex(foreground, background string, alpha float64) string {
	front, back := parseTestHex(foreground), parseTestHex(background)
	return fmt.Sprintf("#%02x%02x%02x",
		int(math.Round(front[0]*alpha+back[0]*(1-alpha))),
		int(math.Round(front[1]*alpha+back[1]*(1-alpha))),
		int(math.Round(front[2]*alpha+back[2]*(1-alpha))))
}

func parseTestHex(value string) [3]float64 {
	var red, green, blue uint8
	if _, err := fmt.Sscanf(value, "#%02x%02x%02x", &red, &green, &blue); err != nil {
		panic(err)
	}
	return [3]float64{float64(red), float64(green), float64(blue)}
}
