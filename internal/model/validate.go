package model

import (
	"fmt"
	"math"
)

// Validate checks the structural invariants of the world and returns the
// first violation found, or nil. It is meant for loading saves, for tests
// and for soak runs: a world that passes can be used by every helper of
// this package without surprises. It checks structure and references, not
// game balance.
//
// Invariants checked:
//
//   - rocks: non-nil, ID == index, Tiles length matches Radius, finite orbit
//     numbers, valid class and owner, Owner set exactly when Colony is set;
//   - tiles: valid terrain; Deposit is GoodNone or a raw good; Amount >= 0;
//   - buildings: non-nil, sorted by strictly ascending ID below
//     NextBuildingID, valid kind and state, standing on a solid tile whose
//     Building field points back at them; derelicts have no owner and every
//     other building belongs to the rock's owner; every tile back-reference
//     resolves; an owned rock has exactly one completed Keystone;
//   - factions: at most MaxFactions, non-nil, ID == index+1, positive Mods
//     multipliers, valid research state;
//   - fleets: non-nil, sorted by strictly ascending ID below NextFleetID,
//     existing owner, at least one ship, valid rock references, world-unique
//     ship IDs below NextShipID, valid ship kinds, at most one garrison per
//     faction and rock, and a garrison is always in orbit;
//   - slugs and salvage reference existing rocks and factions;
//   - numbers that the simulation divides by or accumulates are finite.
func (w *World) Validate() error {
	if w == nil {
		return fmt.Errorf("world is nil")
	}
	if len(w.Factions) > MaxFactions {
		return fmt.Errorf("%d factions, at most %d allowed", len(w.Factions), MaxFactions)
	}
	for i, f := range w.Factions {
		if err := w.validateFaction(i, f); err != nil {
			return err
		}
	}
	seenBuildings := map[BuildingID]RockID{}
	for i, r := range w.Rocks {
		if err := w.validateRock(i, r, seenBuildings); err != nil {
			return err
		}
	}
	if err := w.validateFleets(); err != nil {
		return err
	}
	for i, s := range w.Slugs {
		if w.Rock(s.Target) == nil {
			return fmt.Errorf("slug %d: target rock %d does not exist", i, s.Target)
		}
		if !finite(s.Pos.X, s.Pos.Y, s.Damage) {
			return fmt.Errorf("slug %d: non-finite numbers", i)
		}
	}
	for g, p := range w.Market.Pressure {
		if !finite(p) {
			return fmt.Errorf("market pressure of %s is not finite", Good(g))
		}
	}
	if w.Winner != NoFaction && w.Faction(w.Winner) == nil {
		return fmt.Errorf("winner %d does not exist", w.Winner)
	}
	if w.Tick < 0 {
		return fmt.Errorf("negative tick %d", w.Tick)
	}
	return nil
}

// finite reports whether none of the values is NaN or infinite.
func finite(vs ...float64) bool {
	for _, v := range vs {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// validateFaction checks one faction; see Validate.
func (w *World) validateFaction(i int, f *Faction) error {
	if f == nil {
		return fmt.Errorf("faction slot %d is nil", i)
	}
	if int(f.ID) != i+1 {
		return fmt.Errorf("faction slot %d has ID %d, want %d", i, f.ID, i+1)
	}
	m := f.Mods
	for _, v := range []float64{m.MiningMul, m.SolarMul, m.FoodMul, m.SmeltMul, m.HPMul, m.ShipSpeedMul, m.StaffMul, m.ResearchMul, m.ProdMul} {
		if !(v > 0) || !finite(v) {
			return fmt.Errorf("faction %d: Mods multipliers must be positive and finite (use DefaultMods or ComputeMods)", f.ID)
		}
	}
	if !finite(f.Scrip) {
		return fmt.Errorf("faction %d: Scrip is not finite", f.ID)
	}
	if !f.Personality.Valid() {
		return fmt.Errorf("faction %d: invalid personality %d", f.ID, f.Personality)
	}
	if f.HomeRock != NoRock && w.Rock(f.HomeRock) == nil {
		return fmt.Errorf("faction %d: home rock %d does not exist", f.ID, f.HomeRock)
	}
	res := &f.Research
	if res.Current != TechNone && !res.Current.Valid() {
		return fmt.Errorf("faction %d: invalid current research %d", f.ID, res.Current)
	}
	if len(res.Queue) > MaxResearchQueue {
		return fmt.Errorf("faction %d: research queue has %d entries, at most %d allowed", f.ID, len(res.Queue), MaxResearchQueue)
	}
	for _, t := range res.Queue {
		if !t.Valid() {
			return fmt.Errorf("faction %d: invalid tech %d in research queue", f.ID, t)
		}
	}
	for t, p := range res.Progress {
		if !finite(p) || p < 0 {
			return fmt.Errorf("faction %d: bad research progress on %s", f.ID, TechID(t))
		}
	}
	return nil
}

// validateRock checks one rock, its tiles, buildings, colony and salvage;
// see Validate. seen collects building IDs across rocks to detect
// duplicates.
func (w *World) validateRock(i int, r *Rock, seen map[BuildingID]RockID) error {
	if r == nil {
		return fmt.Errorf("rock slot %d is nil", i)
	}
	if int(r.ID) != i {
		return fmt.Errorf("rock slot %d has ID %d", i, r.ID)
	}
	if r.Radius < 0 || len(r.Tiles) != TileCount(r.Radius) {
		return fmt.Errorf("rock %d: %d tiles do not match radius %d", r.ID, len(r.Tiles), r.Radius)
	}
	if !finite(r.OrbitR, r.Phase0, r.Solar) || !(r.OrbitR > 0) {
		return fmt.Errorf("rock %d: bad orbit numbers", r.ID)
	}
	if !r.Class.Valid() {
		return fmt.Errorf("rock %d: invalid class %d", r.ID, r.Class)
	}
	if r.Owner != NoFaction && w.Faction(r.Owner) == nil {
		return fmt.Errorf("rock %d: owner %d does not exist", r.ID, r.Owner)
	}
	if (r.Owner != NoFaction) != (r.Colony != nil) {
		return fmt.Errorf("rock %d: owner %d but colony present = %v", r.ID, r.Owner, r.Colony != nil)
	}
	for ti := range r.Tiles {
		t := &r.Tiles[ti]
		if !t.Terrain.Valid() {
			return fmt.Errorf("rock %d tile %d: invalid terrain %d", r.ID, ti, t.Terrain)
		}
		if t.Deposit != GoodNone && !t.Deposit.Raw() {
			return fmt.Errorf("rock %d tile %d: deposit %d is not a raw good", r.ID, ti, t.Deposit)
		}
		if !finite(t.Amount) || t.Amount < 0 {
			return fmt.Errorf("rock %d tile %d: bad deposit amount %v", r.ID, ti, t.Amount)
		}
		if t.Building != 0 {
			h, ok := r.HexAt(ti)
			b := r.BuildingByID(t.Building)
			if !ok || b == nil || b.Hex != h {
				return fmt.Errorf("rock %d tile %d: dangling building reference %d", r.ID, ti, t.Building)
			}
		}
	}
	keystones := 0
	var prev BuildingID
	for bi, b := range r.Buildings {
		if b == nil {
			return fmt.Errorf("rock %d: building slot %d is nil", r.ID, bi)
		}
		if b.ID <= prev {
			return fmt.Errorf("rock %d: buildings not sorted by strictly ascending ID at slot %d", r.ID, bi)
		}
		prev = b.ID
		if b.ID >= w.NextBuildingID {
			return fmt.Errorf("rock %d: building %d is not below NextBuildingID %d", r.ID, b.ID, w.NextBuildingID)
		}
		if other, dup := seen[b.ID]; dup {
			return fmt.Errorf("building %d exists on rock %d and rock %d", b.ID, other, r.ID)
		}
		seen[b.ID] = r.ID
		if !b.Kind.Valid() || !b.State.Valid() {
			return fmt.Errorf("rock %d building %d: invalid kind or state", r.ID, b.ID)
		}
		t := r.TileAt(b.Hex)
		if t == nil || t.Terrain == TerrainVoid || t.Building != b.ID {
			return fmt.Errorf("rock %d building %d: tile %v does not point back at it", r.ID, b.ID, b.Hex)
		}
		if b.State == StateDerelict {
			if b.Owner != NoFaction {
				return fmt.Errorf("rock %d building %d: derelict with owner %d", r.ID, b.ID, b.Owner)
			}
		} else if b.Owner != r.Owner || b.Owner == NoFaction {
			return fmt.Errorf("rock %d building %d: owner %d differs from rock owner %d", r.ID, b.ID, b.Owner, r.Owner)
		}
		if !finite(b.HP, b.MaxHP, b.Progress, b.Eff, b.Shield, b.Cooldown, b.QueueProgress) || !(b.MaxHP > 0) {
			return fmt.Errorf("rock %d building %d: bad numbers", r.ID, b.ID)
		}
		if len(b.Queue) > MaxShipQueue {
			return fmt.Errorf("rock %d building %d: ship queue has %d entries, at most %d allowed", r.ID, b.ID, len(b.Queue), MaxShipQueue)
		}
		for _, k := range b.Queue {
			if !k.Valid() {
				return fmt.Errorf("rock %d building %d: invalid ship kind %d in queue", r.ID, b.ID, k)
			}
		}
		if b.DriverTarget != NoRock && w.Rock(b.DriverTarget) == nil {
			return fmt.Errorf("rock %d building %d: driver target %d does not exist", r.ID, b.ID, b.DriverTarget)
		}
		if b.Kind == Keystone && b.State == StateActive {
			keystones++
		}
	}
	if r.Owner != NoFaction && keystones != 1 {
		return fmt.Errorf("rock %d: owned rock has %d completed Keystones, want 1", r.ID, keystones)
	}
	if r.Owner == NoFaction && keystones != 0 {
		return fmt.Errorf("rock %d: unowned rock has a completed Keystone", r.ID)
	}
	if c := r.Colony; c != nil {
		if !finite(c.Pop, c.Morale, c.StorageCap, c.Housing, c.LifeSupport, c.PowerSupply, c.PowerDemand) || c.Pop < 0 {
			return fmt.Errorf("rock %d: bad colony numbers", r.ID)
		}
		for g, v := range c.Stock {
			if !finite(v) || v < 0 {
				return fmt.Errorf("rock %d: bad %s stock %v", r.ID, Good(g), v)
			}
		}
	}
	if s := r.Salvage; s != nil {
		if s.Claimant != NoFaction && w.Faction(s.Claimant) == nil {
			return fmt.Errorf("rock %d: salvage claimant %d does not exist", r.ID, s.Claimant)
		}
		if !finite(s.Scrip, s.ClaimSeconds) {
			return fmt.Errorf("rock %d: bad salvage numbers", r.ID)
		}
	}
	return nil
}

// validateFleets checks every fleet and ship; see Validate.
func (w *World) validateFleets() error {
	type garrisonKey struct {
		rock  RockID
		owner FactionID
	}
	garrisons := map[garrisonKey]FleetID{}
	ships := map[ShipID]FleetID{}
	var prev FleetID
	for i, f := range w.Fleets {
		if f == nil {
			return fmt.Errorf("fleet slot %d is nil", i)
		}
		if f.ID <= prev {
			return fmt.Errorf("fleets not sorted by strictly ascending ID at slot %d", i)
		}
		prev = f.ID
		if f.ID >= w.NextFleetID {
			return fmt.Errorf("fleet %d is not below NextFleetID %d", f.ID, w.NextFleetID)
		}
		if w.Faction(f.Owner) == nil {
			return fmt.Errorf("fleet %d: owner %d does not exist", f.ID, f.Owner)
		}
		if len(f.Ships) == 0 {
			return fmt.Errorf("fleet %d has no ships", f.ID)
		}
		for _, id := range []RockID{f.At, f.Dest, f.Origin, f.Home} {
			if id != NoRock && w.Rock(id) == nil {
				return fmt.Errorf("fleet %d: rock %d does not exist", f.ID, id)
			}
		}
		if !f.Order.Valid() || !f.Stance.Valid() {
			return fmt.Errorf("fleet %d: invalid order or stance", f.ID)
		}
		if !finite(f.Pos.X, f.Pos.Y, f.RetreatAt, f.InitialMaxHP, f.SurveyTimer, f.DwellTimer, f.RetryTimer) {
			return fmt.Errorf("fleet %d: non-finite numbers", f.ID)
		}
		if f.Route != nil && (w.Rock(f.Route.A) == nil || w.Rock(f.Route.B) == nil) {
			return fmt.Errorf("fleet %d: route references a missing rock", f.ID)
		}
		if f.Garrison {
			if f.At == NoRock {
				return fmt.Errorf("fleet %d: garrison in transit", f.ID)
			}
			key := garrisonKey{f.At, f.Owner}
			if other, dup := garrisons[key]; dup {
				return fmt.Errorf("fleets %d and %d are both garrisons of faction %d at rock %d", other, f.ID, f.Owner, f.At)
			}
			garrisons[key] = f.ID
		}
		for si := range f.Ships {
			s := &f.Ships[si]
			if s.ID <= 0 || s.ID >= w.NextShipID {
				return fmt.Errorf("fleet %d: ship ID %d is not in 1..NextShipID-1", f.ID, s.ID)
			}
			if other, dup := ships[s.ID]; dup {
				return fmt.Errorf("ship %d exists in fleet %d and fleet %d", s.ID, other, f.ID)
			}
			ships[s.ID] = f.ID
			if !s.Kind.Valid() {
				return fmt.Errorf("fleet %d ship %d: invalid kind %d", f.ID, s.ID, s.Kind)
			}
			if !finite(s.HP, s.MaxHP) || !(s.MaxHP > 0) {
				return fmt.Errorf("fleet %d ship %d: bad hit points", f.ID, s.ID)
			}
			for g, v := range s.Cargo {
				if !finite(v) || v < 0 {
					return fmt.Errorf("fleet %d ship %d: bad %s cargo %v", f.ID, s.ID, Good(g), v)
				}
			}
		}
	}
	return nil
}
