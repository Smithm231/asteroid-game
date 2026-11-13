package model

import (
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"testing"
)

func TestNewShipAndFleet(t *testing.T) {
	s := NewShip(9, Bulwark, 1.2)
	if s.ID != 9 || s.Kind != Bulwark || math.Abs(s.MaxHP-720) > 1e-9 || s.HP != s.MaxHP || !s.Cargo.IsZero() {
		t.Errorf("NewShip = %+v", s)
	}
	if got := NewShip(1, Dart, 0); got.MaxHP != 50 {
		t.Errorf("hpMul 0 must count as 1: %+v", got)
	}
	if s.Def().Name != "Bulwark" || (*Ship)(nil).Def().Kind != ShipNone || (*Ship)(nil).CargoFree() != 0 {
		t.Error("Ship.Def is wrong")
	}

	f := NewFleet(2, 5)
	if f.ID != 0 || f.Owner != 2 || f.At != 5 || f.Home != 5 || f.Dest != NoRock || f.Origin != NoRock ||
		f.Order != OrderHold || f.Route != nil || f.Garrison || f.InTransit() || !f.Empty() {
		t.Errorf("NewFleet = %+v", f)
	}
	if tr := NewFleet(1, NoRock); !tr.InTransit() || tr.Home != NoRock {
		t.Errorf("fleet created in transit = %+v", tr)
	}
}

func TestFleetAggregates(t *testing.T) {
	f := NewFleet(1, 0)
	f.Ships = []Ship{
		NewShip(1, Hauler, 1),     // speed 5, 120 HP, cargo 150, 1 berth, fuel 0.020
		NewShip(2, Dart, 1),       // speed 10, 50 HP, 1 berth, fuel 0.008
		NewShip(3, Bulwark, 1),    // speed 4, 600 HP, 4 berths, fuel 0.050
		NewShip(4, Hauler, 1),     // second hauler
		NewShip(5, Prospector, 1), // speed 9, 60 HP, 1 berth, fuel 0.010
	}
	f.Ships[1].HP = 20
	f.Ships[2].HP = 450.5

	if got := f.TotalMaxHP(); got != 120+50+600+120+60 {
		t.Errorf("TotalMaxHP = %v", got)
	}
	if got := f.TotalHP(); got != 120+20+450.5+120+60 {
		t.Errorf("TotalHP = %v", got)
	}
	if got := f.Speed(DefaultMods()); got != 4 {
		t.Errorf("Speed = %v, want 4 (the Bulwark)", got)
	}
	fast := DefaultMods()
	fast.ShipSpeedMul = 1.25
	if got := f.Speed(fast); got != 5 {
		t.Errorf("Speed with Overdrive Thrusters = %v, want 5", got)
	}
	if got := f.Speed(Mods{}); got != 4 {
		t.Errorf("Speed with zero Mods = %v, want 4", got)
	}
	if got := f.Berths(); got != 1+1+4+1+1 {
		t.Errorf("Berths = %d", got)
	}
	if got, want := f.FuelPerU(), 0.020+0.008+0.050+0.020+0.010; math.Abs(got-want) > 1e-12 {
		t.Errorf("FuelPerU = %v, want %v", got, want)
	}
	if !f.Armed() || !f.HasKind(Dart) || f.HasKind(Seedship) || f.Count(Hauler) != 2 || f.Count(Cutter) != 0 {
		t.Error("composition helpers are wrong")
	}
	if got := f.ShipIDs(); !slices.Equal(got, []ShipID{1, 2, 3, 4, 5}) {
		t.Errorf("ShipIDs = %v", got)
	}

	// A fleet of unarmed ships is not armed.
	civil := NewFleet(1, 0)
	civil.Ships = []Ship{NewShip(6, Hauler, 1), NewShip(7, Prospector, 1), NewShip(8, Seedship, 1)}
	if civil.Armed() {
		t.Error("Haulers, Prospectors and Seedships are unarmed")
	}
	if got := civil.Speed(DefaultMods()); got != 4 {
		t.Errorf("Speed = %v, want 4 (the Seedship)", got)
	}

	// Empty and nil fleets.
	var none *Fleet
	for name, e := range map[string]*Fleet{"empty": NewFleet(1, 0), "nil": none} {
		if e.Speed(DefaultMods()) != 0 || e.TotalHP() != 0 || e.TotalMaxHP() != 0 || e.CargoCap() != 0 ||
			e.CargoUsed() != 0 || e.CargoFree() != 0 || e.Armed() || e.HasKind(Dart) || e.Berths() != 0 ||
			e.FuelPerU() != 0 || !e.Empty() || e.ShipIndex(1) != -1 || e.Ship(1) != nil ||
			!e.CargoTotal().IsZero() || e.LoadCargo(Alloy, 5) != 0 || e.UnloadCargo(Alloy, 5) != 0 {
			t.Errorf("%s fleet helpers must be inert", name)
		}
		if _, ok := e.RemoveShip(1); ok {
			t.Errorf("%s fleet: RemoveShip succeeded", name)
		}
	}
	if none.InTransit() || none.ShipIDs() != nil {
		t.Error("nil fleet helpers must be inert")
	}
}

func TestFleetCargo(t *testing.T) {
	f := NewFleet(1, 0)
	f.Ships = []Ship{NewShip(1, Hauler, 1), NewShip(2, Dart, 1), NewShip(3, Hauler, 1)}
	if f.CargoCap() != 300 || f.CargoUsed() != 0 || f.CargoFree() != 300 {
		t.Fatalf("cargo = %v / %v", f.CargoUsed(), f.CargoCap())
	}
	// Loading fills ships in order and skips ships without holds.
	if got := f.LoadCargo(Alloy, 100); got != 100 {
		t.Errorf("LoadCargo = %v, want 100", got)
	}
	if got := f.LoadCargo(Food, 120); got != 120 {
		t.Errorf("LoadCargo = %v, want 120", got)
	}
	if f.Ships[0].Cargo != (Stock{Alloy: 100, Food: 50}) || !f.Ships[1].Cargo.IsZero() || f.Ships[2].Cargo != (Stock{Food: 70}) {
		t.Errorf("cargo distribution: %v | %v | %v", f.Ships[0].Cargo, f.Ships[1].Cargo, f.Ships[2].Cargo)
	}
	// Loading is limited by the free capacity.
	if got := f.LoadCargo(Ice, 500); got != 80 {
		t.Errorf("LoadCargo over capacity = %v, want 80", got)
	}
	if f.CargoUsed() != 300 || f.CargoFree() != 0 {
		t.Errorf("cargo = %v used, %v free", f.CargoUsed(), f.CargoFree())
	}
	if got := f.LoadCargo(Ice, 1); got != 0 {
		t.Errorf("LoadCargo into a full fleet = %v", got)
	}
	if got, want := f.CargoTotal(), (Stock{Alloy: 100, Food: 120, Ice: 80}); got != want {
		t.Errorf("CargoTotal = %v, want %v", got, want)
	}
	if f.Ships[0].CargoFree() != 0 || f.Ships[2].CargoFree() != 0 || f.Ships[1].CargoFree() != 0 {
		t.Error("per-ship CargoFree is wrong")
	}
	// Invalid requests.
	if f.LoadCargo(GoodNone, 5) != 0 || f.LoadCargo(Alloy, -5) != 0 || f.UnloadCargo(GoodNone, 5) != 0 || f.UnloadCargo(Alloy, 0) != 0 {
		t.Error("invalid cargo requests must do nothing")
	}

	// Unloading empties ships in order.
	if got := f.UnloadCargo(Food, 60); got != 60 {
		t.Errorf("UnloadCargo = %v, want 60", got)
	}
	if f.Ships[0].Cargo[Food] != 0 || f.Ships[2].Cargo[Food] != 60 {
		t.Errorf("after unloading 60 Food: %v | %v", f.Ships[0].Cargo, f.Ships[2].Cargo)
	}
	if got := f.UnloadCargo(Food, 1000); got != 60 {
		t.Errorf("UnloadCargo beyond the load = %v, want 60", got)
	}
	if got := f.UnloadCargo(Food, 1); got != 0 {
		t.Errorf("UnloadCargo of an absent good = %v", got)
	}
	if got := f.UnloadCargo(Alloy, 100); got != 100 {
		t.Errorf("UnloadCargo = %v, want 100", got)
	}
	if got := f.UnloadCargo(Ice, 80); got != 80 {
		t.Errorf("UnloadCargo = %v, want 80", got)
	}
	if f.CargoUsed() != 0 || !f.CargoTotal().IsZero() {
		t.Errorf("fleet should be empty: %v", f.CargoTotal())
	}
}

func TestFleetShipAccess(t *testing.T) {
	f := NewFleet(1, 0)
	for id := ShipID(10); id < 15; id++ {
		f.Ships = append(f.Ships, NewShip(id, Cutter, 1))
	}
	if f.ShipIndex(12) != 2 || f.ShipIndex(99) != -1 || f.ShipIndex(0) != -1 {
		t.Error("ShipIndex is wrong")
	}
	s := f.Ship(13)
	if s == nil || s.ID != 13 {
		t.Fatalf("Ship(13) = %+v", s)
	}
	s.HP = 1
	if f.Ships[3].HP != 1 {
		t.Error("Ship must return a pointer into the fleet")
	}
	removed, ok := f.RemoveShip(11)
	if !ok || removed.ID != 11 {
		t.Fatalf("RemoveShip(11) = %+v, %v", removed, ok)
	}
	if got := f.ShipIDs(); !slices.Equal(got, []ShipID{10, 12, 13, 14}) {
		t.Errorf("after removal: %v (order must be kept)", got)
	}
	if _, ok := f.RemoveShip(11); ok {
		t.Error("removing the same ship twice must fail")
	}
	for _, id := range []ShipID{14, 10, 13, 12} {
		if _, ok := f.RemoveShip(id); !ok {
			t.Errorf("RemoveShip(%d) failed", id)
		}
	}
	if !f.Empty() {
		t.Errorf("fleet should be empty: %v", f.ShipIDs())
	}
}

func TestRouteHelpers(t *testing.T) {
	r := &Route{A: 3, B: 7, Reserve: 20}
	r.Out[Alloy] = true
	r.Back[Lumen] = true
	if r.Other(3) != 7 || r.Other(7) != 3 || r.Other(5) != NoRock {
		t.Error("Route.Other is wrong")
	}
	if got := r.Loads(3); !got[Alloy] || got[Lumen] {
		t.Errorf("Loads(A) = %v", got)
	}
	if got := r.Loads(7); got[Alloy] || !got[Lumen] {
		t.Errorf("Loads(B) = %v", got)
	}
	if got := r.Loads(1); got != ([NumGoods]bool{}) {
		t.Errorf("Loads(elsewhere) = %v", got)
	}
	var none *Route
	if none.Other(3) != NoRock || none.Loads(3) != ([NumGoods]bool{}) {
		t.Error("nil route helpers must be inert")
	}
}

func TestMarketHelpers(t *testing.T) {
	var m Market
	for _, g := range AllGoods() {
		base := GoodInfoOf(g).BaseValue
		if got := m.Price(g); got != base {
			t.Errorf("Price(%s) at zero pressure = %v, want %v", g, got, base)
		}
		if got := m.BuyPrice(g); math.Abs(got-base*1.12) > 1e-12 {
			t.Errorf("BuyPrice(%s) = %v", g, got)
		}
		if got := m.SellPrice(g); math.Abs(got-base*0.88) > 1e-12 {
			t.Errorf("SellPrice(%s) = %v", g, got)
		}
	}
	m.Pressure[Lumen] = 0.25
	if got := m.Price(Lumen); got != 15 {
		t.Errorf("Price(Lumen) at +0.25 = %v, want 15", got)
	}
	m.Pressure[Ore] = -0.5
	if got := m.Price(Ore); got != 0.5 {
		t.Errorf("Price(Ore) at -0.5 = %v, want 0.5", got)
	}
	if m.Price(GoodNone) != 0 || (*Market)(nil).Price(Ore) != 0 || (*Market)(nil).BuyPrice(Ore) != 0 {
		t.Error("invalid price lookups must be 0")
	}
	// Buying always costs more than selling pays.
	for _, g := range AllGoods() {
		if !(m.BuyPrice(g) > m.SellPrice(g)) {
			t.Errorf("%s: buy %v <= sell %v", g, m.BuyPrice(g), m.SellPrice(g))
		}
	}
	// Pressure is clamped to +-0.6. 200 units move it by 0.1.
	m = Market{}
	m.AddPressure(Fuel, 200*MarketPressurePerUnit)
	if got := m.Pressure[Fuel]; math.Abs(got-0.1) > 1e-12 {
		t.Errorf("pressure after buying 200 = %v, want 0.1", got)
	}
	m.AddPressure(Fuel, 5)
	if m.Pressure[Fuel] != MarketPressureMax {
		t.Errorf("pressure = %v, want the +%v clamp", m.Pressure[Fuel], MarketPressureMax)
	}
	m.AddPressure(Fuel, -50)
	if m.Pressure[Fuel] != -MarketPressureMax {
		t.Errorf("pressure = %v, want the -%v clamp", m.Pressure[Fuel], MarketPressureMax)
	}
	m.AddPressure(GoodNone, 1)
	(*Market)(nil).AddPressure(Fuel, 1)
}

func TestFactionHelpers(t *testing.T) {
	f := NewFaction(1, "Free Charter of Ansel", PersonalityNone)
	if f.ID != 1 || f.IsAI || !f.Alive || f.Research.Current != TechNone || f.Mods != DefaultMods() ||
		f.HomeRock != NoRock || f.AI.TargetRock != NoRock || f.AI.StagingRock != NoRock || f.AI.ExpandRock != NoRock ||
		f.ColorIndex != ColorAmber || f.Scrip != 0 {
		t.Fatalf("NewFaction = %+v", f)
	}
	if ai := NewFaction(3, "x", PersonalityMeridian); !ai.IsAI || ai.Personality != PersonalityMeridian || ai.ColorIndex != ColorMagenta {
		t.Errorf("AI faction = %+v", ai)
	}

	// Nothing researched: exactly the ten starting buildings, no ships.
	wantStart := []BuildingKind{Conduit, Silo, SurveyMast, Sunvane, Habitat, Aerator, Greenhouse, BoreRig, Smelter, Laboratory}
	if got := f.UnlockedBuildings(); !slices.Equal(got, wantStart) {
		t.Errorf("UnlockedBuildings = %v, want %v", got, wantStart)
	}
	if got := f.UnlockedShips(); len(got) != 0 {
		t.Errorf("UnlockedShips = %v, want none", got)
	}
	if f.CanBuild(Keystone) {
		t.Error("the Keystone is never buildable by command")
	}
	if f.CanBuild(BuildingNone) || f.CanBuild(BuildingKind(60)) || f.CanBuildShip(ShipNone) || f.Has(TechNone) || f.Has(TechID(90)) {
		t.Error("invalid kinds must be locked")
	}
	if f.TechCount() != 0 {
		t.Errorf("TechCount = %d", f.TechCount())
	}
	// Available techs: everything without prerequisites.
	wantAvail := []TechID{OrbitalFlight, VolatileCracking, Fabrication, CivicPlanning, PointDefence, DeepBoring, CharterTrade, Photovoltaics}
	if got := f.AvailableTechs(); !slices.Equal(got, wantAvail) {
		t.Errorf("AvailableTechs = %v, want %v", got, wantAvail)
	}
	if f.CanResearch(ColonialCharter) || f.PrereqsMet(ColonialCharter) {
		t.Error("Colonial Charter needs Orbital Flight and Fabrication")
	}

	// Every tech unlocks exactly what its table entry lists.
	for _, tech := range Techs() {
		g := NewFaction(1, "t", PersonalityNone)
		g.Techs[tech.ID] = true
		for _, k := range AllBuildingKinds() {
			want := BuildingDefOf(k).StartUnlocked || slices.Contains(tech.UnlockBuildings, k)
			if got := g.CanBuild(k); got != want {
				t.Errorf("with %s: CanBuild(%s) = %v, want %v", tech.Name, k, got, want)
			}
		}
		for _, k := range AllShipKinds() {
			want := slices.Contains(tech.UnlockShips, k)
			if got := g.CanBuildShip(k); got != want {
				t.Errorf("with %s: CanBuildShip(%s) = %v, want %v", tech.Name, k, got, want)
			}
		}
	}

	f.Techs[OrbitalFlight] = true
	if !f.Has(OrbitalFlight) || !f.CanBuild(LaunchCradle) || !f.CanBuildShip(Prospector) || !f.CanBuildShip(Hauler) || f.CanBuildShip(Seedship) {
		t.Error("Orbital Flight unlocks are wrong")
	}
	if f.CanResearch(OrbitalFlight) {
		t.Error("a known tech cannot be researched again")
	}
	if f.CanResearch(ColonialCharter) {
		t.Error("Colonial Charter still needs Fabrication")
	}
	if !f.CanResearch(DeepScan) || !f.CanResearch(DockEngineering) {
		t.Error("Orbital Flight opens Deep Scan and Dock Engineering")
	}
	f.Techs[Fabrication] = true
	if !f.CanResearch(ColonialCharter) || !f.PrereqsMet(ColonialCharter) {
		t.Error("Colonial Charter should be available now")
	}
	if f.TechCount() != 2 {
		t.Errorf("TechCount = %d", f.TechCount())
	}

	// With every tech known, everything but the Keystone is buildable.
	for i := range f.Techs {
		f.Techs[i] = true
	}
	if got := len(f.UnlockedBuildings()); got != NumBuildingKinds-1 {
		t.Errorf("%d buildings unlocked, want %d", got, NumBuildingKinds-1)
	}
	if got := len(f.UnlockedShips()); got != NumShipKinds {
		t.Errorf("%d ships unlocked, want %d", got, NumShipKinds)
	}
	if got := f.AvailableTechs(); len(got) != 0 {
		t.Errorf("AvailableTechs with everything known = %v", got)
	}

	// Research bookkeeping.
	g := NewFaction(1, "r", PersonalityNone)
	g.Research.Current = Gunnery
	g.Research.Queue = []TechID{SiegeDoctrine}
	g.Research.Progress[Gunnery] = 40
	if !g.InQueue(Gunnery) || !g.InQueue(SiegeDoctrine) || g.InQueue(DeepScan) || g.InQueue(TechNone) {
		t.Error("InQueue is wrong")
	}
	if got := g.ResearchFrac(Gunnery); got != 0.25 {
		t.Errorf("ResearchFrac = %v, want 0.25", got)
	}
	g.Research.Progress[Gunnery] = 500
	if got := g.ResearchFrac(Gunnery); got != 1 {
		t.Errorf("ResearchFrac is not clamped: %v", got)
	}
	g.Techs[DeepScan] = true
	if g.ResearchFrac(DeepScan) != 1 || g.ResearchFrac(TechNone) != 0 {
		t.Error("ResearchFrac edge cases are wrong")
	}

	// Scrip.
	g.AddScrip(100)
	g.AddScrip(-50)
	g.AddScrip(0)
	if g.Scrip != 100 || g.Stats.ScripEarned != 100 {
		t.Errorf("Scrip = %v, earned %v", g.Scrip, g.Stats.ScripEarned)
	}
	if g.SpendScrip(120) {
		t.Error("spent more than available")
	}
	if !g.SpendScrip(0) || !g.SpendScrip(-4) || g.Scrip != 100 {
		t.Error("a non-positive price always succeeds and costs nothing")
	}
	if !g.SpendScrip(60) || g.Scrip != 40 || g.Stats.ScripSpent != 60 {
		t.Errorf("after spending 60: %v, spent %v", g.Scrip, g.Stats.ScripSpent)
	}
	if !g.SpendScrip(40) || g.Scrip != 0 {
		t.Errorf("spending everything: %v", g.Scrip)
	}

	// Directives.
	d := DirectiveState{Done: []bool{true, false}}
	if !d.IsDone(0) || d.IsDone(1) || d.IsDone(2) || d.IsDone(-1) || (*DirectiveState)(nil).IsDone(0) {
		t.Error("DirectiveState.IsDone is wrong")
	}

	// Nil factions are inert.
	var none *Faction
	if none.Has(OrbitalFlight) || none.CanBuild(Silo) || none.CanBuildShip(Dart) || none.CanResearch(Gunnery) ||
		none.PrereqsMet(Gunnery) || none.TechCount() != 0 || none.InQueue(Gunnery) || none.ResearchFrac(Gunnery) != 0 ||
		none.SpendScrip(5) || none.UnlockedBuildings() != nil || none.UnlockedShips() != nil || none.AvailableTechs() != nil {
		t.Error("nil faction helpers must be inert")
	}
	none.AddScrip(5)
	none.RecomputeMods(DiffHard)
}

func TestCommandConstructors(t *testing.T) {
	site := Hex{2, -1}
	route := Route{A: 1, B: 4, Reserve: 10}
	route.Out[Alloy] = true
	tests := []struct {
		name  string
		cmd   Command
		kind  CmdKind
		check func(c Command) bool
	}{
		{"build", BuildCmd(1, 3, Hex{1, 2}, Smelter), CmdBuild,
			func(c Command) bool { return c.Rock == 3 && c.Hex == (Hex{1, 2}) && c.BKind == Smelter }},
		{"demolish", DemolishCmd(1, 3, 8), CmdDemolish,
			func(c Command) bool { return c.Rock == 3 && c.Building == 8 }},
		{"recommission", RecommissionCmd(1, 3, 8), CmdRecommission,
			func(c Command) bool { return c.Rock == 3 && c.Building == 8 }},
		{"enable", SetEnabledCmd(1, 3, 8, true), CmdSetEnabled,
			func(c Command) bool { return c.Rock == 3 && c.Building == 8 && c.Flag }},
		{"disable", SetEnabledCmd(1, 3, 8, false), CmdSetEnabled,
			func(c Command) bool { return !c.Flag }},
		{"priority", SetPriorityCmd(1, 3, 8, PriorityHigh), CmdSetPriority,
			func(c Command) bool { return c.Rock == 3 && c.Building == 8 && c.Int == 2 }},
		{"expedite", ExpediteCmd(1, 3, 8), CmdExpedite,
			func(c Command) bool { return c.Rock == 3 && c.Building == 8 }},
		{"queue ship", QueueShipCmd(1, 3, 8, Cutter), CmdQueueShip,
			func(c Command) bool { return c.Rock == 3 && c.Building == 8 && c.SKind == Cutter }},
		{"cancel ship", CancelShipCmd(1, 3, 8, 2), CmdCancelShip,
			func(c Command) bool { return c.Rock == 3 && c.Building == 8 && c.Int == 2 }},
		{"move", FleetOrderCmd(1, 5, []ShipID{7, 9}, OrderMove, 2), CmdFleetOrder,
			func(c Command) bool {
				return c.Fleet == 5 && slices.Equal(c.Ships, []ShipID{7, 9}) && c.Order == OrderMove && c.Target == 2 &&
					c.Stance == StanceSuppress && c.Retreat == RetreatNever && !c.HasHex
			}},
		{"attack", AttackCmd(1, 5, nil, 2, StanceRaze, RetreatHalf), CmdFleetOrder,
			func(c Command) bool {
				return c.Fleet == 5 && c.Ships == nil && c.Order == OrderAttack && c.Target == 2 &&
					c.Stance == StanceRaze && c.Retreat == 0.5
			}},
		{"colonize at a site", ColonizeCmd(1, 5, nil, 2, &site), CmdFleetOrder,
			func(c Command) bool { return c.Order == OrderColonize && c.Target == 2 && c.HasHex && c.Hex == site }},
		{"colonize anywhere", ColonizeCmd(1, 5, nil, 2, nil), CmdFleetOrder,
			func(c Command) bool {
				return c.Order == OrderColonize && c.Target == 2 && !c.HasHex && c.Hex == (Hex{})
			}},
		{"route", SetRouteCmd(1, 5, []ShipID{7}, route), CmdSetRoute,
			func(c Command) bool { return c.Fleet == 5 && c.Route == route && slices.Equal(c.Ships, []ShipID{7}) }},
		{"scuttle", ScuttleCmd(1, 5, []ShipID{7}), CmdScuttle,
			func(c Command) bool { return c.Fleet == 5 && slices.Equal(c.Ships, []ShipID{7}) }},
		{"set research", SetResearchCmd(1, Gunnery), CmdSetResearch,
			func(c Command) bool { return c.Tech == Gunnery }},
		{"queue research", QueueResearchCmd(1, DeepScan), CmdQueueResearch,
			func(c Command) bool { return c.Tech == DeepScan }},
		{"cancel research", CancelResearchCmd(1, -1), CmdCancelResearch,
			func(c Command) bool { return c.Int == -1 }},
		{"buy", TradeCmd(1, 3, Food, 50), CmdTrade,
			func(c Command) bool { return c.Rock == 3 && c.Good == Food && c.Amount == 50 }},
		{"sell", TradeCmd(1, 3, Ore, -75), CmdTrade,
			func(c Command) bool { return c.Rock == 3 && c.Good == Ore && c.Amount == -75 }},
		{"sponsor", SponsorCmd(1, 3), CmdSponsor,
			func(c Command) bool { return c.Rock == 3 }},
		{"driver target", SetDriverTargetCmd(1, 3, 8, 6), CmdSetDriverTarget,
			func(c Command) bool { return c.Rock == 3 && c.Building == 8 && c.Target == 6 }},
		{"driver clear", SetDriverTargetCmd(1, 3, 8, NoRock), CmdSetDriverTarget,
			func(c Command) bool { return c.Target == NoRock }},
	}
	kinds := map[CmdKind]bool{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.cmd
			if c.Kind != tc.kind || !c.Kind.Valid() {
				t.Errorf("Kind = %s, want %s", c.Kind, tc.kind)
			}
			if c.Faction != 1 {
				t.Errorf("Faction = %d, want 1", c.Faction)
			}
			if !tc.check(c) {
				t.Errorf("fields are wrong: %+v", c)
			}
			// Fields a kind does not use hold their "none" value, so a
			// handler reading the wrong one fails loudly.
			blank := baseCmd(c.Kind, 1)
			usesRock := c.Rock != NoRock
			usesTarget := c.Kind == CmdFleetOrder || c.Kind == CmdSetDriverTarget
			if !usesRock && c.Rock != blank.Rock {
				t.Errorf("unused Rock = %d", c.Rock)
			}
			if !usesTarget && c.Target != NoRock {
				t.Errorf("unused Target = %d, want NoRock", c.Target)
			}
			if c.Kind != CmdBuild && c.BKind != BuildingNone {
				t.Errorf("unused BKind = %d", c.BKind)
			}
			if c.Kind != CmdQueueShip && c.SKind != ShipNone {
				t.Errorf("unused SKind = %d", c.SKind)
			}
			if c.Kind != CmdSetResearch && c.Kind != CmdQueueResearch && c.Tech != TechNone {
				t.Errorf("unused Tech = %d", c.Tech)
			}
			if c.Kind != CmdTrade && c.Good != GoodNone {
				t.Errorf("unused Good = %d", c.Good)
			}
			// Commands survive JSON unchanged.
			data, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			var back Command
			if err := json.Unmarshal(data, &back); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(normalizeCmd(c), normalizeCmd(back)) {
				t.Errorf("JSON round trip changed the command:\n  %+v\n  %+v", c, back)
			}
		})
		kinds[tc.kind] = true
	}
	// Every command kind has a constructor covered above.
	for k := CmdKind(1); k < NumCmdKinds; k++ {
		if !kinds[k] {
			t.Errorf("no constructor test for %s", k)
		}
	}
	if (Command{}).Kind != CmdNone || (Command{}).Kind.Valid() {
		t.Error("the zero Command must be invalid")
	}
}

// normalizeCmd maps an empty Ships slice to nil so that DeepEqual does not
// distinguish "no ships" spellings.
func normalizeCmd(c Command) Command {
	if len(c.Ships) == 0 {
		c.Ships = nil
	}
	return c
}
