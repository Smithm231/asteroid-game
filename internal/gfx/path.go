package gfx

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Path is an outline built from straight lines and curves, in points, for
// shapes the fixed primitives cannot express: curved hulls, concave
// silhouettes, shapes with holes. The zero value is an empty path ready for
// use; Reset lets one Path be reused.
//
// A path may hold several sub-paths; MoveTo starts a new one. Paths are
// rasterised by Ebitengine's vector package with anti-aliasing. That costs
// noticeably more per shape than the Canvas primitives, so prefer drawing
// paths into sprites (Render, SpriteCache) over drawing many of them every
// frame.
type Path struct {
	p vector.Path
}

// Reset empties the path, keeping its memory for reuse.
func (p *Path) Reset() { p.p.Reset() }

// MoveTo starts a new sub-path at (x, y).
func (p *Path) MoveTo(x, y float32) { p.p.MoveTo(x, y) }

// LineTo adds a straight line from the current point to (x, y).
func (p *Path) LineTo(x, y float32) { p.p.LineTo(x, y) }

// QuadTo adds a quadratic Bézier curve to (x, y) with control point
// (cx, cy).
func (p *Path) QuadTo(cx, cy, x, y float32) { p.p.QuadTo(cx, cy, x, y) }

// CubicTo adds a cubic Bézier curve to (x, y) with control points
// (c1x, c1y) and (c2x, c2y).
func (p *Path) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	p.p.CubicTo(c1x, c1y, c2x, c2y, x, y)
}

// ArcTo rounds a corner: it adds a line towards (x1, y1) and a circular arc
// of the given radius that turns towards (x2, y2), tangent to both
// directions, like the canvas arcTo of web browsers.
func (p *Path) ArcTo(x1, y1, x2, y2, radius float32) { p.p.ArcTo(x1, y1, x2, y2, radius) }

// Arc adds a circular arc around (cx, cy) from angle a0 to a1 (radians,
// clockwise on screen from the positive x axis), preceded by a straight
// line from the current point to the arc's start if there is one. The arc
// runs clockwise when a1 > a0 and anticlockwise otherwise; a difference of
// a full turn or more adds the whole circle.
func (p *Path) Arc(cx, cy, r, a0, a1 float32) {
	if a1 == a0 {
		return
	}
	dir := vector.Clockwise
	if a1 < a0 {
		dir = vector.CounterClockwise
	}
	if absf(a1-a0) >= 2*math.Pi {
		a1 = a0 + 2*math.Pi
		dir = vector.Clockwise
	}
	p.p.Arc(cx, cy, r, a0, a1, dir)
}

// Close joins the current point back to the start of the sub-path with a
// straight line.
func (p *Path) Close() { p.p.Close() }

// Rect adds r as a closed sub-path.
func (p *Path) Rect(r Rect) {
	p.MoveTo(r.X, r.Y)
	p.LineTo(r.X+r.W, r.Y)
	p.LineTo(r.X+r.W, r.Y+r.H)
	p.LineTo(r.X, r.Y+r.H)
	p.Close()
}

// Chamfer adds r with the selected corners cut off at 45° by cut, as a
// closed sub-path.
func (p *Path) Chamfer(r Rect, cut float32, corners Corners) {
	var buf [8]Pt
	p.Poly(appendChamfer(buf[:0], r.X, r.Y, r.X+r.W, r.Y+r.H, cut, corners))
}

// RoundRect adds r with all four corners rounded to the given radius, as a
// closed sub-path.
func (p *Path) RoundRect(r Rect, radius float32) {
	radius = max(0, min(radius, r.W/2, r.H/2))
	if radius == 0 {
		p.Rect(r)
		return
	}
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	p.MoveTo(x0+radius, y0)
	p.LineTo(x1-radius, y0)
	p.ArcTo(x1, y0, x1, y0+radius, radius)
	p.LineTo(x1, y1-radius)
	p.ArcTo(x1, y1, x1-radius, y1, radius)
	p.LineTo(x0+radius, y1)
	p.ArcTo(x0, y1, x0, y1-radius, radius)
	p.LineTo(x0, y0+radius)
	p.ArcTo(x0, y0, x0+radius, y0, radius)
	p.Close()
}

// Circle adds a full circle as a closed sub-path.
func (p *Path) Circle(cx, cy, r float32) { p.Ellipse(cx, cy, r, r) }

// Ellipse adds an axis-aligned ellipse with radii rx and ry as a closed
// sub-path.
func (p *Path) Ellipse(cx, cy, rx, ry float32) {
	// Four cubic Béziers; k is the usual circle constant 4/3·(√2-1).
	const k = 0.5522847498
	p.MoveTo(cx+rx, cy)
	p.CubicTo(cx+rx, cy+k*ry, cx+k*rx, cy+ry, cx, cy+ry)
	p.CubicTo(cx-k*rx, cy+ry, cx-rx, cy+k*ry, cx-rx, cy)
	p.CubicTo(cx-rx, cy-k*ry, cx-k*rx, cy-ry, cx, cy-ry)
	p.CubicTo(cx+k*rx, cy-ry, cx+rx, cy-k*ry, cx+rx, cy)
	p.Close()
}

// Poly adds the polygon pts as a closed sub-path.
func (p *Path) Poly(pts []Pt) {
	if len(pts) < 2 {
		return
	}
	p.MoveTo(pts[0].X, pts[0].Y)
	for _, q := range pts[1:] {
		p.LineTo(q.X, q.Y)
	}
	p.Close()
}

// Polyline adds the open line through pts as a sub-path.
func (p *Path) Polyline(pts []Pt) {
	if len(pts) < 2 {
		return
	}
	p.MoveTo(pts[0].X, pts[0].Y)
	for _, q := range pts[1:] {
		p.LineTo(q.X, q.Y)
	}
}

// Hex adds a pointy-top hexagon (see HexPoints) as a closed sub-path.
func (p *Path) Hex(cx, cy, size float32) {
	h := HexPoints(cx, cy, size)
	p.Poly(h[:])
}

// Bounds returns the bounding box of the path in its own coordinates,
// rounded outwards to whole units. An empty path has zero bounds.
func (p *Path) Bounds() Rect {
	b := p.p.Bounds()
	return Rect{float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy())}
}

// Join is the shape of the corners of a stroked path.
type Join uint8

// Corner shapes for StrokePath and DrawPath.
const (
	JoinMiter Join = iota // sharp corners (blunted when very acute)
	JoinRound             // rounded corners
	JoinBevel             // corners cut straight across
)

// PathStyle says how DrawPath renders a path: filled, stroked or both,
// optionally moved, scaled and turned first.
type PathStyle struct {
	// Fill is the fill colour. The zero Color means no fill.
	Fill Color
	// FillTo, when set, turns the fill into a vertical gradient from Fill
	// at the top of the path to FillTo at its bottom. Gradient fills go
	// through an offscreen mask and cost a few draw calls each.
	FillTo Color
	// EvenOdd fills by the even-odd rule instead of non-zero winding, which
	// turns every nested sub-path into a hole.
	EvenOdd bool

	// Stroke is the outline colour. The zero Color means no outline.
	Stroke Color
	// Width is the outline width in points. It is not affected by Scale.
	Width float32
	// Join and Cap shape the outline's corners and open ends.
	Join Join
	Cap  Cap

	// The path is scaled by Scale (zero means 1), turned by Rotation
	// (radians, clockwise on screen) about its own origin, then moved by
	// (X, Y).
	X, Y     float32
	Scale    float32
	Rotation float32
}

var (
	pathPx       vector.Path // the path being drawn, in destination pixels
	pathFillOpts vector.FillOptions
	pathDrawOpts = vector.DrawPathOptions{AntiAlias: true}
	pathStroke   vector.StrokeOptions
	maskImage    *ebiten.Image // scratch coverage mask for gradient fills
	maskOpts     = ebiten.DrawTrianglesOptions{
		ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
	}
	maskVerts [4]ebiten.Vertex
)

// FillPath fills p with a flat colour (non-zero winding), anti-aliased.
func (c *Canvas) FillPath(p *Path, col Color) {
	c.DrawPath(p, PathStyle{Fill: col})
}

// FillPathV fills p with a vertical gradient from top, at the path's highest
// point, to bottom at its lowest.
func (c *Canvas) FillPathV(p *Path, top, bottom Color) {
	if top == bottom {
		c.DrawPath(p, PathStyle{Fill: top})
		return
	}
	if top == (Color{}) {
		top.R = 1 // keep "transparent black at the top" from reading as "no fill"
	}
	c.DrawPath(p, PathStyle{Fill: top, FillTo: bottom})
}

// StrokePath draws the outline of p, centred on it, with mitred corners and
// butt caps. Use DrawPath for other joins and caps.
func (c *Canvas) StrokePath(p *Path, width float32, col Color) {
	c.DrawPath(p, PathStyle{Stroke: col, Width: width})
}

// DrawPath renders p as st describes: the fill first, then the outline on
// top.
func (c *Canvas) DrawPath(p *Path, st PathStyle) {
	if p == nil || c.Dst == nil {
		return
	}
	fill := st.Fill != (Color{}) && (st.Fill.A != 0 || st.FillTo.A != 0)
	stroke := st.Stroke.A != 0 && st.Width > 0
	if !fill && !stroke {
		return
	}
	s := c.scale()
	var m ebiten.GeoM
	if st.Scale != 0 && st.Scale != 1 {
		m.Scale(float64(st.Scale), float64(st.Scale))
	}
	if st.Rotation != 0 {
		m.Rotate(float64(st.Rotation))
	}
	m.Translate(float64(st.X), float64(st.Y))
	m.Scale(float64(s), float64(s))
	m.Translate(float64(c.ox), float64(c.oy))

	pathPx.Reset()
	pathPx.AddPath(&p.p, &vector.AddPathOptions{GeoM: m})
	b := pathPx.Bounds()
	if b.Empty() && !stroke {
		return
	}
	wp := st.Width * s
	margin := float32(1)
	if stroke {
		margin += wp * miterLimit / 2
	}
	if !c.visible(float32(b.Min.X)-margin, float32(b.Min.Y)-margin, float32(b.Max.X)+margin, float32(b.Max.Y)+margin) {
		return
	}

	if fill {
		pathFillOpts.FillRule = vector.FillRuleNonZero
		if st.EvenOdd {
			pathFillOpts.FillRule = vector.FillRuleEvenOdd
		}
		if st.FillTo != (Color{}) && st.FillTo != st.Fill {
			// The mask pass rebuilds pathPx in mask coordinates, so the
			// stroke below needs it restored.
			c.fillPixelPathV(b, st.Fill, st.FillTo, func(dx, dy float64) {
				mm := m
				mm.Translate(dx, dy)
				pathPx.Reset()
				pathPx.AddPath(&p.p, &vector.AddPathOptions{GeoM: mm})
			})
		} else {
			c.fillPixelPath(st.Fill)
		}
	}
	if stroke {
		pathStroke = vector.StrokeOptions{Width: wp, MiterLimit: miterLimit}
		switch st.Join {
		case JoinRound:
			pathStroke.LineJoin = vector.LineJoinRound
		case JoinBevel:
			pathStroke.LineJoin = vector.LineJoinBevel
		}
		switch st.Cap {
		case CapRound:
			pathStroke.LineCap = vector.LineCapRound
		case CapSquare:
			pathStroke.LineCap = vector.LineCapSquare
		}
		pathDrawOpts.ColorScale.Reset()
		pathDrawOpts.ColorScale.Scale(premul(st.Stroke))
		vector.StrokePath(c.Dst, &pathPx, &pathStroke, &pathDrawOpts)
	}
}

// fillPixelPath fills pathPx on the destination with a flat colour.
func (c *Canvas) fillPixelPath(col Color) {
	pathDrawOpts.ColorScale.Reset()
	pathDrawOpts.ColorScale.Scale(premul(col))
	vector.FillPath(c.Dst, &pathPx, &pathFillOpts, &pathDrawOpts)
}

// fillPixelPathV fills the path whose pixel bounds are b with a vertical
// gradient. The vector package only fills in flat colours, so the path's
// coverage is first rendered in white into a scratch image and then copied
// to the destination through a quad whose vertex colours carry the
// gradient. shift must rebuild pathPx translated by (dx, dy) pixels; a
// final shift(0, 0) restores it.
func (c *Canvas) fillPixelPathV(b image.Rectangle, top, bottom Color, shift func(dx, dy float64)) {
	// One pixel of margin for anti-aliased edges, then only the part that
	// can be seen.
	area := image.Rect(b.Min.X-1, b.Min.Y-1, b.Max.X+1, b.Max.Y+1).Intersect(c.Dst.Bounds())
	if area.Empty() {
		return
	}
	w, h := area.Dx(), area.Dy()
	if maskImage == nil || maskImage.Bounds().Dx() < w || maskImage.Bounds().Dy() < h {
		mw, mh := (w+255)&^255, (h+255)&^255
		if maskImage != nil {
			mw = max(mw, maskImage.Bounds().Dx())
			mh = max(mh, maskImage.Bounds().Dy())
			maskImage.Deallocate()
		}
		maskImage = ebiten.NewImage(mw, mh)
	}
	mask := maskImage.SubImage(image.Rect(0, 0, w, h)).(*ebiten.Image)
	mask.Clear()

	shift(-float64(area.Min.X), -float64(area.Min.Y))
	pathDrawOpts.ColorScale.Reset()
	vector.FillPath(mask, &pathPx, &pathFillOpts, &pathDrawOpts)
	shift(0, 0)

	p := vgrad(top, bottom, float32(b.Min.Y), float32(b.Max.Y))
	ct, cb := p.at(float32(area.Min.Y)), p.at(float32(area.Max.Y))
	quad := [4]struct {
		x, y int
		c    pcol
	}{
		{area.Min.X, area.Min.Y, ct}, {area.Max.X, area.Min.Y, ct},
		{area.Max.X, area.Max.Y, cb}, {area.Min.X, area.Max.Y, cb},
	}
	for i, q := range quad {
		maskVerts[i] = ebiten.Vertex{
			DstX: float32(q.x), DstY: float32(q.y),
			SrcX: float32(q.x - area.Min.X), SrcY: float32(q.y - area.Min.Y),
			ColorR: q.c.r, ColorG: q.c.g, ColorB: q.c.b, ColorA: q.c.a,
		}
	}
	c.Dst.DrawTriangles32(maskVerts[:], glowIndices[:], mask, &maskOpts)
}

// fillConcave fills the pixel-space polygon px, which need not be convex.
func (c *Canvas) fillConcave(px []Pt, top, bottom Color) {
	build := func(dx, dy float64) {
		pathPx.Reset()
		pathPx.MoveTo(px[0].X+float32(dx), px[0].Y+float32(dy))
		for _, q := range px[1:] {
			pathPx.LineTo(q.X+float32(dx), q.Y+float32(dy))
		}
		pathPx.Close()
	}
	build(0, 0)
	pathFillOpts.FillRule = vector.FillRuleNonZero
	if top == bottom {
		c.fillPixelPath(top)
		return
	}
	c.fillPixelPathV(pathPx.Bounds(), top, bottom, build)
}
