package model

import (
	"errors"
	"math"
	"testing"
)

func TestIssuer(t *testing.T) {
	w := buildTestWorld(t)
	if f, err := w.Issuer(1); err != nil || f != w.Faction(1) {
		t.Errorf("Issuer(1) = %v, %v", f, err)
	}
	for _, id := range []FactionID{0, 3, 99} {
		if f, err := w.Issuer(id); f != nil || !errors.Is(err, ErrNoFaction) {
			t.Errorf("Issuer(%d) = %v, %v, want ErrNoFaction", id, f, err)
		}
	}
	w.Faction(2).Alive = false
	if f, err := w.Issuer(2); f != nil || !errors.Is(err, ErrEliminated) {
		t.Errorf("Issuer(eliminated) = %v, %v", f, err)
	}
	w.GameOver = true
	if f, err := w.Issuer(1); f != nil || !errors.Is(err, ErrGameOver) {
		t.Errorf("Issuer after game over = %v, %v", f, err)
	}
	var none *World
	if _, err := none.Issuer(1); err == nil {
		t.Error("Issuer on a nil world must fail")
	}
}

func TestOwnershipChecks(t *testing.T) {
	w := buildTestWorld(t)

	rockTests := []struct {
		f    FactionID
		rock RockID
		want error
	}{
		{1, 0, nil},
		{2, 1, nil},
		{2, 0, ErrNotOwner},
		{1, 2, ErrNotOwner}, // unowned rock
		{0, 2, ErrNotOwner}, // NoFaction owns nothing, not even unowned rocks
		{1, 9, ErrNoRock},
		{1, NoRock, ErrNoRock},
	}
	for _, tc := range rockTests {
		r, err := w.OwnedRock(tc.f, tc.rock)
		if !errors.Is(err, tc.want) {
			t.Errorf("OwnedRock(%d, %d) error = %v, want %v", tc.f, tc.rock, err, tc.want)
		}
		if (r != nil) != (tc.want == nil) {
			t.Errorf("OwnedRock(%d, %d) rock = %v", tc.f, tc.rock, r)
		}
		if r != nil && (r.ID != tc.rock || r.Colony == nil) {
			t.Errorf("OwnedRock(%d, %d) returned rock %d", tc.f, tc.rock, r.ID)
		}
	}

	// Put a derelict on the player's rock: the owner of the rock may act on
	// it.
	wreck := NewActiveBuilding(w.NewBuildingID(), Habitat, 2, Hex{3, 0}, 1)
	wreck.MakeDerelict()
	if err := w.Rock(0).AddBuilding(wreck); err != nil {
		t.Fatal(err)
	}
	buildingTests := []struct {
		f    FactionID
		rock RockID
		id   BuildingID
		want error
	}{
		{1, 0, 2, nil},        // own Bore Rig
		{1, 0, 4, nil},        // own construction site
		{1, 0, wreck.ID, nil}, // derelict on own rock
		{1, 0, 99, ErrNoBuilding},
		{1, 0, 8, ErrNoBuilding}, // exists, but on rock 1
		{2, 0, 2, ErrNotOwner},
		{1, 1, 8, ErrNotOwner},
		{1, 2, 11, ErrNotOwner}, // derelict on an unowned rock
		{1, 7, 2, ErrNoRock},
	}
	for _, tc := range buildingTests {
		r, b, err := w.OwnedBuilding(tc.f, tc.rock, tc.id)
		if !errors.Is(err, tc.want) {
			t.Errorf("OwnedBuilding(%d, %d, %d) error = %v, want %v", tc.f, tc.rock, tc.id, err, tc.want)
		}
		if tc.want == nil {
			if r == nil || b == nil || r.ID != tc.rock || b.ID != tc.id {
				t.Errorf("OwnedBuilding(%d, %d, %d) = %v, %v", tc.f, tc.rock, tc.id, r, b)
			}
		} else if r != nil || b != nil {
			t.Errorf("OwnedBuilding(%d, %d, %d) returned values with an error", tc.f, tc.rock, tc.id)
		}
	}

	fleetTests := []struct {
		f    FactionID
		id   FleetID
		want error
	}{
		{1, 1, nil},
		{1, 2, nil},
		{2, 3, nil},
		{1, 3, ErrNotOwner},
		{0, 1, ErrNotOwner},
		{1, 44, ErrNoFleet},
		{1, 0, ErrNoFleet},
	}
	for _, tc := range fleetTests {
		fl, err := w.OwnedFleet(tc.f, tc.id)
		if !errors.Is(err, tc.want) {
			t.Errorf("OwnedFleet(%d, %d) error = %v, want %v", tc.f, tc.id, err, tc.want)
		}
		if (fl != nil) != (tc.want == nil) {
			t.Errorf("OwnedFleet(%d, %d) fleet = %v", tc.f, tc.id, fl)
		}
	}
}

func TestFoundAndDissolveColony(t *testing.T) {
	w := buildTestWorld(t)
	w.Faction(1).Mods.HPMul = 1.2
	r2 := w.Rock(2)
	r2.TileAt(Hex{1, 0}).Rubble = true
	r2.TileAt(Hex{-2, 0}).Terrain = TerrainCrag
	nextID := w.NextBuildingID

	// Refused foundings change nothing.
	refused := []struct {
		name string
		rock RockID
		f    FactionID
		hex  Hex
		want error
	}{
		{"missing rock", 9, 1, Hex{}, ErrNoRock},
		{"missing faction", 2, 5, Hex{1, 0}, ErrNoFaction},
		{"claimed rock", 1, 1, Hex{2, 0}, ErrRockClaimed},
		{"own rock", 0, 1, Hex{2, 0}, ErrRockClaimed},
		{"out of bounds", 2, 1, Hex{9, 9}, ErrBadHex},
		{"blocked terrain", 2, 1, Hex{-2, 0}, ErrBadHex},
		{"derelict in the way", 2, 1, Hex{0, 0}, ErrTileOccupied},
	}
	for _, tc := range refused {
		if b, err := w.FoundColony(tc.rock, tc.f, tc.hex); b != nil || !errors.Is(err, tc.want) {
			t.Errorf("%s: FoundColony = %v, %v, want %v", tc.name, b, err, tc.want)
		}
	}
	if r2.Owner != NoFaction || r2.Colony != nil || len(r2.Buildings) != 1 || w.NextBuildingID != nextID {
		t.Fatal("a refused founding changed the world")
	}

	// A successful founding.
	w.Tick = 5000
	k, err := w.FoundColony(2, 1, Hex{1, 0})
	if err != nil {
		t.Fatalf("FoundColony: %v", err)
	}
	if k.Kind != Keystone || k.Owner != 1 || k.Hex != (Hex{1, 0}) || !k.Operational() || k.ID != nextID ||
		math.Abs(k.MaxHP-1440) > 1e-9 || k.HP != k.MaxHP {
		t.Errorf("Keystone = %+v", k)
	}
	if r2.Owner != 1 || r2.Colony == nil || r2.Keystone() != k || r2.BuildingAt(Hex{1, 0}) != k {
		t.Errorf("rock after founding: owner %d, colony %v", r2.Owner, r2.Colony)
	}
	c := r2.Colony
	if c.FoundedTick != 5000 || !c.Stock.IsZero() || c.Pop != 0 || c.Morale != StartMorale || c.StorageCap != 400 {
		t.Errorf("new colony = %+v", c)
	}
	if r2.TileAt(Hex{1, 0}).Rubble {
		t.Error("founding must clear rubble under the Keystone")
	}
	// Tiles within 3 hexes of the Keystone are revealed to the founder
	// (the whole radius-2 rock here), and to nobody else.
	for _, h := range r2.BlobHexes() {
		if !r2.TileAt(h).KnownTo(1) {
			t.Errorf("tile %v was not revealed to the founder", h)
		}
		if r2.TileAt(h).KnownTo(2) {
			t.Errorf("tile %v was revealed to a bystander", h)
		}
	}
	// The derelict that was already there stays a derelict.
	if d := r2.BuildingAt(Hex{0, 0}); d == nil || !d.Derelict() || d.Owner != NoFaction {
		t.Errorf("existing derelict changed: %+v", d)
	}
	if err := w.Validate(); err != nil {
		t.Errorf("world invalid after founding: %v", err)
	}
	if w.ColonyCount(1) != 2 || w.BerthCap(1) != 8 {
		t.Errorf("after founding: %d colonies, %d berths", w.ColonyCount(1), w.BerthCap(1))
	}
	if _, err := w.FoundColony(2, 2, Hex{0, 1}); !errors.Is(err, ErrRockClaimed) {
		t.Errorf("founding on the new colony's rock: %v", err)
	}

	// Losing a colony: remove the Keystone, then dissolve.
	r0 := w.Rock(0)
	before := len(r0.Buildings)
	keystone := r0.Keystone()
	if r0.RemoveBuilding(keystone.ID) == nil {
		t.Fatal("could not remove the Keystone")
	}
	lost, owner := r0.DissolveColony()
	if lost == nil || owner != 1 || lost.Stock[Alloy] != 143.875 {
		t.Errorf("DissolveColony = %+v, %d", lost, owner)
	}
	if r0.Owner != NoFaction || r0.Colony != nil || r0.HasColony() || r0.Keystone() != nil {
		t.Error("the rock must be unowned after its colony fell")
	}
	if len(r0.Buildings) != before-1 {
		t.Errorf("%d buildings left, want %d", len(r0.Buildings), before-1)
	}
	for _, b := range r0.Buildings {
		if !b.Derelict() || b.Owner != NoFaction || b.Operational() || len(b.Queue) != 0 {
			t.Errorf("building %d did not become a derelict: %+v", b.ID, b)
		}
		if r0.TileAt(b.Hex).Building != b.ID {
			t.Errorf("building %d lost its tile", b.ID)
		}
	}
	if w.BerthCap(1) != 4 || w.ColonyCount(1) != 1 {
		t.Errorf("after the fall: %d berths, %d colonies", w.BerthCap(1), w.ColonyCount(1))
	}
	if err := w.Validate(); err != nil {
		t.Errorf("world invalid after the colony fell: %v", err)
	}
	// Dissolving again, or on a bare or nil rock, does nothing.
	if lost, owner := r0.DissolveColony(); lost != nil || owner != NoFaction {
		t.Error("second DissolveColony must do nothing")
	}
	var none *Rock
	if lost, owner := none.DissolveColony(); lost != nil || owner != NoFaction {
		t.Error("DissolveColony on a nil rock must do nothing")
	}
	// The emptied rock can be colonised again, by anyone.
	if _, err := w.FoundColony(0, 2, Hex{2, 0}); err != nil {
		t.Errorf("re-founding on a fallen colony's rock: %v", err)
	}
	if err := w.Validate(); err != nil {
		t.Errorf("world invalid after re-founding: %v", err)
	}
}

func TestRescaleHP(t *testing.T) {
	w := buildTestWorld(t)
	type hp struct{ hp, max float64 }
	snap := func() (b map[BuildingID]hp, s map[ShipID]hp) {
		b, s = map[BuildingID]hp{}, map[ShipID]hp{}
		for _, r := range w.Rocks {
			for _, x := range r.Buildings {
				b[x.ID] = hp{x.HP, x.MaxHP}
			}
		}
		for _, f := range w.Fleets {
			for _, x := range f.Ships {
				s[x.ID] = hp{x.HP, x.MaxHP}
			}
		}
		return b, s
	}
	b0, s0 := snap()
	raidMax := w.Fleet(3).InitialMaxHP

	// No-ops.
	w.RescaleHP(1, 1)
	w.RescaleHP(1, 0)
	w.RescaleHP(1, -2)
	w.RescaleHP(0, 1.2)
	if b, s := snap(); len(b) != len(b0) || len(s) != len(s0) {
		t.Fatal("snapshot size changed")
	} else {
		for id, v := range b {
			if v != b0[id] {
				t.Fatalf("no-op rescale changed building %d", id)
			}
		}
		for id, v := range s {
			if v != s0[id] {
				t.Fatalf("no-op rescale changed ship %d", id)
			}
		}
	}

	w.RescaleHP(1, 1.2)
	b1, s1 := snap()
	for _, r := range w.Rocks {
		for _, x := range r.Buildings {
			factor := 1.0
			if x.Owner == 1 {
				factor = 1.2
			}
			if math.Abs(b1[x.ID].max-b0[x.ID].max*factor) > 1e-9 || math.Abs(b1[x.ID].hp-b0[x.ID].hp*factor) > 1e-9 {
				t.Errorf("building %d (owner %d): %v -> %v, want factor %v", x.ID, x.Owner, b0[x.ID], b1[x.ID], factor)
			}
		}
	}
	for _, f := range w.Fleets {
		for _, x := range f.Ships {
			factor := 1.0
			if f.Owner == 1 {
				factor = 1.2
			}
			if math.Abs(s1[x.ID].max-s0[x.ID].max*factor) > 1e-9 || math.Abs(s1[x.ID].hp-s0[x.ID].hp*factor) > 1e-9 {
				t.Errorf("ship %d (owner %d): %v -> %v, want factor %v", x.ID, f.Owner, s0[x.ID], s1[x.ID], factor)
			}
		}
	}
	// Health fractions are preserved: the damaged Bore Rig keeps its ratio.
	rig := w.Building(0, 2)
	if got, want := rig.HPFrac(), b0[2].hp/b0[2].max; math.Abs(got-want) > 1e-12 {
		t.Errorf("Bore Rig health fraction changed: %v -> %v", want, got)
	}
	// Derelicts (owner 0) and the other faction are untouched, including
	// the rival fleet's retreat baseline.
	if w.Fleet(3).InitialMaxHP != raidMax {
		t.Error("the rival fleet's InitialMaxHP changed")
	}
	w.RescaleHP(2, 2)
	if got := w.Fleet(3).InitialMaxHP; math.Abs(got-2*raidMax) > 1e-9 {
		t.Errorf("InitialMaxHP = %v, want %v", got, 2*raidMax)
	}
	if d := w.Building(2, 11); d.MaxHP != b0[11].max {
		t.Errorf("derelict was rescaled: %v -> %v", b0[11].max, d.MaxHP)
	}
	if err := w.Validate(); err != nil {
		t.Errorf("world invalid after rescaling: %v", err)
	}
}
