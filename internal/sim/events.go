package sim

import (
	"slices"

	"drifthold/internal/model"
)

// Derelict hulk contents.
const (
	hulkAlloyMin, hulkAlloyMax = 40.0, 120.0
	hulkPartsMin, hulkPartsMax = 10.0, 40.0
	hulkScripMin, hulkScripMax = 80.0, 240.0
)

// stepEvents runs one slow step of the random event system: ongoing events
// always play out; new ones are rolled only when the game has events enabled.
func stepEvents(w *model.World, dt float64) {
	ev := &w.Events
	stepSqualls(w)
	stepFlare(w, dt)
	stepSalvage(w, dt)
	if !w.Settings.Events || w.Tick < ev.NextRollTick {
		return
	}
	ev.NextRollTick = w.Tick + model.CyclesToTicks(w.RNG.Range(model.EventGapMinCycles, model.EventGapMaxCycles))
	kinds := []model.EventKind{
		model.EventSquall, model.EventFlare, model.EventRichSeam,
		model.EventHulk, model.EventRefugees, model.EventMarketShock,
	}
	weights := []float64{0.20, 0.18, 0.16, 0.20, 0.16, 0.10}
	i := w.RNG.Weighted(weights)
	if i < 0 {
		return
	}
	happened := false
	switch kinds[i] {
	case model.EventSquall:
		happened = startSquall(w)
	case model.EventFlare:
		happened = startFlare(w)
	case model.EventRichSeam:
		happened = richSeam(w)
	case model.EventHulk:
		happened = spawnHulk(w)
	case model.EventRefugees:
		happened = refugees(w)
	case model.EventMarketShock:
		g := model.Good(w.RNG.Intn(model.NumGoods))
		sign := w.RNG.Sign()
		w.Market.AddPressure(g, sign*model.MarketShockPressure)
		word := "soared"
		if sign < 0 {
			word = "collapsed"
		}
		w.Logf(model.NoFaction, model.SevInfo, model.NoRock, "Exchange report: the price of %s has %s.", g, word)
		happened = true
	}
	if happened {
		ev.LastKind, ev.LastTick = kinds[i], w.Tick
		ev.Count++
	}
}

func colonisedRocks(w *model.World) []*model.Rock {
	var out []*model.Rock
	for _, r := range w.Rocks {
		if r.Colony != nil {
			out = append(out, r)
		}
	}
	return out
}

func startSquall(w *model.World) bool {
	r, ok := model.Pick(&w.RNG, colonisedRocks(w))
	if !ok {
		return false
	}
	start := w.Tick + model.SecondsToTicks(model.SquallWarnSeconds)
	w.Events.Squalls = append(w.Events.Squalls, model.Squall{
		Rock:           r.ID,
		StartTick:      start,
		EndTick:        start + model.SecondsToTicks(model.SquallSeconds),
		ImpactsLeft:    w.RNG.IntRange(model.SquallImpactsMin, model.SquallImpactsMax),
		NextImpactTick: start,
	})
	note(w, r.Owner, model.SevWarn, r.ID, "Micrometeor squall inbound for %s!", r.Name)
	w.Emit(model.NewFX(model.FXAlert, r.ID, model.Hex{}, r.Owner))
	return true
}

func stepSqualls(w *model.World) {
	ev := &w.Events
	if len(ev.Squalls) == 0 {
		return
	}
	squalls := ev.Squalls
	ev.Squalls = nil
	for _, sq := range squalls {
		r := w.Rock(sq.Rock)
		if r == nil {
			continue
		}
		span := max(1, sq.EndTick-sq.StartTick)
		for sq.ImpactsLeft > 0 && w.Tick >= sq.NextImpactTick {
			blob := r.BlobHexes()
			h := blob[w.RNG.Intn(len(blob))]
			dmg := w.RNG.Range(model.SquallDamageMin, model.SquallDamageMax)
			w.Emit(model.NewFX(model.FXMeteorImpact, r.ID, h, model.NoFaction))
			if b := r.BuildingAt(h); b != nil && !b.Derelict() {
				damageBuilding(w, r, b, dmg, model.NoFaction)
			}
			sq.ImpactsLeft--
			sq.NextImpactTick += max(1, span/max(1, sq.ImpactsLeft+1))
		}
		if sq.ImpactsLeft > 0 {
			ev.Squalls = append(ev.Squalls, sq)
		}
	}
}

func startFlare(w *model.World) bool {
	if w.FlareActive() {
		return false
	}
	w.Events.FlareStartTick = w.Tick
	w.Events.FlareEndTick = w.Tick + model.SecondsToTicks(model.FlareSeconds)
	w.Logf(model.NoFaction, model.SevWarn, model.NoRock,
		"Emberfall is flaring: Sunvanes surge, sensors are half blind and ships in transit take hull damage.")
	w.Emit(model.NewFX(model.FXFlareStart, model.NoRock, model.Hex{}, model.NoFaction))
	return true
}

func stepFlare(w *model.World, dt float64) {
	ev := &w.Events
	if ev.FlareEndTick == 0 {
		return
	}
	if w.Tick >= ev.FlareEndTick {
		if w.Tick-model.SlowTickInterval < ev.FlareEndTick {
			w.Logf(model.NoFaction, model.SevInfo, model.NoRock, "The flare has passed.")
			w.Emit(model.NewFX(model.FXFlareEnd, model.NoRock, model.Hex{}, model.NoFaction))
		}
		return
	}
	if !w.FlareActive() {
		return
	}
	for _, fl := range w.Fleets {
		if !fl.InTransit() {
			continue
		}
		for i := range fl.Ships {
			s := &fl.Ships[i]
			s.HP = max(1, s.HP-s.MaxHP*model.FlareHullLossFrac*dt/model.FlareSeconds)
		}
	}
}

func richSeam(w *model.World) bool {
	type spot struct {
		r *model.Rock
		b *model.Building
	}
	var rigs []spot
	for _, r := range colonisedRocks(w) {
		for _, b := range r.Buildings {
			if b.Def().MineRate > 0 && b.Working() {
				rigs = append(rigs, spot{r, b})
			}
		}
	}
	s, ok := model.Pick(&w.RNG, rigs)
	if !ok {
		return false
	}
	var tiles []*model.Tile
	for _, h := range reach(s.b.Hex) {
		if t := s.r.TileAt(h); t != nil && t.Terrain.Solid() && t.Deposit != model.GoodNone && t.Deposit.Raw() &&
			(t.Amount > 0 || t.KnownTo(s.b.Owner)) && t.Deposit.Valid() {
			tiles = append(tiles, t)
		}
	}
	t, ok := model.Pick(&w.RNG, tiles)
	if !ok {
		return false
	}
	t.Amount += float64(int(w.RNG.Range(model.SeamAmountMin, model.SeamAmountMax)))
	t.Reveal(s.b.Owner)
	note(w, s.b.Owner, model.SevGood, s.r.ID, "A Bore Rig on %s has struck a rich seam of %s.", s.r.Name, t.Deposit)
	return true
}

func spawnHulk(w *model.World) bool {
	var free []*model.Rock
	for _, r := range w.Rocks {
		if r.Salvage == nil {
			free = append(free, r)
		}
	}
	r, ok := model.Pick(&w.RNG, free)
	if !ok {
		return false
	}
	r.Salvage = &model.Salvage{
		Goods: model.Stock{
			model.Alloy: float64(int(w.RNG.Range(hulkAlloyMin, hulkAlloyMax))),
			model.Parts: float64(int(w.RNG.Range(hulkPartsMin, hulkPartsMax))),
		},
		Scrip:      float64(int(w.RNG.Range(hulkScripMin, hulkScripMax))),
		SpawnTick:  w.Tick,
		ExpireTick: w.Tick + model.SalvageLifetimeTicks,
	}
	w.Logf(model.NoFaction, model.SevInfo, r.ID, "A derelict hulk has drifted into orbit of %s. First to reach it keeps the salvage.", r.Name)
	return true
}

// stepSalvage lets the rock's owner, or the first fleet in orbit, claim a
// derelict hulk, and removes hulks that have drifted away.
func stepSalvage(w *model.World, dt float64) {
	for _, r := range w.Rocks {
		sv := r.Salvage
		if sv == nil {
			continue
		}
		if w.Tick >= sv.ExpireTick {
			r.Salvage = nil
			continue
		}
		claimant := r.Owner
		var claimFleet *model.Fleet
		if claimant == model.NoFaction {
			for _, fl := range w.Fleets {
				if fl.At == r.ID && !fl.Empty() {
					claimant, claimFleet = fl.Owner, fl
					break
				}
			}
		}
		if claimant != sv.Claimant {
			sv.Claimant, sv.ClaimSeconds = claimant, 0
		}
		if claimant == model.NoFaction {
			continue
		}
		sv.ClaimSeconds += dt
		if sv.ClaimSeconds < model.SalvageClaimSeconds-1e-9 {
			continue
		}
		dest := r.Colony
		if dest == nil && claimFleet != nil {
			dest = w.Colony(HomeFor(w, claimFleet))
		}
		if dest != nil {
			dest.AddStockClamped(sv.Goods)
		}
		if fac := w.Faction(claimant); fac != nil {
			fac.AddScrip(sv.Scrip)
			note(w, claimant, model.SevGood, r.ID, "Salvage recovered at %s: %s and %s Scrip.",
				r.Name, sv.Goods, model.FormatQty(sv.Scrip))
		}
		r.Salvage = nil
	}
}

func refugees(w *model.World) bool {
	n := float64(w.RNG.IntRange(model.RefugeesMin, model.RefugeesMax))
	cands := slices.DeleteFunc(colonisedRocks(w), func(r *model.Rock) bool {
		return r.Colony.Housing-r.Colony.Pop < n
	})
	r, ok := model.Pick(&w.RNG, cands)
	if !ok {
		return false
	}
	r.Colony.Pop += n
	note(w, r.Owner, model.SevGood, r.ID, "A refugee convoy has joined %s: %s new colonists.", r.Name, model.FormatQty(n))
	return true
}
