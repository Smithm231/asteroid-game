// Package gfx is Drifthold's drawing layer: layout rectangles, colours, the
// theme, a point-based canvas of anti-aliased primitives, text, and the
// space backdrop. Everything is generated in code; nothing is loaded from
// disk.
//
// # Coordinates
//
// All geometry handed to this package is in points. A [Canvas] multiplies by
// its Scale (pixels per point) at draw time, snaps axis-aligned UI geometry
// to the pixel grid and instantiates text faces at size × Scale, so output is
// crisp at any UI scale. Code outside gfx never multiplies by the scale.
//
// # Drawing
//
// A Canvas wraps a destination image:
//
//	cv := gfx.NewCanvas(screen, in.Scale)
//	th := gfx.Default()
//	cv.Panel(r, th.Chamfer, gfx.CornersAll, th.Panel, th.Rule)
//	cv.Text("Bore Rig", r.X+th.Pad, r.Y+th.Pad, th.Head())
//
// Rectangles, chamfered panels, lines, polygons, circles, arcs and hexes are
// tessellated with a one-pixel feathered edge and drawn as plain triangles,
// which Ebitengine batches together with text glyphs; several thousand of
// them fit in a frame. Arbitrary outlines (curves, concave shapes, holes) go
// through [Path] and are rasterised by Ebitengine's vector package, which is
// slower per shape and best used for sprites that are rendered once with
// [Render] or a [SpriteCache].
//
// A Canvas and the package-level caches behind it are not safe for concurrent
// use: draw from the game's Draw (or Update) goroutine only.
package gfx
