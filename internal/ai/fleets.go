package ai

import (
	"math"

	"drifthold/internal/model"
	"drifthold/internal/sim"
)

// shipCounts tallies the faction's ships per kind, existing and queued.
func (b *brain) shipCounts() (have, queued [model.NumShipKinds]int) {
	for _, fl := range b.w.Fleets {
		if fl.Owner != b.f {
			continue
		}
		for i := range fl.Ships {
			have[fl.Ships[i].Kind]++
		}
	}
	for _, r := range b.w.Rocks {
		if r.Owner != b.f {
			continue
		}
		for _, bd := range r.Buildings {
			for _, k := range bd.Queue {
				queued[k]++
			}
		}
	}
	return have, queued
}

// base returns the rock the faction stages from: its home while it holds it,
// else its first rock with a Launch Cradle, else its first rock.
func (b *brain) base() *model.Rock {
	if h := b.w.Rock(b.fac.HomeRock); h != nil && h.Owner == b.f {
		return h
	}
	rocks := b.w.RocksOf(b.f)
	for _, r := range rocks {
		if r.CountKind(model.LaunchCradle, true) > 0 {
			return r
		}
	}
	if len(rocks) > 0 {
		return rocks[0]
	}
	return nil
}

// targeted reports whether one of the faction's fleets already has the given
// order on the rock.
func (b *brain) targeted(rock model.RockID, order model.OrderKind) bool {
	for _, fl := range b.w.Fleets {
		if fl.Owner == b.f && fl.Order == order && fl.Dest == rock {
			return true
		}
	}
	return false
}

// colonyTarget picks the unclaimed rock most worth a Seedship, or NoRock.
func (b *brain) colonyTarget(from model.RockID) model.RockID {
	hasFrost, hasGlint := false, false
	for _, r := range b.w.RocksOf(b.f) {
		hasFrost = hasFrost || r.Class == model.ClassFrost
		hasGlint = hasGlint || r.Class == model.ClassGlint
	}
	best, bestScore := model.NoRock, math.Inf(-1)
	for _, r := range b.w.Rocks {
		if r.Owner != model.NoFaction || b.targeted(r.ID, model.OrderColonize) {
			continue
		}
		score := float64(r.BlobSize())/80 - b.w.RockDist(from, r.ID)/350
		if r.Surveyed(b.f) {
			score += 1.5
		}
		switch {
		case r.Class == model.ClassFrost && !hasFrost:
			score += 4
		case r.Class == model.ClassGlint && !hasGlint:
			score += 2.5
		case r.Class == model.ClassFerric:
			score += 1
		}
		if score > bestScore {
			best, bestScore = r.ID, score
		}
	}
	return best
}

// shipyards keeps every idle Launch Cradle busy with what the faction needs
// most.
func (b *brain) shipyards() {
	have, queued := b.shipCounts()
	total := func(k model.ShipKind) int { return have[k] + queued[k] }
	colonies := b.w.ColonyCount(b.f)
	for _, r := range b.w.RocksOf(b.f) {
		for _, bd := range r.Buildings {
			if bd.Kind != model.LaunchCradle || !bd.Operational() || len(bd.Queue) > 0 || bd.Owner != b.f {
				continue
			}
			kind := model.ShipNone
			unsurveyed := false
			for _, o := range b.w.Rocks {
				if !o.Surveyed(b.f) {
					unsurveyed = true
					break
				}
			}
			switch {
			case total(model.Prospector) == 0 && unsurveyed && b.fac.CanBuildShip(model.Prospector):
				kind = model.Prospector
			case b.fac.CanBuildShip(model.Seedship) && colonies+total(model.Seedship) < b.prof.colonies &&
				r.Colony.Pop >= model.SeedshipMinPop+6 && b.w.Cycle() > 8 && b.colonyTarget(r.ID) != model.NoRock:
				kind = model.Seedship
			case b.fac.CanBuildShip(model.Hauler) && total(model.Hauler) < colonies-1:
				kind = model.Hauler
			default:
				kind = b.combatPick(total)
				// Warships come out of surplus, not out of the colony's bones.
				if kind != model.ShipNone && r.Colony.Stock[model.Alloy] < model.ShipDefOf(kind).Cost[model.Alloy]+90 {
					kind = model.ShipNone
				}
			}
			if kind == model.ShipNone {
				continue
			}
			if b.exec(model.QueueShipCmd(b.f, r.ID, bd.ID, kind)) == nil {
				queued[kind]++
			}
		}
	}
}

// combatPick returns the warship kind the fleet is shortest of, among those
// the faction can build.
func (b *brain) combatPick(total func(model.ShipKind) int) model.ShipKind {
	type share struct {
		k model.ShipKind
		w float64
	}
	mix := []share{{model.Dart, 0.25}, {model.Cutter, 0.35}, {model.Hammer, 0.30}, {model.Bulwark, 0.10}}
	sum, wsum := 0.0, 0.0
	for _, s := range mix {
		if b.fac.CanBuildShip(s.k) {
			sum += float64(total(s.k))
			wsum += s.w
		}
	}
	if wsum == 0 {
		return model.ShipNone
	}
	best, bestGap := model.ShipNone, math.Inf(-1)
	for _, s := range mix {
		if !b.fac.CanBuildShip(s.k) {
			continue
		}
		if s.k == model.Bulwark && b.w.FactionTotal(b.f, model.Lumen) < model.ShipDefOf(s.k).Cost[model.Lumen] {
			continue
		}
		if gap := s.w/wsum*(sum+1) - float64(total(s.k)); gap > bestGap {
			best, bestGap = s.k, gap
		}
	}
	return best
}

// fleets gives idle ships something to do and launches attack waves.
func (b *brain) fleets() {
	for _, fl := range b.w.FleetsOf(b.f) {
		if !fl.Garrison || fl.InTransit() {
			continue
		}
		for i := range fl.Ships {
			s := fl.Ships[i]
			one := []model.ShipID{s.ID}
			switch {
			case s.Def().CanSurvey:
				if t := b.surveyTarget(fl.At); t != model.NoRock {
					if b.exec(model.FleetOrderCmd(b.f, fl.ID, one, model.OrderSurvey, t)) == nil {
						return // the fleet changed under us; continue next step
					}
				}
			case s.Def().CanColonize:
				if t := b.colonyTarget(fl.At); t != model.NoRock {
					if b.exec(model.ColonizeCmd(b.f, fl.ID, one, t, nil)) == nil {
						return
					}
				}
			case s.Def().Cargo > 0:
				if rt, ok := b.freightRoute(fl.At); ok {
					if b.exec(model.SetRouteCmd(b.f, fl.ID, one, rt)) == nil {
						return
					}
				}
			}
		}
	}
	b.aimDrivers()
	b.attack()
}

// surveyTarget returns the nearest rock the faction has not surveyed and is
// not already surveying.
func (b *brain) surveyTarget(from model.RockID) model.RockID {
	best, bestD := model.NoRock, math.Inf(1)
	for _, r := range b.w.Rocks {
		if r.Surveyed(b.f) || b.targeted(r.ID, model.OrderSurvey) {
			continue
		}
		d := b.w.RockDist(from, r.ID) // own rocks come first: they are right here
		if r.Owner != model.NoFaction {
			d += 400 // rival rocks shoot back: leave them for last
		}
		if d < bestD {
			best, bestD = r.ID, d
		}
	}
	return best
}

// freightRoute proposes a route between the staging rock and an outpost that
// no Hauler serves yet: supplies out, raw goods back.
func (b *brain) freightRoute(from model.RockID) (model.Route, bool) {
	base := b.base()
	if base == nil || base.ID != from {
		return model.Route{}, false
	}
	for _, r := range b.w.RocksOf(b.f) {
		if r.ID == base.ID {
			continue
		}
		served := false
		for _, fl := range b.w.Fleets {
			if fl.Owner == b.f && fl.Route != nil && (fl.Route.B == r.ID || fl.Route.A == r.ID) {
				served = true
				break
			}
		}
		if served {
			continue
		}
		rt := model.Route{A: base.ID, B: r.ID, Reserve: 60}
		rt.Out[model.Alloy], rt.Out[model.Parts] = true, true
		for _, g := range []model.Good{model.Ice, model.Lumen, model.Ore, model.Silica, model.Fuel} {
			rt.Back[g] = true
		}
		return rt, true
	}
	return model.Route{}, false
}

// aimDrivers points every idle Mass Driver at the nearest rival colony in
// range.
func (b *brain) aimDrivers() {
	for _, r := range b.w.RocksOf(b.f) {
		for _, bd := range r.Buildings {
			def := bd.Def()
			if def.DriverRange <= 0 || !bd.Operational() || bd.Owner != b.f {
				continue
			}
			if t := b.w.Rock(bd.DriverTarget); t != nil && b.w.RockDist(r.ID, t.ID) <= def.DriverRange {
				continue
			}
			best, bestD := model.NoRock, def.DriverRange
			for _, o := range b.w.Rocks {
				if o.Owner == model.NoFaction || o.Owner == b.f {
					continue
				}
				if d := b.w.RockDist(r.ID, o.ID); d <= bestD {
					best, bestD = o.ID, d
				}
			}
			if best != bd.DriverTarget {
				b.exec(model.SetDriverTargetCmd(b.f, r.ID, bd.ID, best))
			}
		}
	}
}

// defenceOf estimates how hard a rival rock is to crack, from what the
// faction can see.
func (b *brain) defenceOf(r *model.Rock) float64 {
	d := 0.0
	for _, bd := range r.Buildings {
		if bd.Owner != r.Owner {
			continue
		}
		switch bd.Kind {
		case model.FlakBattery:
			d += 2
		case model.LanceTower:
			d += 4
		case model.AegisProjector:
			d += 3
		}
	}
	for _, fl := range b.w.FleetsAt(r.ID) {
		if fl.Owner == r.Owner && sim.Visible(b.w, b.f, fl) {
			for i := range fl.Ships {
				if fl.Ships[i].Def().Armed() {
					d++
				}
			}
		}
	}
	return d
}

// attack launches a wave from the staging rock once enough warships have
// gathered there.
func (b *brain) attack() {
	w := b.w
	if w.Tick < b.st.Timers[tNextAttack] {
		return
	}
	delay := difficultyDelay(w.Settings.Difficulty)
	if float64(w.Cycle()) < b.prof.firstAttack*delay {
		return
	}
	base := b.base()
	if base == nil {
		return
	}
	g := w.GarrisonOf(base.ID, b.f)
	if g == nil {
		return
	}
	// Under attack at home: nobody leaves.
	for _, fl := range w.FleetsAt(base.ID) {
		if model.Hostile(fl.Owner, b.f) && fl.Armed() {
			return
		}
	}
	var armed []model.ShipID
	guard := b.prof.homeGuard
	for i := range g.Ships {
		s := g.Ships[i]
		if !s.Def().Armed() || s.HP < 0.6*s.MaxHP {
			continue
		}
		if guard > 0 && s.Kind == model.Dart {
			guard-- // interceptors make the best home guard
			continue
		}
		armed = append(armed, s.ID)
	}
	if guard > 0 && len(armed) > guard {
		armed = armed[guard:]
	}
	need := min(14, b.prof.waveSize+b.prof.waveGrowth*b.st.Wave)
	if w.Settings.Difficulty == model.DiffEasy {
		need += 2
	}
	// A fleet that has filled every berth has nothing left to wait for.
	allIn := w.BerthsFree(b.f) <= 1 && len(armed) >= b.prof.waveSize
	if len(armed) < need && !allIn {
		return
	}
	// Pick the softest reachable rival colony.
	var target *model.Rock
	bestScore := math.Inf(1)
	for _, r := range w.Rocks {
		if r.Owner == model.NoFaction || r.Owner == b.f {
			continue
		}
		score := w.RockDist(base.ID, r.ID)/120 + b.defenceOf(r)
		if of := w.Faction(r.Owner); of != nil && !of.IsAI && w.Settings.Difficulty == model.DiffHard {
			score -= 2
		}
		if score < bestScore {
			target, bestScore = r, score
		}
	}
	if target == nil {
		return
	}
	// Do not throw a wave at a wall it cannot dent.
	if float64(len(armed)) < 0.8*b.defenceOf(target) && !allIn {
		return
	}
	// Send only as many ships as the Fuel on hand can launch.
	fuel := base.Colony.Stock[model.Fuel]
	perU, dist := 0.0, w.RockDist(base.ID, target.ID)
	keep := 0
	for _, id := range armed {
		s := g.Ship(id)
		if s == nil || (perU+s.Def().FuelPerU)*dist > fuel {
			break
		}
		perU += s.Def().FuelPerU
		keep++
	}
	if keep < len(armed) {
		if keep < (b.prof.waveSize+1)/2 {
			b.st.Timers[tNextAttack] = w.Tick + model.SecondsToTicks(30) // wait for the Crackers
			return
		}
		armed = armed[:keep]
	}
	if b.exec(model.AttackCmd(b.f, g.ID, armed, target.ID, b.prof.stance, b.prof.retreat)) == nil {
		b.st.Wave++
		b.st.TargetRock = target.ID
		b.st.Timers[tNextAttack] = w.Tick + model.CyclesToTicks(b.prof.waveGap*delay)
	} else {
		b.st.Timers[tNextAttack] = w.Tick + model.SecondsToTicks(20) // probably short of Fuel: try again soon
	}
}
