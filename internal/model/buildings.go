package model

// BuildingDef is the content-table entry of a building kind. The first group
// of fields is common to every kind; the remaining "function parameters" are
// zero unless the kind has that function.
//
// Rates are per cycle and are scaled by the systems with the building's
// efficiency and the owner's Mods; times are in game-seconds; ranges on a
// rock surface are in hexes and ranges in space are in u.
type BuildingDef struct {
	// Kind is the building kind this entry describes.
	Kind BuildingKind
	// Name is the display name, e.g. "Bore Rig".
	Name string
	// Category is the palette tab and the staffing / power rank.
	Category Category
	// Cost is deducted from the colony stock on placement.
	Cost Stock
	// BuildTime is the construction time in game-seconds (0 for the
	// Keystone, which is never constructed).
	BuildTime float64
	// Power is the power produced (positive) or consumed (negative). For a
	// Sunvane (SolarPower) this is the output at solar factor 1.
	Power float64
	// SolarPower is true when Power is multiplied by the rock's Solar
	// factor, Mods.SolarMul and the flare bonus (Sunvane).
	SolarPower bool
	// Staff is the number of workers needed before Mods.StaffMul.
	Staff int
	// MaxHP is the hit points of a completed building before Mods.HPMul.
	MaxHP float64
	// Unlock is the technology that unlocks the kind, or TechNone when it is
	// StartUnlocked or can never be built by command (Keystone).
	Unlock TechID
	// StartUnlocked is true for the kinds every faction can build from the
	// start.
	StartUnlocked bool
	// MaxPerRock is the per-rock limit of this kind, or 0 for unlimited.
	MaxPerRock int

	// Housing is the number of colonists housed.
	Housing float64
	// LifeSupport is the number of colonists supplied with air. The
	// Keystone's is constant; an Aerator's is scaled by its efficiency.
	LifeSupport float64
	// Storage is the capacity added for every good.
	Storage float64
	// Berths is the number of ship berths provided.
	Berths int
	// SensorRange is the radius in u within which enemy fleets are seen.
	SensorRange float64
	// RevealRadius is the hex radius around the building within which
	// deposits are revealed when it is founded (Keystone).
	RevealRadius int

	// Inputs are the goods consumed per cycle at full efficiency. For
	// Thermal Stack and Lumen Core this is the fuel burned to make power.
	Inputs Stock
	// Outputs are the goods produced per cycle at full efficiency.
	Outputs Stock
	// MineRate is the total number of deposit units mined per cycle from the
	// building's own tile and its six neighbours (Bore Rig).
	MineRate float64
	// ResearchRate is the research points generated per cycle (Laboratory).
	ResearchRate float64
	// MoraleCover is the number of colonists whose morale one building
	// covers at full efficiency (Commons).
	MoraleCover float64

	// TurretDPS is the damage per second against ships in orbit before the
	// hull multiplier (Flak Battery, Lance Tower).
	TurretDPS float64
	// HullMul is the turret damage multiplier per target hull class; all
	// zero for non-turrets.
	HullMul [NumHulls]float64

	// ShieldPool is the maximum shield pool (Aegis Projector).
	ShieldPool float64
	// ShieldRegen is the pool regeneration per game-second at full
	// efficiency.
	ShieldRegen float64
	// ShieldRadius is the hex radius within which friendly buildings are
	// shielded (the projector itself included).
	ShieldRadius int

	// RepairRadius is the hex radius within which friendly buildings are
	// repaired (Mender Hub).
	RepairRadius int
	// RepairRate is the HP per game-second restored to each damaged building
	// in range at full efficiency.
	RepairRate float64
	// RepairAlloyPerHP is the Alloy consumed per HP repaired.
	RepairAlloyPerHP float64
	// DockRepairRate is the HP per game-second restored to each friendly
	// ship in orbit at full efficiency (Dock Ring).
	DockRepairRate float64

	// SurveyRadius is the hex radius within which hidden tiles are revealed
	// before Mods.SurveyBonus (Survey Mast).
	SurveyRadius int
	// SurveyInterval is the time in game-seconds, at full efficiency, to
	// reveal one tile.
	SurveyInterval float64

	// DriverRange is the maximum distance in u to the target rock (Mass
	// Driver).
	DriverRange float64
	// DriverDamage is the damage one slug deals to one building.
	DriverDamage float64
	// DriverCooldown is the time in game-seconds between two slugs.
	DriverCooldown float64
	// DriverAmmo is the Alloy consumed per slug.
	DriverAmmo float64

	// QueueSize is the maximum length of the ship build queue (Launch
	// Cradle).
	QueueSize int
	// Trade is true when the building enables market trading and settler
	// sponsorship at its rock (Exchange Dock).
	Trade bool

	// Role is a one-line functional summary for tooltips.
	Role string
	// Description is the codex text: one or two sentences.
	Description string
}

// IsPowerProducer reports whether the kind generates power.
func (d BuildingDef) IsPowerProducer() bool { return d.Power > 0 }

// IsPowerConsumer reports whether the kind draws power.
func (d BuildingDef) IsPowerConsumer() bool { return d.Power < 0 }

// PowerDraw returns the power the kind consumes as a positive number, or 0
// for producers and neutral kinds.
func (d BuildingDef) PowerDraw() float64 {
	if d.Power < 0 {
		return -d.Power
	}
	return 0
}

// IsTurret reports whether the kind shoots at ships in orbit.
func (d BuildingDef) IsTurret() bool { return d.TurretDPS > 0 }

// IsDefence reports whether the kind is one of the "defences" of the
// bombardment target orders: Flak Battery, Lance Tower, Aegis Projector and
// Mass Driver, i.e. every kind with a turret, a shield or a driver. The
// Mender Hub sits in the Defence category but is not a defence in this
// sense.
func (d BuildingDef) IsDefence() bool {
	return d.TurretDPS > 0 || d.ShieldPool > 0 || d.DriverRange > 0
}

// Produces reports whether the kind turns inputs into output goods
// (Greenhouse, Smelter, Cracker, Fabricator).
func (d BuildingDef) Produces() bool { return !d.Outputs.IsZero() }

// DPSVs returns the turret's damage per second against the given hull class
// (TurretDPS x HullMul), or 0 for non-turrets and unknown hulls.
func (d BuildingDef) DPSVs(h Hull) float64 {
	if !h.Valid() {
		return 0
	}
	return d.TurretDPS * d.HullMul[h]
}

// DefaultPriority returns the Priority a new building of this kind gets:
// PriorityHigh for Life and Defence buildings, PriorityNormal otherwise.
func (d BuildingDef) DefaultPriority() int { return d.Category.DefaultPriority() }

// CommandBuildable reports whether the kind can ever be placed by a build
// command: everything except the Keystone, which only a Seedship or world
// generation founds.
func (d BuildingDef) CommandBuildable() bool { return d.StartUnlocked || d.Unlock != TechNone }

// buildingDefs is the building table, indexed by BuildingKind.
var buildingDefs = [NumBuildingKinds]BuildingDef{
	Keystone: {
		Kind: Keystone, Name: "Keystone", Category: CatCore,
		Power: 20, Staff: 0, MaxHP: 1200,
		Unlock: TechNone, MaxPerRock: 1,
		Housing: 12, LifeSupport: 12, Storage: 400, Berths: 4, SensorRange: 260, RevealRadius: 3,
		Role:        "Colony command: power 20, housing 12, life support 12, storage 400, 4 berths, sensors 260 u.",
		Description: "The charter seal of a colony, sunk deep into the rock. If the Keystone falls, the colony falls with it.",
	},
	Conduit: {
		Kind: Conduit, Name: "Conduit", Category: CatCore,
		Cost: Stock{Alloy: 4}, BuildTime: 3, MaxHP: 60,
		Unlock: TechNone, StartUnlocked: true,
		Role:        "Cheap connector tile that extends the buildable area.",
		Description: "A trench of cable and pipe laid across the regolith. It does nothing on its own, but every structure must touch the grid.",
	},
	Silo: {
		Kind: Silo, Name: "Silo", Category: CatCore,
		Cost: Stock{Alloy: 30, Silica: 10}, BuildTime: 15, Power: -1, MaxHP: 300,
		Unlock: TechNone, StartUnlocked: true,
		Storage:     300,
		Role:        "+300 storage for every good.",
		Description: "Pressure-sealed bins cut into the rock. More Silos mean deeper reserves of every good.",
	},
	SurveyMast: {
		Kind: SurveyMast, Name: "Survey Mast", Category: CatCore,
		Cost: Stock{Alloy: 15, Silica: 5}, BuildTime: 12, Power: -2, Staff: 1, MaxHP: 100,
		Unlock: TechNone, StartUnlocked: true,
		SurveyRadius: 4, SurveyInterval: 1.5,
		Role:        "Reveals hidden deposits within 4 hexes, one tile at a time.",
		Description: "A sounding mast that listens to the rock beneath it and charts whatever veins lie nearby.",
	},
	Sunvane: {
		Kind: Sunvane, Name: "Sunvane", Category: CatPower,
		Cost: Stock{Alloy: 20, Silica: 15}, BuildTime: 15, Power: 12, SolarPower: true, MaxHP: 120,
		Unlock: TechNone, StartUnlocked: true,
		Role:        "Solar collector: power 12 x the rock's sunlight.",
		Description: "Broad vanes turned to catch Emberfall's red light. Rocks on tighter orbits yield far more.",
	},
	ThermalStack: {
		Kind: ThermalStack, Name: "Thermal Stack", Category: CatPower,
		Cost: Stock{Alloy: 60, Silica: 20, Parts: 10}, BuildTime: 30, Power: 45, Staff: 2, MaxHP: 250,
		Unlock:      VolatileCracking,
		Inputs:      Stock{Fuel: 6},
		Role:        "Generator: power 45 while it burns 6 Fuel per cycle.",
		Description: "A burner stack that turns cracked Fuel into steady power, whatever the sunlight.",
	},
	LumenCore: {
		Kind: LumenCore, Name: "Lumen Core", Category: CatPower,
		Cost: Stock{Alloy: 150, Parts: 60, Lumen: 20}, BuildTime: 60, Power: 160, Staff: 4, MaxHP: 400,
		Unlock:      LumenFusion,
		Inputs:      Stock{Lumen: 0.5},
		Role:        "Reactor: power 160 while it burns 0.5 Lumen per cycle.",
		Description: "A lattice of Lumen crystal held at the edge of fusion. A sliver of fuel keeps a whole colony lit.",
	},
	Habitat: {
		Kind: Habitat, Name: "Habitat", Category: CatLife,
		Cost: Stock{Alloy: 30, Silica: 30}, BuildTime: 20, Power: -3, MaxHP: 200,
		Unlock: TechNone, StartUnlocked: true,
		Housing:     16,
		Role:        "Housing for 16 colonists.",
		Description: "Stacked pressure cabins for crews and their families. Crowded colonies grow sullen.",
	},
	Aerator: {
		Kind: Aerator, Name: "Aerator", Category: CatLife,
		Cost: Stock{Alloy: 25, Silica: 10}, BuildTime: 18, Power: -5, Staff: 1, MaxHP: 150,
		Unlock: TechNone, StartUnlocked: true,
		LifeSupport: 24,
		Role:        "Life support for 24 colonists.",
		Description: "Scrubs what the colony exhales and keeps the air fit to breathe. Without enough of them a colony suffocates.",
	},
	Greenhouse: {
		Kind: Greenhouse, Name: "Greenhouse", Category: CatLife,
		Cost: Stock{Alloy: 20, Silica: 40}, BuildTime: 25, Power: -6, Staff: 2, MaxHP: 140,
		Unlock: TechNone, StartUnlocked: true,
		Inputs: Stock{Ice: 2}, Outputs: Stock{Food: 14},
		Role:        "Grows 14 Food per cycle from 2 Ice.",
		Description: "Racks of hardy greens under grow-lamps, watered with melted Ice.",
	},
	Commons: {
		Kind: Commons, Name: "Commons", Category: CatLife,
		Cost: Stock{Alloy: 40, Silica: 40, Parts: 5}, BuildTime: 30, Power: -4, Staff: 2, MaxHP: 180,
		Unlock:      CivicPlanning,
		MoraleCover: 40,
		Role:        "Raises morale; one Commons covers 40 colonists.",
		Description: "A shared hall with a canteen, a stage and a view of the Shoal. Rested crews work harder.",
	},
	BoreRig: {
		Kind: BoreRig, Name: "Bore Rig", Category: CatIndustry,
		Cost: Stock{Alloy: 35}, BuildTime: 20, Power: -8, Staff: 3, MaxHP: 220,
		Unlock: TechNone, StartUnlocked: true,
		MineRate:    36,
		Role:        "Mines 36 units per cycle from deposits on its own tile and the six around it.",
		Description: "A drill frame anchored over a vein. It draws evenly from every deposit it can reach until they run dry.",
	},
	Smelter: {
		Kind: Smelter, Name: "Smelter", Category: CatIndustry,
		Cost: Stock{Alloy: 50, Silica: 20}, BuildTime: 30, Power: -14, Staff: 3, MaxHP: 260,
		Unlock: TechNone, StartUnlocked: true,
		Inputs: Stock{Ore: 30}, Outputs: Stock{Alloy: 20},
		Role:        "Smelts 30 Ore into 20 Alloy per cycle.",
		Description: "Arc furnaces that reduce raw Ore to Alloy, the stuff every structure is made of.",
	},
	Cracker: {
		Kind: Cracker, Name: "Cracker", Category: CatIndustry,
		Cost: Stock{Alloy: 45, Silica: 15, Parts: 5}, BuildTime: 30, Power: -10, Staff: 2, MaxHP: 220,
		Unlock: VolatileCracking,
		Inputs: Stock{Ice: 20}, Outputs: Stock{Fuel: 16},
		Role:        "Cracks 20 Ice into 16 Fuel per cycle.",
		Description: "Breaks Ice into volatile fractions and bottles them as Fuel for stacks and ships.",
	},
	Fabricator: {
		Kind: Fabricator, Name: "Fabricator", Category: CatIndustry,
		Cost: Stock{Alloy: 70, Silica: 30}, BuildTime: 40, Power: -16, Staff: 4, MaxHP: 260,
		Unlock: Fabrication,
		Inputs: Stock{Alloy: 12, Silica: 8}, Outputs: Stock{Parts: 8},
		Role:        "Turns 12 Alloy and 8 Silica into 8 Parts per cycle.",
		Description: "Precision mills and printers that turn Alloy and Silica into machine Parts.",
	},
	Laboratory: {
		Kind: Laboratory, Name: "Laboratory", Category: CatScience,
		Cost: Stock{Alloy: 60, Silica: 40, Parts: 10}, BuildTime: 45, Power: -12, Staff: 4, MaxHP: 200,
		Unlock: TechNone, StartUnlocked: true,
		ResearchRate: 6,
		Role:         "Generates 6 research points per cycle.",
		Description:  "Benches, test rigs and people paid to ask why. Each Laboratory adds to the Hold's research.",
	},
	LaunchCradle: {
		Kind: LaunchCradle, Name: "Launch Cradle", Category: CatSpace,
		Cost: Stock{Alloy: 120, Silica: 40, Parts: 30}, BuildTime: 60, Power: -15, Staff: 4, MaxHP: 400,
		Unlock: OrbitalFlight, MaxPerRock: 3,
		QueueSize:   MaxShipQueue,
		Role:        "Shipyard with a build queue of up to 5 ships.",
		Description: "A gantry and catapult rail where hulls are assembled and flung into orbit.",
	},
	DockRing: {
		Kind: DockRing, Name: "Dock Ring", Category: CatSpace,
		Cost: Stock{Alloy: 80, Parts: 20}, BuildTime: 40, Power: -6, Staff: 2, MaxHP: 300,
		Unlock: DockEngineering,
		Berths: 6, DockRepairRate: 2,
		Role:        "+6 berths; repairs friendly ships in orbit at 2 HP per second.",
		Description: "An orbital mooring ring tethered to the surface. It berths more ships and patches those that limp home.",
	},
	ExchangeDock: {
		Kind: ExchangeDock, Name: "Exchange Dock", Category: CatSpace,
		Cost: Stock{Alloy: 60, Silica: 30, Parts: 15}, BuildTime: 40, Power: -5, Staff: 2, MaxHP: 250,
		Unlock: CharterTrade, MaxPerRock: 1,
		Trade:       true,
		Role:        "Enables market trading and settler sponsorship at this rock.",
		Description: "A bonded wharf licensed to trade with the Exchange and to take on sponsored settlers.",
	},
	FarseerArray: {
		Kind: FarseerArray, Name: "Farseer Array", Category: CatSpace,
		Cost: Stock{Alloy: 50, Silica: 30, Parts: 15}, BuildTime: 35, Power: -10, Staff: 2, MaxHP: 150,
		Unlock:      DeepScan,
		SensorRange: 720,
		Role:        "Long-range sensors: reveals enemy fleets within 720 u.",
		Description: "A wide dish farm that tracks distant drive flares long before they arrive.",
	},
	MenderHub: {
		Kind: MenderHub, Name: "Mender Hub", Category: CatDefence,
		Cost: Stock{Alloy: 60, Parts: 20}, BuildTime: 35, Power: -8, Staff: 2, MaxHP: 200,
		Unlock:       FieldRepair,
		RepairRadius: 4, RepairRate: 6, RepairAlloyPerHP: 0.02,
		Role:        "Repairs damaged buildings within 4 hexes at 6 HP per second, using a little Alloy.",
		Description: "A depot of repair drones that swarm out to patch nearby structures, even under fire.",
	},
	FlakBattery: {
		Kind: FlakBattery, Name: "Flak Battery", Category: CatDefence,
		Cost: Stock{Alloy: 60, Parts: 10}, BuildTime: 30, Power: -6, Staff: 2, MaxHP: 350,
		Unlock:    PointDefence,
		TurretDPS: 8, HullMul: [NumHulls]float64{HullLight: 1.5, HullMedium: 1.0, HullHeavy: 0.6},
		Role:        "Turret: 8 damage per second against ships in orbit; strong against Light hulls, weak against Heavy.",
		Description: "Rapid-fire guns that fill low orbit with shrapnel. Deadly to small craft, feeble against heavy plate.",
	},
	LanceTower: {
		Kind: LanceTower, Name: "Lance Tower", Category: CatDefence,
		Cost: Stock{Alloy: 90, Parts: 30, Lumen: 10}, BuildTime: 45, Power: -20, Staff: 3, MaxHP: 400,
		Unlock:    LumenOptics,
		TurretDPS: 20, HullMul: [NumHulls]float64{HullLight: 0.5, HullMedium: 1.0, HullHeavy: 1.5},
		Role:        "Turret: 20 damage per second against ships in orbit; strong against Heavy hulls, weak against Light.",
		Description: "A Lumen-focused beam that burns through heavy plate but struggles to track nimble craft.",
	},
	AegisProjector: {
		Kind: AegisProjector, Name: "Aegis Projector", Category: CatDefence,
		Cost: Stock{Alloy: 100, Parts: 40, Lumen: 15}, BuildTime: 50, Power: -30, Staff: 3, MaxHP: 300,
		Unlock:     AegisFields,
		ShieldPool: 600, ShieldRegen: 10, ShieldRadius: 3,
		Role:        "Shield pool of 600 that absorbs all damage to friendly buildings within 3 hexes.",
		Description: "Throws a shimmering field over its neighbours. While the pool holds, nothing beneath it takes harm.",
	},
	MassDriver: {
		Kind: MassDriver, Name: "Mass Driver", Category: CatDefence,
		Cost: Stock{Alloy: 140, Parts: 50}, BuildTime: 60, Power: -25, Staff: 4, MaxHP: 350,
		Unlock:      MassAcceleration,
		DriverRange: 600, DriverDamage: 150, DriverCooldown: 15, DriverAmmo: 8,
		Role:        "Bombards a chosen enemy rock within 600 u: one 150-damage slug every 15 s for 8 Alloy.",
		Description: "A rail gun the length of a hangar that hurls Alloy slugs at a rival rock as it drifts into range.",
	},
}

// Buildings returns the building table in BuildingKind order. The slice is
// shared and read-only: do not modify it.
func Buildings() []BuildingDef { return buildingDefs[:] }

// BuildingDefOf returns the table entry of kind. For an invalid kind it
// returns an entry with Kind == BuildingNone, Name "Unknown" and
// Unlock == TechNone, so that callers never index out of range.
func BuildingDefOf(kind BuildingKind) BuildingDef {
	if !kind.Valid() {
		return BuildingDef{Kind: BuildingNone, Name: "Unknown", Unlock: TechNone}
	}
	return buildingDefs[kind]
}

// BuildingsIn returns the building kinds of a category in table order (the
// content of one build-palette tab, Keystone included for CatCore).
func BuildingsIn(cat Category) []BuildingKind {
	var out []BuildingKind
	for i := range buildingDefs {
		if buildingDefs[i].Category == cat {
			out = append(out, buildingDefs[i].Kind)
		}
	}
	return out
}

// StartBuildings returns the building kinds every faction can build from the
// start, in table order.
func StartBuildings() []BuildingKind {
	var out []BuildingKind
	for i := range buildingDefs {
		if buildingDefs[i].StartUnlocked {
			out = append(out, buildingDefs[i].Kind)
		}
	}
	return out
}
