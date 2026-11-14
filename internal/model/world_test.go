package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
)

// buildTestWorld assembles a small but fully populated world by hand: three
// rocks (two colonised, one bare with salvage and a derelict), two factions,
// buildings in every state, fleets in orbit and in transit with cargo and a
// route, slugs, market pressure, pending commands, a log and event state.
// Every field type of the model appears with a non-zero value somewhere, so
// that the JSON round trip is a meaningful test.
func buildTestWorld(t *testing.T) *World {
	t.Helper()
	settings := DefaultSettings(0xC0FFEE1234567890)
	settings.Rivals = 1
	settings.RivalKinds = []Personality{PersonalityVane}
	settings.Difficulty = DiffHard
	w := NewWorld(settings)
	w.Tick = 4321

	// Factions.
	player := NewFaction(1, "Free Charter of Ansel", PersonalityNone)
	player.Scrip = 412.75
	player.HomeRock = 0
	player.Techs[OrbitalFlight] = true
	player.Techs[DeepBoring] = true
	player.Research = Research{
		Current: Fabrication,
		Queue:   []TechID{PointDefence, Gunnery},
		Rate:    12,
	}
	player.Research.Progress[Fabrication] = 33.5
	player.Research.Progress[CivicPlanning] = 7.25
	player.Research.Started[Fabrication] = true
	player.RecomputeMods(settings.Difficulty)
	player.Directives = DirectiveState{Done: []bool{true, true, false}, Current: 2, LastDoneTick: 3000}
	player.Stats = Stats{
		ShipsBuilt: 3, BuildingsBuilt: 9, RocksSurveyed: 1, TechsResearched: 2,
		Mined: Stock{Ore: 812.5, Ice: 40}, Produced: Stock{Alloy: 301.25},
		Bought: Stock{Food: 20}, Sold: Stock{Ore: 55},
		ScripEarned: 96.4, ScripSpent: 83.65, DamageDealt: 12.5, DamageTaken: 3.75, PeakPop: 18.5,
	}

	rival := NewFaction(2, PersonalityVane.String(), PersonalityVane)
	rival.Scrip = 388
	rival.HomeRock = 1
	rival.Techs[PointDefence] = true
	rival.RecomputeMods(settings.Difficulty)
	rival.AI = AIState{
		Phase: 2, Step: 5, Wave: 1,
		Timers:      []int{4800, 0, 6000},
		TargetRock:  0,
		StagingRock: 1,
		ExpandRock:  2,
		StrikeFleet: 3,
		Rocks:       []RockID{2, 0},
		Fleets:      []FleetID{3},
		Floats:      []float64{0.25, -1.5, 1e-9},
		Ints:        []int32{7, -2, 0},
	}
	w.Factions = []*Faction{player, rival}

	// Rock 0: the player's home.
	r0 := testRock(0, 4)
	r0.Name = "Ansel's Rest"
	r0.Class = ClassStony
	r0.OrbitR = 520
	r0.Phase0 = 0.75
	r0.Solar = SolarFactor(r0.OrbitR)
	r0.ShapeSeed = 0xFEDCBA9876543210
	r0.TileAt(Hex{4, -4}).Terrain = TerrainVoid
	r0.TileAt(Hex{-2, 0}).Terrain = TerrainCrag
	r0.TileAt(Hex{-2, 1}).Terrain = TerrainFissure
	ore := r0.TileAt(Hex{1, 1})
	ore.Deposit, ore.Amount = Ore, 733.125
	ice := r0.TileAt(Hex{-1, 2})
	ice.Deposit, ice.Amount = Ice, 410
	r0.TileAt(Hex{2, 2}).Deposit = Silica // depleted vein
	r0.TileAt(Hex{3, -1}).Rubble = true
	r0.RevealWithin(Hex{}, 3, 1)
	r0.TileAt(Hex{1, 1}).Reveal(2)
	r0.Owner = 1
	r0.Colony = NewColony(0)
	c0 := r0.Colony
	c0.Stock = Stock{Ore: 12.5, Ice: 61.25, Silica: 99, Alloy: 143.875, Fuel: 30, Parts: 20, Food: 88.5}
	c0.Pop, c0.Morale, c0.MoraleTarget = 16.4, 63.5, 65
	c0.Housing, c0.LifeSupport, c0.StorageCap = 28, 36, 700
	c0.PowerSupply, c0.PowerDemand = 44.5, 39
	c0.Workers, c0.WorkersNeeded, c0.WorkersUsed = 16, 18, 16
	c0.CommonsCoverage, c0.PopRate = 0.4, 0.69
	c0.MarkAttacked(4000)
	c0.MarkSponsored(3600)
	c0.Produced = Stock{Ore: 36, Alloy: 20}
	c0.Consumed = Stock{Ore: 30, Food: 8.2, Ice: 3.28}
	c0.ProducedAcc = Stock{Ore: 0.6}
	c0.ConsumedAcc = Stock{Food: 0.137}

	add := func(r *Rock, b *Building) *Building {
		t.Helper()
		if err := r.AddBuilding(b); err != nil {
			t.Fatalf("AddBuilding(%d on rock %d): %v", b.ID, r.ID, err)
		}
		return b
	}
	hp := player.Mods.HPMul
	key0 := add(r0, NewActiveBuilding(w.NewBuildingID(), Keystone, 1, Hex{0, 0}, hp))
	key0.Powered, key0.Staffed, key0.Eff, key0.PowerOut = true, 1, 1, 20
	rig := add(r0, NewActiveBuilding(w.NewBuildingID(), BoreRig, 1, Hex{1, 0}, hp))
	rig.Powered, rig.Workers, rig.Staffed, rig.Eff, rig.PowerOut = true, 3, 1, 1.1, -8
	rig.HP = 180.5
	smelter := add(r0, NewActiveBuilding(w.NewBuildingID(), Smelter, 1, Hex{0, 1}, hp))
	smelter.Enabled, smelter.Idle, smelter.Priority = false, IdleDisabled, PriorityLow
	site := add(r0, NewBuilding(w.NewBuildingID(), Laboratory, 1, Hex{-1, 1}, hp))
	site.Progress = 0.4375
	site.HP = site.ConstructionHP()
	cradle := add(r0, NewActiveBuilding(w.NewBuildingID(), LaunchCradle, 1, Hex{-1, 0}, hp))
	cradle.Powered, cradle.Workers, cradle.Staffed, cradle.Eff = true, 3, 0.75, 0.75
	cradle.Queue = []ShipKind{Hauler, Prospector}
	cradle.QueueProgress = 0.3
	mast := add(r0, NewActiveBuilding(w.NewBuildingID(), SurveyMast, 1, Hex{0, -1}, hp))
	mast.SurveyAcc = 0.9
	mast.Idle = IdleNoPower

	// Rock 1: the rival's home, with defences.
	r1 := testRock(1, 3)
	r1.Name = "Vane Anchorage"
	r1.Class = ClassFerric
	r1.OrbitR = 610.5
	r1.Phase0 = 3.9
	r1.Solar = SolarFactor(r1.OrbitR)
	r1.ShapeSeed = 42
	r1.RevealAll(2)
	r1.Owner = 2
	r1.Colony = NewColony(0)
	r1.Colony.Stock = Stock{Alloy: 200, Lumen: 4.5, Food: 60, Ice: 35}
	r1.Colony.Pop, r1.Colony.Morale = 21, 48.25
	r1.Colony.Starving = true
	r1.Colony.Suffocating = true
	hp2 := rival.Mods.HPMul
	add(r1, NewActiveBuilding(w.NewBuildingID(), Keystone, 2, Hex{0, 0}, hp2))
	flak := add(r1, NewActiveBuilding(w.NewBuildingID(), FlakBattery, 2, Hex{1, 0}, hp2))
	flak.Target = 2
	aegis := add(r1, NewActiveBuilding(w.NewBuildingID(), AegisProjector, 2, Hex{0, 1}, hp2))
	aegis.Shield = 412.5
	driver := add(r1, NewActiveBuilding(w.NewBuildingID(), MassDriver, 2, Hex{-1, 0}, hp2))
	driver.DriverTarget = 0
	driver.Cooldown = 7.5

	// Rock 2: unowned, with a derelict and salvage in orbit.
	r2 := testRock(2, 2)
	r2.Name = "Glint Seven"
	r2.Class = ClassGlint
	r2.OrbitR = 880
	r2.Phase0 = 5.5
	r2.Solar = SolarFactor(r2.OrbitR)
	lum := r2.TileAt(Hex{1, -1})
	lum.Deposit, lum.Amount = Lumen, 150
	r2.SurveyedBy.Set(1)
	derelict := add(r2, NewActiveBuilding(w.NewBuildingID(), Silo, 2, Hex{0, 0}, 1))
	derelict.MakeDerelict()
	derelict.HP = 120
	r2.Salvage = &Salvage{
		Goods: Stock{Alloy: 45, Parts: 12}, Scrip: 80,
		SpawnTick: 3900, ExpireTick: 3900 + SalvageLifetimeTicks,
		Claimant: 1, ClaimSeconds: 4.5,
	}
	w.Rocks = []*Rock{r0, r1, r2}

	// Fleets.
	garrison := NewFleet(1, 0)
	garrison.Garrison = true
	garrison.Ships = []Ship{
		NewShip(w.NewShipID(), Prospector, hp),
		NewShip(w.NewShipID(), Dart, hp),
	}
	garrison.Ships[1].HP = 31.5
	w.AddFleet(garrison)

	hauler := NewFleet(1, NoRock)
	hauler.Home = 0
	hauler.Origin = 0
	hauler.Dest = 2
	hauler.Pos = Vec2{X: 301.5, Y: -122.25}
	hauler.Order = OrderHaul
	hauler.Route = &Route{A: 0, B: 2, Reserve: 25}
	hauler.Route.Out[Alloy] = true
	hauler.Route.Out[Food] = true
	hauler.Route.Back[Lumen] = true
	hauler.DwellTimer, hauler.RetryTimer = 2.5, 1
	hauler.LaunchTick = 4100
	hs := NewShip(w.NewShipID(), Hauler, hp)
	hs.Cargo = Stock{Alloy: 75, Food: 60.5}
	hauler.Ships = []Ship{hs}
	w.AddFleet(hauler)

	raid := NewFleet(2, 0)
	raid.Home = 1
	raid.Origin = 1
	raid.Dest = 0
	raid.Order = OrderAttack
	raid.Stance = StanceRaid
	raid.RetreatAt = RetreatQuarter
	raid.Engaged = true
	raid.LaunchTick = 3500
	raid.ColonizeHex, raid.HasColonizeHex = Hex{1, -2}, true
	raid.SurveyTimer = 3.25
	for i := 0; i < 3; i++ {
		s := NewShip(w.NewShipID(), Dart, hp2)
		s.TargetShip = 2
		s.TargetBuilding = rig.ID
		raid.Ships = append(raid.Ships, s)
	}
	raid.Ships[0].HP = 12.25
	raid.InitialMaxHP = raid.TotalMaxHP()
	w.AddFleet(raid)

	// Slugs, market, events.
	w.Slugs = []Slug{{Owner: 2, From: 1, Target: 0, Pos: Vec2{X: 12.5, Y: 400.125}, Damage: 150}}
	w.Market.Pressure[Ore] = -0.0375
	w.Market.Pressure[Lumen] = 0.4
	w.Events = EventState{
		NextRollTick: 6000, FlareStartTick: 4200, FlareEndTick: 4650,
		Squalls:  []Squall{{Rock: 0, StartTick: 4400, EndTick: 4500, ImpactsLeft: 9, NextImpactTick: 4412}},
		LastKind: EventFlare, LastTick: 4200, Count: 2,
	}

	// Pending commands: one of every kind.
	site10 := Hex{2, -1}
	w.Enqueue(BuildCmd(1, 0, Hex{2, 0}, Habitat))
	w.Enqueue(DemolishCmd(1, 0, smelter.ID))
	w.Enqueue(RecommissionCmd(1, 2, derelict.ID))
	w.Enqueue(SetEnabledCmd(1, 0, smelter.ID, true))
	w.Enqueue(SetPriorityCmd(1, 0, rig.ID, PriorityHigh))
	w.Enqueue(ExpediteCmd(1, 0, site.ID))
	w.Enqueue(QueueShipCmd(1, 0, cradle.ID, Hauler))
	w.Enqueue(CancelShipCmd(1, 0, cradle.ID, 1))
	w.Enqueue(FleetOrderCmd(1, garrison.ID, []ShipID{1}, OrderSurvey, 2))
	w.Enqueue(AttackCmd(2, raid.ID, nil, 0, StanceRaze, RetreatHalf))
	w.Enqueue(ColonizeCmd(1, garrison.ID, []ShipID{2}, 2, &site10))
	w.Enqueue(SetRouteCmd(1, hauler.ID, nil, *hauler.Route))
	w.Enqueue(ScuttleCmd(1, garrison.ID, []ShipID{2}))
	w.Enqueue(SetResearchCmd(1, CivicPlanning))
	w.Enqueue(QueueResearchCmd(1, CharterTrade))
	w.Enqueue(CancelResearchCmd(1, -1))
	w.Enqueue(TradeCmd(1, 0, Ore, -40))
	w.Enqueue(SponsorCmd(1, 0))
	w.Enqueue(SetDriverTargetCmd(2, 1, driver.ID, NoRock))

	// Log and FX.
	w.Logf(0, SevInfo, NoRock, "A solar flare washes over the Shoal.")
	w.Logf(1, SevAlert, 0, "%s is under attack by %s!", r0.Name, rival.Name)
	w.Logf(2, SevGood, 1, "Research complete: %s", PointDefence)
	w.Logf(1, SevWarn, 0, "%v", (&ShortageError{Good: Alloy, Need: 35}).Error())
	fx := NewFX(FXShotFlak, 1, Hex{1, 0}, 2)
	fx.B = Vec2{X: 3, Y: 4}
	fx.Amount = 0.8
	w.Emit(fx)

	if err := w.Validate(); err != nil {
		t.Fatalf("hand-built world is invalid: %v", err)
	}
	return w
}

func TestWorldJSONRoundTrip(t *testing.T) {
	w := buildTestWorld(t)

	first, err := json.Marshal(w)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back World
	if err := json.Unmarshal(first, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	second, err := json.Marshal(&back)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !bytes.Equal(first, second) {
		i := 0
		for i < len(first) && i < len(second) && first[i] == second[i] {
			i++
		}
		lo, hi := max(0, i-80), min(len(first), i+80)
		t.Fatalf("round trip is not byte-identical; first difference at byte %d:\n  first:  ...%s...\n  second: ...%s...",
			i, first[lo:hi], second[lo:min(len(second), hi)])
	}
	if err := back.Validate(); err != nil {
		t.Errorf("restored world is invalid: %v", err)
	}

	// FX are transient and must not be saved.
	if strings.Contains(string(first), `"FX"`) {
		t.Error("FX must not appear in the serialised world")
	}
	if len(back.FX) != 0 {
		t.Errorf("restored world has %d FX", len(back.FX))
	}

	// Spot-check that real state survived (not just that both sides are
	// equally empty).
	if back.Tick != 4321 || back.Version != SaveVersion || back.Settings.Seed != 0xC0FFEE1234567890 {
		t.Errorf("header: tick %d, version %d, seed %#x", back.Tick, back.Version, back.Settings.Seed)
	}
	if back.RNG != w.RNG {
		t.Errorf("RNG state changed: %#x -> %#x", w.RNG.State, back.RNG.State)
	}
	if len(back.Rocks) != 3 || len(back.Factions) != 2 || len(back.Fleets) != 3 || len(back.Slugs) != 1 ||
		len(back.Pending) != 19 || len(back.Log) != 4 {
		t.Fatalf("collection sizes changed: %d rocks, %d factions, %d fleets, %d slugs, %d pending, %d log",
			len(back.Rocks), len(back.Factions), len(back.Fleets), len(back.Slugs), len(back.Pending), len(back.Log))
	}
	r0 := back.Rock(0)
	if r0.ShapeSeed != 0xFEDCBA9876543210 || r0.Colony.Stock[Alloy] != 143.875 || r0.Colony.LastAttackedTick != 4000 {
		t.Errorf("rock 0 state lost: %+v", r0.Colony)
	}
	if tile := r0.TileAt(Hex{1, 1}); tile.Deposit != Ore || tile.Amount != 733.125 || !tile.KnownTo(1) || !tile.KnownTo(2) {
		t.Errorf("ore tile lost: %+v", tile)
	}
	if tile := r0.TileAt(Hex{4, -4}); tile.Terrain != TerrainVoid || tile.Deposit != GoodNone {
		t.Errorf("void tile changed: %+v", tile)
	}
	if !r0.TileAt(Hex{3, -1}).Rubble {
		t.Error("rubble lost")
	}
	if b := r0.BuildingAt(Hex{-1, 0}); b == nil || b.Kind != LaunchCradle || !slices.Equal(b.Queue, []ShipKind{Hauler, Prospector}) {
		t.Errorf("cradle lost: %+v", b)
	}
	if s := back.Rock(2).Salvage; s == nil || s.Goods[Parts] != 12 || s.Claimant != 1 {
		t.Errorf("salvage lost: %+v", s)
	}
	if f := back.Fleet(2); f == nil || f.Route == nil || !f.Route.Out[Food] || !f.Route.Back[Lumen] || f.Ships[0].Cargo[Food] != 60.5 || f.At != NoRock {
		t.Errorf("hauling fleet lost: %+v", f)
	}
	if f := back.Faction(2); f.AI.StrikeFleet != 3 || !slices.Equal(f.AI.Ints, []int32{7, -2, 0}) || f.Mods.ProdMul != 1.25 {
		t.Errorf("rival state lost: %+v", f.AI)
	}
	if f := back.Faction(1); f.Research.Current != Fabrication || f.Research.Progress[Fabrication] != 33.5 || !f.Techs[DeepBoring] || f.Mods.MiningMul != 1.25 {
		t.Errorf("player state lost: %+v", f.Research)
	}
	if c := back.Pending[10]; c.Kind != CmdFleetOrder || c.Order != OrderColonize || !c.HasHex || c.Hex != (Hex{2, -1}) {
		t.Errorf("colonize command lost: %+v", c)
	}
	if back.Log[1].Text != "Ansel's Rest is under attack by The Vane Compact!" || back.Log[3].Text != "Not enough Alloy (need 35)" {
		t.Errorf("log lost: %+v", back.Log)
	}
	if back.NextBuildingID != w.NextBuildingID || back.NextFleetID != 4 || back.NextShipID != 7 || back.LogSeq != 4 {
		t.Errorf("counters lost: %d %d %d %d", back.NextBuildingID, back.NextFleetID, back.NextShipID, back.LogSeq)
	}

	// A third generation is still identical, and Clone gives the same
	// bytes.
	var third World
	if err := json.Unmarshal(second, &third); err != nil {
		t.Fatal(err)
	}
	thirdBytes, _ := json.Marshal(&third)
	if !bytes.Equal(first, thirdBytes) {
		t.Error("third-generation save differs")
	}
	clone, err := w.Clone()
	if err != nil {
		t.Fatalf("Clone: %v", err)
	}
	cloneBytes, _ := json.Marshal(clone)
	if !bytes.Equal(first, cloneBytes) {
		t.Error("Clone serialises differently from the original")
	}
	// The clone is independent of the original.
	clone.Rock(0).Colony.Stock[Alloy] = 1
	clone.Fleets[0].Ships[0].HP = 1
	if w.Rock(0).Colony.Stock[Alloy] != 143.875 || w.Fleets[0].Ships[0].HP == 1 {
		t.Error("Clone shares state with the original")
	}

	// Indented output round-trips as well (the save package may choose
	// either).
	pretty, err := json.MarshalIndent(w, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	var fromPretty World
	if err := json.Unmarshal(pretty, &fromPretty); err != nil {
		t.Fatal(err)
	}
	compact, _ := json.Marshal(&fromPretty)
	if !bytes.Equal(first, compact) {
		t.Error("indented save does not restore the same world")
	}
}

func TestEmptyWorldJSON(t *testing.T) {
	for _, w := range []*World{{}, NewWorld(DefaultSettings(1))} {
		a, err := json.Marshal(w)
		if err != nil {
			t.Fatal(err)
		}
		var back World
		if err := json.Unmarshal(a, &back); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(&back)
		if !bytes.Equal(a, b) {
			t.Errorf("empty world does not round-trip:\n%s\n%s", a, b)
		}
		if err := w.Validate(); err != nil {
			t.Errorf("empty world is invalid: %v", err)
		}
	}
}

func TestWorldLookups(t *testing.T) {
	w := buildTestWorld(t)
	if w.Rock(0) == nil || w.Rock(2).Name != "Glint Seven" {
		t.Error("Rock lookup failed")
	}
	for _, id := range []RockID{NoRock, -5, 3, 1000} {
		if w.Rock(id) != nil {
			t.Errorf("Rock(%d) should be nil", id)
		}
	}
	if w.Faction(1).Name != "Free Charter of Ansel" || w.Faction(2).Personality != PersonalityVane {
		t.Error("Faction lookup failed")
	}
	for _, id := range []FactionID{0, 3, 200} {
		if w.Faction(id) != nil {
			t.Errorf("Faction(%d) should be nil", id)
		}
	}
	if w.Human() != w.Faction(1) {
		t.Error("Human() must be faction 1")
	}
	if w.Fleet(2) == nil || w.Fleet(2).Order != OrderHaul {
		t.Error("Fleet lookup failed")
	}
	for _, id := range []FleetID{0, -1, 4, 99} {
		if w.Fleet(id) != nil {
			t.Errorf("Fleet(%d) should be nil", id)
		}
	}
	if b := w.Building(0, 2); b == nil || b.Kind != BoreRig {
		t.Errorf("Building(0, 2) = %+v", b)
	}
	if w.Building(1, 2) != nil || w.Building(9, 2) != nil || w.Building(0, 0) != nil {
		t.Error("Building must be nil for the wrong rock or ID")
	}
	if w.Colony(0) == nil || w.Colony(2) != nil || w.Colony(NoRock) != nil {
		t.Error("Colony lookup is wrong")
	}
	if w.Mods(1).MiningMul != 1.25 || w.Mods(0) != DefaultMods() || w.Mods(9) != DefaultMods() {
		t.Error("Mods lookup is wrong")
	}

	// Lookups on a nil world do not panic.
	var none *World
	if none.Rock(0) != nil || none.Faction(1) != nil || none.Fleet(1) != nil || none.Human() != nil ||
		none.Building(0, 1) != nil || none.Colony(0) != nil || none.FleetsAt(0) != nil ||
		none.GarrisonOf(0, 1) != nil || none.RocksOf(1) != nil || none.RemoveFleet(1) != nil ||
		none.BerthCap(1) != 0 || none.ShipCount(1) != 0 || none.FactionTotal(1, Alloy) != 0 ||
		none.FactionTake(1, Alloy, 5) || none.AliveCount() != 0 || none.Mods(1) != DefaultMods() {
		t.Error("nil world lookups must be inert")
	}
	if none.Validate() == nil {
		t.Error("a nil world is not valid")
	}

	// Queries.
	if got := w.RocksOf(1); len(got) != 1 || got[0].ID != 0 {
		t.Errorf("RocksOf(1) = %v", got)
	}
	if got := w.RocksOf(0); got != nil {
		t.Errorf("RocksOf(NoFaction) = %v, want nil", got)
	}
	if w.ColonyCount(1) != 1 || w.ColonyCount(2) != 1 || w.ColonyCount(0) != 0 {
		t.Error("ColonyCount is wrong")
	}
	if w.TotalPop(1) != 16.4 || w.TotalPop(2) != 21 {
		t.Error("TotalPop is wrong")
	}
	if w.AliveCount() != 2 {
		t.Errorf("AliveCount = %d", w.AliveCount())
	}
	if !Hostile(1, 2) || Hostile(1, 1) || Hostile(0, 1) || Hostile(2, 0) {
		t.Error("Hostile is wrong")
	}
}

func TestWorldTime(t *testing.T) {
	w := NewWorld(DefaultSettings(1))
	for _, tc := range []struct{ tick, cycle int }{{0, 1}, {599, 1}, {600, 2}, {1799, 3}, {1800, 4}, {6000, 11}} {
		w.Tick = tc.tick
		if got := w.Cycle(); got != tc.cycle {
			t.Errorf("tick %d: Cycle() = %d, want %d", tc.tick, got, tc.cycle)
		}
	}
	w.Tick = 1234
	if got := w.Seconds(); got != 123.4 {
		t.Errorf("Seconds() = %v", got)
	}
	// Slow systems run every 10 ticks.
	for tick := 0; tick < 40; tick++ {
		w.Tick = tick
		if got, want := w.SlowTick(), tick%10 == 0; got != want {
			t.Errorf("tick %d: SlowTick = %v", tick, got)
		}
		// AI: (Tick + 5*f) % 20 == 0.
		for f := FactionID(1); f <= 4; f++ {
			if got, want := w.AITick(f), (tick+5*int(f))%20 == 0; got != want {
				t.Errorf("tick %d faction %d: AITick = %v", tick, f, got)
			}
		}
	}
	// Each faction thinks exactly once per AIInterval.
	for f := FactionID(1); f <= 4; f++ {
		n := 0
		for tick := 100; tick < 100+AIInterval; tick++ {
			w.Tick = tick
			if w.AITick(f) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("faction %d thinks %d times per interval", f, n)
		}
	}

	w.Tick = 500
	if w.FlareActive() || w.SensorMul() != 1 || w.FlareBonus() != 1 {
		t.Error("no flare expected")
	}
	w.Events.FlareStartTick, w.Events.FlareEndTick = 500, 500+SecondsToTicks(FlareSeconds)
	if !w.FlareActive() || w.SensorMul() != 0.5 || w.FlareBonus() != 1.6 {
		t.Error("flare expected")
	}
	w.Tick = 949
	if !w.FlareActive() {
		t.Error("flare lasts until its end tick")
	}
	w.Tick = 950
	if w.FlareActive() {
		t.Error("flare is over at its end tick")
	}
}

func TestIDCounters(t *testing.T) {
	// Counters work on a zero world and on NewWorld alike, starting at 1.
	for name, w := range map[string]*World{"zero": {}, "new": NewWorld(GameSettings{})} {
		for want := 1; want <= 5; want++ {
			if got := w.NewBuildingID(); int(got) != want {
				t.Errorf("%s: NewBuildingID = %d, want %d", name, got, want)
			}
			if got := w.NewFleetID(); int(got) != want {
				t.Errorf("%s: NewFleetID = %d, want %d", name, got, want)
			}
			if got := w.NewShipID(); int(got) != want {
				t.Errorf("%s: NewShipID = %d, want %d", name, got, want)
			}
		}
		if w.NextBuildingID != 6 || w.NextFleetID != 6 || w.NextShipID != 6 {
			t.Errorf("%s: counters = %d %d %d", name, w.NextBuildingID, w.NextFleetID, w.NextShipID)
		}
	}
}

func TestFleetOrdering(t *testing.T) {
	w := NewWorld(GameSettings{})
	ids := func() []FleetID {
		var out []FleetID
		for _, f := range w.Fleets {
			out = append(out, f.ID)
		}
		return out
	}
	if w.AddFleet(nil) != nil || len(w.Fleets) != 0 {
		t.Error("AddFleet(nil) must be ignored")
	}
	// Auto-assigned IDs are ascending.
	a := w.AddFleet(NewFleet(1, 0))
	b := w.AddFleet(NewFleet(1, 0))
	c := w.AddFleet(NewFleet(2, 1))
	if a.ID != 1 || b.ID != 2 || c.ID != 3 {
		t.Fatalf("auto IDs = %d %d %d", a.ID, b.ID, c.ID)
	}
	// Explicit IDs in scrambled order land in sorted positions and push the
	// counter ahead.
	for _, id := range []FleetID{40, 7, 25, 12, 9} {
		f := NewFleet(1, 0)
		f.ID = id
		w.AddFleet(f)
	}
	if got, want := ids(), []FleetID{1, 2, 3, 7, 9, 12, 25, 40}; !slices.Equal(got, want) {
		t.Fatalf("fleet order = %v, want %v", got, want)
	}
	if w.NextFleetID != 41 {
		t.Errorf("NextFleetID = %d, want 41", w.NextFleetID)
	}
	if d := w.AddFleet(NewFleet(2, 1)); d.ID != 41 {
		t.Errorf("next auto ID = %d, want 41", d.ID)
	}
	// Adding an existing ID replaces the entry instead of duplicating it.
	replacement := NewFleet(2, 1)
	replacement.ID = 12
	w.AddFleet(replacement)
	if got := ids(); len(got) != 9 || w.Fleet(12) != replacement {
		t.Errorf("replacement failed: %v", got)
	}
	for _, id := range ids() {
		if f := w.Fleet(id); f == nil || f.ID != id {
			t.Errorf("Fleet(%d) lookup failed", id)
		}
	}

	// Removal keeps the order: middle, first, last, missing.
	if w.RemoveFleet(9) == nil {
		t.Error("RemoveFleet(9) failed")
	}
	if w.RemoveFleet(1) != a {
		t.Error("RemoveFleet(1) returned the wrong fleet")
	}
	if w.RemoveFleet(41) == nil {
		t.Error("RemoveFleet(41) failed")
	}
	if w.RemoveFleet(9) != nil || w.RemoveFleet(0) != nil || w.RemoveFleet(1000) != nil {
		t.Error("removing a missing fleet must return nil")
	}
	if got, want := ids(), []FleetID{2, 3, 7, 12, 25, 40}; !slices.Equal(got, want) {
		t.Fatalf("after removals: %v, want %v", got, want)
	}
	// IDs are never reused.
	if e := w.AddFleet(NewFleet(1, 0)); e.ID != 42 {
		t.Errorf("ID after removals = %d, want 42", e.ID)
	}
	if !slices.IsSortedFunc(w.Fleets, func(x, y *Fleet) int { return int(x.ID) - int(y.ID) }) {
		t.Error("fleets are not sorted")
	}
}

func TestFleetQueries(t *testing.T) {
	w := buildTestWorld(t)
	at0 := w.FleetsAt(0)
	if len(at0) != 2 || at0[0].ID != 1 || at0[1].ID != 3 {
		t.Errorf("FleetsAt(0) = %v", at0)
	}
	if got := w.FleetsAt(1); len(got) != 0 {
		t.Errorf("FleetsAt(1) = %v", got)
	}
	if got := w.FleetsAt(NoRock); got != nil {
		t.Errorf("FleetsAt(NoRock) = %v; fleets in transit are not 'at' a rock", got)
	}
	if got := w.FleetsOf(1); len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Errorf("FleetsOf(1) = %v", got)
	}
	if got := w.FleetsOf(2); len(got) != 1 || got[0].ID != 3 {
		t.Errorf("FleetsOf(2) = %v", got)
	}
	if g := w.GarrisonOf(0, 1); g == nil || g.ID != 1 {
		t.Errorf("GarrisonOf(0, 1) = %+v", g)
	}
	// The raiding fleet at rock 0 is not a garrison.
	if w.GarrisonOf(0, 2) != nil || w.GarrisonOf(1, 1) != nil || w.GarrisonOf(NoRock, 1) != nil || w.GarrisonOf(0, 0) != nil {
		t.Error("GarrisonOf must be nil where there is none")
	}

	// Positions.
	if got, want := w.FleetPos(w.Fleet(1)), w.Rock(0).PosAt(w.Tick); got != want {
		t.Errorf("FleetPos in orbit = %v, want the rock's position %v", got, want)
	}
	if got := w.FleetPos(w.Fleet(2)); got != (Vec2{X: 301.5, Y: -122.25}) {
		t.Errorf("FleetPos in transit = %v", got)
	}
	if w.FleetPos(nil) != (Vec2{}) {
		t.Error("FleetPos(nil) must be the origin")
	}
	if got, want := w.RockPos(1), w.Rock(1).PosAt(w.Tick); got != want {
		t.Errorf("RockPos = %v, want %v", got, want)
	}
	d := w.RockDist(0, 1)
	if want := w.Rock(0).PosAt(w.Tick).Dist(w.Rock(1).PosAt(w.Tick)); d != want || d <= 0 {
		t.Errorf("RockDist = %v, want %v", d, want)
	}
	if w.RockDist(0, 0) != 0 || w.RockDist(0, 9) != 0 || w.RockDist(NoRock, 1) != 0 {
		t.Error("RockDist degenerate cases are wrong")
	}
	if got := w.RockDistAt(0, 1, 0); math.Abs(got-w.Rock(0).PosAt(0).Dist(w.Rock(1).PosAt(0))) > 1e-12 {
		t.Errorf("RockDistAt = %v", got)
	}
	// Distances between two orbits stay within the triangle bounds.
	for tick := 0; tick < 200000; tick += 7919 {
		got := w.RockDistAt(0, 1, tick)
		if got < 610.5-520-1e-9 || got > 610.5+520+1e-9 {
			t.Fatalf("RockDistAt(tick %d) = %v out of bounds", tick, got)
		}
	}
}

func TestBerthsAndShipCount(t *testing.T) {
	w := buildTestWorld(t)
	// Player: Prospector + Dart in the garrison, a Hauler in transit, and a
	// Hauler + Prospector queued.
	if got := w.ShipCount(1); got != 5 {
		t.Errorf("ShipCount(1) = %d, want 5", got)
	}
	if got := w.BerthsUsed(1); got != 5 {
		t.Errorf("BerthsUsed(1) = %d, want 5 (all one-berth ships)", got)
	}
	if got := w.BerthCap(1); got != 6 {
		t.Errorf("BerthCap(1) = %d, want 6 (one Keystone)", got)
	}
	if got := w.BerthsFree(1); got != 1 {
		t.Errorf("BerthsFree(1) = %d, want 1", got)
	}
	// Rival: three Darts, nothing queued.
	if w.ShipCount(2) != 3 || w.BerthsUsed(2) != 3 || w.BerthCap(2) != 6 {
		t.Errorf("rival: ships %d, berths %d/%d", w.ShipCount(2), w.BerthsUsed(2), w.BerthCap(2))
	}
	if w.ShipCount(0) != 0 || w.BerthCap(0) != 0 || w.BerthsUsed(0) != 0 {
		t.Error("NoFaction has no ships or berths")
	}

	// Dock Rings add six berths each, but only while operational.
	r0 := w.Rock(0)
	ring := NewBuilding(w.NewBuildingID(), DockRing, 1, Hex{2, -2}, 1)
	if err := r0.AddBuilding(ring); err != nil {
		t.Fatal(err)
	}
	if got := w.BerthCap(1); got != 6 {
		t.Errorf("a Dock Ring under construction added berths: %d", got)
	}
	ring.Complete()
	if got := w.BerthCap(1); got != 12 {
		t.Errorf("BerthCap with a Dock Ring = %d, want 12", got)
	}
	ring.Enabled = false
	if got := w.BerthCap(1); got != 6 {
		t.Errorf("a disabled Dock Ring added berths: %d", got)
	}
	ring.Enabled = true
	ring.MakeDerelict()
	if got := w.BerthCap(1); got != 6 {
		t.Errorf("a derelict Dock Ring added berths: %d", got)
	}

	// Multi-berth ships count with their berth size.
	heavy := NewFleet(2, 1)
	heavy.Ships = []Ship{NewShip(w.NewShipID(), Bulwark, 1), NewShip(w.NewShipID(), Cutter, 1)}
	w.AddFleet(heavy)
	if got := w.BerthsUsed(2); got != 3+4+2 {
		t.Errorf("BerthsUsed(2) = %d, want 9", got)
	}
	if got := w.ShipCount(2); got != 5 {
		t.Errorf("ShipCount(2) = %d, want 5", got)
	}
	// Queued Seedships count two berths.
	cradle := r0.FirstOfKind(LaunchCradle, false)
	cradle.Queue = append(cradle.Queue, Seedship)
	if got := w.BerthsUsed(1); got != 7 {
		t.Errorf("BerthsUsed(1) with a queued Seedship = %d, want 7", got)
	}
}

func TestFactionTotalAndTake(t *testing.T) {
	newWorld := func(stocks ...float64) *World {
		w := NewWorld(GameSettings{})
		w.Factions = []*Faction{NewFaction(1, "A", PersonalityNone), NewFaction(2, "B", PersonalityVane)}
		for i, s := range stocks {
			r := testRock(RockID(i), 2)
			r.Owner = 1
			r.Colony = NewColony(0)
			r.Colony.Stock[Lumen] = s
			r.Colony.Stock[Alloy] = 100
			w.Rocks = append(w.Rocks, r)
		}
		// One rock of the other faction and one unowned rock, which must
		// never be touched.
		other := testRock(RockID(len(stocks)), 2)
		other.Owner = 2
		other.Colony = NewColony(0)
		other.Colony.Stock[Lumen] = 500
		bare := testRock(RockID(len(stocks)+1), 2)
		w.Rocks = append(w.Rocks, other, bare)
		return w
	}
	lumen := func(w *World) []float64 {
		var out []float64
		for _, r := range w.RocksOf(1) {
			out = append(out, r.Colony.Stock[Lumen])
		}
		return out
	}

	tests := []struct {
		name   string
		stocks []float64
		take   float64
		ok     bool
		want   []float64
	}{
		{"single rock", []float64{50}, 20, true, []float64{30}},
		{"richest first", []float64{10, 40, 25}, 30, true, []float64{10, 10, 25}},
		{"spills to the next richest", []float64{10, 40, 25}, 50, true, []float64{10, 0, 15}},
		{"spills through all", []float64{10, 40, 25}, 70, true, []float64{5, 0, 0}},
		{"exact total", []float64{10, 40, 25}, 75, true, []float64{0, 0, 0}},
		{"too much takes nothing", []float64{10, 40, 25}, 75.5, false, []float64{10, 40, 25}},
		{"tie goes to the lower rock ID", []float64{30, 30, 30}, 30, true, []float64{0, 30, 30}},
		{"tie then spill", []float64{30, 30, 30}, 45, true, []float64{0, 15, 30}},
		{"zero amount is free", []float64{5}, 0, true, []float64{5}},
		{"negative amount is free", []float64{5}, -3, true, []float64{5}},
		{"nothing owned", nil, 1, false, nil},
		{"float dust is tolerated", []float64{20, 10}, 30 + 1e-12, true, []float64{0, 0}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld(tc.stocks...)
			total := 0.0
			for _, s := range tc.stocks {
				total += s
			}
			if got := w.FactionTotal(1, Lumen); got != total {
				t.Errorf("FactionTotal = %v, want %v", got, total)
			}
			if got := w.FactionTake(1, Lumen, tc.take); got != tc.ok {
				t.Fatalf("FactionTake(%v) = %v, want %v", tc.take, got, tc.ok)
			}
			got := lumen(w)
			if len(got) != len(tc.want) {
				t.Fatalf("stocks = %v, want %v", got, tc.want)
			}
			for i := range got {
				if math.Abs(got[i]-tc.want[i]) > 1e-9 {
					t.Errorf("stocks = %v, want %v", got, tc.want)
					break
				}
			}
			// Other goods and other factions are untouched.
			for _, r := range w.RocksOf(1) {
				if r.Colony.Stock[Alloy] != 100 {
					t.Errorf("rock %d: Alloy changed to %v", r.ID, r.Colony.Stock[Alloy])
				}
			}
			if got := w.FactionTotal(2, Lumen); got != 500 {
				t.Errorf("the other faction's Lumen changed to %v", got)
			}
		})
	}

	w := newWorld(10, 20)
	if w.FactionTake(1, GoodNone, 5) || w.FactionTake(0, Lumen, 5) || w.FactionTake(9, Lumen, 5) {
		t.Error("FactionTake must fail for invalid goods and factions")
	}
	if w.FactionTotal(1, GoodNone) != 0 || w.FactionTotal(0, Lumen) != 0 {
		t.Error("FactionTotal must be 0 for invalid goods and factions")
	}
}

func TestLogf(t *testing.T) {
	w := &World{Tick: 77}
	w.Logf(1, SevWarn, 3, "Not enough %s (need %d)", Alloy, 35)
	if len(w.Log) != 1 {
		t.Fatalf("log has %d entries", len(w.Log))
	}
	e := w.Log[0]
	if e.Seq != 1 || e.Tick != 77 || e.Faction != 1 || e.Sev != SevWarn || e.Rock != 3 || e.Text != "Not enough Alloy (need 35)" {
		t.Errorf("entry = %+v", e)
	}
	if !e.VisibleTo(1) || e.VisibleTo(2) {
		t.Error("an addressed entry is visible to its addressee only")
	}
	w.Logf(NoFaction, SevInfo, NoRock, "broadcast")
	if b := w.Log[1]; !b.VisibleTo(1) || !b.VisibleTo(2) || b.Rock != NoRock || b.Seq != 2 {
		t.Errorf("broadcast entry = %+v", b)
	}

	// The log is capped; the oldest entries go first and Seq keeps
	// counting.
	for i := 0; i < MaxLogEntries+50; i++ {
		w.Tick++
		w.Logf(0, SevInfo, NoRock, "entry %d", i)
	}
	if len(w.Log) != MaxLogEntries {
		t.Fatalf("log has %d entries, want %d", len(w.Log), MaxLogEntries)
	}
	if w.LogSeq != MaxLogEntries+52 {
		t.Errorf("LogSeq = %d, want %d", w.LogSeq, MaxLogEntries+52)
	}
	if last := w.Log[len(w.Log)-1]; last.Seq != w.LogSeq || last.Text != "entry 349" {
		t.Errorf("last entry = %+v", last)
	}
	if first := w.Log[0]; first.Seq != w.LogSeq-MaxLogEntries+1 || first.Text != "entry 50" {
		t.Errorf("first entry = %+v", first)
	}
	for i := 1; i < len(w.Log); i++ {
		if w.Log[i].Seq != w.Log[i-1].Seq+1 {
			t.Fatalf("log sequence breaks at %d", i)
		}
	}
}

func TestEmitAndTakeFX(t *testing.T) {
	w := &World{}
	if got := w.TakeFX(); got != nil {
		t.Errorf("TakeFX on an empty buffer = %v", got)
	}
	w.Emit(NewFX(FXBuildPlaced, 2, Hex{1, 1}, 1))
	w.Emit(FX{Kind: FXFlareStart, Rock: NoRock})
	if len(w.FX) != 2 || w.FX[0].Kind != FXBuildPlaced || w.FX[0].Rock != 2 || w.FX[0].Hex != (Hex{1, 1}) || w.FX[0].Faction != 1 {
		t.Fatalf("FX = %+v", w.FX)
	}
	got := w.TakeFX()
	if len(got) != 2 || len(w.FX) != 0 {
		t.Fatalf("TakeFX returned %d, left %d", len(got), len(w.FX))
	}
	// Emitting after a drain does not disturb what the caller took.
	w.Emit(FX{Kind: FXAlert})
	if got[0].Kind != FXBuildPlaced || got[1].Kind != FXFlareStart {
		t.Error("TakeFX result was overwritten by a later Emit")
	}

	// An undrained buffer stays bounded and keeps the newest cues.
	w = &World{}
	for i := 0; i < 3*MaxFX+17; i++ {
		w.Emit(FX{Kind: FXBomb, Amount: float64(i)})
		if len(w.FX) > MaxFX {
			t.Fatalf("FX buffer grew to %d", len(w.FX))
		}
	}
	if last := w.FX[len(w.FX)-1]; last.Amount != float64(3*MaxFX+16) {
		t.Errorf("newest FX lost: %+v", last)
	}
	for i := 1; i < len(w.FX); i++ {
		if w.FX[i].Amount != w.FX[i-1].Amount+1 {
			t.Fatalf("FX order broken at %d", i)
		}
	}
}

func TestValidateCatchesCorruption(t *testing.T) {
	tests := []struct {
		name   string
		break_ func(w *World)
	}{
		{"rock ID mismatch", func(w *World) { w.Rocks[1].ID = 5 }},
		{"nil rock", func(w *World) { w.Rocks[2] = nil }},
		{"tile count", func(w *World) { w.Rocks[0].Tiles = w.Rocks[0].Tiles[:10] }},
		{"owner without colony", func(w *World) { w.Rocks[0].Colony = nil }},
		{"colony without owner", func(w *World) { w.Rocks[2].Colony = NewColony(0) }},
		{"unknown owner", func(w *World) { w.Rocks[2].Owner = 6; w.Rocks[2].Colony = NewColony(0) }},
		{"bad terrain", func(w *World) { w.Rocks[0].Tiles[40].Terrain = 9 }},
		{"non-raw deposit", func(w *World) { w.Rocks[0].TileAt(Hex{1, 1}).Deposit = Alloy }},
		{"negative deposit", func(w *World) { w.Rocks[0].TileAt(Hex{1, 1}).Amount = -1 }},
		{"dangling tile reference", func(w *World) { w.Rocks[0].TileAt(Hex{3, 0}).Building = 999 }},
		{"tile reference cleared", func(w *World) { w.Rocks[0].TileAt(Hex{0, 0}).Building = 0 }},
		{"buildings unsorted", func(w *World) {
			b := w.Rocks[0].Buildings
			b[1], b[2] = b[2], b[1]
		}},
		{"building ID beyond counter", func(w *World) { w.NextBuildingID = 3 }},
		{"building on two rocks", func(w *World) {
			dup := *w.Rocks[0].Buildings[1]
			dup.Hex = Hex{1, 1}
			dup.Owner = 2
			w.Rocks[1].Buildings = slices.Insert(w.Rocks[1].Buildings, 0, &dup)
			w.Rocks[1].TileAt(Hex{1, 1}).Building = dup.ID
		}},
		{"derelict with owner", func(w *World) { w.Rocks[2].Buildings[0].Owner = 1 }},
		{"foreign building", func(w *World) { w.Rocks[0].Buildings[1].Owner = 2 }},
		{"active building on unowned rock", func(w *World) { w.Rocks[2].Buildings[0].State = StateActive }},
		{"no keystone", func(w *World) { w.Rocks[0].Buildings[0].Kind = Silo }},
		{"two keystones", func(w *World) { w.Rocks[0].Buildings[1].Kind = Keystone }},
		{"bad building kind", func(w *World) { w.Rocks[0].Buildings[2].Kind = 99 }},
		{"NaN hit points", func(w *World) { w.Rocks[0].Buildings[2].HP = math.NaN() }},
		{"oversized ship queue", func(w *World) {
			c := w.Rocks[0].FirstOfKind(LaunchCradle, false)
			c.Queue = []ShipKind{Dart, Dart, Dart, Dart, Dart, Dart}
		}},
		{"bad queued ship", func(w *World) { w.Rocks[0].FirstOfKind(LaunchCradle, false).Queue[0] = ShipNone }},
		{"bad driver target", func(w *World) { w.Rocks[1].FirstOfKind(MassDriver, false).DriverTarget = 44 }},
		{"negative stock", func(w *World) { w.Rocks[0].Colony.Stock[Food] = -2 }},
		{"infinite population", func(w *World) { w.Rocks[0].Colony.Pop = math.Inf(1) }},
		{"faction ID mismatch", func(w *World) { w.Factions[1].ID = 5 }},
		{"nil faction", func(w *World) { w.Factions[0] = nil }},
		{"zero mods", func(w *World) { w.Factions[0].Mods = Mods{} }},
		{"bad current research", func(w *World) { w.Factions[0].Research.Current = 60 }},
		{"oversized research queue", func(w *World) {
			w.Factions[0].Research.Queue = []TechID{Gunnery, Gunnery, Gunnery, Gunnery, Gunnery, Gunnery}
		}},
		{"bad home rock", func(w *World) { w.Factions[1].HomeRock = 17 }},
		{"fleets unsorted", func(w *World) { w.Fleets[0], w.Fleets[1] = w.Fleets[1], w.Fleets[0] }},
		{"nil fleet", func(w *World) { w.Fleets[1] = nil }},
		{"fleet ID beyond counter", func(w *World) { w.NextFleetID = 2 }},
		{"empty fleet", func(w *World) { w.Fleets[0].Ships = nil }},
		{"fleet with unknown owner", func(w *World) { w.Fleets[0].Owner = 5 }},
		{"fleet at missing rock", func(w *World) { w.Fleets[0].At = 12 }},
		{"garrison in transit", func(w *World) { w.Fleets[1].Garrison = true }},
		{"two garrisons", func(w *World) {
			g := NewFleet(1, 0)
			g.Garrison = true
			g.Ships = []Ship{NewShip(w.NewShipID(), Dart, 1)}
			w.AddFleet(g)
		}},
		{"duplicate ship ID", func(w *World) { w.Fleets[2].Ships[0].ID = w.Fleets[0].Ships[0].ID }},
		{"ship ID beyond counter", func(w *World) { w.NextShipID = 2 }},
		{"bad ship kind", func(w *World) { w.Fleets[0].Ships[0].Kind = ShipNone }},
		{"negative cargo", func(w *World) { w.Fleets[1].Ships[0].Cargo[Alloy] = -1 }},
		{"route to missing rock", func(w *World) { w.Fleets[1].Route.B = 30 }},
		{"slug to missing rock", func(w *World) { w.Slugs[0].Target = NoRock }},
		{"NaN market", func(w *World) { w.Market.Pressure[Fuel] = math.NaN() }},
		{"missing winner", func(w *World) { w.Winner = 4 }},
		{"negative tick", func(w *World) { w.Tick = -1 }},
		{"too many factions", func(w *World) {
			for len(w.Factions) < MaxFactions+1 {
				w.Factions = append(w.Factions, NewFaction(FactionID(len(w.Factions)+1), "X", PersonalityVane))
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := buildTestWorld(t)
			tc.break_(w)
			if err := w.Validate(); err == nil {
				t.Error("Validate accepted a corrupted world")
			}
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	all := []error{
		ErrNotMine, ErrUnknownCommand, ErrBadCommand, ErrGameOver, ErrNoFaction, ErrEliminated,
		ErrNoRock, ErrNoColony, ErrNoBuilding, ErrNoFleet, ErrNoShip, ErrNotOwner, ErrLocked,
		ErrRockClaimed, ErrBadHex, ErrTileOccupied, ErrBadBuilding,
	}
	seen := map[string]bool{}
	for i, e := range all {
		if e == nil || e.Error() == "" {
			t.Fatalf("sentinel %d is empty", i)
		}
		if seen[e.Error()] {
			t.Errorf("duplicate sentinel text %q", e.Error())
		}
		seen[e.Error()] = true
		for j, other := range all {
			if (i == j) != errors.Is(e, other) {
				t.Errorf("errors.Is(%q, %q) is wrong", e, other)
			}
		}
	}
}
