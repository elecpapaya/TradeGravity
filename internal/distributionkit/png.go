package distributionkit

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	cardWidth  = 1080
	cardHeight = 1350
)

type slideFonts struct {
	header        font.Face
	counter       font.Face
	role          font.Face
	headline      font.Face
	body          font.Face
	evidenceTitle font.Face
	evidence      font.Face
	footer        font.Face
	closers       []io.Closer
}

func newSlideFonts() (slideFonts, error) {
	regular, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return slideFonts{}, fmt.Errorf("parse embedded regular font: %w", err)
	}
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return slideFonts{}, fmt.Errorf("parse embedded bold font: %w", err)
	}

	var result slideFonts
	create := func(target *font.Face, source *opentype.Font, size float64) error {
		face, faceErr := opentype.NewFace(source, &opentype.FaceOptions{
			Size:    size,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if faceErr != nil {
			return faceErr
		}
		*target = face
		if closer, ok := face.(io.Closer); ok {
			result.closers = append(result.closers, closer)
		}
		return nil
	}
	faces := []struct {
		target *font.Face
		source *opentype.Font
		size   float64
	}{
		{&result.header, bold, 24},
		{&result.counter, regular, 22},
		{&result.role, bold, 20},
		{&result.headline, bold, 64},
		{&result.body, regular, 34},
		{&result.evidenceTitle, bold, 18},
		{&result.evidence, regular, 20},
		{&result.footer, regular, 18},
	}
	for _, spec := range faces {
		if err := create(spec.target, spec.source, spec.size); err != nil {
			result.Close()
			return slideFonts{}, fmt.Errorf("create embedded font face: %w", err)
		}
	}
	return result, nil
}

func (fonts slideFonts) Close() {
	for _, closer := range fonts.closers {
		_ = closer.Close()
	}
}

func renderSlidePNG(source briefing, item slide, rootURL string, fonts slideFonts, theme cardTheme) ([]byte, error) {
	canvas := image.NewRGBA(image.Rect(0, 0, cardWidth, cardHeight))
	drawCardBackground(canvas, theme)

	accent := mustHexColor(theme.accent(item.Role), 255)
	drawCircle(canvas, 940, 120, 230, withAlpha(accent, 26))
	drawCircle(canvas, 80, 1260, 250, mustHexColor(theme.decoration, 18))
	drawRoundedRect(canvas, 64, 62, 952, 1226, 28, color.RGBA{}, mustHexColor(theme.frame, theme.frameAlpha), 2)

	drawCardText(canvas, fonts.header, 88, 126, mustHexColor(theme.header, 255), "TRADEGRAVITY · US–CHINA CHIP LENS")
	counter := fmt.Sprintf("%02d / %02d", item.Order, len(source.SocialCarousel.Slides))
	drawRightAlignedText(canvas, fonts.counter, 992, 126, mustHexColor(theme.muted, 255), counter)

	role := strings.ToUpper(strings.ReplaceAll(item.Role, "_", " "))
	pillWidth := maxInt(210, 40+len([]rune(role))*15)
	drawRoundedRect(canvas, 88, 178, pillWidth, 48, 24, withAlpha(accent, 41), withAlpha(accent, 166), 2)
	drawCardText(canvas, fonts.role, 112, 210, accent, role)

	headlineLines := wrapTextForFace(item.Headline, fonts.headline, 880, 4)
	bodyLines := wrapTextForFace(item.Body, fonts.body, 880, 6)
	drawCardLines(canvas, fonts.headline, headlineLines, 88, 330, 78, mustHexColor(theme.headline, 255))
	drawCardLines(canvas, fonts.body, bodyLines, 88, 700, 53, mustHexColor(theme.body, 255))

	drawRoundedRect(canvas, 88, 1060, 904, 154, 18, mustHexColor(theme.panel, theme.panelAlpha), mustHexColor(theme.panelBorder, theme.panelBorderAlpha), 2)
	drawCardText(canvas, fonts.evidenceTitle, 116, 1102, mustHexColor(theme.evidenceTitle, 255), "EVIDENCE")
	evidenceLines := make([]string, 0, 3)
	for _, href := range item.Evidence {
		if len(evidenceLines) == 2 {
			break
		}
		evidenceLines = append(evidenceLines, truncateToWidth(cardEvidenceLabel(href), fonts.evidence, 840))
	}
	evidenceLines = append(evidenceLines, truncateToWidth("OPEN · "+rootURL, fonts.evidence, 840))
	drawCardLines(canvas, fonts.evidence, evidenceLines, 116, 1144, 29, mustHexColor(theme.muted, 255))

	drawCardText(canvas, fonts.footer, 88, 1260, mustHexColor(theme.footer, 255), "Reviewed draft · descriptive customs evidence · not investment advice")

	var output bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&output, canvas); err != nil {
		return nil, fmt.Errorf("encode slide PNG: %w", err)
	}
	return output.Bytes(), nil
}

func drawCardBackground(target *image.RGBA, theme cardTheme) {
	start := mustHexColor(theme.backgroundStart, 255)
	middle := mustHexColor(theme.backgroundMiddle, 255)
	end := mustHexColor(theme.backgroundEnd, 255)
	for y := 0; y < cardHeight; y++ {
		for x := 0; x < cardWidth; x++ {
			progress := float64(x+y) / float64(cardWidth+cardHeight-2)
			if progress <= 0.56 {
				target.SetRGBA(x, y, interpolateColor(start, middle, progress/0.56))
				continue
			}
			target.SetRGBA(x, y, interpolateColor(middle, end, (progress-0.56)/0.44))
		}
	}
}

func interpolateColor(from, to color.RGBA, progress float64) color.RGBA {
	channel := func(a, b uint8) uint8 {
		return uint8(float64(a) + (float64(b)-float64(a))*progress)
	}
	return color.RGBA{R: channel(from.R, to.R), G: channel(from.G, to.G), B: channel(from.B, to.B), A: 255}
}

func drawCircle(target *image.RGBA, centerX, centerY, radius int, fill color.RGBA) {
	for y := maxInt(0, centerY-radius); y < minInt(cardHeight, centerY+radius+1); y++ {
		for x := maxInt(0, centerX-radius); x < minInt(cardWidth, centerX+radius+1); x++ {
			dx, dy := x-centerX, y-centerY
			if dx*dx+dy*dy <= radius*radius {
				blendPixel(target, x, y, fill)
			}
		}
	}
}

func drawRoundedRect(target *image.RGBA, x, y, width, height, radius int, fill, stroke color.RGBA, strokeWidth int) {
	for py := y; py < y+height; py++ {
		for px := x; px < x+width; px++ {
			if !insideRoundedRect(px, py, x, y, width, height, radius) {
				continue
			}
			paint := stroke
			inner := strokeWidth > 0 && insideRoundedRect(px, py, x+strokeWidth, y+strokeWidth, width-strokeWidth*2, height-strokeWidth*2, maxInt(0, radius-strokeWidth))
			if strokeWidth == 0 || inner {
				paint = fill
			}
			blendPixel(target, px, py, paint)
		}
	}
}

func insideRoundedRect(px, py, x, y, width, height, radius int) bool {
	if width <= 0 || height <= 0 || px < x || px >= x+width || py < y || py >= y+height {
		return false
	}
	if radius <= 0 || (px >= x+radius && px < x+width-radius) || (py >= y+radius && py < y+height-radius) {
		return true
	}
	cx := x + radius
	if px >= x+width-radius {
		cx = x + width - radius - 1
	}
	cy := y + radius
	if py >= y+height-radius {
		cy = y + height - radius - 1
	}
	dx, dy := px-cx, py-cy
	return dx*dx+dy*dy <= radius*radius
}

func blendPixel(target *image.RGBA, x, y int, source color.RGBA) {
	if source.A == 0 {
		return
	}
	if source.A == 255 {
		target.SetRGBA(x, y, source)
		return
	}
	destination := target.RGBAAt(x, y)
	alpha := uint32(source.A)
	inverse := uint32(255 - source.A)
	target.SetRGBA(x, y, color.RGBA{
		R: uint8((uint32(source.R)*alpha + uint32(destination.R)*inverse) / 255),
		G: uint8((uint32(source.G)*alpha + uint32(destination.G)*inverse) / 255),
		B: uint8((uint32(source.B)*alpha + uint32(destination.B)*inverse) / 255),
		A: 255,
	})
}

func drawCardText(target *image.RGBA, face font.Face, x, baseline int, fill color.RGBA, value string) {
	drawer := font.Drawer{
		Dst:  target,
		Src:  image.NewUniform(fill),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}
	drawer.DrawString(value)
}

func drawRightAlignedText(target *image.RGBA, face font.Face, right, baseline int, fill color.RGBA, value string) {
	width := font.MeasureString(face, value).Ceil()
	drawCardText(target, face, right-width, baseline, fill, value)
}

func drawCardLines(target *image.RGBA, face font.Face, lines []string, x, baseline, lineHeight int, fill color.RGBA) {
	for index, line := range lines {
		drawCardText(target, face, x, baseline+index*lineHeight, fill, line)
	}
}

func wrapTextForFace(value string, face font.Face, maxWidth, maxLines int) []string {
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
		if font.MeasureString(face, candidate).Ceil() <= maxWidth {
			current = candidate
			continue
		}
		if current != "" {
			lines = append(lines, current)
		}
		current = truncateToWidth(word, face, maxWidth)
	}
	if current != "" {
		lines = append(lines, current)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		lines[maxLines-1] = truncateToWidth(strings.TrimSpace(lines[maxLines-1])+" …", face, maxWidth)
	}
	return lines
}

func truncateToWidth(value string, face font.Face, maxWidth int) string {
	if font.MeasureString(face, value).Ceil() <= maxWidth {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 {
		candidate := strings.TrimSpace(string(runes)) + "…"
		if font.MeasureString(face, candidate).Ceil() <= maxWidth {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return "…"
}

func mustHexColor(value string, alpha uint8) color.RGBA {
	value = strings.TrimPrefix(value, "#")
	if len(value) != 6 {
		panic("invalid card color")
	}
	var red, green, blue uint8
	if _, err := fmt.Sscanf(value, "%02x%02x%02x", &red, &green, &blue); err != nil {
		panic("invalid card color")
	}
	return color.RGBA{R: red, G: green, B: blue, A: alpha}
}

func withAlpha(value color.RGBA, alpha uint8) color.RGBA {
	value.A = alpha
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
