// Package art draws everything in Drifthold that is a picture rather than a
// widget: rock surfaces, buildings, ships, goods and interface glyphs. All of
// it is generated in code on a gfx.Canvas; nothing is loaded from disk.
//
// Coordinates and sizes are in points, like everything on a Canvas.
//
// # The surface view
//
// A rock's surface is seen at an angle, from above and in front. Its hexes
// are pointy-top and lie on a ground plane foreshortened vertically by Tilt:
// the tile at hex h of a rock whose centre hex is drawn at c has its centre
// at HexPos(c, h, hexSize), TileFill and TileStroke draw its outline, and
// PickHex finds the tile under a point. Surface.Draw paints the rock as a
// solid body (the plateau of tiles and, along its near edge, the cliff
// beneath it); structures stand on their tiles and rise straight up, by
// BuildingTop at most. Draw the surface first and the structures after it
// from the far rows to the near ones, so that nearer ones cover farther
// ones.
//
// Ships, goods, glyphs, rock badges, the star and explosions are seen from
// straight above.
//
// # Look
//
// Everything is lit from the upper left. A structure shows three kinds of
// face: tops take a material's light tone, walls towards the lower left its
// plain tone, walls towards the lower right its dark one; shadows fall to
// the right. Materials have three flat tones; the owner's accent colour
// marks a roof, a stripe or a lamp on every structure and hull.
//
// # Cost
//
// The static part of every picture is rendered once into a sprite and drawn
// with a single image call afterwards: a structure's body per kind, accent,
// state and pixel size; a ship per kind, accent, size and heading step; a
// good or glyph per size; a rock's terrain per zoom. Only what moves — a
// turning dish, a blinking beacon, a glow — is drawn live, from a handful of
// stamped marks per structure. Pixel sizes are quantised, and the caches are
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

// Building draws a building of the given kind standing on the tile whose
// ground centre is (cx, cy) and whose corner radius, before foreshortening,
// is size. Its footprint stays inside the tile; it rises above (cx, cy) by
// BuildingTop(kind, size), and lights and smoke may reach a little further.
//
// The picture is placed on whole pixels and its size is rounded to the
// nearest cached step (a whole pixel of radius up to 32 pixels, at most
// about 3 % beyond), which keeps it sharp while the view pans and zooms.
func Building(cv *gfx.Canvas, kind model.BuildingKind, cx, cy, size float32, look BuildingLook) {
	drawStruct(cv, kind, cx, cy, size, look)
}

// BuildingIcon draws a static picture of a building kind inside r, for build
// palettes and the codex: the finished building, lit but frozen, seen at the
// same angle as on a rock and standing on a small slab of bare ground so
// that its dark parts and its shadow read on a dark panel as they do on a
// rock. Tile and building together are fitted into r, so a tall kind is
// drawn a little smaller than a flat one.
func BuildingIcon(cv *gfx.Canvas, kind model.BuildingKind, r gfx.Rect, accent gfx.Color) {
	drawStructIcon(cv, kind, r, accent)
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
// rock (cliff, terrain, shading) and redraws it only when the zoom changes.
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

// Draw paints the surface of rock with the ground centre of its centre hex
// at center and tiles of corner radius hexSize (foreshortened by Tilt, as
// HexPos places them): the rock with its cliff and terrain, a veil over the
// tiles viewer has not surveyed, the deposits viewer knows about (more
// prominent when showDeposits is set) and rubble. The cliff hangs below the
// nearest tiles by a few tile sizes, up to about six under the middle of a
// large rock. Buildings are drawn separately by the caller, after the
// surface.
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
