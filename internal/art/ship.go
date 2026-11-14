package art

import (
	"math"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// Ships are drawn in a unit frame one ship-length long: the hull lies along
// the x axis with its nose at about +0.5 and its engines at about -0.5, and
// the frame is turned by the heading. The light stays where it is on screen
// (upper left), so the two flanks of a hull trade tones as the ship turns.
//
// Small ships are rendered into sprites, one per (kind, accent, pixel size,
// heading step); the steps are fine enough that the nose never jumps by more
// than about half a pixel. Large ships are few (portraits, close-ups) and are
// drawn directly, at their exact heading.

// shipHalf is the half extent of a ship sprite in units: the hull reaches
// 0.5, the exhaust plume and the engine glow further.
const shipHalf = 0.95

// shipLiveAbove is the length in pixels above which ships are drawn live
// instead of through the sprite cache.
const shipLiveAbove = 48

// shipScale is the drawn length of each kind relative to the nominal size:
// light hulls are visibly smaller than the frigate.
var shipScale = [model.NumShipKinds]float32{
	model.Prospector: 0.80,
	model.Hauler:     0.94,
	model.Seedship:   1.00,
	model.Dart:       0.76,
	model.Cutter:     0.92,
	model.Hammer:     0.92,
	model.Bulwark:    1.00,
}

var shipArts = [model.NumShipKinds]func(b *bpen){
	model.Prospector: shipProspector,
	model.Hauler:     shipHauler,
	model.Seedship:   shipSeedship,
	model.Dart:       shipDart,
	model.Cutter:     shipCutter,
	model.Hammer:     shipHammer,
	model.Bulwark:    shipBulwark,
}

func drawShip(cv *gfx.Canvas, kind model.ShipKind, cx, cy, size, heading float32, accent gfx.Color, forceSprite bool) {
	if cv == nil || cv.Dst == nil || !kind.Valid() || !(size > 0) {
		return
	}
	heading = finite(heading)
	s := canvasScale(cv)
	lpx := size * s
	if !forceSprite && lpx > shipLiveAbove {
		ext := shipHalf * size
		if !cv.Visible(gfx.R(cx-ext, cy-ext, 2*ext, 2*ext)) {
			return
		}
		b := bpen{pen: newPen(cv, cx, cy, size*shipScale[kind]).rotated(heading), unit: lpx, lit: true, ship: true}
		b.acc = matOf(accent)
		shipArts[kind](&b)
		return
	}
	unitPx := bucketPx(lpx)
	steps := 64
	if unitPx > 20 {
		steps = 128
	}
	step := 2 * math.Pi / float64(steps)
	hi := int(math.Round(float64(heading)/step)) % steps
	if hi < 0 {
		hi += steps
	}
	ang := float32(float64(hi) * step)
	x, y := snapCentre(cv, cx, cy)
	half := int(math.Ceil(float64(shipHalf * unitPx)))
	ext := float32(half) / s
	if !cv.Visible(gfx.R(x-ext, y-ext, 2*ext, 2*ext)) {
		return
	}
	id := domShip | uint64(kind)<<40 | uint64(hi)<<32 | packColor(accent)
	img := sprite(id, 2*half, 2*half, s, func(sc *gfx.Canvas) {
		c := float32(half) / s
		b := bpen{pen: newPen(sc, c, c, unitPx/s*shipScale[kind]).rotated(ang), unit: unitPx, lit: true, ship: true}
		b.acc = matOf(accent)
		shipArts[kind](&b)
	})
	if lpx > unitPx*1.04 {
		e := ext * lpx / unitPx
		cv.DrawImage(img, gfx.R(x-e, y-e, 2*e, 2*e), nil)
		return
	}
	cv.DrawSprite(img, x, y, nil)
}

// flank returns the tone of a surface whose normal, in the ship's frame,
// is (nx, ny): lit when it faces the light on screen, shaded when it faces
// away.
func (b *bpen) flank(m mat3, nx, ny float32) gfx.Color {
	sx, sy := nx*b.cos-ny*b.sin, nx*b.sin+ny*b.cos
	d := sx*lightX + sy*lightY
	if d >= 0 {
		return gfx.Lerp(m.mid, m.hi, d)
	}
	return gfx.Lerp(m.mid, m.lo, -d)
}

// hull draws a hull that is symmetric about its long axis. port is the
// outline of its port half as x, y pairs with y <= 0, running from the nose
// to the tail, both ends on the axis. The two halves get the tones of two
// roof slopes meeting along a ridge.
func (b *bpen) hull(m mat3, port ...float32) {
	n := len(port) / 2
	var buf [64]float32
	full := buf[:0]
	full = append(full, port...)
	star := make([]float32, 0, len(port))
	for i := n - 1; i >= 0; i-- {
		star = append(star, port[2*i], -port[2*i+1])
	}
	// Skip the two axis points when closing the full outline.
	for i := 0; i < n; i++ {
		x, y := star[2*i], star[2*i+1]
		if y == 0 {
			continue
		}
		full = append(full, x, y)
	}
	b.poly(b.flank(m, 0.25, -1), port...)
	b.poly(b.flank(m, 0.25, 1), star...)
	b.outline(inkLine, b.ow(), 1, full...)
}

// engine draws a nozzle at (x, y) firing backwards: the plume, its hot core
// and the glow around them, all in the faction's light.
func (b *bpen) engine(x, y, r, length float32) {
	c := gfx.Lerp(b.acc.mid, gfx.White, 0.25)
	b.glow(x-r*1.2, y, r*5.5, alpha(c, 0.85))
	b.poly(alpha(c, 0.7), x, y-r, x, y+r, x-length, y)
	b.poly(alpha(gfx.RGB(0xFFFFFF), 0.9), x, y-r*0.5, x, y+r*0.5, x-length*0.55, y)
}

// canopy draws a dark glazed panel.
func (b *bpen) canopy(xy ...float32) {
	b.poly(gfx.RGB(0x16283A), xy...)
}

// ---------------------------------------------------------------------------
// Prospector: a slim survey scout with a sounding dish on a side boom and a
// probe at the nose.
// ---------------------------------------------------------------------------

func shipProspector(b *bpen) {
	b.engine(-0.42, 0, 0.065, 0.26)
	// The dish on its boom, to port.
	b.strut(-0.04, -0.08, -0.04, -0.3, 0.04, matIron.hi)
	b.circle(-0.04, -0.34, 0.165+b.ow()/2, inkLine)
	b.circle(-0.04, -0.34, 0.165-b.ow()/2, matSteel.lo)
	b.circle(-0.04, -0.34, 0.115, matSteel.mid)
	b.circle(-0.04, -0.34, 0.05, b.acc.hi)
	// A stabiliser fin to starboard.
	b.poly(inkLine, -0.36, 0.08, -0.06, 0.1, -0.3, 0.27, -0.4, 0.25)
	b.poly(b.acc.mid, -0.33, 0.11, -0.12, 0.12, -0.3, 0.23, -0.36, 0.22)
	// The probe.
	b.line(0.34, 0, 0.6, 0, 0.035, 1, matIron.hi)
	b.line(0.57, -0.07, 0.57, 0.07, 0.035, 1, matIron.hi)
	b.hull(matSteel, 0.42, 0, 0.3, -0.07, 0.06, -0.14, -0.4, -0.11, -0.44, 0)
	b.poly(b.acc.mid, -0.3, -0.115, -0.16, -0.125, -0.16, 0.125, -0.3, 0.115)
	if !b.tiny() {
		b.canopy(0.3, 0, 0.2, -0.055, 0.06, -0.06, 0.06, 0.06, 0.2, 0.055)
	}
}

// ---------------------------------------------------------------------------
// Hauler: a plain spine with a cab at the front, an engine block at the
// back and three pairs of cargo pods between.
// ---------------------------------------------------------------------------

func shipHauler(b *bpen) {
	b.engine(-0.48, -0.12, 0.06, 0.2)
	b.engine(-0.48, 0.12, 0.06, 0.2)
	b.slab(matIron, 0, -0.4, -0.06, 0.36, -0.06, 0.36, 0.06, -0.4, 0.06)
	pods := [3]mat3{matCream, matSteel, matCream}
	for i, x := range [3]float32{-0.28, -0.045, 0.19} {
		for _, sy := range [2]float32{-1, 1} {
			y0, y1 := sy*0.045, sy*0.29
			if y0 > y1 {
				y0, y1 = y1, y0
			}
			m := pods[i]
			if (i == 1) == (sy < 0) {
				m = b.acc
			}
			b.slab(m, 0, x-0.1, y0, x+0.1, y0, x+0.1, y1, x-0.1, y1)
			if !b.tiny() {
				b.line(x, y0+0.03, x, y1-0.03, 0.025, 0.8, alpha(inkLine, 0.5))
			}
		}
	}
	b.slab(matIron, 0, -0.5, -0.22, -0.38, -0.22, -0.38, 0.22, -0.5, 0.22)
	b.slab(matSteel, 0, 0.32, -0.15, 0.44, -0.15, 0.5, -0.08, 0.5, 0.08, 0.44, 0.15, 0.32, 0.15)
	if !b.tiny() {
		b.canopy(0.41, -0.09, 0.46, -0.06, 0.46, 0.06, 0.41, 0.09)
	}
}

// ---------------------------------------------------------------------------
// Seedship: a drive section and two cradle arms carrying a folded Keystone,
// the same faceted hexagon that will stand on the rock.
// ---------------------------------------------------------------------------

func shipSeedship(b *bpen) {
	b.engine(-0.5, -0.12, 0.07, 0.22)
	b.engine(-0.5, 0.12, 0.07, 0.22)
	for _, sy := range [2]float32{-1, 1} {
		b.strut(-0.12, sy*0.24, 0.4, sy*0.2, 0.06, matIron.mid)
	}
	b.hull(matSteel, -0.06, 0, -0.06, -0.2, -0.2, -0.28, -0.5, -0.2, -0.5, 0)
	b.poly(b.acc.mid, -0.4, -0.2, -0.3, -0.22, -0.3, 0.22, -0.4, 0.2)
	b.slab(matSteel, 0, 0.38, -0.1, 0.47, -0.1, 0.52, -0.05, 0.52, 0.05, 0.47, 0.1, 0.38, 0.1)
	// The Keystone module.
	rim := hexXY(0.14, 0, 0.3, 0)
	b.slab(matSteel, 0, rim[:]...)
	b.pyramid(b.acc, 0.14, 0, 0.21, 6, 0)
	if !b.tiny() {
		b.circle(0.14, 0, 0.07, inkLine)
		b.circle(0.14, 0, 0.045, matCream.hi)
	}
}

// ---------------------------------------------------------------------------
// Dart: a sharp little delta, all nose and engine.
// ---------------------------------------------------------------------------

func shipDart(b *bpen) {
	b.engine(-0.36, 0, 0.075, 0.42)
	b.hull(matSteel, 0.62, 0, 0.02, -0.12, -0.5, -0.42, -0.36, -0.12, -0.42, 0)
	for _, sy := range [2]float32{-1, 1} {
		b.poly(b.acc.mid, -0.2, sy*0.2, -0.47, sy*0.38, -0.36, sy*0.16)
	}
	if !b.tiny() {
		b.canopy(0.34, 0, 0.14, -0.055, 0, 0, 0.14, 0.055)
	}
}

// ---------------------------------------------------------------------------
// Cutter: a solid gunship, hull like a spearhead with a gun pod slung on
// either side.
// ---------------------------------------------------------------------------

func shipCutter(b *bpen) {
	b.engine(-0.47, -0.08, 0.06, 0.24)
	b.engine(-0.47, 0.08, 0.06, 0.24)
	for _, sy := range [2]float32{-1, 1} {
		b.strut(-0.02, sy*0.14, -0.02, sy*0.27, 0.07, matIron.mid)
		b.line(-0.16, sy*0.28, 0.44, sy*0.28, 0.085, 2, inkLine)
		b.line(-0.14, sy*0.28, 0.42, sy*0.28, 0.035, 0.9, matIron.hi)
		b.line(-0.18, sy*0.28, 0.14, sy*0.28, 0.13, 2.4, inkLine)
		b.line(-0.16, sy*0.28, 0.12, sy*0.28, 0.08, 1.2, matIron.mid)
	}
	b.hull(matSteel, 0.52, 0, 0.3, -0.12, -0.24, -0.2, -0.47, -0.13, -0.47, 0)
	b.poly(b.acc.mid, 0.22, -0.04, -0.34, -0.065, -0.34, 0.065, 0.22, 0.04)
	if !b.tiny() {
		b.canopy(0.42, 0, 0.3, -0.05, 0.25, 0, 0.3, 0.05)
		b.circle(-0.08, 0, 0.075, inkLine)
		b.circle(-0.08, 0, 0.05, matIron.hi)
	}
}

// ---------------------------------------------------------------------------
// Hammer: a bomber shaped like its name, a broad blunt head full of
// ordnance on a narrow shaft.
// ---------------------------------------------------------------------------

func shipHammer(b *bpen) {
	b.engine(-0.47, -0.07, 0.06, 0.2)
	b.engine(-0.47, 0.07, 0.06, 0.2)
	b.hull(matSteel, 0.47, 0, 0.47, -0.3, 0.38, -0.36, 0.16, -0.36, 0.06, -0.15, -0.42, -0.15, -0.47, -0.1, -0.47, 0)
	for _, sy := range [2]float32{-1, 1} {
		y0, y1 := sy*0.09, sy*0.3
		if y0 > y1 {
			y0, y1 = y1, y0
		}
		b.poly(inkLine, 0.17, y0, 0.37, y0, 0.37, y1, 0.17, y1)
		if !b.tiny() {
			b.circle(0.27, sy*0.15, 0.04, colHot)
			b.circle(0.27, sy*0.25, 0.04, colHot)
		} else {
			b.circle(0.27, sy*0.2, 0.06, colHot)
		}
	}
	b.poly(b.acc.mid, 0.4, -0.3, 0.45, -0.28, 0.45, 0.28, 0.4, 0.3)
	b.poly(b.acc.mid, -0.3, -0.13, -0.2, -0.13, -0.2, 0.13, -0.3, 0.13)
}

// ---------------------------------------------------------------------------
// Bulwark: a heavy frigate, broad in the beam, plated in two tiers, with
// three turrets down the middle and a row of engines.
// ---------------------------------------------------------------------------

func shipBulwark(b *bpen) {
	b.engine(-0.5, 0, 0.07, 0.2)
	b.engine(-0.5, -0.17, 0.065, 0.18)
	b.engine(-0.5, 0.17, 0.065, 0.18)
	b.hull(matArmor, 0.5, 0, 0.5, -0.1, 0.3, -0.22, 0.02, -0.36, -0.36, -0.38, -0.5, -0.25, -0.5, 0)
	for _, sy := range [2]float32{-1, 1} {
		b.poly(b.acc.mid, 0.02, sy*0.33, -0.34, sy*0.35, -0.34, sy*0.27, 0.0, sy*0.25)
	}
	b.hull(matSteel, 0.38, 0, 0.3, -0.09, 0.0, -0.19, -0.36, -0.2, -0.42, -0.12, -0.42, 0)
	for _, x := range [3]float32{0.17, -0.07, -0.29} {
		if !b.tiny() {
			b.line(x, -0.03, x+0.13, -0.03, 0.03, 0.8, inkLine)
			b.line(x, 0.03, x+0.13, 0.03, 0.03, 0.8, inkLine)
		}
		b.circle(x, 0, 0.07, inkLine)
		b.circle(x, 0, 0.045, matIron.mid)
	}
}
