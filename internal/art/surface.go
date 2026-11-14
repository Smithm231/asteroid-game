package art

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// A rock is seen at an angle, from above and in front: a plateau of hex
// tiles, foreshortened by Tilt, on a body of rock whose cliff shows along
// the near edge (rockpaint.go paints it).
//
// Three layers make up a surface:
//
//   - the static layer (haze, cliff, tiles, craters, crags, fissures) is
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
	ext := (float32(2*rock.Radius+1) + surfaceReach) * hexSize
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

// ---------------------------------------------------------------------------
// The static layer
// ---------------------------------------------------------------------------

// boulder is one faceted outcrop of a crag: centre and size in hex units.
type boulder struct {
	x, y, r float32
	seed    uint64
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
		if !fissure(h.Neighbor(d)) {
			continue
		}
		// Three fissure tiles that all touch would join up into a
		// triangle; leaving out the link along one of the three axes
		// keeps them a branching crack.
		if d%3 == 0 && (fissure(h.Neighbor((d+1)%6)) || fissure(h.Neighbor((d+5)%6))) {
			continue
		}
		x, y := edgePoint(seed, h, d)
		arms = append(arms, arm{x, y, true})
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
	paintVeil(c, e.ox, e.oy, e.hex, rock, viewer)
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
