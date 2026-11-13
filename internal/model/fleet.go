package model

import "math"

// Ship is one vessel inside a fleet.
type Ship struct {
	// ID is the world-unique identifier.
	ID ShipID
	// Kind is the ship kind.
	Kind ShipKind
	// HP is the current hit points.
	HP float64
	// MaxHP is the maximum hit points (the table value x the owner's
	// Mods.HPMul at the time it was set).
	MaxHP float64
	// Cargo is the goods carried; its total never exceeds the kind's Cargo
	// capacity.
	Cargo Stock
	// TargetShip is the enemy ship this ship is shooting at (sticky until
	// it is destroyed or leaves), or 0.
	TargetShip ShipID
	// TargetBuilding is the building this ship is bombarding (sticky until
	// it is destroyed), or 0.
	TargetBuilding BuildingID
}

// NewShip returns a ship of the given kind at full HP, with
// MaxHP = table value x hpMul (pass the owner's Mods.HPMul; values <= 0
// count as 1). The caller supplies the ID (World.NewShipID).
func NewShip(id ShipID, kind ShipKind, hpMul float64) Ship {
	hp := ShipDefOf(kind).MaxHP * mulOr1(hpMul)
	return Ship{ID: id, Kind: kind, HP: hp, MaxHP: hp}
}

// Def returns the content-table entry of the ship's kind.
func (s *Ship) Def() ShipDef {
	if s == nil {
		return ShipDefOf(ShipNone)
	}
	return ShipDefOf(s.Kind)
}

// CargoFree returns the unused cargo capacity of the ship.
func (s *Ship) CargoFree() float64 {
	if s == nil {
		return 0
	}
	return math.Max(0, ShipDefOf(s.Kind).Cargo-s.Cargo.Total())
}

// Route is a freight route between two rocks of the same faction. A hauling
// fleet loads the Out goods at A, flies to B, unloads, loads the Back goods,
// returns to A, unloads, and repeats.
type Route struct {
	// A is the first stop.
	A RockID
	// B is the second stop.
	B RockID
	// Out selects the goods carried from A to B.
	Out [NumGoods]bool
	// Back selects the goods carried from B to A.
	Back [NumGoods]bool
	// Reserve is the amount of each selected good that loading never takes
	// a stop's stock below.
	Reserve float64
}

// Other returns the stop of the route that is not rock: B for A, A for B,
// and NoRock when rock is neither.
func (r *Route) Other(rock RockID) RockID {
	switch {
	case r == nil:
		return NoRock
	case rock == r.A:
		return r.B
	case rock == r.B:
		return r.A
	default:
		return NoRock
	}
}

// Loads returns the goods to load at the given stop: Out at A, Back at B,
// nothing elsewhere.
func (r *Route) Loads(rock RockID) [NumGoods]bool {
	switch {
	case r == nil:
		return [NumGoods]bool{}
	case rock == r.A:
		return r.Out
	case rock == r.B:
		return r.Back
	default:
		return [NumGoods]bool{}
	}
}

// Fleet is a group of ships of one faction sharing a location and an order.
type Fleet struct {
	// ID is the fleet's identifier (assigned by World.AddFleet when 0).
	ID FleetID
	// Owner is the owning faction.
	Owner FactionID
	// Ships are the fleet's ships. A fleet without ships is removed.
	Ships []Ship
	// At is the rock the fleet orbits, or NoRock while in transit.
	At RockID
	// Pos is the fleet's belt position. In transit it is advanced every
	// tick; in orbit use World.FleetPos, which follows the rock.
	Pos Vec2
	// Dest is the rock the fleet is flying to (or, in orbit, the rock its
	// order refers to), or NoRock.
	Dest RockID
	// Origin is the rock the fleet last departed from, or NoRock.
	Origin RockID
	// Home is the rock the fleet returns to (Return, end of a Survey) and
	// whose colony receives collected salvage, or NoRock.
	Home RockID
	// Order is the standing order.
	Order OrderKind
	// Stance is the bombardment target order while attacking.
	Stance Stance
	// RetreatAt is the HP fraction below which an attacking fleet turns to
	// OrderReturn: RetreatNever, RetreatQuarter or RetreatHalf.
	RetreatAt float64
	// InitialMaxHP is the fleet's total max HP when the current Attack order
	// was given; the retreat check compares TotalHP against
	// RetreatAt x InitialMaxHP, so losses count.
	InitialMaxHP float64
	// Garrison marks the faction's idle fleet in orbit of At. A faction has
	// at most one garrison per rock; new and returning ships merge into it.
	Garrison bool
	// Route is the freight route while hauling, else nil.
	Route *Route
	// SurveyTimer is the time in game-seconds spent surveying At.
	SurveyTimer float64
	// DwellTimer is the time in game-seconds left at the current freight
	// stop.
	DwellTimer float64
	// RetryTimer is the time in game-seconds until a refused launch is
	// retried.
	RetryTimer float64
	// ColonizeHex is the requested Keystone site of a Colonize order; it is
	// meaningful only when HasColonizeHex is set.
	ColonizeHex Hex
	// HasColonizeHex is true when ColonizeHex was requested explicitly;
	// otherwise the best available site is chosen on arrival.
	HasColonizeHex bool
	// Engaged is true while the fleet took part in combat in the last tick.
	Engaged bool
	// LaunchTick is the tick at which the fleet last left orbit (0 = never).
	LaunchTick int
}

// NewFleet returns an empty fleet of the given owner holding in orbit of
// rock at, with Home = at and no destination, origin or route. Its ID is 0
// until World.AddFleet assigns one. Pass NoRock for a fleet created in
// transit and set Pos yourself.
func NewFleet(owner FactionID, at RockID) *Fleet {
	return &Fleet{
		Owner:  owner,
		At:     at,
		Dest:   NoRock,
		Origin: NoRock,
		Home:   at,
		Order:  OrderHold,
	}
}

// InTransit reports whether the fleet is flying between rocks.
func (f *Fleet) InTransit() bool { return f != nil && f.At == NoRock }

// Empty reports whether the fleet has no ships.
func (f *Fleet) Empty() bool { return f == nil || len(f.Ships) == 0 }

// Speed returns the fleet's speed in u per game-second: the speed of its
// slowest ship x mods.ShipSpeedMul (pass the owner's Mods). An empty fleet
// has speed 0.
func (f *Fleet) Speed(mods Mods) float64 {
	if f.Empty() {
		return 0
	}
	slowest := math.Inf(1)
	for i := range f.Ships {
		if s := ShipDefOf(f.Ships[i].Kind).Speed; s < slowest {
			slowest = s
		}
	}
	return slowest * mulOr1(mods.ShipSpeedMul)
}

// TotalHP returns the sum of the ships' current HP.
func (f *Fleet) TotalHP() float64 {
	if f == nil {
		return 0
	}
	t := 0.0
	for i := range f.Ships {
		t += f.Ships[i].HP
	}
	return t
}

// TotalMaxHP returns the sum of the ships' max HP.
func (f *Fleet) TotalMaxHP() float64 {
	if f == nil {
		return 0
	}
	t := 0.0
	for i := range f.Ships {
		t += f.Ships[i].MaxHP
	}
	return t
}

// CargoCap returns the fleet's total cargo capacity.
func (f *Fleet) CargoCap() float64 {
	if f == nil {
		return 0
	}
	t := 0.0
	for i := range f.Ships {
		t += ShipDefOf(f.Ships[i].Kind).Cargo
	}
	return t
}

// CargoUsed returns the total amount of goods the fleet carries.
func (f *Fleet) CargoUsed() float64 {
	if f == nil {
		return 0
	}
	t := 0.0
	for i := range f.Ships {
		t += f.Ships[i].Cargo.Total()
	}
	return t
}

// CargoFree returns the fleet's unused cargo capacity.
func (f *Fleet) CargoFree() float64 {
	if f == nil {
		return 0
	}
	t := 0.0
	for i := range f.Ships {
		t += f.Ships[i].CargoFree()
	}
	return t
}

// CargoTotal returns the goods carried by all ships together.
func (f *Fleet) CargoTotal() Stock {
	var s Stock
	if f == nil {
		return s
	}
	for i := range f.Ships {
		s.Add(f.Ships[i].Cargo)
	}
	return s
}

// LoadCargo stows up to amt units of good g, filling ships in slice order,
// and returns the amount actually loaded (limited by free capacity).
func (f *Fleet) LoadCargo(g Good, amt float64) float64 {
	if f == nil || !g.Valid() || !(amt > 0) {
		return 0
	}
	loaded := 0.0
	for i := range f.Ships {
		left := amt - loaded
		if left <= 0 {
			break
		}
		n := math.Min(left, f.Ships[i].CargoFree())
		if n > 0 {
			f.Ships[i].Cargo[g] += n
			loaded += n
		}
	}
	return loaded
}

// UnloadCargo removes up to amt units of good g, emptying ships in slice
// order, and returns the amount actually removed.
func (f *Fleet) UnloadCargo(g Good, amt float64) float64 {
	if f == nil || !g.Valid() || !(amt > 0) {
		return 0
	}
	taken := 0.0
	for i := range f.Ships {
		left := amt - taken
		if left <= 0 {
			break
		}
		have := f.Ships[i].Cargo[g]
		if have <= 0 {
			continue
		}
		if left >= have-StockEpsilon {
			f.Ships[i].Cargo[g] = 0
			taken += math.Min(left, have)
		} else {
			f.Ships[i].Cargo[g] = have - left
			taken += left
		}
	}
	return taken
}

// Count returns the number of ships of the given kind in the fleet.
func (f *Fleet) Count(kind ShipKind) int {
	if f == nil {
		return 0
	}
	n := 0
	for i := range f.Ships {
		if f.Ships[i].Kind == kind {
			n++
		}
	}
	return n
}

// HasKind reports whether the fleet contains a ship of the given kind.
func (f *Fleet) HasKind(kind ShipKind) bool { return f.Count(kind) > 0 }

// Armed reports whether any ship of the fleet has a weapon.
func (f *Fleet) Armed() bool {
	if f == nil {
		return false
	}
	for i := range f.Ships {
		if ShipDefOf(f.Ships[i].Kind).Armed() {
			return true
		}
	}
	return false
}

// Berths returns the number of berths the fleet's ships occupy.
func (f *Fleet) Berths() int {
	if f == nil {
		return 0
	}
	n := 0
	for i := range f.Ships {
		n += ShipDefOf(f.Ships[i].Kind).Berths
	}
	return n
}

// FuelPerU returns the Fuel the fleet burns per u of distance when leaving
// a rock of its own faction: the sum of its ships' FuelPerU.
func (f *Fleet) FuelPerU() float64 {
	if f == nil {
		return 0
	}
	t := 0.0
	for i := range f.Ships {
		t += ShipDefOf(f.Ships[i].Kind).FuelPerU
	}
	return t
}

// ShipIndex returns the index in Ships of the ship with the given ID, or -1.
func (f *Fleet) ShipIndex(id ShipID) int {
	if f == nil || id <= 0 {
		return -1
	}
	for i := range f.Ships {
		if f.Ships[i].ID == id {
			return i
		}
	}
	return -1
}

// Ship returns a pointer to the ship with the given ID, or nil. The pointer
// aliases the Ships slice and is invalidated by any change to that slice.
func (f *Fleet) Ship(id ShipID) *Ship {
	i := f.ShipIndex(id)
	if i < 0 {
		return nil
	}
	return &f.Ships[i]
}

// RemoveShip removes the ship with the given ID, keeping the order of the
// remaining ships, and returns it. ok is false when the fleet has no such
// ship.
func (f *Fleet) RemoveShip(id ShipID) (ship Ship, ok bool) {
	i := f.ShipIndex(id)
	if i < 0 {
		return Ship{}, false
	}
	ship = f.Ships[i]
	copy(f.Ships[i:], f.Ships[i+1:])
	f.Ships[len(f.Ships)-1] = Ship{}
	f.Ships = f.Ships[:len(f.Ships)-1]
	return ship, true
}

// ShipIDs returns the IDs of the fleet's ships in slice order.
func (f *Fleet) ShipIDs() []ShipID {
	if f == nil {
		return nil
	}
	ids := make([]ShipID, len(f.Ships))
	for i := range f.Ships {
		ids[i] = f.Ships[i].ID
	}
	return ids
}

// Slug is a Mass Driver projectile in flight.
type Slug struct {
	// Owner is the faction that fired the slug.
	Owner FactionID
	// From is the rock it was fired from.
	From RockID
	// Target is the rock it flies toward (homing on its current position).
	Target RockID
	// Pos is the slug's belt position.
	Pos Vec2
	// Damage is the damage it deals to one building on arrival.
	Damage float64
}
