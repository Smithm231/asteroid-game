package art

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
	"drifthold/internal/sim"
)

// These tests draw into offscreen images. Ebitengine records such drawing
// without a running game loop, which is enough to prove that nothing panics
// and that the caches behave; how the pictures look is the business of the
// demo program (internal/art/demo).

func testCanvas(scale float32) *gfx.Canvas {
	return gfx.NewCanvas(ebiten.NewImage(640, 480), scale)
}

func testWorld(t testing.TB) *model.World {
	t.Helper()
	w, err := sim.NewGame(model.DefaultSettings(11))
	if err != nil {
		t.Fatalf("NewGame: %v", err)
	}
	return w
}

var testSizes = []float32{0, -3, 0.5, 3, 7.3, 14, 16, 18.5, 28, 61, 240, 900}

func testLooks(accent gfx.Color) []BuildingLook {
	return []BuildingLook{
		{Accent: accent, Progress: 1},
		{Accent: accent, Progress: 1, Time: 12.34},
		{Accent: accent, Progress: 1, Time: -5},
		{Accent: accent, Progress: 1, Off: true, Time: 3},
		{Accent: accent, Progress: 0},
		{Accent: accent, Progress: 0.3, Time: 1.5},
		{Accent: accent, Progress: 0.7, Time: 99},
		{Accent: accent, Progress: -1},
		{Accent: accent, Progress: 1, Damage: 0.2},
		{Accent: accent, Progress: 1, Damage: 0.55, Time: 2},
		{Accent: accent, Progress: 1, Damage: 1, Time: 7},
		{Accent: accent, Progress: 1, Damage: 3, Off: true},
		{Accent: accent, Progress: 1, Derelict: true},
		{Accent: accent, Progress: 0.4, Derelict: true, Damage: 0.9},
		{Progress: 1}, // no accent at all
		{Accent: accent, Progress: float32(math.NaN()), Damage: float32(math.NaN()), Time: float32(math.Inf(1))},
	}
}

func TestBuildingsDraw(t *testing.T) {
	defer ReleaseSprites()
	th := gfx.Default()
	for _, scale := range []float32{1, 1.5, 2} {
		cv := testCanvas(scale)
		for _, kind := range model.AllBuildingKinds() {
			if buildingArts[kind].body == nil {
				t.Fatalf("%v has no body art", kind)
			}
			for _, size := range testSizes {
				for i, look := range testLooks(th.FactionColor(1 + int(kind)%4)) {
					Building(cv, kind, 100+float32(i)*7.3, 90.4, size, look)
				}
			}
			BuildingIcon(cv, kind, gfx.R(10, 10, 48, 40), th.FactionColor(2))
			BuildingIcon(cv, kind, gfx.R(10, 10, 0, 0), th.FactionColor(2))
		}
	}
	// Kinds outside the table are ignored.
	cv := testCanvas(1)
	Building(cv, model.BuildingNone, 50, 50, 20, BuildingLook{Progress: 1})
	Building(cv, model.NumBuildingKinds, 50, 50, 20, BuildingLook{Progress: 1})
	Building(nil, model.Keystone, 50, 50, 20, BuildingLook{Progress: 1})
	Building(&gfx.Canvas{}, model.Keystone, 50, 50, 20, BuildingLook{Progress: 1})
}

func TestBuildingsOnLocalCanvas(t *testing.T) {
	defer ReleaseSprites()
	cv := testCanvas(1.25).Local(gfx.R(33.3, 21.7, 200, 200), 320)
	for _, kind := range model.AllBuildingKinds() {
		Building(cv, kind, 100, 100, 30, BuildingLook{Accent: gfx.RGB(0x3DC9B3), Progress: 1, Time: 1})
		Building(cv, kind, 200, 100, 30, BuildingLook{Accent: gfx.RGB(0x3DC9B3), Progress: 0.5, Time: 1})
	}
}

func TestShipsDraw(t *testing.T) {
	defer ReleaseSprites()
	th := gfx.Default()
	headings := []float32{0, 0.3, -1.5708, 3.1416, -7, 100, float32(math.NaN())}
	for _, scale := range []float32{1, 2} {
		cv := testCanvas(scale)
		for _, kind := range model.AllShipKinds() {
			for _, size := range testSizes {
				for i, h := range headings {
					Ship(cv, kind, 120, 100, size, h, th.FactionColor(i%5))
				}
			}
			ShipIcon(cv, kind, gfx.R(4, 4, 32, 32), th.FactionColor(1))
			ShipIcon(cv, kind, gfx.R(4, 4, 200, 120), th.FactionColor(1))
			ShipIcon(cv, kind, gfx.R(4, 4, 0, 10), th.FactionColor(1))
		}
		Ship(cv, model.ShipNone, 10, 10, 20, 0, th.FactionColor(1))
		Ship(cv, model.NumShipKinds, 10, 10, 20, 0, th.FactionColor(1))
	}
}

func TestGoodsAndIconsDraw(t *testing.T) {
	defer ReleaseSprites()
	th := gfx.Default()
	boxes := []float32{0, 1, 5, 12, 13, 16, 17, 20, 24, 33, 64, 300}
	for _, scale := range []float32{0.75, 1, 1.5, 2} {
		cv := testCanvas(scale)
		for _, b := range boxes {
			r := gfx.R(20.5, 30.25, b, b*0.8)
			for _, g := range model.AllGoods() {
				Good(cv, g, r)
			}
			Good(cv, model.GoodNone, r)
			Good(cv, model.NumGoods, r)
			for id := IconID(-1); id <= NumIcons; id++ {
				Icon(cv, id, r, th.Text)
				Icon(cv, id, r, gfx.Color{})
			}
		}
	}
	for id := IconID(0); id < NumIcons; id++ {
		if iconArts[id] == nil {
			t.Errorf("icon %d has no art", id)
		}
	}
}

// TestIconStroke checks the rule that keeps glyphs sharp: the stroke is a
// whole number of pixels, and it grows with the box.
func TestIconStroke(t *testing.T) {
	prev := float32(0)
	for box := 6; box <= 96; box++ {
		sw := iconStroke(box)
		if sw < 1 || sw != float32(math.Round(float64(sw))) {
			t.Errorf("box %d: stroke %v is not a whole number of pixels", box, sw)
		}
		if sw < prev {
			t.Errorf("box %d: stroke %v is thinner than at the previous size (%v)", box, sw, prev)
		}
		prev = sw
	}
	if got := iconStroke(16); got != 1 {
		t.Errorf("a 16-pixel glyph should have 1-pixel strokes, has %v", got)
	}
	// Snapped coordinates are mirror images about the centre of the grid.
	p := ipen{s: 1, upx: 20.0 / 16, c: 10, sw: 2}
	for u := float32(0); u <= 8; u += 0.1 {
		l, r := p.px(8-u), p.px(8+u)
		if l+r != 2*p.c {
			t.Fatalf("u=%v: %v and %v are not symmetric about %v", u, l, r, p.c)
		}
		if l != float32(math.Round(float64(l))) {
			t.Fatalf("u=%v: %v is not on the pixel grid", u, l)
		}
	}
}

func TestSurfaceDraw(t *testing.T) {
	defer ReleaseSprites()
	w := testWorld(t)
	s := NewSurface()
	defer s.Release()
	cv := testCanvas(1)
	for _, rock := range w.Rocks {
		for _, hex := range []float32{0, 2, 10, 16, 28} {
			for _, viewer := range []model.FactionID{model.NoFaction, model.PlayerFaction, 2, 200} {
				s.Draw(cv, rock, gfx.P(320, 240), hex, viewer, false)
				s.Draw(cv, rock, gfx.P(320, 240), hex, viewer, true)
			}
		}
		if len(s.entries) > surfaceEntries {
			t.Fatalf("surface holds %d layers, limit %d", len(s.entries), surfaceEntries)
		}
	}
	// Degenerate input.
	s.Draw(cv, nil, gfx.P(0, 0), 16, 1, false)
	s.Draw(nil, w.Rocks[0], gfx.P(0, 0), 16, 1, false)
	s.Draw(cv, model.NewRock(99, 0), gfx.P(100, 100), 16, 1, true)
	s.Draw(cv, model.NewRock(98, 3), gfx.P(100, 100), 16, 1, true) // all Void
	s.Draw(cv, w.Rocks[0], gfx.P(1e6, 1e6), 16, 1, true)           // far off screen
	var nilSurface *Surface
	nilSurface.Draw(cv, w.Rocks[0], gfx.P(0, 0), 16, 1, false)
	nilSurface.Release()

	// A very close zoom is capped rather than allocating a huge image.
	big := NewSurface()
	defer big.Release()
	big.Draw(cv, w.Rocks[0], gfx.P(320, 240), 400, 1, true)
	if len(big.entries) != 1 {
		t.Fatalf("got %d layers", len(big.entries))
	}
	b := big.entries[0].img.Bounds()
	if b.Dx() > surfaceMaxPx+2 || b.Dy() > surfaceMaxPx+2 {
		t.Errorf("layer is %d × %d pixels, cap is %d", b.Dx(), b.Dy(), surfaceMaxPx)
	}
}

// TestSurfaceCache checks when the static layer is painted afresh: on first
// sight, when a zoom has settled, and when the terrain changes — but not for
// every step of a moving zoom, nor when only knowledge or deposits change.
func TestSurfaceCache(t *testing.T) {
	defer ReleaseSprites()
	w := testWorld(t)
	rock := w.Rock(w.Human().HomeRock)
	cv := testCanvas(1)
	s := NewSurface()
	defer s.Release()
	c := gfx.P(320, 240)
	rendered := func() int {
		n := 0
		for _, e := range s.entries {
			if e.img != nil {
				n++
			}
		}
		return n
	}

	s.Draw(cv, rock, c, 16, 1, false)
	if rendered() != 1 {
		t.Fatalf("first draw: %d layers", rendered())
	}
	first := s.entries[0].img
	for i := 0; i < 10; i++ {
		s.Draw(cv, rock, gfx.P(c.X+float32(i), c.Y), 16, 1, i%2 == 0)
	}
	if rendered() != 1 || s.entries[0].img != first {
		t.Fatalf("redrawing at the same zoom re-rendered the layer")
	}

	// A sweeping zoom: every frame a new size. Nothing new is painted.
	for i := 1; i <= 40; i++ {
		s.Draw(cv, rock, c, 16+float32(i)*0.31, 1, false)
	}
	if rendered() != 1 {
		t.Fatalf("a moving zoom painted %d layers", rendered())
	}
	// The zoom comes to rest: the second frame at the new size is sharp.
	s.Draw(cv, rock, c, 28, 1, false)
	if rendered() != 1 {
		t.Fatalf("first frame at rest painted a layer")
	}
	s.Draw(cv, rock, c, 28, 1, false)
	if rendered() != 2 {
		t.Fatalf("second frame at rest: %d layers, want 2", rendered())
	}

	// Knowledge and deposits do not touch the static layer.
	before := s.find(s.entries[len(s.entries)-1].key)
	img := before.img
	for i := range rock.Tiles {
		if rock.Tiles[i].Terrain != model.TerrainVoid && !rock.Tiles[i].Known.Has(1) {
			rock.Tiles[i].Reveal(1)
			break
		}
	}
	for i := range rock.Tiles {
		if rock.Tiles[i].HasDeposit() {
			rock.Tiles[i].Amount /= 2
		}
	}
	s.Draw(cv, rock, c, 28, 1, true)
	if before.img != img || rendered() != 2 {
		t.Fatalf("revealing a tile re-rendered the static layer")
	}

	// A change of terrain does.
	for i := range rock.Tiles {
		if rock.Tiles[i].Terrain == model.TerrainRegolith && rock.Tiles[i].Building == 0 {
			rock.Tiles[i].Terrain = model.TerrainCrag
			break
		}
	}
	s.Draw(cv, rock, c, 28, 1, false)
	s.Draw(cv, rock, c, 28, 1, false)
	found := false
	for _, e := range s.entries {
		if e != before && e.key.hexQ == before.key.hexQ {
			found = true
		}
	}
	if !found {
		t.Fatalf("changed terrain was not re-rendered")
	}

	// Many rocks and zooms through one Surface stay within its limits.
	for round := 0; round < 4; round++ {
		for i, r := range w.Rocks {
			for k := 0; k < 3; k++ {
				s.Draw(cv, r, c, 9+float32(i%4)+float32(round)*7, 1, false)
			}
		}
	}
	if len(s.entries) > surfaceEntries {
		t.Fatalf("surface holds %d layers, limit %d", len(s.entries), surfaceEntries)
	}
	total := 0
	for _, e := range s.entries {
		b := e.img.Bounds()
		total += b.Dx() * b.Dy()
	}
	if total > surfaceBudgetPx && len(s.entries) > 1 {
		t.Fatalf("surface holds %d pixels in %d layers, budget %d", total, len(s.entries), surfaceBudgetPx)
	}

	// A screen full of thumbnails: every rock keeps its layer, so none is
	// painted twice.
	thumbs := NewSurface()
	defer thumbs.Release()
	for round := 0; round < 3; round++ {
		for _, r := range w.Rocks {
			thumbs.Draw(cv, r, c, 3, 1, false)
		}
	}
	if len(thumbs.entries) != len(w.Rocks) && len(w.Rocks) <= surfaceEntries {
		t.Fatalf("%d thumbnails kept for %d rocks", len(thumbs.entries), len(w.Rocks))
	}
}

func TestBlobOutline(t *testing.T) {
	w := testWorld(t)
	for _, rock := range w.Rocks {
		loops := blobOutline(rock)
		if len(loops) == 0 {
			t.Fatalf("%s: no outline", rock.Name)
		}
		// Every boundary edge of the blob appears exactly once.
		want := 0
		for _, h := range rock.BlobHexes() {
			for d := 0; d < 6; d++ {
				if !rock.InBlob(h.Neighbor(d)) {
					want++
				}
			}
		}
		got := 0
		for _, loop := range loops {
			got += len(loop)
			for i, v := range loop {
				n := loop[(i+1)%len(loop)]
				if d := math.Hypot(float64(n.x-v.x), float64(n.y-v.y)); math.Abs(d-1) > 1e-3 {
					t.Fatalf("%s: outline edge of length %v", rock.Name, d)
				}
				if l := math.Hypot(float64(v.nx), float64(v.ny)); math.Abs(l-1) > 1e-3 {
					t.Fatalf("%s: normal of length %v", rock.Name, l)
				}
			}
		}
		if got != want {
			t.Errorf("%s: outline has %d edges, the blob has %d boundary edges", rock.Name, got, want)
		}
		// The outer loop runs clockwise on screen (positive area with y
		// down) and its normals point away from the rock.
		var area float64
		for i, v := range loops[0] {
			n := loops[0][(i+1)%len(loops[0])]
			area += float64(v.x)*float64(n.y) - float64(n.x)*float64(v.y)
		}
		big := 0
		for i := range loops {
			if len(loops[i]) > len(loops[big]) {
				big = i
			}
		}
		if big == 0 && area <= 0 {
			t.Errorf("%s: outer loop is not clockwise", rock.Name)
		}
		if _, ok := badgeOutline(rock); !ok {
			t.Errorf("%s: no badge outline", rock.Name)
		}
	}
	// An empty rock has no outline and draws nothing.
	if loops := blobOutline(model.NewRock(5, 4)); len(loops) != 0 {
		t.Errorf("empty rock has %d loops", len(loops))
	}
}

func TestSmallPicturesDraw(t *testing.T) {
	defer ReleaseSprites()
	w := testWorld(t)
	for _, scale := range []float32{1, 2} {
		cv := testCanvas(scale)
		for _, rock := range w.Rocks {
			for _, r := range []float32{0, -1, 2, 6, 9.5, 14, 40, 700} {
				RockBadge(cv, rock, 100.3, 80.7, r)
			}
		}
		RockBadge(cv, nil, 10, 10, 8)
		RockBadge(cv, model.NewRock(7, 3), 10, 10, 8)
		for _, r := range []float32{0, 1, 8, 30, 200} {
			for _, tm := range []float32{0, 1.7, 1e6, -4} {
				Star(cv, 200, 200, r, tm)
			}
			for ph := float32(-0.2); ph < 1.3; ph += 0.07 {
				Explosion(cv, 300, 200, r, ph)
			}
			Explosion(cv, 300, 200, r, float32(math.NaN()))
		}
	}
}

func TestBucketPx(t *testing.T) {
	seen := map[float32]bool{}
	prev := float32(0)
	for ps := float32(0.1); ps < 2000; ps += 0.173 {
		b := bucketPx(ps)
		if b < prev {
			t.Fatalf("bucketPx(%v) = %v is below the bucket of a smaller size (%v)", ps, b, prev)
		}
		prev = b
		seen[b] = true
		if b != float32(math.Round(float64(b))) {
			t.Fatalf("bucketPx(%v) = %v is not a whole number of pixels", ps, b)
		}
		if ps >= 8 && ps <= maxUnitPx {
			if e := math.Abs(float64(b-ps)) / float64(ps); e > 0.065 {
				t.Fatalf("bucketPx(%v) = %v is %.1f %% off", ps, b, 100*e)
			}
		}
	}
	if len(seen) > 100 {
		t.Errorf("%d distinct buckets; a continuous zoom should need at most 100", len(seen))
	}
}

// TestSpriteCacheBounded sweeps the size of everything continuously, as a
// long animated zoom would, and checks that the number of sprites kept
// stays within the caches' limits.
func TestSpriteCacheBounded(t *testing.T) {
	ReleaseSprites()
	defer ReleaseSprites()
	th := gfx.Default()
	w := testWorld(t)
	cv := testCanvas(1)
	const limit = 1024 + 256 + 24

	// One kind, one look: the sprites needed are bounded by the buckets.
	for size := float32(4); size < 400; size *= 1.003 {
		Building(cv, model.Keystone, 100, 100, size, BuildingLook{Accent: th.FactionColor(1), Progress: 1})
	}
	if n := CachedSprites(); n > 100 {
		t.Errorf("sweeping one building over all sizes cached %d sprites", n)
	}

	peak := 0
	i := 0
	for size := float32(5); size < 260; size *= 1.021 {
		for _, kind := range model.AllBuildingKinds() {
			look := BuildingLook{Accent: th.FactionColor(1 + i%4), Progress: 1, Time: size}
			switch i % 5 {
			case 1:
				look.Off = true
			case 2:
				look.Damage = 0.6
			case 3:
				look.Progress = 0.5
			case 4:
				look.Derelict = true
			}
			Building(cv, kind, 200, 150, size, look)
			i++
		}
		for _, kind := range model.AllShipKinds() {
			Ship(cv, kind, 200, 150, size, size*0.37, th.FactionColor(i%5))
		}
		for _, g := range model.AllGoods() {
			Good(cv, g, gfx.R(10, 10, size, size))
		}
		for id := IconID(0); id < NumIcons; id++ {
			Icon(cv, id, gfx.R(10, 10, size, size), th.Text)
		}
		for _, rock := range w.Rocks[:4] {
			RockBadge(cv, rock, 50, 50, size)
		}
		peak = max(peak, CachedSprites())
	}
	if peak > limit {
		t.Errorf("sprite caches grew to %d sprites, limit %d", peak, limit)
	}
	if peak == 0 {
		t.Errorf("nothing was cached")
	}
	ReleaseSprites()
	if n := CachedSprites(); n != 0 {
		t.Errorf("%d sprites left after ReleaseSprites", n)
	}
}

// BenchmarkColonyFrame measures what the art costs the game's Draw callback
// for a busy frame: a full rock surface with the deposit overlay and 150
// running buildings. (It times the recording of the draw commands; the demo
// program's -bench mode times the same scene inside a real game loop.)
func BenchmarkColonyFrame(b *testing.B) {
	w := testWorld(b)
	rock := w.Rock(w.Human().HomeRock)
	th := gfx.Default()
	dst := ebiten.NewImage(1600, 900)
	cv := gfx.NewCanvas(dst, 1)
	s := NewSurface()
	defer s.Release()
	defer ReleaseSprites()
	kinds := model.AllBuildingKinds()
	var hexes []model.Hex
	for i := range rock.Tiles {
		if h, ok := rock.HexAt(i); ok && rock.Tiles[i].Terrain == model.TerrainRegolith {
			hexes = append(hexes, h)
		}
	}
	frame := func(t float32) {
		c := gfx.P(800, 450)
		s.Draw(cv, rock, c, 16, model.PlayerFaction, true)
		for n := 0; n < 150; n++ {
			v := model.HexToPixel(hexes[n%len(hexes)], 16)
			look := BuildingLook{Accent: th.FactionColor(1 + n%4), Progress: 1, Time: t + float32(n)*0.37}
			Building(cv, kinds[n%len(kinds)], c.X+float32(v.X), c.Y+float32(v.Y), 16, look)
		}
	}
	frame(0)
	frame(0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%64 == 0 {
			// Outside a game loop nothing consumes the recorded commands;
			// clearing the target now and then keeps the queue short.
			b.StopTimer()
			dst.Clear()
			b.StartTimer()
		}
		frame(float32(i) / 60)
	}
}
