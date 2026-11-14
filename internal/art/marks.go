package art

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// Deposits and rubble lie on top of the static rock layer. They change while
// the game runs (veins are mined out, surveys reveal new ones, buildings
// fall), so they are not baked into it; each is a small sprite picked by
// good, richness and a per-tile variant, drawn once per tile and frame.

// markHalf is the half extent of a deposit or rubble sprite in hex units.
const markHalf = 1.06

// markVariants is the number of different arrangements per good and level.
const markVariants = 3

// depositLevel grades the amount left in a deposit from 0 (nearly mined
// out) to 3 (rich), relative to what a fresh vein of that good holds.
func depositLevel(g model.Good, amount float64) int {
	full := model.VeinAmountMax
	if g == model.Lumen {
		full = model.LumenAmountMax
	}
	switch f := amount / full; {
	case f < 0.18:
		return 0
	case f < 0.45:
		return 1
	case f < 0.8:
		return 2
	}
	return 3
}

func drawSurfaceMarks(cv *gfx.Canvas, rock *model.Rock, cx, cy, hexSize float32, viewer model.FactionID, emphasise bool) {
	s := canvasScale(cv)
	unitPx := bucketPx(hexSize * s)
	half := int(math.Ceil(float64(markHalf * unitPx)))
	ext := float32(half) / s
	// Beyond the largest sprite size the sprites are stretched.
	stretch := hexSize*s > unitPx*1.04
	if stretch {
		ext *= hexSize * s / unitPx
	}
	put := func(img *ebiten.Image, x, y float32) {
		if stretch {
			cv.DrawImage(img, gfx.R(x-ext, y-ext, 2*ext, 2*ext), nil)
			return
		}
		cv.DrawSprite(img, x, y, nil)
	}
	surveyed := rock.SurveyedBy.Has(viewer)
	emph := uint64(0)
	if emphasise {
		emph = 1
	}
	for i := range rock.Tiles {
		t := &rock.Tiles[i]
		if t.Terrain == model.TerrainVoid {
			continue
		}
		dep := t.HasDeposit() && (surveyed || t.Known.Has(viewer)) && (emphasise || t.Building == 0)
		if !dep && !t.Rubble {
			continue
		}
		h, ok := rock.HexAt(i)
		if !ok {
			continue
		}
		ux, uy := hexCentre(h)
		x, y := cx+ux*hexSize, cy+uy*hexSize*Tilt
		if !cv.Visible(gfx.R(x-ext, y-ext, 2*ext, 2*ext)) {
			continue
		}
		variant := hash2(rock.ShapeSeed^0xDE90, h.Q, h.R) % markVariants
		if t.Rubble {
			img := sprite(domRubble|variant, 2*half, 2*half, s, func(sc *gfx.Canvas) {
				c := float32(half) / s
				b := bpen{pen: newPen(sc, c, c, unitPx/s), unit: unitPx}
				paintRubble(&b, int(variant))
			})
			put(img, x, y)
		}
		if dep {
			g, level := t.Deposit, depositLevel(t.Deposit, t.Amount)
			id := domDeposit | uint64(g)<<16 | uint64(level)<<8 | variant<<4 | emph
			img := sprite(id, 2*half, 2*half, s, func(sc *gfx.Canvas) {
				c := float32(half) / s
				b := bpen{pen: newPen(sc, c, c, unitPx/s), unit: unitPx}
				paintDeposit(&b, g, level, int(variant), emphasise)
			})
			put(img, x, y)
		}
	}
}

// paintDeposit draws the outcrop of a deposit in a frame whose unit is the
// hex radius.
func paintDeposit(b *bpen, g model.Good, level, variant int, emph bool) {
	base := gfx.Default().GoodColor(int(g))
	m := mat3{gfx.Lighten(base, 0.36), base, gfx.Darken(base, 0.42)}
	rim := gfx.Color{R: 0x08, G: 0x09, B: 0x0E, A: 0x9A}
	if emph {
		// The survey overlay: the tile is tinted and ringed in the good's
		// colour and the outcrop itself is brighter.
		hex := tileXY(0.88)
		for i := 1; i < len(hex); i += 2 {
			hex[i] *= Tilt
		}
		b.poly(alpha(base, 0.2), hex[:]...)
		b.outline(alpha(inkLine, 0.5), max(0.11, 3*b.px), 0, hex[:]...)
		b.outline(alpha(gfx.Lighten(base, 0.15), 0.95), max(0.055, 1.4*b.px), 0, hex[:]...)
		m = mat3{gfx.Lighten(base, 0.5), gfx.Lighten(base, 0.12), gfx.Darken(base, 0.3)}
		rim.A = 0xD0
	} else {
		for _, c := range []*gfx.Color{&m.hi, &m.mid, &m.lo} {
			*c = gfx.Darken(desat(*c, 0.12), 0.1)
		}
	}

	r := newRnd(uint64(g)*7919 + uint64(variant)*104729 + 17)
	count := [4]int{2, 3, 4, 6}[level]
	size := [4]float32{0.72, 0.86, 1, 1.12}[level]
	spread := [4]float32{0.16, 0.27, 0.37, 0.45}[level]
	if g == model.Lumen {
		count = [4]int{1, 1, 2, 3}[level]
		spread = [4]float32{0, 0, 0.2, 0.28}[level]
		size *= 1.15
	}
	if emph {
		size *= 1.12
	}
	if b.unit < 13 {
		// Too small for a crowd: fewer, larger pieces.
		count = (count + 1) / 2
		size *= 1.5
	}

	// Scatter the pieces, keeping them apart.
	type spot struct{ x, y, a, k float32 }
	spots := make([]spot, 0, count)
	for len(spots) < count {
		var best spot
		bestD := float32(-1)
		for try := 0; try < 8; try++ {
			a, d := r.in(0, tau), spread*float32(math.Sqrt(float64(r.f())))
			c := spot{d * cosf(a), d * sinf(a) * 0.9 * Tilt, r.in(-0.55, 0.55), r.in(0.8, 1.15)}
			near := float32(9)
			for _, o := range spots {
				near = min(near, float32(math.Hypot(float64(c.x-o.x), float64(c.y-o.y))))
			}
			if near > bestD {
				best, bestD = c, near
			}
		}
		spots = append(spots, best)
	}
	// Paint from the back (top) to the front (bottom).
	for i := 1; i < len(spots); i++ {
		for j := i; j > 0 && spots[j].y < spots[j-1].y; j-- {
			spots[j], spots[j-1] = spots[j-1], spots[j]
		}
	}

	if g == model.Lumen {
		b.glow(0, 0, 0.5+0.12*float32(level), alpha(base, 0.5))
	}
	for _, sp := range spots {
		k := size * sp.k
		switch g {
		case model.Ore:
			depositNugget(b, &r, sp.x, sp.y, 0.15*k, m, rim)
		case model.Ice:
			depositShard(b, sp.x, sp.y, 0.36*k, 0.11*k, sp.a, m, rim)
		case model.Silica:
			depositPlate(b, sp.x, sp.y, 0.17*k, sp.a*0.6, m, rim)
		case model.Lumen:
			depositGem(b, sp.x, sp.y, 0.3*k, sp.a*0.5, m, rim)
		}
	}
	if g == model.Silica && b.unit >= 13 {
		// Loose sand around the plates.
		for i := 0; i < 2+2*level; i++ {
			a, d := r.in(0, tau), r.in(0.1, spread+0.16)
			b.circle(d*cosf(a), d*sinf(a)*Tilt, max(0.022, 0.6*b.px), alpha(m.hi, 0.85))
		}
	}
}

// grown returns the polygon xy pushed out from (cx, cy) by d, for rims.
func grown(dst, xy []float32, cx, cy, d float32) []float32 {
	for i := 0; i+1 < len(xy); i += 2 {
		dx, dy := xy[i]-cx, xy[i+1]-cy
		l := float32(math.Hypot(float64(dx), float64(dy)))
		if l == 0 {
			l = 1
		}
		dst = append(dst, xy[i]+dx/l*d, xy[i+1]+dy/l*d)
	}
	return dst
}

// depositNugget draws a lump of ore.
func depositNugget(b *bpen, r *rnd, x, y, s float32, m mat3, rim gfx.Color) {
	const n = 6
	var xy [2 * n]float32
	a0 := r.in(0, tau)
	for i := 0; i < n; i++ {
		a := a0 + tau*float32(i)/n
		rr := s * r.in(0.75, 1.15)
		xy[2*i], xy[2*i+1] = x+rr*cosf(a), y+rr*sinf(a)*0.9
	}
	var buf [2 * n]float32
	b.poly(rim, grown(buf[:0], xy[:], x, y, max(0.03, 1*b.px))...)
	b.poly(m.lo, xy[:]...)
	// The two faces nearest the light.
	fx, fy := x+s*0.15, y+s*0.15
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		mx, my := (xy[2*i]+xy[2*j])/2-x, (xy[2*i+1]+xy[2*j+1])/2-y
		l := float32(math.Hypot(float64(mx), float64(my)))
		switch f := facing(mx/l, my/l); {
		case f > 0.55:
			b.poly(m.hi, fx, fy, xy[2*i], xy[2*i+1], xy[2*j], xy[2*j+1])
		case f > -0.1:
			b.poly(m.mid, fx, fy, xy[2*i], xy[2*i+1], xy[2*j], xy[2*j+1])
		}
	}
}

// depositShard draws a splinter of ice of length l and half width w,
// leaning by a.
func depositShard(b *bpen, x, y, l, w, a float32, m mat3, rim gfx.Color) {
	sn, cs := sinf(a), cosf(a)
	at := func(u, v float32) (float32, float32) { return x + u*cs - v*sn, y + u*sn + v*cs }
	tx, ty := at(0, -0.62*l)
	rx, ry := at(w, -0.05*l)
	bx, by := at(0, 0.38*l)
	lx, ly := at(-w, -0.05*l)
	var buf [8]float32
	b.poly(rim, grown(buf[:0], []float32{tx, ty, rx, ry, bx, by, lx, ly}, x, y-0.1*l, max(0.03, 1*b.px))...)
	// The flank towards the light is the bright one.
	left, right := m.hi, m.lo
	if facing(-cs, -sn) < facing(cs, sn) {
		left, right = m.mid, m.hi
	}
	b.poly(right, tx, ty, rx, ry, bx, by)
	b.poly(left, tx, ty, bx, by, lx, ly)
}

// depositPlate draws a flat flake of silica.
func depositPlate(b *bpen, x, y, s, a float32, m mat3, rim gfx.Color) {
	sn, cs := sinf(a), cosf(a)
	at := func(u, v float32) (float32, float32) { return x + u*cs - v*sn, y + u*sn + v*cs }
	x0, y0 := at(-s, 0)
	x1, y1 := at(0, -0.62*s)
	x2, y2 := at(s, 0)
	x3, y3 := at(0, 0.62*s)
	var buf [8]float32
	b.poly(rim, grown(buf[:0], []float32{x0, y0, x1, y1, x2, y2, x3, y3}, x, y, max(0.03, 1*b.px))...)
	b.poly(m.lo, x0, y0, x2, y2, x3, y3)
	b.poly(m.hi, x0, y0, x1, y1, x2, y2)
	cx, cy := at(0, -0.1*s)
	b.poly(m.mid, x0, y0, cx, cy, x2, y2, x3, y3)
}

// depositGem draws a Lumen crystal of height h.
func depositGem(b *bpen, x, y, h, a float32, m mat3, rim gfx.Color) {
	sn, cs := sinf(a), cosf(a)
	w := h * 0.27
	at := func(u, v float32) (float32, float32) { return x + u*cs - v*sn, y + u*sn + v*cs }
	var p [12]float32
	for i, q := range [6][2]float32{{0, -0.5}, {1, -0.2}, {1, 0.2}, {0, 0.5}, {-1, 0.2}, {-1, -0.2}} {
		p[2*i], p[2*i+1] = at(q[0]*w, q[1]*h)
	}
	var buf [12]float32
	b.poly(rim, grown(buf[:0], p[:], x, y, max(0.03, 1*b.px))...)
	b.poly(m.lo, p[:]...)
	b.poly(m.mid, p[0], p[1], p[6], p[7], p[8], p[9], p[10], p[11])
	cx, cy := at(w*0.15, -0.12*h)
	b.poly(m.hi, p[0], p[1], cx, cy, p[10], p[11])
	b.glow(x, y, h*0.9, alpha(gfx.RGB(0xE6D8FF), 0.35))
}

// paintRubble draws the wreckage of a fallen building: a scorched patch
// strewn with plating and bent girders.
func paintRubble(b *bpen, variant int) {
	r := newRnd(uint64(variant)*7717 + 5)
	b.blob(0, 0.03*Tilt, 0.84, 0.76*Tilt, gfx.Color{R: 0x09, G: 0x08, B: 0x0A, A: 0x78})
	// Girders.
	for i := 0; i < 2; i++ {
		a := r.in(0, pi)
		l := r.in(0.3, 0.5)
		x, y := r.in(-0.2, 0.2), r.in(-0.2, 0.2)*Tilt
		dx, dy := l*cosf(a), l*sinf(a)*Tilt
		b.line(x-dx, y-dy, x+dx, y+dy, 0.1, 2.2, alpha(inkLine, 0.9))
		b.line(x-dx, y-dy, x+dx, y+dy, 0.045, 0.9, matIron.mid)
	}
	// Plating.
	tones := [...]gfx.Color{matIron.hi, matIron.mid, matSteel.lo, matSteel.mid, matIron.lo}
	n := 7
	if b.tiny() {
		n = 4
	}
	for i := 0; i < n; i++ {
		a, d := r.in(0, tau), r.in(0.08, 0.62)
		x, y := d*cosf(a), d*sinf(a)*0.9*Tilt
		s := r.in(0.09, 0.17)
		if b.tiny() {
			s *= 1.3
		}
		k := 3 + r.n(3)
		var xy [10]float32
		a0 := r.in(0, tau)
		for j := 0; j < k; j++ {
			aa := a0 + tau*float32(j)/float32(k) + r.in(-0.3, 0.3)
			rr := s * r.in(0.6, 1.1)
			xy[2*j], xy[2*j+1] = x+rr*cosf(aa), y+rr*sinf(aa)
		}
		var buf [10]float32
		b.poly(alpha(inkLine, 0.85), grown(buf[:0], xy[:2*k], x, y, max(0.025, 0.8*b.px))...)
		c := tones[r.n(len(tones))]
		b.poly(c, xy[:2*k]...)
		if !b.tiny() {
			// A lit edge on the side towards the light.
			b.line(xy[0], xy[1], xy[2], xy[3], 0.025, 0.8, alpha(gfx.Lighten(c, 0.4), 0.8))
		}
	}
	if !b.tiny() {
		for i := 0; i < 6; i++ {
			a, d := r.in(0, tau), r.in(0.3, 0.78)
			b.circle(d*cosf(a), d*sinf(a)*Tilt, max(0.02, 0.6*b.px), alpha(matDark.lo, 0.9))
		}
	}
}
