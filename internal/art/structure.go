package art

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// Structures stand on their tile and rise from it. Every kind is split in
// two layers. The body is everything that never moves; it is rendered once
// per (kind, accent, pixel size, state) into a sprite. The motion layer
// holds the few parts that turn, blink or glow; it is drawn live on top of
// the sprite while the structure runs (see live.go) and is baked into the
// sprite, at rest and unlit, for structures that are off, derelict or still
// under construction. The sprite is taller than it is wide and is hung by
// the tile's ground centre, not by its middle.

// The extent of a structure sprite around the ground centre of its tile, in
// units of the tile's corner radius.
const (
	structHalfW  = 1.3
	structAbove  = 2.0
	structBelow  = Tilt + 0.3
	structFront  = 0.84 * Tilt // how far below the ground centre a footprint reaches on screen
	structGround = 0.9         // radius of the plot marked out for a building site
)

// structArt is the angled picture of a building kind.
type structArt struct {
	body   func(b *bpen)
	motion func(b *bpen) // may be nil
	top    float32       // height of the art above the ground centre, in units
}

var structArts [model.NumBuildingKinds]structArt

// structHits remembers, per kind, the damage points its body recorded the
// last time it was drawn; the live fire of a damaged structure burns there.
var structHits [model.NumBuildingKinds]struct {
	pts [6][3]float32
	n   int
}

// buildingTop is BuildingTop.
func buildingTop(kind model.BuildingKind, size float32) float32 {
	if !kind.Valid() {
		return size * 0.8
	}
	return size * structArts[kind].top
}

// structDims returns the pixel geometry of a structure sprite at unitPx
// pixels per unit: half its width, and how far it reaches above and below
// the ground centre.
func structDims(unitPx float32) (halfW, above, below int) {
	return int(math.Ceil(float64(structHalfW * unitPx))),
		int(math.Ceil(float64(structAbove * unitPx))),
		int(math.Ceil(float64(structBelow * unitPx)))
}

// structSprite returns the body sprite of a structure.
func structSprite(cv *gfx.Canvas, kind model.BuildingKind, accent gfx.Color, unitPx float32, variant, dmg int) *ebiten.Image {
	s := canvasScale(cv)
	halfW, above, below := structDims(unitPx)
	id := domStruct | uint64(kind)<<40 | uint64(variant)<<36 | uint64(dmg)<<32 | packColor(accent)
	return sprite(id, 2*halfW, above+below, s, func(sc *gfx.Canvas) {
		b := bpen{pen: newPen(sc, float32(halfW)/s, float32(above)/s, unitPx/s), unit: unitPx}
		b.acc = matOf(accent)
		b.lit = variant == varLit
		switch variant {
		case varDerelict:
			b.acc = matOf(gfx.RGB(0x6C727C))
			b.tone = derelictTone
		case varGhost:
			b.tone = ghostTone
		}
		art := structArts[kind]
		if art.body != nil {
			art.body(&b)
		}
		structHits[kind].pts, structHits[kind].n = b.hits, b.nhits
		if variant != varLit && art.motion != nil {
			art.motion(&b)
		}
		if variant == varDerelict {
			dmg = 4
		}
		if variant != varGhost {
			structDamage(&b, kind, dmg)
		}
	})
}

// structBlit places structure sprites of one size around one ground point.
type structBlit struct {
	cv                *gfx.Canvas
	x, y              float32 // snapped ground centre, points
	halfW, above, all float32 // sprite geometry in points, stretch included
	stretch           float32 // how far sprites are stretched from the size they were rendered at
}

func (p *structBlit) put(img *ebiten.Image, o *gfx.ImageOpts) {
	p.cv.DrawImage(img, gfx.R(p.x-p.halfW, p.y-p.above, 2*p.halfW, p.all), o)
}

func drawStruct(cv *gfx.Canvas, kind model.BuildingKind, cx, cy, size float32, look BuildingLook) {
	if cv == nil || cv.Dst == nil || !kind.Valid() || !(size > 0) {
		return
	}
	look.Time = finite(look.Time)
	s := canvasScale(cv)
	ps := size * s
	unitPx := bucketPx(ps)
	unit := unitPx / s // points per unit, as rendered
	stretch := float32(1)
	if ps > unitPx*1.04 || ps < unitPx*0.9 {
		stretch, unit = ps/unitPx, size
	}
	x, y := snapCentre(cv, cx, cy)
	if !cv.Visible(gfx.R(x-structHalfW*unit, y-structAbove*unit, 2*structHalfW*unit, (structAbove+structBelow)*unit)) {
		return
	}
	halfW, above, below := structDims(unitPx)
	blit := structBlit{cv: cv, x: x, y: y,
		halfW: float32(halfW) * stretch / s, above: float32(above) * stretch / s, all: float32(above+below) * stretch / s,
		stretch: stretch}

	switch {
	case look.Derelict:
		blit.put(structSprite(cv, kind, look.Accent, unitPx, varDerelict, 0), nil)
	case look.Progress < 1:
		structSite(&blit, kind, unit, unitPx, look)
	case look.Off:
		img := structSprite(cv, kind, look.Accent, unitPx, varDark, damageLevel(look.Damage))
		blit.put(img, &gfx.ImageOpts{Tint: gfx.RGB(0xB4B9C8)})
	default:
		dmg := damageLevel(look.Damage)
		blit.put(structSprite(cv, kind, look.Accent, unitPx, varLit, dmg), nil)
		b := &livePen
		*b = bpen{pen: newPen(cv, x, y, unit), unit: unit * s, lit: true, live: true, t: look.Time}
		b.acc = matOf(look.Accent)
		if m := structArts[kind].motion; m != nil {
			m(b)
		}
		if dmg >= 2 {
			structFire(b, kind, dmg, look.Time)
		}
	}
}

// structAnchor returns the i-th damage point of a kind, with a fallback
// near the base for kinds that recorded none.
func structAnchor(kind model.BuildingKind, i int) (x, y, h float32) {
	hs := &structHits[kind]
	if hs.n == 0 {
		fall := [3][3]float32{{-0.2, 0.3, 0.12}, {0.25, 0.25, 0.14}, {0, 0.4, 0.08}}
		p := fall[i%3]
		return p[0], p[1], p[2]
	}
	p := hs.pts[i%hs.n]
	return p[0], p[1], p[2]
}

// structDamage paints scorch marks and cracks on a structure for damage
// level 1..3, or the ruin of a derelict for level 4. The marks sit on the
// points the body recorded with hit.
func structDamage(b *bpen, kind model.BuildingKind, level int) {
	if level <= 0 {
		return
	}
	r := newRnd(uint64(kind)*977 + 13)
	scorch := gfx.Color{R: 0x12, G: 0x0E, B: 0x10, A: 0xC8}
	crack := alpha(inkLine, 0.92)
	burns := [...]int{0, 1, 3, 5, 3}[level]
	cracks := [...]int{0, 2, 3, 5, 6}[level]
	for i := 0; i < 6; i++ {
		ax, ay, ah := structAnchor(kind, i)
		c := b.at(ax+r.in(-0.05, 0.05), ay, ah+r.in(-0.05, 0.05))
		// A plain pen centred on the mark: offsets are in screen units.
		q := b.pen
		q.ox, q.oy = c.X, c.Y
		rad := r.in(0.06, 0.1) + 0.012*float32(min(level, 3))
		var xy [18]float32
		for k := 0; k < 9; k++ {
			a := tau * float32(k) / 9
			rr := rad * r.in(0.5, 1.15)
			xy[2*k], xy[2*k+1] = rr*cosf(a), rr*sinf(a)*0.85
		}
		a := r.in(0, tau)
		var line [10]float32
		var lx, ly float32
		for k := 0; k < 5; k++ {
			line[2*k], line[2*k+1] = lx, ly
			a += r.in(-0.9, 0.9)
			step := r.in(0.06, 0.12)
			lx, ly = lx+step*cosf(a), ly+step*sinf(a)
		}
		if i < burns {
			q.blob(0, 0, rad*2, rad*1.8, alpha(scorch, 0.42))
			q.poly(scorch, xy[:]...)
			if level < 4 && b.lit && level >= 2 && i%2 == 0 {
				q.circle(rad*0.25, -rad*0.1, max(0.028, 1.1*b.px), gfx.RGB(0xFF9A4A))
			}
		}
		if i < cracks {
			q.polyline(crack, 0.03, 1, line[:]...)
		}
	}
	if level == 4 {
		// Plating fallen on the ground around a ruin.
		for i := 0; i < 7; i++ {
			a := r.in(0, tau)
			d := r.in(0.5, 0.85)
			c := b.at(d*cosf(a), d*sinf(a), 0)
			q := b.pen
			q.ox, q.oy = c.X, c.Y
			s := r.in(0.03, 0.06)
			q.poly(alpha(gfx.RGB(0x1A1E28), 0.9), -s, -s*0.3, s*0.7, -s*0.6, s, s*0.4, -s*0.5, s*0.6)
		}
	}
}

// structFire adds the unsteady glow of a fire inside a badly damaged
// structure, and the smoke that rises from it.
func structFire(b *bpen, kind model.BuildingKind, dmg int, t float32) {
	x, y, h := structAnchor(kind, 0)
	f := 0.55 + 0.25*sinf(t*13.1) + 0.2*sinf(t*7.3+1.7)
	if dmg >= 3 {
		f += 0.25
	}
	b.glowAt(x, y, h, 0.42, alpha(colHot, 0.55*clamp01(f)))
	n := 2
	if dmg >= 3 {
		n = 3
		x2, y2, h2 := structAnchor(kind, 2)
		g := 0.5 + 0.3*sinf(t*11.3+2.1) + 0.2*sinf(t*5.9)
		b.glowAt(x2, y2, h2, 0.3, alpha(colHot, 0.45*clamp01(g)))
	}
	for k := 0; k < n; k++ {
		d := frac(t*0.3 + float32(k)/float32(n))
		b.puffAt(x+0.22*d, y, h+0.1+0.75*d, 0.07+0.11*d, gfx.Color{R: 0x3A, G: 0x36, B: 0x3A, A: uint8(120 * (1 - d))})
	}
}

// tileXY returns the corners of the pointy-top tile of radius r around the
// ground centre as x, y pairs on the ground: right-hand upper corner first,
// then clockwise on screen.
func tileXY(r float32) [12]float32 {
	const w = sqrt3 / 2
	return [12]float32{w * r, -r / 2, w * r, r / 2, 0, r, -w * r, r / 2, -w * r, -r / 2, 0, -r}
}

// sitePlot returns the picture of the ground of a building site at unitPx
// pixels per unit, and half its width and height in pixels; the ground
// centre of the tile is in its middle.
func sitePlot(s, unitPx float32) (img *ebiten.Image, halfW, halfH float32) {
	halfW = float32(math.Ceil(float64(unitPx)))
	halfH = float32(math.Ceil(float64(unitPx * (structGround*Tilt + 0.1))))
	id := domStruct | 0xFF<<40 | uint64(unitPx*4)
	return sprite(id, int(2*halfW), int(2*halfH), s, func(sc *gfx.Canvas) {
		b := bpen{pen: newPen(sc, halfW/s, halfH/s, unitPx/s), unit: unitPx}
		plot := tileXY(structGround)
		var g [6]gfx.Pt
		for i := range g {
			g[i] = b.at(plot[2*i], plot[2*i+1], 0)
		}
		sc.FillPoly(g[:], gfx.Color{R: 0x0E, G: 0x1A, B: 0x2E, A: 0xA6})
		sc.StrokePoly(g[:], b.w(0.025, 1), alpha(colBlueprint, 0.35))
		mw := b.w(0.04, 1)
		for i, c := range g {
			a, n := g[(i+5)%6], g[(i+1)%6]
			const l = 0.24
			sc.Polyline([]gfx.Pt{
				{X: c.X + (a.X-c.X)*l, Y: c.Y + (a.Y-c.Y)*l}, c,
				{X: c.X + (n.X-c.X)*l, Y: c.Y + (n.Y-c.Y)*l},
			}, mw, colHazard)
		}
	}), halfW, halfH
}

// structSite shows a building site: the plot marked out on the ground, a
// blueprint of the finished structure standing on it, the part already
// built rising below the work line, and a scaffold ring that climbs with
// the work.
func structSite(blit *structBlit, kind model.BuildingKind, unit, unitPx float32, look BuildingLook) {
	cv := blit.cv
	p := clamp01(look.Progress)
	b := &livePen
	*b = bpen{pen: newPen(cv, blit.x, blit.y, unit), unit: unitPx, live: true, t: look.Time}
	topH := structArts[kind].top

	// The plot: blueprint paper with a mark at every corner.
	plot := tileXY(structGround)
	{
		s := canvasScale(cv)
		img, hw, hh := sitePlot(s, unitPx)
		w, h := 2*hw*blit.stretch/s, 2*hh*blit.stretch/s
		cv.DrawImage(img, gfx.R(blit.x-w/2, blit.y-h/2, w, h), nil)
	}

	// The scaffold ring hangs at the height the work has reached; its far
	// half goes behind the structure, its near half in front.
	hr := p * topH
	ring := func(from, to int) {
		for i := from; i < to; i++ {
			k, n := i%6, (i+1)%6
			b.rod(alpha(colHazard, 0.95), 0.035, 1, plot[2*k], plot[2*k+1], hr, plot[2*n], plot[2*n+1], hr)
		}
	}
	post := func(k int) {
		b.rod(alpha(colHazard, 0.7), 0.03, 1, plot[2*k], plot[2*k+1], 0, plot[2*k], plot[2*k+1], hr)
	}
	working := p > 0 && p < 1
	if working {
		ring(4, 7) // upper left, top, upper right, lower right corners: the far side
		post(0)
		post(4)
	}

	// The drawing above the work line, the built part below it. The line
	// starts at the front of the footprint and climbs to the top.
	lineY := structFront - (structFront+topH)*p
	split := cv.Snap(blit.y + lineY*unit)
	top := blit.y - blit.above
	cv.PushClip(gfx.R(blit.x-blit.halfW, top, 2*blit.halfW, split-top))
	blit.put(structSprite(cv, kind, look.Accent, unitPx, varGhost, 0), nil)
	cv.PopClip()
	if p > 0 {
		cv.PushClip(gfx.R(blit.x-blit.halfW, split, 2*blit.halfW, top+blit.all-split))
		blit.put(structSprite(cv, kind, look.Accent, unitPx, varDark, 0), nil)
		cv.PopClip()
	}

	if working {
		ring(1, 4) // lower right, bottom, lower left, upper left: the near side
		post(1)
		post(3)
		// The welding head runs to and fro along the near side.
		u := 0.5 + 0.5*sinf(look.Time*2.3) // 0..1 from the lower right to the lower left corner
		var hx, hy float32
		if u < 0.5 {
			hx, hy = lerpf(plot[2], plot[4], u*2), lerpf(plot[3], plot[5], u*2)
		} else {
			hx, hy = lerpf(plot[4], plot[6], u*2-1), lerpf(plot[5], plot[7], u*2-1)
		}
		b.puffAt(hx, hy, hr, max(0.05, 1.2*b.px), gfx.RGB(0xFFF3C4))
		b.glowAt(hx, hy, hr, 0.24, alpha(gfx.RGB(0xFFE9A8), 0.4+0.2*sinf(look.Time*31)))
	}
}

// iconPlate is the colour of the tile under a structure icon: the slate of
// a Stony rock, sunk towards the panels it is shown on.
var iconPlate = gfx.Lerp(gfx.Default().RockColor(int(model.ClassStony)), gfx.RGB(0x12151F), 0.3)

// drawStructIcon draws a structure standing on a small slab of ground,
// fitted into r.
func drawStructIcon(cv *gfx.Canvas, kind model.BuildingKind, r gfx.Rect, accent gfx.Color) {
	if cv == nil || cv.Dst == nil || !kind.Valid() || r.Empty() {
		return
	}
	topH := structArts[kind].top
	// The picture spans 2·0.9 units across and, from the top of the
	// structure to the bottom of the slab, topH + 0.7 units down.
	size := min(r.W/1.84, r.H/(topH+0.75))
	x, gy := snapCentre(cv, r.MidX(), r.MidY()+(topH-0.6)/2*size)
	b := bpen{pen: newPen(cv, x, gy, size), unit: size * canvasScale(cv)}
	tile := tileXY(1)
	const depth = 0.13
	// The slab: its two near faces, then its top.
	b.face(gfx.Darken(iconPlate, 0.28), tile[6], tile[7], 0, tile[4], tile[5], 0, tile[4], tile[5], -depth, tile[6], tile[7], -depth)
	b.face(gfx.Darken(iconPlate, 0.5), tile[4], tile[5], 0, tile[2], tile[3], 0, tile[2], tile[3], -depth, tile[4], tile[5], -depth)
	var g [6]gfx.Pt
	for i := range g {
		g[i] = b.at(tile[2*i], tile[2*i+1], 0)
	}
	cv.FillPoly(g[:], gfx.Lighten(iconPlate, 0.04))
	cv.StrokePoly(g[:], 1/canvasScale(cv), alpha(gfx.Lighten(iconPlate, 0.3), 0.55))
	drawStruct(cv, kind, x, gy, size*0.94, BuildingLook{Accent: accent, Progress: 1})
}
