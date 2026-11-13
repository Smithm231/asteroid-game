package model

import (
	"errors"
	"math"
	"slices"
	"testing"
)

// testRock returns a rock of the given radius whose whole disc is Regolith.
func testRock(id RockID, radius int) *Rock {
	r := NewRock(id, radius)
	for i := range r.Tiles {
		if _, ok := r.HexAt(i); ok {
			r.Tiles[i].Terrain = TerrainRegolith
		}
	}
	r.OrbitR = 500
	r.Solar = SolarFactor(500)
	return r
}

func TestFactionMask(t *testing.T) {
	var m FactionMask
	for f := FactionID(0); f < 10; f++ {
		if m.Has(f) {
			t.Errorf("empty mask has faction %d", f)
		}
	}
	m.Set(1)
	m.Set(7)
	m.Set(0) // ignored
	m.Set(8) // ignored: beyond MaxFactions
	if !m.Has(1) || !m.Has(7) || m.Has(0) || m.Has(8) || m.Has(3) {
		t.Errorf("mask = %08b", m)
	}
	if m != 0b10000010 {
		t.Errorf("mask bits = %08b, want bit n for faction n", m)
	}
	if got := m.With(3); !got.Has(3) || m.Has(3) {
		t.Error("With must return a new mask and leave the receiver alone")
	}
	if got := m.Without(1); got.Has(1) || !m.Has(1) {
		t.Error("Without must return a new mask and leave the receiver alone")
	}
	m.Clear(7)
	if m.Has(7) || !m.Has(1) {
		t.Errorf("after Clear(7): %08b", m)
	}
	if !FactionID(1).Valid() || !FactionID(7).Valid() || FactionID(0).Valid() || FactionID(8).Valid() {
		t.Error("FactionID.Valid is wrong")
	}
}

func TestTileIndexing(t *testing.T) {
	if TileCount(-1) != 0 || TileCount(0) != 1 || TileCount(7) != 225 || TileCount(11) != 529 {
		t.Error("TileCount is wrong")
	}
	for radius := 0; radius <= RockRadiusMax; radius++ {
		r := NewRock(3, radius)
		if len(r.Tiles) != TileCount(radius) {
			t.Fatalf("radius %d: %d tiles, want %d", radius, len(r.Tiles), TileCount(radius))
		}
		w := 2*radius + 1
		seen := map[int]bool{}
		inDisc := 0
		// Scan a generous square around the rock.
		for rr := -radius - 3; rr <= radius+3; rr++ {
			for q := -radius - 3; q <= radius+3; q++ {
				h := Hex{q, rr}
				idx := r.Index(h)
				if h.Len() > radius {
					if idx != -1 || r.TileAt(h) != nil || r.InBounds(h) || r.InBlob(h) {
						t.Fatalf("radius %d: %v is outside the disc but has index %d", radius, h, idx)
					}
					continue
				}
				inDisc++
				if want := (rr+radius)*w + (q + radius); idx != want {
					t.Fatalf("radius %d: Index(%v) = %d, want %d", radius, h, idx, want)
				}
				if idx < 0 || idx >= len(r.Tiles) {
					t.Fatalf("radius %d: Index(%v) = %d out of range", radius, h, idx)
				}
				if seen[idx] {
					t.Fatalf("radius %d: index %d used twice", radius, idx)
				}
				seen[idx] = true
				if r.TileAt(h) != &r.Tiles[idx] {
					t.Fatalf("radius %d: TileAt(%v) does not alias Tiles[%d]", radius, h, idx)
				}
				if back, ok := r.HexAt(idx); !ok || back != h {
					t.Fatalf("radius %d: HexAt(%d) = %v,%v, want %v", radius, idx, back, ok, h)
				}
			}
		}
		if inDisc != HexCount(radius) {
			t.Fatalf("radius %d: %d hexes in the disc, want %d", radius, inDisc, HexCount(radius))
		}
		// Unused corners of the bounding square have no hex.
		unused := 0
		for i := range r.Tiles {
			if _, ok := r.HexAt(i); !ok {
				unused++
				if seen[i] {
					t.Fatalf("radius %d: index %d is both used and unused", radius, i)
				}
			}
		}
		if unused != TileCount(radius)-HexCount(radius) {
			t.Fatalf("radius %d: %d unused entries, want %d", radius, unused, TileCount(radius)-HexCount(radius))
		}
		// Hexes() lists the disc in strictly increasing index order.
		hexes := r.Hexes()
		if len(hexes) != HexCount(radius) {
			t.Fatalf("radius %d: Hexes() has %d entries", radius, len(hexes))
		}
		prev := -1
		for _, h := range hexes {
			idx := r.Index(h)
			if idx <= prev {
				t.Fatalf("radius %d: Hexes() is not in index order at %v", radius, h)
			}
			prev = idx
		}
		if _, ok := r.HexAt(-1); ok {
			t.Error("HexAt(-1) must fail")
		}
		if _, ok := r.HexAt(len(r.Tiles)); ok {
			t.Error("HexAt(len) must fail")
		}
	}
	// A fresh rock is all Void without deposits.
	r := NewRock(0, 4)
	for i := range r.Tiles {
		if r.Tiles[i].Terrain != TerrainVoid || r.Tiles[i].Deposit != GoodNone || r.Tiles[i].HasDeposit() {
			t.Fatalf("fresh tile %d = %+v", i, r.Tiles[i])
		}
	}
	if r.BlobSize() != 0 || len(r.BlobHexes()) != 0 || r.InBlob(Hex{}) {
		t.Error("a fresh rock must have an empty blob")
	}
	// Nil rocks are inert.
	var none *Rock
	if none.Index(Hex{}) != -1 || none.TileAt(Hex{}) != nil || none.InBlob(Hex{}) || none.Hexes() != nil ||
		none.BlobHexes() != nil || none.BuildingAt(Hex{}) != nil || none.BuildingByID(1) != nil ||
		none.CountKind(Silo, false) != 0 || none.Keystone() != nil || none.HasColony() || none.OwnedBy(1) ||
		none.RemoveBuilding(1) != nil || none.Surveyed(1) || none.RevealAll(1) != 0 || none.BlobSize() != 0 {
		t.Error("nil rock helpers must be inert")
	}
	if err := none.AddBuilding(&Building{ID: 1}); !errors.Is(err, ErrBadBuilding) {
		t.Errorf("AddBuilding on a nil rock: %v", err)
	}
}

func TestBlobAndTiles(t *testing.T) {
	r := NewRock(0, 3)
	solid := []Hex{{0, 0}, {1, 0}, {0, 1}, {-1, 1}, {2, -1}}
	for _, h := range solid {
		r.TileAt(h).Terrain = TerrainRegolith
	}
	r.TileAt(Hex{2, -1}).Terrain = TerrainCrag
	if r.BlobSize() != len(solid) {
		t.Errorf("BlobSize = %d, want %d", r.BlobSize(), len(solid))
	}
	blob := r.BlobHexes()
	if len(blob) != len(solid) {
		t.Fatalf("BlobHexes = %v", blob)
	}
	for i := 1; i < len(blob); i++ {
		if r.Index(blob[i-1]) >= r.Index(blob[i]) {
			t.Errorf("BlobHexes not in index order: %v", blob)
		}
	}
	for _, h := range solid {
		if !r.InBlob(h) {
			t.Errorf("%v should be in the blob", h)
		}
	}
	if r.InBlob(Hex{-3, 0}) || r.InBlob(Hex{9, 9}) {
		t.Error("void and out-of-bounds hexes are not in the blob")
	}

	tile := r.TileAt(Hex{1, 0})
	if !tile.Buildable() {
		t.Error("empty Regolith must be buildable")
	}
	if r.TileAt(Hex{2, -1}).Buildable() || r.TileAt(Hex{-3, 0}).Buildable() || (*Tile)(nil).Buildable() {
		t.Error("Crag, Void and nil tiles are not buildable")
	}
	tile.Building = 5
	if tile.Buildable() {
		t.Error("an occupied tile is not buildable")
	}
	tile.Building = 0

	// Deposits and knowledge.
	tile.Deposit, tile.Amount = Ore, 700
	if !tile.HasDeposit() {
		t.Error("tile with Ore must have a deposit")
	}
	if g, amt, ok := r.VisibleDeposit(Hex{1, 0}, 1); ok || g != GoodNone || amt != 0 {
		t.Error("hidden deposit must not be visible")
	}
	if !tile.Reveal(1) || tile.Reveal(1) {
		t.Error("Reveal must report true exactly once")
	}
	if tile.Reveal(0) || tile.Reveal(9) {
		t.Error("Reveal must ignore invalid factions")
	}
	if !tile.KnownTo(1) || tile.KnownTo(2) || !r.KnownTo(Hex{1, 0}, 1) || r.KnownTo(Hex{1, 0}, 2) {
		t.Error("knowledge is per faction")
	}
	if g, amt, ok := r.VisibleDeposit(Hex{1, 0}, 1); !ok || g != Ore || amt != 700 {
		t.Errorf("VisibleDeposit = %v, %v, %v", g, amt, ok)
	}
	tile.Amount = 0
	if tile.HasDeposit() {
		t.Error("a depleted tile has no deposit")
	}
	if _, _, ok := r.VisibleDeposit(Hex{1, 0}, 1); ok {
		t.Error("a depleted deposit is not visible")
	}
	if r.KnownTo(Hex{50, 50}, 1) {
		t.Error("out-of-bounds hexes are never known")
	}
}

func TestRevealHelpers(t *testing.T) {
	r := testRock(0, 5)
	// Punch a hole so that Void tiles are skipped.
	r.TileAt(Hex{1, 0}).Terrain = TerrainVoid

	if got, want := r.RevealWithin(Hex{0, 0}, 1, 2), 6; got != want {
		t.Errorf("RevealWithin radius 1 revealed %d tiles, want %d", got, want)
	}
	if got := r.RevealWithin(Hex{0, 0}, 1, 2); got != 0 {
		t.Errorf("second RevealWithin revealed %d tiles, want 0", got)
	}
	if got, want := r.RevealWithin(Hex{0, 0}, 3, 2), HexCount(3)-HexCount(1); got != want {
		t.Errorf("RevealWithin radius 3 revealed %d more tiles, want %d", got, want)
	}
	if r.TileAt(Hex{1, 0}).KnownTo(2) {
		t.Error("Void tiles must not be revealed")
	}
	if r.RevealWithin(Hex{0, 0}, 3, 0) != 0 {
		t.Error("RevealWithin for NoFaction must do nothing")
	}
	// Near the edge only tiles on the rock count.
	edge := Hex{5, 0}
	inRange := 0
	for _, h := range edge.WithinRadius(2) {
		if r.InBlob(h) && !r.KnownTo(h, 3) {
			inRange++
		}
	}
	if got := r.RevealWithin(edge, 2, 3); got != inRange {
		t.Errorf("RevealWithin at the edge revealed %d, want %d", got, inRange)
	}

	// NearestHidden walks outward in Spiral order.
	r2 := testRock(1, 4)
	centre := Hex{0, 0}
	var order []Hex
	for {
		h, ok := r2.NearestHidden(centre, 2, 1)
		if !ok {
			break
		}
		order = append(order, h)
		r2.TileAt(h).Reveal(1)
	}
	if want := centre.Spiral(2); !slices.Equal(order, want) {
		t.Errorf("NearestHidden order = %v, want %v", order, want)
	}
	if _, ok := r2.NearestHidden(centre, 2, 1); ok {
		t.Error("nothing should be hidden within radius 2 any more")
	}
	if h, ok := r2.NearestHidden(centre, 3, 1); !ok || centre.Dist(h) != 3 {
		t.Errorf("NearestHidden radius 3 = %v, %v", h, ok)
	}
	// Other factions still see nothing.
	if h, ok := r2.NearestHidden(centre, 2, 2); !ok || h != centre {
		t.Errorf("NearestHidden for another faction = %v, %v", h, ok)
	}

	// RevealAll marks the rock surveyed.
	total := r2.BlobSize()
	if got := r2.RevealAll(2); got != total {
		t.Errorf("RevealAll revealed %d tiles, want %d", got, total)
	}
	if !r2.Surveyed(2) || r2.Surveyed(1) {
		t.Error("only faction 2 has surveyed the rock")
	}
	if _, ok := r2.NearestHidden(centre, 4, 2); ok {
		t.Error("a surveyed rock has no hidden tiles")
	}
	if got := r2.RevealAll(2); got != 0 {
		t.Errorf("second RevealAll revealed %d tiles", got)
	}
}

func TestBuildingBookkeeping(t *testing.T) {
	r := testRock(2, 4)
	r.TileAt(Hex{3, 0}).Terrain = TerrainVoid
	r.Owner = 1
	r.Colony = NewColony(0)

	// Insert in scrambled ID order.
	specs := []struct {
		id   BuildingID
		kind BuildingKind
		hex  Hex
	}{
		{7, Silo, Hex{1, 0}},
		{2, Keystone, Hex{0, 0}},
		{11, BoreRig, Hex{0, 1}},
		{5, Silo, Hex{-1, 0}},
		{9, Sunvane, Hex{0, -1}},
		{1, Conduit, Hex{1, -1}},
	}
	for _, s := range specs {
		b := NewActiveBuilding(s.id, s.kind, 1, s.hex, 1)
		if err := r.AddBuilding(b); err != nil {
			t.Fatalf("AddBuilding(%d): %v", s.id, err)
		}
	}
	wantIDs := []BuildingID{1, 2, 5, 7, 9, 11}
	gotIDs := func() []BuildingID {
		var ids []BuildingID
		for _, b := range r.Buildings {
			ids = append(ids, b.ID)
		}
		return ids
	}
	if got := gotIDs(); !slices.Equal(got, wantIDs) {
		t.Fatalf("building IDs = %v, want %v (sorted)", got, wantIDs)
	}
	for _, s := range specs {
		if got := r.TileAt(s.hex).Building; got != s.id {
			t.Errorf("tile %v references building %d, want %d", s.hex, got, s.id)
		}
		b := r.BuildingAt(s.hex)
		if b == nil || b.ID != s.id || b.Kind != s.kind {
			t.Errorf("BuildingAt(%v) = %+v", s.hex, b)
		}
		if r.BuildingByID(s.id) != b {
			t.Errorf("BuildingByID(%d) differs from BuildingAt", s.id)
		}
	}
	if r.BuildingAt(Hex{2, 0}) != nil || r.BuildingAt(Hex{40, 0}) != nil {
		t.Error("BuildingAt must be nil for empty and out-of-bounds tiles")
	}
	if r.BuildingByID(3) != nil || r.BuildingByID(0) != nil || r.BuildingByID(-4) != nil || r.BuildingByID(99) != nil {
		t.Error("BuildingByID must be nil for unknown IDs")
	}

	// Rejected additions change nothing.
	bad := []struct {
		name string
		b    *Building
		want error
	}{
		{"nil", nil, ErrBadBuilding},
		{"no ID", &Building{ID: 0, Hex: Hex{2, 0}}, ErrBadBuilding},
		{"negative ID", &Building{ID: -3, Hex: Hex{2, 0}}, ErrBadBuilding},
		{"out of bounds", &Building{ID: 20, Hex: Hex{9, 9}}, ErrBadHex},
		{"void tile", &Building{ID: 20, Hex: Hex{3, 0}}, ErrBadHex},
		{"occupied tile", &Building{ID: 20, Hex: Hex{0, 0}}, ErrTileOccupied},
		{"duplicate ID", &Building{ID: 7, Hex: Hex{2, 0}}, ErrBadBuilding},
	}
	for _, tc := range bad {
		if err := r.AddBuilding(tc.b); !errors.Is(err, tc.want) {
			t.Errorf("AddBuilding(%s) = %v, want %v", tc.name, err, tc.want)
		}
	}
	if got := gotIDs(); !slices.Equal(got, wantIDs) {
		t.Fatalf("rejected additions changed the list: %v", got)
	}
	if r.TileAt(Hex{2, 0}).Building != 0 {
		t.Error("a rejected addition marked a tile")
	}

	// Counting.
	if got := r.CountKind(Silo, false); got != 2 {
		t.Errorf("CountKind(Silo, all) = %d, want 2", got)
	}
	r.BuildingByID(5).Enabled = false
	r.BuildingByID(7).State = StateConstructing
	if got := r.CountKind(Silo, true); got != 0 {
		t.Errorf("CountKind(Silo, active) = %d, want 0 (one disabled, one under construction)", got)
	}
	r.BuildingByID(5).Enabled = true
	if got := r.CountKind(Silo, true); got != 1 {
		t.Errorf("CountKind(Silo, active) = %d, want 1", got)
	}
	if got := r.CountKind(LanceTower, false); got != 0 {
		t.Errorf("CountKind(LanceTower) = %d", got)
	}
	if b := r.FirstOfKind(Silo, false); b == nil || b.ID != 5 {
		t.Errorf("FirstOfKind(Silo, all) = %+v, want building 5", b)
	}
	if b := r.FirstOfKind(Silo, true); b == nil || b.ID != 5 {
		t.Errorf("FirstOfKind(Silo, active) = %+v, want building 5", b)
	}
	if r.FirstOfKind(MassDriver, false) != nil {
		t.Error("FirstOfKind must be nil when there is none")
	}
	if k := r.Keystone(); k == nil || k.ID != 2 {
		t.Errorf("Keystone() = %+v", k)
	}
	if !r.HasColony() || !r.OwnedBy(1) || r.OwnedBy(2) || r.OwnedBy(0) {
		t.Error("ownership helpers are wrong")
	}

	// Adjacency to own buildings.
	if !r.AdjacentToOwned(Hex{2, 0}, 1) {
		t.Error("(2,0) touches the Silo at (1,0)")
	}
	if r.AdjacentToOwned(Hex{2, 0}, 2) || r.AdjacentToOwned(Hex{2, 0}, 0) {
		t.Error("adjacency must be per owner")
	}
	if r.AdjacentToOwned(Hex{-3, 3}, 1) {
		t.Error("(-3,3) touches nothing")
	}

	// Removal.
	if r.RemoveBuilding(3) != nil || r.RemoveBuilding(0) != nil {
		t.Error("removing an unknown building must return nil")
	}
	removed := r.RemoveBuilding(7)
	if removed == nil || removed.ID != 7 {
		t.Fatalf("RemoveBuilding(7) = %+v", removed)
	}
	if r.TileAt(Hex{1, 0}).Building != 0 {
		t.Error("RemoveBuilding left the tile reference behind")
	}
	if r.BuildingByID(7) != nil || r.BuildingAt(Hex{1, 0}) != nil {
		t.Error("removed building is still found")
	}
	if got, want := gotIDs(), []BuildingID{1, 2, 5, 9, 11}; !slices.Equal(got, want) {
		t.Errorf("after removal: %v, want %v", got, want)
	}
	// The freed tile and ID can be used again.
	if err := r.AddBuilding(NewBuilding(7, Habitat, 1, Hex{1, 0}, 1)); err != nil {
		t.Errorf("re-adding on the freed tile: %v", err)
	}
	// Remove everything, first and last included.
	for _, id := range []BuildingID{1, 11, 5, 2, 9, 7} {
		if r.RemoveBuilding(id) == nil {
			t.Errorf("RemoveBuilding(%d) failed", id)
		}
	}
	if len(r.Buildings) != 0 {
		t.Errorf("buildings left: %v", gotIDs())
	}
	for i := range r.Tiles {
		if r.Tiles[i].Building != 0 {
			t.Fatalf("tile %d still references building %d", i, r.Tiles[i].Building)
		}
	}
}

func TestBuildingLifecycle(t *testing.T) {
	b := NewBuilding(4, FlakBattery, 2, Hex{1, 1}, 1.2)
	if b.ID != 4 || b.Kind != FlakBattery || b.Owner != 2 || b.Hex != (Hex{1, 1}) {
		t.Fatalf("NewBuilding = %+v", b)
	}
	if math.Abs(b.MaxHP-420) > 1e-9 || math.Abs(b.HP-84) > 1e-9 {
		t.Errorf("HP = %v / %v, want 84 / 420", b.HP, b.MaxHP)
	}
	if b.State != StateConstructing || b.Progress != 0 || !b.Enabled || b.Priority != PriorityHigh || b.DriverTarget != NoRock {
		t.Errorf("NewBuilding defaults = %+v", b)
	}
	if !b.Constructing() || b.Completed() || b.Operational() || b.Working() || b.Derelict() || b.Damaged() {
		t.Error("state predicates are wrong for a construction site")
	}
	if got := b.RemainingBuildSeconds(); got != 30 {
		t.Errorf("RemainingBuildSeconds = %v, want 30", got)
	}
	b.Progress = 0.1
	if got := b.ConstructionHP(); math.Abs(got-84) > 1e-9 {
		t.Errorf("ConstructionHP at 10%% = %v, want the 20%% floor (84)", got)
	}
	b.Progress = 0.6
	if got := b.ConstructionHP(); math.Abs(got-252) > 1e-9 {
		t.Errorf("ConstructionHP at 60%% = %v, want 252", got)
	}
	if got := b.RemainingBuildSeconds(); got < 11.999 || got > 12.001 {
		t.Errorf("RemainingBuildSeconds = %v, want 12", got)
	}

	b.Complete()
	if !b.Completed() || !b.Operational() || b.Constructing() || b.Progress != 1 || b.RemainingBuildSeconds() != 0 {
		t.Errorf("after Complete: %+v", b)
	}
	if b.Working() {
		t.Error("a building has not worked before the economy evaluated it")
	}
	b.Eff = 0.8
	if !b.Working() {
		t.Error("an operational building with Eff > 0 is working")
	}
	b.Enabled = false
	if b.Operational() || b.Working() {
		t.Error("a disabled building is neither operational nor working")
	}
	b.Enabled = true
	if !b.Damaged() || math.Abs(b.HPFrac()-0.2) > 1e-9 {
		t.Errorf("Damaged = %v, HPFrac = %v", b.Damaged(), b.HPFrac())
	}

	a := NewActiveBuilding(5, MassDriver, 1, Hex{}, 0)
	if a.HP != 350 || a.MaxHP != 350 || !a.Operational() || a.Damaged() || a.HPFrac() != 1 {
		t.Errorf("NewActiveBuilding = %+v", a)
	}
	if a.Def().Kind != MassDriver || a.Priority != PriorityHigh {
		t.Errorf("Def/Priority wrong: %+v", a)
	}
	a.Shield, a.Cooldown, a.DriverTarget, a.Queue, a.Target, a.Eff = 50, 3, 2, []ShipKind{Dart}, 9, 1
	a.MakeDerelict()
	if !a.Derelict() || a.Owner != NoFaction || a.Operational() || a.Shield != 0 || a.Cooldown != 0 ||
		a.DriverTarget != NoRock || a.Queue != nil || a.Target != 0 || a.Eff != 0 || a.HP != 350 {
		t.Errorf("after MakeDerelict: %+v", a)
	}

	if NewBuilding(1, Sunvane, 1, Hex{}, 1).Priority != PriorityNormal {
		t.Error("a Sunvane defaults to normal priority")
	}
	if NewBuilding(1, Greenhouse, 1, Hex{}, 1).Priority != PriorityHigh {
		t.Error("a Greenhouse (Life) defaults to high priority")
	}

	var none *Building
	if none.Completed() || none.Operational() || none.Working() || none.Constructing() || none.Derelict() ||
		none.Damaged() || none.HPFrac() != 0 || none.ConstructionHP() != 0 || none.RemainingBuildSeconds() != 0 ||
		none.Def().Kind != BuildingNone {
		t.Error("nil building helpers must be inert")
	}
	none.Complete()
	none.MakeDerelict()
}
