// Package art draws everything in Drifthold that is a picture rather than a
// widget: rock surfaces, buildings, ships, goods and interface glyphs. All of
// it is generated in code on a gfx.Canvas; nothing is loaded from disk.
//
// Coordinates and sizes are in points, like everything on a Canvas. Hexes are
// pointy-top: the tile at hex h of a rock whose centre hex is drawn at c lies
// at c + model.HexToPixel(h, hexSize).
//
// # Look
//
// Everything is seen from straight above and lit from the upper left: faces
// towards the light take a material's light tone, faces away from it the
// dark one, and shadows fall to the lower right. Materials have three flat
// tones; the owner's accent colour marks a roof, a stripe or a lamp on every
// structure and hull.
//
// # Cost
//
// The static part of every picture is rendered once into a sprite and drawn
// with a single image call afterwards: a building's body per kind, accent,
// state and pixel size; a ship per kind, accent, size and heading step; a
// good or glyph per size; a rock's terrain per zoom. Only what moves — a
// turning dish, a blinking beacon, a glow — is drawn live, a handful of
// primitives per building. Pixel sizes are quantised, and the caches are
// bounded and evict the least recently used sprite, so a continuous zoom
// cannot grow them without limit. See CachedSprites and ReleaseSprites.
//
// Like package gfx, this package must only be used from the goroutine that
// draws.
package art

import (
	"math"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// BuildingLook describes the state a building is drawn in.
//
// The flags are looked at in this order: a Derelict is drawn as a ruin
// whatever else is set; otherwise Progress below 1 shows a building site;
// otherwise Off shows the finished building dark and still; otherwise it
// runs, with its lights on and its moving parts driven by Time.
type BuildingLook struct {
	// Accent is the owner's faction colour (neutral grey for derelicts).
	Accent gfx.Color
	// Progress is the construction progress, 0..1; 1 for a finished building.
	Progress float32
	// Derelict draws the building dark, cold and broken.
	Derelict bool
	// Off draws a finished building that is not running: no lights, no motion.
	Off bool
	// Damage is 0 for an intact building up to 1 for one about to fall.
	Damage float32
	// Time is the animation clock in seconds. Buildings of one kind move
	// in step when they share a clock; add a per-building offset to let
	// them drift apart.
	Time float32
}

// Building draws a building of the given kind centred on (cx, cy), sized to
// sit inside a hex tile whose corner radius is size.
//
// The picture is placed on whole pixels and its size is rounded to the
// nearest cached step (a whole pixel of radius up to 32 pixels, at most
// about 3 % beyond), which keeps it sharp while the view pans and zooms.
func Building(cv *gfx.Canvas, kind model.BuildingKind, cx, cy, size float32, look BuildingLook) {
	drawBuilding(cv, kind, cx, cy, size, look)
}

// BuildingIcon draws a static picture of a building kind inside r, for build
// palettes and the codex: the finished building, lit but frozen, standing on
// a tile of bare ground so that its dark parts and its shadow read on a dark
// panel as they do on a rock. The tile is the largest pointy-top hex that
// fits in r.
func BuildingIcon(cv *gfx.Canvas, kind model.BuildingKind, r gfx.Rect, accent gfx.Color) {
	drawBuildingIcon(cv, kind, r, accent)
}

// Ship draws a ship of the given kind centred on (cx, cy), about size points
// long, pointing along heading (radians, 0 = right, clockwise on screen).
// Light hulls are drawn a little shorter than size, so that a Dart beside a
// Bulwark looks its weight. The exhaust plume and engine glow reach beyond
// the hull, behind it.
func Ship(cv *gfx.Canvas, kind model.ShipKind, cx, cy, size, heading float32, accent gfx.Color) {
	drawShip(cv, kind, cx, cy, size, heading, accent, false)
}

// ShipIcon draws a static picture of a ship kind inside r.
func ShipIcon(cv *gfx.Canvas, kind model.ShipKind, r gfx.Rect, accent gfx.Color) {
	size := min(r.W, r.H) * 0.82
	drawShip(cv, kind, r.MidX(), r.MidY()-size*0.08, size, -math.Pi/2, accent, true)
}

// Good draws the symbol of a good inside r, in the theme's colour for that
// good. The symbol fills the largest square that fits in r, centred.
func Good(cv *gfx.Canvas, g model.Good, r gfx.Rect) {
	drawGood(cv, g, r)
}

// IconID names an interface glyph.
type IconID int

// Interface glyphs.
const (
	IconPower IconID = iota
	IconColonists
	IconMorale
	IconScrip
	IconResearch
	IconFleet
	IconBelt
	IconSurface
	IconPause
	IconPlay
	IconFast
	IconFaster
	IconMenu
	IconClose
	IconHelp
	IconDirectives
	IconExchange
	IconHolds
	IconWarning
	IconHousing
	IconAir
	IconStorage
	IconBerth
	IconShield
	IconTarget
	IconWorker
	IconCycle
	IconDeposit
	IconDemolish
	IconPriority
	IconOnOff
	IconLog
	IconSave
	IconCheck
	IconArrowLeft
	IconArrowRight
	IconArrowUp
	IconArrowDown
	IconLock
	NumIcons
)

// Icon draws an interface glyph inside r in the given colour. The glyph
// fills the largest square that fits in r, centred; its strokes are whole
// pixels wide and lie on the pixel grid at every size and scale.
func Icon(cv *gfx.Canvas, id IconID, r gfx.Rect, col gfx.Color) {
	drawIcon(cv, id, r, col)
}

// Surface draws rock surfaces. It caches the expensive static part of each
// rock (terrain, shading, outline) and redraws it only when the zoom changes.
//
// One Surface serves any number of rocks and zooms; it keeps the layers of
// the few most recently shown and renders the others again on demand. While
// the zoom changes from frame to frame (an animated zoom) it stretches the
// layer it has instead of painting a new one every frame, and paints the
// sharp one as soon as the size has been asked for twice in a row.
type Surface struct {
	entries []*surfEntry
	pending []surfPending
	seq     uint64
}

// NewSurface returns an empty surface renderer.
func NewSurface() *Surface { return &Surface{} }

// Draw paints the surface of rock with its centre hex at center and tiles of
// corner radius hexSize: the blob with its terrain, the deposits that viewer
// knows about (more prominent when showDeposits is set) and rubble. Buildings
// are drawn separately by the caller.
func (s *Surface) Draw(cv *gfx.Canvas, rock *model.Rock, center gfx.Pt, hexSize float32, viewer model.FactionID, showDeposits bool) {
	s.draw(cv, rock, center, hexSize, viewer, showDeposits)
}

// Release frees the cached images.
func (s *Surface) Release() {
	if s != nil {
		s.release()
	}
}

// RockBadge draws a small silhouette of a rock, radius points across, for the
// belt view and for lists: the outline follows the rock's actual blob (with
// its unevenness exaggerated so that it survives at a few pixels), in the
// colour of its class, lit from the upper left. It fits inside the circle of
// the given radius around (cx, cy).
func RockBadge(cv *gfx.Canvas, rock *model.Rock, cx, cy, radius float32) {
	drawRockBadge(cv, rock, cx, cy, radius)
}

// Star draws Emberfall, the red dwarf at the centre of the belt: a disc of
// radius r whose corona, reaching to about 4.6 r, breathes and flickers with
// time (seconds).
func Star(cv *gfx.Canvas, cx, cy, r, time float32) {
	drawStar(cv, cx, cy, r, time)
}

// Explosion draws one frame of a blast of radius r; phase runs from 0 (flash)
// to 1 (gone). The shock ring reaches r as the blast ends; embers and the
// light it throws reach about twice as far. Phases outside 0..1 draw nothing.
func Explosion(cv *gfx.Canvas, cx, cy, r, phase float32) {
	drawExplosion(cv, cx, cy, r, phase)
}
