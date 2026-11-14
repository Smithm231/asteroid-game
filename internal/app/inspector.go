package app

import (
	"fmt"

	"drifthold/internal/art"
	"drifthold/internal/audio"
	"drifthold/internal/gfx"
	"drifthold/internal/model"
	"drifthold/internal/sim"
	"drifthold/internal/ui/kit"
)

// orderPick is a fleet order waiting for the player to choose its target rock
// on the belt map.
type orderPick struct {
	active  bool
	fleet   model.FleetID
	ships   []model.ShipID
	order   model.OrderKind
	stance  model.Stance
	retreat float64
}

func (g *Game) inspector(r gfx.Rect) {
	ui, th := g.a.ui, g.a.th
	ui.Panel(r)
	f := g.flow(r.Inset(th.Pad + 2))
	switch g.sel.kind {
	case selBuilding:
		rock := g.w.Rock(g.sel.rock)
		if b := g.w.Building(g.sel.rock, g.sel.building); b != nil && rock != nil {
			g.inspectBuilding(f, rock, b)
			return
		}
		g.sel = selection{}
	case selFleet:
		if fl := g.w.Fleet(g.sel.fleet); fl != nil && sim.Visible(g.w, g.me, fl) {
			g.inspectFleet(f, fl)
			return
		}
		g.sel = selection{}
	case selRock:
		if rock := g.w.Rock(g.sel.rock); rock != nil {
			g.inspectRock(f, rock)
			return
		}
		g.sel = selection{}
	case selTile:
		if rock := g.w.Rock(g.sel.rock); rock != nil {
			g.inspectTile(f, rock, g.sel.hex)
		}
	}
	if g.view == viewSurface {
		g.inspectColony(f)
	} else if g.sel.kind == selNone {
		f.header("Belt map")
		f.text("Click a rock or a fleet to inspect it. Double-click a rock to view its surface. With one of your fleets selected, right-click a rock to give it orders.", th.Small().WithColor(th.TextDim))
	}
}

func (g *Game) hpBar(f *flow, hp, maxHP float64) {
	frac := float32(0)
	if maxHP > 0 {
		frac = float32(hp / maxHP)
	}
	col := f.th.Good
	switch {
	case frac < 0.35:
		col = f.th.Alert
	case frac < 0.7:
		col = f.th.Warn
	}
	f.bar(fmt.Sprintf("%.0f / %.0f", hp, maxHP), frac, col)
}

func (g *Game) inspectBuilding(f *flow, rock *model.Rock, b *model.Building) {
	ui, th := f.ui, f.th
	def := b.Def()
	mine := b.Owner == g.me
	onMyRock := rock.Owner == g.me
	f.header(def.Name)
	head := f.row(56)
	accent := th.FactionColor(int(b.Owner))
	look := art.BuildingLook{Accent: accent, Progress: float32(b.Progress), Derelict: b.Derelict(), Off: !b.Working() && b.Completed(), Time: g.clock}
	ui.Paint(func(cv *gfx.Canvas) {
		art.Building(cv, b.Kind, head.X+28, head.MidY(), 26, look)
	})
	owner := "Derelict"
	if of := g.w.Faction(b.Owner); of != nil {
		owner = of.Name
	}
	ui.Label(gfx.R(head.X+64, head.Y+4, head.W-64, 18), owner, th.Small().WithColor(accent))
	status, scol := "Working", th.Good
	switch {
	case b.Derelict():
		status, scol = "Derelict", th.TextDim
	case b.Constructing():
		status, scol = fmt.Sprintf("Under construction, %s left", fmtTime(b.RemainingBuildSeconds())), th.Info
	case b.Idle != model.IdleNone:
		status, scol = b.Idle.String(), th.Warn
	case !b.Working() && b.Kind != model.Conduit:
		status, scol = "Idle", th.TextDim
	}
	ui.Label(gfx.R(head.X+64, head.Y+24, head.W-64, 18), status, th.Body().WithColor(scol))
	f.gap(4)
	g.hpBar(f, b.HP, b.MaxHP)

	if b.Constructing() {
		f.bar(fmt.Sprintf("%.0f %%", b.Progress*100), float32(b.Progress), th.Info)
	}
	if mine && !b.Constructing() {
		if def.Power != 0 {
			f.kv("Power", fmt.Sprintf("%+.0f", b.PowerOut), gfx.Color{})
		}
		if need := g.faction().Mods.StaffNeeded(def.Staff); need > 0 {
			f.kv("Workers", fmt.Sprintf("%d / %d", b.Workers, need), gfx.Color{})
		}
		if b.Eff > 0 && def.Staff > 0 {
			f.kv("Efficiency", fmt.Sprintf("%.0f %%", b.Eff*100), gfx.Color{})
		}
		if def.ShieldPool > 0 {
			f.bar(fmt.Sprintf("Shield %.0f / %.0f", b.Shield, def.ShieldPool), float32(b.Shield/def.ShieldPool), th.Info)
		}
	}
	f.gap(2)
	f.text(def.Role, th.Small().WithColor(th.Text))
	f.text(def.Description, th.Small().WithColor(th.TextDim))

	switch {
	case b.Derelict() && onMyRock:
		cost := def.Cost.Scaled(model.RecommissionCostFrac)
		if f.button("recommission", "Recommission", kit.Btn{Primary: true, Tip: &kit.Tip{Body: "Put this derelict back to work for 40 % of its original cost.", Rows: []kit.TipRow{{K: "Cost", V: costText(cost)}}}}) {
			g.issue(model.RecommissionCmd(g.me, rock.ID, b.ID))
		}
		if f.button("salvage", "Salvage", kit.Btn{Tip: &kit.Tip{Body: "Break the derelict up for 30 % of its Alloy, Silica and Parts."}}) {
			if g.issue(model.DemolishCmd(g.me, rock.ID, b.ID)) {
				g.a.snd.Play(audio.Demolish)
				g.sel = selection{}
			}
		}
	case mine && b.Constructing():
		price := sim.ExpediteQuote(b)
		if f.button("expedite", fmt.Sprintf("Expedite (%.0f Scrip)", price), kit.Btn{Disabled: g.faction().Scrip < price, Why: "Not enough Scrip.",
			Tip: &kit.Tip{Body: "Pay Scrip to finish this construction at once."}}) {
			g.issue(model.ExpediteCmd(g.me, rock.ID, b.ID))
		}
		if f.button("cancel", "Cancel construction", kit.Btn{Tip: &kit.Tip{Body: "Cancel and get the full cost back."}}) {
			if g.issue(model.DemolishCmd(g.me, rock.ID, b.ID)) {
				g.sel = selection{}
			}
		}
	case mine && b.Kind != model.Keystone:
		row := f.row(th.RowH)
		f.gap(4)
		lbl := "Switch off"
		if !b.Enabled {
			lbl = "Switch on"
		}
		if ui.Button("onoff", row.Col(0, 2, 6), lbl, kit.Btn{Icon: icon(art.IconOnOff), Tip: &kit.Tip{Body: "A building that is switched off uses no power and no workers."}}) {
			g.issue(model.SetEnabledCmd(g.me, rock.ID, b.ID, !b.Enabled))
		}
		if ui.Button("demolish", row.Col(1, 2, 6), "Demolish", kit.Btn{Icon: icon(art.IconDemolish), Tip: &kit.Tip{Body: "Tear it down and recover half its Alloy, Silica and Parts.", Foot: "Hotkey: Delete"}}) {
			if g.issue(model.DemolishCmd(g.me, rock.ID, b.ID)) {
				g.a.snd.Play(audio.Demolish)
				g.sel = selection{}
				return
			}
		}
		if def.Staff > 0 || def.PowerDraw() > 0 {
			prow := f.row(th.RowH)
			f.gap(4)
			ui.Label(prow.CutLeft(64), "Priority", th.Small().WithColor(th.TextDim))
			for i, name := range []string{"Low", "Normal", "High"} {
				if ui.Button("prio/"+name, prow.Col(i, 3, 4), name, kit.Btn{Selected: b.Priority == i,
					Tip: &kit.Tip{Body: "When power or workers run short, high-priority buildings are served first."}}) {
					g.issue(model.SetPriorityCmd(g.me, rock.ID, b.ID, i))
				}
			}
		}
	}
	if !mine || !b.Completed() {
		return
	}
	switch {
	case def.QueueSize > 0:
		g.inspectCradle(f, rock, b)
	case def.DriverRange > 0:
		g.inspectDriver(f, rock, b)
	}
}

func (g *Game) inspectCradle(f *flow, rock *model.Rock, b *model.Building) {
	ui, th := f.ui, f.th
	fac := g.faction()
	f.gap(4)
	f.header("Shipyard")
	f.kv("Berths in use", fmt.Sprintf("%d / %d", g.w.BerthsUsed(g.me), g.w.BerthCap(g.me)), gfx.Color{})
	for i, k := range b.Queue {
		row := f.row(24)
		def := model.ShipDefOf(k)
		name := def.Name
		if i == 0 {
			ui.Progress(gfx.R(row.X, row.Y+2, row.W-28, 20), float32(b.QueueProgress), th.RuleDim, name)
		} else {
			ui.Label(gfx.R(row.X+6, row.Y, row.W-34, row.H), name, th.Small().WithColor(th.TextDim))
		}
		if ui.Button(fmt.Sprint("unqueue", i), gfx.R(row.Right()-24, row.Y+2, 22, 20), "", kit.Btn{Quiet: true, Icon: icon(art.IconClose), Tip: &kit.Tip{Body: "Cancel for a full refund."}}) {
			g.issue(model.CancelShipCmd(g.me, rock.ID, b.ID, i))
			return
		}
	}
	f.gap(4)
	stock := rock.Colony.Stock
	accent := th.FactionColor(int(g.me))
	for _, k := range model.AllShipKinds() {
		def := model.ShipDefOf(k)
		row := f.row(40)
		f.gap(3)
		tip := &kit.Tip{Title: def.Name, Body: def.Role, Foot: def.Description, Rows: []kit.TipRow{
			{K: "Cost", V: costText(def.Cost)}, {K: "Build time", V: fmt.Sprintf("%.0f s", def.BuildTime)},
			{K: "Berths", V: fmt.Sprint(def.Berths)}, {K: "Hull", V: fmt.Sprintf("%.0f (%s)", def.MaxHP, def.Hull)},
			{K: "Speed", V: fmt.Sprintf("%.0f u/s", def.Speed)},
		}}
		if def.Armed() {
			tip.Rows = append(tip.Rows, kit.TipRow{K: "Damage: ships / buildings", V: fmt.Sprintf("%.0f / %.0f", def.DPSShips, def.DPSBuildings)})
		}
		o := kit.Btn{Tip: tip}
		if !fac.CanBuildShip(k) {
			o.Disabled, o.Tip, o.Why = true, nil, "Requires research: "+def.Unlock.String()+"."
		}
		locked := o.Disabled
		if ui.Button("ship/"+def.Name, row, "", o) {
			g.issue(model.QueueShipCmd(g.me, rock.ID, b.ID, k))
		}
		ui.Paint(func(cv *gfx.Canvas) {
			if locked {
				art.Icon(cv, art.IconLock, gfx.R(row.X+10, row.MidY()-8, 16, 16), th.TextFaint)
				cv.TextFit(gfx.R(row.X+42, row.Y, row.W-46, row.H), def.Name, th.Body().WithColor(th.TextFaint))
				return
			}
			art.ShipIcon(cv, k, gfx.R(row.X+4, row.Y+4, 32, 32), accent)
			cv.TextFit(gfx.R(row.X+42, row.Y+1, row.W-46, 20), def.Name, th.Body())
			drawCost(cv, th, row.X+42, row.Y+23, def.Cost, &stock)
		})
	}
}

func (g *Game) inspectDriver(f *flow, rock *model.Rock, b *model.Building) {
	ui, th := f.ui, f.th
	def := b.Def()
	f.gap(4)
	f.header("Target")
	any := false
	for _, o := range g.w.Rocks {
		if o.Owner == model.NoFaction || o.Owner == g.me {
			continue
		}
		any = true
		d := g.w.RockDist(rock.ID, o.ID)
		note := fmt.Sprintf("%.0f u", d)
		col := th.TextDim
		if d <= def.DriverRange {
			note += " in range"
			col = th.Good
		}
		row := f.row(26)
		f.gap(2)
		if ui.Button(fmt.Sprint("driver/", o.ID), row, "", kit.Btn{Selected: b.DriverTarget == o.ID, Quiet: true}) {
			t := o.ID
			if b.DriverTarget == o.ID {
				t = model.NoRock
			}
			g.issue(model.SetDriverTargetCmd(g.me, rock.ID, b.ID, t))
		}
		o := o
		ui.Paint(func(cv *gfx.Canvas) {
			cv.TextFit(gfx.R(row.X+8, row.Y, row.W-110, row.H), o.Name, th.Body().WithColor(th.FactionColor(int(o.Owner))))
			cv.TextFit(gfx.R(row.Right()-104, row.Y, 98, row.H), note, th.Small().WithColor(col).Aligned(gfx.AlignRight))
		})
	}
	if !any {
		f.text("No rival colonies to aim at.", th.Small().WithColor(th.TextDim))
	} else {
		f.text(fmt.Sprintf("Range %.0f u. The driver fires whenever its target drifts into range.", def.DriverRange), th.Small().WithColor(th.TextDim))
	}
}

func (g *Game) inspectTile(f *flow, rock *model.Rock, h model.Hex) {
	th := f.th
	t := rock.TileAt(h)
	if t == nil || !t.Terrain.Solid() {
		return
	}
	f.header("Ground")
	f.kv("Terrain", t.Terrain.String(), gfx.Color{})
	switch {
	case !rock.KnownTo(h, g.me):
		f.kv("Deposit", "Unsurveyed", th.TextDim)
	case t.HasDeposit():
		f.kv("Deposit", fmt.Sprintf("%s, %s left", t.Deposit, model.FormatQty(t.Amount)), th.GoodColor(int(t.Deposit)))
	default:
		f.kv("Deposit", "None", th.TextDim)
	}
	if t.Rubble {
		f.kv("Rubble", "Building here recovers 10 Alloy", th.TextDim)
	}
	f.gap(8)
}

// selectedShips returns the ships of fl the player has ticked, or nil when
// that is the whole fleet.
func (g *Game) selectedShips(fl *model.Fleet) []model.ShipID {
	if g.incFor != fl.ID {
		g.incFor = fl.ID
		for k := range g.include {
			g.include[k] = true
		}
	}
	var ids []model.ShipID
	for i := range fl.Ships {
		if g.include[fl.Ships[i].Kind] {
			ids = append(ids, fl.Ships[i].ID)
		}
	}
	if len(ids) == len(fl.Ships) {
		return nil
	}
	return ids
}

func fleetTitle(w *model.World, fl *model.Fleet) string {
	switch {
	case fl.InTransit():
		return "Bound for " + rockLabel(w, fl.Dest)
	case fl.Garrison:
		return "Garrison of " + rockLabel(w, fl.At)
	}
	return "In orbit of " + rockLabel(w, fl.At)
}

func rockLabel(w *model.World, id model.RockID) string {
	if r := w.Rock(id); r != nil {
		return r.Name
	}
	return "nowhere"
}

func (g *Game) inspectFleet(f *flow, fl *model.Fleet) {
	ui, th := f.ui, f.th
	mine := fl.Owner == g.me
	accent := th.FactionColor(int(fl.Owner))
	f.header("Fleet")
	if of := g.w.Faction(fl.Owner); of != nil {
		f.ui.Label(f.row(18), of.Name, th.Small().WithColor(accent))
	}
	f.ui.Label(f.row(20), fleetTitle(g.w, fl), th.Body())
	order := fl.Order.String()
	if fl.Order == model.OrderAttack {
		order += " (" + fl.Stance.String() + ")"
	}
	f.kv("Orders", order, gfx.Color{})
	if eta, ok := sim.ETA(g.w, fl); ok {
		f.kv("Arrives in", fmtTime(eta), gfx.Color{})
	}
	if c := fl.CargoUsed(); c > 0.5 {
		f.kv("Cargo", fl.CargoTotal().String(), gfx.Color{})
	}
	g.hpBar(f, fl.TotalHP(), fl.TotalMaxHP())
	if mine {
		g.selectedShips(fl) // make sure the ticks belong to this fleet
	}
	for _, k := range model.AllShipKinds() {
		n := fl.Count(k)
		if n == 0 {
			continue
		}
		row := f.row(30)
		f.gap(2)
		def := model.ShipDefOf(k)
		on := g.include[k] || !mine
		if mine && len(fl.Ships) > 1 {
			if ui.Button("inc/"+def.Name, row, "", kit.Btn{Selected: on, Quiet: true, Tip: &kit.Tip{Body: "Click to include or leave out these ships when you give an order. Ordering only some ships splits them off as a new fleet."}}) {
				g.include[k] = !g.include[k]
			}
		}
		ui.Paint(func(cv *gfx.Canvas) {
			col := th.Text
			if !on {
				col = th.TextFaint
			}
			art.ShipIcon(cv, k, gfx.R(row.X+4, row.Y+3, 24, 24), accent)
			cv.TextFit(gfx.R(row.X+36, row.Y, row.W-80, row.H), def.Name, th.Body().WithColor(col))
			cv.TextIn(gfx.R(row.Right()-44, row.Y, 38, row.H), fmt.Sprintf("×%d", n), th.Num().WithColor(col).Aligned(gfx.AlignRight))
		})
	}
	if !mine {
		return
	}
	ids := g.selectedShips(fl)
	has := func(pred func(model.ShipDef) bool) bool {
		for i := range fl.Ships {
			if g.include[fl.Ships[i].Kind] && pred(fl.Ships[i].Def()) {
				return true
			}
		}
		return false
	}
	anySel := has(func(model.ShipDef) bool { return true })
	f.gap(6)
	f.header("Orders")
	pick := func(id, label string, order model.OrderKind, ok bool, why string, tip string) {
		sel := g.orders.active && g.orders.fleet == fl.ID && g.orders.order == order
		if f.button("order/"+id, label, kit.Btn{Disabled: !ok || !anySel, Why: why, Selected: sel, Tip: &kit.Tip{Body: tip}}) {
			if sel {
				g.orders.active = false
				return
			}
			g.orders = orderPick{active: true, fleet: fl.ID, ships: ids, order: order, stance: g.win.stance, retreat: g.win.retreat}
			g.view = viewBelt
			g.place = model.BuildingNone
		}
	}
	pick("move", "Move to…", model.OrderMove, true, "", "Fly to a rock and hold there (or join your garrison). Then click the destination on the belt map.")
	pick("attack", "Attack…", model.OrderAttack, has(model.ShipDef.Armed), "Those ships carry no weapons.", "Fly to a rival rock and fight. Then click the target on the belt map.")
	if has(model.ShipDef.Armed) {
		row := f.row(th.RowH)
		f.gap(3)
		for i, s := range []model.Stance{model.StanceSuppress, model.StanceRaid, model.StanceRaze} {
			tips := []string{"Defences first, then the Keystone.", "Industry and power first.", "Straight for the Keystone."}
			if ui.Button("stance/"+s.String(), row.Col(i, 3, 4), s.String(), kit.Btn{Selected: g.win.stance == s, Tip: &kit.Tip{Title: s.String(), Body: tips[i]}}) {
				g.win.stance = s
			}
		}
		row = f.row(th.RowH)
		f.gap(4)
		ui.Label(row.CutLeft(64), "Retreat", th.Small().WithColor(th.TextDim))
		for i, lv := range model.RetreatLevels() {
			name := []string{"Never", "At 25 %", "At 50 %"}[i]
			if ui.Button("retreat/"+name, row.Col(i, 3, 4), name, kit.Btn{Selected: g.win.retreat == lv, Tip: &kit.Tip{Body: "Break off the attack when the fleet's hull strength falls this low."}}) {
				g.win.retreat = lv
			}
		}
	}
	pick("survey", "Survey…", model.OrderSurvey, has(func(d model.ShipDef) bool { return d.CanSurvey }), "A survey needs a Prospector.", "Reveal every deposit of a rock. Takes 15 seconds in orbit.")
	pick("colonize", "Found colony…", model.OrderColonize, has(func(d model.ShipDef) bool { return d.CanColonize }), "Founding a colony needs a Seedship.", "Land a Seedship on an unclaimed rock to found a new Keystone.")
	if f.button("order/route", "Freight route…", kit.Btn{Disabled: !has(func(d model.ShipDef) bool { return d.Cargo > 0 }) || g.w.ColonyCount(g.me) < 2,
		Why: "A freight route needs a Hauler and two colonies.", Tip: &kit.Tip{Body: "Shuttle goods between two of your colonies automatically."}}) {
		g.openRoute(fl, ids)
	}
	row := f.row(th.RowH)
	f.gap(4)
	if ui.Button("order/return", row.Col(0, 2, 6), "Return home", kit.Btn{Disabled: !anySel}) {
		g.issue(model.FleetOrderCmd(g.me, fl.ID, ids, model.OrderReturn, model.NoRock))
	}
	if ui.Button("order/scuttle", row.Col(1, 2, 6), "Scuttle", kit.Btn{Disabled: !anySel, Tip: &kit.Tip{Body: "Destroy the selected ships to free their berths. This cannot be undone."}}) {
		g.issue(model.ScuttleCmd(g.me, fl.ID, ids))
	}
}

func (g *Game) inspectRock(f *flow, rock *model.Rock) {
	ui, th := f.ui, f.th
	f.header(rock.Name)
	owner, ocol := "Unclaimed", th.TextDim
	if of := g.w.Faction(rock.Owner); of != nil {
		owner, ocol = of.Name, th.FactionColor(int(rock.Owner))
	}
	f.kv("Held by", owner, ocol)
	f.kv("Class", rock.Class.String(), th.RockColor(int(rock.Class)))
	f.kv("Sunlight", fmt.Sprintf("%.0f %%", rock.Solar*100), gfx.Color{})
	f.kv("Size", fmt.Sprintf("%d tiles", rock.BlobSize()), gfx.Color{})
	if cur := g.cur(); cur != nil && cur.ID != rock.ID {
		f.kv("Distance from "+cur.Name, fmt.Sprintf("%.0f u", g.w.RockDist(cur.ID, rock.ID)), gfx.Color{})
	}
	if rock.Salvage != nil {
		f.kv("Derelict hulk", fmt.Sprintf("%s, %.0f Scrip", rock.Salvage.Goods, rock.Salvage.Scrip), th.Info)
	}
	// What the player knows lies in the ground.
	var known model.Stock
	knownTiles := 0
	for i := range rock.Tiles {
		t := &rock.Tiles[i]
		if !t.Terrain.Solid() {
			continue
		}
		h, _ := rock.HexAt(i)
		if rock.KnownTo(h, g.me) {
			knownTiles++
			if t.HasDeposit() {
				known[t.Deposit] += t.Amount
			}
		}
	}
	f.gap(4)
	f.header("Deposits")
	switch {
	case knownTiles == 0:
		f.text("Unsurveyed. Send a Prospector to learn what lies in this rock.", th.Small().WithColor(th.TextDim))
	default:
		any := false
		for _, gd := range model.RawGoods() {
			if known[gd] > 0 {
				f.kv(gd.String(), fmtQty(known[gd]), th.GoodColor(int(gd)))
				any = true
			}
		}
		if !any {
			f.text("Nothing found so far.", th.Small().WithColor(th.TextDim))
		}
		if !rock.Surveyed(g.me) {
			f.text("Partly surveyed.", th.Small().WithColor(th.TextDim))
		}
	}
	if c := rock.Colony; c != nil && rock.Owner == g.me {
		f.gap(4)
		f.header("Colony")
		f.kv("Colonists", fmt.Sprintf("%.0f / %.0f", c.Pop, c.Housing), gfx.Color{})
		f.kv("Morale", fmt.Sprintf("%.0f", c.Morale), gfx.Color{})
	}
	f.gap(4)
	fleets := 0
	for _, fl := range g.w.FleetsAt(rock.ID) {
		if !sim.Visible(g.w, g.me, fl) {
			continue
		}
		if fleets == 0 {
			f.header("In orbit")
		}
		fleets++
		g.fleetRow(f, fl)
	}
	f.gap(6)
	if f.button("viewrock", "View surface", kit.Btn{Primary: true}) {
		g.gotoRock(rock.ID)
	}
	_ = ui
}

// fleetRow is a one-line clickable summary of a fleet.
func (g *Game) fleetRow(f *flow, fl *model.Fleet) {
	ui, th := f.ui, f.th
	row := f.row(28)
	f.gap(2)
	if ui.Button(fmt.Sprint("fleet/", fl.ID), row, "", kit.Btn{Quiet: true, Selected: g.sel.kind == selFleet && g.sel.fleet == fl.ID}) {
		g.sel = selection{kind: selFleet, fleet: fl.ID}
	}
	col := th.FactionColor(int(fl.Owner))
	name := "Fleet"
	if fl.Garrison {
		name = "Garrison"
	}
	if of := g.w.Faction(fl.Owner); of != nil && fl.Owner != g.me {
		name = of.Name
	}
	ui.Paint(func(cv *gfx.Canvas) {
		art.Icon(cv, art.IconFleet, gfx.R(row.X+6, row.MidY()-8, 16, 16), col)
		cv.TextFit(gfx.R(row.X+28, row.Y, row.W-100, row.H), name, th.Body().WithColor(col))
		cv.TextIn(gfx.R(row.Right()-70, row.Y, 64, row.H), fmt.Sprintf("%d ships", len(fl.Ships)), th.Small().WithColor(th.TextDim).Aligned(gfx.AlignRight))
	})
}

func (g *Game) inspectColony(f *flow) {
	th := f.th
	rock := g.cur()
	if rock == nil {
		return
	}
	if rock.Owner != g.me || rock.Colony == nil {
		g.inspectRock(f, rock)
		return
	}
	c := rock.Colony
	f.header("Colony on " + rock.Name)
	f.kv("Colonists", fmt.Sprintf("%.0f / %.0f housed", c.Pop, c.Housing), gfx.Color{})
	f.kv("Growth per cycle", fmtRate(c.PopRate), gfx.Color{})
	f.kv("Workers", fmt.Sprintf("%d busy, %d idle, %d wanted", c.WorkersUsed, c.IdleWorkers(), c.WorkersNeeded), gfx.Color{})
	f.kv("Life support", fmt.Sprintf("for %.0f", c.LifeSupport), gfx.Color{})
	f.kv("Storage", fmt.Sprintf("%.0f of each good", c.StorageCap), gfx.Color{})
	f.kv("Sunlight", fmt.Sprintf("%.0f %%", rock.Solar*100), gfx.Color{})
	// Things that want the player's attention.
	var warns []string
	switch {
	case c.Suffocating && c.Stock[model.Ice] < 1:
		warns = append(warns, "Out of Ice: colonists are suffocating. Mine, haul in or buy Ice.")
	case c.Suffocating:
		warns = append(warns, "Not enough life support: build an Aerator.")
	}
	if c.Starving {
		warns = append(warns, "Out of Food: build a Greenhouse or buy Food.")
	}
	if c.PowerDeficit() {
		warns = append(warns, "Power deficit: build Sunvanes or switch something off.")
	}
	if c.WorkersNeeded > c.Workers {
		warns = append(warns, "More jobs than workers: build Habitats so the colony can grow, or set priorities.")
	}
	if c.Pop >= c.Housing-0.5 && c.WorkersNeeded >= c.Workers {
		warns = append(warns, "Housing is full: the colony cannot grow.")
	}
	for _, wn := range warns {
		f.text("• "+wn, th.Small().WithColor(th.Warn))
	}
	sites := 0
	for _, b := range rock.Buildings {
		if b.Constructing() && b.Owner == g.me {
			sites++
		}
	}
	if sites > 0 {
		f.kv("Under construction", fmt.Sprint(sites), th.Info)
	}
	f.gap(6)
	n := 0
	for _, fl := range g.w.FleetsAt(rock.ID) {
		if !sim.Visible(g.w, g.me, fl) {
			continue
		}
		if n == 0 {
			f.header("In orbit")
		}
		n++
		g.fleetRow(f, fl)
	}
	if n == 0 && sites == 0 && len(warns) == 0 {
		f.gap(4)
		f.text("Pick a structure from the Build list and click a tile next to one of your structures. Click any structure to inspect it.", th.Small().WithColor(th.TextDim))
	}
}
