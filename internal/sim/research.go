package sim

import (
	"errors"
	"slices"

	"drifthold/internal/model"
)

// ResearchRate returns faction f's research output in points per cycle: the
// sum over its working Laboratories, times its research modifier.
func ResearchRate(w *model.World, f model.FactionID) float64 {
	fac := w.Faction(f)
	if fac == nil {
		return 0
	}
	rate := 0.0
	for _, r := range w.Rocks {
		if r.Owner != f {
			continue
		}
		for _, b := range r.Buildings {
			if def := b.Def(); def.ResearchRate > 0 && b.Working() {
				rate += def.ResearchRate * b.Eff
			}
		}
	}
	return rate * fac.Mods.ResearchMul
}

// stepResearch advances every faction's current project by dt game-seconds.
func stepResearch(w *model.World, dt float64) {
	for _, fac := range w.Factions {
		if !fac.Alive {
			continue
		}
		rs := &fac.Research
		rs.Rate = ResearchRate(w, fac.ID)
		if rs.Current == model.TechNone {
			startNext(fac)
		}
		if rs.Current == model.TechNone {
			continue
		}
		t := rs.Current
		def := model.TechDefOf(t)
		if !rs.Started[t] {
			if def.Lumen > 0 && !w.FactionTake(fac.ID, model.Lumen, def.Lumen) {
				rs.Stalled = true
				continue
			}
			rs.Started[t] = true
		}
		rs.Stalled = false
		rs.Progress[t] += model.PerCycle(rs.Rate, dt)
		if rs.Progress[t] >= def.Cost-1e-9 {
			grantTech(w, fac, t)
		}
	}
}

// startNext moves the first researchable queue entry into the current slot.
func startNext(fac *model.Faction) {
	rs := &fac.Research
	for len(rs.Queue) > 0 {
		t := rs.Queue[0]
		if fac.CanResearch(t) {
			rs.Current = t
			rs.Queue = rs.Queue[1:]
			return
		}
		if fac.Has(t) {
			rs.Queue = rs.Queue[1:] // already known: drop it
			continue
		}
		return // waits for a prerequisite that is not known yet
	}
}

// grantTech gives a faction a technology and applies everything it changes.
func grantTech(w *model.World, fac *model.Faction, t model.TechID) {
	if !t.Valid() || fac.Techs[t] {
		return
	}
	oldHP := fac.Mods.HPMul
	fac.Techs[t] = true
	fac.Research.Progress[t] = model.TechDefOf(t).Cost
	fac.Research.Started[t] = true
	fac.RecomputeMods(w.Settings.Difficulty)
	if oldHP > 0 && fac.Mods.HPMul != oldHP {
		w.RescaleHP(fac.ID, fac.Mods.HPMul/oldHP)
	}
	fac.Stats.TechsResearched++
	if fac.Research.Current == t {
		fac.Research.Current = model.TechNone
	}
	fac.Research.Queue = slices.DeleteFunc(fac.Research.Queue, func(q model.TechID) bool { return q == t })
	fx := model.NewFX(model.FXTechDone, model.NoRock, model.Hex{}, fac.ID)
	fx.Amount = float64(t)
	w.Emit(fx)
	note(w, fac.ID, model.SevGood, model.NoRock, "Research complete: %s.", model.TechDefOf(t).Name)
	startNext(fac)
}

// SetResearch makes tech the current project of faction f. Progress on the
// project it replaces is kept.
func SetResearch(w *model.World, f model.FactionID, tech model.TechID) error {
	fac, err := w.Issuer(f)
	if err != nil {
		return err
	}
	if !tech.Valid() {
		return model.ErrBadCommand
	}
	if fac.Has(tech) {
		return errors.New("That technology is already known")
	}
	if !fac.PrereqsMet(tech) {
		return errors.New("Its prerequisites have not been researched")
	}
	rs := &fac.Research
	rs.Queue = slices.DeleteFunc(rs.Queue, func(q model.TechID) bool { return q == tech })
	rs.Current = tech
	rs.Stalled = false
	return nil
}

// QueueResearch appends tech to faction f's research queue (or starts it when
// nothing is being researched). A tech may be queued behind its own
// prerequisites.
func QueueResearch(w *model.World, f model.FactionID, tech model.TechID) error {
	fac, err := w.Issuer(f)
	if err != nil {
		return err
	}
	if !tech.Valid() {
		return model.ErrBadCommand
	}
	if fac.Has(tech) {
		return errors.New("That technology is already known")
	}
	if fac.InQueue(tech) {
		return errors.New("That technology is already queued")
	}
	rs := &fac.Research
	for _, req := range model.TechDefOf(tech).Requires {
		if !fac.Has(req) && !fac.InQueue(req) {
			return errors.New("Its prerequisites have not been researched")
		}
	}
	if rs.Current == model.TechNone && fac.CanResearch(tech) {
		rs.Current = tech
		return nil
	}
	if len(rs.Queue) >= model.MaxResearchQueue {
		return errors.New("The research queue is full")
	}
	rs.Queue = append(rs.Queue, tech)
	return nil
}

// CancelResearch cancels the current project (index -1) or removes a queue
// entry. Progress already made is kept.
func CancelResearch(w *model.World, f model.FactionID, index int) error {
	fac, err := w.Issuer(f)
	if err != nil {
		return err
	}
	rs := &fac.Research
	if index == -1 {
		if rs.Current == model.TechNone {
			return errors.New("Nothing is being researched")
		}
		rs.Current = model.TechNone
		rs.Stalled = false
		startNext(fac)
		return nil
	}
	if index < 0 || index >= len(rs.Queue) {
		return model.ErrBadCommand
	}
	rs.Queue = slices.Delete(rs.Queue, index, index+1)
	return nil
}
