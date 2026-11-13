// Package model defines every data type shared by the Drifthold simulation
// and its user interface.
//
// It contains, in rough reading order:
//
//   - identifiers and enumerations (ids.go, enums.go);
//   - hex-grid, vector and orbit maths (hex.go, vec.go, orbit.go);
//   - the deterministic random number generator (rng.go);
//   - goods and the Stock container (stock.go, goods.go);
//   - the immutable content tables for buildings, ships and technologies and
//     every tuning constant of the game rules (consts.go, buildings.go,
//     ships.go, techs.go, content_check.go);
//   - the complete, JSON-serialisable world state (world.go, rock.go,
//     colony.go, building.go, faction.go, fleet.go, market.go);
//   - the command, log and FX records exchanged between the simulation and the
//     application (command.go, logfx.go, errors.go);
//   - the ownership checks every command handler starts with and the
//     bookkeeping of founding and losing a colony (checks.go);
//   - a consistency checker for a whole world (validate.go).
//
// # Rules of the package
//
// The package imports only the standard library and contains no game rules
// that need another package: it stores state, answers pure queries about it
// and performs small bookkeeping mutations (adding a building keeps the tile
// back-reference and the sort order intact, for example). The systems that
// advance the game live in sibling packages and build on this contract.
//
// Everything reachable from World is plain data that round-trips through
// encoding/json byte for byte: no functions, channels, interfaces, cyclic
// pointers or maps. Cross references always use IDs, never pointers.
//
// Everything here is deterministic. Randomness comes only from World.RNG (or
// from generators derived from it); no helper iterates over a Go map, reads
// the clock or starts a goroutine.
//
// # Conventions
//
//   - RockID uses -1 (NoRock) for "none" because rock 0 is a real rock. Every
//     other ID type uses 0 for "none". Good, BuildingKind, ShipKind and TechID
//     are zero-based table indices and use -1 (GoodNone, BuildingNone,
//     ShipNone, TechNone) for "none". Because of this a few zero values are
//     NOT "empty": use the New... constructors (NewWorld, NewRock, NewColony,
//     NewBuilding, NewFaction, NewFleet, NewShip), which fill those fields.
//   - Times are stored as sim ticks (int). A stored tick of 0 means "never"
//     for the Last...Tick fields.
//   - Rates in the content tables are per cycle unless a comment says
//     otherwise; build times and cooldowns are in game-seconds.
//   - Content accessors return values or shared read-only slices. Never write
//     through a slice returned by Buildings, Ships, Techs or Goods, nor through
//     the slices inside a TechDef.
//   - Lookups are nil-safe: they return nil (or a zero value) instead of
//     panicking when given an unknown ID or a nil receiver.
package model
