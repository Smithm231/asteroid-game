package model

// Command is a player or AI instruction to the simulation. It is one flat
// struct; Kind selects which fields are read. Commands are queued in
// World.Pending and executed at the start of the next tick, each by the
// system that owns its kind.
//
// Fields used per kind (Faction, the issuer, is always required):
//
//	CmdBuild           Rock, Hex, BKind
//	CmdDemolish        Rock, Building
//	CmdRecommission    Rock, Building
//	CmdSetEnabled      Rock, Building, Flag (true = on)
//	CmdSetPriority     Rock, Building, Int (PriorityLow..PriorityHigh)
//	CmdExpedite        Rock, Building
//	CmdQueueShip       Rock, Building (the Launch Cradle), SKind
//	CmdCancelShip      Rock, Building (the Launch Cradle), Int (queue index)
//	CmdFleetOrder      Fleet, Ships (empty = whole fleet), Order, Target,
//	                   Stance, Retreat, Hex + HasHex (Colonize site)
//	CmdSetRoute        Fleet, Ships (empty = whole fleet), Route
//	CmdScuttle         Fleet, Ships (empty = whole fleet)
//	CmdSetResearch     Tech
//	CmdQueueResearch   Tech
//	CmdCancelResearch  Int (-1 = current project, else queue index)
//	CmdTrade           Rock, Good, Amount (positive buys, negative sells)
//	CmdSponsor         Rock
//	CmdSetDriverTarget Rock, Building (the Mass Driver), Target (NoRock clears)
//
// The zero Command has Kind CmdNone and is rejected. Note that the zero
// value of Rock and Target is rock 0, not NoRock; the constructor functions
// below (BuildCmd, FleetOrderCmd, ...) fill every field a kind needs and set
// unused rock fields to NoRock.
type Command struct {
	// Kind discriminates the command.
	Kind CmdKind
	// Faction is the issuer; handlers verify that it owns what it commands.
	Faction FactionID
	// Rock is the rock the command acts on.
	Rock RockID
	// Hex is a tile on Rock (CmdBuild) or the requested Keystone site on
	// Target (CmdFleetOrder with OrderColonize, when HasHex is set).
	Hex Hex
	// HasHex is true when Hex is an explicit colonisation site. When false a
	// Colonize order picks the best available site.
	HasHex bool
	// BKind is the building kind to place (CmdBuild).
	BKind BuildingKind
	// SKind is the ship kind to queue (CmdQueueShip).
	SKind ShipKind
	// Building is the building acted on, on Rock.
	Building BuildingID
	// Fleet is the fleet acted on.
	Fleet FleetID
	// Ships selects a subset of the fleet's ships; empty means every ship.
	// Ordering a proper subset splits it into a new fleet.
	Ships []ShipID
	// Order is the new fleet order (CmdFleetOrder).
	Order OrderKind
	// Target is the destination rock of a fleet order, or the target rock of
	// a Mass Driver (NoRock clears it).
	Target RockID
	// Stance is the attack stance (CmdFleetOrder with OrderAttack).
	Stance Stance
	// Retreat is the retreat threshold: RetreatNever, RetreatQuarter or
	// RetreatHalf (CmdFleetOrder with OrderAttack).
	Retreat float64
	// Route is the freight route (CmdSetRoute).
	Route Route
	// Tech is the technology (research commands).
	Tech TechID
	// Good is the good traded (CmdTrade).
	Good Good
	// Amount is the number of units to trade: positive buys, negative sells
	// (CmdTrade). At most MaxTradeUnits in magnitude.
	Amount float64
	// Flag is a generic boolean argument (CmdSetEnabled).
	Flag bool
	// Int is a generic integer argument (priority, queue index).
	Int int
}

// baseCmd returns a command of the given kind and issuer with every ID field
// set to its "none" value.
func baseCmd(kind CmdKind, f FactionID) Command {
	return Command{
		Kind: kind, Faction: f,
		Rock: NoRock, Target: NoRock,
		BKind: BuildingNone, SKind: ShipNone, Tech: TechNone, Good: GoodNone,
		Route: Route{A: NoRock, B: NoRock},
	}
}

// BuildCmd returns a CmdBuild: place a building of the given kind on a tile.
func BuildCmd(f FactionID, rock RockID, hex Hex, kind BuildingKind) Command {
	c := baseCmd(CmdBuild, f)
	c.Rock, c.Hex, c.BKind = rock, hex, kind
	return c
}

// DemolishCmd returns a CmdDemolish: cancel a construction, demolish a
// completed building or salvage a derelict.
func DemolishCmd(f FactionID, rock RockID, building BuildingID) Command {
	c := baseCmd(CmdDemolish, f)
	c.Rock, c.Building = rock, building
	return c
}

// RecommissionCmd returns a CmdRecommission: reactivate a derelict building.
func RecommissionCmd(f FactionID, rock RockID, building BuildingID) Command {
	c := baseCmd(CmdRecommission, f)
	c.Rock, c.Building = rock, building
	return c
}

// SetEnabledCmd returns a CmdSetEnabled: switch a building on or off.
func SetEnabledCmd(f FactionID, rock RockID, building BuildingID, on bool) Command {
	c := baseCmd(CmdSetEnabled, f)
	c.Rock, c.Building, c.Flag = rock, building, on
	return c
}

// SetPriorityCmd returns a CmdSetPriority: set a building's priority
// (PriorityLow, PriorityNormal or PriorityHigh).
func SetPriorityCmd(f FactionID, rock RockID, building BuildingID, priority int) Command {
	c := baseCmd(CmdSetPriority, f)
	c.Rock, c.Building, c.Int = rock, building, priority
	return c
}

// ExpediteCmd returns a CmdExpedite: pay Scrip to finish a construction now.
func ExpediteCmd(f FactionID, rock RockID, building BuildingID) Command {
	c := baseCmd(CmdExpedite, f)
	c.Rock, c.Building = rock, building
	return c
}

// QueueShipCmd returns a CmdQueueShip: add a ship to a Launch Cradle queue.
func QueueShipCmd(f FactionID, rock RockID, cradle BuildingID, kind ShipKind) Command {
	c := baseCmd(CmdQueueShip, f)
	c.Rock, c.Building, c.SKind = rock, cradle, kind
	return c
}

// CancelShipCmd returns a CmdCancelShip: remove queue entry index from a
// Launch Cradle queue.
func CancelShipCmd(f FactionID, rock RockID, cradle BuildingID, index int) Command {
	c := baseCmd(CmdCancelShip, f)
	c.Rock, c.Building, c.Int = rock, cradle, index
	return c
}

// FleetOrderCmd returns a CmdFleetOrder with the given order and target
// rock, default stance (Suppress) and no retreat. ships selects a subset of
// the fleet (nil or empty = the whole fleet). Use it for Hold, Move, Survey
// and Return; AttackCmd and ColonizeCmd cover the orders with extra
// arguments.
func FleetOrderCmd(f FactionID, fleet FleetID, ships []ShipID, order OrderKind, target RockID) Command {
	c := baseCmd(CmdFleetOrder, f)
	c.Fleet, c.Ships, c.Order, c.Target = fleet, ships, order, target
	return c
}

// AttackCmd returns a CmdFleetOrder with OrderAttack on target, using the
// given stance and retreat threshold (RetreatNever, RetreatQuarter or
// RetreatHalf).
func AttackCmd(f FactionID, fleet FleetID, ships []ShipID, target RockID, stance Stance, retreat float64) Command {
	c := FleetOrderCmd(f, fleet, ships, OrderAttack, target)
	c.Stance, c.Retreat = stance, retreat
	return c
}

// ColonizeCmd returns a CmdFleetOrder with OrderColonize on target. site is
// the requested Keystone hex, or nil to let the simulation pick the best
// available site.
func ColonizeCmd(f FactionID, fleet FleetID, ships []ShipID, target RockID, site *Hex) Command {
	c := FleetOrderCmd(f, fleet, ships, OrderColonize, target)
	if site != nil {
		c.Hex, c.HasHex = *site, true
	}
	return c
}

// SetRouteCmd returns a CmdSetRoute: assign a freight route to a fleet (or
// to a subset of its ships) and start hauling.
func SetRouteCmd(f FactionID, fleet FleetID, ships []ShipID, route Route) Command {
	c := baseCmd(CmdSetRoute, f)
	c.Fleet, c.Ships, c.Route = fleet, ships, route
	return c
}

// ScuttleCmd returns a CmdScuttle: destroy own ships (nil or empty ships =
// the whole fleet).
func ScuttleCmd(f FactionID, fleet FleetID, ships []ShipID) Command {
	c := baseCmd(CmdScuttle, f)
	c.Fleet, c.Ships = fleet, ships
	return c
}

// SetResearchCmd returns a CmdSetResearch: make tech the current project
// (progress on the replaced project is kept).
func SetResearchCmd(f FactionID, tech TechID) Command {
	c := baseCmd(CmdSetResearch, f)
	c.Tech = tech
	return c
}

// QueueResearchCmd returns a CmdQueueResearch: append tech to the research
// queue.
func QueueResearchCmd(f FactionID, tech TechID) Command {
	c := baseCmd(CmdQueueResearch, f)
	c.Tech = tech
	return c
}

// CancelResearchCmd returns a CmdCancelResearch: index -1 cancels the
// current project, any other value removes that queue entry.
func CancelResearchCmd(f FactionID, index int) Command {
	c := baseCmd(CmdCancelResearch, f)
	c.Int = index
	return c
}

// TradeCmd returns a CmdTrade at rock: a positive amount buys that many
// units of good, a negative amount sells.
func TradeCmd(f FactionID, rock RockID, good Good, amount float64) Command {
	c := baseCmd(CmdTrade, f)
	c.Rock, c.Good, c.Amount = rock, good, amount
	return c
}

// SponsorCmd returns a CmdSponsor: sponsor settlers at rock.
func SponsorCmd(f FactionID, rock RockID) Command {
	c := baseCmd(CmdSponsor, f)
	c.Rock = rock
	return c
}

// SetDriverTargetCmd returns a CmdSetDriverTarget: aim the Mass Driver
// building on rock at target (NoRock clears the target).
func SetDriverTargetCmd(f FactionID, rock RockID, building BuildingID, target RockID) Command {
	c := baseCmd(CmdSetDriverTarget, f)
	c.Rock, c.Building, c.Target = rock, building, target
	return c
}
