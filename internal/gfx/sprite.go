package gfx

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// PixelSize returns the size in pixels of an image that holds w × h points
// at scale pixels per point: each side rounded up, at least 1.
func PixelSize(w, h, scale float32) (pw, ph int) {
	scale = effScale(scale)
	pw = max(1, int(math.Ceil(float64(w*scale)-1e-3)))
	ph = max(1, int(math.Ceil(float64(h*scale)-1e-3)))
	return pw, ph
}

// Render creates a transparent image big enough for w × h points at scale
// pixels per point, lets draw paint it through a canvas of that scale whose
// origin is the image's top-left corner, and returns it.
//
// It is the way to turn expensive art (paths, gradients, many primitives)
// into a sprite that is drawn each frame with one DrawSprite call. Creating
// images is costly: call Render when something changes, never every frame.
// Pass the canvas scale you will draw the sprite on (Canvas.Scale) so that
// one image pixel lands on one screen pixel.
func Render(w, h, scale float32, draw func(cv *Canvas)) *ebiten.Image {
	scale = effScale(scale)
	pw, ph := PixelSize(w, h, scale)
	img := ebiten.NewImage(pw, ph)
	if draw != nil {
		draw(&Canvas{Dst: img, Scale: scale})
	}
	return img
}

// SpriteCache keeps rendered sprites keyed by a caller-chosen id and their
// pixel size, so that procedural art is rasterised once per appearance and
// size instead of every frame. When the UI scale or a zoom level changes,
// the pixel size changes with it and fresh sprites are rendered; the ones no
// longer asked for are evicted, least recently used first.
//
//	img := cache.Get(uint64(kind)<<8|uint64(faction), 28, 28, cv.Scale, func(sc *gfx.Canvas) {
//		drawBuilding(sc, kind, faction) // paints into (0,0)-(28,28)
//	})
//	cv.DrawSprite(img, x, y, nil)
//
// A SpriteCache is not safe for concurrent use.
type SpriteCache struct {
	limit   int
	clock   uint64
	entries map[spriteKey]*spriteEntry
}

type spriteKey struct {
	id     uint64
	pw, ph int32
}

type spriteEntry struct {
	img  *ebiten.Image
	used uint64
}

// DefaultSpriteLimit is the capacity of a SpriteCache created with a limit
// that is not positive.
const DefaultSpriteLimit = 512

// NewSpriteCache returns an empty cache holding at most limit sprites.
func NewSpriteCache(limit int) *SpriteCache {
	if limit <= 0 {
		limit = DefaultSpriteLimit
	}
	return &SpriteCache{limit: limit, entries: make(map[spriteKey]*spriteEntry)}
}

// Get returns the sprite for id at the pixel size of w × h points at scale
// pixels per point, rendering it with draw (see Render) if it is not cached.
//
// The id must capture everything that changes the picture apart from its
// size: kind, faction, state, animation frame. draw is only called on a
// miss, so the id — not the closure — decides what is shown.
//
// The returned image belongs to the cache: draw it right away and do not
// keep it, because a later Get may evict and release it.
func (sc *SpriteCache) Get(id uint64, w, h, scale float32, draw func(cv *Canvas)) *ebiten.Image {
	scale = effScale(scale)
	pw, ph := PixelSize(w, h, scale)
	key := spriteKey{id, int32(pw), int32(ph)}
	sc.clock++
	if e := sc.entries[key]; e != nil {
		e.used = sc.clock
		return e.img
	}
	if len(sc.entries) >= sc.limit {
		sc.evict()
	}
	e := &spriteEntry{img: Render(w, h, scale, draw), used: sc.clock}
	sc.entries[key] = e
	return e.img
}

// evict releases the least recently used quarter of the cache (at least one
// sprite), so that a full cache does not evict on every miss.
func (sc *SpriteCache) evict() {
	n := max(1, len(sc.entries)/4)
	for ; n > 0 && len(sc.entries) > 0; n-- {
		var oldest spriteKey
		best := uint64(math.MaxUint64)
		for k, e := range sc.entries {
			if e.used < best {
				best, oldest = e.used, k
			}
		}
		sc.entries[oldest].img.Deallocate()
		delete(sc.entries, oldest)
	}
}

// Len returns the number of sprites currently cached.
func (sc *SpriteCache) Len() int { return len(sc.entries) }

// Clear releases every cached sprite.
func (sc *SpriteCache) Clear() {
	for k, e := range sc.entries {
		e.img.Deallocate()
		delete(sc.entries, k)
	}
}
