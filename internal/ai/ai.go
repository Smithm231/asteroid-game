// Package ai contains the brains of the rival Holds. A brain looks at the
// world exactly as a player could (it respects hidden deposits and sensor
// range) and acts only through the same commands a player has.
package ai

import (
	"drifthold/internal/model"
	"drifthold/internal/sim"
)

// Timer slots in model.AIState.Timers.
const (
	tNextAttack = iota // earliest tick of the next attack wave
	tNextTrade         // earliest tick of the next market action
	tStuckSince        // tick since which the top building need has been unaffordable
	numTimers
)

// profile is what distinguishes the three rival personalities.
type profile struct {
	research    []model.TechID // preferred research order
	colonies    int            // how many colonies it wants
	flakHome    int            // Flak Batteries wanted on the home rock
	firstAttack float64        // earliest cycle of the first attack (Normal difficulty)
	waveSize    int            // combat ships in the first wave
	waveGrowth  int            // extra ships per later wave
	waveGap     float64        // cycles between waves
	stance      model.Stance
	retreat     float64
	homeGuard   int // combat ships kept at home
	labs        int // Laboratories wanted on the home rock (late game)
}

var commonLate = []model.TechID{
	model.CharterTrade, model.DockEngineering, model.EfficientSmelting, model.Photovoltaics, model.HydroStacks,
	model.DeepScan, model.FieldRepair, model.MassAcceleration,
	model.ReactivePlating, model.OverdriveThrusters, model.Automation, model.LumenOptics,
	model.AegisFields, model.CapitalHulls, model.LumenFusion,
}

func profileOf(p model.Personality) profile {
	switch p {
	case model.PersonalityVane: // raiders: early, frequent light attacks
		return profile{
			research: append([]model.TechID{
				model.OrbitalFlight, model.PointDefence, model.Fabrication, model.VolatileCracking,
				model.Gunnery, model.DeepBoring, model.SiegeDoctrine, model.DockEngineering, model.CivicPlanning,
				model.ColonialCharter,
			}, commonLate...),
			colonies: 2, flakHome: 2, firstAttack: 14, waveSize: 4, waveGrowth: 1, waveGap: 6,
			stance: model.StanceRaid, retreat: model.RetreatHalf, homeGuard: 1, labs: 2,
		}
	case model.PersonalityMeridian: // expansionist: many rocks, medium fleets
		return profile{
			research: append([]model.TechID{
				model.OrbitalFlight, model.Fabrication, model.VolatileCracking, model.ColonialCharter,
				model.PointDefence, model.DeepBoring, model.CivicPlanning, model.Gunnery, model.SiegeDoctrine,
			}, commonLate...),
			colonies: 5, flakHome: 2, firstAttack: 22, waveSize: 6, waveGrowth: 2, waveGap: 9,
			stance: model.StanceRaze, retreat: model.RetreatQuarter, homeGuard: 2, labs: 2,
		}
	default: // Ostrander: industrial turtle, late heavy fleets
		return profile{
			research: append([]model.TechID{
				model.Fabrication, model.DeepBoring, model.OrbitalFlight, model.PointDefence,
				model.VolatileCracking, model.CivicPlanning, model.EfficientSmelting, model.Gunnery,
				model.ColonialCharter, model.SiegeDoctrine,
			}, commonLate...),
			colonies: 3, flakHome: 4, firstAttack: 30, waveSize: 9, waveGrowth: 3, waveGap: 12,
			stance: model.StanceSuppress, retreat: model.RetreatQuarter, homeGuard: 3, labs: 3,
		}
	}
}

// difficultyDelay stretches or shrinks the attack timetable.
func difficultyDelay(d model.Difficulty) float64 {
	switch d {
	case model.DiffEasy:
		return 1.6
	case model.DiffHard:
		return 0.7
	}
	return 1
}

// Step lets the brain of faction f think once. It is the function to install
// as sim.Sim.AI.
func Step(w *model.World, f model.FactionID) {
	fac := w.Faction(f)
	if fac == nil || !fac.Alive || w.GameOver {
		return
	}
	st := &fac.AI
	for len(st.Timers) < numTimers {
		st.Timers = append(st.Timers, 0)
	}
	b := &brain{w: w, f: f, fac: fac, st: st, prof: profileOf(fac.Personality)}
	b.research()
	for _, r := range w.RocksOf(f) {
		b.colony(r)
	}
	b.shipyards()
	b.fleets()
	b.trade()
}

// brain is the per-step working state of one faction's AI.
type brain struct {
	w    *model.World
	f    model.FactionID
	fac  *model.Faction
	st   *model.AIState
	prof profile
}

func (b *brain) exec(c model.Command) error { return sim.Exec(b.w, c) }

// ----------------------------------------------------------------- research

func (b *brain) research() {
	rs := &b.fac.Research
	if rs.Current != model.TechNone && !rs.Stalled {
		return
	}
	for _, t := range b.prof.research {
		if !b.fac.CanResearch(t) {
			continue
		}
		if def := model.TechDefOf(t); def.Lumen > 0 && !rs.Started[t] &&
			b.w.FactionTotal(b.f, model.Lumen) < def.Lumen {
			continue
		}
		if b.exec(model.SetResearchCmd(b.f, t)) == nil {
			return
		}
	}
}

// -------------------------------------------------------------------- trade

func (b *brain) trade() {
	if b.w.Tick < b.st.Timers[tNextTrade] {
		return
	}
	b.st.Timers[tNextTrade] = b.w.Tick + model.SecondsToTicks(20)
	for _, r := range b.w.RocksOf(b.f) {
		if !sim.CanTrade(r) {
			continue
		}
		c := r.Colony
		// Sell what is overflowing.
		for _, g := range model.AllGoods() {
			if c.Stock[g] > 0.9*c.StorageCap && c.StorageCap > 0 {
				b.exec(model.TradeCmd(b.f, r.ID, g, -min(80, c.Stock[g]-0.6*c.StorageCap)))
			}
		}
		// Buy what the colony is starved of.
		type want struct {
			g     model.Good
			below float64
		}
		for _, wnt := range []want{{model.Food, 25}, {model.Ice, 25}, {model.Alloy, 40}, {model.Silica, 30}, {model.Parts, 20}, {model.Fuel, 15}} {
			if c.Stock[wnt.g] < wnt.below && b.fac.Scrip > 150 {
				b.exec(model.TradeCmd(b.f, r.ID, wnt.g, min(40, c.Free(wnt.g))))
			}
		}
		if b.fac.Scrip > 500 && c.Housing-c.Pop >= model.SponsorColonists && !c.Starving && !c.Suffocating {
			b.exec(model.SponsorCmd(b.f, r.ID))
		}
	}
}
