package art

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// Buildings are drawn in a unit frame: the origin is the centre of the tile
// and one unit is the tile's corner radius, so the pointy-top hex spans
// x = ±0.866 and y = ±1. Light comes from the upper left; shadows fall to
// the lower right.
//
// Every kind is split in two layers. The body is everything that never
// moves; it is rendered once per (kind, accent, pixel size, state) into a
// sprite. The motion layer holds the few parts that turn, blink or glow; it
// is drawn live on top of the sprite while the building runs and is baked
// into the sprite, at rest and unlit, for buildings that are off, derelict
// or still under construction.

// Light direction (pointing from the surface towards the light).
const (
	lightX = -0.6
	lightY = -0.8
)

// Shadow displacement per unit of height.
const (
	shadowDX = 0.60
	shadowDY = 0.80
)

// Materials.
var (
	matSteel  = mat(0xDCE2EA, 0xAEB8C6, 0x6F7B8E) // pale plating
	matIron   = mat(0x8A95A6, 0x5A6577, 0x39414F) // structural grey
	matDark   = mat(0x4A5365, 0x2E3545, 0x1B202C) // machinery, pits
	matCream  = mat(0xF3ECDA, 0xD4CBB3, 0x9A917B) // ceramic shells
	matCopper = mat(0xE7AB7C, 0xB7714A, 0x713F2A) // hot pipework
	matSolar  = mat(0x5B84D0, 0x27468A, 0x142550) // collector cells
	matGlass  = mat(0x9FDCEF, 0x3E7F9B, 0x1B3C50) // glazing

	colWindow    = gfx.RGB(0xFFDF9A) // lit window
	colWindowOff = gfx.RGB(0x1B2130) // dark window
	colLamp      = gfx.RGB(0xFFC46B) // lantern light
	colHot       = gfx.RGB(0xFF8A3C) // furnace orange
	colHotCore   = gfx.RGB(0xFFE08A) // furnace yellow
	colLumen     = gfx.RGB(0xB690FF)
	colLumenHi   = gfx.RGB(0xE6D8FF)
	colShield    = gfx.RGB(0x7FD4F2)
	colHazard    = gfx.RGB(0xE6C84F)
	colBlueprint = gfx.RGB(0x6FB7E6)
	colRepair    = gfx.RGB(0x7BC96F)
)

// bpen is the pen buildings are drawn with.
type bpen struct {
	pen
	acc  mat3    // faction accent
	lit  bool    // lights are on
	t    float32 // animation clock, seconds
	unit float32 // pixels per unit, for level of detail
}

// big reports whether there is room for fine detail.
func (b *bpen) big() bool { return b.unit >= 26 }

// tiny reports whether only the boldest shapes survive.
func (b *bpen) tiny() bool { return b.unit < 13 }

// ow is the outline width in units.
func (b *bpen) ow() float32 { return max(0.035, 1*b.px) }

// bw is the bevel width in units.
func (b *bpen) bw() float32 { return max(0.05, 1*b.px) }

// lamp returns c when the lights are on and the dark window colour
// otherwise.
func (b *bpen) lamp(c gfx.Color) gfx.Color {
	if b.lit {
		return c
	}
	return colWindowOff
}

// halo adds a glow, but only while the lights are on.
func (b *bpen) halo(x, y, r float32, c gfx.Color) {
	if b.lit {
		b.glow(x, y, r, c)
	}
}

// shadowPoly draws the drop shadow of a prism of the given height.
func (b *bpen) shadowPoly(h float32, xy ...float32) {
	if h <= 0 {
		return
	}
	q := b.pen
	q.ox += h * shadowDX * b.k
	q.oy += h * shadowDY * b.k
	q.poly(inkShadow, xy...)
}

// shadowCircle draws the drop shadow of a cylinder.
func (b *bpen) shadowCircle(x, y, r, h float32) {
	if h <= 0 {
		return
	}
	b.circle(x+h*shadowDX, y+h*shadowDY, r, inkShadow)
}

// edges paints the lit and shaded bevels of a convex prism top: a light
// line just inside every edge that faces the light and a dark one inside
// every edge that faces away.
func (b *bpen) edges(m mat3, xy ...float32) {
	n := len(xy) / 2
	if n < 3 {
		return
	}
	// Orientation: positive area means clockwise on screen.
	var area float32
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		area += xy[2*i]*xy[2*j+1] - xy[2*j]*xy[2*i+1]
	}
	sgn := float32(1)
	if area < 0 {
		sgn = -1
	}
	bw := b.bw()
	in := b.ow()/2 + bw/2
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		x0, y0, x1, y1 := xy[2*i], xy[2*i+1], xy[2*j], xy[2*j+1]
		dx, dy := x1-x0, y1-y0
		l := float32(math.Hypot(float64(dx), float64(dy)))
		if l < 1e-4 {
			continue
		}
		dx, dy = dx/l, dy/l
		// Outward normal of a clockwise polygon is (dy, -dx); the light is
		// fixed on screen, so a turned frame turns the normal first.
		nx, ny := sgn*dy, -sgn*dx
		sx, sy := nx, ny
		if b.turned {
			sx, sy = nx*b.cos-ny*b.sin, nx*b.sin+ny*b.cos
		}
		d := sx*lightX + sy*lightY
		var c gfx.Color
		switch {
		case d > 0.3:
			c = m.hi
		case d < -0.3:
			c = m.lo
		default:
			continue
		}
		t := min(in, l/2)
		b.line(x0-nx*in+dx*t, y0-ny*in+dy*t, x1-nx*in-dx*t, y1-ny*in-dy*t, bw, 1, c)
	}
}

// slab draws a flat-topped prism: shadow, top face, bevels and outline.
func (b *bpen) slab(m mat3, h float32, xy ...float32) {
	b.shadowPoly(h, xy...)
	b.poly(m.mid, xy...)
	b.edges(m, xy...)
	b.outline(inkLine, b.ow(), 1, xy...)
}

// block is slab for an axis-aligned rectangle. In an unturned frame every
// edge, the outline and the bevels are placed on whole pixels, which keeps
// small buildings crisp.
func (b *bpen) block(m mat3, h, x, y, w, hh float32) {
	if b.turned {
		b.slab(m, h, x, y, x+w, y, x+w, y+hh, x, y+hh)
		return
	}
	s := canvasScale(b.cv)
	o := b.ow()
	r := b.cv.SnapRect(gfx.R(b.ox+(x-o/2)*b.k, b.oy+(y-o/2)*b.k, (w+o)*b.k, (hh+o)*b.k))
	op := max(1, roundf(o*b.k*s)) / s
	bp := max(1, roundf(b.bw()*b.k*s)) / s
	if h > 0 {
		b.cv.FillRect(r.Translate(h*shadowDX*b.k, h*shadowDY*b.k), b.col(inkShadow))
	}
	b.cv.FillRect(r, b.col(m.mid))
	in := r.Inset(op)
	if in.W >= 3*bp && in.H >= 3*bp {
		hi, lo := b.col(m.hi), b.col(m.lo)
		b.cv.FillRect(gfx.R(in.X, in.Y, in.W-bp, bp), hi)
		b.cv.FillRect(gfx.R(in.X, in.Y, bp, in.H-bp), hi)
		b.cv.FillRect(gfx.R(in.X+bp, in.Bottom()-bp, in.W-bp, bp), lo)
		b.cv.FillRect(gfx.R(in.Right()-bp, in.Y+bp, bp, in.H-bp), lo)
	}
	b.cv.StrokeRect(r, op, b.col(inkLine))
}

// Angles of the lit and shaded rims of a round shape.
const (
	litAngle   = -2.2143 // atan2(lightY, lightX)
	shadeAngle = litAngle + math.Pi
)

// drum draws a cylinder seen from above: shadow, top face, lit and shaded
// rim and outline.
func (b *bpen) drum(m mat3, h, x, y, r float32) {
	b.shadowCircle(x, y, r, h)
	o := b.ow()
	b.circle(x, y, r+o/2, inkLine)
	b.circle(x, y, r-o/2, m.mid)
	bw := b.bw()
	if r > 2.5*bw {
		rr := r - o/2 - bw/2
		b.arc(x, y, rr, litAngle-1.0, litAngle+1.0, bw, 1, m.hi)
		b.arc(x, y, rr, shadeAngle-1.0, shadeAngle+1.0, bw, 1, m.lo)
	}
}

// dome draws a hemisphere seen from above.
func (b *bpen) dome(m mat3, h, x, y, r float32) {
	b.shadowCircle(x, y, r, h)
	o := b.ow()
	b.circle(x, y, r+o/2, inkLine)
	b.circle(x, y, r-o/2, m.lo)
	b.circle(x+lightX*r*0.14, y+lightY*r*0.14, r*0.80, m.mid)
	b.circle(x+lightX*r*0.38, y+lightY*r*0.38, r*0.36, m.hi)
}

// pill draws a horizontal or vertical capsule (a cylinder lying on its
// side) between two end points, with the given radius.
func (b *bpen) pill(m mat3, h, x0, y0, x1, y1, r float32) {
	o := b.ow()
	horizontal := y0 == y1
	if h > 0 {
		sx, sy := h*shadowDX, h*shadowDY
		b.circle(x0+sx, y0+sy, r, inkShadow)
		b.circle(x1+sx, y1+sy, r, inkShadow)
		if horizontal {
			b.quad(x0+sx, y0-r+sy, x1-x0, 2*r, inkShadow)
		} else {
			b.quad(x0-r+sx, y0+sy, 2*r, y1-y0, inkShadow)
		}
	}
	for pass := 0; pass < 2; pass++ {
		rr, c := r+o/2, inkLine
		if pass == 1 {
			rr, c = r-o/2, m.mid
		}
		b.circle(x0, y0, rr, c)
		b.circle(x1, y1, rr, c)
		if horizontal {
			b.quad(x0, y0-rr, x1-x0, 2*rr, c)
		} else {
			b.quad(x0-rr, y0, 2*rr, y1-y0, c)
		}
	}
	// A lying cylinder is lightest along its upper-left flank and darkest
	// along the opposite one.
	bw := max(r*0.3, b.bw())
	if horizontal {
		b.quad(x0, y0-r+o/2+bw*0.5, x1-x0, bw, m.hi)
		b.quad(x0, y0+r-o/2-bw*1.3, x1-x0, bw, m.lo)
	} else {
		b.quad(x0-r+o/2+bw*0.5, y0, bw, y1-y0, m.hi)
		b.quad(x0+r-o/2-bw*1.3, y0, bw, y1-y0, m.lo)
	}
}

// window draws a small light with a faint halo.
func (b *bpen) window(x, y, w, h float32, c gfx.Color) {
	b.rect(x-w/2, y-h/2, w, h, b.lamp(c))
	if b.lit && !b.tiny() {
		b.glow(x, y, max(w, h)*1.8, alpha(c, 0.35))
	}
}

// lampDot draws a round light with a halo.
func (b *bpen) lampDot(x, y, r float32, c gfx.Color) {
	b.circle(x, y, r, b.lamp(c))
	b.halo(x, y, r*3.5, alpha(c, 0.55))
}

// buildingArt holds the two layers of a kind.
type buildingArt struct {
	body   func(b *bpen)
	motion func(b *bpen) // may be nil
}

var buildingArts [model.NumBuildingKinds]buildingArt

// ---------------------------------------------------------------------------
// Sprite cache
// ---------------------------------------------------------------------------

// Sprites are kept in three caches by size so that a handful of very large
// pictures cannot evict hundreds of small ones, and so that the memory held
// stays bounded whatever sizes are asked for.
var (
	spritesSmall  = gfx.NewSpriteCache(1024) // up to 96 px a side
	spritesMedium = gfx.NewSpriteCache(256)  // up to 256 px
	spritesLarge  = gfx.NewSpriteCache(32)   // anything bigger
)

// Domains keep the ids of the different kinds of sprites apart.
const (
	domBuilding uint64 = (iota + 1) << 60
	domShip
	domDeposit
	domRubble
	domBadge
	domGood
)

// sprite returns the cached sprite id of wpx × hpx pixels, rendering it with
// draw on a miss. The canvas handed to draw has the given scale.
func sprite(id uint64, wpx, hpx int, scale float32, draw func(cv *gfx.Canvas)) *ebiten.Image {
	sc := spritesSmall
	switch m := max(wpx, hpx); {
	case m > 256:
		sc = spritesLarge
	case m > 96:
		sc = spritesMedium
	}
	return sc.Get(id, float32(wpx)/scale, float32(hpx)/scale, scale, draw)
}

// CachedSprites returns the number of sprites the package currently holds
// (buildings, ships, deposits, rubble and rock badges).
func CachedSprites() int {
	return spritesSmall.Len() + spritesMedium.Len() + spritesLarge.Len()
}

// ReleaseSprites frees every cached sprite. They are rendered again on
// demand; call it when the art will not be shown for a long time, or after a
// change of the UI scale to drop the sprites of the old scale at once.
func ReleaseSprites() {
	spritesSmall.Clear()
	spritesMedium.Clear()
	spritesLarge.Clear()
}

// maxUnitPx is the largest unit size sprites are rendered at; bigger
// pictures are stretched from it.
const maxUnitPx = 384

// bucketPx quantises a size in pixels so that a continuous zoom produces a
// bounded number of distinct sprites: whole pixels when small, coarser steps
// (at most about 3 %) when large.
func bucketPx(ps float32) float32 {
	switch {
	case !(ps > 4):
		return 4
	case ps < 32:
		return roundf(ps)
	case ps < 64:
		return roundf(ps/2) * 2
	case ps < 128:
		return roundf(ps/4) * 4
	case ps < 256:
		return roundf(ps/8) * 8
	default:
		return min(roundf(ps/16)*16, maxUnitPx)
	}
}

// Building sprite variants.
const (
	varLit      = iota // running: lights on, motion layer left out
	varDark            // lights out, motion layer baked at rest
	varDerelict        // dark, cold, broken
)

// spriteHalf is the half extent of a building sprite in units: the art
// reaches about 0.9, shadows and halos a little further.
const spriteHalf = 1.2

// buildingSprite returns the body sprite of a building.
func buildingSprite(cv *gfx.Canvas, kind model.BuildingKind, accent gfx.Color, unitPx float32, variant, dmg int) *ebiten.Image {
	s := canvasScale(cv)
	half := int(math.Ceil(float64(spriteHalf * unitPx)))
	id := domBuilding | uint64(kind)<<40 | uint64(variant)<<36 | uint64(dmg)<<32 | packColor(accent)
	return sprite(id, 2*half, 2*half, s, func(sc *gfx.Canvas) {
		c := float32(half) / s
		b := bpen{pen: newPen(sc, c, c, unitPx/s), unit: unitPx}
		b.acc = matOf(accent)
		b.lit = variant == varLit
		if variant == varDerelict {
			b.acc = matOf(gfx.RGB(0x6C727C))
			b.tone = derelictTone
		}
		art := buildingArts[kind]
		if art.body != nil {
			art.body(&b)
		}
		if variant != varLit && art.motion != nil {
			art.motion(&b)
		}
		if variant == varDerelict {
			dmg = 4
		}
		drawDamage(&b, kind, dmg)
	})
}

// derelictTone turns any colour into its dead version: mostly grey, dark and
// a little cold.
func derelictTone(c gfx.Color) gfx.Color {
	if c == inkLine || c == inkShadow {
		return c
	}
	d := desat(c, 0.78)
	d = gfx.Darken(d, 0.42)
	cold := gfx.Color{R: 0x2A, G: 0x33, B: 0x45, A: c.A}
	return gfx.Lerp(d, cold, 0.22)
}

// damageLevel quantises BuildingLook.Damage into the four baked levels.
func damageLevel(d float32) int {
	switch {
	case !(d > 0.12):
		return 0
	case d < 0.4:
		return 1
	case d < 0.7:
		return 2
	}
	return 3
}

// damageSpots are the places, in unit coordinates, where damage shows. All
// kinds have something solid near their centre, so the marks stay close.
var damageSpots = [...][2]float32{
	{-0.18, 0.12}, {0.22, -0.2}, {0.1, 0.3}, {-0.3, -0.25}, {0.34, 0.14}, {-0.05, -0.05},
}

// drawDamage paints scorch marks and cracks for damage level 1..3, or the
// ruin of a derelict for level 4.
func drawDamage(b *bpen, kind model.BuildingKind, level int) {
	if level <= 0 {
		return
	}
	r := newRnd(uint64(kind)*977 + 13)
	scorch := gfx.Color{R: 0x0C, G: 0x0B, B: 0x0D, A: 0xB8}
	n := []int{0, 2, 4, 6, 6}[level]
	for i := 0; i < n; i++ {
		sp := damageSpots[i]
		x, y := sp[0]+r.in(-0.06, 0.06), sp[1]+r.in(-0.06, 0.06)
		rad := r.in(0.10, 0.16) + 0.02*float32(level)
		// An irregular burn: a jagged blot with a softer skirt.
		b.blob(x, y, rad*1.9, rad*1.9, alpha(scorch, 0.5))
		var xy [20]float32
		for k := 0; k < 10; k++ {
			a := tau * float32(k) / 10
			rr := rad * r.in(0.55, 1.1)
			xy[2*k], xy[2*k+1] = x+rr*cosf(a), y+rr*sinf(a)
		}
		b.poly(scorch, xy[:]...)
		// A crack running out of it.
		a := r.in(0, tau)
		cx, cy := x, y
		var line [10]float32
		for k := 0; k < 5; k++ {
			line[2*k], line[2*k+1] = cx, cy
			a += r.in(-0.9, 0.9)
			step := r.in(0.08, 0.16)
			cx, cy = cx+step*cosf(a), cy+step*sinf(a)
		}
		b.polyline(alpha(inkLine, 0.9), 0.035, 1, line[:]...)
		if level < 4 && b.lit && i%2 == 0 && level >= 2 {
			b.circle(x+rad*0.2, y-rad*0.1, max(0.03, 1.2*b.px), gfx.RGB(0xFF9A4A))
		}
	}
	if level == 4 {
		// Dust and fallen plating around a ruin.
		for i := 0; i < 9; i++ {
			a := r.in(0, tau)
			d := r.in(0.35, 0.8)
			x, y := d*cosf(a), d*sinf(a)
			s := r.in(0.03, 0.07)
			b.poly(alpha(gfx.RGB(0x151821), 0.9), x-s, y-s*0.4, x+s*0.7, y-s, x+s, y+s*0.6, x-s*0.5, y+s)
		}
	}
}

// ---------------------------------------------------------------------------
// The exported entry point
// ---------------------------------------------------------------------------

func drawBuilding(cv *gfx.Canvas, kind model.BuildingKind, cx, cy, size float32, look BuildingLook) {
	if cv == nil || cv.Dst == nil || !kind.Valid() || !(size > 0) {
		return
	}
	s := canvasScale(cv)
	ps := size * s
	unitPx := bucketPx(ps)
	unit := unitPx / s // points per unit, as rendered
	stretch := ps > unitPx*1.04 || ps < unitPx*0.9
	if stretch {
		unit = size
	}
	x, y := snapCentre(cv, cx, cy)
	ext := spriteHalf * unit
	if !cv.Visible(gfx.R(x-ext, y-ext, 2*ext, 2*ext)) {
		return
	}
	put := func(img *ebiten.Image, o *gfx.ImageOpts) {
		if stretch {
			cv.DrawImage(img, gfx.R(x-ext, y-ext, 2*ext, 2*ext), o)
			return
		}
		cv.DrawSprite(img, x, y, o)
	}

	switch {
	case look.Derelict:
		put(buildingSprite(cv, kind, look.Accent, unitPx, varDerelict, 0), nil)
	case look.Progress < 1:
		drawConstruction(cv, kind, x, y, unit, unitPx, look, put)
	case look.Off:
		img := buildingSprite(cv, kind, look.Accent, unitPx, varDark, damageLevel(look.Damage))
		put(img, &gfx.ImageOpts{Tint: gfx.RGB(0xB4B9C8)})
	default:
		dmg := damageLevel(look.Damage)
		put(buildingSprite(cv, kind, look.Accent, unitPx, varLit, dmg), nil)
		b := bpen{pen: newPen(cv, x, y, unit), unit: unit * s, lit: true, t: look.Time}
		b.acc = matOf(look.Accent)
		if m := buildingArts[kind].motion; m != nil {
			m(&b)
		}
		if dmg >= 2 {
			drawFlicker(&b, kind, dmg, look.Time)
		}
	}
}

// drawFlicker adds the unsteady glow of a fire inside a badly damaged
// building.
func drawFlicker(b *bpen, kind model.BuildingKind, dmg int, t float32) {
	r := newRnd(uint64(kind)*977 + 13)
	sp := damageSpots[0]
	x, y := sp[0]+r.in(-0.06, 0.06), sp[1]+r.in(-0.06, 0.06)
	f := 0.55 + 0.25*sinf(t*13.1) + 0.2*sinf(t*7.3+1.7)
	if dmg >= 3 {
		f += 0.25
	}
	b.glow(x, y, 0.42, alpha(colHot, 0.55*clamp01(f)))
	if dmg >= 3 {
		sp = damageSpots[2]
		g := 0.5 + 0.3*sinf(t*11.3+2.1) + 0.2*sinf(t*5.9)
		b.glow(sp[0], sp[1], 0.3, alpha(colHot, 0.45*clamp01(g)))
	}
}

// drawConstruction shows a building site: drafting marks around the plot,
// the finished structure as a pale ghost, and the part already built
// revealed from the ground line upwards.
func drawConstruction(cv *gfx.Canvas, kind model.BuildingKind, x, y, unit, unitPx float32, look BuildingLook, put func(*ebiten.Image, *gfx.ImageOpts)) {
	p := clamp01(look.Progress)
	s := canvasScale(cv)
	b := bpen{pen: newPen(cv, x, y, unit), unit: unit * s, t: look.Time}
	img := buildingSprite(cv, kind, look.Accent, unitPx, varDark, 0)

	// The plot: a dark bed with a dashed outline.
	const plot = 0.80
	hex := gfx.HexPoints(x, y, plot*unit)
	cv.FillPoly(hex[:], gfx.Color{R: 0x0D, G: 0x10, B: 0x18, A: 0x70})
	dash := max(0.16*unit, 3/s)
	cv.StrokePolyline(hex[:], true, gfx.Stroke{Width: b.w(0.03, 1), Dash: dash, Gap: dash * 0.7}, alpha(colBlueprint, 0.75))

	// The ghost of what is to come.
	put(img, &gfx.ImageOpts{Tint: gfx.Color{R: 0x86, G: 0xC4, B: 0xF0, A: 0x58}})

	// The built part, from the bottom of the plot up to the work line.
	const top, bottom = -0.86, 0.90
	lineY := bottom - (bottom-top)*p
	if p > 0 {
		ext := spriteHalf * unit
		cv.PushClip(gfx.R(x-ext, y+lineY*unit, 2*ext, ext-lineY*unit))
		put(img, nil)
		cv.PopClip()
	}

	// Scaffold: corner marks and the work line with its travelling head.
	const hw = 0.74
	c := colHazard
	w := float32(0.035)
	for _, sx := range [2]float32{-1, 1} {
		for _, sy := range [2]float32{-1, 1} {
			cx, cy := sx*hw, sy*0.78
			b.line(cx, cy, cx-sx*0.2, cy, w, 1, c)
			b.line(cx, cy, cx, cy-sy*0.2, w, 1, c)
		}
	}
	if p > 0 && p < 1 {
		b.line(-hw, lineY, hw, lineY, w, 1, alpha(c, 0.9))
		hx := hw * 0.86 * sinf(look.Time*2.3)
		b.rect(hx-0.07, lineY-0.05, 0.14, 0.10, c)
		b.glow(hx, lineY, 0.3, alpha(gfx.RGB(0xFFE9A8), 0.5+0.3*sinf(look.Time*31)))
	}
}
