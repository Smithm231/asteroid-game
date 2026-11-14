package art

import (
	"math"
	"sort"

	"drifthold/internal/gfx"
)

// This file is the toolkit for drawing volumes in the angled surface view.
//
// A structure is authored in the frame of its tile: the origin is the tile's
// centre on the ground, x runs to the right, y towards the viewer, both in
// units of the tile's corner radius. The ground is foreshortened by Tilt, so
// the ground point (x, y) is drawn at (x, y·Tilt); a point h above it is
// drawn h higher on screen, so heights are simply in units of screen rise.
//
// Boxes stand corner-on to the viewer, the classic isometric way: their
// edges run along the two diagonals A = (1, 1) (towards the lower right of
// the screen) and B = (-1, 1) (towards the lower left), so that a box shows
// its top, its left wall and its right wall. The light comes from the upper
// left: tops take a material's light tone, left walls its plain tone, right
// walls its dark tone, and anything in between is toned by the way it
// faces.

// at projects the ground point (x, y), raised by h, to canvas points.
func (b *bpen) at(x, y, h float32) gfx.Pt {
	if b.ext != nil {
		b.ext.add(x, y, h)
	}
	return gfx.Pt{X: b.ox + x*b.k, Y: b.oy + (y*Tilt-h)*b.k}
}

// soft is at for things without substance (light, smoke), which do not
// count towards the extent of a structure.
func (b *bpen) soft(x, y, h float32) gfx.Pt {
	return gfx.Pt{X: b.ox + x*b.k, Y: b.oy + (y*Tilt-h)*b.k}
}

// pts3 projects a flat list of x, y, h triples into the scratch buffer.
func (b *bpen) pts3(xyh []float32) []gfx.Pt {
	ptsBuf = ptsBuf[:0]
	for i := 0; i+2 < len(xyh); i += 3 {
		ptsBuf = append(ptsBuf, b.at(xyh[i], xyh[i+1], xyh[i+2]))
	}
	return ptsBuf
}

// face fills the polygon through the given x, y, h triples.
func (b *bpen) face(c gfx.Color, xyh ...float32) {
	b.cv.FillPoly(b.pts3(xyh), b.col(c))
}

// loop strokes the closed outline through the given triples.
func (b *bpen) loop(c gfx.Color, width, minPx float32, xyh ...float32) {
	b.cv.StrokePoly(b.pts3(xyh), b.w(width, minPx), b.col(c))
}

// edge strokes the open line through the given triples.
func (b *bpen) edge(c gfx.Color, width, minPx float32, xyh ...float32) {
	b.cv.Polyline(b.pts3(xyh), b.w(width, minPx), b.col(c))
}

// hairline is the width, in units, of the fine lines that pick out the
// inner edges of a volume.
func (b *bpen) hairline() float32 { return max(0.022, 0.8*b.px) }

// wallTone returns the tone of an upright wall whose outward normal on the
// ground has x component nx (-1 faces left, +1 right): the left-hand wall of
// a box gets the plain tone, its right-hand wall the dark one.
func wallTone(m mat3, nx float32) gfx.Color {
	t := (nx + 0.7071) / 1.4142
	if t < 0 {
		return gfx.Lerp(m.mid, m.hi, min(1, -t)*0.45)
	}
	return gfx.Lerp(m.mid, m.lo, min(1, t))
}

// slopeTone returns the tone of a roof slope whose outward normal on the
// ground is (nx, ny): slopes turned to the upper left are the lightest.
func slopeTone(m mat3, nx, ny float32) gfx.Color {
	d := -0.8*nx - 0.3*ny
	if d >= 0 {
		return gfx.Lerp(m.mid, m.hi, min(1, d*1.1))
	}
	return gfx.Lerp(m.mid, m.lo, min(1, -d*1.1))
}

// hullBuf is the scratch space of outlineOf.
var hullBuf, hullOut []gfx.Pt

// outlineOf strokes the silhouette (the convex hull) of the given canvas
// points in ink.
func (b *bpen) outlineOf(pts []gfx.Pt) {
	if len(pts) < 3 {
		return
	}
	hullBuf = append(hullBuf[:0], pts...)
	sort.Slice(hullBuf, func(i, j int) bool {
		if hullBuf[i].X != hullBuf[j].X {
			return hullBuf[i].X < hullBuf[j].X
		}
		return hullBuf[i].Y < hullBuf[j].Y
	})
	cross := func(o, p, q gfx.Pt) float32 { return (p.X-o.X)*(q.Y-o.Y) - (p.Y-o.Y)*(q.X-o.X) }
	out := hullOut[:0]
	for _, p := range hullBuf {
		for len(out) >= 2 && cross(out[len(out)-2], out[len(out)-1], p) <= 0 {
			out = out[:len(out)-1]
		}
		out = append(out, p)
	}
	lower := len(out) + 1
	for i := len(hullBuf) - 2; i >= 0; i-- {
		p := hullBuf[i]
		for len(out) >= lower && cross(out[len(out)-2], out[len(out)-1], p) <= 0 {
			out = out[:len(out)-1]
		}
		out = append(out, p)
	}
	out = out[:len(out)-1]
	hullOut = out
	b.cv.StrokePoly(out, b.w(b.ow(), 1), b.col(inkLine))
}

// ---------------------------------------------------------------------------
// Boxes
// ---------------------------------------------------------------------------

// ibox is the footprint of a box standing corner-on: centred on (x, y) on
// the ground, reaching da along the diagonal A (to the lower right) and db
// along the diagonal B (to the lower left) either way.
type ibox struct{ x, y, da, db float32 }

// pt returns the ground point at (a, c) in the box's own frame, where the
// footprint spans -1..1 along both diagonals. The left corner is (-1, 1),
// the front corner (1, 1), the right corner (1, -1), the back corner
// (-1, -1).
func (q ibox) pt(a, c float32) (x, y float32) {
	return q.x + a*q.da - c*q.db, q.y + a*q.da + c*q.db
}

// box draws a box from height h0 to h1: left wall, right wall, top, the
// fine lines of its near edges and its silhouette.
func (b *bpen) box(m mat3, q ibox, h0, h1 float32) {
	lx, ly := q.pt(-1, 1)
	fx, fy := q.pt(1, 1)
	rx, ry := q.pt(1, -1)
	kx, ky := q.pt(-1, -1)
	b.face(m.mid, lx, ly, h0, fx, fy, h0, fx, fy, h1, lx, ly, h1)
	b.face(m.lo, fx, fy, h0, rx, ry, h0, rx, ry, h1, fx, fy, h1)
	b.face(m.hi, lx, ly, h1, fx, fy, h1, rx, ry, h1, kx, ky, h1)
	hl := b.hairline()
	if !b.tiny() {
		b.edge(alpha(gfx.Darken(m.lo, 0.35), 0.7), hl, 0, fx, fy, h0, fx, fy, h1)
	}
	b.edge(gfx.Lighten(m.hi, 0.45), hl, 0, lx, ly, h1, fx, fy, h1, rx, ry, h1)
	b.loop(inkLine, b.ow(), 1, lx, ly, h0, fx, fy, h0, rx, ry, h0, rx, ry, h1, kx, ky, h1, lx, ly, h1)
}

// wallL paints a panel on the left wall of the box q: from a0 to a1 along
// the wall (-1 at the left corner, 1 at the front corner), from height h0
// to h1.
func (b *bpen) wallL(c gfx.Color, q ibox, a0, a1, h0, h1 float32) {
	x0, y0 := q.pt(a0, 1)
	x1, y1 := q.pt(a1, 1)
	b.face(c, x0, y0, h0, x1, y1, h0, x1, y1, h1, x0, y0, h1)
}

// wallR paints a panel on the right wall of the box q: from c0 to c1 along
// the wall (1 at the front corner, -1 at the right corner).
func (b *bpen) wallR(c gfx.Color, q ibox, c0, c1, h0, h1 float32) {
	x0, y0 := q.pt(1, c0)
	x1, y1 := q.pt(1, c1)
	b.face(c, x0, y0, h0, x1, y1, h0, x1, y1, h1, x0, y0, h1)
}

// topQ paints a panel lying on the top of the box q at height h, spanning
// a0..a1 and c0..c1 in the box's frame.
func (b *bpen) topQ(c gfx.Color, q ibox, a0, a1, c0, c1, h float32) {
	x0, y0 := q.pt(a0, c1)
	x1, y1 := q.pt(a1, c1)
	x2, y2 := q.pt(a1, c0)
	x3, y3 := q.pt(a0, c0)
	b.face(c, x0, y0, h, x1, y1, h, x2, y2, h, x3, y3, h)
}

// winL draws a row of n lit windows along the left wall of q between a0
// and a1, from height h0 to h1.
func (b *bpen) winL(c gfx.Color, q ibox, n int, a0, a1, h0, h1 float32) {
	step := (a1 - a0) / float32(n)
	for i := 0; i < n; i++ {
		s := a0 + step*(float32(i)+0.22)
		b.wallL(b.lamp(c), q, s, s+step*0.56, h0, h1)
	}
}

// winR is winL for the right wall.
func (b *bpen) winR(c gfx.Color, q ibox, n int, c0, c1, h0, h1 float32) {
	step := (c1 - c0) / float32(n)
	for i := 0; i < n; i++ {
		s := c0 + step*(float32(i)+0.22)
		b.wallR(b.lamp(c), q, s, s+step*0.56, h0, h1)
	}
}

// ---------------------------------------------------------------------------
// Prisms and pyramids over any convex footprint
// ---------------------------------------------------------------------------

// prism draws an upright prism over the convex ground polygon given as x, y
// pairs, from height h0 to h1: every wall that faces the viewer, then the
// top.
func (b *bpen) prism(m mat3, h0, h1 float32, xy ...float32) {
	n := len(xy) / 2
	if n < 3 {
		return
	}
	sgn := polySign(xy)
	var all [64]gfx.Pt
	pts := all[:0]
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		x0, y0, x1, y1 := xy[2*i], xy[2*i+1], xy[2*j], xy[2*j+1]
		pts = append(pts, b.at(x0, y0, h0), b.at(x0, y0, h1))
		// Outward normal of a clockwise edge (dx, dy) is (dy, -dx).
		nx, ny := sgn*(y1-y0), -sgn*(x1-x0)
		if ny <= 1e-4 {
			continue
		}
		l := float32(math.Hypot(float64(nx), float64(ny)))
		b.face(wallTone(m, nx/l), x0, y0, h0, x1, y1, h0, x1, y1, h1, x0, y0, h1)
	}
	var top [96]float32
	t := top[:0]
	for i := 0; i < n; i++ {
		t = append(t, xy[2*i], xy[2*i+1], h1)
	}
	b.face(m.hi, t...)
	// Fine lines along the top's near edges and down the corners between
	// walls.
	hl := b.hairline()
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		x0, y0, x1, y1 := xy[2*i], xy[2*i+1], xy[2*j], xy[2*j+1]
		if ny := -sgn * (x1 - x0); ny > 1e-4 {
			b.edge(gfx.Lighten(m.hi, 0.45), hl, 0, x0, y0, h1, x1, y1, h1)
			if !b.tiny() {
				b.edge(alpha(gfx.Darken(m.lo, 0.35), 0.5), hl, 0, x0, y0, h0, x0, y0, h1)
			}
		}
	}
	b.outlineOf(pts)
}

// polySign returns 1 for a ground polygon given clockwise on screen and -1
// for an anticlockwise one.
func polySign(xy []float32) float32 {
	n := len(xy) / 2
	var area float32
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += xy[2*i]*xy[2*j+1] - xy[2*j]*xy[2*i+1]
	}
	if area < 0 {
		return -1
	}
	return 1
}

// spire draws a roof rising from the convex ground polygon at height h0
// to an apex above (ax, ay) at height h1. All its slopes are drawn, far
// ones first.
func (b *bpen) spire(m mat3, h0, h1, ax, ay float32, xy ...float32) {
	n := len(xy) / 2
	if n < 3 {
		return
	}
	sgn := polySign(xy)
	var all [40]gfx.Pt
	pts := append(all[:0], b.at(ax, ay, h1))
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			x0, y0, x1, y1 := xy[2*i], xy[2*i+1], xy[2*j], xy[2*j+1]
			nx, ny := sgn*(y1-y0), -sgn*(x1-x0)
			if (ny > 0) != (pass == 1) {
				continue
			}
			l := float32(math.Hypot(float64(nx), float64(ny)))
			b.face(slopeTone(m, nx/l, ny/l), x0, y0, h0, x1, y1, h0, ax, ay, h1)
			pts = append(pts, b.at(x0, y0, h0))
		}
	}
	if !b.tiny() {
		hl := b.hairline()
		for i := 0; i < n; i++ {
			b.edge(alpha(gfx.Lighten(m.hi, 0.4), 0.7), hl, 0, xy[2*i], xy[2*i+1], h0, ax, ay, h1)
		}
	}
	b.outlineOf(pts)
}

// wallQ paints a panel on the upright wall that stands on the ground line
// from (x0, y0) to (x1, y1): from u0 to u1 along it (0..1), from height h0
// to h1.
func (b *bpen) wallQ(c gfx.Color, x0, y0, x1, y1, u0, u1, h0, h1 float32) {
	ax, ay := x0+(x1-x0)*u0, y0+(y1-y0)*u0
	bx, by := x0+(x1-x0)*u1, y0+(y1-y0)*u1
	b.face(c, ax, ay, h0, bx, by, h0, bx, by, h1, ax, ay, h1)
}

// hexa draws a six-sided block that need not be upright: bot holds the four
// corners of its base on the ground plane at height h0, top the four
// corners of its upper face at height h1, corner for corner. The sides that
// face the viewer are drawn, then the top.
func (b *bpen) hexa(m mat3, h0, h1 float32, bot, top [8]float32) {
	sgn := polySign(bot[:])
	var all [8]gfx.Pt
	for i := 0; i < 4; i++ {
		all[i] = b.at(bot[2*i], bot[2*i+1], h0)
		all[4+i] = b.at(top[2*i], top[2*i+1], h1)
	}
	hl := b.hairline()
	for i := 0; i < 4; i++ {
		j := (i + 1) % 4
		p0, p1, p2, p3 := all[i], all[j], all[4+j], all[4+i]
		area := p0.X*p1.Y - p1.X*p0.Y + p1.X*p2.Y - p2.X*p1.Y + p2.X*p3.Y - p3.X*p2.Y + p3.X*p0.Y - p0.X*p3.Y
		if area*sgn <= 0 {
			continue
		}
		nx, ny := sgn*(bot[2*j+1]-bot[2*i+1]), -sgn*(bot[2*j]-bot[2*i])
		l := float32(math.Hypot(float64(nx), float64(ny)))
		if l == 0 {
			continue
		}
		b.cv.FillPoly([]gfx.Pt{p0, p1, p2, p3}, b.col(wallTone(m, nx/l)))
		b.cv.Line(p3.X, p3.Y, p2.X, p2.Y, hl*b.k, b.col(gfx.Lighten(m.hi, 0.45)))
	}
	b.cv.FillPoly(all[4:], b.col(m.hi))
	b.outlineOf(all[:])
}

// ngon returns the corners of a regular polygon on the ground as x, y
// pairs: n corners at distance r from (cx, cy), the first at ground angle
// a0 (0 points right, π/2 towards the viewer).
func ngon(dst []float32, cx, cy, r float32, n int, a0 float32) []float32 {
	for i := 0; i < n; i++ {
		a := a0 + float32(i)*tau/float32(n)
		dst = append(dst, cx+r*cosf(a), cy+r*sinf(a))
	}
	return dst
}

// ---------------------------------------------------------------------------
// Round things
// ---------------------------------------------------------------------------

// roundSteps returns how many straight pieces to draw a ground circle of
// radius r with.
func (b *bpen) roundSteps(r float32) int {
	n := int(r*b.unit*0.9) + 8
	return min(max(n&^1, 10), 28)
}

// ring3 returns the canvas points of the ground circle of radius r around
// (cx, cy) at height h.
func (b *bpen) ring3(dst []gfx.Pt, cx, cy, h, r float32, n int) []gfx.Pt {
	for i := 0; i < n; i++ {
		a := tau * float32(i) / float32(n)
		dst = append(dst, b.at(cx+r*cosf(a), cy+r*sinf(a), h))
	}
	return dst
}

// disc fills a flat round plate: the ground circle of radius r around
// (cx, cy) at height h.
func (b *bpen) disc(c gfx.Color, cx, cy, h, r float32) {
	if b.live {
		b.stampOval(b.at(cx, cy, h), r*b.k, r*Tilt*b.k, 0, 0, c)
		return
	}
	var buf [32]gfx.Pt
	b.cv.FillPoly(b.ring3(buf[:0], cx, cy, h, r, b.roundSteps(r)), b.col(c))
}

// hoop strokes the ground circle of radius r around (cx, cy) at height h.
func (b *bpen) hoop(c gfx.Color, width, minPx, cx, cy, h, r float32) {
	if b.live {
		// A live hoop is a ring of light.
		b.stampRing(b.soft(cx, cy, h), r*b.k, r*Tilt*b.k, c)
		return
	}
	var buf [32]gfx.Pt
	b.cv.StrokePoly(b.ring3(buf[:0], cx, cy, h, r, b.roundSteps(r)), b.w(width, minPx), b.col(c))
}

// arcG strokes part of a ground circle, from ground angle a0 to a1.
func (b *bpen) arcG(c gfx.Color, width, minPx, cx, cy, h, r, a0, a1 float32) {
	var buf [32]gfx.Pt
	pts := buf[:0]
	n := max(3, int(float32(b.roundSteps(r))*absf32(a1-a0)/tau)+2)
	for i := 0; i <= n; i++ {
		a := a0 + (a1-a0)*float32(i)/float32(n)
		pts = append(pts, b.at(cx+r*cosf(a), cy+r*sinf(a), h))
	}
	b.cv.Polyline(pts, b.w(width, minPx), b.col(c))
}

func absf32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// frustum draws an upright round body around (cx, cy): radius r0 at height
// h0, r1 at h1. With r0 = r1 it is a cylinder, with r1 = 0 a cone. Its side
// is a fan of flat facets, light on the left and dark on the right.
func (b *bpen) frustum(m mat3, cx, cy, r0, r1, h0, h1 float32) {
	n := b.roundSteps(max(r0, r1))
	var all [64]gfx.Pt
	pts := b.ring3(all[:0], cx, cy, h0, r0, n)
	if r1 > 0 {
		pts = b.ring3(pts, cx, cy, h1, r1, n)
	} else {
		pts = append(pts, b.at(cx, cy, h1))
	}
	// The near half of the side, from the left round the front to the
	// right.
	facets := n / 2
	for i := 0; i < facets; i++ {
		a0 := pi - pi*float32(i)/float32(facets)
		a1 := pi - pi*float32(i+1)/float32(facets)
		c0, s0, c1, s1 := cosf(a0), sinf(a0), cosf(a1), sinf(a1)
		b.face(wallTone(m, cosf((a0+a1)/2)),
			cx+r0*c0, cy+r0*s0, h0, cx+r0*c1, cy+r0*s1, h0,
			cx+r1*c1, cy+r1*s1, h1, cx+r1*c0, cy+r1*s0, h1)
	}
	if r1 > 0 {
		b.disc(m.hi, cx, cy, h1, r1)
		if !b.tiny() {
			b.arcG(gfx.Lighten(m.hi, 0.45), b.hairline(), 0, cx, cy, h1, r1, 0, pi)
		}
	}
	b.outlineOf(pts)
}

// cyl draws an upright cylinder.
func (b *bpen) cyl(m mat3, cx, cy, r, h0, h1 float32) {
	b.frustum(m, cx, cy, r, r, h0, h1)
}

// band paints a stripe round the near half of a cylinder of radius r,
// from height h0 to h1.
func (b *bpen) band(c gfx.Color, cx, cy, r, h0, h1 float32) {
	var buf [40]gfx.Pt
	pts := buf[:0]
	n := b.roundSteps(r) / 2
	for i := 0; i <= n; i++ {
		a := pi - pi*float32(i)/float32(n)
		pts = append(pts, b.at(cx+r*cosf(a), cy+r*sinf(a), h1))
	}
	for i := n; i >= 0; i-- {
		a := pi - pi*float32(i)/float32(n)
		pts = append(pts, b.at(cx+r*cosf(a), cy+r*sinf(a), h0))
	}
	b.cv.FillPoly(pts, b.col(c))
}

// domeUp draws a dome standing on the ground circle of radius r around
// (cx, cy) at height h0 and rising by rise: half a ball when rise is about
// r, flatter or taller otherwise.
func (b *bpen) domeUp(m mat3, cx, cy, r, h0, rise float32) {
	c := b.at(cx, cy, h0)
	rr := r * b.k
	// The silhouette of an ellipsoid seen at this angle is an ellipse
	// around its centre; the dome is its upper half on the near half of
	// the base.
	v := float32(math.Sqrt(float64(r*Tilt*r*Tilt+rise*rise))) * b.k
	low := r * Tilt * b.k
	n := b.roundSteps(r)
	shape := func(sx, sy, k float32) []gfx.Pt {
		var buf [64]gfx.Pt
		pts := buf[:0]
		for i := 0; i <= n; i++ {
			a := pi * float32(i) / float32(n)
			pts = append(pts, gfx.Pt{X: c.X + sx + rr*k*cosf(a), Y: c.Y + sy - v*k*sinf(a)})
		}
		for i := 1; i < n; i++ {
			a := pi + pi*float32(i)/float32(n)
			pts = append(pts, gfx.Pt{X: c.X + sx + rr*k*cosf(a), Y: c.Y + sy - low*k*sinf(a)})
		}
		return append([]gfx.Pt(nil), pts...)
	}
	full := shape(0, 0, 1)
	b.cv.FillPoly(full, b.col(m.lo))
	b.cv.FillPoly(shape(-0.1*rr, -0.07*v, 0.86), b.col(m.mid))
	b.cv.FillPoly(shape(-0.3*rr, -0.34*v, 0.42), b.col(m.hi))
	b.cv.StrokePoly(full, b.w(b.ow(), 1), b.col(inkLine))
}

// ball draws a sphere of radius r centred h above the ground point
// (cx, cy).
func (b *bpen) ball(m mat3, cx, cy, h, r float32) {
	c := b.at(cx, cy, h)
	rr := r * b.k
	b.cv.FillCircle(c.X, c.Y, rr, b.col(m.lo))
	b.cv.FillCircle(c.X-0.12*rr, c.Y-0.14*rr, rr*0.84, b.col(m.mid))
	b.cv.FillCircle(c.X-0.32*rr, c.Y-0.36*rr, rr*0.4, b.col(m.hi))
	b.cv.StrokeCircle(c.X, c.Y, rr, b.w(b.ow(), 1), b.col(inkLine))
}

// ---------------------------------------------------------------------------
// Pipes, beams, lights and shadows
// ---------------------------------------------------------------------------

// tube draws a round pipe or tank of radius r between two points in space:
// dark below, lit along the flank that faces the light.
func (b *bpen) tube(m mat3, x0, y0, h0, x1, y1, h1, r float32) {
	p, q := b.at(x0, y0, h0), b.at(x1, y1, h1)
	dx, dy := q.X-p.X, q.Y-p.Y
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l == 0 {
		return
	}
	// The unit normal of the pipe on screen that points towards the light.
	nx, ny := -dy/l, dx/l
	if nx*lightX+ny*lightY < 0 {
		nx, ny = -nx, -ny
	}
	rr := r * b.k
	line := func(off, width float32, c gfx.Color) {
		b.cv.StrokeLine(p.X+nx*off, p.Y+ny*off, q.X+nx*off, q.Y+ny*off, gfx.Stroke{Width: width, Cap: gfx.CapRound}, b.col(c))
	}
	line(0, 2*rr+2*b.w(b.ow(), 1), inkLine)
	line(0, 2*rr, m.lo)
	line(0.22*rr, 1.5*rr, m.mid)
	if rr*canvasScale(b.cv) > 2.5 {
		line(0.5*rr, 0.5*rr, m.hi)
	}
}

// beam draws a strut between two points in space: an ink casing round a
// core of colour c.
func (b *bpen) beam(c gfx.Color, width, x0, y0, h0, x1, y1, h1 float32) {
	p, q := b.at(x0, y0, h0), b.at(x1, y1, h1)
	if b.live {
		w := b.w(width, 1)
		b.stampBar(p, q, w, max(b.ow()*b.k, 0.7*b.px*b.k), c)
		return
	}
	b.cv.Line(p.X, p.Y, q.X, q.Y, b.w(width+2*b.ow(), 2.4), b.col(inkLine))
	b.cv.Line(p.X, p.Y, q.X, q.Y, b.w(width, 1), b.col(c))
}

// lampAt draws a round light at a point in space, with a halo while the
// lights are on.
func (b *bpen) lampAt(x, y, h, r float32, c gfx.Color) {
	p := b.at(x, y, h)
	if b.live {
		b.stampOval(p, max(r, 0.9*b.px)*b.k, max(r, 0.9*b.px)*b.k, 0, 0, b.lamp(c))
	} else {
		b.cv.FillCircle(p.X, p.Y, max(r, 0.9*b.px)*b.k, b.col(b.lamp(c)))
	}
	if b.lit {
		b.cv.Glow(p.X, p.Y, r*3.2*b.k, alpha(c, 0.32))
	}
}

// glowAt adds light around a point in space (never filtered: unlit
// variants leave their glows out).
func (b *bpen) glowAt(x, y, h, r float32, c gfx.Color) {
	p := b.soft(x, y, h)
	b.cv.Glow(p.X, p.Y, r*b.k, c)
}

// dotAt fills a small circle at a point in space.
func (b *bpen) dotAt(x, y, h, r float32, c gfx.Color) {
	p := b.at(x, y, h)
	if b.live {
		b.stampOval(p, r*b.k, r*b.k, 0, 0, c)
		return
	}
	b.cv.FillCircle(p.X, p.Y, r*b.k, b.col(c))
}

// shadowG lays the soft shadow of something standing on the ground round
// (cx, cy): rx wide and ry deep in ground units. The light is at the upper
// left, so it leans to the right.
func (b *bpen) shadowG(cx, cy, rx, ry float32) {
	p := b.at(cx+rx*0.16, cy+ry*0.1, 0)
	b.cv.Blob(p.X, p.Y, rx*1.25*b.k, ry*1.3*Tilt*b.k, b.col(inkShadow))
	b.cv.Blob(p.X, p.Y, rx*0.95*b.k, ry*0.95*Tilt*b.k, b.col(inkShadow))
}

// hit records a point on the structure where damage may show.
func (b *bpen) hit(x, y, h float32) {
	if b.nhits < len(b.hits) {
		b.hits[b.nhits] = [3]float32{x, y, h}
		b.nhits++
	}
}
