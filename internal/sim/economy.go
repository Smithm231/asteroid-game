package sim

import (
	"math"
	"sort"

	"drifthold/internal/model"
)

// MoraleTerms is the breakdown of a colony's morale target, for the economy
// and for the UI that explains it.
type MoraleTerms struct {
	Base, Fed, Starving, Suffocating, Crowding, Commons, Attacked float64
}

// Total returns the morale target the terms add up to, clamped to the valid
// morale range.
func (m MoraleTerms) Total() float64 {
	return model.Clamp(m.Base+m.Fed+m.Starving+m.Suffocating+m.Crowding+m.Commons+m.Attacked,
		model.MoraleMin, model.MoraleMax)
}

// MoraleOf returns the morale terms of a colony at tick now, from its cached
// state. Penalties are negative numbers.
func MoraleOf(c *model.Colony, now int) MoraleTerms {
	var m MoraleTerms
	if c == nil {
		return m
	}
	m.Base = model.MoraleBase
	if !c.Starving && !c.Suffocating {
		m.Fed = model.MoraleFedBonus
	}
	if c.Starving {
		m.Starving = -model.MoraleStarvingPenalty
	}
	if c.Suffocating {
		m.Suffocating = -model.MoraleSuffocatingPenalty
	}
	if c.Housing > 0 {
		m.Crowding = -model.MoraleCrowdingPenalty * model.Clamp01((c.Pop-c.Housing)/c.Housing)
	}
	m.Commons = model.MoraleCommonsBonus * model.Clamp01(c.CommonsCoverage)
	if c.AttackedWithin(now, model.MoraleAttackWindowTicks) {
		m.Attacked = -model.MoraleAttackPenalty
	}
	return m
}

// serviceOrder sorts buildings into the order in which staff and power are
// handed out: priority descending, category rank ascending, ID ascending.
func serviceOrder(bs []*model.Building) {
	sort.SliceStable(bs, func(i, j int) bool {
		a, b := bs[i], bs[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		ra, rb := a.Def().Category.Rank(), b.Def().Category.Rank()
		if ra != rb {
			return ra < rb
		}
		return a.ID < b.ID
	})
}

// stepEconomy runs one economy step of dt game-seconds for every colony.
func stepEconomy(w *model.World, dt float64) {
	for _, r := range w.Rocks {
		if r.Colony != nil {
			stepColony(w, r, dt)
		}
	}
	for _, f := range w.Factions {
		if !f.Alive {
			continue
		}
		if p := w.TotalPop(f.ID); p > f.Stats.PeakPop {
			f.Stats.PeakPop = p
		}
	}
}

func stepColony(w *model.World, r *model.Rock, dt float64) {
	c := r.Colony
	fac := w.Faction(r.Owner)
	if fac == nil {
		return
	}
	mods := fac.Mods
	frac := dt / model.SecondsPerCycle

	// 1. Capacity caches and the list of running buildings.
	var ops []*model.Building
	housing, storage := 0.0, 0.0
	for _, b := range r.Buildings {
		if b.Owner != r.Owner {
			continue
		}
		if !b.Operational() {
			b.Powered, b.Workers, b.Staffed, b.Eff, b.PowerOut = false, 0, 0, 0, 0
			b.Idle = model.IdleNone
			if b.Completed() && !b.Enabled {
				b.Idle = model.IdleDisabled
			}
			continue
		}
		def := b.Def()
		housing += def.Housing
		storage += def.Storage
		ops = append(ops, b)
	}
	c.Housing, c.StorageCap = housing, storage

	// 2. Staffing.
	serviceOrder(ops)
	c.Workers = int(math.Floor(c.Pop))
	left := c.Workers
	needed := 0
	for _, b := range ops {
		need := mods.StaffNeeded(b.Def().Staff)
		needed += need
		got := min(need, left)
		left -= got
		b.Workers = got
		b.Staffed = 1
		if need > 0 {
			b.Staffed = float64(got) / float64(need)
		}
		b.Idle = model.IdleNone
	}
	c.WorkersNeeded, c.WorkersUsed = needed, c.Workers-left

	// 3. Power.
	supply := 0.0
	for _, b := range ops {
		def := b.Def()
		if !def.IsPowerProducer() {
			continue
		}
		out := def.Power * b.Staffed
		if def.SolarPower {
			out *= r.Solar * mods.OutputMul(b.Kind) * w.FlareBonus()
		}
		if !def.Inputs.IsZero() && out > 0 {
			burn := def.Inputs.Scaled(frac * b.Staffed)
			if c.Stock.Has(burn) {
				c.Pay(burn)
				for g, amt := range burn {
					c.NoteConsumed(model.Good(g), amt)
				}
			} else {
				out = 0
				b.Idle = model.IdleNoInput
			}
		}
		if out <= 0 && b.Idle == model.IdleNone {
			b.Idle = model.IdleNoStaff
		}
		b.Powered = true
		b.PowerOut = out
		b.Eff = 0
		if out > 0 {
			b.Eff = b.Staffed
		}
		supply += out
	}
	demand := 0.0
	remaining := supply
	mf := model.MoraleFactor(c.Morale)
	for _, b := range ops {
		def := b.Def()
		if def.IsPowerProducer() {
			continue
		}
		draw := def.PowerDraw()
		demand += draw
		b.Powered = draw <= remaining+1e-9
		if b.Powered {
			remaining -= draw
			b.PowerOut = -draw
		} else {
			b.PowerOut = 0
		}
		// 4. Efficiency.
		switch {
		case !b.Powered:
			b.Eff = 0
			b.Idle = model.IdleNoPower
		case b.Staffed <= 0:
			b.Eff = 0
			b.Idle = model.IdleNoStaff
		case draw == 0 && def.Staff == 0:
			b.Eff = 1 // passive structures do not care about morale
		default:
			b.Eff = b.Staffed * mf
		}
	}
	c.PowerSupply, c.PowerDemand = supply, demand

	// 5. Production and building functions.
	lifeSupport, commons := 0.0, 0.0
	dockRepair := 0.0
	for _, b := range ops {
		def := b.Def()
		lifeSupport += def.LifeSupport * lifeEff(b)
		if b.Eff <= 0 {
			continue
		}
		switch {
		case def.Produces():
			produce(c, fac, b, frac, mods)
		case def.MineRate > 0:
			mine(r, c, fac, b, frac, mods)
		case def.SurveyRadius > 0:
			survey(r, b, dt, mods)
		case def.ShieldPool > 0:
			b.Shield = min(def.ShieldPool, b.Shield+def.ShieldRegen*b.Eff*dt)
		case def.RepairRate > 0:
			mend(r, c, b, dt)
		}
		commons += def.MoraleCover * b.Eff
		dockRepair += def.DockRepairRate * b.Eff
	}
	c.LifeSupport = lifeSupport

	// Quiet colonies patch themselves up slowly.
	if !c.AttackedWithin(w.Tick, model.SelfRepairQuietTicks) {
		for _, b := range r.Buildings {
			if b.Completed() && b.HP < b.MaxHP {
				b.HP = min(b.MaxHP, b.HP+model.SelfRepairRate*dt)
			}
		}
	}
	// Dock Rings service friendly ships in orbit.
	if dockRepair > 0 {
		for _, fl := range w.Fleets {
			if fl.At != r.ID || fl.Owner != r.Owner {
				continue
			}
			for i := range fl.Ships {
				s := &fl.Ships[i]
				s.HP = min(s.MaxHP, s.HP+dockRepair*dt)
			}
		}
	}

	// 6. Life support.
	needFood := c.Pop * model.FoodPerColonist * frac
	gotFood := c.Take(model.Food, needFood)
	c.NoteConsumed(model.Food, gotFood)
	c.Starving = c.Pop > 0 && gotFood < needFood-model.StockEpsilon
	needIce := c.Pop * model.IcePerColonist * frac
	gotIce := c.Take(model.Ice, needIce)
	c.NoteConsumed(model.Ice, gotIce)
	c.Suffocating = c.Pop > 0 && (gotIce < needIce-model.StockEpsilon || c.LifeSupport < math.Floor(c.Pop))

	// 7. Morale.
	c.CommonsCoverage = model.Clamp01(commons / max(c.Pop, 1))
	c.MoraleTarget = MoraleOf(c, w.Tick).Total()
	drift := model.MoraleDriftPerSecond * dt
	switch {
	case c.Morale < c.MoraleTarget:
		c.Morale = min(c.MoraleTarget, c.Morale+drift)
	case c.Morale > c.MoraleTarget:
		c.Morale = max(c.MoraleTarget, c.Morale-drift)
	}

	// 8. Population.
	rate := 0.0
	if c.Morale >= model.PopGrowthMinMorale && !c.Starving && !c.Suffocating &&
		c.Pop < c.Housing && c.Pop <= c.Housing*model.PopCrowdingLimit {
		rate += max(model.PopGrowthBase, model.PopGrowthRate*c.Pop) * c.Morale / model.PopGrowthMoraleRef
	}
	if c.Starving {
		rate -= model.PopLossBase + model.PopLossRate*c.Pop
	}
	if c.Suffocating {
		rate -= model.PopLossBase + model.PopLossRate*c.Pop
	}
	if c.Morale < model.PopUnrestMorale {
		rate -= model.PopUnrestLossRate * c.Pop
	}
	c.PopRate = rate
	c.Pop = max(0, c.Pop+rate*frac)
	if rate > 0 && c.Pop > c.Housing {
		c.Pop = c.Housing
	}

	// 9. Stipend.
	fac.AddScrip(model.StipendPerColonist * c.Pop * frac)

	c.RollRates(dt)
}

// lifeEff is the efficiency an Aerator contributes life support with. The
// Keystone's built-in life support always counts in full.
func lifeEff(b *model.Building) float64 {
	if b.Kind == model.Keystone {
		return 1
	}
	return min(1, b.Eff)
}

// produce runs one step of a goods-converting building (Greenhouse, Smelter,
// Cracker, Fabricator), limited by inputs on hand and by storage for outputs.
func produce(c *model.Colony, fac *model.Faction, b *model.Building, frac float64, mods model.Mods) {
	def := b.Def()
	in := def.Inputs.Scaled(frac * b.Eff)
	out := def.Outputs.Scaled(frac * b.Eff * mods.OutputMul(b.Kind))
	k := 1.0
	lackInput := false
	for g := range in {
		if in[g] > 0 {
			if r := c.Stock[g] / in[g]; r < k {
				k, lackInput = r, true
			}
		}
	}
	for g := range out {
		if out[g] > 0 {
			if r := c.Free(model.Good(g)) / out[g]; r < k {
				k, lackInput = r, false
			}
		}
	}
	if k < 1e-6 {
		if lackInput {
			b.Idle = model.IdleNoInput
		} else {
			b.Idle = model.IdleStorageFull
		}
		return
	}
	for g := range in {
		if in[g] > 0 {
			took := c.Take(model.Good(g), in[g]*k)
			c.NoteConsumed(model.Good(g), took)
		}
	}
	for g := range out {
		if out[g] > 0 {
			added := c.AddClamped(model.Good(g), out[g]*k)
			c.NoteProduced(model.Good(g), added)
			fac.Stats.Produced[g] += added
		}
	}
}

// mine runs one step of a Bore Rig: its output is split evenly between the
// deposit tiles in reach (its own tile and the six neighbours).
func mine(r *model.Rock, c *model.Colony, fac *model.Faction, b *model.Building, frac float64, mods model.Mods) {
	var tiles [7]*model.Tile
	n := 0
	if t := r.TileAt(b.Hex); t != nil && t.HasDeposit() {
		tiles[n] = t
		n++
	}
	for _, nb := range b.Hex.Neighbors() {
		if t := r.TileAt(nb); t != nil && t.HasDeposit() {
			tiles[n] = t
			n++
		}
	}
	if n == 0 {
		b.Idle = model.IdleNoDeposit
		return
	}
	share := b.Def().MineRate * mods.OutputMul(b.Kind) * b.Eff * frac / float64(n)
	mined := 0.0
	for _, t := range tiles[:n] {
		amt := min(share, t.Amount, c.Free(t.Deposit))
		if amt <= 0 {
			continue
		}
		t.Amount -= amt
		if t.Amount < 1e-6 {
			t.Amount = 0
		}
		c.Stock[t.Deposit] += amt
		c.NoteProduced(t.Deposit, amt)
		fac.Stats.Mined[t.Deposit] += amt
		mined += amt
	}
	if mined <= 0 {
		b.Idle = model.IdleStorageFull
	}
}

// survey runs one step of a Survey Mast: it reveals the nearest hidden tile
// in range every SurveyInterval seconds.
func survey(r *model.Rock, b *model.Building, dt float64, mods model.Mods) {
	def := b.Def()
	radius := def.SurveyRadius + mods.SurveyBonus
	b.SurveyAcc += dt * b.Eff
	for b.SurveyAcc >= def.SurveyInterval {
		h, ok := r.NearestHidden(b.Hex, radius, b.Owner)
		if !ok {
			b.SurveyAcc = 0
			b.Idle = model.IdleNoWork
			return
		}
		r.TileAt(h).Reveal(b.Owner)
		b.SurveyAcc -= def.SurveyInterval
	}
}

// mend runs one step of a Mender Hub: every damaged friendly building in
// range is repaired, paid for in Alloy.
func mend(r *model.Rock, c *model.Colony, b *model.Building, dt float64) {
	def := b.Def()
	worked := false
	for _, o := range r.Buildings {
		if o.Owner != b.Owner || !o.Completed() || o.HP >= o.MaxHP || o.Hex.Dist(b.Hex) > def.RepairRadius {
			continue
		}
		hp := min(def.RepairRate*b.Eff*dt, o.MaxHP-o.HP)
		if def.RepairAlloyPerHP > 0 {
			hp = min(hp, c.Stock[model.Alloy]/def.RepairAlloyPerHP)
		}
		if hp <= 0 {
			b.Idle = model.IdleNoInput
			continue
		}
		used := c.Take(model.Alloy, hp*def.RepairAlloyPerHP)
		c.NoteConsumed(model.Alloy, used)
		o.HP += hp
		worked = true
	}
	if !worked && b.Idle == model.IdleNone {
		b.Idle = model.IdleNoWork
	}
}

// reach returns a hex and its six neighbours: the tiles a Bore Rig works.
func reach(h model.Hex) []model.Hex {
	nb := h.Neighbors()
	return append([]model.Hex{h}, nb[:]...)
}
