package art

import (
	"math"

	"drifthold/internal/gfx"
)

// pen draws on a canvas in a private unit frame: its origin sits at (ox, oy)
// canvas points, one unit is k points long and the frame may be turned. All
// the pictures of this package are authored in such frames (a building in
// units of its hex radius, a ship in units of its length), so one drawing
// routine serves every size, place and heading.
type pen struct {
	cv       *gfx.Canvas
	ox, oy   float32 // origin in canvas points
	k        float32 // canvas points per unit
	cos, sin float32 // rotation of the frame (clockwise on screen)
	ang      float32 // the same rotation as an angle
	turned   bool
	px       float32 // length of one device pixel, in units
	// tone, when set, filters every colour on its way to the canvas. It is
	// how one drawing yields its derelict or switched-off variant.
	tone func(gfx.Color) gfx.Color
}

// newPen returns a pen whose unit frame is centred on (ox, oy) with units of
// k points.
func newPen(cv *gfx.Canvas, ox, oy, k float32) pen {
	s := cv.Scale
	if !(s > 0) {
		s = 1
	}
	px := float32(1)
	if k > 0 {
		px = 1 / (k * s)
	}
	return pen{cv: cv, ox: ox, oy: oy, k: k, cos: 1, px: px}
}

// rotated returns the pen with its frame turned by a radians.
func (p pen) rotated(a float32) pen {
	if a == 0 {
		return p
	}
	sin, cos := math.Sincos(float64(a))
	p.cos, p.sin, p.ang, p.turned = float32(cos), float32(sin), a, true
	return p
}

// ptsBuf is the scratch outline shared by every pen; drawing is single
// threaded.
var ptsBuf []gfx.Pt

// pt maps a unit-frame point to canvas points.
func (p *pen) pt(x, y float32) gfx.Pt {
	if p.turned {
		x, y = x*p.cos-y*p.sin, x*p.sin+y*p.cos
	}
	return gfx.Pt{X: p.ox + x*p.k, Y: p.oy + y*p.k}
}

// pts maps a flat list of x, y pairs into the scratch buffer.
func (p *pen) pts(xy []float32) []gfx.Pt {
	ptsBuf = ptsBuf[:0]
	for i := 0; i+1 < len(xy); i += 2 {
		ptsBuf = append(ptsBuf, p.pt(xy[i], xy[i+1]))
	}
	return ptsBuf
}

// col applies the tone filter.
func (p *pen) col(c gfx.Color) gfx.Color {
	if p.tone != nil && c.A != 0 {
		return p.tone(c)
	}
	return c
}

// w converts a stroke width in units to points, never thinner than minPx
// device pixels.
func (p *pen) w(units, minPx float32) float32 {
	return max(units, minPx*p.px) * p.k
}

// poly fills the polygon given as x, y pairs.
func (p *pen) poly(c gfx.Color, xy ...float32) {
	p.cv.FillPoly(p.pts(xy), p.col(c))
}

// polyV fills a polygon with a vertical gradient (in screen space).
func (p *pen) polyV(top, bottom gfx.Color, xy ...float32) {
	p.cv.FillPolyV(p.pts(xy), p.col(top), p.col(bottom))
}

// outline strokes the closed polygon given as x, y pairs; width in units
// with a floor of minPx pixels.
func (p *pen) outline(c gfx.Color, width, minPx float32, xy ...float32) {
	p.cv.StrokePoly(p.pts(xy), p.w(width, minPx), p.col(c))
}

// polyline strokes the open line through the x, y pairs.
func (p *pen) polyline(c gfx.Color, width, minPx float32, xy ...float32) {
	p.cv.Polyline(p.pts(xy), p.w(width, minPx), p.col(c))
}

// line strokes one segment with butt caps.
func (p *pen) line(x0, y0, x1, y1, width, minPx float32, c gfx.Color) {
	a, b := p.pt(x0, y0), p.pt(x1, y1)
	p.cv.Line(a.X, a.Y, b.X, b.Y, p.w(width, minPx), p.col(c))
}

// rect fills an axis-aligned rectangle given by its top-left corner and
// size. In an unturned frame its edges are snapped to pixels (crisp); in a
// turned frame it becomes a polygon.
func (p *pen) rect(x, y, w, h float32, c gfx.Color) {
	if p.turned {
		p.poly(c, x, y, x+w, y, x+w, y+h, x, y+h)
		return
	}
	p.cv.FillRect(gfx.R(p.ox+x*p.k, p.oy+y*p.k, w*p.k, h*p.k), p.col(c))
}

// quad fills an axis-aligned rectangle without pixel snapping, for parts
// that must meet anti-aliased shapes exactly.
func (p *pen) quad(x, y, w, h float32, c gfx.Color) {
	p.poly(c, x, y, x+w, y, x+w, y+h, x, y+h)
}

// circle fills a disc.
func (p *pen) circle(x, y, r float32, c gfx.Color) {
	q := p.pt(x, y)
	p.cv.FillCircle(q.X, q.Y, r*p.k, p.col(c))
}

// ring strokes a circle.
func (p *pen) ring(x, y, r, width, minPx float32, c gfx.Color) {
	q := p.pt(x, y)
	p.cv.StrokeCircle(q.X, q.Y, r*p.k, p.w(width, minPx), p.col(c))
}

// arc strokes part of a circle between the frame angles a0 and a1.
func (p *pen) arc(x, y, r, a0, a1, width, minPx float32, c gfx.Color) {
	q := p.pt(x, y)
	p.cv.Arc(q.X, q.Y, r*p.k, a0+p.ang, a1+p.ang, p.w(width, minPx), p.col(c))
}

// ngonLine strokes a regular polygon.
func (p *pen) ngonLine(x, y, r float32, n int, a0, width, minPx float32, c gfx.Color) {
	p.cv.StrokePoly(p.ngonPts(x, y, r, n, a0), p.w(width, minPx), p.col(c))
}

func (p *pen) ngonPts(x, y, r float32, n int, a0 float32) []gfx.Pt {
	ptsBuf = ptsBuf[:0]
	for i := 0; i < n; i++ {
		sin, cos := math.Sincos(float64(a0) + 2*math.Pi*float64(i)/float64(n))
		ptsBuf = append(ptsBuf, p.pt(x+r*float32(cos), y+r*float32(sin)))
	}
	return ptsBuf
}

// glow adds a soft light. It bypasses the tone filter: callers that draw
// unlit variants leave their glows out.
func (p *pen) glow(x, y, r float32, c gfx.Color) {
	q := p.pt(x, y)
	p.cv.Glow(q.X, q.Y, r*p.k, c)
}

// glowCore adds a tight light with a faint skirt.
func (p *pen) glowCore(x, y, r float32, c gfx.Color) {
	q := p.pt(x, y)
	p.cv.GlowCore(q.X, q.Y, r*p.k, c)
}

// blob draws a soft-edged ellipse (normal blending), for shadows and smoke.
// In a turned frame the ellipse stays axis-aligned; callers use round blobs
// there.
func (p *pen) blob(x, y, rx, ry float32, c gfx.Color) {
	q := p.pt(x, y)
	p.cv.Blob(q.X, q.Y, rx*p.k, ry*p.k, p.col(c))
}

// path draws a gfx.Path authored in unit coordinates.
func (p *pen) path(pa *gfx.Path, st gfx.PathStyle) {
	st.X, st.Y, st.Scale, st.Rotation = p.ox, p.oy, p.k, p.ang
	st.Fill, st.FillTo, st.Stroke = p.col(st.Fill), p.col(st.FillTo), p.col(st.Stroke)
	p.cv.DrawPath(pa, st)
}

// ---------------------------------------------------------------------------
// Canvas helpers
// ---------------------------------------------------------------------------

// canvasScale returns the effective pixels-per-point factor of cv.
func canvasScale(cv *gfx.Canvas) float32 {
	if cv.Scale > 0 {
		return cv.Scale
	}
	return 1
}

// canvasOrigin returns the pixel position of the canvas's point (0, 0). It
// is zero for ordinary canvases and non-zero for those made with
// Canvas.Local.
func canvasOrigin(cv *gfx.Canvas) (ox, oy float32) {
	if cv.Dst == nil {
		return 0, 0
	}
	b := cv.Dst.Bounds()
	r := cv.Bounds()
	s := canvasScale(cv)
	return float32(b.Min.X) - r.X*s, float32(b.Min.Y) - r.Y*s
}

// snapCentre returns (x, y) moved so that it falls on a pixel boundary of
// the destination: the centre a sprite with even pixel dimensions gets from
// Canvas.DrawSprite. Live details drawn around the result line up with the
// sprite to the pixel.
func snapCentre(cv *gfx.Canvas, x, y float32) (float32, float32) {
	s := canvasScale(cv)
	ox, oy := canvasOrigin(cv)
	return (roundf(x*s+ox) - ox) / s, (roundf(y*s+oy) - oy) / s
}

func roundf(v float32) float32 { return float32(math.Floor(float64(v) + 0.5)) }

// ---------------------------------------------------------------------------
// Colour helpers
// ---------------------------------------------------------------------------

// mat3 is a material: three flat tones for lit, plain and shaded faces.
type mat3 struct{ hi, mid, lo gfx.Color }

func mat(hi, mid, lo uint32) mat3 { return mat3{gfx.RGB(hi), gfx.RGB(mid), gfx.RGB(lo)} }

// matOf derives a material from one colour.
func matOf(c gfx.Color) mat3 {
	return mat3{gfx.Lighten(c, 0.32), c, gfx.Darken(c, 0.40)}
}

// shade lightens (v > 0) or darkens (v < 0) a colour.
func shade(c gfx.Color, v float32) gfx.Color {
	if v >= 0 {
		return gfx.Lighten(c, v)
	}
	return gfx.Darken(c, -v)
}

// desat moves a colour towards its own grey by amt (0..1).
func desat(c gfx.Color, amt float32) gfx.Color {
	l := gfx.Luma(c)
	g := gfx.Color{R: uint8(l * 255), G: uint8(l * 255), B: uint8(l * 255), A: c.A}
	return gfx.Lerp(c, g, amt)
}

// alpha returns c with opacity a.
func alpha(c gfx.Color, a float32) gfx.Color { return gfx.WithAlpha(c, a) }

// packColor packs a colour into 32 bits, for cache keys.
func packColor(c gfx.Color) uint64 {
	return uint64(c.R)<<24 | uint64(c.G)<<16 | uint64(c.B)<<8 | uint64(c.A)
}

// The inks shared by all the art.
var (
	inkLine   = gfx.RGB(0x0A0C12) // outlines
	inkShadow = gfx.Color{R: 0x04, G: 0x05, B: 0x09, A: 0x78}
)

// ---------------------------------------------------------------------------
// Deterministic randomness
// ---------------------------------------------------------------------------

// mix64 is the SplitMix64 finaliser: a cheap, well-mixed 64-bit hash.
func mix64(x uint64) uint64 {
	x += 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	return x ^ (x >> 31)
}

// hash2 hashes a seed with two integer coordinates.
func hash2(seed uint64, x, y int) uint64 {
	return mix64(seed ^ mix64(uint64(int64(x))*0x9E3779B97F4A7C15^uint64(int64(y))*0xC2B2AE3D27D4EB4F))
}

// unit maps a hash to [0, 1).
func unit(h uint64) float32 { return float32(h>>40) / float32(1<<24) }

// rnd is a tiny deterministic generator for procedural detail.
type rnd struct{ s uint64 }

func newRnd(seed uint64) rnd { return rnd{mix64(seed) | 1} }

func (r *rnd) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	return mix64(r.s)
}

// f returns a float in [0, 1).
func (r *rnd) f() float32 { return unit(r.next()) }

// in returns a float in [lo, hi).
func (r *rnd) in(lo, hi float32) float32 { return lo + (hi-lo)*r.f() }

// n returns an int in [0, n).
func (r *rnd) n(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

// valueNoise is smooth 2-D value noise in 0..1 with unit lattice spacing.
func valueNoise(seed uint64, x, y float64) float32 {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := float32(x-x0), float32(y-y0)
	fx = fx * fx * (3 - 2*fx)
	fy = fy * fy * (3 - 2*fy)
	ix, iy := int(x0), int(y0)
	a := unit(hash2(seed, ix, iy))
	b := unit(hash2(seed, ix+1, iy))
	c := unit(hash2(seed, ix, iy+1))
	d := unit(hash2(seed, ix+1, iy+1))
	return (a+(b-a)*fx)*(1-fy) + (c+(d-c)*fx)*fy
}

// fbm sums two octaves of value noise, result in about 0..1.
func fbm(seed uint64, x, y float64) float32 {
	return 0.65*valueNoise(seed, x, y) + 0.35*valueNoise(seed^0xA5A5A5A5, 2.1*x+7.3, 2.1*y-3.1)
}

// Small float32 maths.

func sinf(a float32) float32 { return float32(math.Sin(float64(a))) }
func cosf(a float32) float32 { return float32(math.Cos(float64(a))) }

func clamp01(v float32) float32 { return min(max(v, 0), 1) }

func lerpf(a, b, t float32) float32 { return a + (b-a)*t }

// smooth is the smoothstep of t clamped to 0..1.
func smooth(t float32) float32 {
	t = clamp01(t)
	return t * t * (3 - 2*t)
}

// pulse returns a value swinging between lo and hi with the given period in
// seconds.
func pulse(t, period, lo, hi float32) float32 {
	return lo + (hi-lo)*(0.5+0.5*sinf(t*2*math.Pi/period))
}

// finite returns v, or 0 when v is NaN or infinite: animation clocks and
// angles from the caller are never allowed to poison the geometry.
func finite(v float32) float32 {
	if v != v || v > math.MaxFloat32 || v < -math.MaxFloat32 {
		return 0
	}
	return v
}

const (
	pi  = float32(math.Pi)
	tau = float32(2 * math.Pi)
)
