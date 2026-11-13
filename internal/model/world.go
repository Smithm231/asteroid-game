package model

import (
	"encoding/json"
	"fmt"
	"slices"
)

// GameSettings are the options a game is created with. They are stored in
// the world unchanged for the whole game.
type GameSettings struct {
	// Seed seeds world generation and the simulation RNG.
	Seed uint64
	// BeltSize selects the number of rocks.
	BeltSize BeltSize
	// Rivals is the number of AI rivals, MinRivals..MaxRivals.
	Rivals int
	// Difficulty scales AI production.
	Difficulty Difficulty
	// PlayerName is the name of the human player's Hold.
	PlayerName string
	// RivalKinds optionally fixes the personality of each rival, in faction
	// order; missing or PersonalityNone entries are chosen at random.
	RivalKinds []Personality
	// Events enables random belt events.
	Events bool
	// Directives enables the onboarding directives for the human player.
	Directives bool
}

// DefaultSettings returns the settings of a standard game: the given seed,
// a Medium belt, two rivals of random personality, Normal difficulty, the
// default player name, events and directives on.
func DefaultSettings(seed uint64) GameSettings {
	return GameSettings{
		Seed:       seed,
		BeltSize:   BeltMedium,
		Rivals:     2,
		Difficulty: DiffNormal,
		PlayerName: DefaultPlayerName,
		Events:     true,
		Directives: true,
	}
}

// Normalized returns a copy of the settings with every field forced into
// its valid range: unknown belt sizes become Medium, unknown difficulties
// Normal, Rivals is clamped to MinRivals..MaxRivals, an empty PlayerName
// becomes DefaultPlayerName, and RivalKinds is cut to Rivals entries with
// invalid personalities replaced by PersonalityNone (= random). The slice in
// the result never aliases the receiver's.
func (s GameSettings) Normalized() GameSettings {
	if !s.BeltSize.Valid() {
		s.BeltSize = BeltMedium
	}
	if !s.Difficulty.Valid() {
		s.Difficulty = DiffNormal
	}
	s.Rivals = min(max(s.Rivals, MinRivals), MaxRivals)
	if s.PlayerName == "" {
		s.PlayerName = DefaultPlayerName
	}
	var kinds []Personality
	for i, p := range s.RivalKinds {
		if i >= s.Rivals {
			break
		}
		if !p.Valid() {
			p = PersonalityNone
		}
		kinds = append(kinds, p)
	}
	s.RivalKinds = kinds
	return s
}

// Squall is a scheduled or ongoing micrometeor squall.
type Squall struct {
	// Rock is the colonised rock being hit.
	Rock RockID
	// StartTick is the tick of the first impact (the warning is logged
	// SquallWarnSeconds earlier, when the squall is scheduled).
	StartTick int
	// EndTick is the tick at which the impact phase ends.
	EndTick int
	// ImpactsLeft is the number of impacts still to come.
	ImpactsLeft int
	// NextImpactTick is the tick of the next impact.
	NextImpactTick int
}

// EventState is the persistent state of the random event system.
type EventState struct {
	// NextRollTick is the tick of the next event roll (0 = not scheduled
	// yet).
	NextRollTick int
	// FlareStartTick is the tick at which the current or most recent solar
	// flare began.
	FlareStartTick int
	// FlareEndTick is the tick at which the current solar flare ends; a
	// flare is active while Tick < FlareEndTick (see World.FlareActive).
	FlareEndTick int
	// Squalls are the scheduled and ongoing micrometeor squalls.
	Squalls []Squall
	// LastKind is the kind of the most recent event (EventNone = none yet).
	LastKind EventKind
	// LastTick is the tick of the most recent event.
	LastTick int
	// Count is the number of events that have happened so far.
	Count int
}

// World is the complete state of one game. Everything reachable from it is
// plain data that round-trips through encoding/json; FX is the only field
// that is not saved.
//
// Invariants kept by the helpers of this package and checked by Validate:
// Rocks[i].ID == i; Factions[i].ID == i+1; Fleets is sorted by ascending
// ID; every Rock.Buildings slice is sorted by ascending ID and mirrored by
// Tile.Building.
type World struct {
	// Version is the save-format version (SaveVersion).
	Version int
	// Settings are the options the game was created with.
	Settings GameSettings
	// RNG is the single source of simulation randomness.
	RNG RNG
	// Tick is the number of sim ticks elapsed (TicksPerSecond per
	// game-second).
	Tick int
	// Rocks are the rocks of the belt; a rock's ID is its index.
	Rocks []*Rock
	// Factions are the Holds; faction f is Factions[f-1]. Factions[0] is the
	// human player.
	Factions []*Faction
	// Fleets are all fleets, sorted by ascending ID. Use AddFleet and
	// RemoveFleet.
	Fleets []*Fleet
	// Slugs are the Mass Driver projectiles in flight, in firing order.
	Slugs []Slug
	// Market is the Exchange state.
	Market Market
	// Events is the random event state.
	Events EventState
	// Pending are the commands to execute at the start of the next tick, in
	// order.
	Pending []Command
	// Log is the event log, oldest first, capped at MaxLogEntries. Use
	// Logf.
	Log []LogEntry
	// LogSeq is the Seq of the most recent log entry ever written.
	LogSeq int
	// FX are transient visual / audio cues since the application last
	// drained them. Not saved, never read by the simulation. Use Emit.
	FX []FX `json:"-"`
	// NextBuildingID is the next BuildingID NewBuildingID hands out.
	NextBuildingID BuildingID
	// NextFleetID is the next FleetID NewFleetID hands out.
	NextFleetID FleetID
	// NextShipID is the next ShipID NewShipID hands out.
	NextShipID ShipID
	// GameOver is true once the game has ended.
	GameOver bool
	// Winner is the winning faction once GameOver is set, or NoFaction when
	// the game ended without one (the human player was eliminated while
	// several rivals remain).
	Winner FactionID
	// GameOverTick is the tick at which the game ended (0 while running).
	GameOverTick int
}

// NewWorld returns an empty world for the given settings: Version set to
// SaveVersion, the RNG seeded from Settings.Seed, ID counters at 1, and no
// rocks, factions or fleets. World generation fills in the rest.
func NewWorld(settings GameSettings) *World {
	return &World{
		Version:        SaveVersion,
		Settings:       settings,
		RNG:            NewRNG(settings.Seed),
		NextBuildingID: 1,
		NextFleetID:    1,
		NextShipID:     1,
	}
}

// Clone returns a deep copy of the world made by a JSON round trip. FX are
// not copied. It is meant for tests, tools and look-ahead, not for per-tick
// use.
func (w *World) Clone() (*World, error) {
	data, err := json.Marshal(w)
	if err != nil {
		return nil, err
	}
	c := new(World)
	if err := json.Unmarshal(data, c); err != nil {
		return nil, err
	}
	return c, nil
}

// ---------------------------------------------------------------------------
// Time
// ---------------------------------------------------------------------------

// Cycle returns the displayed date: Tick/TicksPerCycle + 1.
func (w *World) Cycle() int { return CycleOfTick(w.Tick) }

// Seconds returns the elapsed game time in game-seconds.
func (w *World) Seconds() float64 { return float64(w.Tick) / TicksPerSecond }

// SlowTick reports whether the slow systems (construction, economy,
// research, market, events, directives) run on the current tick.
func (w *World) SlowTick() bool { return w.Tick%SlowTickInterval == 0 }

// AITick reports whether the AI of faction f thinks on the current tick:
// (Tick + AIStagger*f) % AIInterval == 0.
func (w *World) AITick(f FactionID) bool {
	return (w.Tick+AIStagger*int(f))%AIInterval == 0
}

// FlareActive reports whether a solar flare is in progress.
func (w *World) FlareActive() bool { return w.Tick < w.Events.FlareEndTick }

// SensorMul returns the factor applied to every sensor range right now:
// FlareSensorMul during a solar flare, 1 otherwise.
func (w *World) SensorMul() float64 {
	if w.FlareActive() {
		return FlareSensorMul
	}
	return 1
}

// FlareBonus returns the "flare bonus" factor applied to Sunvane output
// right now: FlareSolarMul during a solar flare, 1 otherwise. (The tech
// multiplier is the separate Mods.SolarMul.)
func (w *World) FlareBonus() float64 {
	if w.FlareActive() {
		return FlareSolarMul
	}
	return 1
}

// ---------------------------------------------------------------------------
// Lookups
// ---------------------------------------------------------------------------

// Rock returns the rock with the given ID, or nil when there is none (also
// for NoRock and on a nil world).
func (w *World) Rock(id RockID) *Rock {
	if w == nil || id < 0 || int(id) >= len(w.Rocks) {
		return nil
	}
	return w.Rocks[id]
}

// Faction returns the faction with the given ID, or nil when there is none
// (also for NoFaction and on a nil world).
func (w *World) Faction(id FactionID) *Faction {
	if w == nil || id < 1 || int(id) > len(w.Factions) {
		return nil
	}
	return w.Factions[id-1]
}

// Human returns the human player's faction (PlayerFaction), or nil in a
// world without factions.
func (w *World) Human() *Faction { return w.Faction(PlayerFaction) }

// fleetIndex returns the position of fleet id in the sorted Fleets slice
// and whether it is present.
func (w *World) fleetIndex(id FleetID) (int, bool) {
	return slices.BinarySearchFunc(w.Fleets, id, func(f *Fleet, id FleetID) int {
		switch {
		case f.ID < id:
			return -1
		case f.ID > id:
			return 1
		default:
			return 0
		}
	})
}

// Fleet returns the fleet with the given ID, or nil when there is none.
func (w *World) Fleet(id FleetID) *Fleet {
	if w == nil || id <= 0 {
		return nil
	}
	if i, ok := w.fleetIndex(id); ok {
		return w.Fleets[i]
	}
	return nil
}

// Building returns the building with the given ID on the given rock, or nil
// when the rock or the building does not exist.
func (w *World) Building(rock RockID, id BuildingID) *Building {
	return w.Rock(rock).BuildingByID(id)
}

// Colony returns the colony on the given rock, or nil when the rock does
// not exist or is unowned.
func (w *World) Colony(rock RockID) *Colony {
	r := w.Rock(rock)
	if r == nil {
		return nil
	}
	return r.Colony
}

// Mods returns the modifiers of faction f, or DefaultMods when there is no
// such faction (for example for derelicts with owner 0).
func (w *World) Mods(f FactionID) Mods {
	if fac := w.Faction(f); fac != nil {
		return fac.Mods
	}
	return DefaultMods()
}

// ---------------------------------------------------------------------------
// ID counters
// ---------------------------------------------------------------------------

// NewBuildingID returns a fresh, never-used BuildingID (1, 2, 3, ...).
func (w *World) NewBuildingID() BuildingID {
	if w.NextBuildingID < 1 {
		w.NextBuildingID = 1
	}
	id := w.NextBuildingID
	w.NextBuildingID++
	return id
}

// NewFleetID returns a fresh, never-used FleetID (1, 2, 3, ...).
func (w *World) NewFleetID() FleetID {
	if w.NextFleetID < 1 {
		w.NextFleetID = 1
	}
	id := w.NextFleetID
	w.NextFleetID++
	return id
}

// NewShipID returns a fresh, never-used ShipID (1, 2, 3, ...).
func (w *World) NewShipID() ShipID {
	if w.NextShipID < 1 {
		w.NextShipID = 1
	}
	id := w.NextShipID
	w.NextShipID++
	return id
}

// ---------------------------------------------------------------------------
// Fleets
// ---------------------------------------------------------------------------

// AddFleet inserts f into Fleets, keeping the slice sorted by ID, and
// returns f. A fleet with ID 0 gets a fresh ID first. A fleet whose ID is
// already present replaces the stored one. A nil fleet is ignored.
func (w *World) AddFleet(f *Fleet) *Fleet {
	if f == nil {
		return nil
	}
	if f.ID <= 0 {
		f.ID = w.NewFleetID()
	} else if f.ID >= w.NextFleetID {
		w.NextFleetID = f.ID + 1
	}
	i, found := w.fleetIndex(f.ID)
	if found {
		w.Fleets[i] = f
		return f
	}
	w.Fleets = slices.Insert(w.Fleets, i, f)
	return f
}

// RemoveFleet removes the fleet with the given ID from Fleets and returns
// it, or nil when there is no such fleet. The order of the remaining fleets
// is kept.
func (w *World) RemoveFleet(id FleetID) *Fleet {
	if w == nil || id <= 0 {
		return nil
	}
	i, found := w.fleetIndex(id)
	if !found {
		return nil
	}
	f := w.Fleets[i]
	w.Fleets = slices.Delete(w.Fleets, i, i+1)
	return f
}

// FleetsAt returns the fleets in orbit of the given rock, in ID order.
func (w *World) FleetsAt(rock RockID) []*Fleet {
	if w == nil || rock == NoRock {
		return nil
	}
	var out []*Fleet
	for _, f := range w.Fleets {
		if f.At == rock {
			out = append(out, f)
		}
	}
	return out
}

// FleetsOf returns the fleets owned by faction f, in ID order.
func (w *World) FleetsOf(f FactionID) []*Fleet {
	if w == nil || f == NoFaction {
		return nil
	}
	var out []*Fleet
	for _, fl := range w.Fleets {
		if fl.Owner == f {
			out = append(out, fl)
		}
	}
	return out
}

// GarrisonOf returns faction f's garrison fleet in orbit of the given rock,
// or nil when it has none there.
func (w *World) GarrisonOf(rock RockID, f FactionID) *Fleet {
	if w == nil || rock == NoRock || f == NoFaction {
		return nil
	}
	for _, fl := range w.Fleets {
		if fl.Garrison && fl.At == rock && fl.Owner == f {
			return fl
		}
	}
	return nil
}

// FleetPos returns the current belt position of a fleet: the position of
// the rock it orbits, or Fleet.Pos while in transit.
func (w *World) FleetPos(f *Fleet) Vec2 {
	if f == nil {
		return Vec2{}
	}
	if r := w.Rock(f.At); r != nil {
		return r.PosAt(w.Tick)
	}
	return f.Pos
}

// ShipCount returns the number of ships faction f has: ships in existence
// plus ships queued in its Launch Cradles.
func (w *World) ShipCount(f FactionID) int {
	if w == nil || f == NoFaction {
		return 0
	}
	n := 0
	for _, fl := range w.Fleets {
		if fl.Owner == f {
			n += len(fl.Ships)
		}
	}
	for _, r := range w.Rocks {
		if r.Owner != f {
			continue
		}
		for _, b := range r.Buildings {
			if b.Owner == f {
				n += len(b.Queue)
			}
		}
	}
	return n
}

// BerthsUsed returns the number of berths faction f occupies: the Berths of
// every ship in existence plus those of every ship queued in its Launch
// Cradles. It may not exceed BerthCap.
func (w *World) BerthsUsed(f FactionID) int {
	if w == nil || f == NoFaction {
		return 0
	}
	n := 0
	for _, fl := range w.Fleets {
		if fl.Owner == f {
			n += fl.Berths()
		}
	}
	for _, r := range w.Rocks {
		if r.Owner != f {
			continue
		}
		for _, b := range r.Buildings {
			if b.Owner != f {
				continue
			}
			for _, k := range b.Queue {
				n += ShipDefOf(k).Berths
			}
		}
	}
	return n
}

// BerthCap returns faction f's berth capacity: the sum, over the rocks it
// owns, of the Berths of its operational buildings (4 per Keystone, 6 per
// operational Dock Ring).
func (w *World) BerthCap(f FactionID) int {
	if w == nil || f == NoFaction {
		return 0
	}
	n := 0
	for _, r := range w.Rocks {
		if r.Owner != f {
			continue
		}
		for _, b := range r.Buildings {
			if b.Owner == f && b.Operational() {
				n += BuildingDefOf(b.Kind).Berths
			}
		}
	}
	return n
}

// BerthsFree returns BerthCap - BerthsUsed for faction f (negative when the
// faction lost capacity and is over its limit).
func (w *World) BerthsFree(f FactionID) int {
	return w.BerthCap(f) - w.BerthsUsed(f)
}

// ---------------------------------------------------------------------------
// Rocks and faction-wide stock
// ---------------------------------------------------------------------------

// RocksOf returns the rocks owned by faction f, in ID order.
func (w *World) RocksOf(f FactionID) []*Rock {
	if w == nil || f == NoFaction {
		return nil
	}
	var out []*Rock
	for _, r := range w.Rocks {
		if r.Owner == f {
			out = append(out, r)
		}
	}
	return out
}

// ColonyCount returns the number of rocks faction f owns.
func (w *World) ColonyCount(f FactionID) int {
	if w == nil || f == NoFaction {
		return 0
	}
	n := 0
	for _, r := range w.Rocks {
		if r.Owner == f {
			n++
		}
	}
	return n
}

// TotalPop returns the total population of faction f's colonies.
func (w *World) TotalPop(f FactionID) float64 {
	if w == nil || f == NoFaction {
		return 0
	}
	t := 0.0
	for _, r := range w.Rocks {
		if r.Owner == f && r.Colony != nil {
			t += r.Colony.Pop
		}
	}
	return t
}

// FactionTotal returns how much of good g faction f holds across all its
// colonies (ship cargo is not counted).
func (w *World) FactionTotal(f FactionID, g Good) float64 {
	if w == nil || f == NoFaction || !g.Valid() {
		return 0
	}
	t := 0.0
	for _, r := range w.Rocks {
		if r.Owner == f && r.Colony != nil {
			t += r.Colony.Stock[g]
		}
	}
	return t
}

// FactionTake removes amount units of good g from faction f's colonies,
// taking from the colony with the most of it first (ties: lower rock ID
// first). It is all-or-nothing: when the faction holds less than amount in
// total, nothing is taken and it returns false. A non-positive amount
// succeeds trivially.
func (w *World) FactionTake(f FactionID, g Good, amount float64) bool {
	if !(amount > 0) {
		return true
	}
	if w == nil || f == NoFaction || !g.Valid() {
		return false
	}
	if w.FactionTotal(f, g)+StockEpsilon < amount {
		return false
	}
	rocks := w.RocksOf(f)
	slices.SortStableFunc(rocks, func(a, b *Rock) int {
		sa, sb := colonyStock(a, g), colonyStock(b, g)
		switch {
		case sa > sb:
			return -1
		case sa < sb:
			return 1
		default:
			return int(a.ID) - int(b.ID)
		}
	})
	left := amount
	for _, r := range rocks {
		if left <= StockEpsilon {
			break
		}
		left -= r.Colony.Take(g, left)
	}
	return true
}

// colonyStock returns the amount of g in r's colony, or 0 without a colony.
func colonyStock(r *Rock, g Good) float64 {
	if r == nil || r.Colony == nil {
		return 0
	}
	return r.Colony.Stock[g]
}

// RockPos returns the belt position of the given rock at the current tick,
// or the origin when the rock does not exist.
func (w *World) RockPos(id RockID) Vec2 {
	return w.Rock(id).PosAt(w.Tick)
}

// RockDist returns the distance in u between two rocks at the current tick,
// or 0 when either does not exist.
func (w *World) RockDist(a, b RockID) float64 {
	return w.RockDistAt(a, b, w.Tick)
}

// RockDistAt returns the distance in u between two rocks at the given tick,
// or 0 when either does not exist.
func (w *World) RockDistAt(a, b RockID, tick int) float64 {
	ra, rb := w.Rock(a), w.Rock(b)
	if ra == nil || rb == nil {
		return 0
	}
	return ra.PosAt(tick).Dist(rb.PosAt(tick))
}

// AliveCount returns the number of factions that are still alive.
func (w *World) AliveCount() int {
	if w == nil {
		return 0
	}
	n := 0
	for _, f := range w.Factions {
		if f.Alive {
			n++
		}
	}
	return n
}

// Hostile reports whether factions a and b are enemies. Every Hold is
// hostile to every other Hold; NoFaction is hostile to nobody.
func Hostile(a, b FactionID) bool {
	return a != NoFaction && b != NoFaction && a != b
}

// ---------------------------------------------------------------------------
// Commands, log and FX
// ---------------------------------------------------------------------------

// Enqueue appends a command to Pending; it is executed at the start of the
// next tick.
func (w *World) Enqueue(c Command) {
	w.Pending = append(w.Pending, c)
}

// Logf appends an entry to the log. faction is the addressee (NoFaction
// broadcasts to everyone), sev the severity, rock the place the entry is
// about (NoRock for none); format and args are as for fmt.Sprintf. The log
// is capped at MaxLogEntries; the oldest entries are dropped.
func (w *World) Logf(faction FactionID, sev Severity, rock RockID, format string, args ...any) {
	w.LogSeq++
	w.Log = append(w.Log, LogEntry{
		Seq:     w.LogSeq,
		Tick:    w.Tick,
		Faction: faction,
		Sev:     sev,
		Rock:    rock,
		Text:    fmt.Sprintf(format, args...),
	})
	if extra := len(w.Log) - MaxLogEntries; extra > 0 {
		n := copy(w.Log, w.Log[extra:])
		clear(w.Log[n:])
		w.Log = w.Log[:n]
	}
}

// Emit appends a transient visual / audio cue to FX. The simulation never
// reads FX; the application drains the slice after each step. If nobody
// drains it (headless runs) the buffer is bounded: on reaching MaxFX the
// oldest half is dropped.
func (w *World) Emit(fx FX) {
	if len(w.FX) >= MaxFX {
		n := copy(w.FX, w.FX[len(w.FX)-MaxFX/2:])
		w.FX = w.FX[:n]
	}
	w.FX = append(w.FX, fx)
}

// TakeFX returns the FX emitted since the last call and empties the buffer.
// The caller owns the returned slice. This is how the application drains FX
// after each step.
func (w *World) TakeFX() []FX {
	fx := w.FX
	w.FX = nil
	return fx
}
