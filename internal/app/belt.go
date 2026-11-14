package app

import (
	"fmt"
	"math"

	"drifthold/internal/art"
	"drifthold/internal/gfx"
	"drifthold/internal/input"
	"drifthold/internal/model"
	"drifthold/internal/sim"
	"drifthold/internal/ui/kit"
)

// beltCam is the camera of the belt map.
type beltCam struct {
	cx, cy  float64 // belt position at the centre of the view
	zoom    float32 // points per u; 0 = fit the whole belt
	drag    bool
	dragged float32
	lastX   float32
	lastY   float32
}

func (c *beltCam) focus(g *Game, rock model.RockID) {
	p := g.w.RockPos(rock)
	c.cx, c.cy = p.X, p.Y
}

// ctxMenu is the order menu opened by right-clicking a rock on the belt map.
type ctxMenu struct {
	rock model.RockID
	x, y float32
}

func (g *Game) beltFrame(area gfx.Rect) {
	in, ui, th := g.a.in, g.a.ui, g.a.th
	w := g.w
	cam := &g.bcam
	fit := min(area.W, area.H) / float32(2*model.OrbitRMax*1.12)
	if cam.zoom == 0 {
		cam.zoom = fit
	}
	dx, dy, hover := g.panInput(area, &cam.drag, &cam.dragged, &cam.lastX, &cam.lastY)
	cam.cx -= float64(dx / cam.zoom)
	cam.cy -= float64(dy / cam.zoom)
	if hover && in.Wheel != 0 {
		old := cam.zoom
		cam.zoom = max(fit*0.8, min(fit*8, cam.zoom*float32(math.Pow(1.15, float64(in.Wheel)))))
		// Keep the belt position under the cursor fixed.
		wx := cam.cx + float64((in.X-area.MidX())/old)
		wy := cam.cy + float64((in.Y-area.MidY())/old)
		cam.cx = wx - float64((in.X-area.MidX())/cam.zoom)
		cam.cy = wy - float64((in.Y-area.MidY())/cam.zoom)
	}
	lim := float64(model.OrbitRMax * 1.2)
	cam.cx = math.Max(-lim, math.Min(lim, cam.cx))
	cam.cy = math.Max(-lim, math.Min(lim, cam.cy))
	zoom := cam.zoom
	scr := func(p model.Vec2) (float32, float32) {
		return area.MidX() + float32(p.X-cam.cx)*zoom, area.MidY() + float32(p.Y-cam.cy)*zoom
	}

	// Hover.
	hoverRock := model.NoRock
	var hoverFleet model.FleetID
	visible := sim.VisibleFleets(w, g.me)
	if hover && g.menu == nil {
		best := float32(18 * 18)
		for _, r := range w.Rocks {
			x, y := scr(w.RockPos(r.ID))
			if d := (x-in.X)*(x-in.X) + (y-in.Y)*(y-in.Y); d < best {
				best, hoverRock = d, r.ID
			}
		}
		best = 12 * 12
		for _, fl := range visible {
			if !fl.InTransit() {
				continue
			}
			x, y := scr(fl.Pos)
			if d := (x-in.X)*(x-in.X) + (y-in.Y)*(y-in.Y); d < best {
				best, hoverFleet = d, fl.ID
				hoverRock = model.NoRock
			}
		}
	}
	selFleetMine := (*model.Fleet)(nil)
	if g.sel.kind == selFleet {
		if fl := w.Fleet(g.sel.fleet); fl != nil && fl.Owner == g.me {
			selFleetMine = fl
		}
	}

	if hover && in.Pressed[input.Left] && g.menu == nil {
		switch {
		case g.orders.active:
			if hoverRock != model.NoRock {
				g.sendOrder(hoverRock)
			}
		case hoverFleet != 0:
			g.sel = selection{kind: selFleet, fleet: hoverFleet}
		case hoverRock != model.NoRock:
			if in.DoubleClick {
				g.gotoRock(hoverRock)
			} else {
				g.sel = selection{kind: selRock, rock: hoverRock}
			}
		default:
			g.sel = selection{}
		}
	}
	if hover && in.Released[input.Right] && cam.dragged < 6 {
		switch {
		case g.menu != nil:
			g.menu = nil
		case g.orders.active:
			g.orders.active = false
		case selFleetMine != nil && hoverRock != model.NoRock:
			g.menu = &ctxMenu{rock: hoverRock, x: in.X, y: in.Y}
		default:
			g.sel = selection{}
		}
	}

	// Travel estimate while picking a destination.
	etaText := ""
	if hoverRock != model.NoRock {
		var fl *model.Fleet
		if g.orders.active {
			fl = w.Fleet(g.orders.fleet)
		} else {
			fl = selFleetMine
		}
		if fl != nil && fl.Owner == g.me {
			if eta, ok := sim.TravelTime(w, w.FleetPos(fl), hoverRock, fl.Speed(w.Mods(g.me))); ok && eta > 0 {
				etaText = "flight " + fmtTime(eta)
				if fuel := sim.LaunchFuel(w, g.me, fl.Ships, fl.At, hoverRock); fuel > 0 {
					etaText += fmt.Sprintf(", %.0f Fuel", math.Ceil(fuel))
				}
			}
		}
	}

	// ---- drawing
	me, sel, clock, curRock := g.me, g.sel, g.clock, g.rock
	sensors := sim.Sensors(w, me)
	picking := g.orders.active
	a := g.a
	ui.Paint(func(cv *gfx.Canvas) {
		cv.PushClip(area)
		defer cv.PopClip()
		sx, sy := scr(model.Vec2{})
		a.backdrop.Draw(cv, gfx.BackdropView{X: float32(cam.cx) * zoom * 0.2, Y: float32(cam.cy) * zoom * 0.2,
			EmberX: sx, EmberY: sy, EmberRadius: 700 * zoom / fit, Time: clock})
		// Sensor coverage.
		for _, s := range sensors {
			x, y := scr(s.Pos)
			cv.FillCircle(x, y, float32(s.Range)*zoom, gfx.Fade(th.Info, 0.035))
		}
		for _, s := range sensors {
			x, y := scr(s.Pos)
			cv.StrokeCircle(x, y, float32(s.Range)*zoom, 1, gfx.Fade(th.Info, 0.12))
		}
		// Orbits.
		for _, r := range w.Rocks {
			col := gfx.Fade(th.Text, 0.07)
			if r.ID == sel.rock && sel.kind == selRock {
				col = gfx.Fade(th.Rule, 0.25)
			}
			cv.StrokeCircle(sx, sy, float32(r.OrbitR)*zoom, 1, col)
		}
		art.Star(cv, sx, sy, max(10, 26*zoom/fit), clock)

		// Flight paths and fleets in transit.
		for _, fl := range visible {
			if !fl.InTransit() {
				continue
			}
			x, y := scr(fl.Pos)
			col := th.FactionColor(int(fl.Owner))
			tx, ty := scr(w.RockPos(fl.Dest))
			if fl.Owner == me || fl.Order == model.OrderAttack {
				cv.StrokeLine(x, y, tx, ty, gfx.Stroke{Width: 1, Dash: 5, Gap: 5}, gfx.Fade(col, 0.5))
			}
			heading := float32(math.Atan2(float64(ty-y), float64(tx-x)))
			if sel.kind == selFleet && sel.fleet == fl.ID {
				cv.StrokeCircle(x, y, 13, 1.5, th.Rule)
			} else if hoverFleet == fl.ID {
				cv.StrokeCircle(x, y, 13, 1, gfx.Fade(th.Text, 0.5))
			}
			kind := fl.Ships[0].Kind
			for i := range fl.Ships {
				if fl.Ships[i].Def().Berths > model.ShipDefOf(kind).Berths {
					kind = fl.Ships[i].Kind
				}
			}
			art.Ship(cv, kind, x, y, 16, heading, col)
			if len(fl.Ships) > 1 {
				cv.Text(fmt.Sprint(len(fl.Ships)), x+10, y-16, th.NumSmall().WithColor(col))
			}
		}
		// Slugs.
		for _, s := range w.Slugs {
			x, y := scr(s.Pos)
			cv.FillCircle(x, y, 2, th.FactionColor(int(s.Owner)))
			cv.Glow(x, y, 8, gfx.Fade(th.FactionColor(int(s.Owner)), 0.6))
		}
		// Rocks.
		for _, r := range w.Rocks {
			x, y := scr(w.RockPos(r.ID))
			rad := (5 + float32(r.Radius)*0.55) * float32(math.Sqrt(float64(max(1, zoom/fit))))
			if r.Owner != model.NoFaction {
				cv.StrokeCircle(x, y, rad+4, 2, th.FactionColor(int(r.Owner)))
			}
			art.RockBadge(cv, r, x, y, rad)
			if r.ID == curRock {
				cv.StrokeCircle(x, y, rad+8, 1, gfx.Fade(th.Text, 0.5))
			}
			if (sel.kind == selRock && sel.rock == r.ID) || hoverRock == r.ID {
				col := th.Rule
				if hoverRock == r.ID && !(sel.kind == selRock && sel.rock == r.ID) {
					col = gfx.Fade(th.Text, 0.7)
				}
				cv.StrokeCircle(x, y, rad+11, 1.5, col)
			}
			ncol := th.TextDim
			if r.Owner != model.NoFaction {
				ncol = th.FactionColor(int(r.Owner))
			}
			cv.Text(r.Name, x, y+rad+12, th.Small().WithColor(ncol).Aligned(gfx.AlignCenter))
			if r.Salvage != nil {
				art.Icon(cv, art.IconDeposit, gfx.R(x+rad+4, y-rad-12, 12, 12), th.Info)
			}
			// Ships in orbit: one pip per fleet the player can see.
			n := 0
			for _, fl := range visible {
				if fl.At != r.ID {
					continue
				}
				px := x - rad - 10 - float32(n)*9
				col := th.FactionColor(int(fl.Owner))
				cv.FillPoly([]gfx.Pt{{X: px, Y: y - 5}, {X: px + 6, Y: y}, {X: px, Y: y + 5}}, col)
				if sel.kind == selFleet && sel.fleet == fl.ID {
					cv.StrokeCircle(px+2, y, 8, 1.5, th.Rule)
				}
				n++
			}
		}
		// Effects in space.
		for _, e := range g.fx {
			if e.kind == model.FXShipDestroyed || e.kind == model.FXBuildDestroyed || e.kind == model.FXSlugImpact {
				x, y := scr(w.RockPos(e.rock))
				art.Explosion(cv, x, y, 14, e.age/e.life)
			}
		}
		if hoverRock != model.NoRock && etaText != "" {
			x, y := scr(w.RockPos(hoverRock))
			cv.Text(etaText, x, y-34, th.Small().WithColor(th.Text).Aligned(gfx.AlignCenter))
		}
		if picking {
			box := gfx.R(area.MidX()-230, area.Y+12, 460, 30)
			cv.Panel(box, 5, gfx.CornersDiag, gfx.WithAlpha(th.Well, 0.92), th.Rule)
			cv.TextIn(box, "Click the destination rock.  Right-click or Esc cancels.", th.Body().WithColor(th.Rule).Aligned(gfx.AlignCenter))
		}
	})

	// The right-click order menu.
	if g.menu != nil && selFleetMine != nil {
		g.orderMenu(area, selFleetMine)
	} else {
		g.menu = nil
	}
}

// sendOrder issues the pending picked order with the given target.
func (g *Game) sendOrder(target model.RockID) {
	o := g.orders
	var cmd model.Command
	switch o.order {
	case model.OrderAttack:
		cmd = model.AttackCmd(g.me, o.fleet, o.ships, target, o.stance, o.retreat)
	case model.OrderColonize:
		cmd = model.ColonizeCmd(g.me, o.fleet, o.ships, target, nil)
	default:
		cmd = model.FleetOrderCmd(g.me, o.fleet, o.ships, o.order, target)
	}
	if g.issue(cmd) {
		g.orders.active = false
		g.sel = selection{kind: selRock, rock: target}
	}
}

// orderMenu shows the orders that make sense for fleet fl at the clicked rock.
func (g *Game) orderMenu(area gfx.Rect, fl *model.Fleet) {
	ui, th := g.a.ui, g.a.th
	m := g.menu
	rock := g.w.Rock(m.rock)
	if rock == nil {
		g.menu = nil
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
	type item struct {
		label string
		cmd   model.Command
	}
	var items []item
	hostile := rock.Owner != model.NoFaction && rock.Owner != g.me
	if hostile && has(model.ShipDef.Armed) {
		for _, s := range []model.Stance{model.StanceSuppress, model.StanceRaid, model.StanceRaze} {
			items = append(items, item{"Attack: " + s.String(), model.AttackCmd(g.me, fl.ID, ids, rock.ID, s, g.win.retreat)})
		}
	}
	if rock.Owner == model.NoFaction && has(func(d model.ShipDef) bool { return d.CanColonize }) {
		items = append(items, item{"Found a colony here", model.ColonizeCmd(g.me, fl.ID, ids, rock.ID, nil)})
	}
	if !rock.Surveyed(g.me) && has(func(d model.ShipDef) bool { return d.CanSurvey }) {
		items = append(items, item{"Survey", model.FleetOrderCmd(g.me, fl.ID, ids, model.OrderSurvey, rock.ID)})
	}
	if !hostile && has(model.ShipDef.Armed) && rock.Owner != g.me {
		items = append(items, item{"Attack ships in orbit", model.AttackCmd(g.me, fl.ID, ids, rock.ID, model.StanceSuppress, g.win.retreat)})
	}
	items = append(items, item{"Move here", model.FleetOrderCmd(g.me, fl.ID, ids, model.OrderMove, rock.ID)})

	const rowH, w = 28, 210
	h := float32(len(items))*rowH + 34
	box := gfx.R(min(m.x, area.Right()-w-6), min(m.y, area.Bottom()-h-6), w, h)
	old := ui.SetLayer(kit.LayerPopup)
	ui.Block(box)
	ui.Paint(func(cv *gfx.Canvas) {
		cv.Shadow(box.Translate(0, 4), 12, th.Shadow)
		cv.Panel(box, 5, gfx.CornersDiag, th.Panel, th.Rule)
		cv.TextIn(gfx.R(box.X+10, box.Y+4, box.W-20, 24), kit.Upper(rock.Name), th.Label().WithColor(th.Rule))
	})
	in := box.Pad(4, 30, 4, 4)
	for _, it := range items {
		if ui.Button("menu/"+it.label, in.CutTop(rowH).Pad(0, 0, 0, 2), it.label, kit.Btn{Quiet: true, Left: true, Align: gfx.AlignLeft}) {
			g.issue(it.cmd)
			g.menu = nil
		}
	}
	ui.SetLayer(old)
	if g.a.in.Pressed[input.Left] && !box.Contains(g.a.in.X, g.a.in.Y) {
		g.menu = nil
	}
}
