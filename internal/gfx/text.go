package gfx

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
)

// Weight is the heaviness of a typeface.
type Weight uint8

// Weights of the interface typeface. Monospaced text has no medium weight;
// Medium falls back to regular there.
const (
	Regular Weight = iota
	Medium
	Bold
)

// Align is the horizontal placement of text relative to its anchor.
type Align uint8

// Horizontal alignments.
const (
	AlignLeft   Align = iota // the anchor is the left end of each line
	AlignCenter              // the anchor is the middle of each line
	AlignRight               // the anchor is the right end of each line
)

// TextStyle says how a piece of text looks. The zero value is 13-point
// regular cream text, left-aligned. Themes hand out ready-made styles
// (Theme.Body, Theme.Head, ...); adjust a copy with the With methods:
//
//	st := th.Body().WithColor(th.TextDim).Aligned(gfx.AlignRight)
type TextStyle struct {
	// Size is the font size in points. Zero or less means 13.
	Size float32
	// Weight selects regular, medium or bold.
	Weight Weight
	// Mono selects the monospaced family, for numbers that must line up.
	Mono bool
	// Color is the text colour. The zero Color means the theme's cream.
	Color Color
	// Align places each line relative to the x coordinate it is drawn at.
	Align Align
	// Tracking is extra space inserted between characters, in points. It
	// is rounded to whole pixels per gap (a non-zero value never rounds to
	// nothing), which keeps the spacing even.
	Tracking float32
	// Upper draws the text in upper case.
	Upper bool
}

// WithColor returns the style with a different colour.
func (st TextStyle) WithColor(c Color) TextStyle { st.Color = c; return st }

// Aligned returns the style with a different alignment.
func (st TextStyle) Aligned(a Align) TextStyle { st.Align = a; return st }

// Sized returns the style with a different size in points.
func (st TextStyle) Sized(size float32) TextStyle { st.Size = size; return st }

// Weighted returns the style with a different weight.
func (st TextStyle) Weighted(w Weight) TextStyle { st.Weight = w; return st }

// defaultTextSize and defaultTextColor apply to a zero TextStyle.
const defaultTextSize = 13

var defaultTextColor = RGB(0xE9E2D0)

// Ellipsis is the character Ellipsize ends a shortened text with.
const Ellipsis = "…"

// lineSpacing is the distance between baselines as a multiple of the font
// size.
const lineSpacing = 1.32

const (
	fontRegular = iota
	fontMedium
	fontBold
	fontMono
	fontMonoBold
	numFonts
)

var (
	fontOnce    sync.Once
	fontSources [numFonts]*text.GoTextFaceSource
)

func loadFonts() {
	for i, ttf := range [numFonts][]byte{
		goregular.TTF, gomedium.TTF, gobold.TTF, gomono.TTF, gomonobold.TTF,
	} {
		src, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
		if err != nil {
			// The fonts are compiled in; this cannot fail at run time.
			panic(fmt.Sprintf("gfx: parsing built-in font %d: %v", i, err))
		}
		fontSources[i] = src
	}
}

// faceKey identifies an instantiated face: which font and its size in
// quarter pixels.
type faceKey struct {
	font uint8
	q    uint16
}

// face is a font at one pixel size, with its metrics in pixels.
type face struct {
	key      faceKey
	gt       *text.GoTextFace
	size     float32 // font size
	ascent   float32
	descent  float32
	capH     float32
	xH       float32
	lineH    float32 // distance between baselines, whole pixels
	base     float32 // from the top of a line to its baseline, whole pixels
	floorAsc float32 // rows between the top of text/v2's layout box and the baseline
}

var faces = map[faceKey]*face{}

// faceFor returns the face a style resolves to at the given scale. Pixel
// sizes are quantised to quarter pixels so that the set of faces stays
// small while every theme size at every sensible scale is hit exactly.
func faceFor(st TextStyle, scale float32) *face {
	size := st.Size
	if !(size > 0) {
		size = defaultTextSize
	}
	if !(scale > 0) {
		scale = 1
	}
	q := int(math.Round(float64(size*scale) * 4))
	q = min(max(q, 4*3), 4*1024)

	var font uint8
	switch {
	case st.Mono && st.Weight == Bold:
		font = fontMonoBold
	case st.Mono:
		font = fontMono
	case st.Weight == Bold:
		font = fontBold
	case st.Weight == Medium:
		font = fontMedium
	default:
		font = fontRegular
	}
	key := faceKey{font, uint16(q)}
	if f := faces[key]; f != nil {
		return f
	}

	fontOnce.Do(loadFonts)
	px := float64(q) / 4
	gt := &text.GoTextFace{Source: fontSources[font], Size: px}
	m := gt.Metrics()
	f := &face{
		key:      key,
		gt:       gt,
		size:     float32(px),
		ascent:   float32(m.HAscent),
		descent:  float32(m.HDescent),
		capH:     float32(m.CapHeight),
		xH:       float32(m.XHeight),
		floorAsc: float32(math.Floor(m.HAscent)),
	}
	f.lineH = float32(math.Max(math.Round(px*lineSpacing), math.Ceil(m.HAscent+m.HDescent)))
	f.base = roundf((f.lineH-(f.ascent+f.descent))/2 + f.ascent)
	faces[key] = f
	return f
}

// measureKey identifies one measured line.
type measureKey struct {
	face faceKey
	s    string
}

// measureCache maps a line of text in a face to its advance in pixels. It
// is emptied when it grows past measureCacheLimit entries; the hot set
// refills within a frame.
var measureCache = map[measureKey]float32{}

const (
	measureCacheLimit = 1 << 14
	upperCacheLimit   = 1 << 12
	wrapCacheLimit    = 1 << 9
)

// advance returns the width of one line (no line breaks) in pixels,
// without tracking.
func (f *face) advance(s string) float32 {
	if s == "" {
		return 0
	}
	if w, ok := measureCache[measureKey{f.key, s}]; ok {
		return w
	}
	w := float32(text.AdvanceAt(s, len(s), f.gt))
	if len(measureCache) >= measureCacheLimit {
		clear(measureCache)
	}
	// The key is cloned so that the cache never pins a caller's larger
	// string or a buffer that is about to be reused.
	measureCache[measureKey{f.key, strings.Clone(s)}] = w
	return w
}

// width returns the width of one line in pixels including tracking of tp
// pixels per gap.
func (f *face) width(s string, tp float32) float32 {
	w := f.advance(s)
	if tp != 0 {
		if n := utf8.RuneCountInString(s); n > 1 {
			w += tp * float32(n-1)
		}
	}
	return w
}

var upperCache = map[string]string{}

// upper returns s in upper case, remembering the result.
func upper(s string) string {
	plain := true
	for i := 0; i < len(s); i++ {
		if b := s[i]; b >= utf8.RuneSelf || (b >= 'a' && b <= 'z') {
			plain = false
			break
		}
	}
	if plain {
		return s
	}
	if u, ok := upperCache[s]; ok {
		return u
	}
	u := strings.ToUpper(s)
	if len(upperCache) >= upperCacheLimit {
		clear(upperCache)
	}
	upperCache[strings.Clone(s)] = u
	return u
}

// trackPixels converts a style's tracking to whole pixels per gap.
func trackPixels(st TextStyle, scale float32) float32 {
	if st.Tracking == 0 {
		return 0
	}
	t := roundf(st.Tracking * scale)
	if t == 0 {
		if st.Tracking > 0 {
			return 1
		}
		return -1
	}
	return t
}

func effScale(scale float32) float32 {
	if scale > 0 {
		return scale
	}
	return 1
}

// FontMetrics describes a text style's vertical proportions, in points.
type FontMetrics struct {
	Ascent     float32 // baseline to the top of the tallest glyphs
	Descent    float32 // baseline to the bottom of the lowest glyphs
	CapHeight  float32 // baseline to the top of capital letters
	XHeight    float32 // baseline to the top of lower-case letters
	LineHeight float32 // distance between the baselines of consecutive lines
	Baseline   float32 // from the top of a line (the y given to Text) to its baseline
}

// Metrics returns the vertical proportions of st when rendered at scale
// pixels per point.
func Metrics(st TextStyle, scale float32) FontMetrics {
	scale = effScale(scale)
	f := faceFor(st, scale)
	return FontMetrics{
		Ascent:     f.ascent / scale,
		Descent:    f.descent / scale,
		CapHeight:  f.capH / scale,
		XHeight:    f.xH / scale,
		LineHeight: f.lineH / scale,
		Baseline:   f.base / scale,
	}
}

// LineHeight returns the height of one line of text in st, in points: the
// distance from the top of one line to the top of the next. It is a whole
// number of pixels at the given scale.
func LineHeight(st TextStyle, scale float32) float32 {
	scale = effScale(scale)
	return faceFor(st, scale).lineH / scale
}

// MeasureText returns the size in points that str occupies when drawn in st
// at scale pixels per point: the width of its widest line and LineHeight
// times its number of lines. Line breaks ('\n') separate lines. The empty
// string measures 0 × 0.
//
// Results are cached, so measuring the same strings every frame is cheap.
func MeasureText(str string, st TextStyle, scale float32) (w, h float32) {
	if str == "" {
		return 0, 0
	}
	scale = effScale(scale)
	f := faceFor(st, scale)
	if st.Upper {
		str = upper(str)
	}
	tp := trackPixels(st, scale)
	lines := 1
	for {
		i := strings.IndexByte(str, '\n')
		if i < 0 {
			break
		}
		w = max(w, f.width(str[:i], tp))
		str = str[i+1:]
		lines++
	}
	w = max(w, f.width(str, tp))
	return w / scale, float32(lines) * f.lineH / scale
}

// wrapKey identifies one wrapping job.
type wrapKey struct {
	face faceKey
	tp   int16
	w    int32 // available width in quarter pixels
	s    string
}

var wrapCache = map[wrapKey][]string{}

// WrapText breaks str into lines no wider than width points when drawn in
// st at scale pixels per point. Lines break at spaces; a single word wider
// than the whole width is split between characters. Existing line breaks
// ('\n') are kept, blank lines included. Spaces at a break and at the start
// of a paragraph are dropped. If st.Upper is set the returned lines are
// already upper-cased.
//
// The result is cached and shared: treat the returned slice as read-only.
func WrapText(str string, width float32, st TextStyle, scale float32) []string {
	scale = effScale(scale)
	f := faceFor(st, scale)
	if st.Upper {
		str = upper(str)
	}
	tp := trackPixels(st, scale)
	q := int32(math.Floor(float64(width*scale)*4 + 1e-3))
	key := wrapKey{f.key, int16(tp), q, str}
	if lines, ok := wrapCache[key]; ok {
		return lines
	}
	maxW := float32(q) / 4

	var lines []string
	rest := str
	for {
		para, tail, more := strings.Cut(rest, "\n")
		lines = f.wrapParagraph(lines, para, maxW, tp)
		if !more {
			break
		}
		rest = tail
	}
	if len(wrapCache) >= wrapCacheLimit {
		clear(wrapCache)
	}
	key.s = strings.Clone(str)
	wrapCache[key] = lines
	return lines
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' }

// wrapParagraph appends the lines of one paragraph (no line breaks) to out.
func (f *face) wrapParagraph(out []string, para string, maxW, tp float32) []string {
	spaceW := f.advance(" ")
	// Widths of candidate lines are first estimated by adding up cached
	// word widths; only when the estimate is within slack of the limit is
	// the line itself measured. The slack covers kerning across spaces.
	slack := max(1, 0.12*f.size)

	start, end := -1, 0 // the current line is para[start:end]; start < 0 means none
	var adv float32     // estimated advance of the current line
	runes := 0          // runes in the current line

	n := len(para)
	for i := 0; i < n; {
		j := i
		for j < n && isSpace(para[j]) {
			j++
		}
		k := j
		for k < n && !isSpace(para[k]) {
			k++
		}
		if k == j {
			break // only spaces remained
		}
		ww := f.advance(para[j:k])
		wr := utf8.RuneCountInString(para[j:k])

		if start >= 0 {
			est := adv + float32(j-i)*spaceW + ww
			cr := runes + (j - i) + wr
			total := est + tp*float32(cr-1)
			fits := total <= maxW-slack
			if !fits && total <= maxW+slack {
				fits = f.width(para[start:k], tp) <= maxW
			}
			if fits {
				adv, runes, end = est, cr, k
				i = k
				continue
			}
			out = append(out, para[start:end])
			start = -1
		}

		// The word starts a new line. If it is too wide on its own, peel
		// off as many characters as fit, line by line.
		for ww+tp*float32(wr-1) > maxW && wr > 1 {
			cut := f.fitPrefix(para[j:k], maxW, tp)
			out = append(out, para[j:j+cut])
			j += cut
			ww = f.advance(para[j:k])
			wr = utf8.RuneCountInString(para[j:k])
		}
		start, end, adv, runes = j, k, ww, wr
		i = k
	}
	if start >= 0 {
		return append(out, para[start:end])
	}
	return append(out, "")
}

// fitPrefix returns the length in bytes of the longest prefix of s that is
// at most maxW pixels wide, but at least one character.
func (f *face) fitPrefix(s string, maxW, tp float32) int {
	_, first := utf8.DecodeRuneInString(s)
	best := first
	runes := 0
	for i := range s {
		if i == 0 {
			continue
		}
		runes++ // s[:i] holds this many runes
		w := float32(text.AdvanceAt(s, i, f.gt)) + tp*float32(runes-1)
		if w > maxW {
			break
		}
		best = i
	}
	// Shaping a prefix on its own can differ from its width in context;
	// back off until the piece itself fits.
	for best > first && f.width(s[:best], tp) > maxW {
		_, size := utf8.DecodeLastRuneInString(s[:best])
		best -= size
	}
	return best
}

var ellipsisCache = map[wrapKey]string{}

// Ellipsize returns str shortened so that it is at most width points wide
// when drawn in st at scale pixels per point, ending in "…" if anything was
// cut. Only the first line of str is used. A string that already fits is
// returned unchanged; if st.Upper is set a shortened result is upper-cased.
// When not even the ellipsis fits the result is empty.
func Ellipsize(str string, width float32, st TextStyle, scale float32) string {
	scale = effScale(scale)
	f := faceFor(st, scale)
	tp := trackPixels(st, scale)
	maxW := width * scale

	src := str
	if st.Upper {
		src = upper(src)
	}
	multi := false
	if i := strings.IndexByte(src, '\n'); i >= 0 {
		src, multi = src[:i], true
	}
	if !multi && f.width(src, tp) <= maxW {
		return str
	}

	key := wrapKey{f.key, int16(tp), int32(math.Floor(float64(maxW) * 4)), src}
	if multi {
		key.w = -key.w - 1 // a cut line and a whole line shorten differently
	}
	if out, ok := ellipsisCache[key]; ok {
		return out
	}

	out := ""
	ellW := f.advance(Ellipsis)
	if ellW <= maxW {
		// Longest prefix that leaves room for the ellipsis.
		cut := 0
		runes := 0
		for i := range src {
			if i == 0 {
				continue
			}
			runes++
			w := float32(text.AdvanceAt(src, i, f.gt)) + tp*float32(runes) + ellW
			if w > maxW {
				break
			}
			cut = i
		}
		if multi && f.width(src, tp)+tp+ellW <= maxW {
			cut = len(src)
		}
		for {
			head := strings.TrimRight(src[:cut], " \t")
			out = head + Ellipsis
			if cut == 0 || f.width(out, tp) <= maxW {
				break
			}
			_, size := utf8.DecodeLastRuneInString(src[:cut])
			cut -= size
		}
	}
	if len(ellipsisCache) >= wrapCacheLimit {
		clear(ellipsisCache)
	}
	key.s = strings.Clone(src)
	ellipsisCache[key] = out
	return out
}

var (
	textOpts text.DrawOptions
	glyphBuf []text.LazyGlyph
)

// drawLine draws one line with its baseline on pixel row baseline and its
// alignment anchor at pixel column ax. It returns the line's width in
// pixels.
func (c *Canvas) drawLine(f *face, line string, ax, baseline float32, align Align, tp float32, col pcol) float32 {
	if line == "" {
		return 0
	}
	w := f.width(line, tp)
	left := ax
	switch align {
	case AlignCenter:
		left = ax - w/2
	case AlignRight:
		left = ax - w
	}
	left = roundf(left)
	top := baseline - f.floorAsc
	// Glyphs may overhang their advance a little; allow for it when culling.
	if !c.visible(left-f.size, baseline-f.ascent-1, left+w+f.size, baseline+f.descent+1) {
		return w
	}

	op := &textOpts
	op.ColorScale.Reset()
	op.ColorScale.Scale(col.r, col.g, col.b, col.a)
	if tp == 0 {
		op.GeoM.Reset()
		op.GeoM.Translate(float64(left), float64(top))
		text.Draw(c.Dst, line, f.gt, op)
		return w
	}

	// Tracked text: place each glyph image ourselves, shifted by a whole
	// number of pixels per preceding character so that the sub-pixel phase
	// each glyph was rasterised for stays valid.
	glyphBuf = text.AppendLazyGlyphs(glyphBuf[:0], line, f.gt, nil)
	pos, runes := 0, 0
	for i := range glyphBuf {
		g := &glyphBuf[i]
		for pos < g.StartIndexInBytes && pos < len(line) {
			_, size := utf8.DecodeRuneInString(line[pos:])
			pos += size
			runes++
		}
		if g.ImageBounds.Empty() {
			continue
		}
		gx := left + float32(g.ImageBounds.Min.X) + tp*float32(runes)
		gy := top + float32(g.ImageBounds.Min.Y)
		if !c.visible(gx, gy, gx+float32(g.ImageBounds.Dx()), gy+float32(g.ImageBounds.Dy())) {
			continue
		}
		img := g.Image()
		if img == nil {
			continue
		}
		op.GeoM.Reset()
		op.GeoM.Translate(float64(gx), float64(gy))
		c.Dst.DrawImage(img, &op.DrawImageOptions)
	}
	clear(glyphBuf) // drop the references the glyphs hold
	return w
}

// drawText draws str with the baseline of its first line on pixel row
// baseline and returns the width of its widest line in pixels.
func (c *Canvas) drawText(f *face, str string, ax, baseline float32, st TextStyle) float32 {
	col := st.Color
	if col == (Color{}) {
		col = defaultTextColor
	}
	if col.A == 0 || c.Dst == nil {
		return 0
	}
	if st.Upper {
		str = upper(str)
	}
	tp := trackPixels(st, c.scale())
	pc := toPcol(col)
	var w float32
	for {
		i := strings.IndexByte(str, '\n')
		if i < 0 {
			break
		}
		w = max(w, c.drawLine(f, str[:i], ax, baseline, st.Align, tp, pc))
		str = str[i+1:]
		baseline += f.lineH
	}
	return max(w, c.drawLine(f, str, ax, baseline, st.Align, tp, pc))
}

// Text draws str with the top of its first line at y and its alignment
// anchor at x: the left end, middle or right end of each line according to
// st.Align. Line breaks ('\n') start new lines LineHeight apart. It returns
// the width of the widest line in points.
//
// Text is positioned on whole pixels and rendered at size × Scale, so it is
// sharp at every scale.
func (c *Canvas) Text(str string, x, y float32, st TextStyle) float32 {
	if str == "" {
		return 0
	}
	s := c.scale()
	f := faceFor(st, s)
	ax, top := c.px(x, y)
	return c.drawText(f, str, ax, roundf(top)+f.base, st) / s
}

// TextIn draws str inside r: horizontally placed by st.Align (against r's
// left edge, centre or right edge) and vertically centred — a single line by
// its capital height, which looks centred next to icons and in buttons,
// several lines as a block. The text is neither clipped nor shortened; see
// TextFit.
func (c *Canvas) TextIn(r Rect, str string, st TextStyle) {
	if str == "" {
		return
	}
	s := c.scale()
	f := faceFor(st, s)
	ax := r.X
	switch st.Align {
	case AlignCenter:
		ax = r.X + r.W/2
	case AlignRight:
		ax = r.X + r.W
	}
	px, cy := c.px(ax, r.Y+r.H/2)
	var baseline float32
	if n := strings.Count(str, "\n"); n == 0 {
		baseline = roundf(cy + f.capH/2)
	} else {
		baseline = roundf(cy-float32(n+1)*f.lineH/2) + f.base
	}
	c.drawText(f, str, px, baseline, st)
}

// TextFit is TextIn for a single line that must not overflow: the text is
// shortened with an ellipsis to the width of r.
func (c *Canvas) TextFit(r Rect, str string, st TextStyle) {
	c.TextIn(r, Ellipsize(str, r.W, st, c.scale()), st)
}

// MeasureText returns the size of str in points at this canvas's scale; see
// the package-level MeasureText.
func (c *Canvas) MeasureText(str string, st TextStyle) (w, h float32) {
	return MeasureText(str, st, c.scale())
}

// WrapText breaks str into lines at most width points wide at this canvas's
// scale; see the package-level WrapText. The result is shared: read-only.
func (c *Canvas) WrapText(str string, width float32, st TextStyle) []string {
	return WrapText(str, width, st, c.scale())
}

// Ellipsize shortens str to width points at this canvas's scale; see the
// package-level Ellipsize.
func (c *Canvas) Ellipsize(str string, width float32, st TextStyle) string {
	return Ellipsize(str, width, st, c.scale())
}

// LineHeight returns the line height of st in points at this canvas's
// scale.
func (c *Canvas) LineHeight(st TextStyle) float32 {
	return LineHeight(st, c.scale())
}

// TextLines draws lines one below the other, starting with the top of the
// first at y, each aligned on x like Text. It is the companion of WrapText.
// It returns the total height drawn, in points.
func (c *Canvas) TextLines(lines []string, x, y float32, st TextStyle) float32 {
	s := c.scale()
	f := faceFor(st, s)
	ax, top := c.px(x, y)
	baseline := roundf(top) + f.base
	for _, line := range lines {
		if line != "" {
			c.drawText(f, line, ax, baseline, st)
		}
		baseline += f.lineH
	}
	return float32(len(lines)) * f.lineH / s
}
