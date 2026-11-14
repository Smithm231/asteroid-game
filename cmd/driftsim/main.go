// Command driftsim runs Drifthold games without graphics: every Hold is played
// by a rival brain. It is the tool for soak-testing the rules and for judging
// balance (how fast colonies grow, when fleets appear, how games end).
//
//	go run ./cmd/driftsim -seed 7 -cycles 90 -every 10
//	go run ./cmd/driftsim -runs 20 -cycles 120 -quiet
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"drifthold/internal/ai"
	"drifthold/internal/model"
	"drifthold/internal/sim"
)

var verbose bool

func main() {
	seed := flag.Uint64("seed", 1, "world seed (first seed with -runs)")
	runs := flag.Int("runs", 1, "number of games to play, on consecutive seeds")
	cycles := flag.Int("cycles", 90, "maximum game length in cycles (1 cycle = 60 game-seconds)")
	rivals := flag.Int("rivals", 2, "number of rivals besides Hold 1 (1-3)")
	size := flag.Int("size", 1, "belt size: 0 small, 1 medium, 2 large")
	diff := flag.Int("diff", 1, "difficulty: 0 easy, 1 normal, 2 hard")
	every := flag.Int("every", 10, "print a status table every N cycles (0 = never)")
	quiet := flag.Bool("quiet", false, "print only one summary line per game")
	events := flag.Bool("events", true, "enable random belt events")
	validate := flag.Bool("validate", true, "check world invariants every cycle")
	flag.BoolVar(&verbose, "v", false, "also list each Hold's buildings in the status table")
	flag.Parse()

	failed := false
	wins := map[string]int{}
	for i := 0; i < *runs; i++ {
		st := model.DefaultSettings(*seed + uint64(i))
		st.Rivals, st.BeltSize, st.Difficulty, st.Events = *rivals, model.BeltSize(*size), model.Difficulty(*diff), *events
		st.Directives = false
		w, err := sim.NewGame(st)
		if err != nil {
			fmt.Fprintln(os.Stderr, "worldgen:", err)
			os.Exit(1)
		}
		// Hold 1 is normally the human player; here a brain plays it too.
		p := model.RivalPersonalities()[int(st.Seed)%3]
		h := w.Factions[0]
		h.Personality, h.IsAI, h.Name = p, true, p.String()+" (Hold 1)"
		h.RecomputeMods(st.Difficulty)

		s := sim.New(w)
		s.AI = ai.Step
		logSeen := 0
		for c := 1; c <= *cycles && !w.GameOver; c++ {
			s.Run(model.TicksPerCycle)
			if *validate {
				if err := w.Validate(); err != nil {
					fmt.Printf("seed %d cycle %d: INVALID WORLD: %v\n", st.Seed, c, err)
					failed = true
					break
				}
			}
			if !*quiet {
				for _, e := range w.Log {
					if e.Seq > logSeen && e.Faction == model.NoFaction && e.Sev >= model.SevAlert {
						fmt.Printf("  [cycle %d] %s\n", model.CycleOfTick(e.Tick), e.Text)
					}
				}
				logSeen = w.LogSeq
				if *every > 0 && c%*every == 0 {
					table(w, c)
				}
			}
		}
		winner := "none"
		if wf := w.Faction(w.Winner); wf != nil {
			winner = wf.Personality.Archetype()
		}
		wins[winner]++
		fmt.Printf("seed %-4d end cycle %-4d over=%-5v winner=%-12s %s\n", st.Seed, w.Cycle(), w.GameOver, winner, summary(w))
	}
	if *runs > 1 {
		fmt.Println("wins:", wins)
	}
	if failed {
		os.Exit(1)
	}
}

func summary(w *model.World) string {
	var parts []string
	for _, f := range w.Factions {
		tag := f.Personality.Archetype()[:3]
		if !f.Alive {
			parts = append(parts, fmt.Sprintf("%s:dead@%d", tag, model.CycleOfTick(f.EliminatedTick)))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s:c%d p%.0f t%d s%d", tag, w.ColonyCount(f.ID), w.TotalPop(f.ID), f.TechCount(), w.ShipCount(f.ID)))
	}
	return strings.Join(parts, " | ")
}

func table(w *model.World, cycle int) {
	fmt.Printf("--- cycle %d ---\n", cycle)
	fmt.Printf("%-28s %3s %5s %4s %4s %5s %6s %5s %5s %5s %5s %5s %5s  %s\n",
		"hold", "col", "pop", "bld", "tech", "ships", "scrip", "alloy", "parts", "fuel", "food", "ice", "lumen", "research / home colony")
	for _, f := range w.Factions {
		if !f.Alive {
			fmt.Printf("%-28s eliminated in cycle %d\n", f.Name, model.CycleOfTick(f.EliminatedTick))
			continue
		}
		bld := 0
		for _, r := range w.RocksOf(f.ID) {
			for _, b := range r.Buildings {
				if b.Owner == f.ID && b.Completed() {
					bld++
				}
			}
		}
		extra := "-"
		if f.Research.Current != model.TechNone {
			extra = fmt.Sprintf("%s %.0f%%", f.Research.Current, 100*f.ResearchFrac(f.Research.Current))
		}
		if c := w.Colony(f.HomeRock); c != nil {
			extra += fmt.Sprintf(" | morale %.0f pwr %.0f/%.0f idle %d wave %d", c.Morale, c.PowerDemand, c.PowerSupply, c.IdleWorkers(), f.AI.Wave)
		}
		if verbose {
			var kinds [model.NumBuildingKinds]int
			sites := 0
			for _, r := range w.RocksOf(f.ID) {
				for _, b := range r.Buildings {
					if b.Owner == f.ID {
						kinds[b.Kind]++
						if b.Constructing() {
							sites++
						}
					}
				}
			}
			var sb strings.Builder
			for k, n := range kinds {
				if n > 0 && model.BuildingKind(k) != model.Keystone {
					fmt.Fprintf(&sb, "%s×%d ", model.BuildingKind(k), n)
				}
			}
			c := w.Colony(f.HomeRock)
			if c != nil {
				fmt.Fprintf(&sb, "| sites %d housing %.0f air %.0f starving %v suffocating %v ore %.0f silica %.0f", sites, c.Housing, c.LifeSupport, c.Starving, c.Suffocating, c.Stock[model.Ore], c.Stock[model.Silica])
			}
			fmt.Printf("    %s\n", sb.String())
		}
		fmt.Printf("%-28s %3d %5.0f %4d %4d %5d %6.0f %5.0f %5.0f %5.0f %5.0f %5.0f %5.0f  %s\n",
			f.Name, w.ColonyCount(f.ID), w.TotalPop(f.ID), bld, f.TechCount(), w.ShipCount(f.ID), f.Scrip,
			w.FactionTotal(f.ID, model.Alloy), w.FactionTotal(f.ID, model.Parts), w.FactionTotal(f.ID, model.Fuel),
			w.FactionTotal(f.ID, model.Food), w.FactionTotal(f.ID, model.Ice), w.FactionTotal(f.ID, model.Lumen), extra)
	}
}
