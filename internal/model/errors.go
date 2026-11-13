package model

import "errors"

// Shared sentinel errors. Command handlers return errors whose text is a
// user-readable sentence (it is shown in the log); compare with errors.Is.
var (
	// ErrNotMine is returned by a system's Apply function for a command kind
	// that the system does not own. The dispatcher then tries the next
	// system; it is never shown to the player.
	ErrNotMine = errors.New("command is not handled by this system")
	// ErrUnknownCommand is returned when no system owns a command kind.
	ErrUnknownCommand = errors.New("Unknown command")
	// ErrBadCommand is returned for a command whose arguments make no sense
	// (an index out of range, an invalid enum value, ...).
	ErrBadCommand = errors.New("That order makes no sense")
	// ErrGameOver is returned for any command issued after the game ended.
	ErrGameOver = errors.New("The game is over")
	// ErrNoFaction is returned when the issuing faction does not exist.
	ErrNoFaction = errors.New("No such Hold")
	// ErrEliminated is returned when the issuing faction has been
	// eliminated.
	ErrEliminated = errors.New("That Hold has been eliminated")
	// ErrNoRock is returned when a command names a rock that does not exist.
	ErrNoRock = errors.New("No such rock")
	// ErrNoColony is returned when a command needs a colony on a rock that
	// has none.
	ErrNoColony = errors.New("There is no colony on that rock")
	// ErrNoBuilding is returned when a command names a building that does
	// not exist on the given rock.
	ErrNoBuilding = errors.New("No such building")
	// ErrNoFleet is returned when a command names a fleet that does not
	// exist.
	ErrNoFleet = errors.New("No such fleet")
	// ErrNoShip is returned when a command names a ship that is not in the
	// given fleet.
	ErrNoShip = errors.New("No such ship in that fleet")
	// ErrNotOwner is returned when the issuer does not own the rock,
	// building or fleet the command acts on.
	ErrNotOwner = errors.New("That does not belong to your Hold")
	// ErrLocked is returned when a building, ship or tech has not been
	// unlocked by research yet.
	ErrLocked = errors.New("That has not been researched yet")
	// ErrRockClaimed is returned when a colony is to be founded on a rock
	// that already has an owner.
	ErrRockClaimed = errors.New("That rock is already claimed")
	// ErrBadHex is returned when a hex is outside the rock.
	ErrBadHex = errors.New("That tile is not part of the rock")
	// ErrTileOccupied is returned when a tile already holds a building.
	ErrTileOccupied = errors.New("That tile is already occupied")
	// ErrBadBuilding is returned by bookkeeping helpers given a nil building
	// or one with an invalid or duplicate ID.
	ErrBadBuilding = errors.New("Invalid building record")
)

// ShortageError reports that a stock cannot cover a cost. Its text follows
// the house style "Not enough Alloy (need 35)".
type ShortageError struct {
	// Good is the first good (in enum order) that is lacking.
	Good Good
	// Need is the amount of that good the cost requires.
	Need float64
}

// Error implements the error interface.
func (e *ShortageError) Error() string {
	return "Not enough " + e.Good.String() + " (need " + FormatQty(e.Need) + ")"
}

// ScripError reports that a faction cannot afford a Scrip price. Its text is
// "Not enough Scrip (need 120)".
type ScripError struct {
	// Need is the Scrip price.
	Need float64
}

// Error implements the error interface.
func (e *ScripError) Error() string {
	return "Not enough Scrip (need " + FormatQty(e.Need) + ")"
}
