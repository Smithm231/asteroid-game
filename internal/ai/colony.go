package ai

import (
	"errors"

	"drifthold/internal/model"
	"drifthold/internal/sim"
)

// view is what the brain works out about one of its colonies before deciding
// what to build there.
type view struct {
	r     *model.Rock
	c     *model.Colony
	home  bool
	count [model.NumBuildingKinds]int // own buildings, sites included
	sites int                         // construction sites
	// Projected values once every site is finished.
	supply, demand       float64
	housing, lifeSupport float64
	food                 float64 // Food per cycle
	unknown              int     // tiles not yet revealed
}

// want is one entry of a colony's shopping list.
type want struct {
	kind model.BuildingKind
	good model.Good // deposit a Bore Rig is meant for, else GoodNone
}

func (b *brain) look(r *model.Rock) *view {
	v := &view{r: r, c: r.Colony, home: r.ID == b.fac.HomeRock}
	mods := b.fac.Mods
	for _, bd := range r.Buildings {
		if bd.Owner != b.f {
			continue
		}
		def := bd.Def()
		v.count[bd.Kind]++
		if bd.Constructing() {
			v.sites++
		}
		if !bd.Enabled && bd.Kind != model.Keystone {
			continue
		}
		switch {
		case def.SolarPower:
			v.supply += def.Power * r.Solar * mods.SolarMul
		case def.Power > 0:
			v.supply += def.Power
		default:
			v.demand += def.PowerDraw()
		}
		v.housing += def.Housing
		v.lifeSupport += def.LifeSupport
		v.food += def.Outputs[model.Food] * mods.OutputMul(bd.Kind)
	}
	for i := range r.Tiles {
		t := &r.Tiles[i]
		if t.Terrain.Solid() && !t.KnownTo(b.f) {
			v.unknown++
		}
	}
	return v
}

// knownDeposit reports whether the faction knows of material of good g under
// hex h.
func (b *brain) knownDeposit(r *model.Rock, h model.Hex, g model.Good) bool {
	got, amt, ok := r.VisibleDeposit(h, b.f)
	return ok && got == g && amt > 0
}

// rigsOn counts the faction's Bore Rigs (sites included) that reach a known
// deposit of good g.
func (b *brain) rigsOn(r *model.Rock, g model.Good) int {
	n := 0
	for _, bd := range r.Buildings {
		if bd.Kind != model.BoreRig || bd.Owner != b.f {
			continue
		}
		for _, h := range reach(bd.Hex) {
			if b.knownDeposit(r, h, g) {
				n++
				break
			}
		}
	}
	return n
}

// knowsAny reports whether the faction knows of any material of good g on
// the rock.
func (b *brain) knowsAny(r *model.Rock, g model.Good) bool {
	for i := range r.Tiles {
		t := &r.Tiles[i]
		if t.HasDeposit() && t.Deposit == g && (t.KnownTo(b.f) || r.Surveyed(b.f)) {
			return true
		}
	}
	return false
}

func reach(h model.Hex) []model.Hex {
	nb := h.Neighbors()
	return append([]model.Hex{h}, nb[:]...)
}

// wants returns the colony's shopping list, most urgent first.
func (b *brain) wants(v *view) []want {
	c, r, fac := v.c, v.r, b.fac
	var list []want
	idle := c.IdleWorkers()
	// add wants a building regardless of spare hands; staffed wants it only
	// when there are colonists free to run it.
	add := func(k model.BuildingKind) {
		if fac.CanBuild(k) {
			list = append(list, want{k, model.GoodNone})
		}
	}
	staffed := func(k model.BuildingKind) {
		if fac.Mods.StaffNeeded(model.BuildingDefOf(k).Staff) <= idle {
			add(k)
		}
	}
	rig := func(g model.Good) { list = append(list, want{model.BoreRig, g}) }
	spare := idle >= fac.Mods.StaffNeeded(model.BuildingDefOf(model.BoreRig).Staff)
	cycle := b.w.Cycle()
	foodNeed := (c.Pop + 6) * model.FoodPerColonist
	canFarm := c.Stock[model.Ice] > 15 || b.rigsOn(r, model.Ice) > 0

	// Emergencies first: people who cannot breathe or eat.
	if c.Suffocating && v.lifeSupport < c.Pop {
		add(model.Aerator)
	}
	if (c.Starving || c.Stock[model.Food] < 40) && v.food < foodNeed && canFarm {
		add(model.Greenhouse)
	}
	// Keep the lights on.
	if v.demand+6 > v.supply {
		switch {
		case fac.CanBuild(model.LumenCore) && c.Stock[model.Lumen] > 30 && v.demand > 150:
			add(model.LumenCore)
		case fac.CanBuild(model.ThermalStack) && r.Solar < 0.7 && c.Stock[model.Fuel] > 60 &&
			c.Stock[model.Ice] > 300 && v.count[model.Cracker] > 0 && idle >= 2:
			add(model.ThermalStack)
		default:
			add(model.Sunvane)
		}
	}
	// The backbone: Ore to Alloy.
	oreRigs := b.rigsOn(r, model.Ore)
	if oreRigs == 0 {
		rig(model.Ore)
	}
	if v.count[model.Smelter] == 0 && (oreRigs > 0 || c.Stock[model.Ore] > 60) {
		add(model.Smelter)
	}
	// Beds: more colonists are more hands, as long as there is work for them.
	if c.Pop > v.housing-3 && idle < 8 {
		add(model.Habitat)
	}
	// Building material and water before anything fancy.
	if b.rigsOn(r, model.Silica) == 0 && c.Stock[model.Silica] < 120 {
		rig(model.Silica)
	}
	iceRigs := b.rigsOn(r, model.Ice)
	if iceRigs == 0 && c.Stock[model.Ice] < 200 {
		rig(model.Ice)
	}
	// Air and food for the colonists to come.
	if v.lifeSupport < c.Pop+4 {
		add(model.Aerator)
	}
	if v.food < foodNeed && canFarm {
		add(model.Greenhouse)
	}
	// Grow the Alloy line in step: Ore piling up wants another Smelter,
	// Smelters running dry want another rig on Ore.
	if v.count[model.Smelter] < 6 && c.Stock[model.Ore] > 100 {
		staffed(model.Smelter)
	}
	if spare && oreRigs > 0 && oreRigs <= v.count[model.Smelter] && c.Stock[model.Ore] < 60 && v.count[model.BoreRig] < 12 {
		rig(model.Ore)
	}
	// Ice is life: never sit on a single thin supply.
	if spare && iceRigs > 0 && iceRigs < 3 && c.Stock[model.Ice] < 120 {
		rig(model.Ice)
	}
	if v.home && v.count[model.Laboratory] < b.prof.labs && c.Pop >= float64(8+12*v.count[model.Laboratory]) {
		staffed(model.Laboratory)
	}
	if v.unknown > 20 && v.count[model.SurveyMast] == 0 {
		staffed(model.SurveyMast)
	}
	// Technology buildings.
	if v.home {
		if v.count[model.Fabricator] == 0 {
			staffed(model.Fabricator)
		}
		if v.count[model.LaunchCradle] == 0 {
			staffed(model.LaunchCradle)
		}
	}
	if v.count[model.Cracker] == 0 && (iceRigs > 0 || c.Stock[model.Ice] > 150) && c.Stock[model.Ice] > 100 &&
		(v.home || c.Stock[model.Fuel] < 30) {
		staffed(model.Cracker)
	}
	// A fleet that cannot launch is no fleet: keep the Fuel coming.
	if v.home && v.count[model.Cracker] == 1 && c.Stock[model.Fuel] < 120 && c.Stock[model.Ice] > 220 && b.w.ShipCount(b.f) >= 5 {
		staffed(model.Cracker)
	}
	flak := 1
	if v.home {
		flak = b.prof.flakHome
	}
	if c.AttackedWithin(b.w.Tick, 4*model.TicksPerCycle) {
		flak += 2
	}
	if v.count[model.FlakBattery] < flak && (cycle > 10 || c.LastAttackedTick > 0) {
		staffed(model.FlakBattery)
	}
	if c.Pop >= 22 && float64(v.count[model.Commons])*40 < c.Pop {
		staffed(model.Commons)
	}
	// More storage only for goods worth hoarding; raw material may overflow.
	for _, g := range []model.Good{model.Alloy, model.Parts, model.Food, model.Fuel, model.Ice} {
		if c.StorageCap > 0 && c.Stock[g] > 0.85*c.StorageCap && v.count[model.Silo] < 6 {
			add(model.Silo)
			break
		}
	}
	if v.home {
		if v.count[model.ExchangeDock] == 0 && cycle > 12 {
			staffed(model.ExchangeDock)
		}
		if v.count[model.LaunchCradle] > 0 && b.w.BerthsFree(b.f) <= 1 && v.count[model.DockRing] < 4 {
			staffed(model.DockRing)
		}
		if v.count[model.LaunchCradle] == 1 && c.Pop >= 45 && cycle > 30 {
			staffed(model.LaunchCradle)
		}
		if v.count[model.FarseerArray] == 0 && cycle > 25 {
			staffed(model.FarseerArray)
		}
		if v.count[model.MenderHub] == 0 && c.LastAttackedTick > 0 {
			staffed(model.MenderHub)
		}
		if v.count[model.LanceTower] < 2 && c.Stock[model.Lumen] >= 10 {
			staffed(model.LanceTower)
		}
		if v.count[model.AegisProjector] == 0 && c.Stock[model.Lumen] >= 15 {
			staffed(model.AegisProjector)
		}
		if v.count[model.MassDriver] == 0 && cycle > 35 {
			staffed(model.MassDriver)
		}
		if v.count[model.Fabricator] < 2 && c.Stock[model.Alloy] > 200 && c.Stock[model.Parts] < 60 {
			staffed(model.Fabricator)
		}
	}
	// Spare hands: another rig on whatever known raw good is scarcest here.
	if idle >= 4 && v.count[model.BoreRig] < 12 {
		scarce, least := model.GoodNone, 1e18
		for _, g := range []model.Good{model.Ice, model.Silica, model.Lumen, model.Ore} {
			if c.Stock[g] < least && c.Stock[g] < 0.5*c.StorageCap && b.knowsAny(r, g) {
				scarce, least = g, c.Stock[g]
			}
		}
		if scarce != model.GoodNone {
			rig(scarce)
		}
	}
	return list
}

// colony runs the building decisions of one colony for this step.
func (b *brain) colony(r *model.Rock) {
	v := b.look(r)
	b.derelicts(v)
	b.expedite(v)
	b.switches(v)
	limit := 1
	if v.home {
		limit = 2
		if v.c.Stock[model.Alloy] > 300 {
			limit = 3
		}
	}
	if v.sites >= limit {
		return
	}
	stuck := b.st.Timers[tStuckSince]
	patient := stuck == 0 || b.w.Tick-stuck < 3*model.TicksPerCycle
	for _, wn := range b.wants(v) {
		err := b.build(v, wn)
		if err == nil {
			b.st.Timers[tStuckSince] = 0
			return
		}
		var short *model.ShortageError
		if errors.As(err, &short) {
			if b.remedy(v, short.Good) {
				return // working on the shortage itself
			}
			if stuck == 0 {
				b.st.Timers[tStuckSince] = b.w.Tick
			}
			if patient {
				return // save up for the most urgent thing
			}
		}
	}
}

// remedy tries to start fixing a shortage of good g at its source. It reports
// whether it placed something.
func (b *brain) remedy(v *view, g model.Good) bool {
	switch g {
	case model.Silica, model.Ice:
		// A rig already standing on it may simply have no crew: put it first
		// in line for workers.
		for _, bd := range v.r.Buildings {
			if bd.Kind == model.BoreRig && bd.Owner == b.f && bd.Priority != model.PriorityHigh {
				for _, h := range reach(bd.Hex) {
					if b.knownDeposit(v.r, h, g) {
						b.exec(model.SetPriorityCmd(b.f, v.r.ID, bd.ID, model.PriorityHigh))
						break
					}
				}
			}
		}
		if b.rigsOn(v.r, g) < 1 {
			return b.build(v, want{model.BoreRig, g}) == nil
		}
	case model.Alloy:
		if b.rigsOn(v.r, model.Ore) == 0 {
			return b.build(v, want{model.BoreRig, model.Ore}) == nil
		}
	case model.Parts:
		if v.count[model.Fabricator] == 0 && b.fac.CanBuild(model.Fabricator) {
			return b.build(v, want{model.Fabricator, model.GoodNone}) == nil
		}
	}
	return false
}

// errNoSite means no tile suits the wanted building right now.
var errNoSite = errors.New("no suitable site")

// build places the wanted building at the best site, or, for a Bore Rig with
// no deposit in reach, a Conduit toward one or a Survey Mast to find one.
func (b *brain) build(v *view, wn want) error {
	kind := wn.kind
	var site model.Hex
	ok := false
	if kind == model.BoreRig && wn.good != model.GoodNone {
		site, ok = b.rigSite(v, wn.good)
		if !ok {
			if step, found := b.pathStep(v, wn.good); found {
				kind, site, ok = model.Conduit, step, true
			} else if v.unknown > 0 && v.count[model.SurveyMast] < 3 {
				// Nothing known in reach: look further afield.
				kind = model.SurveyMast
				site, ok = b.generalSite(v, kind)
			}
		}
	} else {
		site, ok = b.generalSite(v, kind)
	}
	if !ok {
		return errNoSite
	}
	return b.exec(model.BuildCmd(b.f, v.r.ID, site, kind))
}

// placeable lists the tiles where kind could be placed, ignoring cost.
func (b *brain) placeable(v *view, kind model.BuildingKind) []model.Hex {
	var out []model.Hex
	for i := range v.r.Tiles {
		t := &v.r.Tiles[i]
		if !t.Buildable() {
			continue
		}
		h, _ := v.r.HexAt(i)
		if sim.CanPlaceIgnoringCost(b.w, b.f, v.r.ID, h, kind) == nil {
			out = append(out, h)
		}
	}
	return out
}

// rigSite picks the tile from which a new Bore Rig reaches the most known
// material of good g, avoiding tiles already worked by another rig.
func (b *brain) rigSite(v *view, g model.Good) (model.Hex, bool) {
	best, bestScore := model.Hex{}, 0.0
	for _, h := range b.placeable(v, model.BoreRig) {
		score := 0.0
		for _, x := range reach(h) {
			got, amt, ok := v.r.VisibleDeposit(x, b.f)
			if !ok || amt <= 0 {
				continue
			}
			if got == g {
				score += 200 + min(amt, 1200)
			} else {
				score -= 120 // other material dilutes the rig's output
			}
		}
		if score < 200 {
			continue
		}
		for _, o := range v.r.Buildings {
			if o.Kind == model.BoreRig && o.Owner == b.f && o.Hex.Dist(h) <= 2 {
				score *= 0.5
			}
		}
		if score > bestScore {
			best, bestScore = h, score
		}
	}
	return best, bestScore > 0
}

// pathStep finds the shortest chain of open tiles from the colony's footprint
// to a tile from which a Bore Rig would reach known material of good g, and
// returns the first tile of that chain: where the next Conduit goes.
func (b *brain) pathStep(v *view, g model.Good) (model.Hex, bool) {
	r := v.r
	prev := make([]int32, len(r.Tiles))
	for i := range prev {
		prev[i] = -1
	}
	var queue []int32
	for i := range r.Tiles {
		if !r.Tiles[i].Buildable() {
			continue
		}
		if h, ok := r.HexAt(i); ok && r.AdjacentToOwned(h, b.f) {
			prev[i] = int32(i)
			queue = append(queue, int32(i))
		}
	}
	for qi := 0; qi < len(queue); qi++ {
		i := queue[qi]
		h, _ := r.HexAt(int(i))
		if prev[i] != i {
			for _, x := range reach(h) {
				if b.knownDeposit(r, x, g) {
					for prev[i] != i {
						i = prev[i]
					}
					first, _ := r.HexAt(int(i))
					return first, true
				}
			}
		}
		for _, nb := range h.Neighbors() {
			j := r.Index(nb)
			if j < 0 || prev[j] != -1 || !r.Tiles[j].Buildable() {
				continue
			}
			prev[j] = i
			queue = append(queue, int32(j))
		}
	}
	return model.Hex{}, false
}

// generalSite picks a tile for an ordinary building: compact, close to the
// Keystone, and off known deposits so that those stay free for Bore Rigs.
func (b *brain) generalSite(v *view, kind model.BuildingKind) (model.Hex, bool) {
	ks := v.r.Keystone()
	if ks == nil {
		return model.Hex{}, false
	}
	best, bestScore, found := model.Hex{}, -1e18, false
	for _, h := range b.placeable(v, kind) {
		score := -float64(h.Dist(ks.Hex)) * 2
		if kind == model.SurveyMast {
			score = 0
			for _, x := range h.Spiral(model.BuildingDefOf(kind).SurveyRadius) {
				if t := v.r.TileAt(x); t != nil && t.Terrain.Solid() && !t.KnownTo(b.f) {
					score++
				}
			}
		}
		for _, x := range reach(h) {
			if kind == model.SurveyMast {
				break
			}
			if _, amt, ok := v.r.VisibleDeposit(x, b.f); ok && amt > 0 {
				if x == h {
					score -= 40
				} else {
					score -= 6
				}
			}
			if o := v.r.BuildingAt(x); o != nil && o.Owner == b.f {
				score += 0.5
			}
		}
		if kind == model.SurveyMast && score < 6 {
			continue // would reveal next to nothing
		}
		if !found || score > bestScore {
			best, bestScore, found = h, score, true
		}
	}
	return best, found
}

// derelicts puts the wrecks a re-founded colony inherited back to use, or
// clears them for salvage.
func (b *brain) derelicts(v *view) {
	for _, bd := range v.r.Buildings {
		if !bd.Derelict() {
			continue
		}
		if b.fac.CanBuild(bd.Kind) && b.exec(model.RecommissionCmd(b.f, v.r.ID, bd.ID)) == nil {
			return
		}
		if !b.fac.CanBuild(bd.Kind) {
			b.exec(model.DemolishCmd(b.f, v.r.ID, bd.ID))
			return
		}
	}
}

// expedite spends spare Scrip on finishing the slowest construction site.
func (b *brain) expedite(v *view) {
	if b.fac.Scrip < 700 || v.sites == 0 {
		return
	}
	var pick *model.Building
	for _, bd := range v.r.Buildings {
		if bd.Owner == b.f && bd.Constructing() &&
			(pick == nil || bd.RemainingBuildSeconds() > pick.RemainingBuildSeconds()) {
			pick = bd
		}
	}
	if pick != nil && sim.ExpediteQuote(pick) < b.fac.Scrip-450 {
		b.exec(model.ExpediteCmd(b.f, v.r.ID, pick.ID))
	}
}

// switches turns Ice-hungry industry off while Ice is short, so that the
// colonists keep breathing, and back on once the stock has recovered.
func (b *brain) switches(v *view) {
	ice := v.c.Stock[model.Ice]
	for _, bd := range v.r.Buildings {
		if bd.Owner != b.f || !bd.Completed() || bd.Kind != model.Cracker {
			continue
		}
		switch {
		case bd.Enabled && ice < 50:
			b.exec(model.SetEnabledCmd(b.f, v.r.ID, bd.ID, false))
		case !bd.Enabled && ice > 130:
			b.exec(model.SetEnabledCmd(b.f, v.r.ID, bd.ID, true))
		}
	}
}
