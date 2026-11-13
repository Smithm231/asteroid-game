package gfx

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// This file is the tessellator behind the Canvas primitives. It works in
// destination pixels and produces plain coloured triangles. Edges are
// anti-aliased with a feather: a one-pixel-wide strip straddling the true
// edge whose alpha ramps from full to zero, which reproduces exact box
// coverage for axis-aligned edges and a close approximation for slanted
// ones. Because the result is ordinary geometry drawn with the built-in
// shader from a white texel, Ebitengine merges it with neighbouring draws,
// text glyphs included.

// feather is half the width of the anti-aliasing ramp, in pixels.
const feather = 0.5

// miterLimit caps how far a corner may be pushed out, as a multiple of the
// offset distance. It corresponds to an interior angle of about 29°.
const miterLimit = 4

// pcol is a premultiplied-alpha colour in 0..1, the form vertices take.
type pcol struct{ r, g, b, a float32 }

func toPcol(c Color) pcol {
	r, g, b, a := premul(c)
	return pcol{r, g, b, a}
}

func (p pcol) scale(f float32) pcol { return pcol{p.r * f, p.g * f, p.b * f, p.a * f} }

func lerpPcol(a, b pcol, t float32) pcol {
	return pcol{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t, a.a + (b.a-a.a)*t}
}

// paint colours a filled shape: flat, or a vertical gradient from c0 at
// pixel row y0 to c1 at pixel row y1.
type paint struct {
	c0, c1 pcol
	y0, k  float32 // k = 1/(y1-y0)
	grad   bool
}

func flat(c Color) paint { return paint{c0: toPcol(c)} }

func vgrad(top, bottom Color, y0, y1 float32) paint {
	if top == bottom || !(y1 > y0) {
		return flat(top)
	}
	return paint{c0: toPcol(top), c1: toPcol(bottom), y0: y0, k: 1 / (y1 - y0), grad: true}
}

func (p *paint) at(y float32) pcol {
	if !p.grad {
		return p.c0
	}
	return lerpPcol(p.c0, p.c1, clampf((y-p.y0)*p.k, 0, 1))
}

// meshBuf accumulates the triangles of one draw call. Its slices are reused
// from call to call.
type meshBuf struct {
	v []ebiten.Vertex
	i []uint32

	pts  []Pt // scratch: pixel-space outline of the current primitive
	nrm  []Pt // scratch: unit normals of its edges
	src  []Pt // scratch: the outline being dashed
	dash []Pt // scratch: one dash of a dashed stroke
}

func (m *meshBuf) reset() {
	m.v = m.v[:0]
	m.i = m.i[:0]
}

// vert appends a vertex and returns its index.
func (m *meshBuf) vert(x, y float32, c pcol) uint32 {
	m.v = append(m.v, ebiten.Vertex{
		DstX: x, DstY: y,
		SrcX: whiteSrc, SrcY: whiteSrc,
		ColorR: c.r, ColorG: c.g, ColorB: c.b, ColorA: c.a,
	})
	return uint32(len(m.v) - 1)
}

func (m *meshBuf) tri(a, b, c uint32) { m.i = append(m.i, a, b, c) }

// quad appends the two triangles of the quadrilateral a-b-c-d (in order
// around its outline).
func (m *meshBuf) quad(a, b, c, d uint32) { m.i = append(m.i, a, b, c, a, c, d) }

// rect appends an axis-aligned rectangle with hard edges. Callers pass
// pixel-aligned coordinates, so no feathering is needed.
func (m *meshBuf) rect(x0, y0, x1, y1 float32, c00, c10, c11, c01 pcol) {
	a := m.vert(x0, y0, c00)
	b := m.vert(x1, y0, c10)
	c := m.vert(x1, y1, c11)
	d := m.vert(x0, y1, c01)
	m.quad(a, b, c, d)
}

// edgeNormals fills m.nrm with the unit normal of each edge of pts: n-1
// edges for an open polyline, n for a closed one. The normal of an edge with
// direction (dx, dy) is (dy, -dx); for a polygon with positive signed area
// (clockwise on screen) that points outwards. Zero-length edges must have
// been removed by the caller.
func (m *meshBuf) edgeNormals(pts []Pt, closed bool) []Pt {
	n := len(pts)
	segs := n - 1
	if closed {
		segs = n
	}
	m.nrm = m.nrm[:0]
	for s := 0; s < segs; s++ {
		a, b := pts[s], pts[(s+1)%n]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if l == 0 {
			l = 1
		}
		m.nrm = append(m.nrm, Pt{dy / l, -dx / l})
	}
	return m.nrm
}

// miter returns the vector that moves a corner so that both adjacent edges,
// with unit normals n0 and n1, shift outwards by one unit.
func miter(n0, n1 Pt) Pt {
	d := 1 + n0.X*n1.X + n0.Y*n1.Y
	if d < 2.0/(miterLimit*miterLimit) {
		// Nearly a hairpin: a true miter would shoot off to infinity. Fall
		// back to the limit along the bisector, or to one normal when the
		// bisector is undefined.
		bx, by := n0.X+n1.X, n0.Y+n1.Y
		l := float32(math.Sqrt(float64(bx*bx + by*by)))
		if l < 1e-6 {
			return n0
		}
		return Pt{bx / l * miterLimit, by / l * miterLimit}
	}
	return Pt{(n0.X + n1.X) / d, (n0.Y + n1.Y) / d}
}

// dedupe copies pts into m.pts, dropping points that repeat their
// predecessor (and, for closed outlines, a last point that repeats the
// first). It returns the cleaned slice, which aliases m.pts.
func (m *meshBuf) dedupe(pts []Pt, closed bool) []Pt {
	const eps = 1e-4
	out := m.pts[:0]
	for _, p := range pts {
		if n := len(out); n > 0 && absf(out[n-1].X-p.X) < eps && absf(out[n-1].Y-p.Y) < eps {
			continue
		}
		out = append(out, p)
	}
	if closed {
		for len(out) > 1 && absf(out[0].X-out[len(out)-1].X) < eps && absf(out[0].Y-out[len(out)-1].Y) < eps {
			out = out[:len(out)-1]
		}
	}
	m.pts = out
	return out
}

// signedArea returns twice the signed area of the polygon: positive when the
// points run clockwise on screen (y down).
func signedArea(pts []Pt) float32 {
	var a float32
	n := len(pts)
	for i := 0; i < n; i++ {
		p, q := pts[i], pts[(i+1)%n]
		a += p.X*q.Y - q.X*p.Y
	}
	return a
}

// isConvex reports whether the polygon is convex (all turns in the same
// direction, collinear points allowed).
func isConvex(pts []Pt) bool {
	n := len(pts)
	if n < 3 {
		return false
	}
	var sign float32
	for i := 0; i < n; i++ {
		a, b, c := pts[i], pts[(i+1)%n], pts[(i+2)%n]
		cross := (b.X-a.X)*(c.Y-b.Y) - (b.Y-a.Y)*(c.X-b.X)
		if absf(cross) < 1e-6 {
			continue
		}
		if sign == 0 {
			sign = cross
		} else if (cross > 0) != (sign > 0) {
			return false
		}
	}
	return sign != 0
}

// convex appends a filled convex polygon with feathered edges. pts are in
// pixels and must already be deduplicated (see dedupe); either winding
// works.
func (m *meshBuf) convex(pts []Pt, p *paint) {
	n := len(pts)
	if n < 3 {
		return
	}
	area := signedArea(pts)
	if area == 0 {
		return
	}
	sgn := float32(1)
	if area < 0 {
		sgn = -1
	}
	nrm := m.edgeNormals(pts, true)

	// How far the centroid is from the nearest edge bounds the inward
	// offset: a shape thinner than the feather collapses onto its centre and
	// is drawn fainter instead of inside out.
	var cx, cy float32
	for _, q := range pts {
		cx += q.X
		cy += q.Y
	}
	cx /= float32(n)
	cy /= float32(n)
	inr := float32(math.MaxFloat32)
	for i, q := range pts {
		d := sgn * (nrm[i].X*(q.X-cx) + nrm[i].Y*(q.Y-cy))
		inr = min(inr, d)
	}
	in, gain := float32(feather), float32(1)
	if inr < feather {
		inr = max(inr, 0)
		in = inr * 0.98
		gain = 2 * inr / (inr + feather)
	}

	base := uint32(len(m.v))
	for i, q := range pts {
		mt := miter(nrm[(i+n-1)%n], nrm[i])
		mx, my := sgn*mt.X, sgn*mt.Y
		ix, iy := q.X-mx*in, q.Y-my*in
		m.vert(ix, iy, p.at(iy).scale(gain))
		m.vert(q.X+mx*feather, q.Y+my*feather, pcol{})
	}
	for i := uint32(1); i+1 < uint32(n); i++ {
		m.tri(base, base+2*i, base+2*(i+1))
	}
	for i := uint32(0); i < uint32(n); i++ {
		j := (i + 1) % uint32(n)
		m.quad(base+2*i, base+2*i+1, base+2*j+1, base+2*j)
	}
}

// Cap is the shape of the two ends of an open stroke.
type Cap uint8

// Stroke end shapes.
const (
	CapButt   Cap = iota // cut square at the end point
	CapRound             // semicircle around the end point
	CapSquare            // cut square half a width beyond the end point
)

// stroke appends a polyline stroked w pixels wide. pts are in pixels and
// must already be deduplicated. The stroke is centred on the polyline
// shifted sideways by off pixels (positive towards the edge normal, which is
// outwards for a clockwise closed outline). cap applies to open polylines.
func (m *meshBuf) stroke(pts []Pt, closed bool, off, w float32, col pcol, cap Cap) {
	n := len(pts)
	if closed && n < 3 {
		closed = false
	}
	if n < 2 || !(w > 0) {
		return
	}
	// A line thinner than a pixel is drawn one pixel wide and fainter.
	if w < 1 {
		col = col.scale(w)
		w = 1
	}
	h := w / 2
	o := [4]float32{off - h - feather, off - h + feather, off + h - feather, off + h + feather}
	cols := [4]pcol{{}, col, col, {}}
	solid := o[2]-o[1] > 1e-4 // the stroke has an opaque core between the feathers

	nrm := m.edgeNormals(pts, closed)
	segs := len(nrm)

	// Open ends: square caps move the end point out by half the width, and
	// butt and square caps feather the cut across one pixel.
	first, last := pts[0], pts[n-1]
	d0 := Pt{-nrm[0].Y, nrm[0].X}           // direction of the first segment
	d1 := Pt{-nrm[segs-1].Y, nrm[segs-1].X} // direction of the last segment
	if !closed && cap == CapSquare {
		first = Pt{first.X - d0.X*h, first.Y - d0.Y*h}
		last = Pt{last.X + d1.X*h, last.Y + d1.Y*h}
	}
	var in0, in1 float32 // how far the end rings sit inside the cut
	if !closed && cap != CapRound {
		in0, in1 = feather, feather
	}

	base := uint32(len(m.v))
	for i := 0; i < n; i++ {
		q := pts[i]
		var mt Pt
		switch {
		case closed:
			mt = miter(nrm[(i+segs-1)%segs], nrm[i])
		case i == 0:
			mt = nrm[0]
			q = Pt{first.X + d0.X*in0, first.Y + d0.Y*in0}
		case i == n-1:
			mt = nrm[segs-1]
			q = Pt{last.X - d1.X*in1, last.Y - d1.Y*in1}
		default:
			mt = miter(nrm[i-1], nrm[i])
		}
		for k := 0; k < 4; k++ {
			m.vert(q.X+mt.X*o[k], q.Y+mt.Y*o[k], cols[k])
		}
	}
	for s := 0; s < segs; s++ {
		a := base + 4*uint32(s)
		b := base + 4*uint32((s+1)%n)
		m.quad(a, a+1, b+1, b)
		if solid {
			m.quad(a+1, a+2, b+2, b+1)
		}
		m.quad(a+2, a+3, b+3, b+2)
	}
	if closed {
		return
	}

	endRing := base + 4*uint32(n-1)
	if cap == CapRound {
		m.roundCap(first, nrm[0], Pt{-d0.X, -d0.Y}, base, h, col, solid)
		m.roundCap(last, nrm[segs-1], d1, endRing, h, col, solid)
		return
	}
	m.flatCap(Pt{first.X - d0.X*feather, first.Y - d0.Y*feather}, nrm[0], o, base, solid)
	m.flatCap(Pt{last.X + d1.X*feather, last.Y + d1.Y*feather}, nrm[segs-1], o, endRing, solid)
}

// flatCap closes a stroke end with a feathered straight cut: a ring of
// transparent vertices at q, joined to the end ring starting at index ring.
func (m *meshBuf) flatCap(q, nrm Pt, o [4]float32, ring uint32, solid bool) {
	e := uint32(len(m.v))
	for k := 0; k < 4; k++ {
		m.vert(q.X+nrm.X*o[k], q.Y+nrm.Y*o[k], pcol{})
	}
	m.quad(ring, ring+1, e+1, e)
	if solid {
		m.quad(ring+1, ring+2, e+2, e+1)
	}
	m.quad(ring+2, ring+3, e+3, e+2)
}

// roundCap closes a stroke end with a feathered half disc around q. nrm is
// the normal of the adjoining segment, out the direction pointing away from
// the line, and ring the index of the end ring the cap attaches to.
func (m *meshBuf) roundCap(q, nrm, out Pt, ring uint32, h float32, col pcol, solid bool) {
	ri, ro := h-feather, h+feather
	steps := int(math.Ceil(math.Pi * float64(ro) / 2.5))
	steps = min(max(steps, 3), 32)

	var centre uint32
	if solid {
		centre = m.vert(q.X, q.Y, col)
	}
	// The arc runs from the +normal side (ring vertices 2, 3) through out to
	// the -normal side (ring vertices 1, 0).
	prevIn, prevOut := ring+2, ring+3
	for j := 1; j <= steps; j++ {
		var in, outer uint32
		if j == steps {
			in, outer = ring+1, ring
		} else {
			sin, cos := math.Sincos(math.Pi * float64(j) / float64(steps))
			dx := nrm.X*float32(cos) + out.X*float32(sin)
			dy := nrm.Y*float32(cos) + out.Y*float32(sin)
			in = m.vert(q.X+dx*ri, q.Y+dy*ri, col)
			outer = m.vert(q.X+dx*ro, q.Y+dy*ro, pcol{})
		}
		m.quad(prevIn, prevOut, outer, in)
		if solid {
			m.tri(centre, prevIn, in)
		}
		prevIn, prevOut = in, outer
	}
}

// dashed strokes pts as a sequence of dashes of length on separated by gaps
// of length off (pixels), starting phase pixels into the pattern. pts must
// already be deduplicated; they may alias m.pts.
func (m *meshBuf) dashed(pts []Pt, closed bool, w, on, off, phase float32, col pcol, cap Cap) {
	period := on + off
	if !(on > 0) || !(off > 0) || len(pts) < 2 {
		m.stroke(pts, closed, 0, w, col, cap)
		return
	}
	// The outline is copied because the caller's slice usually is m.pts,
	// which stays free for whoever strokes the dashes.
	m.src = append(m.src[:0], pts...)
	src := m.src
	n := len(src)
	segs := n - 1
	if closed {
		segs = n
	}

	pos := float32(math.Mod(float64(phase), float64(period)))
	if pos < 0 {
		pos += period
	}
	drawing := pos < on
	left := on - pos // distance until the pattern toggles
	if !drawing {
		left = period - pos
	}

	dash := m.dash[:0]
	if drawing {
		dash = append(dash, src[0])
	}
	flush := func() {
		if len(dash) >= 2 {
			m.dash = dash
			m.stroke(dash, false, 0, w, col, cap)
		}
		dash = dash[:0]
	}
	for s := 0; s < segs; s++ {
		a, b := src[s], src[(s+1)%n]
		dx, dy := b.X-a.X, b.Y-a.Y
		l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if l == 0 {
			continue
		}
		var t float32 // distance travelled along this segment
		for l-t > left {
			t += left
			q := Pt{a.X + dx*t/l, a.Y + dy*t/l}
			if drawing {
				dash = append(dash, q)
				flush()
				left = off
			} else {
				dash = append(dash[:0], q)
				left = on
			}
			drawing = !drawing
		}
		left -= l - t
		if drawing {
			dash = append(dash, b)
		}
	}
	flush()
	m.dash = dash
}

// circleSegments returns how many straight segments approximate a circle of
// radius r pixels to within about a tenth of a pixel. The result is a
// multiple of 4 so that the extreme points lie exactly on the axes.
func circleSegments(r float32) int {
	if !(r > 0.5) {
		return 8
	}
	const tol = 0.1
	n := int(math.Ceil(math.Pi / math.Acos(float64(1-tol/r))))
	n = (n + 3) &^ 3
	return min(max(n, 8), maxCircleSegments)
}

const maxCircleSegments = 256

// unitCircles caches the vertices of regular n-gons on the unit circle,
// indexed by n/4.
var unitCircles [maxCircleSegments/4 + 1][]Pt

// unitCircle returns the n vertices of a regular n-gon inscribed in the unit
// circle, starting on the +x axis and running clockwise on screen. n must be
// a multiple of 4 no greater than maxCircleSegments.
func unitCircle(n int) []Pt {
	if t := unitCircles[n/4]; t != nil {
		return t
	}
	t := make([]Pt, n)
	for i := range t {
		sin, cos := math.Sincos(2 * math.Pi * float64(i) / float64(n))
		t[i] = Pt{float32(cos), float32(sin)}
	}
	// Make the axis points exact.
	t[0], t[n/4], t[n/2], t[3*n/4] = Pt{1, 0}, Pt{0, 1}, Pt{-1, 0}, Pt{0, -1}
	unitCircles[n/4] = t
	return t
}

// disc appends a filled circle of radius r pixels with a feathered rim.
func (m *meshBuf) disc(cx, cy, r float32, p *paint) {
	if !(r > 0) {
		return
	}
	n := circleSegments(r + feather)
	unit := unitCircle(n)
	ri, ro := r-feather, r+feather
	gain := float32(1)
	if ri < 0 {
		// Smaller than the feather: keep the right amount of light.
		ri = 0
		gain = min(1, 3*r*r/(ro*ro))
	}
	base := uint32(len(m.v))
	centre := p.at(cy).scale(gain)
	if ri == 0 {
		m.vert(cx, cy, centre)
		for _, u := range unit {
			m.vert(cx+u.X*ro, cy+u.Y*ro, pcol{})
		}
		for i := uint32(0); i < uint32(n); i++ {
			m.tri(base, base+1+i, base+1+(i+1)%uint32(n))
		}
		return
	}
	for _, u := range unit {
		y := cy + u.Y*ri
		m.vert(cx+u.X*ri, y, p.at(y))
		m.vert(cx+u.X*ro, cy+u.Y*ro, pcol{})
	}
	for i := uint32(1); i+1 < uint32(n); i++ {
		m.tri(base, base+2*i, base+2*(i+1))
	}
	for i := uint32(0); i < uint32(n); i++ {
		j := (i + 1) % uint32(n)
		m.quad(base+2*i, base+2*i+1, base+2*j+1, base+2*j)
	}
}

// ring appends a circle outline of radius r and width w pixels, centred on
// the circle.
func (m *meshBuf) ring(cx, cy, r, w float32, col pcol) {
	if !(r > 0) || !(w > 0) {
		return
	}
	if w < 1 {
		col = col.scale(w)
		w = 1
	}
	h := w / 2
	rad := [4]float32{r - h - feather, r - h + feather, r + h - feather, r + h + feather}
	cols := [4]pcol{{}, col, col, {}}
	solid := rad[2]-rad[1] > 1e-4
	// A ring about as wide as its radius has a pinhole or no hole at all:
	// clamp the inner radii and light the centre by how closed the hole is.
	if rad[0] < 0 {
		cols[0] = col.scale(clampf(feather-(r-h), 0, 1))
		rad[0] = 0
	}
	if rad[1] < 0 {
		rad[1] = 0
	}
	n := circleSegments(rad[3])
	unit := unitCircle(n)
	base := uint32(len(m.v))
	for _, u := range unit {
		for k := 0; k < 4; k++ {
			m.vert(cx+u.X*rad[k], cy+u.Y*rad[k], cols[k])
		}
	}
	for i := uint32(0); i < uint32(n); i++ {
		a := base + 4*i
		b := base + 4*((i+1)%uint32(n))
		m.quad(a, a+1, b+1, b)
		if solid {
			m.quad(a+1, a+2, b+2, b+1)
		}
		m.quad(a+2, a+3, b+3, b+2)
	}
}

// arcPoints appends to dst the points of the circular arc from angle a0 to
// a1 (radians, clockwise on screen from the +x axis) and returns it.
func arcPoints(dst []Pt, cx, cy, r, a0, a1 float32) []Pt {
	sweep := float64(a1 - a0)
	full := circleSegments(r)
	steps := int(math.Ceil(math.Abs(sweep) / (2 * math.Pi) * float64(full)))
	steps = max(steps, 1)
	for i := 0; i <= steps; i++ {
		sin, cos := math.Sincos(float64(a0) + sweep*float64(i)/float64(steps))
		dst = append(dst, Pt{cx + r*float32(cos), cy + r*float32(sin)})
	}
	return dst
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
