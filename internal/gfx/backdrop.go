package gfx

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Backdrop is the space behind everything: near-black indigo, a sparse
// starfield in three parallax layers with a few warm stars, and the faint
// red-orange glow of Emberfall.
//
// The stars of each layer are rendered once into a square tile at the
// canvas's pixel density and the tile is repeated across the screen, each
// repetition turned or mirrored differently so that no pattern shows. A
// frame therefore costs a few dozen image draws and two glow quads however
// many stars there are. The same seed always gives the same sky, at every
// scale.
type Backdrop struct {
	seed   uint64
	scale  float32 // pixel density the tiles were rendered at; 0 = not yet
	tilePx int     // side of a tile in pixels
	tiles  [len(starLayers)]*ebiten.Image
}

// BackdropView positions the backdrop for one frame. The zero value shows
// the sky at rest with Emberfall at the canvas origin.
type BackdropView struct {
	// X and Y are the camera position in points. As the camera moves the
	// stars drift the other way, far layers least (a few percent of the
	// camera motion), which gives the scene depth.
	X, Y float32
	// EmberX and EmberY are where Emberfall is on screen, in points. It
	// may be far outside the visible area; the glow then leans in from that
	// side.
	EmberX, EmberY float32
	// EmberRadius is how far the glow reaches, in points. Zero means
	// DefaultEmberRadius; a negative value turns the glow off.
	EmberRadius float32
	// Time is the animation clock in seconds. It drives the slow glints of
	// a few stars; leave it at a constant for a still sky.
	Time float32
}

// DefaultEmberRadius is the reach of the Emberfall glow, in points, when
// BackdropView.EmberRadius is zero.
const DefaultEmberRadius = 1100

// backdropTile is the side of a star tile in points.
const backdropTile = 512

// starLayer describes one parallax layer of the starfield.
type starLayer struct {
	count    int     // stars per tile
	parallax float32 // fraction of the camera motion the layer follows
	sizeMin  float32 // star radius (Gaussian sigma) range, points
	sizeMax  float32
	lumMin   float32 // brightness range
	lumMax   float32
	warm     float32 // fraction of amber and orange stars
	halo     float32 // strength of the soft halo around each star
}

// The three layers, far to near.
var starLayers = [...]starLayer{
	{count: 150, parallax: 0.015, sizeMin: 0.36, sizeMax: 0.58, lumMin: 0.14, lumMax: 0.55, warm: 0.05},
	{count: 52, parallax: 0.040, sizeMin: 0.48, sizeMax: 0.80, lumMin: 0.32, lumMax: 0.85, warm: 0.12},
	{count: 11, parallax: 0.090, sizeMin: 0.75, sizeMax: 1.20, lumMin: 0.70, lumMax: 1.00, warm: 0.34, halo: 0.15},
}

// Star tints.
var (
	starCool  = [3]float32{0.74, 0.83, 1.00}
	starCream = [3]float32{1.00, 0.96, 0.88}
	starAmber = [3]float32{1.00, 0.78, 0.50}
	starEmber = [3]float32{1.00, 0.60, 0.44}
)

// NewBackdrop returns the sky for the given seed. Nothing is rendered until
// the first Draw.
func NewBackdrop(seed uint64) *Backdrop {
	return &Backdrop{seed: seed}
}

// splitmix is a small deterministic random number generator (SplitMix64).
type splitmix struct{ s uint64 }

func (r *splitmix) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// float returns a uniform value in [0, 1).
func (r *splitmix) float() float32 { return float32(r.next()>>40) / (1 << 24) }

// between returns a uniform value in [lo, hi).
func (r *splitmix) between(lo, hi float32) float32 { return lo + (hi-lo)*r.float() }

// hash64 mixes its arguments into one well-scrambled value.
func hash64(a, b, c, d uint64) uint64 {
	r := splitmix{a ^ b*0xD6E8FEB86659FD93 ^ c*0xA24BAED4963EE407 ^ d*0x9FB21C651E98DF25}
	return r.next()
}

// star is one generated star, positioned in points within its tile.
type star struct {
	x, y  float32
	sigma float32
	lum   float32
	tint  [3]float32
}

// layerStars returns the stars of a layer for a seed. The result depends on
// nothing else, so every scale shows the same sky.
func layerStars(seed uint64, layer int) []star {
	l := starLayers[layer]
	r := splitmix{hash64(seed, uint64(layer)+1, 0x57A8, 0)}
	// Stars keep clear of the tile border by the reach of their light, so
	// that nothing is cut off where tiles meet.
	margin := 3*l.sizeMax*(1+3.2*btof(l.halo > 0)) + 1
	out := make([]star, l.count)
	for i := range out {
		u := r.float()
		s := star{
			x:     r.between(margin, backdropTile-margin),
			y:     r.between(margin, backdropTile-margin),
			sigma: r.between(l.sizeMin, l.sizeMax),
			lum:   l.lumMin + (l.lumMax-l.lumMin)*u*u*u, // many faint, few bright
		}
		switch t := r.float(); {
		case t < l.warm*0.45:
			s.tint = starEmber
		case t < l.warm:
			s.tint = starAmber
		case t < l.warm+0.30:
			s.tint = starCool
		default:
			s.tint = starCream
		}
		out[i] = s
	}
	return out
}

func btof(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// renderStars rasterises stars into a size × size premultiplied RGBA pixel
// buffer at the given density (pixels per point).
func renderStars(stars []star, halo float32, size int, density float32) []byte {
	pix := make([]byte, 4*size*size)
	add := func(cx, cy, sigma, lum float32, tint [3]float32) {
		reach := int(math.Ceil(float64(3 * sigma)))
		x0, x1 := max(0, int(cx)-reach), min(size-1, int(cx)+reach)
		y0, y1 := max(0, int(cy)-reach), min(size-1, int(cy)+reach)
		k := -1 / (2 * sigma * sigma)
		for y := y0; y <= y1; y++ {
			dy := float32(y) + 0.5 - cy
			for x := x0; x <= x1; x++ {
				dx := float32(x) + 0.5 - cx
				v := lum * float32(math.Exp(float64(k*(dx*dx+dy*dy))))
				if v < 1.0/512 {
					continue
				}
				o := 4 * (y*size + x)
				pix[o+0] = addSat(pix[o+0], v*tint[0])
				pix[o+1] = addSat(pix[o+1], v*tint[1])
				pix[o+2] = addSat(pix[o+2], v*tint[2])
				pix[o+3] = addSat(pix[o+3], v)
			}
		}
	}
	for _, s := range stars {
		cx, cy := s.x*density, s.y*density
		sigma := max(0.5, s.sigma*density)
		if sigma < 0.7 {
			// A star about one pixel across is centred on a pixel, so that
			// it is one crisp dot and not a smear over four.
			cx = float32(math.Floor(float64(cx))) + 0.5
			cy = float32(math.Floor(float64(cy))) + 0.5
		}
		add(cx, cy, sigma, s.lum, s.tint)
		if halo > 0 {
			add(cx, cy, sigma*3.2, s.lum*halo, s.tint)
		}
	}
	return pix
}

// addSat adds v (0..1) to an 8-bit channel, saturating.
func addSat(b uint8, v float32) uint8 {
	n := int(b) + int(v*255+0.5)
	if n > 255 {
		return 255
	}
	return uint8(n)
}

// prepare renders the star tiles for the given density if they are missing
// or were made for another one.
func (b *Backdrop) prepare(density float32) {
	if b.scale == density && b.tiles[0] != nil {
		return
	}
	b.Release()
	b.scale = density
	b.tilePx = max(16, int(math.Round(float64(backdropTile*density))))
	d := float32(b.tilePx) / backdropTile
	for i := range b.tiles {
		pix := renderStars(layerStars(b.seed, i), starLayers[i].halo, b.tilePx, d)
		img := ebiten.NewImage(b.tilePx, b.tilePx)
		img.WritePixels(pix)
		b.tiles[i] = img
	}
}

// Release frees the star tiles. They are rendered again by the next Draw;
// call it when the backdrop will not be shown for a long time.
func (b *Backdrop) Release() {
	for i, t := range b.tiles {
		if t != nil {
			t.Deallocate()
			b.tiles[i] = nil
		}
	}
	b.scale = 0
}

// tileOrient holds the eight ways a square tile can be turned and mirrored:
// the 2×2 matrices (a, b; c, d) of the symmetries of a square.
var tileOrient = [8][4]float64{
	{1, 0, 0, 1}, {0, -1, 1, 0}, {-1, 0, 0, -1}, {0, 1, -1, 0},
	{-1, 0, 0, 1}, {0, 1, 1, 0}, {1, 0, 0, -1}, {0, -1, -1, 0},
}

// Colours of the backdrop.
var (
	spaceColor     = RGB(0x0B0D14)
	emberWide      = Color{0xD8, 0x44, 0x2A, 0x3A}
	emberInner     = Color{0xF0, 0x98, 0x48, 0x30}
	glintCool      = Color{0xD6, 0xE2, 0xFF, 0xFF}
	glintWarm      = Color{0xFF, 0xC8, 0x8A, 0xFF}
	backdropImgOpt ebiten.DrawImageOptions
)

// Draw paints the backdrop over the whole visible area of cv (its clip):
// the space colour, the Emberfall glow, then the star layers.
func (b *Backdrop) Draw(cv *Canvas, v BackdropView) {
	if cv == nil || cv.Dst == nil {
		return
	}
	area := cv.Dst.Bounds()
	if area.Empty() {
		return
	}
	s := cv.scale()
	b.prepare(s)

	cv.FillRect(cv.Bounds(), spaceColor)

	if v.EmberRadius >= 0 {
		r := v.EmberRadius
		if r == 0 {
			r = DefaultEmberRadius
		}
		cv.Glow(v.EmberX, v.EmberY, r, emberWide)
		cv.Glow(v.EmberX, v.EmberY, r*0.42, emberInner)
	}

	t := b.tilePx
	op := &backdropImgOpt
	for li, layer := range starLayers {
		// Whole-pixel offsets keep the stars sharp. Each layer also gets a
		// fixed shift so that the tile seams of the layers do not line up.
		offX := int(math.Round(float64(-v.X*layer.parallax*s+cv.ox))) + li*t/3
		offY := int(math.Round(float64(-v.Y*layer.parallax*s+cv.oy))) + li*t/5
		ix0, ix1 := floorDiv(area.Min.X-offX, t), floorDiv(area.Max.X-1-offX, t)
		iy0, iy1 := floorDiv(area.Min.Y-offY, t), floorDiv(area.Max.Y-1-offY, t)
		for iy := iy0; iy <= iy1; iy++ {
			for ix := ix0; ix <= ix1; ix++ {
				h := hash64(b.seed, uint64(li), uint64(int64(ix)), uint64(int64(iy)))
				m := tileOrient[h&7]
				// Move the turned tile so that its bounding box starts at
				// the cell's corner.
				ft := float64(t)
				tx := float64(offX+ix*t) - ft*(math.Min(m[0], 0)+math.Min(m[1], 0))
				ty := float64(offY+iy*t) - ft*(math.Min(m[2], 0)+math.Min(m[3], 0))
				op.GeoM.SetElement(0, 0, m[0])
				op.GeoM.SetElement(0, 1, m[1])
				op.GeoM.SetElement(1, 0, m[2])
				op.GeoM.SetElement(1, 1, m[3])
				op.GeoM.SetElement(0, 2, tx)
				op.GeoM.SetElement(1, 2, ty)
				cv.Dst.DrawImage(b.tiles[li], op)

				if li == len(starLayers)-1 {
					b.glints(cv, h, float32(offX+ix*t), float32(offY+iy*t), v.Time)
				}
			}
		}
	}
}

// glints draws the few slowly pulsing sparkles of one near-layer cell whose
// top-left pixel is (x0, y0).
func (b *Backdrop) glints(cv *Canvas, cellHash uint64, x0, y0, time float32) {
	r := splitmix{cellHash}
	s := cv.scale()
	for i := 0; i < 3; i++ {
		x := x0 + r.between(0.06, 0.94)*float32(b.tilePx)
		y := y0 + r.between(0.06, 0.94)*float32(b.tilePx)
		phase := r.float() * 2 * math.Pi
		speed := r.between(0.25, 0.7)
		size := r.between(3.5, 6.5)
		warm := r.float() < 0.35
		// A glint is dark most of the time and swells briefly.
		p := 0.5 + 0.5*float32(math.Sin(float64(phase+time*speed)))
		p = p * p * p * p
		if p < 0.02 {
			continue
		}
		col := glintCool
		if warm {
			col = glintWarm
		}
		// Back to points: the canvas converts again.
		cv.GlowCore((x-cv.ox)/s, (y-cv.oy)/s, size, Fade(col, 0.75*p))
	}
}

// floorDiv divides rounding towards negative infinity.
func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
