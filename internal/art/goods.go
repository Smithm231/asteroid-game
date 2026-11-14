package art

import (
	"math"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// Goods are drawn in a unit box centred on the origin (x and y from -0.5 to
// 0.5). Each has its own silhouette — lump, shards, cube, gem, bar, drop,
// cog, sprout — so that they can be told apart without colour, and each is
// shaded in three tones of its theme colour with the light at the upper
// left. They are small and static, so every size is rendered once into a
// sprite.

func drawGood(cv *gfx.Canvas, g model.Good, r gfx.Rect) {
	if cv == nil || cv.Dst == nil || !g.Valid() || r.Empty() {
		return
	}
	s := canvasScale(cv)
	box := int(math.Round(float64(min(r.W, r.H) * s)))
	if box < 4 {
		box = 4
	}
	// Room around the box for the Lumen glow.
	pad := (box + 3) / 4
	side := box + 2*pad
	img := sprite(domGood|uint64(g), side, side, s, func(sc *gfx.Canvas) {
		c := float32(side) / 2 / s
		p := newPen(sc, c, c, float32(box)/s)
		base := gfx.Default().GoodColor(int(g))
		goodArts[g](&p, mat3{gfx.Lighten(base, 0.42), base, gfx.Darken(base, 0.38)}, float32(box))
	})
	cv.DrawSprite(img, r.MidX(), r.MidY(), nil)
}

var goodArts = [model.NumGoods]func(p *pen, m mat3, box float32){
	model.Ore:    goodOre,
	model.Ice:    goodIce,
	model.Silica: goodSilica,
	model.Lumen:  goodLumen,
	model.Alloy:  goodAlloy,
	model.Fuel:   goodFuel,
	model.Parts:  goodParts,
	model.Food:   goodFood,
}

// Ore: a rough nugget.
func goodOre(p *pen, m mat3, box float32) {
	p.poly(m.lo, -0.42, 0.08, -0.26, -0.3, 0.06, -0.43, 0.38, -0.2, 0.45, 0.16, 0.18, 0.42, -0.22, 0.4)
	p.poly(m.mid, -0.42, 0.08, -0.26, -0.3, 0.06, -0.43, 0.38, -0.2, 0.12, 0.02, -0.16, 0.2)
	p.poly(m.hi, -0.26, -0.3, 0.06, -0.43, 0.16, -0.2, -0.14, -0.06)
	if box >= 14 {
		p.poly(gfx.Lighten(m.hi, 0.3), 0.14, 0.12, 0.26, 0.08, 0.24, 0.22)
		p.poly(gfx.Darken(m.lo, 0.25), -0.2, 0.26, -0.06, 0.22, -0.1, 0.34)
	}
}

// Ice: a cluster of three shards.
func goodIce(p *pen, m mat3, box float32) {
	p.poly(m.mid, -0.44, 0.42, -0.36, -0.12, -0.22, -0.02, -0.16, 0.42)
	p.poly(m.lo, 0.16, 0.42, 0.2, -0.06, 0.32, -0.24, 0.44, 0.0, 0.4, 0.42)
	p.poly(m.mid, 0.16, 0.42, 0.2, -0.06, 0.32, -0.24, 0.3, 0.42)
	p.poly(m.lo, 0, -0.47, 0.17, -0.12, 0.13, 0.42, -0.15, 0.42, -0.17, -0.1)
	p.poly(m.hi, 0, -0.47, 0.02, 0.42, -0.15, 0.42, -0.17, -0.1)
	if box >= 14 {
		p.poly(gfx.White, -0.02, -0.34, -0.03, 0.0, -0.11, -0.08)
	}
}

// Silica: a pale faceted block.
func goodSilica(p *pen, m mat3, box float32) {
	const w, h, d = 0.4, 0.2, 0.26
	// Top, left and right faces of a cube seen from above its corner.
	p.poly(m.hi, 0, -h-d, w, -d, 0, h-d, -w, -d)
	p.poly(m.mid, -w, -d, 0, h-d, 0, h+d, -w, d)
	p.poly(m.lo, w, -d, 0, h-d, 0, h+d, w, d)
	if box >= 16 {
		p.line(-w+0.07, -d+0.12, -0.07, h-d+0.08, 0.03, 0.8, alpha(m.hi, 0.6))
	}
}

// Lumen: a glowing gem.
func goodLumen(p *pen, m mat3, box float32) {
	p.glow(0, 0, 0.72, alpha(m.mid, 0.55))
	p.poly(m.lo, 0, -0.47, 0.25, -0.17, 0.25, 0.17, 0, 0.47, -0.25, 0.17, -0.25, -0.17)
	p.poly(m.mid, 0, -0.47, 0, 0.47, -0.25, 0.17, -0.25, -0.17)
	p.poly(m.hi, 0, -0.47, 0.06, -0.14, 0, 0.1, -0.25, -0.17)
	p.poly(gfx.Lighten(m.hi, 0.5), 0, -0.47, 0.06, -0.14, -0.1, -0.2)
	if box >= 14 {
		// A glint.
		p.line(0.22, -0.36, 0.4, -0.36, 0.035, 0.9, gfx.White)
		p.line(0.31, -0.45, 0.31, -0.27, 0.035, 0.9, gfx.White)
	}
}

// Alloy: an ingot.
func goodAlloy(p *pen, m mat3, box float32) {
	p.poly(m.lo, -0.46, 0.3, -0.36, -0.02, 0.36, -0.02, 0.46, 0.3)
	p.poly(m.mid, -0.46, 0.3, -0.36, -0.02, 0.2, -0.02, 0.28, 0.3)
	p.poly(m.hi, -0.36, -0.02, -0.26, -0.26, 0.26, -0.26, 0.36, -0.02)
	if box >= 16 {
		p.line(-0.2, -0.14, 0.2, -0.14, 0.035, 0.8, alpha(gfx.White, 0.7))
	}
}

// Fuel: a drop.
func goodFuel(p *pen, m mat3, box float32) {
	const cy, r = 0.15, 0.31
	drop := func(sc, dx, dy float32) []float32 {
		// The tip, then the round belly from one tangent point to the
		// other.
		xy := []float32{dx, dy + (-0.48-cy)*sc + cy}
		phi := float32(math.Acos(float64(r / (cy + 0.48))))
		a0, a1 := -pi/2+phi, 3*pi/2-phi
		for i := 0; i <= 14; i++ {
			a := a0 + (a1-a0)*float32(i)/14
			xy = append(xy, dx+sc*r*cosf(a), dy+cy+sc*r*sinf(a))
		}
		return xy
	}
	p.poly(m.lo, drop(1, 0, 0)...)
	p.poly(m.mid, drop(0.8, -0.035, -0.03)...)
	p.circle(-0.1, 0.1, 0.085, m.hi)
}

// Parts: a cog.
func goodParts(p *pen, m mat3, box float32) {
	for i := 0; i < 8; i++ {
		a := float32(i) * tau / 8
		q := p.rotated(a)
		c := m.mid
		if d := cosf(a)*lightX + sinf(a)*lightY; d > 0.3 {
			c = m.hi
		} else if d < -0.3 {
			c = m.lo
		}
		q.poly(c, 0.26, -0.1, 0.47, -0.07, 0.47, 0.07, 0.26, 0.1)
	}
	p.ring(0, 0, 0.245, 0.2, 2, m.mid)
	p.arc(0, 0, 0.3, litAngle-0.9, litAngle+0.9, 0.08, 1, m.hi)
	p.arc(0, 0, 0.3, shadeAngle-0.9, shadeAngle+0.9, 0.08, 1, m.lo)
}

// leaf returns a pointed leaf from (x0, y0) to (x1, y1), bulging by b on
// either side, as x, y pairs.
func leaf(x0, y0, x1, y1, b float32) []float32 {
	dx, dy := x1-x0, y1-y0
	nx, ny := -dy, dx
	xy := []float32{x0, y0}
	for _, t := range [3]float32{0.3, 0.55, 0.8} {
		k := b * sinf(t*pi)
		xy = append(xy, x0+dx*t+nx*k, y0+dy*t+ny*k)
	}
	xy = append(xy, x1, y1)
	for _, t := range [3]float32{0.8, 0.55, 0.3} {
		k := b * sinf(t*pi)
		xy = append(xy, x0+dx*t-nx*k, y0+dy*t-ny*k)
	}
	return xy
}

// Food: a sprout with two leaves.
func goodFood(p *pen, m mat3, box float32) {
	p.polyline(m.lo, 0.07, 1.2, 0.02, 0.47, 0, 0.2, -0.02, 0.0)
	p.poly(m.mid, leaf(-0.02, 0.06, 0.44, -0.3, 0.3)...)
	p.poly(m.hi, leaf(-0.02, 0.0, -0.42, -0.4, 0.34)...)
	if box >= 14 {
		p.line(-0.06, -0.04, -0.3, -0.28, 0.03, 0.8, m.lo)
		p.line(0.04, 0.0, 0.32, -0.21, 0.03, 0.8, m.lo)
	}
}
