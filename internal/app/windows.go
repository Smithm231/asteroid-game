package app

import (
	"fmt"

	"drifthold/internal/art"
	"drifthold/internal/gfx"
	"drifthold/internal/input"
	"drifthold/internal/model"
	"drifthold/internal/sim"
	"drifthold/internal/ui/kit"
)

type windowID int

const (
	windowNone windowID = iota
	windowResearch
	windowFleets
	windowExchange
	windowHolds
	windowCodex
	windowRoute
	windowLog
)

// windowState is what the windows remember between frames.
type windowState struct {
	stance   model.Stance
	retreat  float64
	codexTab int
	codexSel int
	route    routeDraft
}

type routeDraft struct {
	fleet     model.FleetID
	ships     []model.ShipID
	a, b      model.RockID
	out, back [model.NumGoods]bool
	reserve   float64
}

func (g *Game) openWindow(id windowID) {
	g.window = id
	g.place = model.BuildingNone
	g.a.ui.OnSound(kit.SoundOpen)
}

func (g *Game) openRoute(fl *model.Fleet, ids []model.ShipID) {
	d := routeDraft{fleet: fl.ID, ships: ids, a: fl.At, b: model.NoRock, reserve: 60}
	if r := g.w.Rock(d.a); r == nil || r.Owner != g.me {
		d.a = g.faction().HomeRock
	}
	for _, r := range g.w.RocksOf(g.me) {
		if r.ID != d.a {
			d.b = r.ID
			break
		}
	}
	d.out[model.Alloy], d.out[model.Parts] = true, true
	d.back[model.Ice], d.back[model.Ore] = true, true
	g.win.route = d
	g.openWindow(windowRoute)
}

func (g *Game) windowFrame(full gfx.Rect) {
	if g.window == windowNone {
		return
	}
	ui := g.a.ui
	type spec struct {
		title string
		w, h  float32
		body  func(gfx.Rect)
	}
	specs := map[windowID]spec{
		windowResearch: {"Research", 940, 600, g.researchWindow},
		windowFleets:   {"Fleets", 720, 520, g.fleetsWindow},
		windowExchange: {"The Exchange", 700, 500, g.exchangeWindow},
		windowHolds:    {"Holds of the Shoal", 820, 520, g.holdsWindow},
		windowCodex:    {"Codex", 900, 600, g.codexWindow},
		windowRoute:    {"Freight route", 560, 470, g.routeWindow},
		windowLog:      {"Log", 700, 540, g.logWindow},
	}
	sp := specs[g.window]
	r := full.Center(min(sp.w, full.W-20), min(sp.h, full.H-20))
	content, closed := ui.Window(fmt.Sprint("win", g.window), sp.title, r)
	sp.body(content)
	ui.EndWindow()
	if closed {
		g.window = windowNone
	}
}

// ----------------------------------------------------------------- research

func (g *Game) researchWindow(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	fac := g.faction()
	rs := &fac.Research
	side := r.CutRight(250)
	r.CutRight(14)
	// Tech cards in three tier columns.
	for tier := 1; tier <= 3; tier++ {
		col := r.Col(tier-1, 3, 12)
		ui.Label(col.CutTop(20), fmt.Sprintf("TIER %d", tier), th.Label().WithColor(th.TextDim))
		for _, def := range model.Techs() {
			if def.Tier != tier {
				continue
			}
			card := col.CutTop(48)
			col.CutTop(5)
			t := def.ID
			known, current, queued := fac.Has(t), rs.Current == t, false
			for _, q := range rs.Queue {
				queued = queued || q == t
			}
			avail := fac.CanResearch(t)
			tip := &kit.Tip{Title: def.Name, Body: def.EffectText, Foot: def.Description,
				Rows: []kit.TipRow{{K: "Research points", V: model.FormatQty(def.Cost)}}}
			if def.Lumen > 0 {
				tip.Rows = append(tip.Rows, kit.TipRow{K: "Lumen to begin", V: model.FormatQty(def.Lumen), Col: th.GoodColor(int(model.Lumen))})
			}
			for _, req := range def.Requires {
				col := th.Good
				if !fac.Has(req) {
					col = th.Alert
				}
				tip.Rows = append(tip.Rows, kit.TipRow{K: "Requires", V: req.String(), Col: col})
			}
			o := kit.Btn{Tip: tip, Selected: current}
			if ui.Button("tech/"+def.Name, card, "", o) && !known {
				switch {
				case current:
					g.issue(model.CancelResearchCmd(g.me, -1))
				case avail && !g.a.in.Shift && !queued:
					g.issue(model.SetResearchCmd(g.me, t))
				case !queued:
					g.issue(model.QueueResearchCmd(g.me, t))
				}
			}
			frac := float32(fac.ResearchFrac(t))
			ui.Paint(func(cv *gfx.Canvas) {
				nameCol, note, noteCol := th.Text, fmt.Sprintf("%.0f pts", def.Cost), th.TextDim
				switch {
				case known:
					nameCol, note, noteCol = th.Good, "Known", th.Good
				case current:
					nameCol, note, noteCol = th.Rule, fmt.Sprintf("%.0f %%", frac*100), th.Rule
				case queued:
					note, noteCol = "Queued", th.Info
				case !avail:
					nameCol = th.TextFaint
					note = "Locked"
				}
				if def.Lumen > 0 && !known {
					note += fmt.Sprintf(" · %.0f Lumen", def.Lumen)
				}
				cv.TextFit(gfx.R(card.X+10, card.Y+5, card.W-20, 18), def.Name, th.Body().WithColor(nameCol))
				cv.TextFit(gfx.R(card.X+10, card.Y+25, card.W-20, 16), note, th.Small().WithColor(noteCol))
				if frac > 0 && !known {
					bar := gfx.R(card.X+4, card.Bottom()-5, card.W-8, 2)
					cv.FillRect(bar, th.Well)
					cv.FillRect(gfx.R(bar.X, bar.Y, bar.W*frac, bar.H), th.Rule)
				}
			})
		}
	}
	// Sidebar: the current project and the queue.
	f := g.flow(side)
	f.header("Laboratories")
	f.kv("Points per cycle", model.FormatQty(sim.ResearchRate(g.w, g.me)), gfx.Color{})
	f.kv("Technologies known", fmt.Sprintf("%d / %d", fac.TechCount(), model.NumTechs), gfx.Color{})
	f.gap(8)
	f.header("Current project")
	if rs.Current == model.TechNone {
		f.text("Nothing. Click a technology to begin.", th.Small().WithColor(th.Warn))
	} else {
		def := model.TechDefOf(rs.Current)
		f.ui.Label(f.row(20), def.Name, th.Body().WithColor(th.Rule))
		f.bar(fmt.Sprintf("%.0f / %.0f", rs.Progress[rs.Current], def.Cost), float32(fac.ResearchFrac(rs.Current)), th.Rule)
		if rs.Stalled {
			f.text(fmt.Sprintf("Waiting for %.0f Lumen to begin.", def.Lumen), th.Small().WithColor(th.Warn))
		} else if rate := sim.ResearchRate(g.w, g.me); rate > 0 {
			left := (def.Cost - rs.Progress[rs.Current]) / rate
			f.kv("Done in", fmt.Sprintf("%.1f cycles", left), gfx.Color{})
		} else {
			f.text("No working Laboratory.", th.Small().WithColor(th.Warn))
		}
	}
	f.gap(8)
	f.header("Queue")
	if len(rs.Queue) == 0 {
		f.text("Shift-click a technology to queue it behind the current project.", th.Small().WithColor(th.TextDim))
	}
	for i, t := range rs.Queue {
		row := f.row(24)
		ui.Label(gfx.R(row.X, row.Y, row.W-28, row.H), fmt.Sprintf("%d. %s", i+1, t), th.Small())
		if ui.Button(fmt.Sprint("unq", i), gfx.R(row.Right()-24, row.Y+1, 22, 22), "", kit.Btn{Quiet: true, Icon: icon(art.IconClose)}) {
			g.issue(model.CancelResearchCmd(g.me, i))
			return
		}
	}
}

// ------------------------------------------------------------------- fleets

func (g *Game) fleetsWindow(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	head := r.CutTop(24)
	ui.Label(head, fmt.Sprintf("Berths in use: %d of %d.  Keystones give 6 berths each, Dock Rings 6 more.",
		g.w.BerthsUsed(g.me), g.w.BerthCap(g.me)), th.Small().WithColor(th.TextDim))
	r.CutTop(6)
	fleets := g.w.FleetsOf(g.me)
	if len(fleets) == 0 {
		ui.Text(r, "You have no ships. Research Orbital Flight, build a Launch Cradle and order ships from it.", th.Body().WithColor(th.TextDim))
		return
	}
	const rowH = 46
	content := ui.BeginScroll("fleets", r, float32(len(fleets))*rowH)
	for _, fl := range fleets {
		row := content.CutTop(rowH).Pad(0, 0, 0, 4)
		if !ui.Visible(row) {
			continue
		}
		fl := fl
		if ui.Button(fmt.Sprint("fl/", fl.ID), row, "", kit.Btn{Selected: g.sel.kind == selFleet && g.sel.fleet == fl.ID}) {
			g.sel = selection{kind: selFleet, fleet: fl.ID}
			if fl.InTransit() {
				g.view = viewBelt
				g.bcam.cx, g.bcam.cy = fl.Pos.X, fl.Pos.Y
			} else {
				g.gotoRock(fl.At)
				g.sel = selection{kind: selFleet, fleet: fl.ID}
			}
			g.window = windowNone
		}
		summary := ""
		for _, k := range model.AllShipKinds() {
			if n := fl.Count(k); n > 0 {
				if summary != "" {
					summary += ", "
				}
				summary += fmt.Sprintf("%d %s", n, k)
			}
		}
		status := fl.Order.String()
		if eta, ok := sim.ETA(g.w, fl); ok {
			status += ", arrives in " + fmtTime(eta)
		}
		if fl.Route != nil {
			status = fmt.Sprintf("Hauling %s – %s", rockLabel(g.w, fl.Route.A), rockLabel(g.w, fl.Route.B))
		}
		accent := th.FactionColor(int(g.me))
		ui.Paint(func(cv *gfx.Canvas) {
			art.ShipIcon(cv, fl.Ships[0].Kind, gfx.R(row.X+6, row.Y+5, 30, 30), accent)
			cv.TextFit(gfx.R(row.X+46, row.Y+3, row.W-240, 20), fleetTitle(g.w, fl), th.Body())
			cv.TextFit(gfx.R(row.X+46, row.Y+22, row.W-240, 16), summary, th.Small().WithColor(th.TextDim))
			cv.TextFit(gfx.R(row.Right()-190, row.Y, 180, row.H), status, th.Small().WithColor(th.Info).Aligned(gfx.AlignRight))
		})
	}
	ui.EndScroll()
}

// -------------------------------------------------------------------- route

func (g *Game) routeWindow(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	d := &g.win.route
	fl := g.w.Fleet(d.fleet)
	if fl == nil || fl.Owner != g.me {
		g.window = windowNone
		return
	}
	f := g.flow(r)
	f.text("Haulers load the outbound goods at the first colony, unload at the second, load the return goods and come back, for as long as the route stands.", th.Small().WithColor(th.TextDim))
	pick := func(title string, cur *model.RockID, other model.RockID, id string) {
		f.ui.Label(f.row(18), title, th.Label().WithColor(th.TextDim))
		rocks := g.w.RocksOf(g.me)
		row := f.row(th.RowH)
		f.gap(6)
		n := max(len(rocks), 3)
		for i, rk := range rocks {
			if ui.Button(fmt.Sprint(id, rk.ID), row.Col(i, n, 4), rk.Name, kit.Btn{Selected: *cur == rk.ID, Disabled: rk.ID == other}) {
				*cur = rk.ID
			}
		}
	}
	pick("FROM", &d.a, d.b, "ra")
	pick("TO", &d.b, d.a, "rb")
	goods := func(title string, sel *[model.NumGoods]bool, id string) {
		f.ui.Label(f.row(18), title, th.Label().WithColor(th.TextDim))
		row := f.row(th.RowH)
		f.gap(6)
		for i, gd := range model.AllGoods() {
			gd := gd
			if ui.Button(id+gd.String(), row.Col(i, model.NumGoods, 3), "", kit.Btn{Selected: sel[gd], Tip: &kit.Tip{Title: gd.String()},
				Icon: func(cv *gfx.Canvas, r gfx.Rect, _ gfx.Color) { art.Good(cv, gd, r.Center(16, 16)) }}) {
				sel[gd] = !sel[gd]
			}
		}
	}
	goods("CARRY OUT", &d.out, "out")
	goods("CARRY BACK", &d.back, "back")
	row := f.row(th.RowH)
	f.gap(10)
	ui.Label(row.CutLeft(230), fmt.Sprintf("Always leave %.0f of each good behind", d.reserve), th.Small())
	ui.Slider("reserve", row, &d.reserve, 0, 300)
	d.reserve = float64(int(d.reserve/10)) * 10
	if f.button("route/ok", "Start hauling", kit.Btn{Primary: true, Disabled: d.b == model.NoRock || d.a == d.b, Why: "Choose two different colonies."}) {
		rt := model.Route{A: d.a, B: d.b, Out: d.out, Back: d.back, Reserve: d.reserve}
		if g.issue(model.SetRouteCmd(g.me, d.fleet, d.ships, rt)) {
			g.window = windowNone
		}
	}
}

// ----------------------------------------------------------------- exchange

func (g *Game) exchangeWindow(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	rock := g.cur()
	fac := g.faction()
	if rock == nil || rock.Owner != g.me || !sim.CanTrade(rock) {
		name := "this rock"
		if rock != nil {
			name = rock.Name
		}
		msg := "Trading at " + name + " needs a working Exchange Dock there."
		if !fac.Has(model.CharterTrade) {
			msg += " Research Charter Trade to unlock it."
		}
		ui.Text(r, msg, th.Body().WithColor(th.TextDim))
		return
	}
	c := rock.Colony
	head := r.CutTop(24)
	ui.Label(head, fmt.Sprintf("Trading at %s.  Scrip: %s.  Storage: %.0f of each good.", rock.Name, fmtQty(fac.Scrip), c.StorageCap), th.Small().WithColor(th.TextDim))
	r.CutTop(4)
	cols := r.CutTop(20)
	ui.Label(gfx.R(cols.X+150, cols.Y, 80, 20), "IN STORE", th.Label().WithColor(th.TextDim))
	ui.Label(gfx.R(cols.X+236, cols.Y, 90, 20), "PRICE", th.Label().WithColor(th.TextDim))
	for _, gd := range model.AllGoods() {
		row := r.CutTop(34)
		gd := gd
		amt := c.Stock[gd]
		press := g.w.Market.Pressure[gd]
		ui.Paint(func(cv *gfx.Canvas) {
			cv.FillRect(gfx.R(row.X, row.Bottom()-1, row.W, th.Hairline), th.Border)
			art.Good(cv, gd, gfx.R(row.X+6, row.MidY()-9, 18, 18))
			cv.TextIn(gfx.R(row.X+34, row.Y, 110, row.H), gd.String(), th.Body())
			cv.TextIn(gfx.R(row.X+150, row.Y, 70, row.H), fmtQty(amt), th.Num())
			pcol := th.Text
			if press > 0.1 {
				pcol = th.Alert
			} else if press < -0.1 {
				pcol = th.Good
			}
			cv.TextIn(gfx.R(row.X+236, row.Y, 90, row.H), fmt.Sprintf("%.2f", g.w.Market.Price(gd)), th.Num().WithColor(pcol))
		})
		btns := gfx.R(row.X+330, row.Y+4, row.W-330, row.H-8)
		for i, q := range []float64{10, 50} {
			cost := sim.BuyCost(g.w, gd, q)
			if ui.Button(fmt.Sprint("buy", gd, q), btns.Col(i, 4, 6), fmt.Sprintf("Buy %.0f", q),
				kit.Btn{Disabled: fac.Scrip < cost || c.Free(gd) < q, Why: "Not enough Scrip or storage.", Tip: &kit.Tip{Body: fmt.Sprintf("Costs %.0f Scrip.", cost)}}) {
				g.issue(model.TradeCmd(g.me, rock.ID, gd, q))
			}
			gain := sim.SellRevenue(g.w, gd, q)
			if ui.Button(fmt.Sprint("sell", gd, q), btns.Col(i+2, 4, 6), fmt.Sprintf("Sell %.0f", q),
				kit.Btn{Disabled: amt < q, Why: "Not enough in store.", Tip: &kit.Tip{Body: fmt.Sprintf("Pays %.0f Scrip.", gain)}}) {
				g.issue(model.TradeCmd(g.me, rock.ID, gd, -q))
			}
		}
	}
	r.CutTop(12)
	row := r.CutTop(th.RowH + 4)
	why := ""
	switch {
	case !c.SponsorReady(g.w.Tick):
		why = "Settlers were sponsored here too recently."
	case c.Pop+model.SponsorColonists > c.Housing:
		why = "Not enough free housing."
	case fac.Scrip < model.SponsorCost:
		why = "Not enough Scrip."
	}
	if ui.Button("sponsor", row.CutLeft(280), fmt.Sprintf("Sponsor %.0f settlers (%.0f Scrip)", model.SponsorColonists, model.SponsorCost),
		kit.Btn{Disabled: why != "", Why: why, Tip: &kit.Tip{Body: "Pay the passage of new colonists. Once per cycle per colony."}}) {
		g.issue(model.SponsorCmd(g.me, rock.ID))
	}
	ui.Label(row.Pad(14, 0, 0, 0), "Prices rise as you buy and fall as you sell.", th.Small().WithColor(th.TextDim))
}

// -------------------------------------------------------------------- holds

func (g *Game) holdsWindow(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	f := g.flow(r)
	f.header("Your colonies")
	hdr := f.row(18)
	colsX := []float32{0, 190, 280, 350, 440, 510, 580, 650}
	names := []string{"ROCK", "COLONISTS", "MORALE", "POWER", "ALLOY", "FOOD", "ICE", ""}
	for i, n := range names {
		ui.Label(gfx.R(hdr.X+colsX[i], hdr.Y, 90, 18), n, th.Label().WithColor(th.TextDim))
	}
	for _, rock := range g.w.RocksOf(g.me) {
		row := f.row(30)
		c := rock.Colony
		rock := rock
		ui.Paint(func(cv *gfx.Canvas) {
			cv.FillRect(gfx.R(row.X, row.Bottom()-1, row.W, th.Hairline), th.Border)
			art.RockBadge(cv, rock, row.X+10, row.MidY(), 8)
			cv.TextFit(gfx.R(row.X+26, row.Y, 160, row.H), rock.Name, th.Body())
			cell := func(i int, s string, col gfx.Color) {
				cv.TextIn(gfx.R(row.X+colsX[i], row.Y, 86, row.H), s, th.Num().WithColor(col))
			}
			pc := th.Text
			if c.Starving || c.Suffocating {
				pc = th.Alert
			}
			cell(1, fmt.Sprintf("%.0f/%.0f", c.Pop, c.Housing), pc)
			cell(2, fmt.Sprintf("%.0f", c.Morale), th.Text)
			wc := th.Text
			if c.PowerDeficit() {
				wc = th.Alert
			}
			cell(3, fmt.Sprintf("%.0f/%.0f", c.PowerDemand, c.PowerSupply), wc)
			cell(4, fmtQty(c.Stock[model.Alloy]), th.Text)
			cell(5, fmtQty(c.Stock[model.Food]), th.Text)
			cell(6, fmtQty(c.Stock[model.Ice]), th.Text)
		})
		if ui.Button(fmt.Sprint("go/", rock.ID), gfx.R(row.X+colsX[7], row.Y+3, 70, 24), "View", kit.Btn{}) {
			g.gotoRock(rock.ID)
			g.window = windowNone
		}
	}
	f.gap(14)
	f.header("Rival Holds")
	for _, fac := range g.w.Factions {
		if fac.ID == g.me {
			continue
		}
		row := f.row(30)
		col := th.FactionColor(fac.ColorIndex)
		status := fmt.Sprintf("%d colonies", g.w.ColonyCount(fac.ID))
		if !fac.Alive {
			status, col = fmt.Sprintf("Eliminated in cycle %d", model.CycleOfTick(fac.EliminatedTick)), th.TextFaint
		}
		blurb := fac.Personality.Archetype() + ". " + fac.Personality.Blurb()
		ui.Paint(func(cv *gfx.Canvas) {
			cv.FillRect(gfx.R(row.X, row.MidY()-6, 12, 12), col)
			cv.TextFit(gfx.R(row.X+22, row.Y, 190, row.H), fac.Name, th.Body().WithColor(col))
			cv.TextFit(gfx.R(row.X+220, row.Y, 130, row.H), status, th.Small())
			cv.TextFit(gfx.R(row.X+350, row.Y, row.W-350, row.H), blurb, th.Small().WithColor(th.TextDim))
		})
	}
}

// ---------------------------------------------------------------------- log

// logWindow lists the messages addressed to the player, newest first. Clicking
// one that names a rock goes there.
func (g *Game) logWindow(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	var entries []model.LogEntry
	for i := len(g.w.Log) - 1; i >= 0; i-- {
		if e := g.w.Log[i]; e.VisibleTo(g.me) {
			entries = append(entries, e)
		}
	}
	if len(entries) == 0 {
		ui.Text(r, "Nothing has happened yet.", th.Body().WithColor(th.TextDim))
		return
	}
	const rowH = 26
	content := ui.BeginScroll("log", r, float32(len(entries))*rowH)
	for i, e := range entries {
		row := content.CutTop(rowH)
		if !ui.Visible(row) {
			continue
		}
		e := e
		jump := g.w.Rock(e.Rock) != nil
		if jump && ui.Button(fmt.Sprint("log/", e.Seq), row.Pad(0, 0, 0, 2), "", kit.Btn{Quiet: true}) {
			g.gotoRock(e.Rock)
			g.window = windowNone
		}
		col := th.SeverityColor(int(e.Sev))
		odd := i%2 == 1
		ui.Paint(func(cv *gfx.Canvas) {
			if odd {
				cv.FillRect(row.Pad(0, 0, 0, 2), gfx.Fade(th.PanelHi, 0.5))
			}
			cv.FillRect(gfx.R(row.X, row.Y+3, 3, row.H-8), col)
			cv.TextIn(gfx.R(row.X+12, row.Y, 70, row.H-2), fmt.Sprintf("Cycle %d", model.CycleOfTick(e.Tick)), th.NumSmall().WithColor(th.TextDim))
			cv.TextFit(gfx.R(row.X+88, row.Y, row.W-96, row.H-2), e.Text, th.Small())
		})
	}
	ui.EndScroll()
}

// -------------------------------------------------------------------- codex

const guideText = `Your Hold lives while it has a Keystone. Destroy every rival Keystone to win.

BUILDING. Pick a structure from the Build list, then click a tile next to one of your structures. Conduits are cheap tiles that extend your reach. Right-click or Esc stops placing.

THE ALLOY LINE. Bore Rigs mine the deposits on their own tile and the six around it. Smelters turn Ore into Alloy, which almost everything is built from. The Keystone reclaims a trickle of Alloy on its own, so you can always recover.

PEOPLE. Colonists need Food (Greenhouses), air (Aerators) and Ice. Habitats add housing; a fed, housed colony grows by itself. Every working building needs workers and most need power. When either runs short, set priorities or switch buildings off.

RESEARCH. Laboratories earn research points. Open Research (R) and choose a project; Shift-click queues one.

SHIPS. Research Orbital Flight and build a Launch Cradle. Select a fleet by clicking its ships in orbit or on the belt map, then give orders from the panel on the right, or right-click a rock on the belt map. Leaving your own rock costs Fuel, made from Ice by a Cracker.

THE BELT. Rocks circle Emberfall at different speeds, so distances keep changing. Inner rocks get more sunlight; Ice is found further out.

HOTKEYS. Tab: surface / belt map. Space: pause. 1, 2, 3: speed. W A S D or arrows: pan. Mouse wheel: zoom. Right-drag: pan. G: show deposits. Comma and period: previous / next colony. R, F, E, H, L: Research, Fleets, Exchange, Holds, Log. F1: this Codex. F5: quick save. F11: full screen. Delete: demolish. Esc: cancel / menu.`

func (g *Game) codexWindow(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	tabs := []string{"Guide", "Structures", "Ships", "Research", "Goods"}
	if ui.Tabs("codex", r.CutTop(28), tabs, &g.win.codexTab) {
		g.win.codexSel = 0
	}
	r.CutTop(10)
	if g.win.codexTab == 0 {
		content := ui.BeginScroll("guide", r, 760)
		ui.Text(content.Pad(4, 0, 12, 0), guideText, th.Body())
		ui.EndScroll()
		return
	}
	type entry struct {
		name  string
		draw  func(cv *gfx.Canvas, r gfx.Rect)
		lines []string
		rows  []kit.TipRow
	}
	var entries []entry
	accent := th.FactionColor(int(g.me))
	switch g.win.codexTab {
	case 1:
		for _, def := range model.Buildings() {
			def := def
			e := entry{name: def.Name, lines: []string{def.Role, def.Description}}
			e.draw = func(cv *gfx.Canvas, r gfx.Rect) { art.BuildingIcon(cv, def.Kind, r, accent) }
			e.rows = []kit.TipRow{{K: "Category", V: def.Category.String()}, {K: "Cost", V: costText(def.Cost)},
				{K: "Build time", V: fmt.Sprintf("%.0f s", def.BuildTime)}, {K: "Power", V: fmt.Sprintf("%+.0f", def.Power)},
				{K: "Workers", V: fmt.Sprint(def.Staff)}, {K: "Hit points", V: model.FormatQty(def.MaxHP)}}
			if def.Unlock != model.TechNone {
				e.rows = append(e.rows, kit.TipRow{K: "Unlocked by", V: def.Unlock.String()})
			}
			entries = append(entries, e)
		}
	case 2:
		for _, def := range model.Ships() {
			def := def
			e := entry{name: def.Name, lines: []string{def.Role, def.Description}}
			e.draw = func(cv *gfx.Canvas, r gfx.Rect) { art.ShipIcon(cv, def.Kind, r, accent) }
			e.rows = []kit.TipRow{{K: "Hull", V: def.Hull.String()}, {K: "Cost", V: costText(def.Cost)},
				{K: "Build time", V: fmt.Sprintf("%.0f s", def.BuildTime)}, {K: "Berths", V: fmt.Sprint(def.Berths)},
				{K: "Hit points", V: model.FormatQty(def.MaxHP)}, {K: "Speed", V: fmt.Sprintf("%.0f u/s", def.Speed)},
				{K: "Damage to ships", V: fmt.Sprintf("%.0f per second", def.DPSShips)},
				{K: "Damage to structures", V: fmt.Sprintf("%.0f per second", def.DPSBuildings)},
				{K: "Unlocked by", V: def.Unlock.String()}}
			entries = append(entries, e)
		}
	case 3:
		for _, def := range model.Techs() {
			e := entry{name: def.Name, lines: []string{def.EffectText, def.Description}}
			e.rows = []kit.TipRow{{K: "Tier", V: fmt.Sprint(def.Tier)}, {K: "Research points", V: model.FormatQty(def.Cost)}}
			if def.Lumen > 0 {
				e.rows = append(e.rows, kit.TipRow{K: "Lumen to begin", V: model.FormatQty(def.Lumen)})
			}
			for _, req := range def.Requires {
				e.rows = append(e.rows, kit.TipRow{K: "Requires", V: req.String()})
			}
			entries = append(entries, e)
		}
	case 4:
		for _, info := range model.Goods() {
			info := info
			e := entry{name: info.Name, lines: []string{info.Description}}
			e.draw = func(cv *gfx.Canvas, r gfx.Rect) { art.Good(cv, info.Good, r.Inset(10)) }
			kind := "Produced"
			if info.Raw {
				kind = "Mined"
			}
			e.rows = []kit.TipRow{{K: "Kind", V: kind}, {K: "Base value", V: fmt.Sprintf("%.1f Scrip", info.BaseValue)}}
			entries = append(entries, e)
		}
	}
	list := r.CutLeft(230)
	r.CutLeft(16)
	content := ui.BeginScroll(fmt.Sprint("codexlist", g.win.codexTab), list, float32(len(entries))*28)
	for i, e := range entries {
		if ui.Button(fmt.Sprint("cx", g.win.codexTab, i), content.CutTop(28).Pad(0, 0, 0, 2), e.name,
			kit.Btn{Quiet: true, Selected: g.win.codexSel == i, Left: true, Align: gfx.AlignLeft}) {
			g.win.codexSel = i
		}
	}
	ui.EndScroll()
	if g.win.codexSel >= len(entries) {
		g.win.codexSel = 0
	}
	e := entries[g.win.codexSel]
	f := g.flow(r)
	head := f.row(64)
	if e.draw != nil {
		ui.Paint(func(cv *gfx.Canvas) { e.draw(cv, gfx.R(head.X, head.Y, 60, 60)) })
		head.CutLeft(74)
	}
	ui.Label(head, e.name, th.Title())
	f.gap(8)
	for i, l := range e.lines {
		st := th.Body()
		if i > 0 {
			st = st.WithColor(th.TextDim)
		}
		f.text(l, st)
		f.gap(4)
	}
	f.gap(6)
	for _, row := range e.rows {
		f.kv(row.K, row.V, gfx.Color{})
	}
}

// ------------------------------------------------------------------ overlay

// overlayFrame declares what floats over the world view: the directive
// tracker, toasts, the paused banner and the end-of-game summary.
func (g *Game) overlayFrame(full gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	in := g.a.in
	area := gfx.R(full.X+paletteW, full.Y+topH, full.W-paletteW-inspectorW, full.H-topH)

	// Directive tracker, top-left of the world view.
	if d, i, ok := sim.CurrentDirective(g.w, g.me); ok && g.window == windowNone {
		box := gfx.R(area.X+12, area.Y+12, 330, 74)
		ui.Block(box)
		ui.Paint(func(cv *gfx.Canvas) {
			cv.Panel(box, 5, gfx.CornersDiag, gfx.WithAlpha(th.Panel, 0.93), th.RuleDim)
			art.Icon(cv, art.IconDirectives, gfx.R(box.X+10, box.Y+9, 16, 16), th.Rule)
			cv.TextFit(gfx.R(box.X+32, box.Y+6, box.W-120, 20), fmt.Sprintf("DIRECTIVE %d OF %d", i+1, len(sim.Directives())), th.Label().WithColor(th.Rule))
			cv.TextFit(gfx.R(box.Right()-90, box.Y+6, 80, 20), fmt.Sprintf("+%.0f Scrip", d.Reward), th.Small().WithColor(th.TextDim).Aligned(gfx.AlignRight))
			cv.TextFit(gfx.R(box.X+10, box.Y+27, box.W-20, 18), d.Title, th.Body().Weighted(gfx.Medium))
			lines := cv.WrapText(d.Hint, box.W-20, th.Small())
			if len(lines) > 1 {
				lines = lines[:1]
				lines[0] = cv.Ellipsize(d.Hint, box.W-20, th.Small())
			}
			cv.TextLines(lines, box.X+10, box.Y+48, th.Small().WithColor(th.TextDim))
		})
		ui.TipFor("tip/directive", box, &kit.Tip{Title: d.Title, Body: d.Hint})
	}

	// Toasts, bottom-left of the world view, newest lowest.
	y := area.Bottom() - 12
	for i := len(g.toasts) - 1; i >= 0; i-- {
		t := &g.toasts[i]
		t.age += in.Dt
		const life = 10
		if t.age > life {
			continue
		}
		alpha := min(1, (life-t.age)*1.2)
		st := th.Small()
		lines := gfx.WrapText(t.text, 380, st, in.Scale)
		h := float32(len(lines))*gfx.LineHeight(st, in.Scale) + 12
		box := gfx.R(area.X+12, y-h, 404, h)
		y -= h + 5
		col := th.SeverityColor(int(t.sev))
		rock := t.rock
		if g.window == windowNone {
			ui.Block(box)
			if ui.Hovered(box) && in.Pressed[input.Left] && g.w.Rock(rock) != nil {
				g.gotoRock(rock)
			}
		}
		ui.Paint(func(cv *gfx.Canvas) {
			cv.FillRect(box, gfx.WithAlpha(th.Well, 0.9*alpha))
			cv.FillRect(gfx.R(box.X, box.Y, 3, box.H), gfx.Fade(col, alpha))
			cv.TextLines(lines, box.X+12, box.Y+6, st.WithColor(gfx.Fade(th.Text, alpha)))
		})
	}
	// Drop expired toasts.
	for len(g.toasts) > 0 && g.toasts[0].age > 10 {
		g.toasts = g.toasts[1:]
	}

	if g.speed == 0 && !g.w.GameOver {
		box := gfx.R(area.MidX()-60, area.Bottom()-46, 120, 28)
		a := 0.6 + 0.4*ui.Pulse(1.6)
		ui.Paint(func(cv *gfx.Canvas) {
			cv.Panel(box, 5, gfx.CornersDiag, gfx.WithAlpha(th.Well, 0.9), gfx.Fade(th.Rule, a))
			cv.TextIn(box, "PAUSED", th.Label().WithColor(gfx.Fade(th.Rule, a)).Aligned(gfx.AlignCenter))
		})
	}

	if g.w.GameOver && g.a.dialog == dialogNone {
		g.gameOver(full)
	}
}

func (g *Game) gameOver(full gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	old := ui.SetLayer(kit.LayerWindow)
	defer ui.SetLayer(old)
	ui.Scrim()
	box := full.Center(520, 400)
	won := g.w.Winner == g.me
	title, sub, col := "THE SHOAL IS YOURS", "Every rival Keystone has fallen.", th.Rule
	if !won {
		title, sub, col = "YOUR HOLD HAS FALLEN", "Your last Keystone is gone.", th.Alert
		if wf := g.w.Faction(g.w.Winner); wf != nil {
			sub = wf.Name + " holds the Shoal."
		}
	}
	ui.Block(box)
	ui.Paint(func(cv *gfx.Canvas) {
		cv.Shadow(box.Translate(0, 8), 24, th.Shadow)
		cv.Panel(box, 10, gfx.CornersDiag, th.Panel, col)
		cv.Text(title, box.MidX(), box.Y+28, th.Title().WithColor(col).Aligned(gfx.AlignCenter))
		cv.Text(sub, box.MidX(), box.Y+64, th.Body().WithColor(th.TextDim).Aligned(gfx.AlignCenter))
	})
	f := g.flow(box.Pad(60, 104, 60, 20))
	st := g.faction().Stats
	f.kv("Cycles played", fmt.Sprint(model.CycleOfTick(g.w.GameOverTick)), gfx.Color{})
	f.kv("Largest population", fmt.Sprintf("%.0f", st.PeakPop), gfx.Color{})
	f.kv("Structures built", fmt.Sprint(st.BuildingsBuilt), gfx.Color{})
	f.kv("Technologies researched", fmt.Sprint(st.TechsResearched), gfx.Color{})
	f.kv("Colonies founded", fmt.Sprint(st.ColoniesFounded), gfx.Color{})
	f.kv("Ships built / lost", fmt.Sprintf("%d / %d", st.ShipsBuilt, st.ShipsLost), gfx.Color{})
	f.kv("Enemy ships destroyed", fmt.Sprint(st.ShipsDestroyed), gfx.Color{})
	f.kv("Keystones destroyed", fmt.Sprint(st.KeystonesDestroyed), gfx.Color{})
	f.gap(16)
	if f.button("over/title", "Return to the title screen", kit.Btn{Primary: true}) {
		g.a.toTitle()
	}
}
