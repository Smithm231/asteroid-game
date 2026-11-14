package model

import "math"

// This file holds every scalar tuning constant of the game rules, so that no
// system hard-codes a magic number. Numbers that belong to one building, ship
// or technology live in the content tables instead (BuildingDef, ShipDef,
// TechDef). Orbit and sunlight constants are in orbit.go.

// SaveVersion is the value of World.Version written by this version of the
// model. Bump it whenever the serialised layout changes incompatibly.
const SaveVersion = 1

// Time and cadence.
const (
	// TicksPerSecond is the number of sim ticks per game-second.
	TicksPerSecond = 10
	// SecondsPerCycle is the length of one cycle in game-seconds.
	SecondsPerCycle = 60
	// TicksPerCycle is the length of one cycle in ticks (600).
	TicksPerCycle = TicksPerSecond * SecondsPerCycle
	// TickDT is the duration of one tick in game-seconds; movement and
	// combat run every tick with this dt.
	TickDT = 1.0 / TicksPerSecond
	// SlowTickInterval is the tick interval of the slow systems (economy,
	// construction, research, market, events, directives): they run when
	// World.Tick % SlowTickInterval == 0.
	SlowTickInterval = 10
	// SlowDT is the dt, in game-seconds, used by the slow systems.
	SlowDT = float64(SlowTickInterval) / TicksPerSecond
	// AIInterval is the tick interval between two AI steps of one faction.
	AIInterval = 20
	// AIStagger is the per-faction tick offset of the AI schedule: faction f
	// thinks when (Tick + AIStagger*f) % AIInterval == 0.
	AIStagger = 5
)

// SecondsToTicks converts a duration in game-seconds to ticks, rounding to
// the nearest tick.
func SecondsToTicks(seconds float64) int {
	return int(math.Round(seconds * TicksPerSecond))
}

// CyclesToTicks converts a duration in cycles to ticks, rounding to the
// nearest tick.
func CyclesToTicks(cycles float64) int {
	return int(math.Round(cycles * TicksPerCycle))
}

// CycleOfTick returns the displayed date of a tick: tick/TicksPerCycle + 1.
func CycleOfTick(tick int) int { return tick/TicksPerCycle + 1 }

// PerCycle converts a per-cycle rate into the amount that accrues over dt
// game-seconds: rate * dt / SecondsPerCycle.
func PerCycle(rate, dt float64) float64 { return rate * dt / SecondsPerCycle }

// Belt and rock generation.
const (
	// RockRadiusMin is the smallest rock radius in hexes.
	RockRadiusMin = 7
	// RockRadiusMax is the largest rock radius in hexes.
	RockRadiusMax = 11
	// BlobFillMin is the smallest fraction of the hex disc covered by the
	// rock blob.
	BlobFillMin = 0.60
	// BlobFillMax is the largest fraction of the hex disc covered by the rock
	// blob.
	BlobFillMax = 0.75
	// BlockedFracMin is the smallest fraction of blob tiles that are Crag or
	// Fissure.
	BlockedFracMin = 0.10
	// BlockedFracMax is the largest fraction of blob tiles that are Crag or
	// Fissure.
	BlockedFracMax = 0.18

	// FerricMaxRadius is the orbit radius, in u, inside which Ferric rocks
	// are common.
	FerricMaxRadius = 650.0
	// FrostMinRadius is the orbit radius, in u, outside which Frost rocks may
	// occur.
	FrostMinRadius = 560.0
	// GlintRocksMin is the smallest number of Glint rocks in a belt.
	GlintRocksMin = 2
	// GlintRocksMax is the largest number of Glint rocks in a belt.
	GlintRocksMax = 3

	// VeinTilesMin is the smallest number of tiles in an Ore, Ice or Silica
	// vein.
	VeinTilesMin = 3
	// VeinTilesMax is the largest number of tiles in an Ore, Ice or Silica
	// vein.
	VeinTilesMax = 7
	// VeinAmountMin is the smallest deposit amount per vein tile.
	VeinAmountMin = 350.0
	// VeinAmountMax is the largest deposit amount per vein tile.
	VeinAmountMax = 1400.0
	// LumenTilesMin is the smallest number of tiles in a Lumen deposit.
	LumenTilesMin = 1
	// LumenTilesMax is the largest number of tiles in a Lumen deposit.
	LumenTilesMax = 3
	// LumenAmountMin is the smallest deposit amount per Lumen tile.
	LumenAmountMin = 80.0
	// LumenAmountMax is the largest deposit amount per Lumen tile.
	LumenAmountMax = 220.0

	// StartVeinRange is the hex distance from a start Keystone within which
	// an Ore vein and a Silica vein are guaranteed.
	StartVeinRange = 4
	// StartIceRange is the hex distance from a start Keystone within which a
	// small Ice pocket is guaranteed.
	StartIceRange = 6

	// RubbleAlloyBonus is the Alloy granted when a building is placed on a
	// Rubble tile (which clears the rubble).
	RubbleAlloyBonus = 10.0
)

// Game setup.
const (
	// StartPop is the population of every starting colony.
	StartPop = 12.0
	// StartMorale is the morale of every starting colony (and of colonies
	// founded later).
	StartMorale = 60.0
	// StartScrip is the Scrip every faction starts with.
	StartScrip = 400.0
	// MinRivals is the smallest number of AI rivals in a game.
	MinRivals = 1
	// MaxRivals is the largest number of AI rivals in a game.
	MaxRivals = 3
	// DefaultPlayerName is the default name of the human player's Hold.
	DefaultPlayerName = "Free Charter of Ansel"
)

// StartStock returns the stock of every starting colony: Alloy 260,
// Silica 180, Ice 90, Fuel 30, Food 110, Parts 20 (no Ore, no Lumen).
func StartStock() Stock {
	return Stock{Alloy: 260, Silica: 180, Ice: 90, Fuel: 30, Food: 110, Parts: 20}
}

// KeystoneAlloyTrickle is the Alloy a Keystone's reclaimer yields per cycle.
// It is small, but it guarantees that a colony which has spent its last Alloy
// can always work its way back to a Bore Rig and a Smelter.
const KeystoneAlloyTrickle = 4.0

// Faction accent colour indices (the palette itself belongs to the UI).
const (
	// ColorNeutral is the accent index of unowned things (grey).
	ColorNeutral = 0
	// ColorAmber is the accent index of the human player.
	ColorAmber = 1
	// ColorTeal is the accent index of the first rival.
	ColorTeal = 2
	// ColorMagenta is the accent index of the second rival.
	ColorMagenta = 3
	// ColorLime is the accent index of the third rival.
	ColorLime = 4
	// NumAccentColors is the number of defined accent colours.
	NumAccentColors = 5
)

// Colonists, staffing and morale.
const (
	// FoodPerColonist is the Food eaten per colonist per cycle.
	FoodPerColonist = 0.5
	// IcePerColonist is the Ice used per colonist per cycle.
	IcePerColonist = 0.1
	// StipendPerColonist is the Scrip a faction earns per colonist per cycle.
	StipendPerColonist = 0.4

	// MoraleMin is the lowest possible morale.
	MoraleMin = 0.0
	// MoraleMax is the highest possible morale.
	MoraleMax = 100.0
	// MoraleDriftPerSecond is how fast morale moves toward its target, in
	// points per game-second.
	MoraleDriftPerSecond = 0.5
	// MoraleBase is the base morale target.
	MoraleBase = 55.0
	// MoraleFedBonus is added to the target when the colony is neither
	// starving nor suffocating.
	MoraleFedBonus = 10.0
	// MoraleStarvingPenalty is subtracted from the target while Starving.
	MoraleStarvingPenalty = 25.0
	// MoraleSuffocatingPenalty is subtracted from the target while
	// Suffocating.
	MoraleSuffocatingPenalty = 30.0
	// MoraleCrowdingPenalty is the maximum overcrowding penalty: it is scaled
	// by clamp((Pop - housing)/housing, 0, 1).
	MoraleCrowdingPenalty = 20.0
	// MoraleCommonsBonus is the maximum Commons bonus: it is scaled by the
	// commons coverage (0..1).
	MoraleCommonsBonus = 20.0
	// MoraleAttackPenalty is subtracted from the target while the colony was
	// attacked within MoraleAttackWindowTicks.
	MoraleAttackPenalty = 15.0
	// MoraleAttackWindowTicks is how long an attack depresses morale
	// (2 cycles).
	MoraleAttackWindowTicks = 2 * TicksPerCycle

	// MoraleHigh is the morale at or above which buildings get
	// MoraleFactorHigh.
	MoraleHigh = 70.0
	// MoraleLow is the morale below which buildings get MoraleFactorLow.
	MoraleLow = 30.0
	// MoraleFactorHigh is the efficiency factor at high morale.
	MoraleFactorHigh = 1.10
	// MoraleFactorMid is the efficiency factor at ordinary morale.
	MoraleFactorMid = 1.0
	// MoraleFactorLow is the efficiency factor at low morale.
	MoraleFactorLow = 0.75

	// PopGrowthMinMorale is the morale needed for the population to grow.
	PopGrowthMinMorale = 45.0
	// PopGrowthBase is the minimum growth per cycle before the morale factor.
	PopGrowthBase = 0.5
	// PopGrowthRate is the growth per cycle as a fraction of Pop before the
	// morale factor: growth = max(PopGrowthBase, PopGrowthRate*Pop) *
	// Morale/PopGrowthMoraleRef.
	PopGrowthRate = 0.04
	// PopGrowthMoraleRef is the morale at which the growth factor is 1.
	PopGrowthMoraleRef = 60.0
	// PopCrowdingLimit is the multiple of housing above which the population
	// never grows.
	PopCrowdingLimit = 1.25
	// PopLossBase is the fixed loss per cycle for each of Starving and
	// Suffocating.
	PopLossBase = 0.5
	// PopLossRate is the loss per cycle as a fraction of Pop for each of
	// Starving and Suffocating.
	PopLossRate = 0.05
	// PopUnrestMorale is the morale below which colonists leave.
	PopUnrestMorale = 20.0
	// PopUnrestLossRate is the additional loss per cycle, as a fraction of
	// Pop, while morale is below PopUnrestMorale.
	PopUnrestLossRate = 0.02
)

// MoraleFactor returns the efficiency factor for a colony morale:
// MoraleFactorHigh at MoraleHigh or above, MoraleFactorLow below MoraleLow,
// MoraleFactorMid in between.
func MoraleFactor(morale float64) float64 {
	switch {
	case morale >= MoraleHigh:
		return MoraleFactorHigh
	case morale < MoraleLow:
		return MoraleFactorLow
	default:
		return MoraleFactorMid
	}
}

// Building priorities (Building.Priority).
const (
	// PriorityLow is served last for staff and power.
	PriorityLow = 0
	// PriorityNormal is the default priority.
	PriorityNormal = 1
	// PriorityHigh is served first; Life and Defence buildings default to it.
	PriorityHigh = 2
)

// Construction, demolition and repair.
const (
	// PowerDeficitBuildMul scales construction speed while the colony is in
	// power deficit.
	PowerDeficitBuildMul = 0.5
	// CancelRefundFrac is the fraction of the cost refunded when a building
	// under construction is cancelled.
	CancelRefundFrac = 1.0
	// DemolishRefundFrac is the fraction of the Alloy, Silica and Parts cost
	// refunded when a completed building is demolished (capped by storage).
	DemolishRefundFrac = 0.5
	// SalvageRefundFrac is the fraction of the Alloy, Silica and Parts cost
	// refunded when the rock's owner demolishes a derelict.
	SalvageRefundFrac = 0.3
	// RecommissionCostFrac is the fraction of the original cost paid to
	// reactivate a derelict.
	RecommissionCostFrac = 0.4
	// ConstructionMinHPFrac is the minimum HP of a building under
	// construction as a fraction of its max HP (HP is otherwise proportional
	// to progress).
	ConstructionMinHPFrac = 0.2
	// SelfRepairRate is the HP per game-second a completed building regains
	// on its own while the colony is quiet.
	SelfRepairRate = 0.5
	// SelfRepairQuietTicks is how long a colony must not have been attacked
	// before its buildings self-repair (1 cycle).
	SelfRepairQuietTicks = TicksPerCycle
)

// RefundGoods returns the goods that demolition and salvage refunds cover:
// Alloy, Silica and Parts. Use it with Stock.Only.
func RefundGoods() [3]Good { return [3]Good{Alloy, Silica, Parts} }

// Ships and fleets.
const (
	// MaxShipQueue is the maximum number of ships queued in one Launch
	// Cradle.
	MaxShipQueue = 5
	// ArriveRadius is the distance, in u, at which a fleet in transit (or a
	// slug) has arrived at its destination rock.
	ArriveRadius = 6.0
	// SurveySeconds is how long a Prospector must orbit a rock to survey it.
	SurveySeconds = 15.0
	// RouteDwellSeconds is how long a hauling fleet stays at each stop.
	RouteDwellSeconds = 5.0
	// LaunchRetrySeconds is how long a hauling fleet waits before retrying a
	// launch that was refused for lack of Fuel.
	LaunchRetrySeconds = 5.0
	// SeedshipColonists is the number of colonists a Seedship takes from the
	// colony that builds it and lands on the new rock.
	SeedshipColonists = 12.0
	// SeedshipMinPop is the population the building colony needs before a
	// Seedship may be queued.
	SeedshipMinPop = 20.0
	// DefaultShipSensorRange is the sensor radius, in u, of every ship
	// without a dedicated sensor suite.
	DefaultShipSensorRange = 120.0

	// RetreatNever is the Fleet.RetreatAt value of a fleet that never
	// retreats.
	RetreatNever = 0.0
	// RetreatQuarter makes a fleet retreat below 25 % of its max HP.
	RetreatQuarter = 0.25
	// RetreatHalf makes a fleet retreat below 50 % of its max HP.
	RetreatHalf = 0.5
)

// RetreatLevels returns the allowed Fleet.RetreatAt values in increasing
// order: RetreatNever, RetreatQuarter, RetreatHalf.
func RetreatLevels() [3]float64 {
	return [3]float64{RetreatNever, RetreatQuarter, RetreatHalf}
}

// ValidRetreat reports whether v is one of the allowed Fleet.RetreatAt
// values.
func ValidRetreat(v float64) bool {
	return v == RetreatNever || v == RetreatQuarter || v == RetreatHalf
}

// SeedshipStock returns the starter stock of a colony founded by a Seedship:
// Alloy 80, Silica 60, Food 50, Ice 40.
func SeedshipStock() Stock {
	return Stock{Alloy: 80, Silica: 60, Food: 50, Ice: 40}
}

// Combat.
const (
	// DamageJitterMin is the lower bound of the random damage multiplier.
	DamageJitterMin = 0.9
	// DamageJitterMax is the upper bound of the random damage multiplier.
	DamageJitterMax = 1.1
	// KeystoneBounty is the Scrip paid to the faction that destroys a
	// Keystone.
	KeystoneBounty = 250.0
	// SlugSpeed is the speed of a Mass Driver slug in u per game-second.
	SlugSpeed = 60.0
)

// Research.
const (
	// MaxResearchQueue is the maximum number of techs queued behind the
	// current project.
	MaxResearchQueue = 5
)

// Market (Exchange).
const (
	// MarketBuyMul is the mark-up paid when buying: cost = price * 1.12.
	MarketBuyMul = 1.12
	// MarketSellMul is the mark-down received when selling:
	// revenue = price * 0.88.
	MarketSellMul = 0.88
	// MarketPressurePerUnit is the pressure change per unit traded (up when
	// bought, down when sold).
	MarketPressurePerUnit = 1.0 / 2000.0
	// MarketPressureMax is the clamp of market pressure (plus or minus).
	MarketPressureMax = 0.6
	// MarketDecayPerSecond is the fraction by which pressure decays toward 0
	// per game-second.
	MarketDecayPerSecond = 0.015
	// MarketWalkStep is the largest random change of a good's pressure per
	// slow step (the "small random walk"): a uniform value in
	// [-MarketWalkStep, +MarketWalkStep].
	MarketWalkStep = 0.004
	// MaxTradeUnits is the maximum number of units per trade command.
	MaxTradeUnits = 200.0
	// SponsorCost is the Scrip price of sponsoring settlers.
	SponsorCost = 120.0
	// SponsorColonists is the number of colonists a sponsorship adds.
	SponsorColonists = 5.0
	// SponsorCooldownTicks is the minimum time between two sponsorships at
	// the same rock (1 cycle).
	SponsorCooldownTicks = TicksPerCycle
	// ExpediteScripPerSecond is the base Scrip price per remaining
	// game-second of construction when expediting.
	ExpediteScripPerSecond = 1.5
	// ExpediteAlloyDivisor scales the expedite price with the building's
	// Alloy cost: price = ceil(ExpediteScripPerSecond * remainingSeconds *
	// (1 + alloyCost/ExpediteAlloyDivisor)).
	ExpediteAlloyDivisor = 100.0
)

// ExpediteCost returns the Scrip price of finishing a construction with
// remainingSeconds of build time left for a building whose Alloy cost is
// alloyCost: ceil(1.5 * remainingSeconds * (1 + alloyCost/100)). It is 0 when
// nothing remains. A tiny tolerance keeps floating-point error from pushing
// an exact result up to the next whole Scrip (1.5 x 60 x 2.2 is 198, not
// 199).
func ExpediteCost(remainingSeconds, alloyCost float64) float64 {
	if remainingSeconds <= 0 {
		return 0
	}
	raw := ExpediteScripPerSecond * remainingSeconds * (1 + alloyCost/ExpediteAlloyDivisor)
	return math.Ceil(raw - StockEpsilon)
}

// Random events.
const (
	// EventFirstCycle is the first cycle in which an event may happen.
	EventFirstCycle = 4
	// EventGapMinCycles is the smallest gap between two event rolls, in
	// cycles.
	EventGapMinCycles = 3.0
	// EventGapMaxCycles is the largest gap between two event rolls, in
	// cycles.
	EventGapMaxCycles = 5.0

	// SquallWarnSeconds is how long before the first impact a squall is
	// announced.
	SquallWarnSeconds = 20.0
	// SquallSeconds is the duration of the impact phase of a squall.
	SquallSeconds = 10.0
	// SquallImpactsMin is the smallest number of impacts of a squall.
	SquallImpactsMin = 8
	// SquallImpactsMax is the largest number of impacts of a squall.
	SquallImpactsMax = 14
	// SquallDamageMin is the smallest damage of one impact.
	SquallDamageMin = 40.0
	// SquallDamageMax is the largest damage of one impact.
	SquallDamageMax = 80.0

	// FlareSeconds is the duration of a solar flare.
	FlareSeconds = 45.0
	// FlareSolarMul multiplies Sunvane output during a flare.
	FlareSolarMul = 1.6
	// FlareSensorMul multiplies every sensor range during a flare.
	FlareSensorMul = 0.5
	// FlareHullLossFrac is the fraction of its max HP each ship in transit
	// loses in total over a whole flare (never dropping below 1 HP).
	FlareHullLossFrac = 0.10

	// SeamAmountMin is the smallest amount a rich seam adds to a deposit.
	SeamAmountMin = 500.0
	// SeamAmountMax is the largest amount a rich seam adds to a deposit.
	SeamAmountMax = 900.0

	// SalvageLifetimeTicks is how long a derelict hulk's salvage stays in
	// orbit (4 cycles).
	SalvageLifetimeTicks = 4 * TicksPerCycle
	// SalvageClaimSeconds is how long the rock's owner, or the first faction
	// with a fleet in orbit, must wait before collecting salvage.
	SalvageClaimSeconds = 10.0

	// RefugeesMin is the smallest size of a refugee convoy.
	RefugeesMin = 4
	// RefugeesMax is the largest size of a refugee convoy.
	RefugeesMax = 8

	// MarketShockPressure is the size of the pressure jump (up or down) of a
	// market shock.
	MarketShockPressure = 0.4
)

// Log and FX buffers.
const (
	// MaxLogEntries is the cap of World.Log; the oldest entries are dropped.
	MaxLogEntries = 300
	// MaxFX is the cap of World.FX; when an undrained buffer reaches it the
	// oldest half is dropped (FX are cosmetic and never read by the sim).
	MaxFX = 2048
)
