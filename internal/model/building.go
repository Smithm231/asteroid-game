package model

// Building is one structure on a rock.
//
// The fields from Powered to PowerOut are caches written by the economy
// every slow step; other systems and the UI only read them. The remaining
// function-specific fields (Shield, Cooldown, Queue, ...) are meaningful
// only for the kinds named in their comments and stay zero otherwise.
type Building struct {
	// ID is the world-unique identifier.
	ID BuildingID
	// Kind is the building kind.
	Kind BuildingKind
	// Owner is the owning faction. It equals the rock's owner for every
	// building that is not Derelict; derelicts have Owner == NoFaction.
	Owner FactionID
	// Hex is the tile the building stands on.
	Hex Hex
	// HP is the current hit points. Under construction it is proportional
	// to Progress with a floor of ConstructionMinHPFrac x MaxHP.
	HP float64
	// MaxHP is the maximum hit points (the table value x the owner's
	// Mods.HPMul at the time it was set).
	MaxHP float64
	// State is the life-cycle state.
	State BuildState
	// Progress is the construction progress, 0..1 (1 once completed).
	Progress float64
	// Enabled is the owner's on/off switch. A disabled building uses no
	// power and no staff and has no effect.
	Enabled bool
	// Priority is the staffing and power priority: PriorityLow,
	// PriorityNormal or PriorityHigh.
	Priority int

	// Powered is true when the building received the power it needs in the
	// last economy step (always true for kinds that draw none).
	Powered bool
	// Workers is the number of workers assigned in the last economy step.
	Workers int
	// Staffed is assigned / needed workers, 0..1 (1 when none are needed).
	Staffed float64
	// Eff is the efficiency of the last economy step: Powered x Staffed x
	// morale factor for consumers, Staffed for power producers, and 0 when
	// the building is not operational.
	Eff float64
	// Idle says why the building did nothing, for the UI (IdleNone while
	// working).
	Idle IdleReason
	// PowerOut is the power the building actually contributed in the last
	// economy step: positive for supply, negative for draw, 0 when off.
	PowerOut float64

	// Shield is the current shield pool (Aegis Projector).
	Shield float64
	// Cooldown is the time in game-seconds until the building may act again
	// (Mass Driver: next slug).
	Cooldown float64
	// Target is the ship a turret is currently engaging (sticky until it is
	// destroyed or leaves), or 0 (Flak Battery, Lance Tower).
	Target ShipID
	// Queue is the ship build queue, first entry in production (Launch
	// Cradle). At most MaxShipQueue entries; the cost of every entry has
	// already been paid.
	Queue []ShipKind
	// QueueProgress is the build progress of Queue[0], 0..1.
	QueueProgress float64
	// DriverTarget is the rock a Mass Driver bombards, or NoRock.
	DriverTarget RockID
	// SurveyAcc accumulates survey time in game-seconds toward the next
	// revealed tile (Survey Mast).
	SurveyAcc float64
}

// NewBuilding returns a building of the given kind that has just been
// placed: under construction with zero progress, enabled, with the kind's
// default priority, MaxHP = table value x hpMul (pass the owner's
// Mods.HPMul; values <= 0 count as 1) and HP at the construction floor
// (ConstructionMinHPFrac x MaxHP). DriverTarget is NoRock. The caller
// supplies the ID (World.NewBuildingID) and adds it with Rock.AddBuilding.
func NewBuilding(id BuildingID, kind BuildingKind, owner FactionID, hex Hex, hpMul float64) *Building {
	def := BuildingDefOf(kind)
	maxHP := def.MaxHP * mulOr1(hpMul)
	return &Building{
		ID:           id,
		Kind:         kind,
		Owner:        owner,
		Hex:          hex,
		HP:           maxHP * ConstructionMinHPFrac,
		MaxHP:        maxHP,
		State:        StateConstructing,
		Enabled:      true,
		Priority:     def.DefaultPriority(),
		DriverTarget: NoRock,
	}
}

// NewActiveBuilding returns a completed building at full HP: like
// NewBuilding followed by Complete. Use it for Keystones and for worlds
// built by generation or tests.
func NewActiveBuilding(id BuildingID, kind BuildingKind, owner FactionID, hex Hex, hpMul float64) *Building {
	b := NewBuilding(id, kind, owner, hex, hpMul)
	b.Complete()
	b.HP = b.MaxHP
	return b
}

// Def returns the content-table entry of the building's kind.
func (b *Building) Def() BuildingDef {
	if b == nil {
		return BuildingDefOf(BuildingNone)
	}
	return BuildingDefOf(b.Kind)
}

// Complete marks a building under construction as finished: State becomes
// StateActive and Progress 1. HP is left alone, and so are the status caches
// (Powered, Staffed, Eff): they stay at their zero values until the next
// economy step evaluates the building, which in the canonical tick order
// happens right after construction in the same tick. An Aegis Projector
// starts with an empty pool.
func (b *Building) Complete() {
	if b == nil {
		return
	}
	b.State = StateActive
	b.Progress = 1
	b.Idle = IdleNone
}

// Completed reports whether construction has finished and the building is
// owned (State == StateActive).
func (b *Building) Completed() bool { return b != nil && b.State == StateActive }

// Operational reports whether the building is completed, owned and switched
// on. This is what the rules call an "active" building (an active Silo adds
// storage, an active Dock Ring adds berths, ...). It does not consider power
// or staff; use Working for that.
func (b *Building) Operational() bool {
	return b != nil && b.State == StateActive && b.Enabled
}

// Working reports whether the building is operational and ran with non-zero
// efficiency in the last economy step.
func (b *Building) Working() bool { return b.Operational() && b.Eff > 0 }

// Constructing reports whether the building is still under construction.
func (b *Building) Constructing() bool { return b != nil && b.State == StateConstructing }

// Derelict reports whether the building is an ownerless derelict.
func (b *Building) Derelict() bool { return b != nil && b.State == StateDerelict }

// Damaged reports whether a completed or derelict building is below its max
// HP.
func (b *Building) Damaged() bool {
	return b != nil && b.State != StateConstructing && b.HP < b.MaxHP
}

// HPFrac returns HP / MaxHP clamped to 0..1 (0 when MaxHP is not positive).
func (b *Building) HPFrac() float64 {
	if b == nil || !(b.MaxHP > 0) {
		return 0
	}
	return Clamp01(b.HP / b.MaxHP)
}

// ConstructionHP returns the HP a building under construction should have
// at its current Progress: MaxHP x max(Progress, ConstructionMinHPFrac).
func (b *Building) ConstructionHP() float64 {
	if b == nil {
		return 0
	}
	return b.MaxHP * Clamp(b.Progress, ConstructionMinHPFrac, 1)
}

// RemainingBuildSeconds returns the construction time left at full speed,
// in game-seconds: (1 - Progress) x BuildTime, or 0 once completed.
func (b *Building) RemainingBuildSeconds() float64 {
	if b == nil || b.State != StateConstructing {
		return 0
	}
	return Clamp01(1-b.Progress) * BuildingDefOf(b.Kind).BuildTime
}

// MakeDerelict turns the building into an ownerless derelict, as happens to
// every surviving building when a colony falls: Owner becomes NoFaction,
// State StateDerelict, and all activity state (status caches, shield, queue,
// targets) is cleared. HP is kept.
func (b *Building) MakeDerelict() {
	if b == nil {
		return
	}
	b.Owner = NoFaction
	b.State = StateDerelict
	b.Powered = false
	b.Workers = 0
	b.Staffed = 0
	b.Eff = 0
	b.Idle = IdleNone
	b.PowerOut = 0
	b.Shield = 0
	b.Cooldown = 0
	b.Target = 0
	b.Queue = nil
	b.QueueProgress = 0
	b.DriverTarget = NoRock
	b.SurveyAcc = 0
}
