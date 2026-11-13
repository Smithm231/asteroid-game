package gfx

// Corners selects corners of a rectangle, for shapes that treat them
// individually. Combine values with |.
type Corners uint8

// Corner selections.
const (
	CornerTL Corners = 1 << iota // top-left
	CornerTR                     // top-right
	CornerBR                     // bottom-right
	CornerBL                     // bottom-left

	CornersNone   Corners = 0
	CornersTop            = CornerTL | CornerTR
	CornersBottom         = CornerBL | CornerBR
	CornersLeft           = CornerTL | CornerBL
	CornersRight          = CornerTR | CornerBR
	CornersDiag           = CornerTL | CornerBR // the two corners on the main diagonal
	CornersAll            = CornerTL | CornerTR | CornerBR | CornerBL
)

// appendChamfer appends the outline of the rectangle (x0, y0)-(x1, y1) with
// the selected corners cut off at 45° by cut, clockwise on screen from the
// top-left corner. Uncut corners contribute a single point. The cut is
// limited to half the shorter side.
func appendChamfer(dst []Pt, x0, y0, x1, y1, cut float32, corners Corners) []Pt {
	cut = max(0, min(cut, (x1-x0)/2, (y1-y0)/2))
	if cut == 0 {
		corners = CornersNone
	}
	if corners&CornerTL != 0 {
		dst = append(dst, Pt{x0, y0 + cut}, Pt{x0 + cut, y0})
	} else {
		dst = append(dst, Pt{x0, y0})
	}
	if corners&CornerTR != 0 {
		dst = append(dst, Pt{x1 - cut, y0}, Pt{x1, y0 + cut})
	} else {
		dst = append(dst, Pt{x1, y0})
	}
	if corners&CornerBR != 0 {
		dst = append(dst, Pt{x1, y1 - cut}, Pt{x1 - cut, y1})
	} else {
		dst = append(dst, Pt{x1, y1})
	}
	if corners&CornerBL != 0 {
		dst = append(dst, Pt{x0 + cut, y1}, Pt{x0, y1 - cut})
	} else {
		dst = append(dst, Pt{x0, y1})
	}
	return dst
}

// ChamferPoints returns the outline of r with the selected corners cut off
// at 45° by cut points, clockwise on screen from the top-left corner. It is
// the shape FillChamfer fills, before pixel snapping; use it for hit tests
// or to build paths.
func ChamferPoints(r Rect, cut float32, corners Corners) []Pt {
	return appendChamfer(make([]Pt, 0, 8), r.X, r.Y, r.X+r.W, r.Y+r.H, cut, corners)
}

// chamferPixels fills mb.pts with the pixel-snapped chamfered outline of r.
// ok is false when there is nothing to draw.
func (c *Canvas) chamferPixels(r Rect, cut float32, corners Corners) (pts []Pt, y0, y1 float32, ok bool) {
	x0, y0, x1, y1, ok := c.snapRect(r)
	if !ok {
		return nil, 0, 0, false
	}
	cp := float32(0)
	if cut > 0 {
		cp = c.snapLen(cut)
	}
	mb.pts = appendChamfer(mb.pts[:0], x0, y0, x1, y1, cp, corners)
	return mb.dedupe(mb.pts, true), y0, y1, true
}

// FillChamfer fills r with all four corners cut off at 45° by cut points:
// the cut-corner panel shape of the game's interface. The straight edges are
// snapped to pixels and the cuts are anti-aliased.
func (c *Canvas) FillChamfer(r Rect, cut float32, col Color) {
	c.FillChamferC(r, cut, CornersAll, col)
}

// FillChamferC is FillChamfer with a choice of which corners are cut.
func (c *Canvas) FillChamferC(r Rect, cut float32, corners Corners, col Color) {
	c.FillChamferV(r, cut, corners, col, col)
}

// FillChamferV fills a chamfered rectangle with a vertical gradient from
// top to bottom.
func (c *Canvas) FillChamferV(r Rect, cut float32, corners Corners, top, bottom Color) {
	if top.A == 0 && bottom.A == 0 {
		return
	}
	pts, y0, y1, ok := c.chamferPixels(r, cut, corners)
	if !ok || len(pts) < 3 {
		return
	}
	p := vgrad(top, bottom, y0, y1)
	mb.reset()
	mb.convex(pts, &p)
	c.flush()
}

// StrokeChamfer draws a frame of the given width just inside the outline of
// the chamfered rectangle (all four corners cut), so that it exactly covers
// the rim of the same shape drawn with FillChamfer. The width is rounded to
// whole pixels, at least one.
func (c *Canvas) StrokeChamfer(r Rect, cut, width float32, col Color) {
	c.StrokeChamferC(r, cut, width, CornersAll, col)
}

// StrokeChamferC is StrokeChamfer with a choice of which corners are cut.
func (c *Canvas) StrokeChamferC(r Rect, cut, width float32, corners Corners, col Color) {
	if col.A == 0 || !(width > 0) {
		return
	}
	pts, _, _, ok := c.chamferPixels(r, cut, corners)
	if !ok || len(pts) < 3 {
		return
	}
	w := c.snapLen(width)
	mb.reset()
	mb.stroke(pts, true, -w/2, w, toPcol(col), CapButt)
	c.flush()
}

// Panel draws a chamfered rectangle filled with fill and framed, just inside
// its outline, by a hairline in border: the standard panel of the
// interface, in a single draw call. A transparent border draws the fill
// only, and vice versa.
func (c *Canvas) Panel(r Rect, cut float32, corners Corners, fill, border Color) {
	c.PanelV(r, cut, corners, fill, fill, border, 1)
}

// PanelV is Panel with a vertical gradient fill from top to bottom and a
// frame of the given width in points.
func (c *Canvas) PanelV(r Rect, cut float32, corners Corners, top, bottom, border Color, width float32) {
	hasFill := top.A != 0 || bottom.A != 0
	hasBorder := border.A != 0 && width > 0
	if !hasFill && !hasBorder {
		return
	}
	pts, y0, y1, ok := c.chamferPixels(r, cut, corners)
	if !ok || len(pts) < 3 {
		return
	}
	mb.reset()
	if hasFill {
		p := vgrad(top, bottom, y0, y1)
		mb.convex(pts, &p)
	}
	if hasBorder {
		w := c.snapLen(width)
		mb.stroke(pts, true, -w/2, w, toPcol(border), CapButt)
	}
	c.flush()
}

// shadowProfile describes the falloff of Shadow: at each distance from the
// rectangle's edge (as a fraction of the blur; negative is inside) the
// fraction of the colour's opacity. It approximates a smooth step.
var shadowProfile = [...]struct{ at, alpha float32 }{
	{-1, 1}, {-0.4, 0.78}, {0, 0.5}, {0.45, 0.19}, {1, 0},
}

// Shadow draws a soft shadow of the rectangle r: opaque (at col's alpha)
// well inside it, half strength along its edges and fading to nothing blur
// points outside, with rounded-off corners. Draw it before the thing that
// casts it, usually shifted a few points down:
//
//	cv.Shadow(win.Translate(0, 4), 12, th.Shadow)
func (c *Canvas) Shadow(r Rect, blur float32, col Color) {
	if col.A == 0 || c.Dst == nil || !(r.W > 0 && r.H > 0) {
		return
	}
	if !(blur > 0) {
		c.FillRect(r, col)
		return
	}
	s := c.scale()
	x0, y0 := c.px(r.X, r.Y)
	x1, y1 := c.px(r.X+r.W, r.Y+r.H)
	b := blur * s
	if !c.visible(x0-b, y0-b, x1+b, y1+b) {
		return
	}
	full := toPcol(col)
	half := min(x1-x0, y1-y0) / 2

	mb.reset()
	base := uint32(len(mb.v))
	for _, step := range shadowProfile {
		e := max(step.at*b, -half) // never inset past the centre
		ex0, ey0, ex1, ey1 := x0-e, y0-e, x1+e, y1+e
		cut := min(max(0, e+b)*0.6, (ex1-ex0)/2, (ey1-ey0)/2)
		// Always eight vertices, so that consecutive rings pair up; with no
		// cut the corner points simply coincide.
		mb.pts = append(mb.pts[:0],
			Pt{ex0, ey0 + cut}, Pt{ex0 + cut, ey0}, Pt{ex1 - cut, ey0}, Pt{ex1, ey0 + cut},
			Pt{ex1, ey1 - cut}, Pt{ex1 - cut, ey1}, Pt{ex0 + cut, ey1}, Pt{ex0, ey1 - cut})
		pc := full.scale(step.alpha)
		for _, q := range mb.pts {
			mb.vert(q.X, q.Y, pc)
		}
	}
	for i := uint32(1); i+1 < 8; i++ {
		mb.tri(base, base+i, base+i+1)
	}
	for ring := uint32(0); ring+1 < uint32(len(shadowProfile)); ring++ {
		a, bq := base+8*ring, base+8*(ring+1)
		for i := uint32(0); i < 8; i++ {
			j := (i + 1) % 8
			mb.quad(a+i, bq+i, bq+j, a+j)
		}
	}
	c.flush()
}
