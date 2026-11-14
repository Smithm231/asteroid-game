// Package sim contains the rules of Drifthold: world generation and every
// system that advances a world by one tick. It works purely on the data types
// of package model, never touches graphics or the clock, and is fully
// deterministic: the same world and the same commands always give the same
// result.
package sim

import (
	"errors"

	"drifthold/internal/model"
)

// AIStep is the hook through which rival brains act: when set, Step calls it
// for every living computer-controlled faction on that faction's AI ticks.
// It lives here as a variable so that the ai package can depend on sim's
// helpers without an import cycle.
type AIStep func(w *model.World, f model.FactionID)

// Sim advances one world.
type Sim struct {
	// W is the world being simulated.
	W *model.World
	// AI, when not nil, is called for computer-controlled factions.
	AI AIStep
}

// New wraps an existing world.
func New(w *model.World) *Sim { return &Sim{W: w} }

// Exec executes one command immediately and returns the reason if it was
// refused. The text of the error is meant for the player.
func Exec(w *model.World, c model.Command) error {
	if w.GameOver {
		return model.ErrGameOver
	}
	switch c.Kind {
	case model.CmdBuild:
		_, err := Place(w, c.Faction, c.Rock, c.Hex, c.BKind)
		return err
	case model.CmdDemolish:
		return Demolish(w, c.Faction, c.Rock, c.Building)
	case model.CmdRecommission:
		return Recommission(w, c.Faction, c.Rock, c.Building)
	case model.CmdSetEnabled:
		return SetEnabled(w, c.Faction, c.Rock, c.Building, c.Flag)
	case model.CmdSetPriority:
		return SetPriority(w, c.Faction, c.Rock, c.Building, c.Int)
	case model.CmdExpedite:
		return Expedite(w, c.Faction, c.Rock, c.Building)
	case model.CmdQueueShip:
		return QueueShip(w, c.Faction, c.Rock, c.Building, c.SKind)
	case model.CmdCancelShip:
		return CancelShip(w, c.Faction, c.Rock, c.Building, c.Int)
	case model.CmdFleetOrder:
		return FleetOrder(w, c)
	case model.CmdSetRoute:
		return SetRoute(w, c)
	case model.CmdScuttle:
		return Scuttle(w, c)
	case model.CmdSetResearch:
		return SetResearch(w, c.Faction, c.Tech)
	case model.CmdQueueResearch:
		return QueueResearch(w, c.Faction, c.Tech)
	case model.CmdCancelResearch:
		return CancelResearch(w, c.Faction, c.Int)
	case model.CmdTrade:
		return Trade(w, c.Faction, c.Rock, c.Good, c.Amount)
	case model.CmdSponsor:
		return Sponsor(w, c.Faction, c.Rock)
	case model.CmdSetDriverTarget:
		return SetDriverTarget(w, c.Faction, c.Rock, c.Building, c.Target)
	}
	return model.ErrUnknownCommand
}

// Step advances the world by one tick in the canonical order: pending
// commands, fleets, combat, then (on slow ticks) construction, economy,
// shipyards, research, market, events and directives, and finally the rival
// brains. A finished game no longer advances.
func (s *Sim) Step() {
	w := s.W
	if w.GameOver {
		w.Pending = w.Pending[:0]
		return
	}
	if len(w.Pending) > 0 {
		pending := w.Pending
		w.Pending = nil
		for _, c := range pending {
			if err := Exec(w, c); err != nil && !errors.Is(err, model.ErrGameOver) {
				note(w, c.Faction, model.SevWarn, c.Rock, "%s.", err.Error())
			}
		}
	}
	stepFleets(w)
	stepCombat(w)
	if w.SlowTick() && !w.GameOver {
		dt := model.SlowDT
		stepConstruction(w, dt)
		stepEconomy(w, dt)
		stepShipyards(w, dt)
		stepDrivers(w, dt)
		stepResearch(w, dt)
		stepMarket(w, dt)
		stepEvents(w, dt)
		stepDirectives(w)
	}
	if s.AI != nil && !w.GameOver {
		for _, f := range w.Factions {
			if f.IsAI && f.Alive && w.AITick(f.ID) {
				s.AI(w, f.ID)
			}
		}
	}
	w.Tick++
}

// Run advances the world by n ticks (or until the game ends).
func (s *Sim) Run(n int) {
	for i := 0; i < n && !s.W.GameOver; i++ {
		s.Step()
	}
}
