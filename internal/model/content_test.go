package model

import (
	"math"
	"slices"
	"strings"
	"testing"
)

func TestValidateContent(t *testing.T) {
	if err := ValidateContent(); err != nil {
		t.Fatalf("content tables are inconsistent:\n%v", err)
	}
}

// TestEnumValuesStable pins the numeric value of every enumeration constant.
// These numbers are written into save files; a failure here means a change
// broke the save format.
func TestEnumValuesStable(t *testing.T) {
	check := func(name string, got, want int) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %d, want %d (save format changed)", name, got, want)
		}
	}
	// Goods.
	for i, g := range []Good{Ore, Ice, Silica, Lumen, Alloy, Fuel, Parts, Food} {
		check("Good "+g.String(), int(g), i)
	}
	check("NumGoods", NumGoods, 8)
	check("GoodNone", int(GoodNone), -1)
	// Terrain, rock classes.
	for i, v := range []Terrain{TerrainVoid, TerrainRegolith, TerrainCrag, TerrainFissure} {
		check("Terrain "+v.String(), int(v), i)
	}
	check("NumTerrains", NumTerrains, 4)
	for i, v := range []RockClass{ClassFerric, ClassStony, ClassFrost, ClassGlint} {
		check("RockClass "+v.String(), int(v), i)
	}
	check("NumRockClasses", NumRockClasses, 4)
	// Categories and states.
	for i, v := range []Category{CatCore, CatPower, CatLife, CatIndustry, CatScience, CatSpace, CatDefence} {
		check("Category "+v.String(), int(v), i)
	}
	check("NumCategories", NumCategories, 7)
	for i, v := range []BuildState{StateConstructing, StateActive, StateDerelict} {
		check("BuildState "+v.String(), int(v), i)
	}
	check("NumBuildStates", NumBuildStates, 3)
	// Buildings.
	buildings := []BuildingKind{
		Keystone, Conduit, Silo, SurveyMast, Sunvane, ThermalStack, LumenCore, Habitat, Aerator,
		Greenhouse, Commons, BoreRig, Smelter, Cracker, Fabricator, Laboratory, LaunchCradle,
		DockRing, ExchangeDock, FarseerArray, MenderHub, FlakBattery, LanceTower, AegisProjector,
		MassDriver,
	}
	for i, v := range buildings {
		check("BuildingKind "+v.String(), int(v), i)
	}
	check("NumBuildingKinds", NumBuildingKinds, 25)
	check("BuildingNone", int(BuildingNone), -1)
	for i, v := range []IdleReason{
		IdleNone, IdleDisabled, IdleNoPower, IdleNoStaff, IdleNoInput, IdleStorageFull,
		IdleNoDeposit, IdleNoTarget, IdleNoWork,
	} {
		check("IdleReason "+v.String(), int(v), i)
	}
	check("NumIdleReasons", NumIdleReasons, 9)
	// Ships.
	for i, v := range []Hull{HullLight, HullMedium, HullHeavy} {
		check("Hull "+v.String(), int(v), i)
	}
	check("NumHulls", NumHulls, 3)
	for i, v := range []ShipKind{Prospector, Hauler, Seedship, Dart, Cutter, Hammer, Bulwark} {
		check("ShipKind "+v.String(), int(v), i)
	}
	check("NumShipKinds", NumShipKinds, 7)
	check("ShipNone", int(ShipNone), -1)
	for i, v := range []OrderKind{OrderHold, OrderMove, OrderAttack, OrderSurvey, OrderColonize, OrderHaul, OrderReturn} {
		check("OrderKind "+v.String(), int(v), i)
	}
	check("NumOrderKinds", NumOrderKinds, 7)
	for i, v := range []Stance{StanceSuppress, StanceRaid, StanceRaze} {
		check("Stance "+v.String(), int(v), i)
	}
	check("NumStances", NumStances, 3)
	// Techs.
	techs := []TechID{
		OrbitalFlight, VolatileCracking, Fabrication, CivicPlanning, PointDefence, DeepBoring,
		CharterTrade, ColonialCharter, Gunnery, SiegeDoctrine, DeepScan, FieldRepair, Photovoltaics,
		HydroStacks, DockEngineering, EfficientSmelting, MassAcceleration, LumenOptics, AegisFields,
		LumenFusion, CapitalHulls, ReactivePlating, OverdriveThrusters, Automation,
	}
	for i, v := range techs {
		check("TechID "+v.String(), int(v), i)
	}
	check("NumTechs", NumTechs, 24)
	check("TechNone", int(TechNone), -1)
	// Commands.
	cmds := []CmdKind{
		CmdNone, CmdBuild, CmdDemolish, CmdRecommission, CmdSetEnabled, CmdSetPriority, CmdExpedite,
		CmdQueueShip, CmdCancelShip, CmdFleetOrder, CmdSetRoute, CmdScuttle, CmdSetResearch,
		CmdQueueResearch, CmdCancelResearch, CmdTrade, CmdSponsor, CmdSetDriverTarget,
	}
	for i, v := range cmds {
		check("CmdKind "+v.String(), int(v), i)
	}
	check("NumCmdKinds", NumCmdKinds, 18)
	for i, v := range []Severity{SevInfo, SevGood, SevWarn, SevAlert} {
		check("Severity "+v.String(), int(v), i)
	}
	check("NumSeverities", NumSeverities, 4)
	fxs := []FXKind{
		FXNone, FXBuildPlaced, FXBuildDone, FXBuildDestroyed, FXShipBuilt, FXFleetLaunch,
		FXFleetArrive, FXShotFlak, FXShotLance, FXShotShip, FXBomb, FXShipDestroyed, FXShieldHit,
		FXSlugFired, FXSlugImpact, FXMeteorImpact, FXColonyFounded, FXColonyFell, FXTechDone,
		FXDirectiveDone, FXAlert, FXTrade, FXFlareStart, FXFlareEnd,
	}
	for i, v := range fxs {
		check("FXKind "+v.String(), int(v), i)
	}
	check("NumFXKinds", NumFXKinds, 24)
	for i, v := range []EventKind{EventNone, EventSquall, EventFlare, EventRichSeam, EventHulk, EventRefugees, EventMarketShock} {
		check("EventKind "+v.String(), int(v), i)
	}
	check("NumEventKinds", NumEventKinds, 7)
	// Setup enums.
	for i, v := range []Personality{PersonalityNone, PersonalityOstrander, PersonalityVane, PersonalityMeridian} {
		check("Personality "+v.String(), int(v), i)
	}
	check("NumPersonalities", NumPersonalities, 4)
	for i, v := range []Difficulty{DiffEasy, DiffNormal, DiffHard} {
		check("Difficulty "+v.String(), int(v), i)
	}
	check("NumDifficulties", NumDifficulties, 3)
	for i, v := range []BeltSize{BeltSmall, BeltMedium, BeltLarge} {
		check("BeltSize "+v.String(), int(v), i)
	}
	check("NumBeltSizes", NumBeltSizes, 3)
	check("NoRock", int(NoRock), -1)
	check("NoFaction", int(NoFaction), 0)
	check("PlayerFaction", int(PlayerFaction), 1)
	check("MaxFactions", MaxFactions, 7)
}

func TestEnumStrings(t *testing.T) {
	tests := []struct{ got, want string }{
		{Ore.String(), "Ore"},
		{Food.String(), "Food"},
		{GoodNone.String(), "None"},
		{Good(42).String(), "Good(42)"},
		{TerrainRegolith.String(), "Regolith"},
		{Terrain(-1).String(), "Terrain(-1)"},
		{ClassGlint.String(), "Glint"},
		{CatDefence.String(), "Defence"},
		{StateDerelict.String(), "Derelict"},
		{SurveyMast.String(), "Survey Mast"},
		{AegisProjector.String(), "Aegis Projector"},
		{BuildingNone.String(), "None"},
		{BuildingKind(99).String(), "BuildingKind(99)"},
		{HullHeavy.String(), "Heavy"},
		{Bulwark.String(), "Bulwark"},
		{ShipNone.String(), "None"},
		{ShipKind(50).String(), "ShipKind(50)"},
		{OrderColonize.String(), "Colonize"},
		{StanceRaze.String(), "Raze"},
		{Photovoltaics.String(), "Tuned Photovoltaics"},
		{HydroStacks.String(), "Hydroponic Stacks"},
		{TechNone.String(), "None"},
		{TechID(77).String(), "TechID(77)"},
		{CmdSetDriverTarget.String(), "CmdSetDriverTarget"},
		{SevAlert.String(), "Alert"},
		{FXColonyFell.String(), "ColonyFell"},
		{EventHulk.String(), "Derelict hulk"},
		{PersonalityOstrander.String(), "Ostrander Deepworks"},
		{PersonalityVane.String(), "The Vane Compact"},
		{PersonalityMeridian.String(), "Meridian Lattice"},
		{DiffHard.String(), "Hard"},
		{BeltLarge.String(), "Large"},
		{IdleNoPower.String(), "No power"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("String() = %q, want %q", tc.got, tc.want)
		}
	}
	// Validity predicates.
	if GoodNone.Valid() || Good(NumGoods).Valid() || !Food.Valid() {
		t.Error("Good.Valid is wrong")
	}
	if BuildingNone.Valid() || BuildingKind(NumBuildingKinds).Valid() || !MassDriver.Valid() {
		t.Error("BuildingKind.Valid is wrong")
	}
	if ShipNone.Valid() || ShipKind(NumShipKinds).Valid() || !Bulwark.Valid() {
		t.Error("ShipKind.Valid is wrong")
	}
	if TechNone.Valid() || TechID(NumTechs).Valid() || !Automation.Valid() {
		t.Error("TechID.Valid is wrong")
	}
	if CmdNone.Valid() || !CmdBuild.Valid() || CmdKind(NumCmdKinds).Valid() {
		t.Error("CmdKind.Valid is wrong")
	}
	if FXNone.Valid() || !FXFlareEnd.Valid() || EventNone.Valid() || !EventMarketShock.Valid() {
		t.Error("FXKind.Valid or EventKind.Valid is wrong")
	}
	if PersonalityNone.IsRival() || !PersonalityVane.IsRival() || !PersonalityNone.Valid() {
		t.Error("Personality predicates are wrong")
	}
	for _, p := range RivalPersonalities() {
		if !p.IsRival() || p.Archetype() == "" || p.Blurb() == "" {
			t.Errorf("personality %d lacks rival data", p)
		}
	}
	// Raw goods.
	for _, g := range AllGoods() {
		if want := g == Ore || g == Ice || g == Silica || g == Lumen; g.Raw() != want {
			t.Errorf("%s.Raw() = %v", g, g.Raw())
		}
	}
	if RawGoods() != [4]Good{Ore, Ice, Silica, Lumen} {
		t.Error("RawGoods is wrong")
	}
	// All... helpers enumerate every value in order.
	if a := AllBuildingKinds(); a[0] != Keystone || a[NumBuildingKinds-1] != MassDriver {
		t.Error("AllBuildingKinds is wrong")
	}
	if a := AllShipKinds(); a[0] != Prospector || a[NumShipKinds-1] != Bulwark {
		t.Error("AllShipKinds is wrong")
	}
	if a := AllTechs(); a[0] != OrbitalFlight || a[NumTechs-1] != Automation {
		t.Error("AllTechs is wrong")
	}
	if a := AllCategories(); a[0] != CatCore || a[NumCategories-1] != CatDefence {
		t.Error("AllCategories is wrong")
	}
	if a := AllRockClasses(); a[0] != ClassFerric || a[NumRockClasses-1] != ClassGlint {
		t.Error("AllRockClasses is wrong")
	}
}

func TestCategoryRankAndPriority(t *testing.T) {
	// Staffing / power order: Life, Power, Industry, Science, Space,
	// Defence, Core.
	order := []Category{CatLife, CatPower, CatIndustry, CatScience, CatSpace, CatDefence, CatCore}
	for i, c := range order {
		if got := c.Rank(); got != i {
			t.Errorf("%s.Rank() = %d, want %d", c, got, i)
		}
	}
	if got := Category(99).Rank(); got != NumCategories {
		t.Errorf("unknown category rank = %d", got)
	}
	for _, c := range AllCategories() {
		want := PriorityNormal
		if c == CatLife || c == CatDefence {
			want = PriorityHigh
		}
		if got := c.DefaultPriority(); got != want {
			t.Errorf("%s.DefaultPriority() = %d, want %d", c, got, want)
		}
	}
	if BuildingDefOf(Aerator).DefaultPriority() != PriorityHigh ||
		BuildingDefOf(FlakBattery).DefaultPriority() != PriorityHigh ||
		BuildingDefOf(Smelter).DefaultPriority() != PriorityNormal ||
		BuildingDefOf(Keystone).DefaultPriority() != PriorityNormal {
		t.Error("BuildingDef.DefaultPriority is wrong")
	}
}

func TestGoodsTable(t *testing.T) {
	want := []struct {
		g     Good
		name  string
		value float64
		raw   bool
	}{
		{Ore, "Ore", 1.0, true},
		{Ice, "Ice", 1.2, true},
		{Silica, "Silica", 0.8, true},
		{Lumen, "Lumen", 12, true},
		{Alloy, "Alloy", 3.0, false},
		{Fuel, "Fuel", 3.0, false},
		{Parts, "Parts", 9.0, false},
		{Food, "Food", 1.5, false},
	}
	if len(Goods()) != NumGoods || len(want) != NumGoods {
		t.Fatalf("goods table has %d entries, want %d", len(Goods()), NumGoods)
	}
	for _, w := range want {
		info := GoodInfoOf(w.g)
		if info.Good != w.g || info.Name != w.name || info.BaseValue != w.value || info.Raw != w.raw {
			t.Errorf("GoodInfoOf(%s) = %+v", w.name, info)
		}
		if len(info.Short) != 3 || strings.ToUpper(info.Short) != info.Short {
			t.Errorf("%s: short name %q is not three upper-case letters", w.name, info.Short)
		}
		if Goods()[w.g] != info {
			t.Errorf("Goods()[%d] differs from GoodInfoOf", w.g)
		}
	}
	if info := GoodInfoOf(GoodNone); info.Good != GoodNone || info.BaseValue != 0 {
		t.Errorf("GoodInfoOf(GoodNone) = %+v", info)
	}
}

// TestBuildingTable re-states the building table of the specification and
// compares it with the content table, column by column.
func TestBuildingTable(t *testing.T) {
	const start = TechID(-2) // marker for "start unlocked" in this table only
	want := []struct {
		kind   BuildingKind
		name   string
		cat    Category
		cost   Stock
		build  float64
		power  float64
		staff  int
		hp     float64
		unlock TechID
	}{
		{Keystone, "Keystone", CatCore, Stock{}, 0, 20, 0, 1200, TechNone},
		{Conduit, "Conduit", CatCore, Stock{Alloy: 4}, 3, 0, 0, 60, start},
		{Silo, "Silo", CatCore, Stock{Alloy: 30, Silica: 10}, 15, -1, 0, 300, start},
		{SurveyMast, "Survey Mast", CatCore, Stock{Alloy: 15, Silica: 5}, 12, -2, 1, 100, start},
		{Sunvane, "Sunvane", CatPower, Stock{Alloy: 20, Silica: 15}, 15, 12, 0, 120, start},
		{ThermalStack, "Thermal Stack", CatPower, Stock{Alloy: 60, Silica: 20, Parts: 10}, 30, 45, 2, 250, VolatileCracking},
		{LumenCore, "Lumen Core", CatPower, Stock{Alloy: 150, Parts: 60, Lumen: 20}, 60, 160, 4, 400, LumenFusion},
		{Habitat, "Habitat", CatLife, Stock{Alloy: 30, Silica: 30}, 20, -3, 0, 200, start},
		{Aerator, "Aerator", CatLife, Stock{Alloy: 25, Silica: 10}, 18, -5, 1, 150, start},
		{Greenhouse, "Greenhouse", CatLife, Stock{Alloy: 20, Silica: 40}, 25, -6, 2, 140, start},
		{Commons, "Commons", CatLife, Stock{Alloy: 40, Silica: 40, Parts: 5}, 30, -4, 2, 180, CivicPlanning},
		{BoreRig, "Bore Rig", CatIndustry, Stock{Alloy: 35}, 20, -8, 3, 220, start},
		{Smelter, "Smelter", CatIndustry, Stock{Alloy: 50, Silica: 20}, 30, -14, 3, 260, start},
		{Cracker, "Cracker", CatIndustry, Stock{Alloy: 45, Silica: 15, Parts: 5}, 30, -10, 2, 220, VolatileCracking},
		{Fabricator, "Fabricator", CatIndustry, Stock{Alloy: 70, Silica: 30}, 40, -16, 4, 260, Fabrication},
		{Laboratory, "Laboratory", CatScience, Stock{Alloy: 60, Silica: 40, Parts: 10}, 45, -12, 4, 200, start},
		{LaunchCradle, "Launch Cradle", CatSpace, Stock{Alloy: 120, Silica: 40, Parts: 30}, 60, -15, 4, 400, OrbitalFlight},
		{DockRing, "Dock Ring", CatSpace, Stock{Alloy: 80, Parts: 20}, 40, -6, 2, 300, DockEngineering},
		{ExchangeDock, "Exchange Dock", CatSpace, Stock{Alloy: 60, Silica: 30, Parts: 15}, 40, -5, 2, 250, CharterTrade},
		{FarseerArray, "Farseer Array", CatSpace, Stock{Alloy: 50, Silica: 30, Parts: 15}, 35, -10, 2, 150, DeepScan},
		{MenderHub, "Mender Hub", CatDefence, Stock{Alloy: 60, Parts: 20}, 35, -8, 2, 200, FieldRepair},
		{FlakBattery, "Flak Battery", CatDefence, Stock{Alloy: 60, Parts: 10}, 30, -6, 2, 350, PointDefence},
		{LanceTower, "Lance Tower", CatDefence, Stock{Alloy: 90, Parts: 30, Lumen: 10}, 45, -20, 3, 400, LumenOptics},
		{AegisProjector, "Aegis Projector", CatDefence, Stock{Alloy: 100, Parts: 40, Lumen: 15}, 50, -30, 3, 300, AegisFields},
		{MassDriver, "Mass Driver", CatDefence, Stock{Alloy: 140, Parts: 50}, 60, -25, 4, 350, MassAcceleration},
	}
	if len(want) != NumBuildingKinds || len(Buildings()) != NumBuildingKinds {
		t.Fatalf("table sizes: test %d, content %d, want %d", len(want), len(Buildings()), NumBuildingKinds)
	}
	for _, w := range want {
		d := BuildingDefOf(w.kind)
		wantUnlock, wantStart := w.unlock, false
		if w.unlock == start {
			wantUnlock, wantStart = TechNone, true
		}
		if d.Kind != w.kind || d.Name != w.name || d.Category != w.cat || d.Cost != w.cost ||
			d.BuildTime != w.build || d.Power != w.power || d.Staff != w.staff || d.MaxHP != w.hp ||
			d.Unlock != wantUnlock || d.StartUnlocked != wantStart {
			t.Errorf("%s: content %+v does not match the specification row %+v", w.name, d, w)
		}
		if Buildings()[w.kind].Name != w.name {
			t.Errorf("Buildings()[%d] is %q, want %q", w.kind, Buildings()[w.kind].Name, w.name)
		}
	}

	// Starting unlocked list.
	wantStart := []BuildingKind{Conduit, Silo, SurveyMast, Sunvane, Habitat, Aerator, Greenhouse, BoreRig, Smelter, Laboratory}
	if got := StartBuildings(); !slices.Equal(got, wantStart) {
		t.Errorf("StartBuildings() = %v, want %v", got, wantStart)
	}

	// Per-rock limits.
	for _, k := range AllBuildingKinds() {
		wantMax := 0
		switch k {
		case Keystone, ExchangeDock:
			wantMax = 1
		case LaunchCradle:
			wantMax = 3
		}
		if got := BuildingDefOf(k).MaxPerRock; got != wantMax {
			t.Errorf("%s: MaxPerRock = %d, want %d", k, got, wantMax)
		}
	}

	// Function parameters.
	k := BuildingDefOf(Keystone)
	if k.Housing != 12 || k.LifeSupport != 12 || k.Storage != 400 || k.Berths != 4 || k.SensorRange != 260 || k.RevealRadius != 3 {
		t.Errorf("Keystone parameters: %+v", k)
	}
	if k.CommandBuildable() {
		t.Error("the Keystone must not be command-buildable")
	}
	if d := BuildingDefOf(Silo); d.Storage != 300 {
		t.Errorf("Silo storage = %v", d.Storage)
	}
	if d := BuildingDefOf(SurveyMast); d.SurveyRadius != 4 || d.SurveyInterval != 1.5 {
		t.Errorf("Survey Mast: %+v", d)
	}
	if d := BuildingDefOf(Sunvane); !d.SolarPower || !d.IsPowerProducer() {
		t.Errorf("Sunvane: %+v", d)
	}
	if d := BuildingDefOf(ThermalStack); d.Inputs != (Stock{Fuel: 6}) || d.SolarPower || !d.Outputs.IsZero() {
		t.Errorf("Thermal Stack: %+v", d)
	}
	if d := BuildingDefOf(LumenCore); d.Inputs != (Stock{Lumen: 0.5}) {
		t.Errorf("Lumen Core: %+v", d)
	}
	if d := BuildingDefOf(Habitat); d.Housing != 16 {
		t.Errorf("Habitat housing = %v", d.Housing)
	}
	if d := BuildingDefOf(Aerator); d.LifeSupport != 24 {
		t.Errorf("Aerator life support = %v", d.LifeSupport)
	}
	if d := BuildingDefOf(Greenhouse); d.Inputs != (Stock{Ice: 2}) || d.Outputs != (Stock{Food: 14}) {
		t.Errorf("Greenhouse: %+v", d)
	}
	if d := BuildingDefOf(Commons); d.MoraleCover != 40 {
		t.Errorf("Commons cover = %v", d.MoraleCover)
	}
	if d := BuildingDefOf(BoreRig); d.MineRate != 36 {
		t.Errorf("Bore Rig mine rate = %v", d.MineRate)
	}
	if d := BuildingDefOf(Smelter); d.Inputs != (Stock{Ore: 30}) || d.Outputs != (Stock{Alloy: 20}) || !d.Produces() {
		t.Errorf("Smelter: %+v", d)
	}
	if d := BuildingDefOf(Cracker); d.Inputs != (Stock{Ice: 20}) || d.Outputs != (Stock{Fuel: 16}) {
		t.Errorf("Cracker: %+v", d)
	}
	if d := BuildingDefOf(Fabricator); d.Inputs != (Stock{Alloy: 12, Silica: 8}) || d.Outputs != (Stock{Parts: 8}) {
		t.Errorf("Fabricator: %+v", d)
	}
	if d := BuildingDefOf(Laboratory); d.ResearchRate != 6 {
		t.Errorf("Laboratory rate = %v", d.ResearchRate)
	}
	if d := BuildingDefOf(LaunchCradle); d.QueueSize != 5 || MaxShipQueue != 5 {
		t.Errorf("Launch Cradle queue = %d, MaxShipQueue = %d, want 5", d.QueueSize, MaxShipQueue)
	}
	if d := BuildingDefOf(DockRing); d.Berths != 6 || d.DockRepairRate != 2 {
		t.Errorf("Dock Ring: %+v", d)
	}
	if d := BuildingDefOf(ExchangeDock); !d.Trade {
		t.Error("Exchange Dock must enable trade")
	}
	if d := BuildingDefOf(FarseerArray); d.SensorRange != 720 {
		t.Errorf("Farseer range = %v", d.SensorRange)
	}
	if d := BuildingDefOf(MenderHub); d.RepairRadius != 4 || d.RepairRate != 6 || d.RepairAlloyPerHP != 0.02 {
		t.Errorf("Mender Hub: %+v", d)
	}
	flak := BuildingDefOf(FlakBattery)
	if !flak.IsTurret() || flak.TurretDPS != 8 || flak.DPSVs(HullLight) != 12 || flak.DPSVs(HullMedium) != 8 ||
		math.Abs(flak.DPSVs(HullHeavy)-4.8) > 1e-12 || flak.DPSVs(Hull(9)) != 0 {
		t.Errorf("Flak Battery: %+v", flak)
	}
	lance := BuildingDefOf(LanceTower)
	if !lance.IsTurret() || lance.TurretDPS != 20 || lance.DPSVs(HullLight) != 10 || lance.DPSVs(HullMedium) != 20 || lance.DPSVs(HullHeavy) != 30 {
		t.Errorf("Lance Tower: %+v", lance)
	}
	if d := BuildingDefOf(AegisProjector); d.ShieldPool != 600 || d.ShieldRegen != 10 || d.ShieldRadius != 3 {
		t.Errorf("Aegis Projector: %+v", d)
	}
	if d := BuildingDefOf(MassDriver); d.DriverRange != 600 || d.DriverDamage != 150 || d.DriverCooldown != 15 || d.DriverAmmo != 8 {
		t.Errorf("Mass Driver: %+v", d)
	}
	if d := BuildingDefOf(Smelter); d.PowerDraw() != 14 || !d.IsPowerConsumer() || d.IsPowerProducer() || d.IsTurret() {
		t.Errorf("Smelter power helpers are wrong")
	}
	if d := BuildingDefOf(Conduit); d.PowerDraw() != 0 || d.IsPowerConsumer() || d.IsPowerProducer() {
		t.Errorf("Conduit power helpers are wrong")
	}
	for _, kind := range AllBuildingKinds() {
		want := kind == FlakBattery || kind == LanceTower || kind == AegisProjector || kind == MassDriver
		if got := BuildingDefOf(kind).IsDefence(); got != want {
			t.Errorf("%s.IsDefence() = %v, want %v", kind, got, want)
		}
	}

	// Invalid kinds are safe.
	if d := BuildingDefOf(BuildingNone); d.Kind != BuildingNone || d.Unlock != TechNone || d.Name == "" {
		t.Errorf("BuildingDefOf(BuildingNone) = %+v", d)
	}
	if d := BuildingDefOf(BuildingKind(100)); d.Kind != BuildingNone {
		t.Errorf("BuildingDefOf(100) = %+v", d)
	}

	// Palette tabs partition the table.
	total := 0
	for _, c := range AllCategories() {
		kinds := BuildingsIn(c)
		total += len(kinds)
		for _, kind := range kinds {
			if BuildingDefOf(kind).Category != c {
				t.Errorf("BuildingsIn(%s) contains %s", c, kind)
			}
		}
	}
	if total != NumBuildingKinds {
		t.Errorf("palette tabs hold %d kinds, want %d", total, NumBuildingKinds)
	}
}

// TestShipTable re-states the ship table of the specification.
func TestShipTable(t *testing.T) {
	want := []struct {
		kind    ShipKind
		name    string
		hull    Hull
		cost    Stock
		build   float64
		berths  int
		hp      float64
		speed   float64
		dpsS    float64
		dpsB    float64
		fuel    float64
		unlock  TechID
		cargo   float64
		sensor  float64
		armed   bool
		survey  bool
		colony  bool
		bonusVs ShipKind
	}{
		{Prospector, "Prospector", HullLight, Stock{Alloy: 30, Parts: 10}, 25, 1, 60, 9, 0, 0, 0.010, OrbitalFlight, 0, 220, false, true, false, ShipNone},
		{Hauler, "Hauler", HullMedium, Stock{Alloy: 50, Parts: 15}, 35, 1, 120, 5, 0, 0, 0.020, OrbitalFlight, 150, 120, false, false, false, ShipNone},
		{Seedship, "Seedship", HullMedium, Stock{Alloy: 160, Silica: 60, Parts: 60}, 90, 2, 200, 4, 0, 0, 0.040, ColonialCharter, 0, 120, false, false, true, ShipNone},
		{Dart, "Dart", HullLight, Stock{Alloy: 25, Parts: 8}, 20, 1, 50, 10, 6, 1, 0.008, PointDefence, 0, 120, true, false, false, Hammer},
		{Cutter, "Cutter", HullMedium, Stock{Alloy: 60, Parts: 20}, 40, 2, 160, 7, 10, 5, 0.020, Gunnery, 0, 120, true, false, false, ShipNone},
		{Hammer, "Hammer", HullMedium, Stock{Alloy: 70, Parts: 25}, 45, 2, 120, 6, 2, 22, 0.025, SiegeDoctrine, 0, 120, true, false, false, ShipNone},
		{Bulwark, "Bulwark", HullHeavy, Stock{Alloy: 180, Parts: 60, Lumen: 15}, 100, 4, 600, 4, 22, 12, 0.050, CapitalHulls, 0, 120, true, false, false, ShipNone},
	}
	if len(want) != NumShipKinds || len(Ships()) != NumShipKinds {
		t.Fatalf("table sizes: test %d, content %d, want %d", len(want), len(Ships()), NumShipKinds)
	}
	for _, w := range want {
		d := ShipDefOf(w.kind)
		if d.Kind != w.kind || d.Name != w.name || d.Hull != w.hull || d.Cost != w.cost ||
			d.BuildTime != w.build || d.Berths != w.berths || d.MaxHP != w.hp || d.Speed != w.speed ||
			d.DPSShips != w.dpsS || d.DPSBuildings != w.dpsB || d.FuelPerU != w.fuel ||
			d.Unlock != w.unlock || d.Cargo != w.cargo || d.SensorRange != w.sensor ||
			d.Armed() != w.armed || d.CanSurvey != w.survey || d.CanColonize != w.colony ||
			d.BonusVs != w.bonusVs {
			t.Errorf("%s: content %+v does not match the specification row %+v", w.name, d, w)
		}
		if Ships()[w.kind].Name != w.name {
			t.Errorf("Ships()[%d] is %q", w.kind, Ships()[w.kind].Name)
		}
	}
	// Damage bonus: Dart x1.5 against Hammers, nothing else.
	for _, a := range AllShipKinds() {
		for _, b := range AllShipKinds() {
			wantMul := 1.0
			if a == Dart && b == Hammer {
				wantMul = 1.5
			}
			if got := ShipDefOf(a).MulVs(b); got != wantMul {
				t.Errorf("%s vs %s multiplier = %v, want %v", a, b, got, wantMul)
			}
		}
		if got := ShipDefOf(a).MulVs(ShipNone); got != 1 {
			t.Errorf("%s vs none multiplier = %v", a, got)
		}
	}
	// Only the Hammer bombards from the start.
	for _, k := range AllShipKinds() {
		if got := ShipDefOf(k).Bomber(); got != (k == Hammer) {
			t.Errorf("%s.Bomber() = %v", k, got)
		}
	}
	if d := ShipDefOf(ShipNone); d.Kind != ShipNone || d.Unlock != TechNone || d.BonusVs != ShipNone || d.Armed() {
		t.Errorf("ShipDefOf(ShipNone) = %+v", d)
	}
	if DefaultShipSensorRange != 120 {
		t.Errorf("DefaultShipSensorRange = %v", DefaultShipSensorRange)
	}
}

// TestTechTable re-states the technology table of the specification.
func TestTechTable(t *testing.T) {
	want := []struct {
		id        TechID
		name      string
		tier      int
		points    float64
		lumen     float64
		requires  []TechID
		buildings []BuildingKind
		ships     []ShipKind
	}{
		{OrbitalFlight, "Orbital Flight", 1, 60, 0, nil, []BuildingKind{LaunchCradle}, []ShipKind{Prospector, Hauler}},
		{VolatileCracking, "Volatile Cracking", 1, 60, 0, nil, []BuildingKind{Cracker, ThermalStack}, nil},
		{Fabrication, "Fabrication", 1, 70, 0, nil, []BuildingKind{Fabricator}, nil},
		{CivicPlanning, "Civic Planning", 1, 50, 0, nil, []BuildingKind{Commons}, nil},
		{PointDefence, "Point Defence", 1, 90, 0, nil, []BuildingKind{FlakBattery}, []ShipKind{Dart}},
		{DeepBoring, "Deep Boring", 1, 80, 0, nil, nil, nil},
		{CharterTrade, "Charter Trade", 1, 60, 0, nil, []BuildingKind{ExchangeDock}, nil},
		{ColonialCharter, "Colonial Charter", 2, 150, 0, []TechID{OrbitalFlight, Fabrication}, nil, []ShipKind{Seedship}},
		{Gunnery, "Gunnery", 2, 160, 0, []TechID{PointDefence}, nil, []ShipKind{Cutter}},
		{SiegeDoctrine, "Siege Doctrine", 2, 200, 0, []TechID{Gunnery}, nil, []ShipKind{Hammer}},
		{DeepScan, "Deep Scan", 2, 140, 0, []TechID{OrbitalFlight}, []BuildingKind{FarseerArray}, nil},
		{FieldRepair, "Field Repair", 2, 160, 0, []TechID{Fabrication}, []BuildingKind{MenderHub}, nil},
		{Photovoltaics, "Tuned Photovoltaics", 2, 150, 0, nil, nil, nil},
		{HydroStacks, "Hydroponic Stacks", 2, 130, 0, []TechID{CivicPlanning}, nil, nil},
		{DockEngineering, "Dock Engineering", 2, 160, 0, []TechID{OrbitalFlight}, []BuildingKind{DockRing}, nil},
		{EfficientSmelting, "Efficient Smelting", 2, 170, 0, []TechID{DeepBoring}, nil, nil},
		{MassAcceleration, "Mass Acceleration", 2, 260, 0, []TechID{Fabrication, PointDefence}, []BuildingKind{MassDriver}, nil},
		{LumenOptics, "Lumen Optics", 3, 320, 20, []TechID{PointDefence, DeepScan}, []BuildingKind{LanceTower}, nil},
		{AegisFields, "Aegis Fields", 3, 380, 30, []TechID{LumenOptics}, []BuildingKind{AegisProjector}, nil},
		{LumenFusion, "Lumen Fusion", 3, 420, 40, []TechID{VolatileCracking, Photovoltaics}, []BuildingKind{LumenCore}, nil},
		{CapitalHulls, "Capital Hulls", 3, 450, 30, []TechID{SiegeDoctrine, DockEngineering}, nil, []ShipKind{Bulwark}},
		{ReactivePlating, "Reactive Plating", 3, 350, 0, []TechID{Gunnery, FieldRepair}, nil, nil},
		{OverdriveThrusters, "Overdrive Thrusters", 3, 300, 0, []TechID{DockEngineering}, nil, nil},
		{Automation, "Automation", 3, 400, 0, []TechID{Fabrication, EfficientSmelting}, nil, nil},
	}
	if len(want) != NumTechs || len(Techs()) != NumTechs {
		t.Fatalf("table sizes: test %d, content %d, want %d", len(want), len(Techs()), NumTechs)
	}
	for _, w := range want {
		d := TechDefOf(w.id)
		if d.ID != w.id || d.Name != w.name || d.Tier != w.tier || d.Cost != w.points || d.Lumen != w.lumen ||
			!slices.Equal(d.Requires, w.requires) || !slices.Equal(d.UnlockBuildings, w.buildings) ||
			!slices.Equal(d.UnlockShips, w.ships) {
			t.Errorf("%s: content %+v does not match the specification row %+v", w.name, d, w)
		}
	}
	if d := TechDefOf(TechNone); d.ID != TechNone || d.Name == "" {
		t.Errorf("TechDefOf(TechNone) = %+v", d)
	}

	// Passive effects.
	effects := map[TechID]TechEffect{
		DeepBoring:         {MiningMul: 1.25},
		DeepScan:           {SurveyBonus: 2},
		Photovoltaics:      {SolarMul: 1.30},
		HydroStacks:        {FoodMul: 1.35},
		EfficientSmelting:  {SmeltMul: 1.25},
		ReactivePlating:    {HPMul: 1.20},
		OverdriveThrusters: {ShipSpeedMul: 1.25},
		Automation:         {StaffMul: 0.75},
	}
	for _, id := range AllTechs() {
		if got, want := TechDefOf(id).Effect, effects[id]; got != want {
			t.Errorf("%s: effect %+v, want %+v", id, got, want)
		}
	}
}

func TestComputeMods(t *testing.T) {
	var none [NumTechs]bool
	if got := ComputeMods(none, false, DiffHard); got != DefaultMods() {
		t.Errorf("no techs, human: %+v, want the defaults", got)
	}
	// Difficulty handicap applies to AI factions only.
	for _, tc := range []struct {
		d    Difficulty
		ai   bool
		want float64
	}{
		{DiffEasy, true, 0.8}, {DiffNormal, true, 1.0}, {DiffHard, true, 1.25},
		{DiffEasy, false, 1.0}, {DiffHard, false, 1.0}, {Difficulty(9), true, 1.0},
	} {
		if got := ComputeMods(none, tc.ai, tc.d).ProdMul; got != tc.want {
			t.Errorf("ProdMul(%s, ai=%v) = %v, want %v", tc.d, tc.ai, got, tc.want)
		}
	}
	// Each effect tech moves exactly its own field.
	single := []struct {
		tech TechID
		want func(m *Mods)
	}{
		{DeepBoring, func(m *Mods) { m.MiningMul = 1.25 }},
		{Photovoltaics, func(m *Mods) { m.SolarMul = 1.30 }},
		{HydroStacks, func(m *Mods) { m.FoodMul = 1.35 }},
		{EfficientSmelting, func(m *Mods) { m.SmeltMul = 1.25 }},
		{ReactivePlating, func(m *Mods) { m.HPMul = 1.20 }},
		{OverdriveThrusters, func(m *Mods) { m.ShipSpeedMul = 1.25 }},
		{Automation, func(m *Mods) { m.StaffMul = 0.75 }},
		{DeepScan, func(m *Mods) { m.SurveyBonus = 2 }},
		{OrbitalFlight, func(m *Mods) {}},
	}
	for _, tc := range single {
		var techs [NumTechs]bool
		techs[tc.tech] = true
		want := DefaultMods()
		tc.want(&want)
		if got := ComputeMods(techs, false, DiffNormal); got != want {
			t.Errorf("%s alone: %+v, want %+v", tc.tech, got, want)
		}
	}
	// Everything at once.
	var all [NumTechs]bool
	for i := range all {
		all[i] = true
	}
	want := Mods{
		MiningMul: 1.25, SolarMul: 1.30, FoodMul: 1.35, SmeltMul: 1.25, HPMul: 1.20,
		ShipSpeedMul: 1.25, StaffMul: 0.75, ResearchMul: 1, SurveyBonus: 2, ProdMul: 1.25,
	}
	if got := ComputeMods(all, true, DiffHard); got != want {
		t.Errorf("all techs: %+v, want %+v", got, want)
	}

	// Faction.RecomputeMods uses the same function.
	f := NewFaction(2, "Rival", PersonalityVane)
	f.Techs[DeepBoring] = true
	f.RecomputeMods(DiffEasy)
	if f.Mods.MiningMul != 1.25 || f.Mods.ProdMul != 0.8 {
		t.Errorf("RecomputeMods: %+v", f.Mods)
	}
}

func TestModsHelpers(t *testing.T) {
	def := DefaultMods()
	auto := def
	auto.StaffMul = 0.75
	tests := []struct {
		staff     int
		def, auto int
	}{
		{0, 0, 0}, {1, 1, 1}, {2, 2, 2}, {3, 3, 3}, {4, 4, 3}, {8, 8, 6}, {-2, 0, 0},
	}
	for _, tc := range tests {
		if got := def.StaffNeeded(tc.staff); got != tc.def {
			t.Errorf("default StaffNeeded(%d) = %d, want %d", tc.staff, got, tc.def)
		}
		if got := auto.StaffNeeded(tc.staff); got != tc.auto {
			t.Errorf("automated StaffNeeded(%d) = %d, want %d", tc.staff, got, tc.auto)
		}
	}
	// A zero Mods must not zero everything out.
	var zero Mods
	if zero.StaffNeeded(3) != 3 || zero.MaxHP(100) != 100 || zero.OutputMul(BoreRig) != 1 {
		t.Error("helpers must treat a zero Mods as neutral")
	}
	plated := def
	plated.HPMul = 1.2
	if got := plated.MaxHP(600); math.Abs(got-720) > 1e-9 {
		t.Errorf("MaxHP = %v, want 720", got)
	}

	m := Mods{MiningMul: 1.25, SolarMul: 1.3, FoodMul: 1.35, SmeltMul: 1.25, HPMul: 1, ShipSpeedMul: 1, StaffMul: 1, ResearchMul: 2, ProdMul: 0.8}
	out := map[BuildingKind]float64{
		BoreRig:    1.25 * 0.8,
		Greenhouse: 1.35 * 0.8,
		Smelter:    1.25 * 0.8,
		Cracker:    0.8,
		Fabricator: 0.8,
		Sunvane:    1.3,
	}
	for _, k := range AllBuildingKinds() {
		want, ok := out[k]
		if !ok {
			want = 1
		}
		if got := m.OutputMul(k); math.Abs(got-want) > 1e-12 {
			t.Errorf("OutputMul(%s) = %v, want %v", k, got, want)
		}
	}
}

func TestScalarRules(t *testing.T) {
	if TicksPerSecond != 10 || TicksPerCycle != 600 || SecondsPerCycle != 60 {
		t.Error("time constants are wrong")
	}
	if TickDT != 0.1 || SlowDT != 1.0 || SlowTickInterval != 10 || AIInterval != 20 || AIStagger != 5 {
		t.Error("cadence constants are wrong")
	}
	if SecondsToTicks(15) != 150 || SecondsToTicks(0.04) != 0 || SecondsToTicks(0.06) != 1 {
		t.Error("SecondsToTicks is wrong")
	}
	if CyclesToTicks(2) != 1200 || CyclesToTicks(0.5) != 300 {
		t.Error("CyclesToTicks is wrong")
	}
	if CycleOfTick(0) != 1 || CycleOfTick(599) != 1 || CycleOfTick(600) != 2 || CycleOfTick(1800) != 4 {
		t.Error("CycleOfTick is wrong")
	}
	if got := PerCycle(36, 1); math.Abs(got-0.6) > 1e-12 {
		t.Errorf("PerCycle(36, 1) = %v, want 0.6", got)
	}

	// Morale factor thresholds.
	for _, tc := range []struct{ morale, want float64 }{
		{100, 1.10}, {70, 1.10}, {69.99, 1.0}, {55, 1.0}, {30, 1.0}, {29.99, 0.75}, {0, 0.75},
	} {
		if got := MoraleFactor(tc.morale); got != tc.want {
			t.Errorf("MoraleFactor(%v) = %v, want %v", tc.morale, got, tc.want)
		}
	}
	if MoraleAttackWindowTicks != 1200 || SelfRepairQuietTicks != 600 || SponsorCooldownTicks != 600 || SalvageLifetimeTicks != 2400 {
		t.Error("tick windows are wrong")
	}

	// Starting and Seedship stock.
	if got, want := StartStock(), (Stock{Alloy: 160, Silica: 130, Ice: 90, Fuel: 30, Food: 110, Parts: 20}); got != want {
		t.Errorf("StartStock() = %v", got)
	}
	if StartStock()[Ore] != 0 || StartStock()[Lumen] != 0 {
		t.Error("starting colonies must have no Ore and no Lumen")
	}
	if got, want := SeedshipStock(), (Stock{Alloy: 80, Silica: 60, Food: 50, Ice: 40}); got != want {
		t.Errorf("SeedshipStock() = %v", got)
	}
	if StartPop != 14 || StartMorale != 60 || StartScrip != 400 || SeedshipColonists != 12 || SeedshipMinPop != 20 {
		t.Error("setup constants are wrong")
	}
	if RefundGoods() != [3]Good{Alloy, Silica, Parts} {
		t.Error("RefundGoods is wrong")
	}

	// Expedite: ceil(1.5 x remaining x (1 + alloy/100)).
	for _, tc := range []struct{ remaining, alloy, want float64 }{
		{20, 35, 41},    // 1.5*20*1.35 = 40.5
		{10, 0, 15},     // 1.5*10
		{60, 120, 198},  // 1.5*60*2.2
		{0, 50, 0},      // nothing left
		{-3, 50, 0},     // nothing left
		{0.1, 100, 1},   // 0.3 -> 1
		{45, 60, 108},   // 1.5*45*1.6
		{3, 4, 5},       // 1.5*3*1.04 = 4.68
		{30, 150, 113},  // 1.5*30*2.5 = 112.5
		{100, 180, 420}, // 1.5*100*2.8
	} {
		if got := ExpediteCost(tc.remaining, tc.alloy); got != tc.want {
			t.Errorf("ExpediteCost(%v, %v) = %v, want %v", tc.remaining, tc.alloy, got, tc.want)
		}
	}

	// Retreat levels.
	if RetreatLevels() != [3]float64{0, 0.25, 0.5} {
		t.Error("RetreatLevels is wrong")
	}
	for _, v := range []float64{0, 0.25, 0.5} {
		if !ValidRetreat(v) {
			t.Errorf("ValidRetreat(%v) = false", v)
		}
	}
	for _, v := range []float64{0.1, 0.75, 1, -0.25} {
		if ValidRetreat(v) {
			t.Errorf("ValidRetreat(%v) = true", v)
		}
	}

	// Setup helpers.
	if BeltSmall.RockCount() != 10 || BeltMedium.RockCount() != 13 || BeltLarge.RockCount() != 16 || BeltSize(7).RockCount() != 13 {
		t.Error("BeltSize.RockCount is wrong")
	}
	if DiffEasy.ProdMul() != 0.8 || DiffNormal.ProdMul() != 1.0 || DiffHard.ProdMul() != 1.25 {
		t.Error("Difficulty.ProdMul is wrong")
	}
	if DefaultPlayerName != "Free Charter of Ansel" {
		t.Errorf("DefaultPlayerName = %q", DefaultPlayerName)
	}
}

func TestGameSettings(t *testing.T) {
	d := DefaultSettings(77)
	if d.Seed != 77 || d.BeltSize != BeltMedium || d.Rivals != 2 || d.Difficulty != DiffNormal ||
		d.PlayerName != DefaultPlayerName || !d.Events || !d.Directives || d.RivalKinds != nil {
		t.Errorf("DefaultSettings = %+v", d)
	}
	if n := d.Normalized(); n.Seed != d.Seed || n.Rivals != d.Rivals || n.PlayerName != d.PlayerName || len(n.RivalKinds) != 0 {
		t.Errorf("normalising valid settings changed them: %+v", n)
	}

	messy := GameSettings{
		BeltSize: BeltSize(9), Rivals: 12, Difficulty: Difficulty(-3),
		RivalKinds: []Personality{PersonalityVane, Personality(40), PersonalityMeridian, PersonalityOstrander},
	}
	n := messy.Normalized()
	if n.BeltSize != BeltMedium || n.Difficulty != DiffNormal || n.Rivals != MaxRivals || n.PlayerName != DefaultPlayerName {
		t.Errorf("Normalized = %+v", n)
	}
	if want := []Personality{PersonalityVane, PersonalityNone, PersonalityMeridian}; !slices.Equal(n.RivalKinds, want) {
		t.Errorf("Normalized RivalKinds = %v, want %v", n.RivalKinds, want)
	}
	if len(messy.RivalKinds) != 4 || messy.RivalKinds[1] != Personality(40) {
		t.Error("Normalized modified its receiver's slice")
	}
	if got := (GameSettings{Rivals: -4}).Normalized().Rivals; got != MinRivals {
		t.Errorf("Rivals clamped to %d, want %d", got, MinRivals)
	}
}
