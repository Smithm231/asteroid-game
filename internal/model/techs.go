package model

import "math"

// TechEffect is the passive modifier a technology grants in addition to its
// unlocks. Multipliers are factors applied to the matching Mods field; a
// value of 0 means "no change" (so the zero TechEffect does nothing).
type TechEffect struct {
	// MiningMul multiplies Mods.MiningMul (Bore Rig output).
	MiningMul float64
	// SolarMul multiplies Mods.SolarMul (Sunvane output).
	SolarMul float64
	// FoodMul multiplies Mods.FoodMul (Greenhouse output).
	FoodMul float64
	// SmeltMul multiplies Mods.SmeltMul (Smelter output, same input).
	SmeltMul float64
	// HPMul multiplies Mods.HPMul (ship and building max HP).
	HPMul float64
	// ShipSpeedMul multiplies Mods.ShipSpeedMul (fleet speed).
	ShipSpeedMul float64
	// StaffMul multiplies Mods.StaffMul (workers needed per building).
	StaffMul float64
	// ResearchMul multiplies Mods.ResearchMul (research points).
	ResearchMul float64
	// SurveyBonus is added to Mods.SurveyBonus (Survey Mast radius in hexes).
	SurveyBonus int
}

// IsZero reports whether the effect changes nothing.
func (e TechEffect) IsZero() bool { return e == TechEffect{} }

// TechDef is the content-table entry of a technology.
type TechDef struct {
	// ID is the technology this entry describes.
	ID TechID
	// Name is the display name, e.g. "Tuned Photovoltaics".
	Name string
	// Tier is the tier, 1..3.
	Tier int
	// Cost is the number of research points needed.
	Cost float64
	// Lumen is the Lumen paid once, when research on the tech starts (0 for
	// most techs).
	Lumen float64
	// Requires lists the techs that must be known first. Read-only.
	Requires []TechID
	// UnlockBuildings lists the building kinds the tech unlocks. Read-only.
	UnlockBuildings []BuildingKind
	// UnlockShips lists the ship kinds the tech unlocks. Read-only.
	UnlockShips []ShipKind
	// Effect is the passive modifier the tech grants (zero for pure unlock
	// techs).
	Effect TechEffect
	// EffectText is a one-line summary of everything the tech grants.
	EffectText string
	// Description is the codex text: one or two sentences.
	Description string
}

// techDefs is the technology table, indexed by TechID.
var techDefs = [NumTechs]TechDef{
	OrbitalFlight: {
		ID: OrbitalFlight, Name: "Orbital Flight", Tier: 1, Cost: 60,
		UnlockBuildings: []BuildingKind{LaunchCradle},
		UnlockShips:     []ShipKind{Prospector, Hauler},
		EffectText:      "Unlocks the Launch Cradle, the Prospector and the Hauler.",
		Description:     "Catapult launch and orbital rendezvous: the first step off the rock.",
	},
	VolatileCracking: {
		ID: VolatileCracking, Name: "Volatile Cracking", Tier: 1, Cost: 60,
		UnlockBuildings: []BuildingKind{Cracker, ThermalStack},
		EffectText:      "Unlocks the Cracker and the Thermal Stack.",
		Description:     "Splitting Ice into storable Fuel, and burning that Fuel for power.",
	},
	Fabrication: {
		ID: Fabrication, Name: "Fabrication", Tier: 1, Cost: 70,
		UnlockBuildings: []BuildingKind{Fabricator},
		EffectText:      "Unlocks the Fabricator.",
		Description:     "Precision manufacture of machine Parts from Alloy and Silica.",
	},
	CivicPlanning: {
		ID: CivicPlanning, Name: "Civic Planning", Tier: 1, Cost: 50,
		UnlockBuildings: []BuildingKind{Commons},
		EffectText:      "Unlocks the Commons.",
		Description:     "Shared spaces and fair shift rotas that keep a crew content.",
	},
	PointDefence: {
		ID: PointDefence, Name: "Point Defence", Tier: 1, Cost: 90,
		UnlockBuildings: []BuildingKind{FlakBattery},
		UnlockShips:     []ShipKind{Dart},
		EffectText:      "Unlocks the Flak Battery and the Dart.",
		Description:     "Rapid-fire guns, for the surface and for small craft.",
	},
	DeepBoring: {
		ID: DeepBoring, Name: "Deep Boring", Tier: 1, Cost: 80,
		Effect:      TechEffect{MiningMul: 1.25},
		EffectText:  "Bore Rig output +25%.",
		Description: "Longer drill strings and harder bits reach richer rock.",
	},
	CharterTrade: {
		ID: CharterTrade, Name: "Charter Trade", Tier: 1, Cost: 60,
		UnlockBuildings: []BuildingKind{ExchangeDock},
		EffectText:      "Unlocks the Exchange Dock.",
		Description:     "A trading licence with the Exchange, and the paperwork to use it.",
	},
	ColonialCharter: {
		ID: ColonialCharter, Name: "Colonial Charter", Tier: 2, Cost: 150,
		Requires:    []TechID{OrbitalFlight, Fabrication},
		UnlockShips: []ShipKind{Seedship},
		EffectText:  "Unlocks the Seedship.",
		Description: "The legal and technical means to plant a second Keystone.",
	},
	Gunnery: {
		ID: Gunnery, Name: "Gunnery", Tier: 2, Cost: 160,
		Requires:    []TechID{PointDefence},
		UnlockShips: []ShipKind{Cutter},
		EffectText:  "Unlocks the Cutter.",
		Description: "Stabilised mounts and fire control for ship-to-ship guns.",
	},
	SiegeDoctrine: {
		ID: SiegeDoctrine, Name: "Siege Doctrine", Tier: 2, Cost: 200,
		Requires:    []TechID{Gunnery},
		UnlockShips: []ShipKind{Hammer},
		EffectText:  "Unlocks the Hammer.",
		Description: "Ordnance and tactics for breaking surface structures from orbit.",
	},
	DeepScan: {
		ID: DeepScan, Name: "Deep Scan", Tier: 2, Cost: 140,
		Requires:        []TechID{OrbitalFlight},
		UnlockBuildings: []BuildingKind{FarseerArray},
		Effect:          TechEffect{SurveyBonus: 2},
		EffectText:      "Unlocks the Farseer Array; Survey Mast range +2.",
		Description:     "Wide-baseline sensing, for deep space and deep rock alike.",
	},
	FieldRepair: {
		ID: FieldRepair, Name: "Field Repair", Tier: 2, Cost: 160,
		Requires:        []TechID{Fabrication},
		UnlockBuildings: []BuildingKind{MenderHub},
		EffectText:      "Unlocks the Mender Hub.",
		Description:     "Autonomous repair drones that keep working while the guns are still firing.",
	},
	Photovoltaics: {
		ID: Photovoltaics, Name: "Tuned Photovoltaics", Tier: 2, Cost: 150,
		Effect:      TechEffect{SolarMul: 1.30},
		EffectText:  "Sunvane output +30%.",
		Description: "Vane coatings tuned to Emberfall's red spectrum.",
	},
	HydroStacks: {
		ID: HydroStacks, Name: "Hydroponic Stacks", Tier: 2, Cost: 130,
		Requires:    []TechID{CivicPlanning},
		Effect:      TechEffect{FoodMul: 1.35},
		EffectText:  "Greenhouse output +35%.",
		Description: "Vertical racks that grow more under the same pressure hull.",
	},
	DockEngineering: {
		ID: DockEngineering, Name: "Dock Engineering", Tier: 2, Cost: 160,
		Requires:        []TechID{OrbitalFlight},
		UnlockBuildings: []BuildingKind{DockRing},
		EffectText:      "Unlocks the Dock Ring.",
		Description:     "Tethered orbital moorings for larger fleets.",
	},
	EfficientSmelting: {
		ID: EfficientSmelting, Name: "Efficient Smelting", Tier: 2, Cost: 170,
		Requires:    []TechID{DeepBoring},
		Effect:      TechEffect{SmeltMul: 1.25},
		EffectText:  "Smelter output +25% from the same Ore.",
		Description: "Hotter arcs and better slag recovery.",
	},
	MassAcceleration: {
		ID: MassAcceleration, Name: "Mass Acceleration", Tier: 2, Cost: 260,
		Requires:        []TechID{Fabrication, PointDefence},
		UnlockBuildings: []BuildingKind{MassDriver},
		EffectText:      "Unlocks the Mass Driver.",
		Description:     "Rail launch scaled up from cargo to bombardment.",
	},
	LumenOptics: {
		ID: LumenOptics, Name: "Lumen Optics", Tier: 3, Cost: 320, Lumen: 20,
		Requires:        []TechID{PointDefence, DeepScan},
		UnlockBuildings: []BuildingKind{LanceTower},
		EffectText:      "Unlocks the Lance Tower.",
		Description:     "Lumen crystal lenses that hold a beam together all the way to orbit.",
	},
	AegisFields: {
		ID: AegisFields, Name: "Aegis Fields", Tier: 3, Cost: 380, Lumen: 30,
		Requires:        []TechID{LumenOptics},
		UnlockBuildings: []BuildingKind{AegisProjector},
		EffectText:      "Unlocks the Aegis Projector.",
		Description:     "Lumen-driven fields that soak up incoming fire.",
	},
	LumenFusion: {
		ID: LumenFusion, Name: "Lumen Fusion", Tier: 3, Cost: 420, Lumen: 40,
		Requires:        []TechID{VolatileCracking, Photovoltaics},
		UnlockBuildings: []BuildingKind{LumenCore},
		EffectText:      "Unlocks the Lumen Core.",
		Description:     "Sustained fusion inside a Lumen lattice.",
	},
	CapitalHulls: {
		ID: CapitalHulls, Name: "Capital Hulls", Tier: 3, Cost: 450, Lumen: 30,
		Requires:    []TechID{SiegeDoctrine, DockEngineering},
		UnlockShips: []ShipKind{Bulwark},
		EffectText:  "Unlocks the Bulwark.",
		Description: "Keel frames large enough for a true warship.",
	},
	ReactivePlating: {
		ID: ReactivePlating, Name: "Reactive Plating", Tier: 3, Cost: 350,
		Requires:    []TechID{Gunnery, FieldRepair},
		Effect:      TechEffect{HPMul: 1.20},
		EffectText:  "Ship and building max HP +20%.",
		Description: "Layered plate that sheds part of every hit.",
	},
	OverdriveThrusters: {
		ID: OverdriveThrusters, Name: "Overdrive Thrusters", Tier: 3, Cost: 300,
		Requires:    []TechID{DockEngineering},
		Effect:      TechEffect{ShipSpeedMul: 1.25},
		EffectText:  "Ship speed +25%.",
		Description: "Drives that run hotter, for longer.",
	},
	Automation: {
		ID: Automation, Name: "Automation", Tier: 3, Cost: 400,
		Requires:    []TechID{Fabrication, EfficientSmelting},
		Effect:      TechEffect{StaffMul: 0.75},
		EffectText:  "Staff needs -25%.",
		Description: "Machines that mind the machines.",
	},
}

// Techs returns the technology table in TechID order. The slice and the
// slices inside its entries are shared and read-only: do not modify them.
func Techs() []TechDef { return techDefs[:] }

// TechDefOf returns the table entry of id. For an invalid id it returns an
// entry with ID == TechNone and Name "Unknown". The slices inside the entry
// are shared and read-only.
func TechDefOf(id TechID) TechDef {
	if !id.Valid() {
		return TechDef{ID: TechNone, Name: "Unknown"}
	}
	return techDefs[id]
}

// Mods are a faction's global modifiers. They are a pure function of the
// faction's known technologies and of the difficulty (see ComputeMods) and
// are cached in Faction.Mods. All multipliers are 1 when nothing applies;
// the zero Mods is NOT neutral, use DefaultMods.
type Mods struct {
	// MiningMul multiplies Bore Rig output (Deep Boring).
	MiningMul float64
	// SolarMul multiplies Sunvane output (Tuned Photovoltaics).
	SolarMul float64
	// FoodMul multiplies Greenhouse output (Hydroponic Stacks).
	FoodMul float64
	// SmeltMul multiplies Smelter output for the same input (Efficient
	// Smelting).
	SmeltMul float64
	// HPMul multiplies the max HP of ships and buildings (Reactive Plating).
	HPMul float64
	// ShipSpeedMul multiplies fleet speed (Overdrive Thrusters).
	ShipSpeedMul float64
	// StaffMul multiplies the workers each building needs (Automation).
	StaffMul float64
	// ResearchMul multiplies research points.
	ResearchMul float64
	// SurveyBonus is added to the Survey Mast radius, in hexes (Deep Scan).
	SurveyBonus int
	// ProdMul is the AI difficulty handicap applied to mining and to all
	// goods production: Easy 0.8, Normal 1.0, Hard 1.25; always 1.0 for
	// human factions.
	ProdMul float64
}

// DefaultMods returns the neutral modifiers: every multiplier 1, no survey
// bonus.
func DefaultMods() Mods {
	return Mods{
		MiningMul: 1, SolarMul: 1, FoodMul: 1, SmeltMul: 1,
		HPMul: 1, ShipSpeedMul: 1, StaffMul: 1, ResearchMul: 1,
		SurveyBonus: 0, ProdMul: 1,
	}
}

// mulOr1 returns x, or 1 when x is not positive. It makes helpers tolerant
// of a zero Mods value.
func mulOr1(x float64) float64 {
	if x > 0 {
		return x
	}
	return 1
}

// applyFactor multiplies *dst by f when f is a real factor (non-zero).
func applyFactor(dst *float64, f float64) {
	if f != 0 {
		*dst *= f
	}
}

// ComputeMods returns the modifiers of a faction that knows the given techs.
// It starts from DefaultMods, applies the Effect of every known tech and sets
// ProdMul to d.ProdMul() for AI factions (1 for humans). It is the single
// source of truth for Faction.Mods.
func ComputeMods(techs [NumTechs]bool, isAI bool, d Difficulty) Mods {
	m := DefaultMods()
	for t, known := range techs {
		if !known {
			continue
		}
		e := techDefs[t].Effect
		applyFactor(&m.MiningMul, e.MiningMul)
		applyFactor(&m.SolarMul, e.SolarMul)
		applyFactor(&m.FoodMul, e.FoodMul)
		applyFactor(&m.SmeltMul, e.SmeltMul)
		applyFactor(&m.HPMul, e.HPMul)
		applyFactor(&m.ShipSpeedMul, e.ShipSpeedMul)
		applyFactor(&m.StaffMul, e.StaffMul)
		applyFactor(&m.ResearchMul, e.ResearchMul)
		m.SurveyBonus += e.SurveyBonus
	}
	if isAI {
		m.ProdMul = d.ProdMul()
	}
	return m
}

// StaffNeeded returns the workers a building with base staff needs under
// these modifiers: ceil(staff x StaffMul), and 0 stays 0.
func (m Mods) StaffNeeded(staff int) int {
	if staff <= 0 {
		return 0
	}
	return int(math.Ceil(float64(staff)*mulOr1(m.StaffMul) - StockEpsilon))
}

// MaxHP returns base scaled by HPMul: the max HP of a ship or building of
// this faction.
func (m Mods) MaxHP(base float64) float64 { return base * mulOr1(m.HPMul) }

// OutputMul returns the multiplier these modifiers apply to the output of a
// building kind (inputs are never scaled by it):
//
//   - Bore Rig: MiningMul x ProdMul (applies to MineRate)
//   - Greenhouse: FoodMul x ProdMul
//   - Smelter: SmeltMul x ProdMul
//   - Cracker, Fabricator: ProdMul
//   - Sunvane: SolarMul (applies to Power; the rock's Solar factor and the
//     flare bonus are separate)
//   - everything else: 1 (research points use ResearchMul, applied per
//     faction by the research system)
func (m Mods) OutputMul(kind BuildingKind) float64 {
	switch kind {
	case BoreRig:
		return mulOr1(m.MiningMul) * mulOr1(m.ProdMul)
	case Greenhouse:
		return mulOr1(m.FoodMul) * mulOr1(m.ProdMul)
	case Smelter:
		return mulOr1(m.SmeltMul) * mulOr1(m.ProdMul)
	case Cracker, Fabricator:
		return mulOr1(m.ProdMul)
	case Sunvane:
		return mulOr1(m.SolarMul)
	default:
		return 1
	}
}
