package sim

import (
	"errors"
	"fmt"

	"drifthold/internal/model"
)

// note logs a message addressed to one faction. Rivals never read their log,
// so nothing is formatted or stored for computer-controlled factions.
func note(w *model.World, f model.FactionID, sev model.Severity, rock model.RockID, format string, args ...any) {
	if fac := w.Faction(f); fac == nil || fac.IsAI {
		return
	}
	w.Logf(f, sev, rock, format, args...)
}

// ownBuilding returns a building that belongs to faction f itself (never a
// derelict), together with its rock.
func ownBuilding(w *model.World, f model.FactionID, rock model.RockID, id model.BuildingID) (*model.Rock, *model.Building, error) {
	r, b, err := w.OwnedBuilding(f, rock, id)
	if err != nil {
		return nil, nil, err
	}
	if b.Derelict() || b.Owner != f {
		return nil, nil, errors.New("That structure is a derelict")
	}
	return r, b, nil
}

// countOwned returns how many buildings of a kind faction f has on the rock,
// in any state. Per-rock limits apply to this count.
func countOwned(r *model.Rock, kind model.BuildingKind, f model.FactionID) int {
	n := 0
	for _, b := range r.Buildings {
		if b.Kind == kind && b.Owner == f {
			n++
		}
	}
	return n
}

// CanPlace reports whether faction f may place a building of the given kind
// on a tile right now. It returns nil when the placement is legal and
// otherwise an error whose text explains the problem to the player. The cost
// check is included.
func CanPlace(w *model.World, f model.FactionID, rock model.RockID, hex model.Hex, kind model.BuildingKind) error {
	return canPlace(w, f, rock, hex, kind, true)
}

// CanPlaceIgnoringCost is CanPlace without the stock check; the build palette
// uses it to tell "cannot build here" from "cannot afford yet".
func CanPlaceIgnoringCost(w *model.World, f model.FactionID, rock model.RockID, hex model.Hex, kind model.BuildingKind) error {
	return canPlace(w, f, rock, hex, kind, false)
}

func canPlace(w *model.World, f model.FactionID, rock model.RockID, hex model.Hex, kind model.BuildingKind, checkCost bool) error {
	fac, err := w.Issuer(f)
	if err != nil {
		return err
	}
	r, err := w.OwnedRock(f, rock)
	if err != nil {
		return err
	}
	def := model.BuildingDefOf(kind)
	if !kind.Valid() || !def.CommandBuildable() {
		return model.ErrBadCommand
	}
	if !fac.CanBuild(kind) {
		return model.ErrLocked
	}
	t := r.TileAt(hex)
	if t == nil || !t.Terrain.Solid() {
		return model.ErrBadHex
	}
	if t.Building != 0 {
		return model.ErrTileOccupied
	}
	if t.Terrain != model.TerrainRegolith {
		return errors.New("That ground is too broken to build on")
	}
	if !r.AdjacentToOwned(hex, f) {
		return errors.New("Must be built next to one of your structures")
	}
	if def.MaxPerRock > 0 && countOwned(r, kind, f) >= def.MaxPerRock {
		return fmt.Errorf("Only %d %s allowed per rock", def.MaxPerRock, def.Name)
	}
	if checkCost {
		if err := r.Colony.Stock.Shortage(def.Cost); err != nil {
			return err
		}
	}
	return nil
}

// Place puts a new building under construction, deducting its cost.
func Place(w *model.World, f model.FactionID, rock model.RockID, hex model.Hex, kind model.BuildingKind) (*model.Building, error) {
	if err := CanPlace(w, f, rock, hex, kind); err != nil {
		return nil, err
	}
	r := w.Rock(rock)
	def := model.BuildingDefOf(kind)
	r.Colony.Pay(def.Cost)
	t := r.TileAt(hex)
	if t.Rubble {
		t.Rubble = false
		r.Colony.AddClamped(model.Alloy, model.RubbleAlloyBonus)
	}
	b := model.NewBuilding(w.NewBuildingID(), kind, f, hex, w.Mods(f).HPMul)
	if err := r.AddBuilding(b); err != nil {
		r.Colony.AddStockClamped(def.Cost)
		return nil, err
	}
	w.Emit(model.NewFX(model.FXBuildPlaced, rock, hex, f))
	return b, nil
}

// advanceConstruction raises a site's progress by dp and grows its HP by the
// matching share of max HP, so damage taken during construction is kept.
// It reports whether the building is now complete.
func advanceConstruction(b *model.Building, dp float64) bool {
	floor := model.ConstructionMinHPFrac
	old := b.Progress
	b.Progress = old + dp
	if b.Progress >= 1-1e-9 {
		b.Progress = 1
	}
	grow := max(b.Progress, floor) - max(old, floor)
	b.HP = min(b.MaxHP, b.HP+grow*b.MaxHP)
	return b.Progress >= 1
}

// finishBuilding completes a construction and does the one-off work some kinds
// need when they come online.
func finishBuilding(w *model.World, r *model.Rock, b *model.Building) {
	b.Complete()
	if b.HP > b.MaxHP-1e-6 {
		b.HP = b.MaxHP
	}
	if fac := w.Faction(b.Owner); fac != nil {
		fac.Stats.BuildingsBuilt++
	}
	if b.Kind == model.BoreRig {
		// The crew can see what the rig is biting into.
		r.RevealWithin(b.Hex, 1, b.Owner)
	}
	fx := model.NewFX(model.FXBuildDone, r.ID, b.Hex, b.Owner)
	fx.Amount = float64(b.Kind)
	w.Emit(fx)
	note(w, b.Owner, model.SevInfo, r.ID, "%s completed on %s.", b.Def().Name, r.Name)
}

// stepConstruction advances every construction site by dt game-seconds.
func stepConstruction(w *model.World, dt float64) {
	for _, r := range w.Rocks {
		if r.Colony == nil {
			continue
		}
		speed := 1.0
		if r.Colony.PowerDeficit() {
			speed = model.PowerDeficitBuildMul
		}
		for _, b := range r.Buildings {
			if !b.Constructing() {
				continue
			}
			bt := b.Def().BuildTime
			if bt <= 0 {
				bt = 1
			}
			if advanceConstruction(b, speed*dt/bt) {
				finishBuilding(w, r, b)
			}
		}
	}
}

// queueRefund returns the goods paid for a cradle's queued ships, and the
// colonists reserved by queued Seedships.
func queueRefund(b *model.Building) (model.Stock, float64) {
	var s model.Stock
	pop := 0.0
	for _, k := range b.Queue {
		s.Add(model.ShipDefOf(k).Cost)
		if model.ShipDefOf(k).CanColonize {
			pop += model.SeedshipColonists
		}
	}
	return s, pop
}

// Demolish removes a building on faction f's rock: a construction site is
// cancelled with a full refund, a completed building is demolished for half
// its Alloy, Silica and Parts, and a derelict is salvaged for 30 %.
func Demolish(w *model.World, f model.FactionID, rock model.RockID, id model.BuildingID) error {
	if _, err := w.Issuer(f); err != nil {
		return err
	}
	r, b, err := w.OwnedBuilding(f, rock, id)
	if err != nil {
		return err
	}
	def := b.Def()
	var refund model.Stock
	switch {
	case b.Derelict():
		refund = def.Cost.Refundable().Scaled(model.SalvageRefundFrac)
	case b.Kind == model.Keystone:
		return errors.New("A Keystone cannot be demolished")
	case b.Constructing():
		refund = def.Cost.Scaled(model.CancelRefundFrac)
	default:
		refund = def.Cost.Refundable().Scaled(model.DemolishRefundFrac)
		q, pop := queueRefund(b)
		refund.Add(q)
		r.Colony.Pop += pop
	}
	r.RemoveBuilding(b.ID)
	r.Colony.AddStockClamped(refund)
	return nil
}

// Recommission reactivates a derelict on faction f's rock for 40 % of its
// original cost. The building keeps its current HP.
func Recommission(w *model.World, f model.FactionID, rock model.RockID, id model.BuildingID) error {
	if _, err := w.Issuer(f); err != nil {
		return err
	}
	r, b, err := w.OwnedBuilding(f, rock, id)
	if err != nil {
		return err
	}
	if !b.Derelict() {
		return errors.New("Only derelicts can be recommissioned")
	}
	def := b.Def()
	if !def.CommandBuildable() {
		return model.ErrBadCommand
	}
	if def.MaxPerRock > 0 && countOwned(r, b.Kind, f) >= def.MaxPerRock {
		return fmt.Errorf("Only %d %s allowed per rock", def.MaxPerRock, def.Name)
	}
	cost := def.Cost.Scaled(model.RecommissionCostFrac)
	if err := r.Colony.Stock.Shortage(cost); err != nil {
		return err
	}
	r.Colony.Pay(cost)
	b.Owner = f
	b.State = model.StateActive
	b.Progress = 1
	b.Enabled = true
	b.Priority = def.DefaultPriority()
	b.DriverTarget = model.NoRock
	w.Emit(model.NewFX(model.FXBuildDone, rock, b.Hex, f))
	return nil
}

// SetEnabled switches one of faction f's buildings on or off.
func SetEnabled(w *model.World, f model.FactionID, rock model.RockID, id model.BuildingID, on bool) error {
	if _, err := w.Issuer(f); err != nil {
		return err
	}
	_, b, err := ownBuilding(w, f, rock, id)
	if err != nil {
		return err
	}
	if b.Kind == model.Keystone {
		return errors.New("A Keystone cannot be switched off")
	}
	b.Enabled = on
	return nil
}

// SetPriority sets the staffing and power priority of one of faction f's
// buildings.
func SetPriority(w *model.World, f model.FactionID, rock model.RockID, id model.BuildingID, priority int) error {
	if _, err := w.Issuer(f); err != nil {
		return err
	}
	_, b, err := ownBuilding(w, f, rock, id)
	if err != nil {
		return err
	}
	if priority < model.PriorityLow || priority > model.PriorityHigh {
		return model.ErrBadCommand
	}
	b.Priority = priority
	return nil
}

// ExpediteQuote returns the Scrip price of finishing a construction now.
func ExpediteQuote(b *model.Building) float64 {
	if b == nil || !b.Constructing() {
		return 0
	}
	return model.ExpediteCost(b.RemainingBuildSeconds(), b.Def().Cost[model.Alloy])
}

// Expedite pays Scrip to finish one of faction f's constructions immediately.
func Expedite(w *model.World, f model.FactionID, rock model.RockID, id model.BuildingID) error {
	fac, err := w.Issuer(f)
	if err != nil {
		return err
	}
	r, b, err := ownBuilding(w, f, rock, id)
	if err != nil {
		return err
	}
	if !b.Constructing() {
		return errors.New("That structure is already complete")
	}
	price := ExpediteQuote(b)
	if !fac.SpendScrip(price) {
		return &model.ScripError{Need: price}
	}
	advanceConstruction(b, 1)
	finishBuilding(w, r, b)
	return nil
}
