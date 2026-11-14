package art

import (
	"math"
	"testing"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// measure draws the body of a kind, and its motion layer if asked — as it
// runs on screen at different moments, and as it is baked at rest — with a
// pen that takes note of every projected point.
func measure(kind model.BuildingKind, unitPx float32, motion bool) extent {
	var e extent
	cv := testCanvas(1)
	b := bpen{pen: newPen(cv, 300, 300, unitPx), unit: unitPx, lit: true, ext: &e}
	b.acc = matOf(gfx.RGB(0x3DC9B3))
	art := structArts[kind]
	art.body(&b)
	if motion && art.motion != nil {
		b.live = true
		for tm := float32(0); tm < 40; tm += 0.21 {
			b.t = tm
			art.motion(&b)
		}
		b.live, b.lit, b.t = false, false, 0
		art.motion(&b)
	}
	return e
}

// TestBuildingTop checks the heights the surface view places status marks
// by: every kind has one, none is taller than 1.6 tile radii, it scales
// with the tile, and it is honest — it is how far the structure, moving
// parts included, reaches above the ground centre of its tile on screen
// (light and smoke aside).
func TestBuildingTop(t *testing.T) {
	// The heights in the table were taken at the classic slope; at another
	// Tilt the far parts of a structure sit a little higher or lower.
	over, under := float32(0.005), float32(0.07)
	if Tilt != 0.5 {
		over, under = 0.2, 0.3
	}
	for _, kind := range model.AllBuildingKinds() {
		top := structArts[kind].top
		if !(top > 0.15 && top <= 1.6) {
			t.Errorf("%v: top %.2f is outside 0.15..1.6 tile radii", kind, top)
		}
		for _, size := range []float32{14, 22, 30, 61.5} {
			if got, want := BuildingTop(kind, size), top*size; math.Abs(float64(got-want)) > 1e-3 {
				t.Errorf("BuildingTop(%v, %v) = %v, want %v", kind, size, got, want)
			}
		}
		for _, unitPx := range []float32{10, 22, 60} {
			e := measure(kind, unitPx, true)
			if e.rise > top+over || e.rise < top-under {
				t.Errorf("%v at %v px: the structure rises %.3f on screen, its declared top is %.2f", kind, unitPx, e.rise, top)
			}
		}
	}
	// The flat kinds stay low (the far vane of a Sunvane is what reaches
	// highest on screen, not anything tall), the masts and towers do not.
	for kind, most := range map[model.BuildingKind]float32{model.Conduit: 0.4, model.Sunvane: 0.7} {
		if top := structArts[kind].top; top > most {
			t.Errorf("%v should lie nearly flat, top is %.2f", kind, top)
		}
	}
	tall := []model.BuildingKind{model.SurveyMast, model.LanceTower, model.ThermalStack, model.FarseerArray}
	for _, kind := range tall {
		if top := structArts[kind].top; top < 1.4 {
			t.Errorf("%v should be tall, top is %.2f", kind, top)
		}
	}
	// Kinds outside the table still get a usable answer.
	for _, kind := range []model.BuildingKind{model.BuildingNone, model.NumBuildingKinds} {
		if got := BuildingTop(kind, 20); !(got > 0 && got < 40) {
			t.Errorf("BuildingTop(%v, 20) = %v", kind, got)
		}
	}
}

// TestStructureExtents checks that every structure stands inside its tile
// and inside the sprite it is rendered into, motion layer included.
func TestStructureExtents(t *testing.T) {
	for _, kind := range model.AllBuildingKinds() {
		for _, unitPx := range []float32{10, 22, 60} {
			body := measure(kind, unitPx, false)
			if body.n == 0 {
				t.Fatalf("%v draws nothing", kind)
			}
			if body.ground > 0.02 {
				t.Errorf("%v at %v px: reaches %.2f beyond its tile on the ground", kind, unitPx, body.ground)
			}
			all := measure(kind, unitPx, true)
			const edge = 0.04 // room for outlines
			if all.minX < -structHalfW+edge || all.maxX > structHalfW-edge {
				t.Errorf("%v at %v px: spans x %.2f..%.2f, the sprite ±%.2f", kind, unitPx, all.minX, all.maxX, structHalfW)
			}
			if -all.minY > structAbove-edge || all.maxY > structBelow-edge {
				t.Errorf("%v at %v px: reaches %.2f up and %.2f down, the sprite %.2f and %.2f",
					kind, unitPx, -all.minY, all.maxY, structAbove, structBelow)
			}
			// Nothing of a structure hangs below the near corner of its
			// tile.
			if all.maxY > Tilt+0.02 {
				t.Errorf("%v at %v px: reaches %.2f below the ground centre, the tile ends at %.2f", kind, unitPx, all.maxY, Tilt)
			}
		}
		if structHitsOf(kind) == 0 {
			t.Errorf("%v records no damage points", kind)
		}
	}
}

// structHitsOf draws the body of a kind and returns how many damage points
// it recorded.
func structHitsOf(kind model.BuildingKind) int {
	cv := testCanvas(1)
	b := bpen{pen: newPen(cv, 300, 300, 22), unit: 22}
	structArts[kind].body(&b)
	return b.nhits
}

// TestStructureStates draws every kind in every state at the sizes the
// surface view uses and checks what ends up in the caches: the body of a
// running structure is one sprite however long it runs.
func TestStructureStates(t *testing.T) {
	ReleaseSprites()
	defer ReleaseSprites()
	cv := testCanvas(1)
	acc := gfx.RGB(0xE8A33D)
	for _, kind := range model.AllBuildingKinds() {
		before := CachedSprites()
		for i := 0; i < 200; i++ {
			Building(cv, kind, 100, 100, 22, BuildingLook{Accent: acc, Progress: 1, Time: float32(i) * 0.137})
		}
		if n := CachedSprites() - before; n != 1 {
			t.Errorf("%v: 200 frames of a running structure cached %d sprites, want 1", kind, n)
		}
	}
	ReleaseSprites()
	// A site while it is being built: the drawing, the built part and the
	// plot, whatever the progress.
	for i := 0; i <= 100; i++ {
		Building(cv, model.Smelter, 100, 100, 22, BuildingLook{Accent: acc, Progress: float32(i) / 100, Time: float32(i)})
	}
	if n := CachedSprites(); n > 4 {
		t.Errorf("a building site cached %d sprites", n)
	}
	// Damage is baked in four levels.
	ReleaseSprites()
	for i := 0; i <= 100; i++ {
		Building(cv, model.Silo, 100, 100, 22, BuildingLook{Accent: acc, Progress: 1, Damage: float32(i) / 100, Time: float32(i)})
	}
	if n := CachedSprites(); n > 4 {
		t.Errorf("damage levels cached %d sprites, want at most 4", n)
	}
	// Icons at the sizes of the build palette.
	for _, kind := range model.AllBuildingKinds() {
		for _, side := range []float32{36, 48, 60} {
			BuildingIcon(cv, kind, gfx.R(10, 10, side, side), acc)
			BuildingIcon(cv, kind, gfx.R(10, 10, side*1.6, side), acc)
		}
	}
}

// TestLiveStamps checks the little pictures the motion layers are stamped
// from.
func TestLiveStamps(t *testing.T) {
	defer ReleaseSprites()
	prev := 0
	for px := float32(0); px < 400; px += 0.37 {
		i := stampIndex(px)
		if i < prev {
			t.Fatalf("stampIndex(%v) = %d is below that of a smaller mark (%d)", px, i, prev)
		}
		prev = i
		if n := stampLadder[i]; n < px && i != len(stampLadder)-1 {
			t.Fatalf("a mark of %v px gets a stamp of %v px", px, n)
		}
	}
	for _, c := range [][3]float32{{8, 6, 0}, {8, 6, 1}, {8, 0, 1}, {128, 3, 40}, {2, 2, 0.1}} {
		if got := casingFor(c[0], c[1], c[2]); got < 0 || got > maxCasing {
			t.Errorf("casingFor(%v, %v, %v) = %d", c[0], c[1], c[2], got)
		}
	}
	if casingFor(8, 6, 0) != 0 {
		t.Errorf("a mark without a casing got one")
	}
	// Every mark at every size, on canvases of different scales.
	for _, scale := range []float32{1, 1.5, 2} {
		cv := testCanvas(scale)
		for _, unit := range []float32{0.5, 4, 14, 22, 30, 110, 900} {
			b := bpen{pen: newPen(cv, 100, 100, unit), unit: unit * scale, lit: true, live: true}
			b.acc = matOf(gfx.RGB(0x3DC9B3))
			for _, r := range []float32{0, 0.001, 0.03, 0.2, 1.5} {
				b.dotAt(0, 0, 0.5, r, colLamp)
				b.knobAt(0.1, 0, 0.5, r, colLamp)
				b.puffAt(0.1, 0.2, 0.5, r, colLamp)
				b.disc(colLamp, 0, 0, 0.2, r)
				b.hoop(colLamp, 0.03, 1, 0, 0, 0, r)
				b.beam(colLamp, r, 0, 0, 0, 0.5, 0.2, 0.8)
				b.rod(colLamp, r, 1, 0, 0, 0, 0.5, 0.2, 0.8)
				b.rod(colLamp, r, 1, 0.3, 0.3, 0.3, 0.3, 0.3, 0.3) // no length
				b.ovalAt(colLamp, true, 0, 0, 1, [3]float32{r, 0, 0}, [3]float32{0, r * 0.3, r})
				b.ovalAt(colLamp, false, 0, 0, 1, [3]float32{r, 0, 0}, [3]float32{r, 0, 0}) // edge on
			}
		}
	}
	made := 0
	for i := range discStamps {
		for j := range discStamps[i] {
			if discStamps[i][j] != nil {
				made++
			}
			if barStamps[i][j] != nil {
				made++
			}
		}
		if ringStamps[i] != nil {
			made++
		}
	}
	if made == 0 {
		t.Fatalf("no stamps were made")
	}
	ReleaseSprites()
	for i := range discStamps {
		for j := range discStamps[i] {
			if discStamps[i][j] != nil || barStamps[i][j] != nil {
				t.Fatalf("stamps left after ReleaseSprites")
			}
		}
		if ringStamps[i] != nil {
			t.Fatalf("stamps left after ReleaseSprites")
		}
	}
}
