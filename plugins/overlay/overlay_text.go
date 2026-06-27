package overlay

import (
	"strings"
)

const (
	maxOverlayBaseWidth = 520
	maxOverlayLines     = 3
)

// FormatOverlayText wraps live transcript text to at most maxOverlayLines lines.
func FormatOverlayText(text string, scale float64) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}
	if isASCIILabel(text) {
		return text
	}
	lines := wrapOverlayTextLines(text, overlayMaxTextWidth(scale), scale)
	if len(lines) == 0 {
		return text
	}
	return strings.Join(lines, "\n")
}

func overlayMaxTextWidth(scale float64) int {
	dotSize := scaledInt(14, scale)
	gap := scaledInt(14, scale)
	pad := scaledInt(14, scale)
	maxTextW := scaledInt(maxOverlayBaseWidth, scale) - dotSize - gap - pad*2
	minW := scaledInt(80, scale)
	if maxTextW < minW {
		return minW
	}
	return maxTextW
}

func overlayTextLines(label string) []string {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil
	}
	return strings.Split(label, "\n")
}
