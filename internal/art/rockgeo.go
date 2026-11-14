package art

import (
	"math"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// This file holds what the surface renderer and the rock badge share: the
// outline of a rock's blob and its colours.
//
// Geometry is in hex units: the centre of hex (q, r) is at
// (√3·(q + r/2), 1.5·r) and every hex has corner radius 1.

const sqrt3 = 1.7320508

// hexCentre returns the centre of h in hex units.
func hexCentre(h model.Hex) (x, y float32) {
	return sqrt3 * (float32(h.Q) + float32(h.R)/2), 1.5 * float32(h.R)
}

// Corner c of a hex (clockwise from the top, like gfx.HexPoints) sits on an
// integer lattice: x = (√3/2)·X, y = Y/2, with the offsets below added to
// (2q + r, 3r).
var (
	cornerDX = [6]int{0, 1, 1, 0, -1, -1}
	cornerDY = [6]int{-2, -1, 1, 2, 1, -1}
)

// dirEdge gives, for each of model.HexDirections, the two corners of the
// edge shared with the neighbour in that direction, in clockwise order.
var dirEdge = [6][2]int{{1, 2}, {0, 1}, {5, 0}, {4, 5}, {3, 4}, {2, 3}}

// dirVec is the unit vector towards the neighbour in each direction.
var dirVec = func() (v [6][2]float32) {
	for d, o := range model.HexDirections() {
		x, y := hexCentre(o)
		l := float32(math.Hypot(float64(x), float64(y)))
		v[d] = [2]float32{x / l, y / l}
	}
	return v
}()

// outlineVert is one corner of a blob outline.
type outlineVert struct {
	x, y   float32 // position in hex units
	nx, ny float32 // unit vector pointing out of the rock (bisector)
	convex bool    // the rock bulges here (it has a notch otherwise)
	key    uint64  // stable id of the lattice point, for seeding
}

// blobOutline traces the boundary of the rock's non-Void tiles. It returns
// one closed loop per connected stretch of boundary: the outer edge runs
// clockwise on screen, the rims of holes anticlockwise.
func blobOutline(rock *model.Rock) [][]outlineVert {
	type vkey struct{ x, y int }
	type edge struct {
		from, to vkey
		used     bool
	}
	var edges []edge
	start := map[vkey]int{}
	for i := range rock.Tiles {
		if rock.Tiles[i].Terrain == model.TerrainVoid {
			continue
		}
		h, ok := rock.HexAt(i)
		if !ok {
			continue
		}
		bx, by := 2*h.Q+h.R, 3*h.R
		for d := 0; d < 6; d++ {
			if rock.InBlob(h.Neighbor(d)) {
				continue
			}
			a, b := dirEdge[d][0], dirEdge[d][1]
			e := edge{
				from: vkey{bx + cornerDX[a], by + cornerDY[a]},
				to:   vkey{bx + cornerDX[b], by + cornerDY[b]},
			}
			start[e.from] = len(edges)
			edges = append(edges, e)
		}
	}
	var loops [][]outlineVert
	for i := range edges {
		if edges[i].used {
			continue
		}
		var keys []vkey
		for j := i; !edges[j].used; {
			edges[j].used = true
			keys = append(keys, edges[j].from)
			next, ok := start[edges[j].to]
			if !ok {
				break
			}
			j = next
		}
		n := len(keys)
		if n < 3 {
			continue
		}
		loop := make([]outlineVert, n)
		for k, v := range keys {
			loop[k].x = float32(v.x) * sqrt3 / 2
			loop[k].y = float32(v.y) / 2
			loop[k].key = uint64(uint32(int32(v.x)))<<32 | uint64(uint32(int32(v.y)))
		}
		for k := range loop {
			p, c, q := loop[(k+n-1)%n], loop[k], loop[(k+1)%n]
			ax, ay := c.x-p.x, c.y-p.y
			bx, by := q.x-c.x, q.y-c.y
			// Outward normal of a clockwise edge (dx, dy) is (dy, -dx).
			nx, ny := ay+by, -ax-bx
			l := float32(math.Hypot(float64(nx), float64(ny)))
			if l > 0 {
				nx, ny = nx/l, ny/l
			}
			loop[k].nx, loop[k].ny = nx, ny
			loop[k].convex = ax*by-ay*bx > 0
		}
		loops = append(loops, loop)
	}
	return loops
}

// rockPalette is the set of colours a rock of some class is painted in.
type rockPalette struct {
	base, alt                     gfx.Color // ground and its second hue
	cliffLit, cliffMid, cliffDark gfx.Color // the skirt below the plateau
	rimLit, rimDark               gfx.Color // lines along the plateau edge
	outline, grid                 gfx.Color
	cragLit, cragMid, cragDark    gfx.Color
	crack, crackLip               gfx.Color
}

// rockAlt is the second ground hue per class, mixed in by a slow noise.
var rockAlt = [model.NumRockClasses]uint32{
	model.ClassFerric: 0x7F5241,
	model.ClassStony:  0x6B665F,
	model.ClassFrost:  0xA3B9C8,
	model.ClassGlint:  0x5D6BA3,
}

func paletteFor(class model.RockClass) rockPalette {
	base := gfx.Default().RockColor(int(class))
	alt := base
	if class.Valid() {
		alt = gfx.RGB(rockAlt[class])
	}
	return rockPalette{
		base:      base,
		alt:       alt,
		cliffLit:  gfx.Lerp(gfx.Lighten(base, 0.26), gfx.RGB(0xE9E2D0), 0.12),
		cliffMid:  gfx.Darken(base, 0.36),
		cliffDark: gfx.Darken(base, 0.64),
		rimLit:    gfx.Lerp(gfx.Lighten(base, 0.45), gfx.RGB(0xFFF1D6), 0.25),
		rimDark:   gfx.Darken(base, 0.5),
		outline:   gfx.Lerp(gfx.Darken(base, 0.86), gfx.RGB(0x07080C), 0.5),
		grid:      gfx.Darken(base, 0.55),
		cragLit:   gfx.Lerp(gfx.Lighten(base, 0.2), gfx.RGB(0xE9E2D0), 0.1),
		cragMid:   gfx.Darken(base, 0.24),
		cragDark:  gfx.Lerp(gfx.Darken(base, 0.66), gfx.RGB(0x0B0D14), 0.25),
		crack:     gfx.Lerp(gfx.Darken(base, 0.88), gfx.RGB(0x05060A), 0.5),
		crackLip:  gfx.Lighten(base, 0.3),
	}
}

// tone returns c lightened by v > 0 or darkened by v < 0.
func tone(c gfx.Color, v float32) gfx.Color { return shade(c, v) }

// facing returns how squarely a surface with outward normal (nx, ny) faces
// the light: 1 towards it, -1 away.
func facing(nx, ny float32) float32 { return nx*lightX + ny*lightY }
