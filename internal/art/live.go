package art

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
)

// The motion layer of every running structure is drawn afresh every frame,
// so its marks have to be the cheapest there are. Building an anti-aliased
// circle or line out of triangles each time costs several times as much as
// copying a small picture, so a live pen (bpen.live) makes its dots, bars
// and rings from a handful of white stamps, rendered once, which are
// stretched, turned and tinted into place.
//
// A stamp carries a transparent margin of one pixel and is always drawn
// with linear filtering at about its natural size or smaller, which gives
// its edge the same soft pixel an anti-aliased primitive has.

// stampLadder holds the sizes stamps are made in; a mark uses the smallest
// that is not smaller than itself, so it is never magnified until it is
// bigger than the largest.
var stampLadder = [...]float32{2, 3, 4, 5, 6, 8, 10, 12, 16, 20, 24, 32, 48, 64, 96, 128}

// maxCasing is the widest ink casing of a stamp, in stamp pixels.
const maxCasing = 8

// The stamps made so far. There are few enough of them, and nearly all are
// tiny, that they are kept until ReleaseSprites.
var (
	discStamps [len(stampLadder)][maxCasing + 1]*ebiten.Image
	barStamps  [len(stampLadder)][maxCasing + 1]*ebiten.Image
	ringStamps [len(stampLadder)]*ebiten.Image
)

// releaseStamps frees every stamp.
func releaseStamps() {
	for i := range discStamps {
		for j := range discStamps[i] {
			if img := discStamps[i][j]; img != nil {
				img.Deallocate()
				discStamps[i][j] = nil
			}
			if img := barStamps[i][j]; img != nil {
				img.Deallocate()
				barStamps[i][j] = nil
			}
		}
		if img := ringStamps[i]; img != nil {
			img.Deallocate()
			ringStamps[i] = nil
		}
	}
}

// stampIndex returns the rung of the ladder for a mark of px pixels.
func stampIndex(px float32) int {
	for i, n := range stampLadder {
		if n >= px {
			return i
		}
	}
	return len(stampLadder) - 1
}

// casingFor returns the width in stamp pixels of the ink casing of a stamp
// of size n that stands for a mark of px pixels with a casing of casePx
// pixels.
func casingFor(n, px, casePx float32) int {
	if !(casePx > 0) || !(px > 0) {
		return 0
	}
	return int(min(max(roundf(casePx*n/px), 1), maxCasing))
}

var stampWhite = gfx.RGB(0xFFFFFF)

// discStamp returns a white disc stampLadder[i] pixels across inside an ink
// casing c pixels wide (c may be 0), and the side of the picture in pixels.
func discStamp(i, c int) (*ebiten.Image, float32) {
	n, cs := stampLadder[i], float32(c)
	side := n + 2*cs + 2
	img := discStamps[i][c]
	if img == nil {
		img = gfx.Render(side, side, 1, func(cv *gfx.Canvas) {
			if c > 0 {
				cv.FillCircle(side/2, side/2, n/2+cs, inkLine)
			}
			cv.FillCircle(side/2, side/2, n/2, stampWhite)
		})
		discStamps[i][c] = img
	}
	return img, side
}

// barStampLen is the length in pixels of the white core of a bar stamp.
const barStampLen = 32

// barStamp returns a white bar barStampLen pixels long and stampLadder[i]
// wide inside an ink casing c pixels wide (c may be 0), and the size of the
// picture.
func barStamp(i, c int) (img *ebiten.Image, w, h float32) {
	n, cs := stampLadder[i], float32(c)
	w, h = barStampLen+2*cs+2, n+2*cs+2
	img = barStamps[i][c]
	if img == nil {
		img = gfx.Render(w, h, 1, func(cv *gfx.Canvas) {
			if c > 0 {
				cv.FillRect(gfx.R(1, 1, w-2, h-2), inkLine)
			}
			cv.FillRect(gfx.R(1+cs, 1+cs, barStampLen, n), stampWhite)
		})
		barStamps[i][c] = img
	}
	return img, w, h
}

// ringStampLine is the width in pixels of the line of a ring stamp.
const ringStampLine = 1.6

// ringStamp returns a white circle outline of radius stampLadder[i] pixels
// and the side of the picture.
func ringStamp(i int) (*ebiten.Image, float32) {
	n := stampLadder[i]
	side := 2*n + 4
	img := ringStamps[i]
	if img == nil {
		img = gfx.Render(side, side, 1, func(cv *gfx.Canvas) {
			cv.StrokeCircle(side/2, side/2, n, ringStampLine, stampWhite)
		})
		ringStamps[i] = img
	}
	return img, side
}

// stampOpts is the one set of options every stamp is drawn with; drawing
// is single threaded.
var stampOpts = gfx.ImageOpts{Smooth: true}

// scale returns the device pixels per canvas point of the pen.
func (b *bpen) scale() float32 { return 1 / (b.px * b.k) }

// stampOval draws the ellipse with half axes rx and ry points round the
// canvas point p, turned by rot, in colour c; casing is the width in points
// of an ink line round it, or 0.
func (b *bpen) stampOval(p gfx.Pt, rx, ry, rot, casing float32, c gfx.Color) {
	if c.A == 0 || !(rx > 0 && ry > 0) {
		return
	}
	s := b.scale()
	px := 2 * max(rx, ry) * s
	i := stampIndex(px)
	n := stampLadder[i]
	img, side := discStamp(i, casingFor(n, px, casing*s))
	w, h := 2*rx*side/n, 2*ry*side/n
	stampOpts.Tint, stampOpts.Rotation = c, rot
	b.cv.DrawImage(img, gfx.R(p.X-w/2, p.Y-h/2, w, h), &stampOpts)
}

// stampBar draws a straight bar from p to q, width points wide, in colour
// c; casing is the width in points of an ink line round it, or 0.
func (b *bpen) stampBar(p, q gfx.Pt, width, casing float32, c gfx.Color) {
	dx, dy := q.X-p.X, q.Y-p.Y
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if c.A == 0 || !(l > 0) || !(width > 0) {
		return
	}
	s := b.scale()
	i := stampIndex(width * s)
	n := stampLadder[i]
	img, sw, sh := barStamp(i, casingFor(n, width*s, casing*s))
	w, h := l*sw/barStampLen, width*sh/n
	stampOpts.Tint, stampOpts.Rotation = c, float32(math.Atan2(float64(dy), float64(dx)))
	b.cv.DrawImage(img, gfx.R((p.X+q.X-w)/2, (p.Y+q.Y-h)/2, w, h), &stampOpts)
}

// stampRing draws the outline of the ellipse with half axes rx and ry
// points round p in colour c.
func (b *bpen) stampRing(p gfx.Pt, rx, ry float32, c gfx.Color) {
	if c.A == 0 || !(rx > 0 && ry > 0) {
		return
	}
	i := stampIndex(max(rx, ry) * b.scale())
	n := stampLadder[i]
	img, side := ringStamp(i)
	w, h := rx*side/n, ry*side/n
	stampOpts.Tint, stampOpts.Rotation = c, 0
	b.cv.DrawImage(img, gfx.R(p.X-w/2, p.Y-h/2, w, h), &stampOpts)
}

// ovalAt fills the ellipse that a circle in space makes on screen: the
// points centre + u·e1 + v·e2 with u² + v² ≤ 1, where e1 and e2 are two
// perpendicular radii of the circle given as x, y, h triples. With casing
// it gets an ink line round it.
func (b *bpen) ovalAt(c gfx.Color, casing bool, cx, cy, ch float32, e1, e2 [3]float32) {
	// The two radii on screen, in points.
	ax, ay := e1[0]*b.k, (e1[1]*Tilt-e1[2])*b.k
	bx, by := e2[0]*b.k, (e2[1]*Tilt-e2[2])*b.k
	p := b.at(cx, cy, ch)
	if !b.live {
		var buf [24]gfx.Pt
		pts := buf[:0]
		for i := 0; i < len(buf); i++ {
			t := tau * float32(i) / float32(len(buf))
			u, v := cosf(t), sinf(t)
			pts = append(pts, gfx.Pt{X: p.X + u*ax + v*bx, Y: p.Y + u*ay + v*by})
		}
		b.cv.FillPoly(pts, b.col(c))
		if casing {
			b.cv.StrokePoly(pts, b.w(b.ow(), 1), b.col(inkLine))
		}
		return
	}
	// The half axes and the turn of the ellipse are the singular values
	// and the first rotation of the matrix with columns (ax, ay), (bx, by).
	e, f := (ax+by)/2, (ax-by)/2
	g, h := (ay+bx)/2, (ay-bx)/2
	q, r := float32(math.Hypot(float64(e), float64(h))), float32(math.Hypot(float64(f), float64(g)))
	rot := float32(math.Atan2(float64(g), float64(f))+math.Atan2(float64(h), float64(e))) / 2
	cs := float32(0)
	if casing {
		cs = b.w(b.ow(), 1)
	}
	b.stampOval(p, q+r, max(absf32(q-r), 0.3*b.px*b.k), rot, cs, c)
}

// puffAt draws a round puff of smoke or a spark at a point in space: a
// plain disc of radius r units.
func (b *bpen) puffAt(x, y, h, r float32, c gfx.Color) {
	p := b.soft(x, y, h)
	if b.live {
		b.stampOval(p, r*b.k, r*b.k, 0, 0, c)
		return
	}
	b.cv.FillCircle(p.X, p.Y, r*b.k, b.col(c))
}

// knobAt draws a round knob at a point in space: a disc of colour c and
// radius r units inside an ink line.
func (b *bpen) knobAt(x, y, h, r float32, c gfx.Color) {
	p := b.at(x, y, h)
	o := b.ow()
	if b.live {
		b.stampOval(p, r*b.k, r*b.k, 0, o*b.k, c)
		return
	}
	b.cv.FillCircle(p.X, p.Y, (r+o)*b.k, b.col(inkLine))
	b.cv.FillCircle(p.X, p.Y, r*b.k, b.col(c))
}

// rod draws a thin line between two points in space, without a casing.
func (b *bpen) rod(c gfx.Color, width, minPx, x0, y0, h0, x1, y1, h1 float32) {
	p, q := b.at(x0, y0, h0), b.at(x1, y1, h1)
	if b.live {
		b.stampBar(p, q, b.w(width, minPx), 0, c)
		return
	}
	b.cv.Line(p.X, p.Y, q.X, q.Y, b.w(width, minPx), b.col(c))
}
