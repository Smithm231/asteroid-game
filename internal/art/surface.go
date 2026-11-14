package art

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// A rock is painted as a plateau of hex tiles standing on an irregular skirt
// of faceted cliff: the skirt gives the blob its darker outline and its
// light and shaded sides, the plateau carries the terrain.
//
// Three layers make up a surface:
//
//   - the static layer (shadow, skirt, tiles, craters, crags, fissures) is
//     rendered into one image per rock and zoom and redrawn only when the
//     terrain changes;
//   - the veil over unsurveyed tiles is a second image of the same size,
//     re-rendered when the viewer learns something new;
//   - deposits and rubble change as the game runs and are drawn from small
//     cached sprites, one per tile.
//
// A frame therefore costs two image draws plus one sprite per deposit.

// A Surface keeps the static layers it rendered most recently: as many as
// fit in surfaceBudgetPx pixels (the newest always, however large), and
// never more than surfaceEntries. A close-up of a big rock takes several
// million pixels, a thumbnail a few thousand, so one Surface serves a
// colony view as well as a screen full of small rocks without painting
// anything twice. The veil of a layer is an image of the same size again.
const (
	surfaceEntries  = 32
	surfaceBudgetPx = 16 << 20
)

// surfaceMaxPx is the largest side of a static layer in pixels; closer zooms
// are stretched from it.
const surfaceMaxPx = 3072

// surfaceMargin is the room, in hex units, left around the tiles for the
// skirt and the shadow.
const surfaceMargin = 2.1

type surfKey struct {
	id     model.RockID
	seed   uint64
	terr   uint64 // hash of the terrain
	class  model.RockClass
	radius int
	hexQ   int32  // hex size in eighths of a pixel
	scale  uint32 // bits of the canvas scale
}

// sameRock reports whether two keys show the same rock, whatever the zoom.
func (k surfKey) sameRock(o surfKey) bool {
	k.hexQ, k.scale = o.hexQ, o.scale
	return k == o
}

type surfEntry struct {
	key        surfKey
	img        *ebiten.Image
	ox, oy     float32 // the centre of hex (0, 0) inside img, in points at hex size hex
	w, h       float32 // size of img in points at hex size hex
	hex        float32 // hex size in points the image was rendered for
	rscale     float32 // canvas scale the image was rendered at
	used       uint64
	veils      map[model.FactionID]*surfVeil
	palette    rockPalette
	renderedAt uint64
}

type surfVeil struct {
	img   *ebiten.Image
	known uint64 // hash of the viewer's knowledge the image shows
}

type surfPending struct {
	key  surfKey
	seen uint64
}

func (e *surfEntry) release() {
	if e.img != nil {
		e.img.Deallocate()
		e.img = nil
	}
	for f, v := range e.veils {
		v.img.Deallocate()
		delete(e.veils, f)
	}
}

// rockState hashes what the static layer and the veil depend on. allKnown is
// true when no tile is hidden from the viewer.
func rockState(rock *model.Rock, viewer model.FactionID) (terr, known uint64, allKnown bool) {
	terr, known = 0xCBF29CE484222325, 0x84222325CBF29CE4
	surveyed := rock.SurveyedBy.Has(viewer)
	allKnown = true
	for i := range rock.Tiles {
		t := &rock.Tiles[i]
		terr = (terr ^ uint64(t.Terrain)) * 0x100000001B3
		k := uint64(1)
		if t.Terrain != model.TerrainVoid && !surveyed && !t.Known.Has(viewer) {
			k = 0
			allKnown = false
		}
		known = (known ^ k) * 0x100000001B3
	}
	return terr, known, allKnown
}

func (s *Surface) draw(cv *gfx.Canvas, rock *model.Rock, center gfx.Pt, hexSize float32, viewer model.FactionID, showDeposits bool) {
	if s == nil || cv == nil || cv.Dst == nil || rock == nil || !(hexSize > 0) || len(rock.Tiles) == 0 {
		return
	}
	sc := canvasScale(cv)
	// Nothing to do for a rock that is entirely off screen.
	ext := (float32(2*rock.Radius+1) + surfaceMargin) * hexSize
	if !cv.Visible(gfx.R(center.X-ext, center.Y-ext, 2*ext, 2*ext)) {
		return
	}
	terr, known, allKnown := rockState(rock, viewer)
	key := surfKey{
		id: rock.ID, seed: rock.ShapeSeed, terr: terr, class: rock.Class, radius: rock.Radius,
		hexQ: int32(math.Round(float64(hexSize * sc * 8))), scale: math.Float32bits(sc),
	}
	s.seq++
	e := s.find(key)
	if e == nil {
		// While the zoom is changing every frame, stretching the layer we
		// already have is far cheaper than painting a new one; a size is
		// only rendered once it has been asked for twice running.
		if fb := s.nearest(key); fb != nil && !s.asked(key) {
			e = fb
		} else {
			e = s.render(key, rock, hexSize, sc)
		}
	}
	e.used = s.seq

	cx, cy := snapCentre(cv, center.X, center.Y)
	k := hexSize / e.hex
	dst := gfx.R(cx-e.ox*k, cy-e.oy*k, e.w*k, e.h*k)
	cv.DrawImage(e.img, dst, nil)

	if !allKnown {
		v := e.veils[viewer]
		if v == nil || v.known != known {
			v = s.renderVeil(e, v, rock, viewer, known)
		}
		cv.DrawImage(v.img, dst, &gfx.ImageOpts{Tint: gfx.Opacity(veilOpacity)})
	}
	drawSurfaceMarks(cv, rock, cx, cy, hexSize, viewer, showDeposits)
}

// find returns the entry with exactly this key.
func (s *Surface) find(key surfKey) *surfEntry {
	for _, e := range s.entries {
		if e.key == key {
			return e
		}
	}
	return nil
}

// nearest returns the entry of the same rock whose zoom is closest to
// key's, provided stretching it would not look too poor.
func (s *Surface) nearest(key surfKey) *surfEntry {
	var best *surfEntry
	bestR := float64(3)
	for _, e := range s.entries {
		if !e.key.sameRock(key) || e.key.scale != key.scale {
			continue
		}
		r := float64(key.hexQ) / float64(e.key.hexQ)
		if r < 1 {
			r = 1 / r
		}
		if r < bestR {
			best, bestR = e, r
		}
	}
	return best
}

// asked records that key was wanted and reports whether it was already
// wanted a moment ago.
func (s *Surface) asked(key surfKey) bool {
	const memory = 96 // draws
	hit := false
	n := 0
	for _, p := range s.pending {
		if s.seq-p.seen > memory {
			continue
		}
		if p.key == key {
			hit = true
			continue
		}
		s.pending[n] = p
		n++
	}
	s.pending = s.pending[:n]
	if !hit {
		s.pending = append(s.pending, surfPending{key, s.seq})
	}
	return hit
}

// makeRoom evicts least recently used layers until one of px pixels can be
// added within the limits.
func (s *Surface) makeRoom(px int) {
	for len(s.entries) > 0 {
		total := px
		old := 0
		for i, e := range s.entries {
			b := e.img.Bounds()
			total += b.Dx() * b.Dy()
			if e.used < s.entries[old].used {
				old = i
			}
		}
		if len(s.entries) < surfaceEntries && total <= surfaceBudgetPx {
			return
		}
		s.entries[old].release()
		s.entries = append(s.entries[:old], s.entries[old+1:]...)
	}
}

func (s *Surface) release() {
	for _, e := range s.entries {
		e.release()
	}
	s.entries = nil
	s.pending = nil
}

// render paints the static layer of rock for key and stores it, evicting
// the least recently used layers if the Surface is full.
func (s *Surface) render(key surfKey, rock *model.Rock, hexSize, sc float32) *surfEntry {
	// Bounds of the blob in hex units.
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for i := range rock.Tiles {
		if rock.Tiles[i].Terrain == model.TerrainVoid {
			continue
		}
		if h, ok := rock.HexAt(i); ok {
			x, y := hexCentre(h)
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
		}
	}
	if minX > maxX {
		minX, maxX, minY, maxY = 0, 0, 0, 0
	}
	minX, maxX = minX-sqrt3/2-surfaceMargin, maxX+sqrt3/2+surfaceMargin
	minY, maxY = minY-1-surfaceMargin, maxY+1+surfaceMargin

	// The image scale: the canvas scale, unless that would make the image
	// too large.
	rs := sc
	if side := max(maxX-minX, maxY-minY) * hexSize * sc; side > surfaceMaxPx {
		rs = sc * surfaceMaxPx / side
	}
	hexPx := hexSize * rs
	oxPx := float32(math.Ceil(float64(-minX * hexPx)))
	oyPx := float32(math.Ceil(float64(-minY * hexPx)))
	wPx := oxPx + float32(math.Ceil(float64(maxX*hexPx)))
	hPx := oyPx + float32(math.Ceil(float64(maxY*hexPx)))

	s.makeRoom(int(wPx) * int(hPx))
	e := &surfEntry{
		key: key, hex: hexSize, rscale: rs,
		ox: oxPx / rs, oy: oyPx / rs, w: wPx / rs, h: hPx / rs,
		veils: map[model.FactionID]*surfVeil{}, palette: paletteFor(rock.Class),
	}
	e.img = gfx.Render(e.w, e.h, rs, func(c *gfx.Canvas) {
		p := newPen(c, e.ox, e.oy, hexSize)
		paintRock(&p, rock, e.palette)
	})
	s.entries = append(s.entries, e)
	return e
}

// ---------------------------------------------------------------------------
// The static layer
// ---------------------------------------------------------------------------

// skirtReach returns how far, in hex units, the skirt extends beyond the
// outline corner v.
func skirtReach(seed uint64, v outlineVert) float32 {
	r := unit(mix64(seed ^ v.key*0x9E3779B97F4A7C15))
	if v.convex {
		return 0.30 + 0.36*r
	}
	return 0.20 + 0.16*r
}

// paintRock draws the static layer through a pen whose unit is the hex size
// and whose origin is the centre of hex (0, 0).
func paintRock(p *pen, rock *model.Rock, pal rockPalette) {
	seed := rock.ShapeSeed
	hexPx := 1 / p.px // pixels per hex unit
	loops := blobOutline(rock)
	if len(loops) == 0 {
		return
	}

	// The skirt outline: every corner of the blob pushed outwards by an
	// uneven amount.
	skirt := make([][]float32, len(loops))
	for li, loop := range loops {
		xy := make([]float32, 0, 2*len(loop))
		for _, v := range loop {
			d := skirtReach(seed, v) * 1.155
			xy = append(xy, v.x+v.nx*d, v.y+v.ny*d)
		}
		skirt[li] = xy
	}

	// A soft shadow around the whole rock, heavier to the lower right.
	for _, loop := range loops {
		for i, v := range loop {
			if i%2 == 1 {
				continue
			}
			p.blob(v.x+0.25, v.y+0.35, 1.9, 1.9, gfx.Color{R: 2, G: 3, B: 6, A: 0x50})
		}
	}

	// The skirt: a dark base, then one facet per outline edge, toned by the
	// way it faces.
	var path gfx.Path
	for _, xy := range skirt {
		path.MoveTo(xy[0], xy[1])
		for i := 2; i < len(xy); i += 2 {
			path.LineTo(xy[i], xy[i+1])
		}
		path.Close()
	}
	p.path(&path, gfx.PathStyle{Fill: pal.cliffMid})
	for li, loop := range loops {
		n := len(loop)
		xy := skirt[li]
		for i := 0; i < n; i++ {
			j := (i + 1) % n
			a, b := loop[i], loop[j]
			ex, ey := b.x-a.x, b.y-a.y
			l := float32(math.Hypot(float64(ex), float64(ey)))
			f := facing(ey/l, -ex/l)
			f += (unit(mix64(seed^a.key*31+7)) - 0.5) * 0.3
			var c gfx.Color
			if f >= 0 {
				c = gfx.Lerp(pal.cliffMid, pal.cliffLit, min(1, f*1.15))
			} else {
				c = gfx.Lerp(pal.cliffMid, pal.cliffDark, min(1, -f*1.15))
			}
			p.poly(c, a.x, a.y, b.x, b.y, xy[2*j], xy[2*j+1], xy[2*i], xy[2*i+1])
		}
	}
	p.path(&path, gfx.PathStyle{Stroke: pal.outline, Width: p.w(0.05, 1.2), Join: gfx.JoinRound})

	// The plateau.
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
	if len(tiles) == 0 {
		return
	}
	mx, my := sx/float32(len(tiles)), sy/float32(len(tiles))
	span := float32(rock.Radius)*sqrt3 + 1
	grow := 0.5 * p.px // hide the seams between neighbouring tiles
	for i := range tiles {
		ti := &tiles[i]
		h := ti.h
		// Value: slow noise, a little per-tile grain, the lie of the land
		// towards the light, and a rim that is lit on the upper left and
		// shaded on the lower right.
		v := (fbm(seed, float64(ti.x)*0.21, float64(ti.y)*0.21) - 0.5) * 0.36
		v += (unit(hash2(seed^0x51, h.Q, h.R)) - 0.5) * 0.085
		v += 0.05 * ((ti.x-mx)*lightX + (ti.y-my)*lightY) / span
		for d := 0; d < 6; d++ {
			n := h.Neighbor(d)
			if !rock.InBlob(n) {
				v += 0.055 * facing(dirVec[d][0], dirVec[d][1])
			} else if !rock.InBlob(n.Neighbor(d)) {
				v += 0.022 * facing(dirVec[d][0], dirVec[d][1])
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
		q := p.pt(ti.x, ti.y)
		p.cv.FillHex(q.X, q.Y, (1+grow)*p.k, ti.ground)
	}

	// Mottling that ignores the tile boundaries: soft lighter and darker
	// patches, kept to the interior so that none spills over the edge.
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
		c := alpha(tone(pal.base, 0.2), 0.34)
		if r.f() < 0.55 {
			c = alpha(tone(pal.base, -0.3), 0.38)
		}
		p.blob(ti.x+d*cosf(a), ti.y+d*sinf(a), rad, rad*r.in(0.7, 1), c)
	}

	// A faint grid: each shared edge once.
	gridW := p.w(0.025, 1)
	gridC := alpha(pal.grid, 0.26)
	for i := range tiles {
		ti := &tiles[i]
		c := gfx.HexPoints(ti.x, ti.y, 1)
		for _, d := range [3]int{0, 5, 4} {
			if !rock.InBlob(ti.h.Neighbor(d)) {
				continue
			}
			a, b := c[dirEdge[d][0]], c[dirEdge[d][1]]
			p.line(a.X, a.Y, b.X, b.Y, gridW/p.k, 0, gridC)
		}
	}

	// The plateau edge: a light line where it faces the light, a dark one
	// where it faces away.
	rimW := max(0.07, 1.3*p.px)
	for _, loop := range loops {
		n := len(loop)
		for i := 0; i < n; i++ {
			a, b := loop[i], loop[(i+1)%n]
			ex, ey := b.x-a.x, b.y-a.y
			l := float32(math.Hypot(float64(ex), float64(ey)))
			nx, ny := ey/l, -ex/l
			f := facing(nx, ny)
			in := rimW / 2
			x0, y0, x1, y1 := a.x-nx*in, a.y-ny*in, b.x-nx*in, b.y-ny*in
			switch {
			case f > 0.2:
				p.line(x0, y0, x1, y1, rimW, 0, alpha(pal.rimLit, 0.35+0.5*f))
			case f < -0.2:
				p.line(x0, y0, x1, y1, rimW, 0, alpha(pal.rimDark, 0.3-0.4*f))
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
				rad := max(r.in(0.025, 0.06), 0.55*p.px)
				c := tone(ti.ground, -0.14)
				if r.f() < 0.4 {
					c = tone(ti.ground, 0.12)
				}
				p.circle(ti.x+d*cosf(a), ti.y+d*sinf(a), rad, alpha(c, 0.8))
			}
		}
		if r.f() < 0.13 {
			paintCrater(p, ti.x+r.in(-0.2, 0.2), ti.y+r.in(-0.2, 0.2), r.in(0.2, 0.36), ti.ground)
		}
	}

	// Blocked ground. Fissures first, so that the shadows of crags fall
	// across them.
	isT := func(h model.Hex, t model.Terrain) bool {
		tl := rock.TileAt(h)
		return tl != nil && tl.Terrain == t
	}
	for i := range tiles {
		if tiles[i].t.Terrain == model.TerrainFissure {
			paintFissure(p, seed, tiles[i].h, tiles[i].x, tiles[i].y, pal, func(h model.Hex) bool { return isT(h, model.TerrainFissure) })
		}
	}
	// Crags: a boulder per tile and a smaller one astride every edge shared
	// by two crag tiles, so that neighbours run together into a ridge. They
	// are painted from the back of the rock to the front.
	var rocks []boulder
	for i := range tiles {
		ti := &tiles[i]
		if ti.t.Terrain != model.TerrainCrag {
			continue
		}
		r := newRnd(hash2(seed^0xC4A6, ti.h.Q, ti.h.R))
		rocks = append(rocks, boulder{ti.x + r.in(-0.08, 0.08), ti.y + r.in(-0.08, 0.08), r.in(0.66, 0.82), r.next()})
		for _, d := range [3]int{0, 5, 4} {
			if !isT(ti.h.Neighbor(d), model.TerrainCrag) {
				continue
			}
			nx, ny := hexCentre(ti.h.Neighbor(d))
			rocks = append(rocks, boulder{(ti.x+nx)/2 + r.in(-0.12, 0.12), (ti.y+ny)/2 + r.in(-0.12, 0.12), r.in(0.42, 0.56), r.next()})
		}
	}
	for i := 1; i < len(rocks); i++ {
		for j := i; j > 0 && rocks[j].y+rocks[j].x*0.3 < rocks[j-1].y+rocks[j-1].x*0.3; j-- {
			rocks[j], rocks[j-1] = rocks[j-1], rocks[j]
		}
	}
	for _, bl := range rocks {
		paintBoulder(p, bl, pal)
	}
}

// paintCrater draws a small bowl: its near wall in shadow, its far wall
// catching the light.
func paintCrater(p *pen, x, y, r float32, ground gfx.Color) {
	p.circle(x, y, r, tone(ground, -0.1))
	w := max(r*0.22, 1*p.px)
	p.arc(x, y, r-w/2, litAngle-1.25, litAngle+1.25, w, 0, alpha(tone(ground, -0.3), 0.85))
	p.arc(x, y, r-w/2, shadeAngle-1.1, shadeAngle+1.1, w, 0, alpha(tone(ground, 0.2), 0.85))
	if r/p.px > 4 {
		p.arc(x, y, r+w*0.4, litAngle-0.9, litAngle+0.9, w*0.7, 0, alpha(tone(ground, 0.16), 0.6))
	}
}

// boulder is one faceted outcrop of a crag: centre and size in hex units.
type boulder struct {
	x, y, r float32
	seed    uint64
}

// paintBoulder draws an outcrop: a ring of facets around an off-centre
// summit, light towards the light and dark away from it, with a shadow
// thrown to the lower right.
func paintBoulder(p *pen, bl boulder, pal rockPalette) {
	r := newRnd(bl.seed)
	const n = 7
	var ring [2 * n]float32
	a0 := r.in(0, tau)
	for i := 0; i < n; i++ {
		a := a0 + tau*float32(i)/n + r.in(-0.18, 0.18)
		rad := bl.r * r.in(0.8, 1.04)
		ring[2*i], ring[2*i+1] = bl.x+rad*cosf(a), bl.y+rad*sinf(a)*0.94
	}
	// The summit sits towards the light, so the lit facets are short and
	// steep and the shaded ones long.
	px, py := bl.x+bl.r*r.in(-0.2, 0.08), bl.y+bl.r*r.in(-0.24, 0.04)
	var sh [2 * n]float32
	for i := 0; i < n; i++ {
		sh[2*i], sh[2*i+1] = ring[2*i]+0.2*bl.r, ring[2*i+1]+0.27*bl.r
	}
	p.poly(gfx.Color{R: 4, G: 5, B: 9, A: 0x70}, sh[:]...)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		ax, ay, bx, by := ring[2*i], ring[2*i+1], ring[2*j], ring[2*j+1]
		ex, ey := bx-ax, by-ay
		l := float32(math.Hypot(float64(ex), float64(ey)))
		f := facing(ey/l, -ex/l) + r.in(-0.14, 0.14)
		var c gfx.Color
		if f >= 0 {
			c = gfx.Lerp(pal.cragMid, pal.cragLit, min(1, f*1.25))
		} else {
			c = gfx.Lerp(pal.cragMid, pal.cragDark, min(1, -f*1.25))
		}
		p.poly(c, px, py, ax, ay, bx, by)
	}
	p.outline(alpha(pal.outline, 0.8), 0.035, 1, ring[:]...)
}

// edgePoint returns where a fissure crosses the edge between h and its
// neighbour in direction d; both tiles compute the same point.
func edgePoint(seed uint64, h model.Hex, d int) (x, y float32) {
	n := h.Neighbor(d)
	a, b := h, n
	if b.Q < a.Q || (b.Q == a.Q && b.R < a.R) {
		a, b = b, a
	}
	t := unit(hash2(seed^0xF155, a.Q*131+b.Q, a.R*131+b.R))*0.4 - 0.2
	cx, cy := hexCentre(h)
	c := gfx.HexPoints(cx, cy, 1)
	p0, p1 := c[dirEdge[d][0]], c[dirEdge[d][1]]
	// Canonical direction along the edge, so that t means the same thing
	// from both sides.
	if p1.X < p0.X || (p1.X == p0.X && p1.Y < p0.Y) {
		p0, p1 = p1, p0
	}
	return (p0.X+p1.X)/2 + (p1.X-p0.X)*t, (p0.Y+p1.Y)/2 + (p1.Y-p0.Y)*t
}

// paintFissure draws the cracks of a fissure tile: jagged arms from a point
// near the centre to every neighbouring fissure tile, or ending in the tile
// when there is none that way.
func paintFissure(p *pen, seed uint64, h model.Hex, cx, cy float32, pal rockPalette, fissure func(model.Hex) bool) {
	r := newRnd(hash2(seed^0xF156, h.Q, h.R))
	ox, oy := cx+r.in(-0.15, 0.15), cy+r.in(-0.15, 0.15)
	type arm struct {
		x, y float32
		open bool // runs on into the next tile
	}
	var arms []arm
	for d := 0; d < 6; d++ {
		if fissure(h.Neighbor(d)) {
			x, y := edgePoint(seed, h, d)
			arms = append(arms, arm{x, y, true})
		}
	}
	// A lone tile, or the end of a chain, still needs to read as a crack:
	// add dead-end arms until there are two, roughly opposite, and now and
	// then a short branch.
	deadEnd := func(a, lo, hi float32) {
		d := r.in(lo, hi)
		arms = append(arms, arm{ox + d*cosf(a), oy + d*sinf(a), false})
	}
	switch len(arms) {
	case 0:
		a := r.in(0, tau)
		deadEnd(a+r.in(-0.3, 0.3), 0.55, 0.76)
		deadEnd(a+pi+r.in(-0.4, 0.4), 0.55, 0.76)
	case 1:
		a := float32(math.Atan2(float64(arms[0].y-oy), float64(arms[0].x-ox))) + pi
		deadEnd(a+r.in(-0.5, 0.5), 0.5, 0.72)
	}
	if len(arms) == 2 && r.f() < 0.5 {
		a := float32(math.Atan2(float64(arms[0].y-oy), float64(arms[0].x-ox))) + pi/2
		if r.f() < 0.5 {
			a += pi
		}
		deadEnd(a+r.in(-0.35, 0.35), 0.35, 0.55)
	}
	const w0 = 0.2
	for pass := 0; pass < 2; pass++ {
		rr := r // the same wiggles in both passes
		for _, a := range arms {
			// Three jagged segments.
			var pts [8]float32
			dx, dy := a.x-ox, a.y-oy
			l := float32(math.Hypot(float64(dx), float64(dy)))
			nx, ny := -dy/l, dx/l
			for k := 0; k < 4; k++ {
				t := float32(k) / 3
				j := float32(0)
				if k == 1 || k == 2 {
					j = rr.in(-0.14, 0.14)
				}
				pts[2*k], pts[2*k+1] = ox+dx*t+nx*j, oy+dy*t+ny*j
			}
			w1 := float32(0.13)
			if !a.open {
				w1 = 0.02
			}
			for k := 0; k < 3; k++ {
				wa := lerpf(w0, w1, float32(k)/3)
				wb := lerpf(w0, w1, float32(k+1)/3)
				x0, y0, x1, y1 := pts[2*k], pts[2*k+1], pts[2*k+2], pts[2*k+3]
				sx, sy := x1-x0, y1-y0
				sl := float32(math.Hypot(float64(sx), float64(sy)))
				mx, my := -sy/sl, sx/sl
				if pass == 0 {
					// The far lip of the crack catches the light.
					const lx, ly = 0.045, 0.06
					p.poly(alpha(pal.crackLip, 0.7), x0+mx*wa/2+lx, y0+my*wa/2+ly, x1+mx*wb/2+lx, y1+my*wb/2+ly,
						x1-mx*wb/2+lx, y1-my*wb/2+ly, x0-mx*wa/2+lx, y0-my*wa/2+ly)
				} else {
					p.poly(pal.crack, x0+mx*wa/2, y0+my*wa/2, x1+mx*wb/2, y1+my*wb/2, x1-mx*wb/2, y1-my*wb/2, x0-mx*wa/2, y0-my*wa/2)
					p.circle(x0, y0, wa/2, pal.crack)
				}
			}
		}
		if pass == 1 {
			p.circle(ox, oy, w0*0.62, pal.crack)
		}
	}
}

// ---------------------------------------------------------------------------
// The veil over unsurveyed ground
// ---------------------------------------------------------------------------

// veilOpacity is how strongly the veil image is laid over the ground.
const veilOpacity = 0.36

var (
	veilFill  = gfx.RGB(0x06070C)
	veilHatch = gfx.RGB(0x596178)
)

// renderVeil paints the veil for the viewer's current knowledge into v's
// image (or a new one) and stores it on e.
func (s *Surface) renderVeil(e *surfEntry, v *surfVeil, rock *model.Rock, viewer model.FactionID, known uint64) *surfVeil {
	if v == nil {
		pw, ph := gfx.PixelSize(e.w, e.h, e.rscale)
		v = &surfVeil{img: ebiten.NewImage(pw, ph)}
		e.veils[viewer] = v
	} else {
		v.img.Clear()
	}
	v.known = known
	c := gfx.NewCanvas(v.img, e.rscale)
	p := newPen(c, e.ox, e.oy, e.hex)
	hexPx := 1 / p.px
	grow := 0.5 * p.px
	// Hatching runs along lines x + y = const, a fixed number of pixels
	// apart so that it lines up from tile to tile.
	gap := max(0.42, 5*p.px)
	lw := max(1.0, float32(math.Round(float64(hexPx*0.035)))) * p.px
	for i := range rock.Tiles {
		t := &rock.Tiles[i]
		if t.Terrain == model.TerrainVoid {
			continue
		}
		h, ok := rock.HexAt(i)
		if !ok || rock.KnownTo(h, viewer) {
			continue
		}
		x, y := hexCentre(h)
		q := p.pt(x, y)
		c.FillHex(q.X, q.Y, (1+grow)*p.k, veilFill)
		corners := gfx.HexPoints(x, y, 1+grow)
		k0 := int(math.Ceil(float64((x + y - 1.6) / gap)))
		k1 := int(math.Floor(float64((x + y + 1.6) / gap)))
		for k := k0; k <= k1; k++ {
			if x0, y0, x1, y1, ok := clipDiagonal(corners[:], float32(k)*gap); ok {
				p.line(x0, y0, x1, y1, lw, 0, veilHatch)
			}
		}
	}
	return v
}

// clipDiagonal returns the part of the line x + y = c that lies inside the
// convex polygon poly.
func clipDiagonal(poly []gfx.Pt, c float32) (x0, y0, x1, y1 float32, ok bool) {
	n := 0
	var out [2]gfx.Pt
	for i := range poly {
		a, b := poly[i], poly[(i+1)%len(poly)]
		da, db := a.X+a.Y-c, b.X+b.Y-c
		if (da < 0) == (db < 0) {
			continue
		}
		t := da / (da - db)
		if n < 2 {
			out[n] = gfx.Pt{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
			n++
		}
	}
	if n < 2 {
		return 0, 0, 0, 0, false
	}
	return out[0].X, out[0].Y, out[1].X, out[1].Y, true
}
