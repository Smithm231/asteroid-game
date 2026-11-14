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
	ship bool    // the unit is a ship length, not a hex radius
}

// big reports whether there is room for fine detail.
func (b *bpen) big() bool {
	if b.ship {
		return b.unit >= 40
	}
	return b.unit >= 26
}

// tiny reports whether only the boldest shapes survive.
func (b *bpen) tiny() bool {
	if b.ship {
		return b.unit < 20
	}
	return b.unit < 13
}

// ow is the outline width in units.
func (b *bpen) ow() float32 {
	if b.ship {
		return max(0.03, 0.7*b.px)
	}
	return max(0.035, 1*b.px)
}

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
	b.circle(x, y, r, m.mid)
	bw := b.bw()
	if r > 2.5*bw {
		rr := r - o/2 - bw/2
		b.arc(x, y, rr, litAngle-1.0, litAngle+1.0, bw, 1, m.hi)
		b.arc(x, y, rr, shadeAngle-1.0, shadeAngle+1.0, bw, 1, m.lo)
	}
	b.ring(x, y, r, o, 1, inkLine)
}

// dome draws a hemisphere seen from above.
func (b *bpen) dome(m mat3, h, x, y, r float32) {
	b.shadowCircle(x, y, r, h)
	b.circle(x, y, r, m.lo)
	b.circle(x+lightX*r*0.14, y+lightY*r*0.14, r*0.80, m.mid)
	b.circle(x+lightX*r*0.38, y+lightY*r*0.38, r*0.36, m.hi)
	b.ring(x, y, r, b.ow(), 1, inkLine)
}

// pill draws a horizontal or vertical capsule (a cylinder lying on its
// side) between two end points, with the given radius.
func (b *bpen) pill(m mat3, h, x0, y0, x1, y1, r float32) {
	o := b.ow()
	horizontal := y0 == y1
	body := func(dx, dy float32, c gfx.Color) {
		b.circle(x0+dx, y0+dy, r, c)
		b.circle(x1+dx, y1+dy, r, c)
		if horizontal {
			b.quad(x0+dx, y0-r+dy, x1-x0, 2*r, c)
		} else {
			b.quad(x0-r+dx, y0+dy, 2*r, y1-y0, c)
		}
	}
	if h > 0 {
		body(h*shadowDX, h*shadowDY, inkShadow)
	}
	body(0, 0, m.mid)
	// A lying cylinder is lightest along its upper-left flank and darkest
	// along the opposite one.
	bw := max(r*0.3, b.bw())
	if horizontal {
		b.quad(x0, y0-r+o/2+bw*0.5, x1-x0, bw, m.hi)
		b.quad(x0, y0+r-o/2-bw*1.3, x1-x0, bw, m.lo)
		b.arc(x0, y0, r, pi/2, 3*pi/2, o, 1, inkLine)
		b.arc(x1, y1, r, -pi/2, pi/2, o, 1, inkLine)
		b.line(x0, y0-r, x1, y1-r, o, 1, inkLine)
		b.line(x0, y0+r, x1, y1+r, o, 1, inkLine)
	} else {
		b.quad(x0-r+o/2+bw*0.5, y0, bw, y1-y0, m.hi)
		b.quad(x0+r-o/2-bw*1.3, y0, bw, y1-y0, m.lo)
		b.arc(x0, y0, r, pi, 2*pi, o, 1, inkLine)
		b.arc(x1, y1, r, 0, pi, o, 1, inkLine)
		b.line(x0-r, y0, x1-r, y1, o, 1, inkLine)
		b.line(x0+r, y0, x1+r, y1, o, 1, inkLine)
	}
}

// colPit is the dark at the bottom of shafts, stacks and wells.
var colPit = gfx.RGB(0x0D0F16)

// well draws a round opening: its floor in c inside an ink rim.
func (b *bpen) well(x, y, r float32, c gfx.Color) {
	b.circle(x, y, r, c)
	b.ring(x, y, r, b.ow(), 1.2, inkLine)
}

// window draws a small light with a faint halo.
func (b *bpen) window(x, y, w, h float32, c gfx.Color) {
	b.rect(x-w/2, y-h/2, w, h, b.lamp(c))
	if b.lit && !b.tiny() {
		b.glow(x, y, max(w, h)*1.5, alpha(c, 0.16))
	}
}

// lampDot draws a round light with a halo.
func (b *bpen) lampDot(x, y, r float32, c gfx.Color) {
	b.circle(x, y, r, b.lamp(c))
	b.halo(x, y, r*3, alpha(c, 0.3))
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
	spritesLarge  = gfx.NewSpriteCache(24)   // anything bigger
)

// Domains keep the ids of the different kinds of sprites apart.
const (
	domBuilding uint64 = (iota + 1) << 60
	domShip
	domDeposit
	domRubble
	domBadge
	domGood
	domIcon
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
const maxUnitPx = 256

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
		return maxUnitPx
	}
}

// Building sprite variants.
const (
	varLit      = iota // running: lights on, motion layer left out
	varDark            // lights out, motion layer baked at rest
	varDerelict        // dark, cold, broken
	varGhost           // blueprint drawing of the finished building
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
		switch variant {
		case varDerelict:
			b.acc = matOf(gfx.RGB(0x6C727C))
			b.tone = derelictTone
		case varGhost:
			b.tone = ghostTone
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
		if variant != varGhost {
			drawDamage(&b, kind, dmg)
		}
	})
}

// derelictTone turns any colour into its dead version: mostly grey, darker
// and a little cold.
func derelictTone(c gfx.Color) gfx.Color {
	if c == inkLine || c == inkShadow {
		return c
	}
	d := desat(c, 0.82)
	d = gfx.Darken(d, 0.3)
	cold := gfx.Color{R: 0x36, G: 0x41, B: 0x58, A: c.A}
	return gfx.Lerp(d, cold, 0.25)
}

// ghostTone turns a building into a drafting-table drawing of itself: pale
// blue lines, faint washes for the faces, no shadows.
func ghostTone(c gfx.Color) gfx.Color {
	switch c {
	case inkShadow:
		return gfx.Color{}
	case inkLine:
		return gfx.Color{R: 0x9C, G: 0xD3, B: 0xF7, A: 0xE6}
	}
	a := (0.03 + 0.13*gfx.Luma(c)) * float32(c.A) / 255
	return gfx.WithAlpha(colBlueprint, a)
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
	scorch := gfx.Color{R: 0x12, G: 0x0E, B: 0x10, A: 0xC8}
	crack := alpha(inkLine, 0.92)
	burns := [...]int{0, 1, 3, 5, 2}[level]
	cracks := [...]int{0, 2, 3, 5, 6}[level]
	for i := 0; i < len(damageSpots); i++ {
		sp := damageSpots[i]
		x, y := sp[0]+r.in(-0.06, 0.06), sp[1]+r.in(-0.06, 0.06)
		rad := r.in(0.06, 0.10) + 0.012*float32(min(level, 3))
		var xy [18]float32
		for k := 0; k < 9; k++ {
			a := tau * float32(k) / 9
			rr := rad * r.in(0.5, 1.15)
			xy[2*k], xy[2*k+1] = x+rr*cosf(a), y+rr*sinf(a)
		}
		a := r.in(0, tau)
		var line [10]float32
		cx, cy := x, y
		for k := 0; k < 5; k++ {
			line[2*k], line[2*k+1] = cx, cy
			a += r.in(-0.9, 0.9)
			step := r.in(0.07, 0.14)
			cx, cy = cx+step*cosf(a), cy+step*sinf(a)
		}
		if i < burns {
			b.blob(x, y, rad*2.1, rad*2.1, alpha(scorch, 0.42))
			b.poly(scorch, xy[:]...)
			if level < 4 && b.lit && level >= 2 && i%2 == 0 {
				b.circle(x+rad*0.25, y-rad*0.1, max(0.028, 1.1*b.px), gfx.RGB(0xFF9A4A))
			}
		}
		if i < cracks {
			b.polyline(crack, 0.03, 1, line[:]...)
		}
	}
	if level == 4 {
		// Plating fallen around a ruin.
		for i := 0; i < 7; i++ {
			a := r.in(0, tau)
			d := r.in(0.45, 0.8)
			x, y := d*cosf(a), d*sinf(a)
			s := r.in(0.03, 0.06)
			b.poly(alpha(gfx.RGB(0x1A1E28), 0.9), x-s, y-s*0.4, x+s*0.7, y-s, x+s, y+s*0.6, x-s*0.5, y+s)
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
	look.Time = finite(look.Time)
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
		// The pen lives in a package variable: handing a local to the
		// motion function through its func value would move it to the
		// heap for every building of every frame.
		b := &livePen
		*b = bpen{pen: newPen(cv, x, y, unit), unit: unit * s, lit: true, t: look.Time}
		b.acc = matOf(look.Accent)
		if m := buildingArts[kind].motion; m != nil {
			m(b)
		}
		if dmg >= 2 {
			drawFlicker(b, kind, dmg, look.Time)
		}
	}
}

// iconGround is the colour of the tile under a building icon: the slate of a
// Stony rock, sunk towards the panels it is shown on.
var iconGround = gfx.Lerp(gfx.Default().RockColor(int(model.ClassStony)), gfx.RGB(0x12151F), 0.3)

func drawBuildingIcon(cv *gfx.Canvas, kind model.BuildingKind, r gfx.Rect, accent gfx.Color) {
	if cv == nil || cv.Dst == nil || !kind.Valid() || r.Empty() {
		return
	}
	// A pointy-top hex of radius R is 2R tall and √3·R wide.
	size := min(r.H/2, r.W/sqrt3)
	x, y := snapCentre(cv, r.MidX(), r.MidY())
	cv.FillHexV(x, y, size, gfx.Lighten(iconGround, 0.05), gfx.Darken(iconGround, 0.1))
	cv.StrokeHex(x, y, size-0.5/canvasScale(cv), 1/canvasScale(cv), alpha(gfx.Lighten(iconGround, 0.25), 0.5))
	drawBuilding(cv, kind, x, y, size*0.94, BuildingLook{Accent: accent, Progress: 1})
}

// livePen is the pen the motion layers of running buildings are drawn with.
var livePen bpen

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

// drawConstruction shows a building site: a blueprint of the finished
// structure on a dark plot, the part already built standing below the work
// line, and drafting marks at the corners of the plot.
func drawConstruction(cv *gfx.Canvas, kind model.BuildingKind, x, y, unit, unitPx float32, look BuildingLook, put func(*ebiten.Image, *gfx.ImageOpts)) {
	p := clamp01(look.Progress)
	b := bpen{pen: newPen(cv, x, y, unit), unit: unitPx, t: look.Time}

	// The plot: blueprint paper, with a mark at every corner.
	const plot = 0.85
	hex := gfx.HexPoints(x, y, plot*unit)
	cv.FillPoly(hex[:], gfx.Color{R: 0x0E, G: 0x1A, B: 0x2E, A: 0xA6})
	cv.StrokePoly(hex[:], b.w(0.025, 1), alpha(colBlueprint, 0.35))
	mw := b.w(0.04, 1)
	for i, c := range hex {
		a, n := hex[(i+5)%6], hex[(i+1)%6]
		const l = 0.24
		cv.Polyline([]gfx.Pt{
			{X: c.X + (a.X-c.X)*l, Y: c.Y + (a.Y-c.Y)*l}, c,
			{X: c.X + (n.X-c.X)*l, Y: c.Y + (n.Y-c.Y)*l},
		}, mw, colHazard)
	}

	// The drawing above the work line, the built part below it. The line
	// starts at the bottom of the plot and climbs.
	lineY := plot - 2*plot*p
	ext := spriteHalf * unit
	split := cv.Snap(y + lineY*unit)
	cv.PushClip(gfx.R(x-ext, y-ext, 2*ext, split-(y-ext)))
	put(buildingSprite(cv, kind, look.Accent, unitPx, varGhost, 0), nil)
	cv.PopClip()
	if p > 0 {
		cv.PushClip(gfx.R(x-ext, split, 2*ext, y+ext-split))
		put(buildingSprite(cv, kind, look.Accent, unitPx, varDark, 0), nil)
		cv.PopClip()
	}

	// The work line, as wide as the plot is at that height, with its
	// travelling head.
	if p > 0 && p < 1 {
		ay := lineY
		if ay < 0 {
			ay = -ay
		}
		hw := min(plot*sqrt3/2, (plot-ay)*sqrt3)
		if hw > 0.1 {
			b.line(-hw, lineY, hw, lineY, 0.035, 1, alpha(colHazard, 0.95))
			hx := (hw - 0.08) * sinf(look.Time*2.3)
			b.rect(hx-0.07, lineY-0.045, 0.14, 0.09, gfx.RGB(0xFFF3C4))
			b.glow(hx, lineY, 0.22, alpha(gfx.RGB(0xFFE9A8), 0.35+0.2*sinf(look.Time*31)))
		}
	}
}
