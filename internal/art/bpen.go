package art

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
)

// This file holds what structures and ships share: the materials, the pen
// they are drawn with, and the sprite caches.
//
// Both are authored in a unit frame (a structure in units of its tile's
// corner radius, standing on the tilted ground, see vol.go; a ship in
// units of its length, seen from straight above) and both are lit from the
// upper left.

// Light direction on screen (pointing from a surface towards the light).
const (
	lightX = -0.6
	lightY = -0.8
)

// Shadow displacement per unit of height, for things seen from straight
// above.
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
	matArmor  = mat(0xB3BCC9, 0x7C8696, 0x4B5462) // armour plate
	matPlate  = mat(0xC3CAD5, 0x8D97A6, 0x58616F) // heavy industry
	matFuel   = mat(0xF7B083, 0xEE7B45, 0x9A4622) // fuel tanks
	matIce    = mat(0xE2F5FC, 0x9FD8EE, 0x5A93AB) // air and ice tanks
	matLeaf   = mat(0x9ED3A0, 0x57A070, 0x2E6B4E) // greenhouse glazing
	matPad    = mat(0x77818F, 0x464F5E, 0x2A303C) // landing pads

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

// bpen is the pen structures and ships are drawn with.
type bpen struct {
	pen
	acc  mat3    // faction accent
	lit  bool    // lights are on
	t    float32 // animation clock, seconds
	unit float32 // pixels per unit, for level of detail
	ship bool    // the unit is a ship length, not a hex radius
	live bool    // drawing a motion layer on screen: marks are stamped (see live.go)

	// hits are the points on a structure where damage may show, recorded
	// by its body as it is drawn (see hit).
	hits  [6][3]float32
	nhits int

	// ext, when set, takes note of every point projected with at; the
	// tests measure structures with it.
	ext *extent
}

// extent is how far a drawing in the frame of a tile reaches: on screen
// (x to the right, y down, both in units from the ground centre), and on
// the ground beyond the outline of the tile.
type extent struct {
	minX, maxX, minY, maxY float32
	rise                   float32 // highest point on screen of anything that stands up from the ground
	ground                 float32 // furthest any ground point lies outside the tile; negative inside
	n                      int
}

func (e *extent) add(x, y, h float32) {
	sy := y*Tilt - h
	if e.n == 0 {
		e.minX, e.maxX, e.minY, e.maxY, e.ground = x, x, sy, sy, -1
	}
	e.n++
	e.minX, e.maxX = min(e.minX, x), max(e.maxX, x)
	e.minY, e.maxY = min(e.minY, sy), max(e.maxY, sy)
	if h == 0 {
		// Outside the pointy-top hex of corner radius 1?
		ax, ay := absf32(x), absf32(y)
		e.ground = max(e.ground, ax-sqrt3/2, ay-(1-ax/sqrt3))
	}
	if h > 0.02 {
		e.rise = max(e.rise, -sy)
	}
}

// livePen is the pen the motion layers of running structures are drawn
// with. It lives in a package variable: handing a local to a motion
// function through its func value would move it to the heap for every
// structure of every frame.
var livePen bpen

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

// Angles of the lit and shaded rims of a round shape.
const (
	litAngle   = -2.2143 // atan2(lightY, lightX)
	shadeAngle = litAngle + math.Pi
)

// colPit is the dark at the bottom of shafts, stacks and wells.
var colPit = gfx.RGB(0x0D0F16)

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
	domShip uint64 = (iota + 1) << 60
	domDeposit
	domRubble
	domBadge
	domGood
	domIcon
	domStruct
)

// (The stamps of live.go are few and tiny and are kept apart from these
// caches.)

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
// (structures, ships, deposits, rubble, goods, glyphs and rock badges).
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
	releaseStamps()
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

// Structure sprite variants.
const (
	varLit      = iota // running: lights on, motion layer left out
	varDark            // lights out, motion layer baked at rest
	varDerelict        // dark, cold, broken
	varGhost           // blueprint drawing of the finished structure
)

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
