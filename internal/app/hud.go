package app

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/art"
	"drifthold/internal/gfx"
	"drifthold/internal/model"
	"drifthold/internal/sim"
	"drifthold/internal/ui/kit"
)

const (
	topH       = 68
	paletteW   = 232
	inspectorW = 316
)

// flow lays widgets out top to bottom inside a rectangle.
type flow struct {
	g  *Game
	ui *kit.Ctx
	th *gfx.Theme
	r  gfx.Rect
}

func (g *Game) flow(r gfx.Rect) *flow { return &flow{g: g, ui: g.a.ui, th: g.a.th, r: r} }

func (f *flow) row(h float32) gfx.Rect { return f.r.CutTop(h) }
func (f *flow) gap(h float32)          { f.r.CutTop(h) }

func (f *flow) header(title string) {
	f.r = f.ui.Header(f.r, kit.Upper(title))
}

func (f *flow) kv(k, v string, col gfx.Color) {
	r := f.row(19)
	if col == (gfx.Color{}) {
		col = f.th.Text
	}
	f.ui.Label(r, k, f.th.Small().WithColor(f.th.TextDim))
	f.ui.Label(r, v, f.th.NumSmall().WithColor(col).Aligned(gfx.AlignRight))
}

func (f *flow) text(s string, st gfx.TextStyle) {
	h := f.ui.Text(f.r, s, st)
	f.r.CutTop(h + 4)
}

func (f *flow) bar(label string, frac float32, col gfx.Color) {
	f.ui.Progress(f.row(16), frac, col, label)
	f.gap(4)
}

func (f *flow) button(id, label string, o kit.Btn) bool {
	r := f.row(f.th.RowH)
	f.gap(4)
	return f.ui.Button(id, r, label, o)
}

func icon(id art.IconID) func(cv *gfx.Canvas, r gfx.Rect, col gfx.Color) {
	return func(cv *gfx.Canvas, r gfx.Rect, col gfx.Color) { art.Icon(cv, id, r, col) }
}

// costText describes a cost as "35 Alloy, 20 Silica".
func costText(cost model.Stock) string {
	if cost.IsZero() {
		return "free"
	}
	s := ""
	for g, amt := range cost {
		if amt > 0 {
			if s != "" {
				s += ", "
			}
			s += fmt.Sprintf("%s %s", model.FormatQty(amt), model.Good(g))
		}
	}
	return s
}

// drawCost draws a cost as good symbols with amounts, red where stock is
// lacking, and returns the width used.
func drawCost(cv *gfx.Canvas, th *gfx.Theme, x, y float32, cost model.Stock, stock *model.Stock) float32 {
	x0 := x
	for g, amt := range cost {
		if amt <= 0 {
			continue
		}
		art.Good(cv, model.Good(g), gfx.R(x, y, 12, 12))
		col := th.TextDim
		if stock != nil && stock[g] < amt-model.StockEpsilon {
			col = th.Alert
		}
		w := cv.Text(model.FormatQty(amt), x+14, y-1, th.NumSmall().WithColor(col))
		x += 14 + w + 7
	}
	return x - x0
}

// layout declares the fixed interface panels and returns the area left for
// the world view.
func (g *Game) layout(full gfx.Rect) gfx.Rect {
	top := full.CutTop(topH)
	g.topBar(top)
	left := full.CutLeft(paletteW)
	right := full.CutRight(inspectorW)
	if g.view == viewSurface {
		g.palette(left)
	} else {
		g.beltSidebar(left)
	}
	g.inspector(right)
	return full
}

// ------------------------------------------------------------------ top bar

func (g *Game) topBar(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	ui.Block(r)
	ui.Paint(func(cv *gfx.Canvas) {
		cv.FillRect(r, th.Panel)
		cv.FillRect(gfx.R(r.X, r.Bottom()-1, r.W, th.Hairline), th.Rule)
		cv.FillRect(gfx.R(r.X, r.Y+34, r.W, th.Hairline), th.Border)
	})
	row1 := gfx.R(r.X+8, r.Y+4, r.W-16, 28)
	row2 := gfx.R(r.X+8, r.Y+38, r.W-16, 26)
	fac := g.faction()
	rock := g.cur()

	// Row 1, left: view switch and rock selector.
	label := "Belt map"
	if g.view == viewBelt {
		label = "Surface"
	}
	if ui.Button("view", row1.CutLeft(104), label, kit.Btn{Hotkey: ebiten.KeyTab, Icon: icon(art.IconBelt),
		Tip: &kit.Tip{Title: "Switch view", Body: "Toggle between the rock surface and the map of the belt.", Foot: "Hotkey: Tab"}}) {
		g.toggleView()
	}
	row1.CutLeft(8)
	if ui.Button("rock-", row1.CutLeft(24), "", kit.Btn{Quiet: true, Icon: icon(art.IconArrowLeft)}) {
		g.cycleRock(-1)
	}
	nameR := row1.CutLeft(170)
	if rock != nil {
		owner := th.FactionColor(int(rock.Owner))
		ui.Paint(func(cv *gfx.Canvas) {
			art.RockBadge(cv, rock, nameR.X+12, nameR.MidY(), 9)
			cv.TextFit(gfx.R(nameR.X+28, nameR.Y, nameR.W-28, nameR.H), kit.Upper(rock.Name), th.Head().WithColor(owner))
		})
	}
	if ui.Button("rock+", row1.CutLeft(24), "", kit.Btn{Quiet: true, Icon: icon(art.IconArrowRight)}) {
		g.cycleRock(1)
	}
	row1.CutLeft(12)

	// Row 1, right: menu, speed, date, Scrip.
	if ui.Button("menu", row1.CutRight(34), "", kit.Btn{Icon: icon(art.IconMenu), Tip: &kit.Tip{Title: "Menu", Foot: "Hotkey: Esc"}}) {
		g.a.openDialog(dialogMenu)
	}
	row1.CutRight(8)
	speeds := []struct {
		v   int
		ic  art.IconID
		tip string
	}{{4, art.IconFaster, "Four times speed (3)"}, {2, art.IconFast, "Double speed (2)"}, {1, art.IconPlay, "Normal speed (1)"}, {0, art.IconPause, "Pause (Space)"}}
	for _, s := range speeds {
		if ui.Button(fmt.Sprint("speed", s.v), row1.CutRight(30), "", kit.Btn{Selected: g.speed == s.v, Icon: icon(s.ic), Tip: &kit.Tip{Body: s.tip}}) {
			g.speed = s.v
		}
		row1.CutRight(2)
	}
	row1.CutRight(10)
	dateR := row1.CutRight(92)
	scripR := row1.CutRight(100)
	cycle := g.w.Cycle()
	scrip := 0.0
	if fac != nil {
		scrip = fac.Scrip
	}
	ui.Paint(func(cv *gfx.Canvas) {
		art.Icon(cv, art.IconCycle, gfx.R(dateR.X, dateR.MidY()-8, 16, 16), th.TextDim)
		cv.TextIn(gfx.R(dateR.X+20, dateR.Y, dateR.W-20, dateR.H), fmt.Sprintf("Cycle %d", cycle), th.Num())
		art.Icon(cv, art.IconScrip, gfx.R(scripR.X, scripR.MidY()-8, 16, 16), th.Rule)
		cv.TextIn(gfx.R(scripR.X+20, scripR.Y, scripR.W-20, scripR.H), fmtQty(scrip), th.Num().WithColor(th.Rule))
	})
	ui.TipFor("tip/scrip", scripR, &kit.Tip{Title: "Scrip", Body: "Your Hold's currency. Colonists pay a stipend every cycle; the Exchange turns goods into Scrip and back. Scrip also expedites construction and sponsors settlers."})

	// Row 1, middle: windows.
	wins := []struct {
		id   windowID
		name string
		key  ebiten.Key
		ic   art.IconID
	}{
		{windowResearch, "Research", ebiten.KeyR, art.IconResearch},
		{windowFleets, "Fleets", ebiten.KeyF, art.IconFleet},
		{windowExchange, "Exchange", ebiten.KeyE, art.IconExchange},
		{windowHolds, "Holds", ebiten.KeyH, art.IconHolds},
		{windowCodex, "Codex", ebiten.KeyF1, art.IconHelp},
	}
	for _, wd := range wins {
		if row1.W < 96 {
			break
		}
		o := kit.Btn{Selected: g.window == wd.id, Hotkey: wd.key, Icon: icon(wd.ic)}
		if wd.id == windowResearch && fac != nil && fac.Research.Current == model.TechNone && len(fac.AvailableTechs()) > 0 &&
			sim.ResearchRate(g.w, g.me) > 0 {
			o.Primary = ui.Pulse(1.2) > 0.5 // idle laboratories: nudge the player
		}
		if ui.Button("win/"+wd.name, row1.CutLeft(98), wd.name, o) {
			if g.window == wd.id {
				g.window = windowNone
			} else {
				g.openWindow(wd.id)
			}
		}
		row1.CutLeft(4)
	}

	// Row 2: the colony's stock and vital signs.
	if rock == nil || rock.Colony == nil || rock.Owner != g.me {
		msg := "No colony of yours on this rock."
		if rock != nil && rock.Owner != model.NoFaction {
			if of := g.w.Faction(rock.Owner); of != nil {
				msg = "Colony of " + of.Name + "."
			}
		}
		ui.Label(row2, msg, th.Body().WithColor(th.TextDim))
		return
	}
	c := rock.Colony
	cellW := min(float32(92), (row2.W-470)/model.NumGoods)
	for _, gd := range model.AllGoods() {
		cell := row2.CutLeft(cellW)
		amt, rate, capy := c.Stock[gd], c.NetRate(gd), c.StorageCap
		col := th.Text
		switch {
		case amt >= capy-0.5 && capy > 0:
			col = th.Warn
		case amt < 1 && (gd == model.Food || gd == model.Ice):
			col = th.Alert
		}
		rcol := th.TextDim
		if rate < -0.05 {
			rcol = th.Alert
		} else if rate > 0.05 {
			rcol = th.Good
		}
		gd := gd
		ui.Paint(func(cv *gfx.Canvas) {
			art.Good(cv, gd, gfx.R(cell.X, cell.MidY()-8, 16, 16))
			w := cv.Text(fmtQty(amt), cell.X+20, cell.Y+5, th.Num().WithColor(col))
			if cellW > 76 {
				cv.Text(fmtRate(rate), cell.X+24+w, cell.Y+7, th.NumSmall().WithColor(rcol))
			}
		})
		info := model.GoodInfoOf(gd)
		ui.TipFor("tip/good/"+info.Name, cell, &kit.Tip{Title: info.Name, Body: info.Description, Rows: []kit.TipRow{
			{K: "In store", V: fmt.Sprintf("%s / %s", model.FormatQty(amt), model.FormatQty(capy))},
			{K: "Made per cycle", V: model.FormatQty(c.Produced[gd]), Col: th.Good},
			{K: "Used per cycle", V: model.FormatQty(c.Consumed[gd]), Col: th.Warn},
			{K: "Exchange value", V: fmt.Sprintf("%.1f Scrip", g.w.Market.Price(gd))},
		}})
	}
	stat := func(id string, w float32, ic art.IconID, text string, col gfx.Color, tip *kit.Tip) {
		cell := row2.CutLeft(w)
		ui.Paint(func(cv *gfx.Canvas) {
			art.Icon(cv, ic, gfx.R(cell.X, cell.MidY()-8, 16, 16), col)
			cv.Text(text, cell.X+20, cell.Y+5, th.Num().WithColor(col))
		})
		ui.TipFor("tip/"+id, cell, tip)
	}
	pcol := th.Text
	if c.PowerDeficit() {
		pcol = th.Alert
	}
	stat("power", 118, art.IconPower, fmt.Sprintf("%.0f/%.0f", c.PowerDemand, c.PowerSupply), pcol,
		&kit.Tip{Title: "Power", Body: "Power wanted by your buildings against power generated. In a deficit the lowest-priority buildings shut down and construction slows.",
			Rows: []kit.TipRow{{K: "Generated", V: model.FormatQty(c.PowerSupply)}, {K: "Wanted", V: model.FormatQty(c.PowerDemand)}}})
	hcol := th.Text
	if c.Pop > c.Housing {
		hcol = th.Warn
	}
	stat("pop", 112, art.IconColonists, fmt.Sprintf("%.0f/%.0f", c.Pop, c.Housing), hcol,
		&kit.Tip{Title: "Colonists", Body: "Colonists against housing. A fed, housed colony with decent morale grows by itself; Habitats add housing.",
			Rows: []kit.TipRow{
				{K: "Change per cycle", V: fmtRate(c.PopRate)},
				{K: "Workers busy", V: fmt.Sprintf("%d of %d", c.WorkersUsed, c.Workers)},
				{K: "Workers wanted", V: fmt.Sprint(c.WorkersNeeded)},
				{K: "Life support for", V: model.FormatQty(c.LifeSupport)},
			}})
	mcol := th.Good
	switch {
	case c.Morale < model.MoraleLow:
		mcol = th.Alert
	case c.Morale < model.MoraleHigh:
		mcol = th.Text
	}
	mt := sim.MoraleOf(c, g.w.Tick)
	rows := []kit.TipRow{{K: "Base", V: fmt.Sprintf("%+.0f", mt.Base)}}
	for _, t := range []struct {
		k string
		v float64
	}{{"Fed and breathing", mt.Fed}, {"Starving", mt.Starving}, {"Suffocating", mt.Suffocating},
		{"Overcrowding", mt.Crowding}, {"Commons", mt.Commons}, {"Recently attacked", mt.Attacked}} {
		if t.v != 0 {
			col := th.Good
			if t.v < 0 {
				col = th.Alert
			}
			rows = append(rows, kit.TipRow{K: t.k, V: fmt.Sprintf("%+.0f", t.v), Col: col})
		}
	}
	rows = append(rows, kit.TipRow{K: "Heading for", V: fmt.Sprintf("%.0f", mt.Total())})
	stat("morale", 76, art.IconMorale, fmt.Sprintf("%.0f", c.Morale), mcol,
		&kit.Tip{Title: "Morale", Body: "High morale (70+) makes buildings 10 % more productive; below 30 they lose a quarter, and below 20 colonists leave.", Rows: rows})
	if c.Starving || c.Suffocating {
		msg := "STARVING"
		if c.Suffocating {
			msg = "NO AIR"
		}
		warn := row2.CutLeft(90)
		a := 0.55 + 0.45*ui.Pulse(0.9)
		ui.Paint(func(cv *gfx.Canvas) {
			cv.TextIn(warn, msg, th.Label().WithColor(gfx.Fade(th.Alert, a)))
		})
	}
}

// ------------------------------------------------------------ build palette

// paletteTabs is the order of the category tabs: short names on the first
// row, long ones on the second.
var paletteTabs = [model.NumCategories]model.Category{
	model.CatCore, model.CatPower, model.CatLife, model.CatSpace,
	model.CatIndustry, model.CatScience, model.CatDefence,
}

func (g *Game) palette(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	ui.Panel(r)
	in := r.Inset(th.Pad)
	rock, fac := g.cur(), g.faction()
	if rock == nil || rock.Owner != g.me || fac == nil {
		f := g.flow(in)
		f.header("Build")
		f.text("You can only build on rocks where your Hold has a Keystone.", th.Small().WithColor(th.TextDim))
		return
	}
	in = ui.Header(in, "BUILD")
	// Two rows of category tabs.
	tabs := in.CutTop(52)
	for i, cat := range paletteTabs {
		var cell gfx.Rect
		if i < 4 {
			cell = gfx.R(tabs.X, tabs.Y, tabs.W, 24).Col(i, 4, 2)
		} else {
			cell = gfx.R(tabs.X, tabs.Y+26, tabs.W, 24).Col(i-4, 3, 2)
		}
		if ui.Button("cat/"+cat.String(), cell, cat.String(), kit.Btn{Selected: g.tab == int(cat), Quiet: true, Tight: true}) {
			g.tab = int(cat)
		}
	}
	in.CutTop(6)
	foot := in.CutBottom(30)
	if ui.Button("deposits", foot, "Show deposits", kit.Btn{Selected: g.deposit, Hotkey: ebiten.KeyG,
		Tip: &kit.Tip{Body: "Highlight the deposits you have surveyed.", Foot: "Hotkey: G"}}) {
		g.deposit = !g.deposit
	}
	in.CutBottom(6)

	var kinds []model.BuildingKind
	for _, k := range model.BuildingsIn(model.Category(g.tab)) {
		if model.BuildingDefOf(k).CommandBuildable() {
			kinds = append(kinds, k)
		}
	}
	const rowH = 46
	content := ui.BeginScroll("palette", in, float32(len(kinds))*rowH)
	stock := rock.Colony.Stock
	for _, k := range kinds {
		def := model.BuildingDefOf(k)
		cell := content.CutTop(rowH).Pad(0, 0, 0, 4)
		if !ui.Visible(cell) {
			continue
		}
		locked := !fac.CanBuild(k)
		tip := &kit.Tip{Title: def.Name, Body: def.Role, Rows: []kit.TipRow{
			{K: "Cost", V: costText(def.Cost)},
			{K: "Build time", V: fmt.Sprintf("%.0f s", def.BuildTime)},
		}, Foot: def.Description}
		if def.Power != 0 {
			tip.Rows = append(tip.Rows, kit.TipRow{K: "Power", V: fmt.Sprintf("%+.0f", def.Power)})
		}
		if def.Staff > 0 {
			tip.Rows = append(tip.Rows, kit.TipRow{K: "Workers", V: fmt.Sprint(fac.Mods.StaffNeeded(def.Staff))})
		}
		o := kit.Btn{Selected: g.place == k, Tip: tip}
		if locked {
			o.Disabled = true
			o.Tip = nil
			o.Why = "Requires research: " + def.Unlock.String() + "."
		}
		if ui.Button("build/"+def.Name, cell, "", o) {
			if g.place == k {
				g.place = model.BuildingNone
			} else {
				g.place = k
				g.sel = selection{}
			}
		}
		accent := th.FactionColor(int(g.me))
		ui.Paint(func(cv *gfx.Canvas) {
			ic := gfx.R(cell.X+4, cell.Y+3, 36, 36)
			if locked {
				art.Icon(cv, art.IconLock, ic.Inset(9), th.TextFaint)
				cv.TextFit(gfx.R(cell.X+46, cell.Y+2, cell.W-50, 20), def.Name, th.Body().WithColor(th.TextFaint))
				cv.TextFit(gfx.R(cell.X+46, cell.Y+21, cell.W-50, 16), def.Unlock.String(), th.Small().WithColor(th.TextFaint))
				return
			}
			art.BuildingIcon(cv, k, ic, accent)
			cv.TextFit(gfx.R(cell.X+46, cell.Y+2, cell.W-50, 20), def.Name, th.Body())
			drawCost(cv, th, cell.X+46, cell.Y+24, def.Cost, &stock)
		})
	}
	ui.EndScroll()
}

// -------------------------------------------------------------- belt sidebar

func (g *Game) beltSidebar(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	ui.Panel(r)
	in := ui.Header(r.Inset(th.Pad), "THE SHOAL")
	const rowH = 30
	content := ui.BeginScroll("rocks", in, float32(len(g.w.Rocks))*rowH)
	for _, rock := range g.w.Rocks {
		row := content.CutTop(rowH).Pad(0, 0, 0, 3)
		if !ui.Visible(row) {
			continue
		}
		rock := rock
		selected := g.sel.kind == selRock && g.sel.rock == rock.ID
		if ui.Button(fmt.Sprint("rock/", rock.ID), row, "", kit.Btn{Selected: selected, Quiet: true}) {
			if selected {
				g.gotoRock(rock.ID)
			} else {
				g.sel = selection{kind: selRock, rock: rock.ID}
				g.bcam.focus(g, rock.ID)
			}
		}
		col := th.TextDim
		if rock.Owner != model.NoFaction {
			col = th.FactionColor(int(rock.Owner))
		}
		note := rock.Class.String()
		if rock.Salvage != nil {
			note = "Derelict hulk"
		}
		ui.Paint(func(cv *gfx.Canvas) {
			art.RockBadge(cv, rock, row.X+13, row.MidY(), 8)
			cv.TextFit(gfx.R(row.X+28, row.Y, row.W-110, row.H), rock.Name, th.Body().WithColor(col))
			cv.TextFit(gfx.R(row.Right()-84, row.Y, 80, row.H), note, th.Small().WithColor(th.TextDim).Aligned(gfx.AlignRight))
		})
	}
	ui.EndScroll()
}
