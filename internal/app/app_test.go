package app

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/input"
	"drifthold/internal/model"
	"drifthold/internal/sim"
)

// frameInput builds an input snapshot for a 1600x900 point window.
func frameInput(frame int64) input.State {
	return input.State{W: 1600, H: 900, Scale: 1, Dt: 1.0 / 60, Frame: frame}
}

var fuzzKeys = []ebiten.Key{
	ebiten.KeyTab, ebiten.KeySpace, ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyR, ebiten.KeyF,
	ebiten.KeyE, ebiten.KeyH, ebiten.KeyF1, ebiten.KeyEscape, ebiten.KeyG, ebiten.KeyComma, ebiten.KeyPeriod,
	ebiten.KeyHome, ebiten.KeyDelete, ebiten.KeyEnter, ebiten.KeyBackspace, ebiten.KeyW, ebiten.KeyA, ebiten.KeyS, ebiten.KeyD,
}

// monkey drives the application with random but deliberate actions: clicks
// (press and release on the same spot), double clicks, right clicks, drags,
// wheel turns, key presses and typing. It finds the panics that hide in
// rarely visited corners of the interface.
func monkey(t *testing.T, a *App, seed uint64, frames int, prepare func(*Game)) {
	t.Helper()
	rng := model.NewRNG(seed)
	screen := ebiten.NewImage(1600, 900)
	x, y := float32(800), float32(450)
	var queue []input.State // frames of the action in progress
	point := func() (float32, float32) {
		switch rng.Intn(6) {
		case 0: // top bar
			return float32(rng.Range(0, 1600)), float32(rng.Range(0, 68))
		case 1: // left panel
			return float32(rng.Range(0, 232)), float32(rng.Range(68, 900))
		case 2: // right panel
			return float32(rng.Range(1284, 1600)), float32(rng.Range(68, 900))
		case 3: // where windows and dialogs sit
			return float32(rng.Range(330, 1270)), float32(rng.Range(150, 750))
		}
		return float32(rng.Range(232, 1284)), float32(rng.Range(68, 900))
	}
	for f := 1; f <= frames; f++ {
		if len(queue) == 0 {
			st := func() input.State { s := frameInput(0); s.X, s.Y = x, y; return s }
			switch act := rng.Intn(10); {
			case act < 5: // click
				x, y = point()
				b := input.Left
				if rng.Chance(0.15) {
					b = input.Right
				}
				mv, dn, up := st(), st(), st()
				dn.Down[b], dn.Pressed[b] = true, true
				dn.DoubleClick = b == input.Left && rng.Chance(0.15)
				dn.Shift = rng.Chance(0.1)
				up.Released[b] = true
				queue = append(queue, mv, dn, up)
			case act < 6: // drag
				x, y = point()
				b := input.Button(rng.Intn(3))
				dn := st()
				dn.Down[b], dn.Pressed[b] = true, true
				queue = append(queue, dn)
				for i := 0; i < 4; i++ {
					x += float32(rng.Range(-60, 60))
					y += float32(rng.Range(-60, 60))
					mid := st()
					mid.Down[b] = true
					queue = append(queue, mid)
				}
				up := st()
				up.Released[b] = true
				queue = append(queue, up)
			case act < 7: // wheel
				x, y = point()
				s := st()
				s.Wheel = float32(rng.Range(-3, 3))
				queue = append(queue, s)
			case act < 9: // key
				s := st()
				s.PressKey(fuzzKeys[rng.Intn(len(fuzzKeys))])
				queue = append(queue, s)
			default: // typing
				s := st()
				s.Chars = []rune{rune('a' + rng.Intn(26)), '7'}
				queue = append(queue, s)
			}
		}
		in := queue[0]
		queue = queue[1:]
		in.Frame = int64(f)
		a.Frame(in)
		if f%4 == 0 {
			a.Draw(screen)
		}
		a.quit = false
		if a.game == nil && rng.Chance(0.01) {
			a.startGame(model.DefaultSettings(seed + uint64(f)))
		}
		if a.game != nil && prepare != nil && f%300 == 1 {
			prepare(a.game)
		}
		if a.game != nil {
			if err := a.game.w.Validate(); err != nil {
				t.Fatalf("seed %d frame %d: world invalid: %v", seed, f, err)
			}
		}
	}
}

// enrich gives the player techs, goods and ships so that the monkey reaches
// every part of the interface, not just the opening.
func enrich(g *Game) {
	w := g.w
	fac := w.Human()
	if fac == nil || !fac.Alive {
		return
	}
	for t := range fac.Techs {
		fac.Techs[t] = true
	}
	fac.RecomputeMods(w.Settings.Difficulty)
	fac.Scrip = 5000
	for _, r := range w.RocksOf(fac.ID) {
		for gd := range r.Colony.Stock {
			r.Colony.Stock[gd] = max(r.Colony.Stock[gd], r.Colony.StorageCap*0.7)
		}
		r.Colony.Pop = max(r.Colony.Pop, 40)
		if w.GarrisonOf(r.ID, fac.ID) == nil {
			fl := model.NewFleet(fac.ID, r.ID)
			fl.Garrison = true
			for _, k := range model.AllShipKinds() {
				fl.Ships = append(fl.Ships, model.NewShip(w.NewShipID(), k, 1))
			}
			w.AddFleet(fl)
		}
	}
}

func TestMonkeyTitleAndSetup(t *testing.T) {
	a := New(Options{Mute: true, Portable: t.TempDir(), FixedStep: true})
	monkey(t, a, 1, 1500, nil)
}

func TestMonkeyGame(t *testing.T) {
	frames := 6000
	if testing.Short() {
		frames = 1200
	}
	for seed := uint64(1); seed <= 3; seed++ {
		st := model.DefaultSettings(seed)
		a := New(Options{Mute: true, Portable: t.TempDir(), FixedStep: true, NewGame: &st})
		monkey(t, a, seed*101, frames, enrich)
	}
}

// A finished game must show its summary and lead back to the title screen.
func TestGameOverScreen(t *testing.T) {
	st := model.DefaultSettings(4)
	st.Rivals = 1
	a := New(Options{Mute: true, Portable: t.TempDir(), FixedStep: true, NewGame: &st})
	w := a.game.w
	// Knock out the rival's Keystone directly.
	rival := w.Factions[1]
	rock := w.Rock(rival.HomeRock)
	fl := model.NewFleet(model.PlayerFaction, rock.ID)
	for i := 0; i < 30; i++ {
		fl.Ships = append(fl.Ships, model.NewShip(w.NewShipID(), model.Hammer, 1))
	}
	fl.Order, fl.Dest, fl.Stance, fl.Home = model.OrderAttack, rock.ID, model.StanceRaze, w.Human().HomeRock
	w.AddFleet(fl)
	screen := ebiten.NewImage(1600, 900)
	a.game.speed = 4
	for f := 1; f <= 4000 && !w.GameOver; f++ {
		a.Frame(frameInput(int64(f)))
	}
	if !w.GameOver || w.Winner != model.PlayerFaction {
		t.Fatalf("game over %v winner %d", w.GameOver, w.Winner)
	}
	for f := 0; f < 5; f++ {
		a.Frame(frameInput(int64(5000 + f)))
		a.Draw(screen)
	}
	if !a.game.ended {
		t.Error("the end of the game was not noticed")
	}
	_ = sim.Directives
}

func BenchmarkFrame(b *testing.B) {
	st := model.DefaultSettings(2)
	a := New(Options{Mute: true, Portable: b.TempDir(), FixedStep: true, NewGame: &st})
	enrich(a.game)
	a.game.speed = 0
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Frame(frameInput(int64(i + 1)))
	}
}

// buildOneOfEach puts a finished building of every kind on the player's home
// rock, plus a construction site and a derelict.
func buildOneOfEach(t *testing.T, g *Game) {
	t.Helper()
	w := g.w
	r := w.Rock(w.Human().HomeRock)
	var free []model.Hex
	for _, h := range r.Keystone().Hex.Spiral(2 * r.Radius) {
		if tl := r.TileAt(h); tl != nil && tl.Buildable() {
			free = append(free, h)
		}
	}
	i := 0
	for _, k := range model.AllBuildingKinds() {
		if k == model.Keystone {
			continue
		}
		b := model.NewActiveBuilding(w.NewBuildingID(), k, g.me, free[i], 1)
		i++
		if err := r.AddBuilding(b); err != nil {
			t.Fatal(err)
		}
	}
	site := model.NewBuilding(w.NewBuildingID(), model.Smelter, g.me, free[i], 1)
	site.Progress = 0.4
	r.AddBuilding(site)
	wreck := model.NewActiveBuilding(w.NewBuildingID(), model.Silo, g.me, free[i+1], 1)
	wreck.MakeDerelict()
	wreck.HP = wreck.MaxHP / 3
	r.AddBuilding(wreck)
}

// TestWalkEveryScreen visits every panel, window and dialog deliberately and
// draws each one, with a colony that has everything there is to show.
func TestWalkEveryScreen(t *testing.T) {
	st := model.DefaultSettings(8)
	a := New(Options{Mute: true, Portable: t.TempDir(), FixedStep: true, NewGame: &st})
	g := a.game
	w := g.w
	enrich(g)
	buildOneOfEach(t, g)
	screen := ebiten.NewImage(1600, 900)
	frame := int64(0)
	show := func(what string, in func(*input.State)) {
		t.Helper()
		for i := 0; i < 3; i++ {
			frame++
			s := frameInput(frame)
			s.X, s.Y = 700, 400
			if in != nil && i == 1 {
				in(&s)
			}
			a.Frame(s)
			a.Draw(screen)
		}
		if a.game != nil {
			if err := a.game.w.Validate(); err != nil {
				t.Fatalf("%s: %v", what, err)
			}
		}
	}
	home := w.Rock(w.Human().HomeRock)
	a.game.sim.Run(30) // let the economy evaluate the new buildings

	// Every building in the inspector, with its special panels.
	for _, b := range home.Buildings {
		g.sel = selection{kind: selBuilding, rock: home.ID, building: b.ID}
		show("inspect "+b.Kind.String(), nil)
	}
	for tab := 0; tab < model.NumCategories; tab++ {
		g.tab = tab
		g.place = model.BuildingsIn(model.Category(tab))[len(model.BuildingsIn(model.Category(tab)))-1]
		show("palette", func(s *input.State) { s.X, s.Y = 760, 470 })
	}
	g.place = model.BuildingNone
	g.sel = selection{kind: selTile, rock: home.ID, hex: home.Keystone().Hex.Neighbor(2)}
	show("tile", nil)

	// Fleets: inspector, orders, freight route.
	fl := w.GarrisonOf(home.ID, g.me)
	g.sel = selection{kind: selFleet, fleet: fl.ID}
	show("fleet", nil)
	g.include[model.Dart] = false
	show("fleet subset", nil)
	other := w.Factions[1].HomeRock
	for _, o := range []model.OrderKind{model.OrderMove, model.OrderSurvey, model.OrderAttack} {
		g.orders = orderPick{active: true, fleet: fl.ID, ships: g.selectedShips(fl), order: o}
		g.view = viewBelt
		show("order pick", nil)
		g.sendOrder(other)
	}
	if len(w.FleetsOf(g.me)) < 2 {
		t.Error("sending part of the garrison away should have split the fleet")
	}
	g.view = viewBelt
	for _, f2 := range w.FleetsOf(g.me) {
		g.sel = selection{kind: selFleet, fleet: f2.ID}
		g.menu = &ctxMenu{rock: other, x: 700, y: 400}
		show("order menu", nil)
		g.menu = &ctxMenu{rock: 3, x: 1590, y: 890}
		show("order menu at the edge", nil)
		g.menu = nil
	}
	for _, r := range w.Rocks {
		g.sel = selection{kind: selRock, rock: r.ID}
		show("rock", nil)
	}
	// A second colony makes freight routes possible.
	var spare *model.Rock
	for _, r := range w.Rocks {
		if r.Owner == model.NoFaction {
			spare = r
			break
		}
	}
	if _, err := w.FoundColony(spare.ID, g.me, bestHex(spare)); err != nil {
		t.Fatal(err)
	}
	enrich(g)
	if fl = w.GarrisonOf(home.ID, g.me); fl == nil {
		t.Fatal("no garrison left at home")
	}
	g.sel = selection{kind: selFleet, fleet: fl.ID}
	g.openRoute(fl, nil)
	show("route window", nil)
	g.win.route.b = spare.ID
	show("route window with target", nil)

	// Freeze time and declare every building running, so that the Exchange
	// (which needs a working dock) shows its trading table.
	g.speed = 0
	for _, b := range home.Buildings {
		if b.Completed() {
			b.Powered, b.Staffed, b.Eff = true, 1, 1
		}
	}
	for _, id := range []windowID{windowResearch, windowFleets, windowExchange, windowHolds, windowCodex} {
		g.gotoRock(home.ID)
		g.openWindow(id)
		show("window", nil)
		if id == windowCodex {
			for tab := 0; tab < 5; tab++ {
				g.win.codexTab, g.win.codexSel = tab, 3
				show("codex", func(s *input.State) { s.Wheel = -2 })
			}
		}
	}
	g.window = windowNone
	g.gotoRock(other) // a rival's surface
	show("rival surface", nil)
	g.gotoRock(home.ID)

	// Dialogs, saving and loading.
	for _, d := range []dialogID{dialogMenu, dialogOptions, dialogSave, dialogLoad} {
		a.openDialog(d)
		show("dialog", nil)
		a.dialog = dialogNone
	}
	if err := a.saveGame("walk test"); err != nil {
		t.Fatal(err)
	}
	a.openDialog(dialogLoad)
	show("load dialog with a save", nil)
	if len(a.dlg.saves) != 1 {
		t.Fatalf("%d saves listed", len(a.dlg.saves))
	}
	tick := w.Tick
	if err := a.loadGame(a.dlg.saves[0].Path); err != nil {
		t.Fatal(err)
	}
	if a.game.w.Tick != tick || a.game.w == w {
		t.Error("loading did not replace the world with the saved one")
	}
	show("after load", nil)
	a.toTitle()
	show("title", nil)
	a.screen = screenSetup
	show("setup", func(s *input.State) { s.Chars = []rune("12") })
	a.openDialog(dialogOptions)
	show("options on title", nil)
}

// bestHex returns a buildable tile near the centre of a rock.
func bestHex(r *model.Rock) model.Hex {
	for _, h := range (model.Hex{}).Spiral(r.Radius) {
		if tl := r.TileAt(h); tl != nil && tl.Buildable() {
			return h
		}
	}
	return model.Hex{}
}
