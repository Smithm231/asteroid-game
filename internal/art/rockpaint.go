package art

import (
	"math"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// The rock in the angled view.
//
// The ground is foreshortened by Tilt: tile h is centred where HexPos puts
// it, and its outline is the one TileStroke draws. Seen from above and in
// front, a rock is more than its top: every tile is the head of a six-sided
// column of rock hanging down from the plateau, and the two near faces of a
// column show wherever no nearer column hides them. The columns are longest
// under the middle of the rock and short towards its left and right ends,
// so the body tapers like a keel; their faces are banded with strata that
// darken with depth, the left-hand faces catching the light and the
// right-hand ones in shade. The far edge of the plateau has only a thin
// rim.

// surfaceReach is the most room, in hex units, a rock's picture takes
// beyond its tiles in any direction: the cliff, the points of its columns
// and the haze beneath.
const surfaceReach = 9

// glowRoom is the room, in hex units, left under the lowest column for the
// glow beneath the rock.
const glowRoom = 1.5

// rockShape is what the angled painter works out about a rock before it
// paints: the depth of every column, and the extent of the picture.
type rockShape struct {
	depth                  []float32 // per tile index, in hex units; 0 for Void
	tip                    []float32 // how far the point of each column hangs below its foot
	minX, maxX, minY, maxY float32   // bounds on screen, in hex units from the centre hex
	cx, cy, hw             float32   // middle of the plateau on screen and its half width
	keel                   float32   // the deepest column
	foot                   float32   // the lowest point of any column on screen
}

// shapeOf computes the columns and bounds of a rock.
func shapeOf(rock *model.Rock) rockShape {
	lay := rockShape{depth: make([]float32, len(rock.Tiles)), tip: make([]float32, len(rock.Tiles))}
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	n := 0
	for i := range rock.Tiles {
		if rock.Tiles[i].Terrain == model.TerrainVoid {
			continue
		}
		if h, ok := rock.HexAt(i); ok {
			x, y := hexCentre(h)
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
			n++
		}
	}
	if n == 0 {
		return lay
	}
	cx := (minX + maxX) / 2
	hw := (maxX-minX)/2 + 1
	seed := rock.ShapeSeed ^ 0xD33B
	dmax := min(max(0.36*hw, 1.5), 3.3)
	bottom := maxY * Tilt
	for i := range rock.Tiles {
		if rock.Tiles[i].Terrain == model.TerrainVoid {
			continue
		}
		h, ok := rock.HexAt(i)
		if !ok {
			continue
		}
		x, y := hexCentre(h)
		nx := (x - cx) / hw
		g := float32(math.Pow(float64(max(0, 1-nx*nx)), 0.7))
		// A slow swell along the rock and some raggedness from column to
		// column.
		swell := 0.78 + 0.44*valueNoise(seed, float64(x)*0.21, 3.1)
		d := 0.5 + dmax*g*swell
		d += 0.6 * (valueNoise(seed^7, float64(x)*0.8, float64(y)*0.8) - 0.5)
		d += 0.24 * unit(hash2(seed, h.Q, h.R))
		d = max(d, 0.4)
		lay.depth[i] = d
		lay.tip[i] = 0.2 + (0.3+0.55*unit(hash2(seed^0x7A9E, h.Q, h.R)))*min(1, d/1.2)
		lay.keel = max(lay.keel, d)
		bottom = max(bottom, (y+1)*Tilt+d, y*Tilt+d+lay.tip[i])
	}
	lay.cx, lay.cy, lay.hw = cx, (minY+maxY)/2*Tilt, hw
	lay.foot = bottom
	const margin = 1.3
	lay.minX, lay.maxX = minX-sqrt3/2-margin, maxX+sqrt3/2+margin
	lay.minY = (minY-1)*Tilt - 1.2 - margin // boulders stand on the far tiles
	lay.maxY = bottom + margin + glowRoom
	return lay
}

// render is render for the angled view.
func (s *Surface) render(key surfKey, rock *model.Rock, hexSize, sc float32) *surfEntry {
	lay := shapeOf(rock)
	rs := sc
	if side := max(lay.maxX-lay.minX, lay.maxY-lay.minY) * hexSize * sc; side > surfaceMaxPx {
		rs = sc * surfaceMaxPx / side
	}
	hexPx := hexSize * rs
	oxPx := float32(math.Ceil(float64(-lay.minX * hexPx)))
	oyPx := float32(math.Ceil(float64(-lay.minY * hexPx)))
	wPx := oxPx + float32(math.Ceil(float64(lay.maxX*hexPx)))
	hPx := oyPx + float32(math.Ceil(float64(lay.maxY*hexPx)))
	wPx, hPx = max(wPx, 1), max(hPx, 1)

	s.makeRoom(int(wPx) * int(hPx))
	e := &surfEntry{
		key: key, hex: hexSize, rscale: rs,
		ox: oxPx / rs, oy: oyPx / rs, w: wPx / rs, h: hPx / rs,
		veils: map[model.FactionID]*surfVeil{}, palette: paletteFor(rock.Class),
	}
	e.img = gfx.Render(e.w, e.h, rs, func(c *gfx.Canvas) {
		paintRock(c, e.ox, e.oy, hexSize, rock, e.palette, lay)
	})
	s.entries = append(s.entries, e)
	return e
}

// strata are the depths, in hex units below the plateau, at which the
// bands of the cliff change; strataTone is how far each band has sunk
// towards the dark of the keel.
var (
	strata     = [...]float32{0, 0.24, 0.6, 1.08, 1.68, 2.4, 3.3, 99}
	strataTone = [...]float32{0, 0.14, 0.3, 0.46, 0.6, 0.72, 0.82}
)

// tileGround returns the corners of tile h on the ground, grown by g about
// its centre, as x, y pairs in gfx.HexPoints order.
func tileGround(x, y, g float32) [12]float32 {
	const w = sqrt3 / 2
	k := 1 + g
	return [12]float32{x, y - k, x + w*k, y - k/2, x + w*k, y + k/2, x, y + k, x - w*k, y + k/2, x - w*k, y - k/2}
}

// paintRock draws the static layer of a rock in the angled view on c,
// with the centre of hex (0, 0) at (ox, oy) and tiles of radius hexSize.
func paintRock(c *gfx.Canvas, ox, oy, hexSize float32, rock *model.Rock, pal rockPalette, lay rockShape) {
	loops := blobOutline(rock)
	if len(loops) == 0 {
		return
	}
	seed := rock.ShapeSeed
	b := bpen{pen: newPen(c, ox, oy, hexSize)}
	b.unit = 1 / b.px
	hexPx := b.unit
	g := b.pen // the pen for flat things on the ground
	g.ky = Tilt

	// A haze of faint light behind and beneath the rock, so that its dark
	// underside stands off from the dark of space.
	{
		x := ox + lay.cx*hexSize
		y := oy + (lay.cy+lay.keel*0.5)*hexSize
		rx := min(lay.hw+2.4, lay.cx-lay.minX, lay.maxX-lay.cx) * hexSize
		ry := min(oy+lay.maxY*hexSize-y, y-oy-lay.minY*hexSize)
		haze := gfx.Lerp(gfx.RGB(0x34406C), pal.base, 0.18)
		c.Blob(x, y, rx, ry, alpha(haze, 0.5))
		// The glow beneath: strongest just under the feet of the columns,
		// which stand out against it.
		gy := oy + (lay.foot-0.5)*hexSize
		gry := oy + lay.maxY*hexSize - gy
		glow := gfx.Lerp(gfx.RGB(0x43568F), pal.base, 0.3)
		c.Blob(x, gy, rx*0.8, gry, alpha(glow, 0.42))
		c.Blob(x, gy, rx*0.5, gry*0.7, alpha(glow, 0.22))
	}

	// The columns, far rows first. Tiles are stored row by row, so their
	// index order is back to front.
	leftTop := tone(pal.base, -0.08)
	rightTop := tone(pal.base, -0.46)
	deep := gfx.Lerp(tone(pal.base, -0.78), gfx.RGB(0x0A0C14), 0.45)
	seam := tone(pal.alt, -0.3)
	edgeC := alpha(gfx.Lerp(pal.outline, gfx.RGB(0x06070B), 0.3), 0.55)
	hair := max(0.03, 1*b.px)
	depthAt := func(h model.Hex) float32 {
		i := rock.Index(h)
		if i < 0 {
			return 0
		}
		return lay.depth[i]
	}
	// The strata dip and rise gently along the rock. The offset depends on
	// x alone, so a band runs on unbroken from face to face.
	ph1, ph2 := unit(hash2(seed, 71, 3))*tau, unit(hash2(seed, 72, 5))*tau
	dip := func(x float32) float32 {
		return 0.1*sinf(x*0.58+ph1) + 0.045*sinf(x*1.63+ph2)
	}
	for i := range rock.Tiles {
		d := lay.depth[i]
		if d <= 0 {
			continue
		}
		h, _ := rock.HexAt(i)
		x, y := hexCentre(h)
		{
			// The foot of the column is broken off to a point; its two
			// near faces show below the column wherever nothing nearer
			// hides them.
			k := 0
			for k+2 < len(strata) && strata[k+1] < d {
				k++
			}
			t := min(1, strataTone[k]+0.1)
			tip := d + lay.tip[i]
			b.face(gfx.Lerp(leftTop, deep, t), x-sqrt3/2, y+0.5, -d, x, y+1, -d, x, y, -tip)
			b.face(gfx.Lerp(rightTop, deep, min(1, t+0.08)), x, y+1, -d, x+sqrt3/2, y+0.5, -d, x, y, -tip)
			if hexPx >= 9 {
				b.edge(edgeC, hair, 0, x, y+1, -d, x, y, -tip)
			}
		}
		for side := 0; side < 2; side++ {
			// The left face hangs from the lower left edge of the tile,
			// the right face from its lower right edge.
			ax, ay, bx, by := x-sqrt3/2, y+0.5, x, y+1
			dir, top := 4, leftTop
			if side == 1 {
				ax, ay, bx, by = x, y+1, x+sqrt3/2, y+0.5
				dir, top = 5, rightTop
			}
			d0 := depthAt(h.Neighbor(dir))
			if d <= d0+0.03 {
				continue
			}
			fr := newRnd(hash2(seed^0xFACE, h.Q*2+side, h.R))
			jit := (fr.f() - 0.5) * 0.09
			da, db := dip(ax), dip(bx)
			lap := 0.6 * b.px // hides the seam between bands
			for k := 0; k+1 < len(strata); k++ {
				fa, fb := d0, d0
				if k > 0 {
					fa, fb = min(max(strata[k]+da, d0), d), min(max(strata[k]+db, d0), d)
				}
				ta, tb := d, d
				if k+2 < len(strata) {
					ta, tb = min(max(strata[k+1]+da, d0), d), min(max(strata[k+1]+db, d0), d)
				}
				if ta <= fa && tb <= fb {
					continue
				}
				t := strataTone[k]
				col := gfx.Lerp(tone(top, jit), deep, t)
				switch k {
				case 1, 4:
					// A seam of the rock's other colour.
					col = gfx.Lerp(col, gfx.Lerp(seam, deep, t), 0.5)
				case 3:
					col = tone(col, 0.05)
				}
				la, lb := min(lap, d-ta), min(lap, d-tb)
				b.face(col, ax, ay, -fa, bx, by, -fb, bx, by, -tb-lb, ax, ay, -ta-la)
			}
			if hexPx >= 14 && d-d0 > 0.5 {
				// Cracks along the bedding, and now and then a chip of
				// lighter stone.
				for n := fr.n(3); n > 0; n-- {
					u0 := fr.in(0, 0.6)
					u1 := u0 + fr.in(0.25, 0.4)
					dd := d0 + (d-d0)*fr.in(0.12, 0.85)
					x0, y0, x1, y1 := lerpf(ax, bx, u0), lerpf(ay, by, u0), lerpf(ax, bx, u1), lerpf(ay, by, u1)
					g0, g1 := dd+lerpf(da, db, u0), dd+lerpf(da, db, u1)
					b.edge(alpha(deep, 0.5), hair, 0, x0, y0, -g0, x1, y1, -g1)
				}
			}
			if hexPx >= 9 {
				b.edge(edgeC, hair, 0, ax, ay, -d0, ax, ay, -d)
				b.edge(edgeC, hair, 0, bx, by, -d0, bx, by, -d)
			}
			if d0 == 0 {
				// The lip of the plateau catches the light where it faces it.
				a := float32(0.8)
				if side == 1 {
					a = 0.22
				}
				b.edge(alpha(pal.rimLit, a), max(0.05, 1.2*b.px), 0, ax, ay, 0, bx, by, 0)
			}
		}
	}

	// The plateau: one fill under everything, then the tiles.
	var path gfx.Path
	for _, loop := range loops {
		path.MoveTo(loop[0].x, loop[0].y*Tilt)
		for _, v := range loop[1:] {
			path.LineTo(v.x, v.y*Tilt)
		}
		path.Close()
	}
	b.pen.path(&path, gfx.PathStyle{Fill: pal.base})

	type tileInfo struct {
		h      model.Hex
		x, y   float32
		ground gfx.Color
		t      *model.Tile
	}
	tiles := make([]tileInfo, 0, len(rock.Tiles))
	var sx, sy float32
	for i := range rock.Tiles {
		if rock.Tiles[i].Terrain == model.TerrainVoid {
			continue
		}
		if h, ok := rock.HexAt(i); ok {
			x, y := hexCentre(h)
			tiles = append(tiles, tileInfo{h: h, x: x, y: y, t: &rock.Tiles[i]})
			sx, sy = sx+x, sy+y
		}
	}
	mx, my := sx/float32(len(tiles)), sy/float32(len(tiles))
	span := float32(rock.Radius)*sqrt3 + 1
	grow := 1.3 * b.px // hides the seams between neighbouring tiles
	for i := range tiles {
		ti := &tiles[i]
		h := ti.h
		// Value: slow noise, a little per-tile grain, the lie of the land
		// towards the light, and a rim that is lit towards the upper left
		// and shaded towards the lower right.
		v := (fbm(seed, float64(ti.x)*0.21, float64(ti.y)*0.21) - 0.5) * 0.36
		v += (unit(hash2(seed^0x51, h.Q, h.R)) - 0.5) * 0.085
		v += 0.05 * ((ti.x-mx)*lightX + (ti.y-my)*lightY) / span
		for d := 0; d < 6; d++ {
			n := h.Neighbor(d)
			if !rock.InBlob(n) {
				v += 0.05 * facing(dirVec[d][0], dirVec[d][1])
			} else if !rock.InBlob(n.Neighbor(d)) {
				v += 0.02 * facing(dirVec[d][0], dirVec[d][1])
			}
		}
		switch ti.t.Terrain {
		case model.TerrainCrag:
			v -= 0.10
		case model.TerrainFissure:
			v -= 0.07
		}
		hue := fbm(seed^0xABCD, float64(ti.x)*0.13+9, float64(ti.y)*0.13-4)
		ti.ground = tone(gfx.Lerp(pal.base, pal.alt, clamp01((hue-0.34)*0.9)), v)
		xy := tileGround(ti.x, ti.y, grow)
		g.poly(ti.ground, xy[:]...)
	}

	// Mottling that ignores the tile boundaries, kept to the interior.
	for i := range tiles {
		ti := &tiles[i]
		r := newRnd(hash2(seed^0xB107, ti.h.Q, ti.h.R))
		if r.f() > 0.55 {
			continue
		}
		inner := true
		for d := 0; d < 6 && inner; d++ {
			inner = rock.InBlob(ti.h.Neighbor(d))
		}
		if !inner {
			continue
		}
		a, d := r.in(0, tau), r.in(0, 0.45)
		rad := r.in(0.6, 0.84)
		col := alpha(tone(pal.base, 0.2), 0.34)
		if r.f() < 0.55 {
			col = alpha(tone(pal.base, -0.3), 0.38)
		}
		g.blob(ti.x+d*cosf(a), ti.y+d*sinf(a), rad, rad*r.in(0.7, 1), col)
	}

	// A faint grid: each shared edge once.
	gridW := max(0.025, 1*b.px)
	gridC := alpha(pal.grid, 0.26)
	for i := range tiles {
		ti := &tiles[i]
		xy := tileGround(ti.x, ti.y, 0)
		for _, d := range [3]int{0, 5, 4} {
			if !rock.InBlob(ti.h.Neighbor(d)) {
				continue
			}
			a, e := dirEdge[d][0], dirEdge[d][1]
			g.line(xy[2*a], xy[2*a+1], xy[2*e], xy[2*e+1], gridW, 0, gridC)
		}
	}

	// The far edge of the plateau: a thin rim, light where it faces the
	// light and dark where it faces away. The near edges end in the cliff.
	rimW := max(0.06, 1.3*b.px)
	for _, loop := range loops {
		n := len(loop)
		for i := 0; i < n; i++ {
			p, q := loop[i], loop[(i+1)%n]
			ex, ey := q.x-p.x, q.y-p.y
			l := float32(math.Hypot(float64(ex), float64(ey)))
			nx, ny := ey/l, -ex/l
			if ny > 0.3 {
				continue
			}
			in := rimW / 2
			x0, y0, x1, y1 := p.x-nx*in, p.y-ny*in, q.x-nx*in, q.y-ny*in
			if f := facing(nx, ny); f > 0.2 {
				g.line(x0, y0, x1, y1, rimW, 0, alpha(pal.rimLit, 0.4+0.45*f))
			} else {
				g.line(x0, y0, x1, y1, rimW, 0, alpha(pal.rimDark, 0.55))
			}
		}
	}

	// Ground detail on open regolith: pebbles and the odd small crater.
	for i := range tiles {
		ti := &tiles[i]
		if ti.t.Terrain != model.TerrainRegolith {
			continue
		}
		r := newRnd(hash2(seed^0x77, ti.h.Q, ti.h.R))
		if hexPx >= 12 {
			for k, n := 0, 2+r.n(3); k < n; k++ {
				a, d := r.in(0, tau), r.in(0.1, 0.72)
				rad := max(r.in(0.025, 0.06), 0.6*b.px)
				col := tone(ti.ground, -0.14)
				if r.f() < 0.4 {
					col = tone(ti.ground, 0.12)
				}
				g.circle(ti.x+d*cosf(a), ti.y+d*sinf(a), rad, alpha(col, 0.8))
			}
		}
		if r.f() < 0.13 {
			x, y, rad := ti.x+r.in(-0.2, 0.2), ti.y+r.in(-0.2, 0.2), r.in(0.2, 0.36)
			// A small bowl: its near wall in shadow, its far wall catching
			// the light.
			b.disc(tone(ti.ground, -0.1), x, y, 0, rad)
			w := max(rad*0.2, 1*b.px)
			b.arcG(alpha(tone(ti.ground, -0.3), 0.85), w, 0, x, y, 0, rad-w/2, litAngle-1.25, litAngle+1.25)
			b.arcG(alpha(tone(ti.ground, 0.2), 0.85), w, 0, x, y, 0, rad-w/2, shadeAngle-1.1, shadeAngle+1.1)
		}
	}

	// Blocked ground: fissures, then the boulders of the crags, far ones
	// first.
	isT := func(h model.Hex, t model.Terrain) bool {
		tl := rock.TileAt(h)
		return tl != nil && tl.Terrain == t
	}
	for i := range tiles {
		if tiles[i].t.Terrain == model.TerrainFissure {
			paintFissure(&g, seed, tiles[i].h, tiles[i].x, tiles[i].y, pal, func(h model.Hex) bool { return isT(h, model.TerrainFissure) })
		}
	}
	var rocks []boulder
	for i := range tiles {
		ti := &tiles[i]
		if ti.t.Terrain != model.TerrainCrag {
			continue
		}
		r := newRnd(hash2(seed^0xC4A6, ti.h.Q, ti.h.R))
		rocks = append(rocks, boulder{ti.x + r.in(-0.08, 0.08), ti.y + r.in(-0.08, 0.08), r.in(0.6, 0.74), r.next()})
		for _, d := range [3]int{0, 5, 4} {
			if !isT(ti.h.Neighbor(d), model.TerrainCrag) {
				continue
			}
			nx, ny := hexCentre(ti.h.Neighbor(d))
			rocks = append(rocks, boulder{(ti.x+nx)/2 + r.in(-0.12, 0.12), (ti.y+ny)/2 + r.in(-0.12, 0.12), r.in(0.4, 0.52), r.next()})
		}
	}
	for i := 1; i < len(rocks); i++ {
		for j := i; j > 0 && rocks[j].y < rocks[j-1].y; j-- {
			rocks[j], rocks[j-1] = rocks[j-1], rocks[j]
		}
	}
	for _, bl := range rocks {
		paintBoulder(&b, bl, pal)
	}
}

// paintBoulder draws an outcrop standing on the ground: a ring of
// facets rising from an uneven base to a smaller cap, and the cap itself
// rising to a summit; light on the left, dark on the right, a shadow thrown
// to the right.
func paintBoulder(b *bpen, bl boulder, pal rockPalette) {
	r := newRnd(bl.seed)
	const n = 7
	var base, top [2 * n]float32
	var topH [n]float32
	a0 := r.in(0, tau)
	height := bl.r * r.in(0.62, 0.82)
	cxo, cyo := bl.x+bl.r*r.in(-0.14, 0.04), bl.y+bl.r*r.in(-0.16, 0)
	for i := 0; i < n; i++ {
		a := a0 + tau*float32(i)/n + r.in(-0.18, 0.18)
		rad := bl.r * r.in(0.8, 1.04)
		base[2*i], base[2*i+1] = bl.x+rad*cosf(a), bl.y+rad*sinf(a)*0.94
		k := r.in(0.46, 0.62)
		top[2*i], top[2*i+1] = cxo+(base[2*i]-bl.x)*k, cyo+(base[2*i+1]-bl.y)*k
		topH[i] = height * r.in(0.72, 0.94)
	}
	// The shadow on the ground.
	var sh [3 * n]float32
	for i := 0; i < n; i++ {
		sh[3*i], sh[3*i+1], sh[3*i+2] = base[2*i]+0.34*bl.r, base[2*i+1]+0.14*bl.r, 0
	}
	b.face(gfx.Color{R: 4, G: 5, B: 9, A: 0x70}, sh[:]...)

	sgn := polySign(base[:])
	tint := func(nx, ny float32, roof bool) gfx.Color {
		// Faces turned to the left are lit, those turned to the right in
		// shade; the facets of the cap, which look up, are lighter all
		// round.
		d := -0.85*nx - 0.2*ny + r.in(-0.1, 0.1)
		if roof {
			d += 0.35
		}
		if d >= 0 {
			return gfx.Lerp(pal.cragMid, pal.cragLit, min(1, d*1.1))
		}
		return gfx.Lerp(pal.cragMid, pal.cragDark, min(1, -d*1.2))
	}
	var all [2*n + 1]gfx.Pt
	for i := 0; i < n; i++ {
		all[i] = b.at(base[2*i], base[2*i+1], 0)
		all[n+i] = b.at(top[2*i], top[2*i+1], topH[i])
	}
	summit := b.at(cxo, cyo, height)
	all[2*n] = summit
	// The flanks, far ones first.
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			nx, ny := sgn*(base[2*j+1]-base[2*i+1]), -sgn*(base[2*j]-base[2*i])
			l := float32(math.Hypot(float64(nx), float64(ny)))
			if l == 0 || (ny > 0) != (pass == 1) {
				continue
			}
			b.cv.FillPoly([]gfx.Pt{all[i], all[j], all[n+j], all[n+i]}, tint(nx/l, ny/l, false))
		}
	}
	// The cap.
	for pass := 0; pass < 2; pass++ {
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			nx, ny := sgn*(top[2*j+1]-top[2*i+1]), -sgn*(top[2*j]-top[2*i])
			l := float32(math.Hypot(float64(nx), float64(ny)))
			if l == 0 || (ny > 0) != (pass == 1) {
				continue
			}
			b.cv.FillPoly([]gfx.Pt{all[n+i], all[n+j], summit}, tint(nx/l, ny/l, true))
		}
	}
	if b.unit >= 11 {
		hl := max(0.02, 0.7*b.px) * b.k
		for i := 0; i < n; i++ {
			b.cv.Line(all[i].X, all[i].Y, all[n+i].X, all[n+i].Y, hl, alpha(pal.outline, 0.35))
		}
	}
	hullBuf = hullBuf[:0]
	b.outlineThin(all[:], alpha(pal.outline, 0.85))
}

// outlineThin strokes the silhouette of the given canvas points in colour
// c with a fine line.
func (b *bpen) outlineThin(pts []gfx.Pt, c gfx.Color) {
	keep := b.tone
	b.tone = func(gfx.Color) gfx.Color { return c }
	b.outlineOf(pts)
	b.tone = keep
}

// paintVeil paints the veil over the tiles the viewer has not surveyed,
// in the angled view: each such tile filled dark and hatched.
func paintVeil(c *gfx.Canvas, ox, oy, hexSize float32, rock *model.Rock, viewer model.FactionID) {
	g := newPen(c, ox, oy, hexSize)
	g.ky = Tilt
	hexPx := 1 / g.px
	grow := 1.3 * g.px
	// Hatching runs along lines x + y = const on screen, a fixed number of
	// pixels apart so that it lines up from tile to tile.
	gap := max(0.34, 5*g.px)
	lw := max(1.0, float32(math.Round(float64(hexPx*0.035)))) * g.px
	flat := g
	flat.ky = 0
	for i := range rock.Tiles {
		if rock.Tiles[i].Terrain == model.TerrainVoid {
			continue
		}
		h, ok := rock.HexAt(i)
		if !ok || rock.KnownTo(h, viewer) {
			continue
		}
		x, y := hexCentre(h)
		xy := tileGround(x, y, grow)
		g.poly(veilFill, xy[:]...)
		// The same outline on screen, for clipping the hatch lines.
		var corners [6]gfx.Pt
		for k := range corners {
			corners[k] = gfx.Pt{X: xy[2*k], Y: xy[2*k+1] * Tilt}
		}
		sy := y * Tilt
		k0 := int(math.Ceil(float64((x + sy - 1.5) / gap)))
		k1 := int(math.Floor(float64((x + sy + 1.5) / gap)))
		for k := k0; k <= k1; k++ {
			if x0, y0, x1, y1, ok := clipDiagonal(corners[:], float32(k)*gap); ok {
				flat.line(x0, y0, x1, y1, lw, 0, veilHatch)
			}
		}
	}
}
