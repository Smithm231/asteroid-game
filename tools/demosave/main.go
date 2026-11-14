// Command demosave writes a mid-game save file for testing and screenshots:
// it lets a rival brain play the human Hold for a number of cycles and then
// hands the Hold back to the player.
//
//	go run ./tools/demosave -seed 42 -cycles 45 -out demo.drift
package main

import (
	"flag"
	"fmt"
	"os"

	"drifthold/internal/ai"
	"drifthold/internal/model"
	"drifthold/internal/save"
	"drifthold/internal/sim"
)

func main() {
	seed := flag.Uint64("seed", 42, "world seed")
	cycles := flag.Int("cycles", 45, "cycles to play before saving")
	out := flag.String("out", "demo.drift", "save file to write")
	flag.Parse()

	st := model.DefaultSettings(*seed)
	w, err := sim.NewGame(st)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	h := w.Human()
	name := h.Name
	h.Personality, h.IsAI = model.PersonalityMeridian, true
	s := sim.New(w)
	s.AI = ai.Step
	s.Run(*cycles * model.TicksPerCycle)
	h.Personality, h.IsAI, h.Name = model.PersonalityNone, false, name
	h.RecomputeMods(st.Difficulty)
	w.Log = nil
	if err := save.WriteFile(*out, w, save.MetaOf(w, "demo")); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: cycle %d, %d colonies, %.0f colonists, %d ships\n",
		*out, w.Cycle(), w.ColonyCount(h.ID), w.TotalPop(h.ID), w.ShipCount(h.ID))
}
