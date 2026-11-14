package ai

import (
	"testing"

	"drifthold/internal/model"
	"drifthold/internal/sim"
)

// Brains must turn a bare Keystone into a working colony with research and
// ships, without ever leaving the world in an invalid state.
func TestBrainsBuildWorkingHolds(t *testing.T) {
	cycles := 45
	if testing.Short() {
		cycles = 20
	}
	for seed := uint64(1); seed <= 4; seed++ {
		st := model.DefaultSettings(seed)
		st.Rivals = 3
		w, err := sim.NewGame(st)
		if err != nil {
			t.Fatal(err)
		}
		s := sim.New(w)
		s.AI = Step
		for c := 0; c < cycles && !w.GameOver; c++ {
			s.Run(model.TicksPerCycle)
			if err := w.Validate(); err != nil {
				t.Fatalf("seed %d cycle %d: %v", seed, c, err)
			}
		}
		for _, f := range w.Factions[1:] {
			if !f.Alive {
				continue
			}
			pop := w.TotalPop(f.ID)
			if pop < 20 || f.Stats.BuildingsBuilt < 12 || f.Stats.Produced[model.Alloy] <= 0 {
				t.Errorf("seed %d %s: pop %.0f, %d buildings, alloy produced %.0f",
					seed, f.Name, pop, f.Stats.BuildingsBuilt, f.Stats.Produced[model.Alloy])
			}
			if !testing.Short() && f.TechCount() < 3 {
				t.Errorf("seed %d %s: only %d techs after %d cycles", seed, f.Name, f.TechCount(), cycles)
			}
		}
		// The human Hold is left alone by the brains: nothing was built for it.
		if h := w.Human(); h.Stats.BuildingsBuilt != 0 {
			t.Errorf("seed %d: a brain played the human Hold", seed)
		}
	}
}

func TestBrainIsDeterministic(t *testing.T) {
	run := func() *model.World {
		w, _ := sim.NewGame(model.DefaultSettings(17))
		s := sim.New(w)
		s.AI = Step
		s.Run(12 * model.TicksPerCycle)
		return w
	}
	a, b := run(), run()
	for i := range a.Factions {
		if a.Factions[i].Stats != b.Factions[i].Stats || a.Factions[i].Scrip != b.Factions[i].Scrip {
			t.Fatalf("faction %d diverged between identical runs", i+1)
		}
	}
}
