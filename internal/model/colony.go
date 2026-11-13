package model

import "math"

// RateSmoothingSeconds is the time constant, in game-seconds, of the
// exponential smoothing applied to a colony's displayed production and
// consumption rates (see Colony.RollRates).
const RateSmoothingSeconds = 10.0

// Colony is the state of one owned rock: its stock, its people and the
// values the economy caches every slow step for other systems and the UI.
// A colony exists exactly while the rock has an owner (a Keystone).
type Colony struct {
	// Stock is the colony's goods. No good may exceed StorageCap.
	Stock Stock
	// Pop is the population (fractional; workers = floor(Pop)).
	Pop float64
	// Morale is the colony morale, MoraleMin..MoraleMax.
	Morale float64
	// FoundedTick is the tick at which the colony was founded.
	FoundedTick int

	// Housing is the cached housing capacity (Keystone + operational
	// Habitats).
	Housing float64
	// LifeSupport is the cached life-support capacity in colonists
	// (Keystone + Aerators scaled by efficiency).
	LifeSupport float64
	// StorageCap is the cached storage capacity, the same for every good
	// (Keystone + operational Silos).
	StorageCap float64
	// PowerSupply is the cached total power produced in the last economy
	// step.
	PowerSupply float64
	// PowerDemand is the cached total power wanted by enabled, completed
	// consumers in the last economy step.
	PowerDemand float64
	// Workers is the cached number of available workers, floor(Pop).
	Workers int
	// WorkersNeeded is the cached number of workers all enabled, completed
	// buildings would need for full staffing.
	WorkersNeeded int
	// WorkersUsed is the cached number of workers actually assigned.
	WorkersUsed int

	// Starving is true when the colony ran out of Food in the last economy
	// step.
	Starving bool
	// Suffocating is true when the colony had no Ice or too little
	// life-support capacity in the last economy step.
	Suffocating bool
	// MoraleTarget is the value Morale is currently drifting toward (cached
	// for the UI).
	MoraleTarget float64
	// CommonsCoverage is the cached share of the population covered by
	// Commons, 0..1.
	CommonsCoverage float64
	// PopRate is the cached net population change per cycle (positive =
	// growing), for the UI.
	PopRate float64

	// LastAttackedTick is the tick at which an enemy last damaged a building
	// of this colony; 0 means never. Use MarkAttacked and AttackedWithin.
	LastAttackedTick int
	// LastSponsorTick is the tick of the last settler sponsorship here;
	// 0 means never. Use SponsorReady.
	LastSponsorTick int

	// Produced is the smoothed local production rate per good, in units per
	// cycle (mining and building outputs; freight and trade excluded).
	Produced Stock
	// Consumed is the smoothed local consumption rate per good, in units per
	// cycle (building inputs, fuel burned for power, life support).
	Consumed Stock
	// ProducedAcc accumulates the units produced since the last RollRates.
	ProducedAcc Stock
	// ConsumedAcc accumulates the units consumed since the last RollRates.
	ConsumedAcc Stock
}

// NewColony returns a colony founded at the given tick with no stock and no
// people. Its capacity caches are preset to what a lone Keystone provides
// (housing, life support, storage, power), so that the colony is usable
// before the first economy step; Morale starts at StartMorale and
// MoraleTarget at the same value.
func NewColony(tick int) *Colony {
	k := buildingDefs[Keystone]
	return &Colony{
		Morale:       StartMorale,
		MoraleTarget: StartMorale,
		FoundedTick:  tick,
		Housing:      k.Housing,
		LifeSupport:  k.LifeSupport,
		StorageCap:   k.Storage,
		PowerSupply:  k.Power,
	}
}

// Free returns how many more units of good g fit into storage:
// max(0, StorageCap - Stock[g]). It is 0 for an invalid good.
func (c *Colony) Free(g Good) float64 {
	if c == nil || !g.Valid() {
		return 0
	}
	return math.Max(0, c.StorageCap-c.Stock[g])
}

// AddClamped adds up to amt units of good g, limited by free storage, and
// returns the amount actually added (0 for a non-positive amt or an invalid
// good).
func (c *Colony) AddClamped(g Good, amt float64) float64 {
	if c == nil || !g.Valid() || !(amt > 0) {
		return 0
	}
	if free := c.Free(g); amt > free {
		amt = free
	}
	c.Stock[g] += amt
	return amt
}

// AddStockClamped adds every good of s, each limited by free storage, and
// returns what was actually added. Use it for refunds, deliveries and
// salvage, which are all "capped by storage".
func (c *Colony) AddStockClamped(s Stock) Stock {
	var added Stock
	if c == nil {
		return added
	}
	for g := range s {
		added[g] = c.AddClamped(Good(g), s[g])
	}
	return added
}

// Take removes up to amt units of good g and returns the amount actually
// removed: min(amt, Stock[g]) (0 for a non-positive amt or an invalid good).
// The stock never goes negative.
func (c *Colony) Take(g Good, amt float64) float64 {
	if c == nil || !g.Valid() || !(amt > 0) {
		return 0
	}
	have := c.Stock[g]
	if have <= 0 {
		return 0
	}
	if amt >= have-StockEpsilon {
		// Taking (almost) everything: empty the bin exactly, so that no
		// floating-point dust is left behind.
		c.Stock[g] = 0
		return math.Min(amt, have)
	}
	c.Stock[g] = have - amt
	return amt
}

// CanAfford reports whether the stock covers cost.
func (c *Colony) CanAfford(cost Stock) bool {
	return c != nil && c.Stock.Has(cost)
}

// Pay deducts cost from the stock if the colony can afford all of it, and
// reports whether it did. Nothing is deducted on failure. Amounts are
// clamped at zero to absorb floating-point error.
func (c *Colony) Pay(cost Stock) bool {
	if !c.CanAfford(cost) {
		return false
	}
	c.Stock.Sub(cost)
	c.Stock.ClampMin0()
	return true
}

// PowerDeficit reports whether the cached power demand exceeds the cached
// supply. Construction is slowed while this is true.
func (c *Colony) PowerDeficit() bool {
	return c != nil && c.PowerDemand > c.PowerSupply+StockEpsilon
}

// IdleWorkers returns the cached number of workers without a job:
// max(0, Workers - WorkersUsed).
func (c *Colony) IdleWorkers() int {
	if c == nil {
		return 0
	}
	return max(0, c.Workers-c.WorkersUsed)
}

// MarkAttacked records that an enemy damaged the colony at the given tick.
func (c *Colony) MarkAttacked(tick int) {
	if c == nil {
		return
	}
	c.LastAttackedTick = max(tick, 1)
}

// AttackedWithin reports whether the colony was attacked less than window
// ticks before tick now. Use MoraleAttackWindowTicks for the morale penalty
// and SelfRepairQuietTicks for the self-repair lock-out.
func (c *Colony) AttackedWithin(now, window int) bool {
	return c != nil && c.LastAttackedTick > 0 && now-c.LastAttackedTick < window
}

// MarkSponsored records a settler sponsorship at the given tick.
func (c *Colony) MarkSponsored(tick int) {
	if c == nil {
		return
	}
	c.LastSponsorTick = max(tick, 1)
}

// SponsorReady reports whether settlers may be sponsored again at tick now:
// never sponsored before, or at least SponsorCooldownTicks ago.
func (c *Colony) SponsorReady(now int) bool {
	return c != nil && (c.LastSponsorTick <= 0 || now-c.LastSponsorTick >= SponsorCooldownTicks)
}

// NoteProduced adds amt units of good g to the production accumulator. Call
// it whenever local economy activity adds goods to Stock.
func (c *Colony) NoteProduced(g Good, amt float64) {
	if c == nil || !g.Valid() || !(amt > 0) {
		return
	}
	c.ProducedAcc[g] += amt
}

// NoteConsumed adds amt units of good g to the consumption accumulator.
// Call it whenever local economy activity removes goods from Stock.
func (c *Colony) NoteConsumed(g Good, amt float64) {
	if c == nil || !g.Valid() || !(amt > 0) {
		return
	}
	c.ConsumedAcc[g] += amt
}

// RollRates folds the accumulators into the smoothed per-cycle rates and
// clears them. dt is the time in game-seconds the accumulators cover (the
// economy calls RollRates(SlowDT) once per step). The rates follow an
// exponential moving average with time constant RateSmoothingSeconds.
func (c *Colony) RollRates(dt float64) {
	if c == nil || !(dt > 0) {
		return
	}
	alpha := Clamp01(dt / RateSmoothingSeconds)
	toCycle := SecondsPerCycle / dt
	for g := range c.Produced {
		c.Produced[g] += alpha * (c.ProducedAcc[g]*toCycle - c.Produced[g])
		c.Consumed[g] += alpha * (c.ConsumedAcc[g]*toCycle - c.Consumed[g])
	}
	c.ProducedAcc.Clear()
	c.ConsumedAcc.Clear()
}

// NetRate returns the smoothed net rate of good g in units per cycle:
// Produced - Consumed (0 for an invalid good).
func (c *Colony) NetRate(g Good) float64 {
	if c == nil || !g.Valid() {
		return 0
	}
	return c.Produced[g] - c.Consumed[g]
}
