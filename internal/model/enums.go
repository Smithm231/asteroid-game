package model

import "strconv"

// All enumerations in this file are stored in save files as their numeric
// value. The order of the constants is therefore part of the save format:
// append new values at the end (before the Num... sentinel), never reorder or
// remove existing ones.
//
// Every enumeration has a Num... constant (an untyped integer equal to the
// number of values, usable as an array length and as a loop bound), a String
// method returning the display name, and a Valid method.

// enumName returns names[i], or a diagnostic such as "Good(12)" when i is not
// a declared value.
func enumName(names []string, i int, typ string) string {
	if i >= 0 && i < len(names) {
		return names[i]
	}
	return typ + "(" + strconv.Itoa(i) + ")"
}

// ---------------------------------------------------------------------------
// Goods
// ---------------------------------------------------------------------------

// Good identifies a resource. The values are zero-based and index a Stock.
type Good int8

// The goods, in their fixed order: four raw (mined) goods followed by four
// produced goods.
const (
	Ore    Good = iota // raw: mined from Ore veins, smelted into Alloy
	Ice                // raw: mined from Ice pockets; breathed, farmed with, cracked into Fuel
	Silica             // raw: mined from Silica veins; a construction material
	Lumen              // raw: rare crystal mined on Glint rocks; advanced construction and research
	Alloy              // produced by Smelters; the main construction material
	Fuel               // produced by Crackers; burned by Thermal Stacks and by departing fleets
	Parts              // produced by Fabricators; machinery for advanced buildings and ships
	Food               // produced by Greenhouses; eaten by colonists

	// NumGoods is the number of goods and the length of a Stock.
	NumGoods = iota
)

// GoodNone means "no good", for example a tile without a deposit.
const GoodNone Good = -1

var goodNames = [NumGoods]string{"Ore", "Ice", "Silica", "Lumen", "Alloy", "Fuel", "Parts", "Food"}

// String returns the display name of the good ("None" for GoodNone).
func (g Good) String() string {
	if g == GoodNone {
		return "None"
	}
	return enumName(goodNames[:], int(g), "Good")
}

// Valid reports whether g is one of the NumGoods real goods.
func (g Good) Valid() bool { return g >= 0 && g < NumGoods }

// Raw reports whether g is a mined raw good (Ore, Ice, Silica, Lumen), i.e.
// one that can occur as a tile deposit.
func (g Good) Raw() bool { return g >= Ore && g <= Lumen }

// AllGoods returns every good in enum order.
func AllGoods() [NumGoods]Good {
	var a [NumGoods]Good
	for i := range a {
		a[i] = Good(i)
	}
	return a
}

// ---------------------------------------------------------------------------
// Terrain and rock classes
// ---------------------------------------------------------------------------

// Terrain is the surface type of a hex tile.
type Terrain int8

// Terrain values.
const (
	TerrainVoid     Terrain = iota // not part of the rock (also the zero value)
	TerrainRegolith                // open ground; the only buildable terrain
	TerrainCrag                    // blocked ridge
	TerrainFissure                 // blocked crack

	// NumTerrains is the number of Terrain values.
	NumTerrains = iota
)

var terrainNames = [NumTerrains]string{"Void", "Regolith", "Crag", "Fissure"}

// String returns the display name of the terrain.
func (t Terrain) String() string { return enumName(terrainNames[:], int(t), "Terrain") }

// Valid reports whether t is a declared Terrain value.
func (t Terrain) Valid() bool { return t >= 0 && t < NumTerrains }

// Solid reports whether the terrain is part of the rock (anything but Void).
func (t Terrain) Solid() bool { return t == TerrainRegolith || t == TerrainCrag || t == TerrainFissure }

// RockClass is the geological class of a rock; it decides which deposits
// world generation favours.
type RockClass int8

// Rock classes.
const (
	ClassFerric RockClass = iota // Ore-rich; common on inner orbits
	ClassStony                   // Silica-rich with some Ore
	ClassFrost                   // Ice-rich; outer orbits only
	ClassGlint                   // small and Lumen-bearing; rare

	// NumRockClasses is the number of RockClass values.
	NumRockClasses = iota
)

var rockClassNames = [NumRockClasses]string{"Ferric", "Stony", "Frost", "Glint"}

// String returns the display name of the rock class.
func (c RockClass) String() string { return enumName(rockClassNames[:], int(c), "RockClass") }

// Valid reports whether c is a declared RockClass value.
func (c RockClass) Valid() bool { return c >= 0 && c < NumRockClasses }

// AllRockClasses returns every rock class in enum order.
func AllRockClasses() [NumRockClasses]RockClass {
	var a [NumRockClasses]RockClass
	for i := range a {
		a[i] = RockClass(i)
	}
	return a
}

// ---------------------------------------------------------------------------
// Buildings
// ---------------------------------------------------------------------------

// Category groups building kinds. It is both the build-palette tab (in enum
// order) and, through Rank, the tie-break of staffing and power priority.
type Category int8

// Building categories, in palette-tab order.
const (
	CatCore     Category = iota // Keystone, Conduit, Silo, Survey Mast
	CatPower                    // generators
	CatLife                     // housing, air, food, morale
	CatIndustry                 // mining and refining
	CatScience                  // research
	CatSpace                    // shipyard, docks, trade, sensors
	CatDefence                  // turrets, shields, repair, bombardment

	// NumCategories is the number of Category values.
	NumCategories = iota
)

var categoryNames = [NumCategories]string{"Core", "Power", "Life", "Industry", "Science", "Space", "Defence"}

// categoryRanks holds, per Category, its rank in the staffing and power
// order: Life, Power, Industry, Science, Space, Defence, Core.
var categoryRanks = [NumCategories]int{
	CatLife:     0,
	CatPower:    1,
	CatIndustry: 2,
	CatScience:  3,
	CatSpace:    4,
	CatDefence:  5,
	CatCore:     6,
}

// String returns the display name of the category.
func (c Category) String() string { return enumName(categoryNames[:], int(c), "Category") }

// Valid reports whether c is a declared Category value.
func (c Category) Valid() bool { return c >= 0 && c < NumCategories }

// Rank returns the category's position in the staffing and power order
// (lower is served first): Life 0, Power 1, Industry 2, Science 3, Space 4,
// Defence 5, Core 6. Buildings are served by (Priority descending, Rank
// ascending, ID ascending). Unknown categories rank last.
func (c Category) Rank() int {
	if !c.Valid() {
		return NumCategories
	}
	return categoryRanks[c]
}

// DefaultPriority returns the Priority a new building of this category gets:
// PriorityHigh for Life and Defence, PriorityNormal for everything else.
func (c Category) DefaultPriority() int {
	if c == CatLife || c == CatDefence {
		return PriorityHigh
	}
	return PriorityNormal
}

// AllCategories returns every category in palette-tab order.
func AllCategories() [NumCategories]Category {
	var a [NumCategories]Category
	for i := range a {
		a[i] = Category(i)
	}
	return a
}

// BuildState is the life-cycle state of a building.
type BuildState int8

// Building states.
const (
	StateConstructing BuildState = iota // placed, not finished; no function yet
	StateActive                         // completed and owned; functions when Enabled
	StateDerelict                       // left behind by a fallen colony: owner 0, inert

	// NumBuildStates is the number of BuildState values.
	NumBuildStates = iota
)

var buildStateNames = [NumBuildStates]string{"Constructing", "Active", "Derelict"}

// String returns the display name of the build state.
func (s BuildState) String() string { return enumName(buildStateNames[:], int(s), "BuildState") }

// Valid reports whether s is a declared BuildState value.
func (s BuildState) Valid() bool { return s >= 0 && s < NumBuildStates }

// BuildingKind identifies a kind of building. The values are zero-based
// indices into the building table (see BuildingDefOf).
type BuildingKind int8

// Building kinds, in table order.
const (
	Keystone       BuildingKind = iota // colony command structure
	Conduit                            // connector tile
	Silo                               // storage
	SurveyMast                         // reveals nearby deposits
	Sunvane                            // solar generator
	ThermalStack                       // Fuel-burning generator
	LumenCore                          // Lumen-burning reactor
	Habitat                            // housing
	Aerator                            // life support
	Greenhouse                         // Food production
	Commons                            // morale
	BoreRig                            // mining
	Smelter                            // Ore to Alloy
	Cracker                            // Ice to Fuel
	Fabricator                         // Alloy and Silica to Parts
	Laboratory                         // research points
	LaunchCradle                       // shipyard
	DockRing                           // berths and ship repair
	ExchangeDock                       // market access
	FarseerArray                       // long-range sensors
	MenderHub                          // building repair
	FlakBattery                        // anti-ship turret, strong against Light hulls
	LanceTower                         // anti-ship turret, strong against Heavy hulls
	AegisProjector                     // area shield
	MassDriver                         // long-range bombardment

	// NumBuildingKinds is the number of building kinds.
	NumBuildingKinds = iota
)

// BuildingNone means "no building kind" (for example nothing selected in a
// build palette). It is not a valid table index.
const BuildingNone BuildingKind = -1

// String returns the display name of the building kind ("None" for
// BuildingNone).
func (k BuildingKind) String() string {
	if k == BuildingNone {
		return "None"
	}
	if !k.Valid() {
		return enumName(nil, int(k), "BuildingKind")
	}
	return buildingDefs[k].Name
}

// Valid reports whether k is one of the NumBuildingKinds real kinds.
func (k BuildingKind) Valid() bool { return k >= 0 && k < NumBuildingKinds }

// AllBuildingKinds returns every building kind in table order.
func AllBuildingKinds() [NumBuildingKinds]BuildingKind {
	var a [NumBuildingKinds]BuildingKind
	for i := range a {
		a[i] = BuildingKind(i)
	}
	return a
}

// IdleReason explains, for the UI, why a completed building did nothing in
// the last economy step. It is written by the systems that run the building.
type IdleReason int8

// Idle reasons.
const (
	IdleNone        IdleReason = iota // working normally (or nothing to report)
	IdleDisabled                      // switched off by its owner
	IdleNoPower                       // not powered
	IdleNoStaff                       // no workers assigned
	IdleNoInput                       // an input good or ammunition is missing
	IdleStorageFull                   // no room for its output
	IdleNoDeposit                     // no deposit with material in reach (Bore Rig)
	IdleNoTarget                      // no valid target (Mass Driver, turrets)
	IdleNoWork                        // nothing to do (empty queue, nothing to repair or survey)

	// NumIdleReasons is the number of IdleReason values.
	NumIdleReasons = iota
)

var idleReasonNames = [NumIdleReasons]string{
	"Working", "Switched off", "No power", "No staff", "Missing input",
	"Storage full", "No deposit", "No target", "Standing by",
}

// String returns a short status text for the reason.
func (r IdleReason) String() string { return enumName(idleReasonNames[:], int(r), "IdleReason") }

// Valid reports whether r is a declared IdleReason value.
func (r IdleReason) Valid() bool { return r >= 0 && r < NumIdleReasons }

// ---------------------------------------------------------------------------
// Ships and fleets
// ---------------------------------------------------------------------------

// Hull is the weight class of a ship; turrets apply a damage multiplier per
// hull class.
type Hull int8

// Hull classes.
const (
	HullLight  Hull = iota // Prospector, Dart
	HullMedium             // Hauler, Seedship, Cutter, Hammer
	HullHeavy              // Bulwark

	// NumHulls is the number of Hull values.
	NumHulls = iota
)

var hullNames = [NumHulls]string{"Light", "Medium", "Heavy"}

// String returns the display name of the hull class.
func (h Hull) String() string { return enumName(hullNames[:], int(h), "Hull") }

// Valid reports whether h is a declared Hull value.
func (h Hull) Valid() bool { return h >= 0 && h < NumHulls }

// ShipKind identifies a kind of ship. The values are zero-based indices into
// the ship table (see ShipDefOf).
type ShipKind int8

// Ship kinds, in table order.
const (
	Prospector ShipKind = iota // survey scout
	Hauler                     // freighter
	Seedship                   // colony ship
	Dart                       // interceptor
	Cutter                     // line gunship
	Hammer                     // bomber
	Bulwark                    // heavy frigate

	// NumShipKinds is the number of ship kinds.
	NumShipKinds = iota
)

// ShipNone means "no ship kind". It is not a valid table index.
const ShipNone ShipKind = -1

// String returns the display name of the ship kind ("None" for ShipNone).
func (k ShipKind) String() string {
	if k == ShipNone {
		return "None"
	}
	if !k.Valid() {
		return enumName(nil, int(k), "ShipKind")
	}
	return shipDefs[k].Name
}

// Valid reports whether k is one of the NumShipKinds real kinds.
func (k ShipKind) Valid() bool { return k >= 0 && k < NumShipKinds }

// AllShipKinds returns every ship kind in table order.
func AllShipKinds() [NumShipKinds]ShipKind {
	var a [NumShipKinds]ShipKind
	for i := range a {
		a[i] = ShipKind(i)
	}
	return a
}

// OrderKind is the standing order of a fleet.
type OrderKind int8

// Fleet orders.
const (
	OrderHold     OrderKind = iota // stay in orbit (also the zero value)
	OrderMove                      // fly to Dest, then hold or merge into the garrison
	OrderAttack                    // fly to Dest and fight there, using Stance
	OrderSurvey                    // Prospector: survey Dest, then return home
	OrderColonize                  // Seedship: found a Keystone on Dest
	OrderHaul                      // run the freight Route
	OrderReturn                    // fly to Home and merge into its garrison

	// NumOrderKinds is the number of OrderKind values.
	NumOrderKinds = iota
)

var orderNames = [NumOrderKinds]string{"Hold", "Move", "Attack", "Survey", "Colonize", "Haul", "Return"}

// String returns the display name of the order.
func (o OrderKind) String() string { return enumName(orderNames[:], int(o), "OrderKind") }

// Valid reports whether o is a declared OrderKind value.
func (o OrderKind) Valid() bool { return o >= 0 && o < NumOrderKinds }

// Stance selects the bombardment target order of an attacking fleet.
type Stance int8

// Attack stances.
const (
	StanceSuppress Stance = iota // defences first, then the Keystone, then the rest
	StanceRaid                   // industry and power first, then defences, then the Keystone
	StanceRaze                   // the Keystone first, then defences, then the rest

	// NumStances is the number of Stance values.
	NumStances = iota
)

var stanceNames = [NumStances]string{"Suppress", "Raid", "Raze"}

// String returns the display name of the stance.
func (s Stance) String() string { return enumName(stanceNames[:], int(s), "Stance") }

// Valid reports whether s is a declared Stance value.
func (s Stance) Valid() bool { return s >= 0 && s < NumStances }

// ---------------------------------------------------------------------------
// Technologies
// ---------------------------------------------------------------------------

// TechID identifies a technology. The values are zero-based indices into the
// tech table (see TechDefOf) and into Faction.Techs.
type TechID int8

// Technologies, in table order (tier 1, then tier 2, then tier 3).
const (
	OrbitalFlight      TechID = iota // Launch Cradle, Prospector, Hauler
	VolatileCracking                 // Cracker, Thermal Stack
	Fabrication                      // Fabricator
	CivicPlanning                    // Commons
	PointDefence                     // Flak Battery, Dart
	DeepBoring                       // Bore Rig output +25 %
	CharterTrade                     // Exchange Dock
	ColonialCharter                  // Seedship
	Gunnery                          // Cutter
	SiegeDoctrine                    // Hammer
	DeepScan                         // Farseer Array; Survey Mast range +2
	FieldRepair                      // Mender Hub
	Photovoltaics                    // Sunvane output +30 %
	HydroStacks                      // Greenhouse output +35 %
	DockEngineering                  // Dock Ring
	EfficientSmelting                // Smelter output +25 %
	MassAcceleration                 // Mass Driver
	LumenOptics                      // Lance Tower
	AegisFields                      // Aegis Projector
	LumenFusion                      // Lumen Core
	CapitalHulls                     // Bulwark
	ReactivePlating                  // ship and building max HP +20 %
	OverdriveThrusters               // ship speed +25 %
	Automation                       // staff needs -25 %

	// NumTechs is the number of technologies.
	NumTechs = iota
)

// TechNone means "no technology": nothing being researched, or a building
// that needs no research. It is not a valid table index.
const TechNone TechID = -1

// String returns the display name of the technology ("None" for TechNone).
func (t TechID) String() string {
	if t == TechNone {
		return "None"
	}
	if !t.Valid() {
		return enumName(nil, int(t), "TechID")
	}
	return techDefs[t].Name
}

// Valid reports whether t is one of the NumTechs real technologies.
func (t TechID) Valid() bool { return t >= 0 && t < NumTechs }

// AllTechs returns every technology in table order.
func AllTechs() [NumTechs]TechID {
	var a [NumTechs]TechID
	for i := range a {
		a[i] = TechID(i)
	}
	return a
}

// ---------------------------------------------------------------------------
// Commands, log and FX
// ---------------------------------------------------------------------------

// CmdKind discriminates a Command. See Command for the fields each kind uses.
type CmdKind int8

// Command kinds. CmdNone is the zero value and is never a valid command.
const (
	CmdNone            CmdKind = iota // invalid / empty command
	CmdBuild                          // construct: place a building
	CmdDemolish                       // construct: cancel, demolish or salvage a building
	CmdRecommission                   // construct: reactivate a derelict
	CmdSetEnabled                     // construct: switch a building on or off
	CmdSetPriority                    // construct: set a building's priority 0..2
	CmdExpedite                       // construct: pay Scrip to finish a construction now
	CmdQueueShip                      // fleet: add a ship to a Launch Cradle queue
	CmdCancelShip                     // fleet: remove an entry from a Launch Cradle queue
	CmdFleetOrder                     // fleet: give a fleet (or a subset of its ships) an order
	CmdSetRoute                       // fleet: assign a freight route and start hauling
	CmdScuttle                        // fleet: destroy own ships
	CmdSetResearch                    // research: make a tech the current project
	CmdQueueResearch                  // research: append a tech to the queue
	CmdCancelResearch                 // research: cancel the current project or a queue entry
	CmdTrade                          // market: buy (+) or sell (-) a good
	CmdSponsor                        // market: sponsor settlers
	CmdSetDriverTarget                // combat: set or clear a Mass Driver target

	// NumCmdKinds is the number of CmdKind values, CmdNone included.
	NumCmdKinds = iota
)

var cmdKindNames = [NumCmdKinds]string{
	"CmdNone", "CmdBuild", "CmdDemolish", "CmdRecommission", "CmdSetEnabled", "CmdSetPriority",
	"CmdExpedite", "CmdQueueShip", "CmdCancelShip", "CmdFleetOrder", "CmdSetRoute", "CmdScuttle",
	"CmdSetResearch", "CmdQueueResearch", "CmdCancelResearch", "CmdTrade", "CmdSponsor",
	"CmdSetDriverTarget",
}

// String returns the name of the command kind, e.g. "CmdBuild".
func (k CmdKind) String() string { return enumName(cmdKindNames[:], int(k), "CmdKind") }

// Valid reports whether k is a real command kind (CmdNone is not).
func (k CmdKind) Valid() bool { return k > CmdNone && k < NumCmdKinds }

// Severity grades a log entry.
type Severity int8

// Log severities.
const (
	SevInfo  Severity = iota // neutral information
	SevGood                  // good news
	SevWarn                  // something needs attention (also rejected commands)
	SevAlert                 // urgent: under attack, colony lost, ...

	// NumSeverities is the number of Severity values.
	NumSeverities = iota
)

var severityNames = [NumSeverities]string{"Info", "Good", "Warn", "Alert"}

// String returns the display name of the severity.
func (s Severity) String() string { return enumName(severityNames[:], int(s), "Severity") }

// Valid reports whether s is a declared Severity value.
func (s Severity) Valid() bool { return s >= 0 && s < NumSeverities }

// FXKind identifies a transient visual / audio cue emitted by the simulation.
// FX are never saved, so these values are not part of the save format.
type FXKind int8

// FX kinds. FXNone is the zero value and is never emitted.
const (
	FXNone           FXKind = iota // no effect
	FXBuildPlaced                  // a building was placed (Rock, Hex)
	FXBuildDone                    // a construction finished (Rock, Hex)
	FXBuildDestroyed               // a building was destroyed (Rock, Hex)
	FXShipBuilt                    // a ship left a Launch Cradle (Rock, Hex)
	FXFleetLaunch                  // a fleet left orbit (Rock, Fleet, A = position)
	FXFleetArrive                  // a fleet entered orbit (Rock, Fleet, A = position)
	FXShotFlak                     // a Flak Battery fired (Rock, Hex, B = target position)
	FXShotLance                    // a Lance Tower fired (Rock, Hex, B = target position)
	FXShotShip                     // a ship fired at a ship (Rock, Fleet, A and B)
	FXBomb                         // a ship bombarded a building (Rock, Hex)
	FXShipDestroyed                // a ship was destroyed (Rock or NoRock, Fleet, A = position)
	FXShieldHit                    // an Aegis pool absorbed damage (Rock, Hex)
	FXSlugFired                    // a Mass Driver fired (Rock, Hex, A = from, B = toward)
	FXSlugImpact                   // a slug struck a rock (Rock, Hex)
	FXMeteorImpact                 // a micrometeor struck a rock (Rock, Hex)
	FXColonyFounded                // a Keystone was founded (Rock, Hex)
	FXColonyFell                   // a Keystone was destroyed (Rock, Hex)
	FXTechDone                     // a technology was completed (Amount = TechID)
	FXDirectiveDone                // a directive was completed (Amount = directive index)
	FXAlert                        // generic attention cue (Rock when relevant)
	FXTrade                        // a market trade or sponsorship happened (Rock, Amount)
	FXFlareStart                   // a solar flare began
	FXFlareEnd                     // a solar flare ended

	// NumFXKinds is the number of FXKind values, FXNone included.
	NumFXKinds = iota
)

var fxKindNames = [NumFXKinds]string{
	"None", "BuildPlaced", "BuildDone", "BuildDestroyed", "ShipBuilt", "FleetLaunch", "FleetArrive",
	"ShotFlak", "ShotLance", "ShotShip", "Bomb", "ShipDestroyed", "ShieldHit", "SlugFired",
	"SlugImpact", "MeteorImpact", "ColonyFounded", "ColonyFell", "TechDone", "DirectiveDone",
	"Alert", "Trade", "FlareStart", "FlareEnd",
}

// String returns the name of the FX kind, e.g. "BuildPlaced".
func (k FXKind) String() string { return enumName(fxKindNames[:], int(k), "FXKind") }

// Valid reports whether k is a real FX kind (FXNone is not).
func (k FXKind) Valid() bool { return k > FXNone && k < NumFXKinds }

// EventKind identifies a random belt event.
type EventKind int8

// Event kinds. EventNone is the zero value.
const (
	EventNone        EventKind = iota // no event
	EventSquall                       // micrometeor squall on one colonised rock
	EventFlare                        // belt-wide solar flare
	EventRichSeam                     // a deposit next to a Bore Rig is enriched
	EventHulk                         // derelict hulk: salvage in orbit of a rock
	EventRefugees                     // refugee convoy joins a colony
	EventMarketShock                  // one good's market pressure jumps

	// NumEventKinds is the number of EventKind values, EventNone included.
	NumEventKinds = iota
)

var eventKindNames = [NumEventKinds]string{
	"None", "Micrometeor squall", "Solar flare", "Rich seam", "Derelict hulk", "Refugee convoy", "Market shock",
}

// String returns the display name of the event kind.
func (k EventKind) String() string { return enumName(eventKindNames[:], int(k), "EventKind") }

// Valid reports whether k is a real event kind (EventNone is not).
func (k EventKind) Valid() bool { return k > EventNone && k < NumEventKinds }

// ---------------------------------------------------------------------------
// Game setup
// ---------------------------------------------------------------------------

// Personality selects the behaviour (and the Hold name) of an AI rival.
type Personality int8

// Rival personalities. PersonalityNone is the zero value: a human-controlled
// Hold, or "pick at random" inside GameSettings.RivalKinds.
const (
	PersonalityNone      Personality = iota // human player / unspecified
	PersonalityOstrander                    // Ostrander Deepworks: industrial turtle
	PersonalityVane                         // The Vane Compact: raiders
	PersonalityMeridian                     // Meridian Lattice: expansionist

	// NumPersonalities is the number of Personality values, PersonalityNone
	// included.
	NumPersonalities = iota
)

var personalityNames = [NumPersonalities]string{
	"Independent", "Ostrander Deepworks", "The Vane Compact", "Meridian Lattice",
}

var personalityArchetypes = [NumPersonalities]string{
	"Independent", "Industrialist", "Raider", "Expansionist",
}

var personalityBlurbs = [NumPersonalities]string{
	"A Hold that answers to its Warden alone.",
	"Patient industrialists: a strong economy behind heavy defences, and heavy fleets late in the game.",
	"Raiders who strike early and often with light, fast squadrons.",
	"Expansionists who stake claims on many rocks and field medium fleets.",
}

// String returns the Hold name of the personality, e.g. "The Vane Compact"
// ("Independent" for PersonalityNone).
func (p Personality) String() string { return enumName(personalityNames[:], int(p), "Personality") }

// Valid reports whether p is a declared Personality value (PersonalityNone
// included).
func (p Personality) Valid() bool { return p >= 0 && p < NumPersonalities }

// IsRival reports whether p is one of the three AI personalities.
func (p Personality) IsRival() bool { return p > PersonalityNone && p < NumPersonalities }

// Archetype returns a one-word description of the play style, e.g. "Raider".
func (p Personality) Archetype() string {
	return enumName(personalityArchetypes[:], int(p), "Personality")
}

// Blurb returns a one-sentence description of the play style for setup
// screens.
func (p Personality) Blurb() string {
	if !p.Valid() {
		return ""
	}
	return personalityBlurbs[p]
}

// RivalPersonalities returns the three AI personalities in enum order.
func RivalPersonalities() [3]Personality {
	return [3]Personality{PersonalityOstrander, PersonalityVane, PersonalityMeridian}
}

// Difficulty scales the AI rivals' production (see Difficulty.ProdMul).
type Difficulty int8

// Difficulty levels.
const (
	DiffEasy   Difficulty = iota // AI production x0.8
	DiffNormal                   // AI production x1.0
	DiffHard                     // AI production x1.25

	// NumDifficulties is the number of Difficulty values.
	NumDifficulties = iota
)

var difficultyNames = [NumDifficulties]string{"Easy", "Normal", "Hard"}

var difficultyProdMuls = [NumDifficulties]float64{0.8, 1.0, 1.25}

// String returns the display name of the difficulty.
func (d Difficulty) String() string { return enumName(difficultyNames[:], int(d), "Difficulty") }

// Valid reports whether d is a declared Difficulty value.
func (d Difficulty) Valid() bool { return d >= 0 && d < NumDifficulties }

// ProdMul returns the production handicap applied to AI factions at this
// difficulty: Easy 0.8, Normal 1.0, Hard 1.25 (1.0 for unknown values). Human
// factions always use 1.0; see ComputeMods.
func (d Difficulty) ProdMul() float64 {
	if !d.Valid() {
		return 1
	}
	return difficultyProdMuls[d]
}

// BeltSize selects how many rocks the belt has.
type BeltSize int8

// Belt sizes.
const (
	BeltSmall  BeltSize = iota // 10 rocks
	BeltMedium                 // 13 rocks
	BeltLarge                  // 16 rocks

	// NumBeltSizes is the number of BeltSize values.
	NumBeltSizes = iota
)

var beltSizeNames = [NumBeltSizes]string{"Small", "Medium", "Large"}

var beltRockCounts = [NumBeltSizes]int{10, 13, 16}

// String returns the display name of the belt size.
func (b BeltSize) String() string { return enumName(beltSizeNames[:], int(b), "BeltSize") }

// Valid reports whether b is a declared BeltSize value.
func (b BeltSize) Valid() bool { return b >= 0 && b < NumBeltSizes }

// RockCount returns the number of rocks in a belt of this size: 10, 13 or 16
// (13 for unknown values).
func (b BeltSize) RockCount() int {
	if !b.Valid() {
		return beltRockCounts[BeltMedium]
	}
	return beltRockCounts[b]
}
