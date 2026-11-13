package model

// FactionID identifies a Hold. 0 (NoFaction) means "none / neutral"; real
// factions are numbered 1..MaxFactions. Faction 1 is always the human player.
type FactionID uint8

const (
	// NoFaction is the zero FactionID: no owner, neutral, or "broadcast" when
	// used as the addressee of a log entry.
	NoFaction FactionID = 0
	// PlayerFaction is the ID of the human player's Hold.
	PlayerFaction FactionID = 1
	// MaxFactions is the largest valid FactionID. It is 7 so that a set of
	// factions fits in the uint8 FactionMask (bit n = faction n, bit 0 unused).
	MaxFactions = 7
)

// Valid reports whether f names a possible faction (1..MaxFactions). It does
// not check that the faction exists in a particular world.
func (f FactionID) Valid() bool { return f >= 1 && f <= MaxFactions }

// FactionMask is a set of factions stored as a bitmask: bit n is set when
// faction n is a member. Bit 0 is never set.
type FactionMask uint8

// maskBit returns the mask bit for f, or 0 when f is not a valid faction.
func maskBit(f FactionID) FactionMask {
	if !f.Valid() {
		return 0
	}
	return FactionMask(1) << f
}

// Has reports whether faction f is in the set.
func (m FactionMask) Has(f FactionID) bool { return m&maskBit(f) != 0 }

// With returns the set with faction f added.
func (m FactionMask) With(f FactionID) FactionMask { return m | maskBit(f) }

// Without returns the set with faction f removed.
func (m FactionMask) Without(f FactionID) FactionMask { return m &^ maskBit(f) }

// Set adds faction f to the set in place.
func (m *FactionMask) Set(f FactionID) { *m |= maskBit(f) }

// Clear removes faction f from the set in place.
func (m *FactionMask) Clear(f FactionID) { *m &^= maskBit(f) }

// RockID identifies a rock of the belt. It equals the rock's index in
// World.Rocks. Because rock 0 is a real rock, "none" is NoRock (-1), not 0.
type RockID int32

// NoRock is the RockID meaning "no rock" (for example a fleet in transit has
// Fleet.At == NoRock).
const NoRock RockID = -1

// BuildingID identifies a building. IDs are unique across the whole world
// (not just per rock), are issued by World.NewBuildingID and are never reused.
// 0 means "none".
type BuildingID int32

// FleetID identifies a fleet. IDs are issued by World.NewFleetID and are never
// reused. 0 means "none".
type FleetID int32

// ShipID identifies a single ship. IDs are unique across the whole world, are
// issued by World.NewShipID and are never reused. 0 means "none".
type ShipID int32
