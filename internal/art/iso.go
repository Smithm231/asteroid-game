package art

import (
	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// The surface view looks at a rock from an angle: the ground plane is
// foreshortened vertically by Tilt, and structures rise straight up from
// their tile. Everything that places or picks things on a rock's surface goes
// through the helpers in this file, so the projection lives in one place.

// Tilt is the vertical foreshortening of the ground plane in the surface
// view: 1 looks straight down, 0.5 is the classic two-to-one isometric slope.
const Tilt = 0.5

// HexPos returns where the centre of tile h is drawn, on the ground, for a
// rock whose centre hex is drawn at center with tiles of corner radius
// hexSize.
func HexPos(center gfx.Pt, h model.Hex, hexSize float32) gfx.Pt {
	p := model.HexToPixel(h, float64(hexSize))
	return gfx.P(center.X+float32(p.X), center.Y+float32(p.Y)*Tilt)
}

// PickHex returns the tile whose ground lies under the screen point (x, y).
func PickHex(center gfx.Pt, x, y, hexSize float32) model.Hex {
	return model.PixelToHex(model.Vec2{X: float64(x - center.X), Y: float64(y-center.Y) / Tilt}, float64(hexSize))
}

// tilePoints returns the outline of a tile of corner radius size centred on
// the ground point (cx, cy), scaled by k about its centre.
func tilePoints(cx, cy, size, k float32) []gfx.Pt {
	c := model.HexCorners(model.Vec2{}, float64(size*k))
	pts := make([]gfx.Pt, 6)
	for i, p := range c {
		pts[i] = gfx.P(cx+float32(p.X), cy+float32(p.Y)*Tilt)
	}
	return pts
}

// TileFill fills the ground outline of a tile (slightly inset) with col: the
// way to mark ranges, hover and placement on the surface.
func TileFill(cv *gfx.Canvas, cx, cy, size float32, col gfx.Color) {
	cv.FillPoly(tilePoints(cx, cy, size, 0.96), col)
}

// TileStroke draws the ground outline of a tile.
func TileStroke(cv *gfx.Canvas, cx, cy, size, width float32, col gfx.Color) {
	cv.StrokePoly(tilePoints(cx, cy, size, 0.97), width, col)
}

// BuildingTop returns how far above its ground point, in points, the top of a
// structure of the given kind reaches when drawn at hex size size. The surface
// view uses it to place status marks above a structure.
func BuildingTop(kind model.BuildingKind, size float32) float32 {
	return buildingTop(kind, size)
}
