package overlay

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

var (
	overlayFontMu    sync.Mutex
	overlayFontCache map[float64]font.Face
)

func wrapOverlayTextLines(text string, maxWidth int, scale float64) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if isASCIILabel(text) {
		return []string{text}
	}
	face, err := overlayFontFace(scale)
	if err != nil {
		return []string{fallbackASCIILabel(text)}
	}
	rs := []rune(text)
	var lines []string
	var line []rune
	for _, r := range rs {
		if r == '\n' {
			if len(line) > 0 {
				lines = append(lines, string(line))
				line = nil
			}
			continue
		}
		candidate := append(line, r)
		w := font.MeasureString(face, string(candidate)).Ceil()
		if w > maxWidth && len(line) > 0 {
			lines = append(lines, string(line))
			line = []rune{r}
		} else {
			line = candidate
		}
	}
	if len(line) > 0 {
		lines = append(lines, string(line))
	}
	if len(lines) > maxOverlayLines {
		lines = lines[len(lines)-maxOverlayLines:]
	}
	return lines
}

func overlayTextLineHeight(label string, scale float64) int {
	if isASCIILabel(label) {
		return 7 * scaledInt(3, scale)
	}
	face, err := overlayFontFace(scale)
	if err != nil {
		return scaledInt(18, scale)
	}
	m := face.Metrics()
	return m.Ascent.Ceil() + m.Descent.Ceil() + scaledInt(2, scale)
}

func measureOverlayLine(text string, scale float64) int {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0
	}
	if isASCIILabel(text) {
		return bitmapTextWidth(text, scaledInt(3, scale))
	}
	face, err := overlayFontFace(scale)
	if err != nil {
		return len([]rune(text)) * scaledInt(8, scale)
	}
	return font.MeasureString(face, text).Ceil()
}

func drawOverlayTextLines(canvas *argbCanvas, x, y int, lines []string, scale float64, c rgba) {
	if len(lines) == 0 {
		return
	}
	lineHeight := overlayTextLineHeight(lines[0], scale)
	for i, line := range lines {
		drawOverlayText(canvas, x, y+i*lineHeight, line, scale, c)
	}
}

func drawOverlayText(canvas *argbCanvas, x, y int, text string, scale float64, c rgba) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if isASCIILabel(text) {
		canvas.drawText(x, y, text, scaledInt(3, scale), c)
		return
	}
	face, err := overlayFontFace(scale)
	if err != nil {
		canvas.drawText(x, y, fallbackASCIILabel(text), scaledInt(3, scale), c)
		return
	}
	d := &font.Drawer{
		Dst:  &canvasImageAdapter{canvas: canvas},
		Src:  image.NewUniform(color.RGBA{R: c.r, G: c.g, B: c.b, A: c.a}),
		Face: face,
		Dot:  fixed.P(x, y+face.Metrics().Ascent.Ceil()),
	}
	d.DrawString(text)
}

func isASCIILabel(text string) bool {
	if text == "" {
		return true
	}
	for _, r := range text {
		if r > 127 {
			return false
		}
	}
	return len(text) <= 4
}

func fallbackASCIILabel(text string) string {
	if len([]rune(text)) > 0 {
		return "..."
	}
	return text
}

func overlayFontFace(scale float64) (font.Face, error) {
	if scale <= 0 {
		scale = 1
	}
	size := 14 * scale
	overlayFontMu.Lock()
	defer overlayFontMu.Unlock()
	if overlayFontCache == nil {
		overlayFontCache = make(map[float64]font.Face)
	}
	if face, ok := overlayFontCache[size]; ok {
		return face, nil
	}
	face, err := loadOverlayFont(size)
	if err != nil {
		return nil, err
	}
	overlayFontCache[size] = face
	return face, nil
}

func loadOverlayFont(size float64) (font.Face, error) {
	paths := []string{
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Bold.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansSC-Regular.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/System/Library/Fonts/PingFang.ttc",
		"/System/Library/Fonts/STHeiti Light.ttc",
		"/Library/Fonts/Arial Unicode.ttf",
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.HasSuffix(strings.ToLower(path), ".ttc") {
			coll, err := opentype.ParseCollection(data)
			if err != nil {
				continue
			}
			for i := 0; i < coll.NumFonts(); i++ {
				fnt, err := coll.Font(i)
				if err != nil {
					continue
				}
				face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
					Size:    size,
					DPI:     96,
					Hinting: font.HintingFull,
				})
				if err == nil {
					return face, nil
				}
			}
			continue
		}
		fnt, err := opentype.Parse(data)
		if err != nil {
			continue
		}
		face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
			Size:    size,
			DPI:     96,
			Hinting: font.HintingFull,
		})
		if err == nil {
			return face, nil
		}
	}
	return nil, fmt.Errorf("no overlay font found")
}

type canvasImageAdapter struct {
	canvas *argbCanvas
}

func (a *canvasImageAdapter) ColorModel() color.Model { return color.RGBAModel }

func (a *canvasImageAdapter) Bounds() image.Rectangle {
	return image.Rect(0, 0, a.canvas.w, a.canvas.h)
}

func (a *canvasImageAdapter) At(x, y int) color.Color {
	if x < 0 || y < 0 || x >= a.canvas.w || y >= a.canvas.h {
		return color.RGBA{}
	}
	i := (y*a.canvas.w + x) * 4
	return color.RGBA{
		R: a.canvas.data[i+2],
		G: a.canvas.data[i+1],
		B: a.canvas.data[i+0],
		A: a.canvas.data[i+3],
	}
}

func (a *canvasImageAdapter) Set(x, y int, c color.Color) {
	if x < 0 || y < 0 || x >= a.canvas.w || y >= a.canvas.h {
		return
	}
	r, g, b, alpha := c.RGBA()
	if alpha == 0 {
		return
	}
	cr := rgba{
		r: uint8(r >> 8),
		g: uint8(g >> 8),
		b: uint8(b >> 8),
		a: uint8(alpha >> 8),
	}
	a.canvas.blendPixel(x, y, cr, cr.a)
}

func scaledInt(v int, scale float64) int {
	if scale <= 0 {
		scale = 1
	}
	n := int(float64(v)*scale + 0.5)
	if n < 1 {
		return 1
	}
	return n
}
