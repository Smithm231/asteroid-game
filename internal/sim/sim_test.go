package sim

import (
	"encoding/json"
	"testing"

	"drifthold/internal/model"
)

func newTestGame(t *testing.T, seed uint64) (*Sim, *model.World) {
	t.Helper()
	s := model.DefaultSettings(seed)
	s.Events = false
	w, err := NewGame(s)
	if err != nil {
		t.Fatalf("NewGame(%d): %v", seed, err)
	}
	return New(w), w
}

// siteFor returns a tile where faction f can place kind on rock right now,
// preferring tiles for which want returns true.
func siteFor(w *model.World, f model.FactionID, rock model.RockID, kind model.BuildingKind, want func(model.Hex) bool) (model.Hex, bool) {
	r := w.Rock(rock)
	var fallback model.Hex
	found := false
	for _, h := range r.Keystone().Hex.Spiral(2 * r.Radius) {
		if CanPlaceIgnoringCost(w, f, rock, h, kind) != nil {
			continue
		}
		if want == nil || want(h) {
			return h, true
		}
		if !found {
			fallback, found = h, true
		}
	}
	return fallback, found
}

func mustBuild(t *testing.T, w *model.World, f model.FactionID, rock model.RockID, kind model.BuildingKind, want func(model.Hex) bool) *model.Building {
	t.Helper()
	h, ok := siteFor(w, f, rock, kind, want)
	if !ok {
		t.Fatalf("no site for %v", kind)
	}
	b, err := Place(w, f, rock, h, kind)
	if err != nil {
		t.Fatalf("Place(%v at %v): %v", kind, h, err)
	}
	return b
}

func TestNewGameManySeeds(t *testing.T) {
	for seed := uint64(1); seed <= 60; seed++ {
		for _, size := range []model.BeltSize{model.BeltSmall, model.BeltMedium, model.BeltLarge} {
			s := model.DefaultSettings(seed)
			s.BeltSize = size
			s.Rivals = 1 + int(seed%3)
			w, err := NewGame(s)
			if err != nil {
				t.Fatalf("seed %d size %v: %v", seed, size, err)
			}
			if len(w.Rocks) != size.RockCount() || len(w.Factions) != 1+s.Rivals {
				t.Fatalf("seed %d: %d rocks, %d factions", seed, len(w.Rocks), len(w.Factions))
			}
			frost, glint := 0, 0
			for _, r := range w.Rocks {
				fill := float64(r.BlobSize()) / float64(model.HexCount(r.Radius))
				if fill < 0.5 || fill > 0.85 {
					t.Errorf("seed %d rock %s: fill %.2f", seed, r.Name, fill)
				}
				switch r.Class {
				case model.ClassFrost:
					frost++
				case model.ClassGlint:
					glint++
				}
				if !regolithConnected(r) {
					t.Errorf("seed %d rock %s: open ground is split", seed, r.Name)
				}
			}
			if frost < 2 || glint < model.GlintRocksMin {
				t.Errorf("seed %d: %d frost, %d glint rocks", seed, frost, glint)
			}
			for _, f := range w.Factions {
				r := w.Rock(f.HomeRock)
				if r == nil || r.Owner != f.ID || r.Colony.Pop != model.StartPop {
					t.Fatalf("seed %d: faction %d has no proper start colony", seed, f.ID)
				}
				ks := r.Keystone().Hex
				near := map[model.Good]bool{}
				for _, h := range ks.Spiral(model.StartIceRange) {
					tl := r.TileAt(h)
					if tl == nil || !tl.HasDeposit() {
						continue
					}
					if tl.Deposit == model.Ice || h.Dist(ks) <= model.StartVeinRange {
						near[tl.Deposit] = true
					}
				}
				if !near[model.Ore] || !near[model.Silica] || !near[model.Ice] {
					t.Errorf("seed %d faction %d: start deposits missing: %v", seed, f.ID, near)
				}
			}
		}
	}
}

func worldJSON(t *testing.T, w *model.World) string {
	t.Helper()
	b, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func TestNewGameDeterministic(t *testing.T) {
	a, _ := NewGame(model.DefaultSettings(77))
	b, _ := NewGame(model.DefaultSettings(77))
	c, _ := NewGame(model.DefaultSettings(78))
	if worldJSON(t, a) != worldJSON(t, b) {
		t.Error("same seed gave different worlds")
	}
	if worldJSON(t, a) == worldJSON(t, c) {
		t.Error("different seeds gave the same world")
	}
}

// A colony left alone must tick over without collapsing in its first cycles.
func TestIdleColonyIsStable(t *testing.T) {
	s, w := newTestGame(t, 3)
	s.Run(5 * model.TicksPerCycle)
	c := w.Rock(w.Human().HomeRock).Colony
	if c.Pop < model.StartPop-2 || c.Morale < 40 {
		t.Errorf("idle colony decayed: pop %.1f morale %.1f starving %v suffocating %v", c.Pop, c.Morale, c.Starving, c.Suffocating)
	}
	if err := w.Validate(); err != nil {
		t.Error(err)
	}
}

func oreAt(r *model.Rock, g model.Good) func(model.Hex) bool {
	return func(h model.Hex) bool {
		for _, x := range reach(h) {
			if tl := r.TileAt(x); tl != nil && tl.HasDeposit() && tl.Deposit == g {
				return true
			}
		}
		return false
	}
}

// buildStarter lays down a small working colony for faction f and returns its
// home rock.
func buildStarter(t *testing.T, w *model.World, f model.FactionID) *model.Rock {
	t.Helper()
	r := w.Rock(w.Faction(f).HomeRock)
	r.RevealAll(f)
	r.Colony.Stock[model.Alloy] = 400 // tests are about rules, not the opening budget
	r.Colony.Stock[model.Silica] = 400
	for _, k := range []model.BuildingKind{model.Sunvane, model.Sunvane, model.Sunvane, model.Sunvane} {
		mustBuild(t, w, f, r.ID, k, nil)
	}
	// Conduits toward the ore, then the rig.
	for i := 0; i < 12; i++ {
		if h, ok := siteFor(w, f, r.ID, model.BoreRig, oreAt(r, model.Ore)); ok && oreAt(r, model.Ore)(h) {
			break
		}
		// Extend toward the nearest ore tile.
		var target model.Hex
		best := 99
		for _, h := range r.BlobHexes() {
			if tl := r.TileAt(h); tl.HasDeposit() && tl.Deposit == model.Ore {
				if d := h.Dist(r.Keystone().Hex); d < best {
					best, target = d, h
				}
			}
		}
		h, ok := siteFor(w, f, r.ID, model.Conduit, nil)
		if !ok {
			t.Fatal("cannot extend")
		}
		bestH, bestD := h, 99
		for _, c := range r.BlobHexes() {
			if CanPlaceIgnoringCost(w, f, r.ID, c, model.Conduit) == nil {
				if d := c.Dist(target); d < bestD {
					bestH, bestD = c, d
				}
			}
		}
		if _, err := Place(w, f, r.ID, bestH, model.Conduit); err != nil {
			t.Fatal(err)
		}
	}
	mustBuild(t, w, f, r.ID, model.BoreRig, oreAt(r, model.Ore))
	mustBuild(t, w, f, r.ID, model.Smelter, nil)
	mustBuild(t, w, f, r.ID, model.Greenhouse, nil)
	mustBuild(t, w, f, r.ID, model.Aerator, nil)
	mustBuild(t, w, f, r.ID, model.Habitat, nil)
	mustBuild(t, w, f, r.ID, model.Laboratory, nil)
	return r
}

func TestStarterColonyProducesAndResearches(t *testing.T) {
	s, w := newTestGame(t, 5)
	f := model.PlayerFaction
	r := buildStarter(t, w, f)
	if err := QueueResearch(w, f, model.OrbitalFlight); err != nil {
		t.Fatal(err)
	}
	s.Run(2 * model.TicksPerCycle)
	for _, b := range r.Buildings {
		if b.Constructing() {
			t.Errorf("%v still under construction after 2 cycles", b.Kind)
		}
	}
	oreBefore := w.Faction(f).Stats.Mined[model.Ore]
	s.Run(8 * model.TicksPerCycle)
	fac := w.Faction(f)
	if fac.Stats.Mined[model.Ore] <= oreBefore {
		t.Error("no Ore was mined")
	}
	if fac.Stats.Produced[model.Alloy] <= 0 || fac.Stats.Produced[model.Food] <= 0 {
		t.Errorf("production missing: %v", fac.Stats.Produced)
	}
	if fac.Research.Progress[model.OrbitalFlight] <= 0 {
		t.Error("no research progress")
	}
	c := r.Colony
	if c.Pop <= model.StartPop || c.Starving || c.Suffocating {
		t.Errorf("colony not thriving: pop %.1f starving %v suffocating %v morale %.0f", c.Pop, c.Starving, c.Suffocating, c.Morale)
	}
	if fac.Directives.Current < 4 {
		t.Errorf("directives not progressing: at %d", fac.Directives.Current)
	}
	if err := w.Validate(); err != nil {
		t.Error(err)
	}
	t.Logf("after 10 cycles: pop %.1f morale %.0f power %.0f/%.0f stock %v scrip %.0f research %.0f/60",
		c.Pop, c.Morale, c.PowerDemand, c.PowerSupply, c.Stock, fac.Scrip, fac.Research.Progress[model.OrbitalFlight])
}

func TestConstructionRules(t *testing.T) {
	_, w := newTestGame(t, 9)
	f := model.PlayerFaction
	r := w.Rock(w.Human().HomeRock)
	ks := r.Keystone()
	if err := CanPlace(w, f, r.ID, ks.Hex, model.Sunvane); err == nil {
		t.Error("placing on the Keystone tile must fail")
	}
	// A tile far from every structure is refused for adjacency.
	for _, h := range r.BlobHexes() {
		if h.Dist(ks.Hex) >= 4 && r.TileAt(h).Buildable() {
			if err := CanPlace(w, f, r.ID, h, model.Sunvane); err == nil {
				t.Errorf("placement at %v far from structures must fail", h)
			}
			break
		}
	}
	if err := CanPlace(w, f, r.ID, ks.Hex.Neighbor(0), model.LaunchCradle); err != model.ErrLocked {
		t.Errorf("locked building: %v", err)
	}
	if err := CanPlace(w, 2, r.ID, ks.Hex.Neighbor(0), model.Sunvane); err == nil {
		t.Error("a rival must not build on the player's rock")
	}
	before := r.Colony.Stock
	b := mustBuild(t, w, f, r.ID, model.Silo, nil)
	if got := before.Minus(r.Colony.Stock); got != b.Def().Cost {
		t.Errorf("cost deducted %v, want %v", got, b.Def().Cost)
	}
	// Cancelling refunds everything.
	if err := Demolish(w, f, r.ID, b.ID); err != nil || r.Colony.Stock != before {
		t.Errorf("cancel: %v, stock %v want %v", err, r.Colony.Stock, before)
	}
	// Expedite completes at once and costs Scrip.
	b = mustBuild(t, w, f, r.ID, model.Silo, nil)
	scrip := w.Human().Scrip
	if err := Expedite(w, f, r.ID, b.ID); err != nil || !b.Completed() || w.Human().Scrip >= scrip || b.HP != b.MaxHP {
		t.Errorf("expedite: %v completed=%v hp %.0f/%.0f", err, b.Completed(), b.HP, b.MaxHP)
	}
	// Demolishing a finished building refunds half.
	before = r.Colony.Stock
	if err := Demolish(w, f, r.ID, b.ID); err != nil {
		t.Fatal(err)
	}
	if got := r.Colony.Stock.Minus(before); got != b.Def().Cost.Refundable().Scaled(0.5) {
		t.Errorf("demolish refund %v", got)
	}
	if Demolish(w, f, r.ID, ks.ID) == nil || SetEnabled(w, f, r.ID, ks.ID, false) == nil {
		t.Error("a Keystone must not be demolished or switched off")
	}
}

// giveFleet puts a ready-made fleet in orbit of a rock.
func giveFleet(w *model.World, f model.FactionID, at model.RockID, kinds ...model.ShipKind) *model.Fleet {
	fl := model.NewFleet(f, at)
	for _, k := range kinds {
		fl.Ships = append(fl.Ships, model.NewShip(w.NewShipID(), k, 1))
	}
	fl.Pos = w.RockPos(at)
	return w.AddFleet(fl)
}

func unownedRock(w *model.World) *model.Rock {
	for _, r := range w.Rocks {
		if r.Owner == model.NoFaction {
			return r
		}
	}
	return nil
}

func TestShipyardSurveyAndColonize(t *testing.T) {
	s, w := newTestGame(t, 11)
	f := model.PlayerFaction
	fac := w.Human()
	home := buildStarter(t, w, f)
	s.Run(model.TicksPerCycle)
	for _, tech := range []model.TechID{model.OrbitalFlight, model.Fabrication, model.ColonialCharter} {
		grantTech(w, fac, tech)
	}
	home.Colony.StorageCap = 2000
	home.Colony.Stock = model.Stock{model.Alloy: 900, model.Silica: 400, model.Parts: 300, model.Fuel: 300, model.Food: 300, model.Ice: 300}
	home.Colony.Pop, home.Colony.Housing = 40, 60
	for i := 0; i < 3; i++ {
		mustBuild(t, w, f, home.ID, model.Silo, nil)
		mustBuild(t, w, f, home.ID, model.Habitat, nil)
	}
	for i := 0; i < 9; i++ {
		mustBuild(t, w, f, home.ID, model.Sunvane, nil)
	}
	cradle := mustBuild(t, w, f, home.ID, model.LaunchCradle, nil)
	for _, b := range home.Buildings {
		if b.Constructing() {
			advanceConstruction(b, 1)
			finishBuilding(w, home, b)
		}
	}
	if err := QueueShip(w, f, home.ID, cradle.ID, model.Prospector); err != nil {
		t.Fatal(err)
	}
	if err := QueueShip(w, f, home.ID, cradle.ID, model.Seedship); err != nil {
		t.Fatal(err)
	}
	if err := QueueShip(w, f, home.ID, cradle.ID, model.Cutter); err != model.ErrLocked {
		t.Errorf("locked ship: %v", err)
	}
	s.Run(3 * model.TicksPerCycle)
	g := w.GarrisonOf(home.ID, f)
	if g == nil || g.Count(model.Prospector) != 1 || g.Count(model.Seedship) != 1 {
		t.Fatalf("garrison after building: %+v (queue %v, eff %.2f idle %v)", g, cradle.Queue, cradle.Eff, cradle.Idle)
	}
	target := unownedRock(w)
	pid := []model.ShipID{g.Ships[0].ID}
	if g.Ships[0].Kind != model.Prospector {
		pid[0] = g.Ships[1].ID
	}
	fuel := home.Colony.Stock[model.Fuel]
	if err := FleetOrder(w, model.FleetOrderCmd(f, g.ID, pid, model.OrderSurvey, target.ID)); err != nil {
		t.Fatal(err)
	}
	if home.Colony.Stock[model.Fuel] >= fuel {
		t.Error("launch from an own rock must cost Fuel")
	}
	if len(w.FleetsOf(f)) != 2 {
		t.Fatalf("ordering a subset must split the fleet: %d fleets", len(w.FleetsOf(f)))
	}
	for i := 0; i < 40*model.TicksPerCycle && !target.Surveyed(f); i++ {
		s.Step()
	}
	if !target.Surveyed(f) || fac.Stats.RocksSurveyed != 1 {
		t.Fatal("survey did not complete")
	}
	for i := 0; i < 40*model.TicksPerCycle && len(w.FleetsOf(f)) > 1; i++ {
		s.Step()
	}
	g = w.GarrisonOf(home.ID, f)
	if len(w.FleetsOf(f)) != 1 || g == nil || g.Count(model.Prospector) != 1 {
		t.Fatalf("the Prospector must return and rejoin the garrison: %d fleets", len(w.FleetsOf(f)))
	}
	// Colonise.
	var sid model.ShipID
	for _, sh := range g.Ships {
		if sh.Kind == model.Seedship {
			sid = sh.ID
		}
	}
	home.Colony.Stock[model.Fuel] = 300
	if err := FleetOrder(w, model.ColonizeCmd(f, g.ID, []model.ShipID{sid}, target.ID, nil)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60*model.TicksPerCycle && target.Owner != f; i++ {
		s.Step()
	}
	if target.Owner != f || target.Colony == nil || target.Colony.Pop != model.SeedshipColonists || fac.Stats.ColoniesFounded != 1 {
		t.Fatalf("colony not founded: owner %d", target.Owner)
	}
	if w.BerthCap(f) != 8 {
		t.Errorf("berth cap %d after second colony", w.BerthCap(f))
	}
	// Freight route between the two colonies.
	hf := giveFleet(w, f, home.ID, model.Hauler)
	rt := model.Route{A: home.ID, B: target.ID, Reserve: 50}
	rt.Out[model.Alloy] = true
	if err := Exec(w, model.SetRouteCmd(f, hf.ID, nil, rt)); err != nil {
		t.Fatal(err)
	}
	alloy := target.Colony.Stock[model.Alloy]
	for i := 0; i < 80*model.TicksPerCycle && target.Colony.Stock[model.Alloy] <= alloy+50; i++ {
		s.Step()
	}
	if target.Colony.Stock[model.Alloy] <= alloy+50 {
		t.Errorf("freight route delivered nothing: %.0f -> %.0f", alloy, target.Colony.Stock[model.Alloy])
	}
	if err := w.Validate(); err != nil {
		t.Error(err)
	}
}

func TestAttackDestroysColonyAndEndsGame(t *testing.T) {
	st := model.DefaultSettings(21)
	st.Rivals, st.Events = 1, false
	w, err := NewGame(st)
	if err != nil {
		t.Fatal(err)
	}
	s := New(w)
	f, e := model.PlayerFaction, model.FactionID(2)
	enemyHome := w.Rock(w.Faction(e).HomeRock)
	home := w.Human().HomeRock
	// A defender and a turret make it a fight.
	giveFleet(w, e, enemyHome.ID, model.Dart, model.Dart).Garrison = true
	fl := giveFleet(w, f, home, model.Cutter, model.Cutter, model.Cutter, model.Hammer, model.Hammer, model.Hammer, model.Hammer)
	w.Colony(home).Stock[model.Fuel] = 500
	if err := Exec(w, model.AttackCmd(f, fl.ID, nil, enemyHome.ID, model.StanceRaze, model.RetreatNever)); err != nil {
		t.Fatal(err)
	}
	if eta, ok := ETA(w, fl); !ok || eta <= 0 {
		t.Errorf("ETA = %v, %v", eta, ok)
	}
	for i := 0; i < 120*model.TicksPerCycle && !w.GameOver; i++ {
		s.Step()
	}
	if !w.GameOver || w.Winner != f {
		t.Fatalf("game over %v winner %d (enemy colonies %d)", w.GameOver, w.Winner, w.ColonyCount(e))
	}
	if w.Faction(e).Alive || len(w.FleetsOf(e)) != 0 {
		t.Error("the loser must be eliminated and its fleets removed")
	}
	hf := w.Human()
	if hf.Stats.KeystonesDestroyed != 1 || hf.Stats.ShipsDestroyed != 2 || hf.Scrip < model.StartScrip+model.KeystoneBounty {
		t.Errorf("stats %+v scrip %.0f", hf.Stats, hf.Scrip)
	}
	if enemyHome.Owner != model.NoFaction {
		t.Error("the fallen rock must be unowned")
	}
	for _, b := range enemyHome.Buildings {
		if !b.Derelict() {
			t.Errorf("%v survived as a non-derelict", b.Kind)
		}
	}
	tick := w.Tick
	s.Step()
	if w.Tick != tick {
		t.Error("a finished game must not advance")
	}
	if err := w.Validate(); err != nil {
		t.Error(err)
	}
}

func TestTurretsAndShieldDefend(t *testing.T) {
	st := model.DefaultSettings(31)
	st.Rivals, st.Events = 1, false
	w, _ := NewGame(st)
	s := New(w)
	f, e := model.PlayerFaction, model.FactionID(2)
	home := w.Rock(w.Human().HomeRock)
	fac := w.Human()
	for _, tech := range []model.TechID{model.PointDefence, model.DeepScan, model.LumenOptics, model.AegisFields} {
		grantTech(w, fac, tech)
	}
	home.Colony.StorageCap = 3000
	home.Colony.Stock = model.Stock{model.Alloy: 2000, model.Silica: 800, model.Parts: 800, model.Lumen: 200, model.Food: 500, model.Ice: 500}
	home.Colony.Pop, home.Colony.Housing = 60, 80
	for i := 0; i < 10; i++ {
		mustBuild(t, w, f, home.ID, model.Sunvane, nil)
	}
	for i := 0; i < 4; i++ {
		mustBuild(t, w, f, home.ID, model.FlakBattery, nil)
		mustBuild(t, w, f, home.ID, model.Habitat, nil)
		mustBuild(t, w, f, home.ID, model.Aerator, nil)
	}
	aegis := mustBuild(t, w, f, home.ID, model.AegisProjector, nil)
	for _, b := range home.Buildings {
		if b.Constructing() {
			advanceConstruction(b, 1)
			finishBuilding(w, home, b)
		}
	}
	s.Run(60 * model.TicksPerSecond) // charge the shield
	if aegis.Shield < 500 {
		t.Fatalf("shield did not charge: %.0f (eff %.2f, idle %v)", aegis.Shield, aegis.Eff, aegis.Idle)
	}
	raid := giveFleet(w, e, home.ID, model.Dart, model.Dart, model.Hammer)
	raid.Order, raid.Dest, raid.Stance, raid.Home = model.OrderAttack, home.ID, model.StanceSuppress, w.Faction(e).HomeRock
	for i := 0; i < 5*model.TicksPerCycle && w.Fleet(raid.ID) != nil; i++ {
		s.Step()
	}
	if w.Fleet(raid.ID) != nil {
		t.Errorf("four Flak Batteries failed to stop a small raid: %+v", raid.Ships)
	}
	if aegis.Shield >= aegis.Def().ShieldPool && fac.Stats.DamageTaken == 0 {
		t.Error("the raid did no damage at all, not even to the shield")
	}
	if fac.Stats.ShipsDestroyed != 3 {
		t.Errorf("ships destroyed %d", fac.Stats.ShipsDestroyed)
	}
}

func TestMarketAndResearchCommands(t *testing.T) {
	s, w := newTestGame(t, 41)
	f := model.PlayerFaction
	fac := w.Human()
	home := w.Rock(fac.HomeRock)
	if Trade(w, f, home.ID, model.Alloy, 10) == nil {
		t.Error("trading without an Exchange Dock must fail")
	}
	grantTech(w, fac, model.CharterTrade)
	home.Colony.Stock[model.Alloy], home.Colony.Stock[model.Parts] = 300, 100
	dock := mustBuild(t, w, f, home.ID, model.ExchangeDock, nil)
	for i := 0; i < 3; i++ {
		mustBuild(t, w, f, home.ID, model.Sunvane, nil)
	}
	s.Run(2 * model.TicksPerCycle)
	if !dock.Working() {
		t.Fatalf("exchange dock idle: %v eff %.2f", dock.Idle, dock.Eff)
	}
	scrip, alloy := fac.Scrip, home.Colony.Stock[model.Alloy]
	if err := Trade(w, f, home.ID, model.Alloy, -50); err != nil {
		t.Fatal(err)
	}
	if fac.Scrip <= scrip || home.Colony.Stock[model.Alloy] != alloy-50 || w.Market.Pressure[model.Alloy] >= 0 {
		t.Error("selling must pay Scrip, remove stock and lower the price")
	}
	if err := Trade(w, f, home.ID, model.Food, 20); err != nil {
		t.Fatal(err)
	}
	if Trade(w, f, home.ID, model.Lumen, 200) == nil {
		t.Error("an unaffordable purchase must fail")
	}
	// Research queue.
	if err := QueueResearch(w, f, model.Gunnery); err == nil {
		t.Error("queueing a tech without its prerequisite must fail")
	}
	if err := QueueResearch(w, f, model.PointDefence); err != nil {
		t.Fatal(err)
	}
	if err := QueueResearch(w, f, model.Gunnery); err != nil {
		t.Fatalf("queueing behind a queued prerequisite: %v", err)
	}
	if fac.Research.Current != model.PointDefence || len(fac.Research.Queue) != 1 {
		t.Errorf("research state %+v", fac.Research)
	}
	if err := CancelResearch(w, f, -1); err != nil || fac.Research.Current != model.TechNone {
		t.Errorf("cancel current: %v, current %v (Gunnery must wait for its prerequisite)", err, fac.Research.Current)
	}
}

// fuzzCommand issues a random, frequently invalid command for faction f.
func fuzzCommand(w *model.World, rng *model.RNG, f model.FactionID) model.Command {
	rocks := w.RocksOf(f)
	var rock *model.Rock
	if len(rocks) > 0 {
		rock = rocks[rng.Intn(len(rocks))]
	} else {
		rock = w.Rocks[rng.Intn(len(w.Rocks))]
	}
	anyRock := w.Rocks[rng.Intn(len(w.Rocks))]
	var bid model.BuildingID
	if len(rock.Buildings) > 0 {
		bid = rock.Buildings[rng.Intn(len(rock.Buildings))].ID
	}
	var fid model.FleetID
	var ships []model.ShipID
	if fl := w.FleetsOf(f); len(fl) > 0 {
		x := fl[rng.Intn(len(fl))]
		fid = x.ID
		if rng.Chance(0.5) && len(x.Ships) > 0 {
			ships = []model.ShipID{x.Ships[rng.Intn(len(x.Ships))].ID}
		}
	}
	blob := rock.BlobHexes()
	hex := blob[rng.Intn(len(blob))]
	if ks := rock.Keystone(); ks != nil && rng.Chance(0.8) {
		hex = ks.Hex.Add(model.Hex{Q: rng.IntRange(-4, 4), R: rng.IntRange(-4, 4)})
	}
	switch rng.Intn(17) {
	case 0, 1, 2, 3, 4:
		return model.BuildCmd(f, rock.ID, hex, model.BuildingKind(rng.Intn(model.NumBuildingKinds)))
	case 5:
		return model.DemolishCmd(f, rock.ID, bid)
	case 6:
		return model.SetEnabledCmd(f, rock.ID, bid, rng.Chance(0.7))
	case 7:
		return model.SetPriorityCmd(f, rock.ID, bid, rng.IntRange(-1, 3))
	case 8:
		return model.ExpediteCmd(f, rock.ID, bid)
	case 9:
		return model.QueueShipCmd(f, rock.ID, bid, model.ShipKind(rng.Intn(model.NumShipKinds)))
	case 10:
		return model.CancelShipCmd(f, rock.ID, bid, rng.Intn(3))
	case 11:
		return model.FleetOrderCmd(f, fid, ships, model.OrderKind(rng.Intn(model.NumOrderKinds)), anyRock.ID)
	case 12:
		return model.AttackCmd(f, fid, ships, anyRock.ID, model.Stance(rng.Intn(model.NumStances)), model.RetreatLevels()[rng.Intn(3)])
	case 13:
		rt := model.Route{A: rock.ID, B: anyRock.ID, Reserve: 20}
		rt.Out[rng.Intn(model.NumGoods)] = true
		rt.Back[rng.Intn(model.NumGoods)] = true
		return model.SetRouteCmd(f, fid, ships, rt)
	case 14:
		return model.QueueResearchCmd(f, model.TechID(rng.Intn(model.NumTechs)))
	case 15:
		return model.TradeCmd(f, rock.ID, model.Good(rng.Intn(model.NumGoods)), rng.Range(-60, 60))
	default:
		return model.ColonizeCmd(f, fid, ships, anyRock.ID, nil)
	}
}

// runFuzz plays a whole world with random commands from every faction, with
// generous resources so that every system gets exercised.
func runFuzz(t *testing.T, seed uint64, ticks int) *model.World {
	t.Helper()
	st := model.DefaultSettings(seed)
	st.Rivals = 3
	w, err := NewGame(st)
	if err != nil {
		t.Fatal(err)
	}
	s := New(w)
	rng := model.NewRNG(seed * 7919)
	for _, f := range w.Factions {
		for _, tech := range model.AllTechs() {
			if rng.Chance(0.7) {
				grantTech(w, f, tech)
			}
		}
		f.Scrip = 20000
	}
	for i := 0; i < ticks && !w.GameOver; i++ {
		if i%600 == 0 {
			for _, r := range w.Rocks {
				if r.Colony != nil {
					for g := range r.Colony.Stock {
						r.Colony.Stock[g] = max(r.Colony.Stock[g], r.Colony.StorageCap*0.8)
					}
					r.Colony.Pop = max(r.Colony.Pop, 30)
				}
			}
		}
		if i%1500 == 0 {
			// Hand out ships so that orders and combat are exercised too.
			for _, f := range w.Factions {
				if rocks := w.RocksOf(f.ID); f.Alive && len(rocks) > 0 {
					var kinds []model.ShipKind
					for n := rng.IntRange(2, 5); n > 0; n-- {
						kinds = append(kinds, model.ShipKind(rng.Intn(model.NumShipKinds)))
					}
					fl := giveFleet(w, f.ID, rocks[0].ID, kinds...)
					fl.Home = rocks[0].ID
					settle(w, fl)
				}
			}
		}
		if i%3 == 0 {
			for _, f := range w.Factions {
				if f.Alive {
					w.Enqueue(fuzzCommand(w, &rng, f.ID))
				}
			}
		}
		s.Step()
		if i%997 == 0 {
			if err := w.Validate(); err != nil {
				t.Fatalf("seed %d tick %d: %v", seed, i, err)
			}
			if _, err := json.Marshal(w); err != nil {
				t.Fatalf("seed %d tick %d: %v", seed, i, err)
			}
		}
	}
	if err := w.Validate(); err != nil {
		t.Fatalf("seed %d at end: %v", seed, err)
	}
	return w
}

func TestFuzzSoak(t *testing.T) {
	ticks := 40 * model.TicksPerCycle
	if testing.Short() {
		ticks = 8 * model.TicksPerCycle
	}
	for seed := uint64(1); seed <= 6; seed++ {
		w := runFuzz(t, seed, ticks)
		ships, bld := 0, 0
		for _, f := range w.Factions {
			ships += f.Stats.ShipsBuilt
			bld += f.Stats.BuildingsBuilt
		}
		t.Logf("seed %d: tick %d over=%v fleets %d ships built %d buildings built %d events %d",
			seed, w.Tick, w.GameOver, len(w.Fleets), ships, bld, w.Events.Count)
		lost := 0
		for _, f := range w.Factions {
			lost += f.Stats.ShipsLost
		}
		if bld == 0 || lost == 0 {
			t.Errorf("seed %d: the fuzz run exercised too little (ships lost %d)", seed, lost)
		}
	}
}

func TestSimulationDeterministic(t *testing.T) {
	a := worldJSON(t, runFuzz(t, 99, 6*model.TicksPerCycle))
	b := worldJSON(t, runFuzz(t, 99, 6*model.TicksPerCycle))
	if a != b {
		t.Error("two identical runs diverged")
	}
	// A save/load in the middle must not change the outcome either.
	w := runFuzz(t, 99, 3*model.TicksPerCycle)
	c, err := w.Clone()
	if err != nil {
		t.Fatal(err)
	}
	New(w).Run(300)
	New(c).Run(300)
	w.FX, c.FX = nil, nil
	if worldJSON(t, w) != worldJSON(t, c) {
		t.Error("a cloned world diverged from the original")
	}
}
