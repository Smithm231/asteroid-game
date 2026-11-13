package gfx

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Canvas draws onto an Ebitengine image in points. Every coordinate and
// length passed to its methods is multiplied by Scale at draw time.
//
// Create one per frame (or keep one and update its fields); it is small and
// holds no GPU resources of its own:
//
//	cv := gfx.NewCanvas(screen, in.Scale)
//
// # Crispness
//
// Rectangles, chamfered panels, axis-aligned lines, clip rectangles, images
// and text are snapped to whole pixels, so hairlines stay one sharp pixel
// row at any scale and neighbouring rectangles tile without seams. Free-form
// geometry (slanted lines, polygons, circles, arcs, hexes, paths) is not
// snapped and is anti-aliased instead, so it can move smoothly.
//
// # Colour
//
// A colour's alpha is its opacity. Fully transparent colours draw nothing.
type Canvas struct {
	// Dst is the image drawn to. While a clip is pushed it is a sub-image
	// of the original; drawing to it directly respects the clip.
	Dst *ebiten.Image
	// Scale is the number of pixels per point. A value that is not positive
	// is treated as 1.
	Scale float32

	ox, oy float32         // origin in pixels, non-zero only for Local canvases
	clips  []*ebiten.Image // destinations saved by PushClip
}

// NewCanvas returns a canvas drawing to dst at scale pixels per point.
func NewCanvas(dst *ebiten.Image, scale float32) *Canvas {
	return &Canvas{Dst: dst, Scale: scale}
}

// whiteSrc is the source coordinate, inside whitePixel, that every solid
// vertex samples.
const whiteSrc = 1.5

var (
	// whiteImage is a 3×3 opaque white image; whitePixel is its centre
	// texel, the source of all solid geometry. Being an ordinary small
	// image it lives on Ebitengine's shared atlas next to the text glyphs.
	whiteImage *ebiten.Image
	whitePixel *ebiten.Image

	// mb is the mesh every primitive is built in before it is drawn.
	mb meshBuf

	triOpts = ebiten.DrawTrianglesOptions{
		ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
	}
	imgOpts ebiten.DrawImageOptions
)

func init() {
	whiteImage = ebiten.NewImage(3, 3)
	pix := make([]byte, 4*3*3)
	for i := range pix {
		pix[i] = 0xFF
	}
	// WritePixels rather than Fill keeps the image on the shared atlas.
	whiteImage.WritePixels(pix)
	whitePixel = whiteImage.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
}

// scale returns the effective pixels-per-point factor.
func (c *Canvas) scale() float32 {
	if c.Scale > 0 {
		return c.Scale
	}
	return 1
}

// px converts a point position to pixels without snapping.
func (c *Canvas) px(x, y float32) (float32, float32) {
	s := c.scale()
	return x*s + c.ox, y*s + c.oy
}

// roundf rounds to the nearest integer, halves upwards, so that a shared
// edge always lands on the same pixel boundary.
func roundf(v float32) float32 { return float32(math.Floor(float64(v) + 0.5)) }

// snapRect converts a rectangle to pixel-aligned edges. A rectangle with a
// positive size never collapses below one pixel. ok is false when there is
// nothing to draw: the rectangle is empty or entirely outside the clip.
func (c *Canvas) snapRect(r Rect) (x0, y0, x1, y1 float32, ok bool) {
	if !(r.W > 0 && r.H > 0) || c.Dst == nil {
		return 0, 0, 0, 0, false
	}
	s := c.scale()
	x0 = roundf(r.X*s + c.ox)
	y0 = roundf(r.Y*s + c.oy)
	x1 = roundf((r.X+r.W)*s + c.ox)
	y1 = roundf((r.Y+r.H)*s + c.oy)
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}
	return x0, y0, x1, y1, c.visible(x0, y0, x1, y1)
}

// snapLen converts a thickness to whole pixels, at least one.
func (c *Canvas) snapLen(v float32) float32 {
	return max(1, roundf(v*c.scale()))
}

// visible reports whether the pixel box overlaps the current clip.
func (c *Canvas) visible(x0, y0, x1, y1 float32) bool {
	if c.Dst == nil {
		return false
	}
	b := c.Dst.Bounds()
	return x1 > float32(b.Min.X) && x0 < float32(b.Max.X) &&
		y1 > float32(b.Min.Y) && y0 < float32(b.Max.Y)
}

// flush draws the triangles accumulated in mb.
func (c *Canvas) flush() {
	if len(mb.i) == 0 {
		return
	}
	c.Dst.DrawTriangles32(mb.v, mb.i, whitePixel, &triOpts)
}

// Size returns the size of the destination image in points, ignoring any
// clip.
func (c *Canvas) Size() (w, h float32) {
	if c.Dst == nil {
		return 0, 0
	}
	dst := c.Dst
	if len(c.clips) > 0 {
		dst = c.clips[0]
	}
	b := dst.Bounds()
	s := c.scale()
	return float32(b.Dx()) / s, float32(b.Dy()) / s
}

// Bounds returns the area that drawing can currently reach — the
// destination image intersected with every pushed clip — in points.
func (c *Canvas) Bounds() Rect {
	if c.Dst == nil {
		return Rect{}
	}
	b := c.Dst.Bounds()
	s := c.scale()
	return Rect{
		(float32(b.Min.X) - c.ox) / s, (float32(b.Min.Y) - c.oy) / s,
		float32(b.Dx()) / s, float32(b.Dy()) / s,
	}
}

// Visible reports whether any part of r can currently be drawn: it is false
// for rectangles entirely outside the destination or the clip. Use it to
// skip building content that would be culled anyway.
func (c *Canvas) Visible(r Rect) bool {
	x0, y0 := c.px(r.X, r.Y)
	x1, y1 := c.px(r.X+r.W, r.Y+r.H)
	return c.visible(x0, y0, x1, y1)
}

// Snap returns v moved to the nearest pixel boundary, still in points.
// Layout code that centres things can use it to avoid half-pixel positions.
func (c *Canvas) Snap(v float32) float32 {
	s := c.scale()
	return roundf(v*s) / s
}

// SnapRect returns r with its edges moved to the nearest pixel boundaries,
// still in points. This is the rectangle FillRect actually covers.
func (c *Canvas) SnapRect(r Rect) Rect {
	s := c.scale()
	x0 := roundf(r.X*s+c.ox) - c.ox
	y0 := roundf(r.Y*s+c.oy) - c.oy
	x1 := roundf((r.X+r.W)*s+c.ox) - c.ox
	y1 := roundf((r.Y+r.H)*s+c.oy) - c.oy
	return Rect{x0 / s, y0 / s, (x1 - x0) / s, (y1 - y0) / s}
}

// clipImage returns the destination restricted to r.
func (c *Canvas) clipImage(r Rect) *ebiten.Image {
	s := c.scale()
	x0 := int(roundf(r.X*s + c.ox))
	y0 := int(roundf(r.Y*s + c.oy))
	x1 := int(roundf((r.X+r.W)*s + c.ox))
	y1 := int(roundf((r.Y+r.H)*s + c.oy))
	// SubImage intersects with the current bounds and caches the result per
	// rectangle, so pushing the same clip every frame allocates nothing.
	return c.Dst.SubImage(image.Rect(x0, y0, max(x0, x1), max(y0, y1))).(*ebiten.Image)
}

// PushClip restricts drawing to r (intersected with any clip already in
// force) until the matching PopClip. The clip edges are snapped to pixels
// the same way FillRect snaps, so a clip set to a panel's rectangle matches
// its fill exactly.
func (c *Canvas) PushClip(r Rect) {
	if c.Dst == nil {
		return
	}
	prev := c.Dst
	c.Dst = c.clipImage(r)
	c.clips = append(c.clips, prev)
}

// PopClip removes the clip set by the most recent PushClip. Calling it with
// no clip pushed does nothing.
func (c *Canvas) PopClip() {
	n := len(c.clips)
	if n == 0 {
		return
	}
	c.Dst = c.clips[n-1]
	c.clips[n-1] = nil
	c.clips = c.clips[:n-1]
}

// Sub returns a new canvas that draws to the same image in the same
// coordinates but is clipped to r (and to this canvas's current clip). The
// two canvases are independent afterwards: clips pushed on one do not affect
// the other.
func (c *Canvas) Sub(r Rect) *Canvas {
	sub := &Canvas{Dst: c.Dst, Scale: c.Scale, ox: c.ox, oy: c.oy}
	if c.Dst != nil {
		sub.Dst = c.clipImage(r)
	}
	return sub
}

// Local returns a new canvas with its own coordinate system: its origin is
// r's top-left corner and unitsW of its units span r's width, so art
// authored on a fixed grid can be drawn at any place and size:
//
//	lc := cv.Local(tile, 32) // draw on a 32-unit-wide grid inside tile
//	lc.FillCircle(16, 16, 10, col)
//
// Lengths, line widths and font sizes scale along with positions. The local
// canvas is not clipped to r; it shares the current clip of c.
func (c *Canvas) Local(r Rect, unitsW float32) *Canvas {
	s := c.scale()
	k := float32(1)
	if unitsW > 0 {
		k = r.W / unitsW
	}
	return &Canvas{Dst: c.Dst, Scale: s * k, ox: r.X*s + c.ox, oy: r.Y*s + c.oy}
}

// FillRect fills r with a flat colour. Edges are snapped to pixels.
func (c *Canvas) FillRect(r Rect, col Color) {
	if col.A == 0 {
		return
	}
	x0, y0, x1, y1, ok := c.snapRect(r)
	if !ok {
		return
	}
	p := toPcol(col)
	mb.reset()
	mb.rect(x0, y0, x1, y1, p, p, p, p)
	c.flush()
}

// FillRectV fills r with a vertical gradient from top to bottom.
func (c *Canvas) FillRectV(r Rect, top, bottom Color) {
	if top.A == 0 && bottom.A == 0 {
		return
	}
	x0, y0, x1, y1, ok := c.snapRect(r)
	if !ok {
		return
	}
	t, b := toPcol(top), toPcol(bottom)
	mb.reset()
	mb.rect(x0, y0, x1, y1, t, t, b, b)
	c.flush()
}

// FillRectH fills r with a horizontal gradient from left to right.
func (c *Canvas) FillRectH(r Rect, left, right Color) {
	if left.A == 0 && right.A == 0 {
		return
	}
	x0, y0, x1, y1, ok := c.snapRect(r)
	if !ok {
		return
	}
	l, rt := toPcol(left), toPcol(right)
	mb.reset()
	mb.rect(x0, y0, x1, y1, l, rt, rt, l)
	c.flush()
}

// StrokeRect draws a frame of the given width just inside the edges of r,
// so it never spills outside the rectangle. The width is rounded to whole
// pixels, at least one.
func (c *Canvas) StrokeRect(r Rect, width float32, col Color) {
	if col.A == 0 || !(width > 0) {
		return
	}
	x0, y0, x1, y1, ok := c.snapRect(r)
	if !ok {
		return
	}
	w := c.snapLen(width)
	p := toPcol(col)
	mb.reset()
	if 2*w >= x1-x0 || 2*w >= y1-y0 {
		mb.rect(x0, y0, x1, y1, p, p, p, p)
	} else {
		mb.rect(x0, y0, x1, y0+w, p, p, p, p)
		mb.rect(x0, y1-w, x1, y1, p, p, p, p)
		mb.rect(x0, y0+w, x0+w, y1-w, p, p, p, p)
		mb.rect(x1-w, y0+w, x1, y1-w, p, p, p, p)
	}
	c.flush()
}

// Line draws a straight line of the given width with square-cut ends.
//
// A horizontal or vertical line is snapped to the pixel grid and drawn as a
// crisp bar whose thickness is a whole number of pixels (at least one), so
// Line is safe for hairline rules. Any other line is anti-aliased and may be
// thinner than a pixel, in which case it fades instead.
func (c *Canvas) Line(x0, y0, x1, y1, width float32, col Color) {
	c.StrokeLine(x0, y0, x1, y1, Stroke{Width: width}, col)
}

// Stroke describes how an outline is drawn. The zero value apart from Width
// is a solid line with butt caps.
type Stroke struct {
	// Width is the line width in points.
	Width float32
	// Cap is the shape of the ends of an open line.
	Cap Cap
	// Dash and Gap, when both positive, make the line dashed: Dash points
	// drawn, Gap points skipped, repeating along the line.
	Dash, Gap float32
	// Phase shifts the dash pattern along the line, in points. Animate it
	// to make the dashes march.
	Phase float32
}

func (st Stroke) dashed() bool { return st.Dash > 0 && st.Gap > 0 }

// StrokeLine draws a straight line with full control over caps and dashes.
// Solid butt-capped horizontal and vertical lines are pixel-snapped like
// Line.
func (c *Canvas) StrokeLine(x0, y0, x1, y1 float32, st Stroke, col Color) {
	if col.A == 0 || !(st.Width > 0) || c.Dst == nil {
		return
	}
	if (x0 == x1 || y0 == y1) && st.Cap == CapButt && !st.dashed() {
		c.axisLine(x0, y0, x1, y1, st.Width, col)
		return
	}
	ax, ay := c.px(x0, y0)
	bx, by := c.px(x1, y1)
	mb.pts = append(mb.pts[:0], Pt{ax, ay}, Pt{bx, by})
	c.strokePixels(false, st, col)
}

// axisLine draws a horizontal or vertical line as a pixel-aligned bar.
func (c *Canvas) axisLine(x0, y0, x1, y1, width float32, col Color) {
	if x0 == x1 && y0 == y1 {
		return
	}
	w := c.snapLen(width)
	ax, ay := c.px(min(x0, x1), min(y0, y1))
	bx, by := c.px(max(x0, x1), max(y0, y1))
	var px0, py0, px1, py1 float32
	if y0 == y1 {
		px0, px1 = roundf(ax), roundf(bx)
		py0 = roundf(ay - w/2)
		py1 = py0 + w
	} else {
		py0, py1 = roundf(ay), roundf(by)
		px0 = roundf(ax - w/2)
		px1 = px0 + w
	}
	if px1 <= px0 || py1 <= py0 || !c.visible(px0, py0, px1, py1) {
		return
	}
	p := toPcol(col)
	mb.reset()
	mb.rect(px0, py0, px1, py1, p, p, p, p)
	c.flush()
}

// strokePixels strokes the pixel-space outline held in mb.pts.
func (c *Canvas) strokePixels(closed bool, st Stroke, col Color) {
	pts := mb.dedupe(mb.pts, closed)
	if len(pts) < 2 {
		return
	}
	s := c.scale()
	w := st.Width * s
	if !c.outlineVisible(pts, w/2+1) {
		return
	}
	mb.reset()
	if st.dashed() {
		mb.dashed(pts, closed, w, st.Dash*s, st.Gap*s, st.Phase*s, toPcol(col), st.Cap)
	} else {
		mb.stroke(pts, closed, 0, w, toPcol(col), st.Cap)
	}
	c.flush()
}

// outlineVisible reports whether the bounding box of pts, grown by margin
// pixels, overlaps the clip.
func (c *Canvas) outlineVisible(pts []Pt, margin float32) bool {
	x0, y0 := pts[0].X, pts[0].Y
	x1, y1 := x0, y0
	for _, p := range pts[1:] {
		x0, x1 = min(x0, p.X), max(x1, p.X)
		y0, y1 = min(y0, p.Y), max(y1, p.Y)
	}
	return c.visible(x0-margin, y0-margin, x1+margin, y1+margin)
}

// toPixels converts pts to pixels into mb.pts.
func (c *Canvas) toPixels(pts []Pt) {
	s := c.scale()
	mb.pts = mb.pts[:0]
	for _, p := range pts {
		mb.pts = append(mb.pts, Pt{p.X*s + c.ox, p.Y*s + c.oy})
	}
}

// Polyline draws an open, anti-aliased line through pts with butt caps and
// mitred corners.
func (c *Canvas) Polyline(pts []Pt, width float32, col Color) {
	c.StrokePolyline(pts, false, Stroke{Width: width}, col)
}

// StrokePoly draws the closed outline of the polygon pts, centred on its
// edges, anti-aliased, with mitred corners.
func (c *Canvas) StrokePoly(pts []Pt, width float32, col Color) {
	c.StrokePolyline(pts, true, Stroke{Width: width}, col)
}

// StrokePolyline draws a line through pts with full control: closed joins
// the last point back to the first, and st selects width, caps and dashes.
// Very sharp corners (under about 30°) are blunted.
func (c *Canvas) StrokePolyline(pts []Pt, closed bool, st Stroke, col Color) {
	if col.A == 0 || !(st.Width > 0) || len(pts) < 2 || c.Dst == nil {
		return
	}
	c.toPixels(pts)
	c.strokePixels(closed, st, col)
}

// FillPoly fills the polygon pts with a flat colour, anti-aliased. Convex
// polygons take the fast path; concave ones are filled through the vector
// rasteriser (non-zero winding), which costs more per call.
func (c *Canvas) FillPoly(pts []Pt, col Color) {
	c.fillPoly(pts, col, col)
}

// FillPolyV fills the polygon pts with a vertical gradient running from top
// at its highest point to bottom at its lowest.
func (c *Canvas) FillPolyV(pts []Pt, top, bottom Color) {
	c.fillPoly(pts, top, bottom)
}

func (c *Canvas) fillPoly(pts []Pt, top, bottom Color) {
	if (top.A == 0 && bottom.A == 0) || len(pts) < 3 || c.Dst == nil {
		return
	}
	c.toPixels(pts)
	px := mb.dedupe(mb.pts, true)
	if len(px) < 3 || !c.outlineVisible(px, 1) {
		return
	}
	if !isConvex(px) {
		c.fillConcave(px, top, bottom)
		return
	}
	p := c.gradientOver(px, top, bottom)
	mb.reset()
	mb.convex(px, &p)
	c.flush()
}

// gradientOver returns the paint for a shape with the pixel outline px: flat
// when both colours are equal, otherwise a vertical gradient over the
// outline's height.
func (c *Canvas) gradientOver(px []Pt, top, bottom Color) paint {
	if top == bottom {
		return flat(top)
	}
	y0, y1 := px[0].Y, px[0].Y
	for _, p := range px[1:] {
		y0, y1 = min(y0, p.Y), max(y1, p.Y)
	}
	return vgrad(top, bottom, y0, y1)
}

// FillCircle fills a circle, anti-aliased.
func (c *Canvas) FillCircle(cx, cy, r float32, col Color) {
	if col.A == 0 || !(r > 0) || c.Dst == nil {
		return
	}
	x, y := c.px(cx, cy)
	rp := r * c.scale()
	if !c.visible(x-rp-1, y-rp-1, x+rp+1, y+rp+1) {
		return
	}
	p := flat(col)
	mb.reset()
	mb.disc(x, y, rp, &p)
	c.flush()
}

// FillCircleV fills a circle with a vertical gradient from top to bottom.
func (c *Canvas) FillCircleV(cx, cy, r float32, top, bottom Color) {
	if (top.A == 0 && bottom.A == 0) || !(r > 0) || c.Dst == nil {
		return
	}
	x, y := c.px(cx, cy)
	rp := r * c.scale()
	if !c.visible(x-rp-1, y-rp-1, x+rp+1, y+rp+1) {
		return
	}
	p := vgrad(top, bottom, y-rp, y+rp)
	mb.reset()
	mb.disc(x, y, rp, &p)
	c.flush()
}

// StrokeCircle draws a circle outline of the given width, centred on the
// radius r, anti-aliased.
func (c *Canvas) StrokeCircle(cx, cy, r, width float32, col Color) {
	if col.A == 0 || !(r > 0) || !(width > 0) || c.Dst == nil {
		return
	}
	x, y := c.px(cx, cy)
	s := c.scale()
	rp, wp := r*s, width*s
	if e := rp + wp/2 + 1; !c.visible(x-e, y-e, x+e, y+e) {
		return
	}
	mb.reset()
	mb.ring(x, y, rp, wp, toPcol(col))
	c.flush()
}

// Arc draws the part of a circle outline between the angles a0 and a1, with
// butt caps. Angles are in radians, measured clockwise on screen from the
// positive x axis (so -π/2 is straight up); the arc runs from a0 to a1 in
// the direction of the sign of a1-a0.
func (c *Canvas) Arc(cx, cy, r, a0, a1, width float32, col Color) {
	c.StrokeArc(cx, cy, r, a0, a1, Stroke{Width: width}, col)
}

// StrokeArc is Arc with full control over width, caps and dashes. A sweep of
// a full turn or more draws a closed circle, which is how to get a dashed
// circle:
//
//	cv.StrokeArc(x, y, r, 0, 2*math.Pi, gfx.Stroke{Width: 1, Dash: 6, Gap: 4}, col)
func (c *Canvas) StrokeArc(cx, cy, r, a0, a1 float32, st Stroke, col Color) {
	if col.A == 0 || !(r > 0) || !(st.Width > 0) || a0 == a1 || c.Dst == nil {
		return
	}
	x, y := c.px(cx, cy)
	s := c.scale()
	rp := r * s
	if e := rp + st.Width*s/2 + 1; !c.visible(x-e, y-e, x+e, y+e) {
		return
	}
	closed := absf(a1-a0) >= 2*math.Pi-1e-4
	if closed {
		if a1 > a0 {
			a1 = a0 + 2*math.Pi
		} else {
			a1 = a0 - 2*math.Pi
		}
	}
	mb.pts = arcPoints(mb.pts[:0], x, y, rp, a0, a1)
	if closed {
		mb.pts = mb.pts[:len(mb.pts)-1] // the last point repeats the first
	}
	c.strokePixels(closed, st, col)
}

// FillSector fills the pie slice of a circle between the angles a0 and a1
// (radians, clockwise on screen from the positive x axis). A sweep of a full
// turn or more fills the whole circle.
func (c *Canvas) FillSector(cx, cy, r, a0, a1 float32, col Color) {
	if col.A == 0 || !(r > 0) || a0 == a1 || c.Dst == nil {
		return
	}
	sweep := absf(a1 - a0)
	if sweep >= 2*math.Pi-1e-4 {
		c.FillCircle(cx, cy, r, col)
		return
	}
	x, y := c.px(cx, cy)
	rp := r * c.scale()
	if !c.visible(x-rp-1, y-rp-1, x+rp+1, y+rp+1) {
		return
	}
	mb.pts = append(mb.pts[:0], Pt{x, y})
	mb.pts = arcPoints(mb.pts, x, y, rp, a0, a1)
	px := mb.dedupe(mb.pts, true)
	if len(px) < 3 {
		return
	}
	if sweep > math.Pi {
		// More than a half turn is not convex.
		c.fillConcave(px, col, col)
		return
	}
	p := flat(col)
	mb.reset()
	mb.convex(px, &p)
	c.flush()
}

// hexUnit holds the corners of a pointy-top hexagon of circumradius 1,
// clockwise on screen starting at the top.
var hexUnit = func() (h [6]Pt) {
	for i := range h {
		sin, cos := math.Sincos(-math.Pi/2 + float64(i)*math.Pi/3)
		h[i] = Pt{float32(cos), float32(sin)}
	}
	return h
}()

// HexPoints returns the six corners of the pointy-top hexagon centred on
// (cx, cy) whose corners are size away from the centre, clockwise on screen
// starting at the top corner. The hexagon is √3·size wide and 2·size tall.
func HexPoints(cx, cy, size float32) [6]Pt {
	var h [6]Pt
	for i, u := range hexUnit {
		h[i] = Pt{cx + u.X*size, cy + u.Y*size}
	}
	return h
}

// FillHex fills a pointy-top hexagon centred on (cx, cy) whose corners are
// size away from the centre, anti-aliased.
//
// Neighbouring hexes that share an edge each fade out across that edge, so a
// tiled field lets a little of the background through along the seams. Draw
// tiles over a filled base, or grow size by about half a pixel, to hide
// them.
func (c *Canvas) FillHex(cx, cy, size float32, col Color) {
	c.fillHex(cx, cy, size, col, col)
}

// FillHexV fills a pointy-top hexagon with a vertical gradient.
func (c *Canvas) FillHexV(cx, cy, size float32, top, bottom Color) {
	c.fillHex(cx, cy, size, top, bottom)
}

func (c *Canvas) fillHex(cx, cy, size float32, top, bottom Color) {
	if (top.A == 0 && bottom.A == 0) || !(size > 0) || c.Dst == nil {
		return
	}
	x, y := c.px(cx, cy)
	sp := size * c.scale()
	if !c.visible(x-sp-1, y-sp-1, x+sp+1, y+sp+1) {
		return
	}
	mb.pts = mb.pts[:0]
	for _, u := range hexUnit {
		mb.pts = append(mb.pts, Pt{x + u.X*sp, y + u.Y*sp})
	}
	p := vgrad(top, bottom, y-sp, y+sp)
	mb.reset()
	mb.convex(mb.pts, &p)
	c.flush()
}

// StrokeHex draws the outline of a pointy-top hexagon, centred on its edges,
// anti-aliased.
func (c *Canvas) StrokeHex(cx, cy, size, width float32, col Color) {
	if col.A == 0 || !(size > 0) || !(width > 0) || c.Dst == nil {
		return
	}
	x, y := c.px(cx, cy)
	s := c.scale()
	sp, wp := size*s, width*s
	if e := sp + wp + 1; !c.visible(x-e, y-e, x+e, y+e) {
		return
	}
	mb.pts = mb.pts[:0]
	for _, u := range hexUnit {
		mb.pts = append(mb.pts, Pt{x + u.X*sp, y + u.Y*sp})
	}
	mb.reset()
	mb.stroke(mb.pts, true, 0, wp, toPcol(col), CapButt)
	c.flush()
}

// ImageOpts adjusts DrawImage and DrawSprite. A nil *ImageOpts draws the
// image as it is.
type ImageOpts struct {
	// Tint multiplies the image's colours; its alpha fades the image. The
	// zero Color means no tint. Use Opacity(a) to change only the opacity.
	Tint Color
	// Rotation turns the image about the centre of its destination, in
	// radians, clockwise on screen.
	Rotation float32
	// Additive adds the image's light to what is already there instead of
	// covering it.
	Additive bool
	// Smooth disables pixel snapping and always filters linearly, for
	// sprites that must glide rather than step from pixel to pixel.
	// Rotated images are always drawn smooth.
	Smooth bool
}

// DrawImage draws img stretched over the rectangle dst.
//
// Unless o says otherwise the destination is snapped to whole pixels:
// its position is rounded, and so is its size (independently of the
// position, so a sprite rendered at exactly that size is copied texel for
// texel and stays sharp). Images that end up scaled are filtered linearly.
func (c *Canvas) DrawImage(img *ebiten.Image, dst Rect, o *ImageOpts) {
	if img == nil || c.Dst == nil || !(dst.W > 0 && dst.H > 0) {
		return
	}
	b := img.Bounds()
	iw, ih := float32(b.Dx()), float32(b.Dy())
	if iw <= 0 || ih <= 0 {
		return
	}
	var opts ImageOpts
	if o != nil {
		opts = *o
	}
	if opts.Tint != (Color{}) && opts.Tint.A == 0 {
		return
	}
	s := c.scale()
	x, y := dst.X*s+c.ox, dst.Y*s+c.oy
	w, h := dst.W*s, dst.H*s
	smooth := opts.Smooth || opts.Rotation != 0
	if !smooth {
		x, y = roundf(x), roundf(y)
		w, h = max(1, roundf(w)), max(1, roundf(h))
	}
	// Cull by the bounding circle, which also covers any rotation.
	if e := (w + h) / 2; !c.visible(x+w/2-e, y+h/2-e, x+w/2+e, y+h/2+e) {
		return
	}

	op := &imgOpts
	*op = ebiten.DrawImageOptions{}
	sx, sy := float64(w/iw), float64(h/ih)
	if opts.Rotation != 0 {
		op.GeoM.Translate(-float64(iw)/2, -float64(ih)/2)
		op.GeoM.Scale(sx, sy)
		op.GeoM.Rotate(float64(opts.Rotation))
		op.GeoM.Translate(float64(x+w/2), float64(y+h/2))
	} else {
		op.GeoM.Scale(sx, sy)
		op.GeoM.Translate(float64(x), float64(y))
	}
	if smooth || w != iw || h != ih {
		op.Filter = ebiten.FilterLinear
	}
	if opts.Tint != (Color{}) {
		op.ColorScale.Scale(premul(opts.Tint))
	}
	if opts.Additive {
		op.Blend = ebiten.BlendLighter
	}
	c.Dst.DrawImage(img, op)
}

// DrawSprite draws img centred on (cx, cy) at its natural size, one image
// pixel per screen pixel, which is how sprites from a SpriteCache or Render
// made for this canvas's scale are meant to be shown.
func (c *Canvas) DrawSprite(img *ebiten.Image, cx, cy float32, o *ImageOpts) {
	if img == nil {
		return
	}
	b := img.Bounds()
	s := c.scale()
	w, h := float32(b.Dx())/s, float32(b.Dy())/s
	c.DrawImage(img, Rect{cx - w/2, cy - h/2, w, h}, o)
}
