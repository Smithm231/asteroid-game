package model

// This file holds the ownership checks that every command handler starts
// with. Using them keeps the error a player sees for the same mistake
// identical across systems.

// Issuer returns the faction that issues a command, after the checks common
// to every command: the game is still running (else ErrGameOver), the
// faction exists (else ErrNoFaction) and it is alive (else ErrEliminated).
func (w *World) Issuer(f FactionID) (*Faction, error) {
	if w == nil {
		return nil, ErrNoFaction
	}
	if w.GameOver {
		return nil, ErrGameOver
	}
	fac := w.Faction(f)
	if fac == nil {
		return nil, ErrNoFaction
	}
	if !fac.Alive {
		return nil, ErrEliminated
	}
	return fac, nil
}

// OwnedRock returns the given rock if faction f owns it (has a colony
// there). It fails with ErrNoRock when the rock does not exist and with
// ErrNotOwner when it is unowned or belongs to someone else. On success the
// rock's Colony is never nil.
func (w *World) OwnedRock(f FactionID, rock RockID) (*Rock, error) {
	r := w.Rock(rock)
	if r == nil {
		return nil, ErrNoRock
	}
	if f == NoFaction || r.Owner != f || r.Colony == nil {
		return nil, ErrNotOwner
	}
	return r, nil
}

// OwnedBuilding returns a building on a rock that faction f owns, together
// with the rock. It fails like OwnedRock, and with ErrNoBuilding when the
// rock has no building with that ID. The building is either f's own (in any
// state) or a derelict standing on f's rock, which f may salvage or
// recommission; callers that need one or the other check Building.Derelict.
func (w *World) OwnedBuilding(f FactionID, rock RockID, id BuildingID) (*Rock, *Building, error) {
	r, err := w.OwnedRock(f, rock)
	if err != nil {
		return nil, nil, err
	}
	b := r.BuildingByID(id)
	if b == nil {
		return nil, nil, ErrNoBuilding
	}
	if b.Owner != f && !b.Derelict() {
		return nil, nil, ErrNotOwner
	}
	return r, b, nil
}

// OwnedFleet returns the given fleet if faction f owns it. It fails with
// ErrNoFleet when the fleet does not exist and with ErrNotOwner when it
// belongs to someone else.
func (w *World) OwnedFleet(f FactionID, id FleetID) (*Fleet, error) {
	fl := w.Fleet(id)
	if fl == nil {
		return nil, ErrNoFleet
	}
	if f == NoFaction || fl.Owner != f {
		return nil, ErrNotOwner
	}
	return fl, nil
}

// FoundColony founds a colony of faction f on the given rock with its
// Keystone at hex. It performs the bookkeeping shared by world generation
// and Seedship landings, keeping the invariant "Owner set, Colony set,
// exactly one completed Keystone" intact:
//
//   - a completed Keystone at full HP (scaled by f's Mods.HPMul) is added at
//     hex, and Rubble on that tile is cleared (without the Alloy bonus);
//   - Rock.Owner becomes f and Rock.Colony a NewColony founded at the
//     current tick, with empty stock and no population (the caller adds the
//     starting stock and colonists);
//   - the tiles within the Keystone's RevealRadius are revealed to f.
//
// It fails without changing anything with ErrNoRock or ErrNoFaction for
// unknown IDs, ErrRockClaimed when the rock already has an owner, ErrBadHex
// when hex is not a Regolith tile of the rock and ErrTileOccupied when a
// building (for example a derelict) stands there. It emits no log entry and
// no FX.
func (w *World) FoundColony(rock RockID, f FactionID, hex Hex) (*Building, error) {
	r := w.Rock(rock)
	if r == nil {
		return nil, ErrNoRock
	}
	fac := w.Faction(f)
	if fac == nil {
		return nil, ErrNoFaction
	}
	if r.Owner != NoFaction || r.Colony != nil {
		return nil, ErrRockClaimed
	}
	t := r.TileAt(hex)
	if t == nil || t.Terrain != TerrainRegolith {
		return nil, ErrBadHex
	}
	if t.Building != 0 {
		return nil, ErrTileOccupied
	}
	k := NewActiveBuilding(w.NewBuildingID(), Keystone, f, hex, fac.Mods.HPMul)
	if err := r.AddBuilding(k); err != nil {
		return nil, err
	}
	t.Rubble = false
	r.Owner = f
	r.Colony = NewColony(w.Tick)
	r.RevealWithin(hex, buildingDefs[Keystone].RevealRadius, f)
	return k, nil
}

// DissolveColony performs the bookkeeping of a fallen colony: the rock
// becomes unowned, its colony (stock and colonists) is discarded, and every
// building still standing on it becomes a derelict (see
// Building.MakeDerelict). It returns the discarded colony and the former
// owner, or nil and NoFaction when the rock had no colony. Call it after the
// destroyed Keystone has been removed with RemoveBuilding; awarding the
// bounty, logging, FX and elimination checks are the combat system's job.
func (r *Rock) DissolveColony() (lost *Colony, owner FactionID) {
	if r == nil || (r.Owner == NoFaction && r.Colony == nil) {
		return nil, NoFaction
	}
	lost, owner = r.Colony, r.Owner
	r.Owner = NoFaction
	r.Colony = nil
	for _, b := range r.Buildings {
		b.MakeDerelict()
	}
	return lost, owner
}

// RescaleHP multiplies the MaxHP and the HP of every ship and every
// non-derelict building of faction f by factor, preserving each one's health
// fraction. The research system calls it with newHPMul/oldHPMul when a tech
// changes Mods.HPMul. A factor that is not positive, or exactly 1, does
// nothing.
func (w *World) RescaleHP(f FactionID, factor float64) {
	if w == nil || f == NoFaction || !(factor > 0) || factor == 1 {
		return
	}
	for _, r := range w.Rocks {
		for _, b := range r.Buildings {
			if b.Owner == f && b.State != StateDerelict {
				b.MaxHP *= factor
				b.HP *= factor
			}
		}
	}
	for _, fl := range w.Fleets {
		if fl.Owner != f {
			continue
		}
		for i := range fl.Ships {
			fl.Ships[i].MaxHP *= factor
			fl.Ships[i].HP *= factor
		}
		fl.InitialMaxHP *= factor
	}
}
