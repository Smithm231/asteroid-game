package model

// Research is a faction's research state.
type Research struct {
	// Current is the tech being researched, or TechNone.
	Current TechID
	// Progress holds the research points accumulated per tech. Progress on a
	// tech is kept when the player switches to another project.
	Progress [NumTechs]float64
	// Started records, per tech, that research on it has begun and its
	// Lumen price (if any) has been paid; the price is paid only once.
	Started [NumTechs]bool
	// Queue lists the techs to research after Current, in order; at most
	// MaxResearchQueue entries.
	Queue []TechID
	// Stalled is true while Current cannot start because its Lumen price
	// cannot be paid.
	Stalled bool
	// Rate is the cached research output in points per cycle, for the UI.
	Rate float64
}

// DirectiveState is a faction's progress through the onboarding directives.
// The directive definitions live in the directives package; the model only
// stores the progress.
type DirectiveState struct {
	// Done holds one flag per directive, in directive order. The directives
	// package sizes it; a missing entry counts as not done.
	Done []bool
	// Current is the index of the first directive that is not done; it
	// equals the number of directives once all are done.
	Current int
	// LastDoneTick is the tick at which the most recent directive was
	// completed (0 = none yet), for UI flourishes.
	LastDoneTick int
}

// IsDone reports whether directive i has been completed.
func (d *DirectiveState) IsDone(i int) bool {
	return d != nil && i >= 0 && i < len(d.Done) && d.Done[i]
}

// Stats are a faction's running statistics. They are informational (end
// screens, balance tooling) and also feed some directive predicates, so the
// systems named below must keep them up to date.
type Stats struct {
	// ShipsBuilt counts ships completed by this faction's cradles (fleet).
	ShipsBuilt int
	// ShipsLost counts this faction's ships destroyed by any cause
	// (combat, fleet for scuttling).
	ShipsLost int
	// ShipsDestroyed counts enemy ships this faction destroyed (combat).
	ShipsDestroyed int
	// BuildingsBuilt counts constructions this faction completed
	// (construct).
	BuildingsBuilt int
	// BuildingsLost counts this faction's buildings destroyed by enemies or
	// events (combat, events).
	BuildingsLost int
	// BuildingsDestroyed counts enemy buildings this faction destroyed
	// (combat).
	BuildingsDestroyed int
	// ColoniesFounded counts colonies founded by Seedship (fleet); the
	// starting colony is not counted.
	ColoniesFounded int
	// ColoniesLost counts this faction's Keystones destroyed (combat).
	ColoniesLost int
	// KeystonesDestroyed counts enemy Keystones this faction destroyed
	// (combat).
	KeystonesDestroyed int
	// RocksSurveyed counts rocks fully surveyed by this faction's
	// Prospectors (fleet).
	RocksSurveyed int
	// TechsResearched counts completed technologies (research).
	TechsResearched int
	// SettlersSponsored counts colonists gained through sponsorship
	// (market).
	SettlersSponsored int
	// Mined is the total amount mined per good (economy).
	Mined Stock
	// Produced is the total amount produced by buildings per good (economy).
	Produced Stock
	// Bought is the total amount bought on the market per good (market).
	Bought Stock
	// Sold is the total amount sold on the market per good (market).
	Sold Stock
	// ScripEarned is the total Scrip income from all sources.
	ScripEarned float64
	// ScripSpent is the total Scrip spent.
	ScripSpent float64
	// DamageDealt is the total damage dealt to enemy ships and buildings
	// (combat).
	DamageDealt float64
	// DamageTaken is the total damage taken by own ships and buildings
	// (combat).
	DamageTaken float64
	// PeakPop is the highest total population reached (economy).
	PeakPop float64
}

// AIState is scratch memory for the AI of one faction. The model gives the
// fields no meaning beyond their names: the ai package decides how to use
// them. Everything is plain serialisable data (no maps), so an AI's plans
// survive a save and load. The rock and fleet fields are NoRock / 0 when
// unset (see NewFaction).
type AIState struct {
	// Phase is a coarse strategy phase counter.
	Phase int
	// Step is a sub-step or round-robin counter within the phase.
	Step int
	// Wave counts attack waves launched.
	Wave int
	// Timers are tick deadlines or timestamps, indexed by AI-defined slots.
	Timers []int
	// TargetRock is the rock currently marked for attack, or NoRock.
	TargetRock RockID
	// StagingRock is the rock where an attack force gathers, or NoRock.
	StagingRock RockID
	// ExpandRock is the rock currently marked for colonisation, or NoRock.
	ExpandRock RockID
	// StrikeFleet is the fleet currently carrying out the main attack, or 0.
	StrikeFleet FleetID
	// Rocks is a scratch list of rocks (for example survey targets).
	Rocks []RockID
	// Fleets is a scratch list of fleets (for example hauling fleets).
	Fleets []FleetID
	// Floats is a general-purpose float scratch area.
	Floats []float64
	// Ints is a general-purpose integer scratch area.
	Ints []int32
}

// Faction is one Hold: the human player (ID 1) or an AI rival.
type Faction struct {
	// ID is the faction's identifier; World.Factions[ID-1] is this faction.
	ID FactionID
	// Name is the Hold's display name.
	Name string
	// ColorIndex is the accent colour index (ColorAmber for the player,
	// ColorTeal.. for rivals).
	ColorIndex int
	// IsAI is true for computer-controlled factions.
	IsAI bool
	// Personality is the AI personality (PersonalityNone for humans).
	Personality Personality
	// Alive is false once the faction has lost its last Keystone.
	Alive bool
	// EliminatedTick is the tick at which the faction was eliminated
	// (0 while alive).
	EliminatedTick int
	// HomeRock is the faction's starting rock (NoRock if unset).
	HomeRock RockID
	// Scrip is the faction's currency.
	Scrip float64
	// Techs records which technologies are known.
	Techs [NumTechs]bool
	// Research is the research state.
	Research Research
	// Mods are the cached global modifiers; see ComputeMods and
	// RecomputeMods.
	Mods Mods
	// Directives is the onboarding progress (human player only).
	Directives DirectiveState
	// Stats are running statistics.
	Stats Stats
	// AI is scratch memory for the ai package (unused for humans).
	AI AIState
}

// NewFaction returns a living faction with the given identity and safe
// defaults: no Scrip, no techs, no current research (TechNone), neutral
// Mods, ColorIndex equal to the ID, HomeRock and the AI rock fields set to
// NoRock. IsAI is derived from the personality: any rival personality makes
// an AI faction.
func NewFaction(id FactionID, name string, p Personality) *Faction {
	return &Faction{
		ID:          id,
		Name:        name,
		ColorIndex:  int(id),
		IsAI:        p.IsRival(),
		Personality: p,
		Alive:       true,
		HomeRock:    NoRock,
		Research:    Research{Current: TechNone},
		Mods:        DefaultMods(),
		AI: AIState{
			TargetRock:  NoRock,
			StagingRock: NoRock,
			ExpandRock:  NoRock,
		},
	}
}

// Has reports whether the faction knows technology t. It is false for
// TechNone and invalid IDs.
func (f *Faction) Has(t TechID) bool {
	return f != nil && t.Valid() && f.Techs[t]
}

// CanBuild reports whether the faction has unlocked building kind k: it is
// start-unlocked or its unlock tech is known. It is always false for the
// Keystone, which is never built by command.
func (f *Faction) CanBuild(k BuildingKind) bool {
	if f == nil || !k.Valid() {
		return false
	}
	d := &buildingDefs[k]
	return d.StartUnlocked || f.Has(d.Unlock)
}

// CanBuildShip reports whether the faction has unlocked ship kind k.
func (f *Faction) CanBuildShip(k ShipKind) bool {
	if f == nil || !k.Valid() {
		return false
	}
	return f.Has(shipDefs[k].Unlock)
}

// PrereqsMet reports whether the faction knows every prerequisite of
// technology t.
func (f *Faction) PrereqsMet(t TechID) bool {
	if f == nil || !t.Valid() {
		return false
	}
	for _, req := range techDefs[t].Requires {
		if !f.Techs[req] {
			return false
		}
	}
	return true
}

// CanResearch reports whether technology t is available to research now:
// not yet known and all prerequisites known. It ignores the Lumen price and
// whether t is already current or queued.
func (f *Faction) CanResearch(t TechID) bool {
	return f != nil && t.Valid() && !f.Techs[t] && f.PrereqsMet(t)
}

// TechCount returns the number of technologies the faction knows.
func (f *Faction) TechCount() int {
	if f == nil {
		return 0
	}
	n := 0
	for _, known := range f.Techs {
		if known {
			n++
		}
	}
	return n
}

// UnlockedBuildings returns the building kinds the faction may build, in
// table order.
func (f *Faction) UnlockedBuildings() []BuildingKind {
	var out []BuildingKind
	for k := BuildingKind(0); k < NumBuildingKinds; k++ {
		if f.CanBuild(k) {
			out = append(out, k)
		}
	}
	return out
}

// UnlockedShips returns the ship kinds the faction may build, in table
// order.
func (f *Faction) UnlockedShips() []ShipKind {
	var out []ShipKind
	for k := ShipKind(0); k < NumShipKinds; k++ {
		if f.CanBuildShip(k) {
			out = append(out, k)
		}
	}
	return out
}

// AvailableTechs returns the technologies the faction could research now
// (see CanResearch), in table order.
func (f *Faction) AvailableTechs() []TechID {
	var out []TechID
	for t := TechID(0); t < NumTechs; t++ {
		if f.CanResearch(t) {
			out = append(out, t)
		}
	}
	return out
}

// RecomputeMods sets Mods from the faction's known techs, its IsAI flag and
// the game difficulty (see ComputeMods). Call it after granting a tech and
// when setting up a faction.
func (f *Faction) RecomputeMods(d Difficulty) {
	if f == nil {
		return
	}
	f.Mods = ComputeMods(f.Techs, f.IsAI, d)
}

// InQueue reports whether technology t is the current project or waits in
// the research queue.
func (f *Faction) InQueue(t TechID) bool {
	if f == nil || !t.Valid() {
		return false
	}
	if f.Research.Current == t {
		return true
	}
	for _, q := range f.Research.Queue {
		if q == t {
			return true
		}
	}
	return false
}

// ResearchFrac returns the progress of technology t as a fraction 0..1 of
// its cost (1 for known techs).
func (f *Faction) ResearchFrac(t TechID) float64 {
	if f == nil || !t.Valid() {
		return 0
	}
	if f.Techs[t] {
		return 1
	}
	cost := techDefs[t].Cost
	if !(cost > 0) {
		return 0
	}
	return Clamp01(f.Research.Progress[t] / cost)
}

// AddScrip adds amount to the faction's Scrip and to Stats.ScripEarned.
// Non-positive amounts are ignored.
func (f *Faction) AddScrip(amount float64) {
	if f == nil || !(amount > 0) {
		return
	}
	f.Scrip += amount
	f.Stats.ScripEarned += amount
}

// SpendScrip deducts amount from the faction's Scrip if it can afford it,
// adds it to Stats.ScripSpent and reports whether it paid. A non-positive
// amount always succeeds without changing anything.
func (f *Faction) SpendScrip(amount float64) bool {
	if f == nil {
		return false
	}
	if !(amount > 0) {
		return true
	}
	if f.Scrip+StockEpsilon < amount {
		return false
	}
	f.Scrip -= amount
	if f.Scrip < 0 {
		f.Scrip = 0
	}
	f.Stats.ScripSpent += amount
	return true
}
