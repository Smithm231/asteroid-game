// Package art draws everything in Drifthold that is a picture rather than a
// widget: rock surfaces, buildings, ships, goods and interface glyphs. All of
// it is generated in code on a gfx.Canvas; nothing is loaded from disk.
//
// Coordinates and sizes are in points, like everything on a Canvas. Hexes are
// pointy-top: the tile at hex h of a rock whose centre hex is drawn at c lies
// at c + model.HexToPixel(h, hexSize).
package art

import (
	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// BuildingLook describes the state a building is drawn in.
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
	// Time is the animation clock in seconds.
	Time float32
}

// Building draws a building of the given kind centred on (cx, cy), sized to
// sit inside a hex tile whose corner radius is size.
func Building(cv *gfx.Canvas, kind model.BuildingKind, cx, cy, size float32, look BuildingLook) {
	th := gfx.Default()
	body := gfx.Lerp(th.PanelHi, look.Accent, 0.35)
	if look.Derelict {
		body = gfx.Darken(th.PanelHi, 0.3)
	}
	cv.FillHex(cx, cy, size*0.72, body)
	cv.StrokeHex(cx, cy, size*0.72, 1, look.Accent)
	name := kind.String()
	if len(name) > 2 {
		name = name[:2]
	}
	st := th.Small().Aligned(gfx.AlignCenter)
	cv.Text(name, cx, cy-st.Size*0.6, st)
}

// BuildingIcon draws a static picture of a building kind inside r, for build
// palettes and the codex.
func BuildingIcon(cv *gfx.Canvas, kind model.BuildingKind, r gfx.Rect, accent gfx.Color) {
	Building(cv, kind, r.MidX(), r.MidY(), min(r.W, r.H)*0.55, BuildingLook{Accent: accent, Progress: 1})
}

// Ship draws a ship of the given kind centred on (cx, cy), about size points
// long, pointing along heading (radians, 0 = right, clockwise on screen).
func Ship(cv *gfx.Canvas, kind model.ShipKind, cx, cy, size, heading float32, accent gfx.Color) {
	cv.FillCircle(cx, cy, size*0.4, accent)
}

// ShipIcon draws a static picture of a ship kind inside r.
func ShipIcon(cv *gfx.Canvas, kind model.ShipKind, r gfx.Rect, accent gfx.Color) {
	Ship(cv, kind, r.MidX(), r.MidY(), min(r.W, r.H)*0.8, -1.5708, accent)
}

// Good draws the symbol of a good inside r.
func Good(cv *gfx.Canvas, g model.Good, r gfx.Rect) {
	th := gfx.Default()
	cv.FillCircle(r.MidX(), r.MidY(), min(r.W, r.H)*0.4, th.GoodColor(int(g)))
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

// Icon draws an interface glyph inside r in the given colour.
func Icon(cv *gfx.Canvas, id IconID, r gfx.Rect, col gfx.Color) {
	cv.StrokeCircle(r.MidX(), r.MidY(), min(r.W, r.H)*0.35, 1.5, col)
}

// Surface draws rock surfaces. It caches the expensive static part of each
// rock (terrain, shading, outline) and redraws it only when the zoom changes.
type Surface struct{}

// NewSurface returns an empty surface renderer.
func NewSurface() *Surface { return &Surface{} }

// Draw paints the surface of rock with its centre hex at center and tiles of
// corner radius hexSize: the blob with its terrain, the deposits that viewer
// knows about (more prominent when showDeposits is set) and rubble. Buildings
// are drawn separately by the caller.
func (s *Surface) Draw(cv *gfx.Canvas, rock *model.Rock, center gfx.Pt, hexSize float32, viewer model.FactionID, showDeposits bool) {
	th := gfx.Default()
	base := th.RockColor(int(rock.Class))
	for i := range rock.Tiles {
		t := &rock.Tiles[i]
		if !t.Terrain.Solid() {
			continue
		}
		h, _ := rock.HexAt(i)
		p := model.HexToPixel(h, float64(hexSize))
		x, y := center.X+float32(p.X), center.Y+float32(p.Y)
		col := base
		if t.Terrain != model.TerrainRegolith {
			col = gfx.Darken(base, 0.45)
		}
		cv.FillHex(x, y, hexSize+0.5, col)
		if g, amt, ok := rock.VisibleDeposit(h, viewer); ok && amt > 0 {
			cv.FillCircle(x, y, hexSize*0.3, th.GoodColor(int(g)))
		}
	}
}

// Release frees the cached images.
func (s *Surface) Release() {}

// RockBadge draws a small silhouette of a rock, radius points across, for the
// belt view and for lists.
func RockBadge(cv *gfx.Canvas, rock *model.Rock, cx, cy, radius float32) {
	cv.FillCircle(cx, cy, radius, gfx.Default().RockColor(int(rock.Class)))
}

// Star draws Emberfall, the red dwarf at the centre of the belt.
func Star(cv *gfx.Canvas, cx, cy, r, time float32) {
	cv.Glow(cx, cy, r*3, gfx.RGB(0xE5533D))
	cv.FillCircle(cx, cy, r, gfx.RGB(0xF2A65A))
}

// Explosion draws one frame of a blast of radius r; phase runs from 0 (flash)
// to 1 (gone).
func Explosion(cv *gfx.Canvas, cx, cy, r, phase float32) {
	if phase < 0 || phase >= 1 {
		return
	}
	cv.Glow(cx, cy, r*(0.4+phase), gfx.Fade(gfx.RGB(0xF2A65A), 1-phase))
}
