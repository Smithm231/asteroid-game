package sim

import "drifthold/internal/model"

// Directive is one onboarding goal for the human player.
type Directive struct {
	// Title is the short name of the goal.
	Title string
	// Hint is one sentence telling the player how to get there.
	Hint string
	// Reward is the Scrip granted on completion.
	Reward float64
	done   func(w *model.World, f *model.Faction) bool
}

// hasBuilt reports whether faction f has a completed building of the kind.
func hasBuilt(w *model.World, f model.FactionID, kind model.BuildingKind) bool {
	for _, r := range w.Rocks {
		if r.Owner != f {
			continue
		}
		for _, b := range r.Buildings {
			if b.Kind == kind && b.Completed() && b.Owner == f {
				return true
			}
		}
	}
	return false
}

var directiveDefs = []Directive{
	{"Raise a Sunvane", "Open the Power tab and place a Sunvane next to your Keystone.", 30,
		func(w *model.World, f *model.Faction) bool { return hasBuilt(w, f.ID, model.Sunvane) }},
	{"Sink a Bore Rig beside Ore", "Place a Bore Rig on or next to an Ore deposit; Conduits extend your reach.", 40,
		func(w *model.World, f *model.Faction) bool {
			for _, r := range w.Rocks {
				if r.Owner != f.ID {
					continue
				}
				for _, b := range r.Buildings {
					if b.Kind != model.BoreRig || !b.Completed() {
						continue
					}
					for _, h := range reach(b.Hex) {
						if t := r.TileAt(h); t != nil && t.Deposit == model.Ore && t.Terrain.Solid() {
							return true
						}
					}
				}
			}
			return false
		}},
	{"Fire up a Smelter", "A Smelter turns Ore into Alloy, the material almost everything is built from.", 40,
		func(w *model.World, f *model.Faction) bool { return hasBuilt(w, f.ID, model.Smelter) }},
	{"Raise a Greenhouse and an Aerator", "Colonists need Food and air; both buildings also need Ice.", 40,
		func(w *model.World, f *model.Faction) bool {
			return hasBuilt(w, f.ID, model.Greenhouse) && hasBuilt(w, f.ID, model.Aerator)
		}},
	{"Reach 20 colonists", "Build Habitats and keep morale up; a fed, housed colony grows by itself.", 60,
		func(w *model.World, f *model.Faction) bool { return w.TotalPop(f.ID) >= 20 }},
	{"Build a Laboratory and start research", "Place a Laboratory, then pick a project in the Research window.", 60,
		func(w *model.World, f *model.Faction) bool {
			return hasBuilt(w, f.ID, model.Laboratory) && (f.Research.Current != model.TechNone || f.TechCount() > 0)
		}},
	{"Research Orbital Flight", "Orbital Flight unlocks the Launch Cradle and your first ships.", 80,
		func(w *model.World, f *model.Faction) bool { return f.Has(model.OrbitalFlight) }},
	{"Build a Launch Cradle", "The Launch Cradle is your shipyard; you will find it under the Space tab.", 80,
		func(w *model.World, f *model.Faction) bool { return hasBuilt(w, f.ID, model.LaunchCradle) }},
	{"Survey another rock", "Build a Prospector and send it to survey a rock from the Belt view.", 100,
		func(w *model.World, f *model.Faction) bool { return f.Stats.RocksSurveyed >= 1 }},
	{"Raise a Flak Battery", "Research Point Defence, then build a Flak Battery: rivals will come calling.", 100,
		func(w *model.World, f *model.Faction) bool {
			return f.Has(model.PointDefence) && hasBuilt(w, f.ID, model.FlakBattery)
		}},
	{"Found a second colony", "Research Colonial Charter, build a Seedship and send it to an unclaimed rock.", 200,
		func(w *model.World, f *model.Faction) bool { return f.Stats.ColoniesFounded >= 1 }},
	{"Destroy a rival Keystone", "Send armed ships to attack a rival rock; Hammers are best against structures.", 400,
		func(w *model.World, f *model.Faction) bool { return f.Stats.KeystonesDestroyed >= 1 }},
}

// Directives returns the onboarding goals in order.
func Directives() []Directive { return directiveDefs }

// CurrentDirective returns the directive faction f is working on and its
// index, or false when directives are off or all are done.
func CurrentDirective(w *model.World, f model.FactionID) (Directive, int, bool) {
	fac := w.Faction(f)
	if fac == nil || fac.IsAI || !w.Settings.Directives {
		return Directive{}, 0, false
	}
	i := fac.Directives.Current
	if i < 0 || i >= len(directiveDefs) {
		return Directive{}, i, false
	}
	return directiveDefs[i], i, true
}

// stepDirectives completes the human players' directives whose goals are met.
func stepDirectives(w *model.World) {
	if !w.Settings.Directives {
		return
	}
	for _, fac := range w.Factions {
		if fac.IsAI || !fac.Alive {
			continue
		}
		ds := &fac.Directives
		for len(ds.Done) < len(directiveDefs) {
			ds.Done = append(ds.Done, false)
		}
		for ds.Current < len(directiveDefs) && directiveDefs[ds.Current].done(w, fac) {
			d := directiveDefs[ds.Current]
			ds.Done[ds.Current] = true
			fac.AddScrip(d.Reward)
			ds.LastDoneTick = w.Tick
			fx := model.NewFX(model.FXDirectiveDone, model.NoRock, model.Hex{}, fac.ID)
			fx.Amount = float64(ds.Current)
			w.Emit(fx)
			note(w, fac.ID, model.SevGood, model.NoRock, "Directive complete: %s (+%s Scrip).", d.Title, model.FormatQty(d.Reward))
			ds.Current++
		}
	}
}
