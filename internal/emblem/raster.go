package emblem

import (
	"image"
	"image/draw"
	"math"

	"golang.org/x/image/vector"
)

// pt is a point in the emblem's unit design space: (0,0) is the top-left
// corner of the icon and (1,1) the bottom-right, with y growing downwards.
type pt struct{ x, y float64 }

func (p pt) add(q pt) pt        { return pt{p.x + q.x, p.y + q.y} }
func (p pt) sub(q pt) pt        { return pt{p.x - q.x, p.y - q.y} }
func (p pt) mul(k float64) pt   { return pt{p.x * k, p.y * k} }
func (p pt) dot(q pt) float64   { return p.x*q.x + p.y*q.y }
func (p pt) cross(q pt) float64 { return p.x*q.y - p.y*q.x }
func (p pt) length() float64    { return math.Hypot(p.x, p.y) }

// unit returns p scaled to length one (or p itself when it is the zero vector).
func (p pt) unit() pt {
	l := p.length()
	if l == 0 {
		return p
	}
	return pt{p.x / l, p.y / l}
}

// rgb is an opaque, non-premultiplied colour with components in 0..1.
type rgb struct{ r, g, b float64 }

// hex converts a 0xRRGGBB literal to an rgb colour.
func hex(v uint32) rgb {
	return rgb{
		r: float64(v>>16&0xFF) / 255,
		g: float64(v>>8&0xFF) / 255,
		b: float64(v&0xFF) / 255,
	}
}

// mix blends from a (t=0) to b (t=1).
func mix(a, b rgb, t float64) rgb {
	return rgb{
		r: a.r + (b.r-a.r)*t,
		g: a.g + (b.g-a.g)*t,
		b: a.b + (b.b-a.b)*t,
	}
}

// paint yields the colour and opacity of a shape at a point of the unit
// design space. The colour is not premultiplied.
type paint func(x, y float64) (c rgb, alpha float64)

// solid paints one colour at a constant opacity.
func solid(c rgb, alpha float64) paint {
	return func(float64, float64) (rgb, float64) { return c, alpha }
}

// gradient paints a linear gradient that is c0 at p0 and c1 at p1, clamped
// beyond both ends.
func gradient(p0 pt, c0 rgb, p1 pt, c1 rgb) paint {
	d := p1.sub(p0)
	den := d.dot(d)
	return func(x, y float64) (rgb, float64) {
		t := 0.0
		if den > 0 {
			t = (pt{x, y}).sub(p0).dot(d) / den
		}
		return mix(c0, c1, clamp01(t)), 1
	}
}

// glow paints colour c with an opacity that falls smoothly from peak at the
// centre to zero at the given radius.
func glow(centre pt, radius float64, c rgb, peak float64) paint {
	return func(x, y float64) (rgb, float64) {
		t := clamp01(1 - (pt{x, y}).sub(centre).length()/radius)
		return c, peak * t * t * (3 - 2*t)
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// canvas is a square premultiplied-alpha float image onto which anti-aliased
// polygons are composited with the source-over operator.
type canvas struct {
	n    int       // side length in pixels
	pix  []float64 // premultiplied r, g, b, a per pixel, each in 0..1
	ras  *vector.Rasterizer
	mask *image.Alpha
}

// newCanvas returns a fully transparent canvas of n by n pixels.
func newCanvas(n int) *canvas {
	return &canvas{
		n:    n,
		pix:  make([]float64, 4*n*n),
		ras:  vector.NewRasterizer(n, n),
		mask: image.NewAlpha(image.Rect(0, 0, n, n)),
	}
}

// fill composites the given closed polygons (non-zero winding, exact area
// coverage anti-aliasing) onto the canvas using paint p.
func (c *canvas) fill(p paint, polys ...[]pt) {
	n := float64(c.n)
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	c.ras.Reset(c.n, c.n)
	for _, poly := range polys {
		if len(poly) < 3 {
			continue
		}
		for i, v := range poly {
			x, y := v.x*n, v.y*n
			minX, maxX = math.Min(minX, x), math.Max(maxX, x)
			minY, maxY = math.Min(minY, y), math.Max(maxY, y)
			if i == 0 {
				c.ras.MoveTo(float32(x), float32(y))
			} else {
				c.ras.LineTo(float32(x), float32(y))
			}
		}
		c.ras.ClosePath()
	}
	if minX > maxX {
		return
	}
	c.ras.DrawOp = draw.Src
	c.ras.Draw(c.mask, c.mask.Bounds(), image.Opaque, image.Point{})

	x0, y0 := clampInt(int(math.Floor(minX))-1, 0, c.n), clampInt(int(math.Floor(minY))-1, 0, c.n)
	x1, y1 := clampInt(int(math.Ceil(maxX))+1, 0, c.n), clampInt(int(math.Ceil(maxY))+1, 0, c.n)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			cov := c.mask.Pix[y*c.mask.Stride+x]
			if cov == 0 {
				continue
			}
			col, a := p((float64(x)+0.5)/n, (float64(y)+0.5)/n)
			a *= float64(cov) / 255
			if a <= 0 {
				continue
			}
			o := 4 * (y*c.n + x)
			c.pix[o+0] = col.r*a + c.pix[o+0]*(1-a)
			c.pix[o+1] = col.g*a + c.pix[o+1]*(1-a)
			c.pix[o+2] = col.b*a + c.pix[o+2]*(1-a)
			c.pix[o+3] = a + c.pix[o+3]*(1-a)
		}
	}
}

// image converts the canvas to an 8-bit premultiplied RGBA image.
func (c *canvas) image() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, c.n, c.n))
	for i, v := range c.pix {
		img.Pix[i] = uint8(math.Floor(clamp01(v)*255 + 0.5))
	}
	// Rounding each channel separately can leave a colour channel one step
	// above alpha; keep the result a valid premultiplied image.
	for i := 0; i < len(img.Pix); i += 4 {
		a := img.Pix[i+3]
		for j := 0; j < 3; j++ {
			if img.Pix[i+j] > a {
				img.Pix[i+j] = a
			}
		}
	}
	return img
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// inset moves every edge of a convex polygon inwards by distance d and
// returns the polygon formed by the intersections of neighbouring edges.
func inset(poly []pt, d float64) []pt {
	n := len(poly)
	// Orientation: positive area means the vertices run clockwise on screen
	// (y down), so the inward normal of edge a->b is the left-hand normal.
	area := 0.0
	for i := range poly {
		area += poly[i].cross(poly[(i+1)%n])
	}
	sign := 1.0
	if area < 0 {
		sign = -1
	}
	type line struct{ p, d pt }
	lines := make([]line, n)
	for i := range poly {
		a, b := poly[i], poly[(i+1)%n]
		dir := b.sub(a).unit()
		normal := pt{-dir.y * sign, dir.x * sign}
		lines[i] = line{a.add(normal.mul(d)), dir}
	}
	out := make([]pt, n)
	for i := range poly {
		prev, cur := lines[(i+n-1)%n], lines[i]
		den := prev.d.cross(cur.d)
		if math.Abs(den) < 1e-12 {
			out[i] = cur.p
			continue
		}
		t := cur.p.sub(prev.p).cross(cur.d) / den
		out[i] = prev.p.add(prev.d.mul(t))
	}
	return out
}

// chamfer cuts every corner of a polygon with a short flat, placing the two
// new vertices at distance d from the original corner along its edges.
func chamfer(poly []pt, d float64) []pt {
	if d <= 0 {
		return poly
	}
	n := len(poly)
	out := make([]pt, 0, 2*n)
	for i, v := range poly {
		prev, next := poly[(i+n-1)%n], poly[(i+1)%n]
		out = append(out, v.add(prev.sub(v).unit().mul(d)), v.add(next.sub(v).unit().mul(d)))
	}
	return out
}

// disc approximates a circle with a regular polygon fine enough to be
// indistinguishable from a true circle at any supported icon size.
func disc(centre pt, radius float64) []pt {
	const segments = 96
	out := make([]pt, segments)
	for i := range out {
		a := 2 * math.Pi * float64(i) / segments
		out[i] = pt{centre.x + radius*math.Cos(a), centre.y + radius*math.Sin(a)}
	}
	return out
}

// ribbon builds the outline polygon of a stroke along the sampled curve pts
// whose half-width at sample i is half[i].
func ribbon(pts []pt, half []float64) []pt {
	n := len(pts)
	out := make([]pt, 2*n)
	for i := range pts {
		a, b := pts[max(i-1, 0)], pts[min(i+1, n-1)]
		dir := b.sub(a).unit()
		normal := pt{-dir.y, dir.x}.mul(half[i])
		out[i] = pts[i].add(normal)
		out[2*n-1-i] = pts[i].sub(normal)
	}
	return out
}
