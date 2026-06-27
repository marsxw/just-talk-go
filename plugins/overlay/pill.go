//go:build linux && !no_x11

package overlay

import "strings"

type pillLayout struct {
	width      int
	height     int
	dotSize    int
	dotX       int
	dotY       int
	textX      int
	textY      int
	lineHeight int
	lines      []string
	hasText    bool
}

func layoutPill(label string, scale float64) pillLayout {
	dotSize := scaledInt(14, scale)
	gap := scaledInt(14, scale)
	pad := scaledInt(14, scale)
	maxW := scaledInt(maxOverlayBaseWidth, scale)

	lines := nonEmptyOverlayLines(label)
	hasText := len(lines) > 0

	if !hasText {
		width := scaledInt(basePillW, scale)
		height := pillContentHeight(1, "", scale)
		dotX := pad
		dotY := (height - dotSize) / 2
		return pillLayout{
			width: width, height: height,
			dotSize: dotSize, dotX: dotX, dotY: dotY,
			hasText: false,
		}
	}

	lineHeight := overlayTextLineHeight(lines[0], scale)
	textW := 0
	for _, line := range lines {
		if w := measureOverlayLine(line, scale); w > textW {
			textW = w
		}
	}
	textH := lineHeight * len(lines)

	contentW := dotSize + gap + textW
	width := scaledInt(basePillW, scale)
	if len(lines) > 1 || textW >= overlayMaxTextWidth(scale)-scaledInt(4, scale) {
		width = maxW
	} else if contentW+pad*2 > width {
		width = contentW + pad*2
	}
	if width > maxW {
		width = maxW
	}

	height := pillContentHeight(len(lines), lines[0], scale)

	dotX := pad
	dotY := (height - dotSize) / 2
	textX := dotX + dotSize + gap
	textY := (height - textH) / 2
	if textY < pad {
		textY = pad
	}

	return pillLayout{
		width: width, height: height,
		dotSize: dotSize, dotX: dotX, dotY: dotY,
		textX: textX, textY: textY, lineHeight: lineHeight,
		lines: lines, hasText: true,
	}
}

func nonEmptyOverlayLines(label string) []string {
	var lines []string
	for _, line := range overlayTextLines(label) {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// pillContentHeight returns the pill height for the given number of text lines.
// An empty firstLine uses CJK metrics so the idle pill matches one live caption line.
func pillContentHeight(lineCount int, firstLine string, scale float64) int {
	pad := scaledInt(14, scale)
	height := scaledInt(basePillH, scale)
	if lineCount < 1 {
		lineCount = 1
	}
	if firstLine == "" {
		firstLine = "中"
	}
	lineHeight := overlayTextLineHeight(firstLine, scale)
	if textH := lineHeight*lineCount + pad*2; textH > height {
		height = textH
	}
	return height
}

func drawPillCanvas(w, h int, label string, color statusColor, scale float64) *argbCanvas {
	layout := layoutPill(label, scale)
	if w < layout.width {
		w = layout.width
	}
	if h < layout.height {
		h = layout.height
	}
	p := newARGBCanvas(w, h)
	bg := rgba{20, 20, 20, 215}
	fg := rgba{245, 245, 245, 255}
	dot := rgba{uint8(color.R >> 8), uint8(color.G >> 8), uint8(color.B >> 8), 255}
	radius := h / 2
	if radius > w/2 {
		radius = w / 2
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if coverage := roundedRectCoverage(x, y, w, h, radius); coverage > 0 {
				c := bg
				c.a = uint8(uint16(c.a) * uint16(coverage) / 255)
				p.setPixel(x, y, c)
			}
		}
	}
	p.fillCircleAA(layout.dotX+layout.dotSize/2, layout.dotY+layout.dotSize/2, layout.dotSize/2, dot)
	if layout.hasText {
		drawOverlayTextLines(p, layout.textX, layout.textY, layout.lines, scale, fg)
	}
	return p
}
