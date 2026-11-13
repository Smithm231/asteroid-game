package model

// LogEntry is one line of the event log (World.Log).
type LogEntry struct {
	// Seq is a number that increases by one with every entry ever logged in
	// this world (the first entry has Seq 1). The UI uses it to tell which
	// entries are new even after old ones were dropped.
	Seq int
	// Tick is the sim tick at which the entry was logged.
	Tick int
	// Faction is the addressee; NoFaction (0) broadcasts to every faction.
	Faction FactionID
	// Sev is the severity.
	Sev Severity
	// Rock is the place the entry is about, or NoRock; the UI can jump
	// there.
	Rock RockID
	// Text is the finished, user-readable message.
	Text string
}

// VisibleTo reports whether faction f should see the entry: it is a
// broadcast or addressed to f.
func (e LogEntry) VisibleTo(f FactionID) bool {
	return e.Faction == NoFaction || e.Faction == f
}

// FX is a transient visual / audio cue. The simulation appends FX with
// World.Emit and never reads them; the application drains World.FX after
// every step. FX are not saved. Which fields are meaningful depends on Kind
// (see the FXKind constants); unused fields are zero, except that an unused
// Rock is NoRock when the FX was built with NewFX.
type FX struct {
	// Kind says what happened.
	Kind FXKind
	// Rock is the rock where it happened, or NoRock.
	Rock RockID
	// Hex is the tile on Rock, when relevant.
	Hex Hex
	// Faction is the faction that caused or owns the effect, when relevant.
	Faction FactionID
	// Fleet is the fleet involved, when relevant.
	Fleet FleetID
	// A is a belt position (source), when relevant.
	A Vec2
	// B is a belt position (target), when relevant.
	B Vec2
	// Amount is a magnitude (damage, units, an ID) whose meaning depends on
	// Kind.
	Amount float64
}

// NewFX returns an FX of the given kind at a rock tile, owned by faction f.
// Pass NoRock when the effect is not tied to a rock. Set the remaining
// fields (Fleet, A, B, Amount) on the result as needed.
func NewFX(kind FXKind, rock RockID, hex Hex, f FactionID) FX {
	return FX{Kind: kind, Rock: rock, Hex: hex, Faction: f}
}
