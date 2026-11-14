package app

import (
	"fmt"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/art"
	"drifthold/internal/gfx"
	"drifthold/internal/input"
	"drifthold/internal/model"
	"drifthold/internal/sim"
	"drifthold/internal/ui/kit"
)

// surfaceCam is the camera of the surface view.
type surfaceCam struct {
	off     gfx.Pt  // pan, in points
	hex     float32 // tile corner radius in points (the zoom)
	drag    bool
	dragged float32
	lastX   float32
	lastY   float32
}

// orbitShip is where one ship of a fleet in orbit is drawn this frame.
type orbitShip struct {
	fleet   model.FleetID
	owner   model.FactionID
	kind    model.ShipKind
	x, y    float32
	heading float32
}

const (
	minHex = 9
	maxHex = 44
)

// panWorld applies the keyboard and drag panning shared by both world views
// and returns the pan delta in points and whether the pointer is free for the
// view (not over the interface).
func (g *Game) panInput(area gfx.Rect, drag *bool, dragged, lastX, lastY *float32) (dx, dy float32, hover bool) {
	in, ui := g.a.in, g.a.ui
	hover = ui.Hovered(area) && !ui.WantMouse()
	if !ui.WantKeys() && !in.KeysTaken && g.window == windowNone && g.a.dialog == dialogNone && !in.Ctrl {
		speed := 520 * in.Dt
		if in.Shift {
			speed *= 2.2
		}
		if in.KeyDown(ebiten.KeyA) || in.KeyDown(ebiten.KeyArrowLeft) {
			dx += speed
		}
		if in.KeyDown(ebiten.KeyD) || in.KeyDown(ebiten.KeyArrowRight) {
			dx -= speed
		}
		if in.KeyDown(ebiten.KeyW) || in.KeyDown(ebiten.KeyArrowUp) {
			dy += speed
		}
		if in.KeyDown(ebiten.KeyS) || in.KeyDown(ebiten.KeyArrowDown) {
			dy -= speed
		}
	}
	held := in.Down[input.Right] || in.Down[input.Middle]
	if hover && (in.Pressed[input.Right] || in.Pressed[input.Middle]) {
		*drag, *dragged = true, 0
		*lastX, *lastY = in.X, in.Y
	}
	if *drag {
		if held {
			mx, my := in.X-*lastX, in.Y-*lastY
			dx += mx
			dy += my
			*dragged += float32(math.Abs(float64(mx)) + math.Abs(float64(my)))
			*lastX, *lastY = in.X, in.Y
		} else {
			*drag = false
		}
	}
	return dx, dy, hover
}

func (g *Game) surfaceFrame(area gfx.Rect) {
	in, ui, th := g.a.in, g.a.ui, g.a.th
	rock := g.cur()
	if rock == nil {
		return
	}
	cam := &g.scam
	dx, dy, hover := g.panInput(area, &cam.drag, &cam.dragged, &cam.lastX, &cam.lastY)
	cam.off.X += dx
	cam.off.Y += dy
	cx, cy := area.MidX(), area.MidY()
	if hover && in.Wheel != 0 {
		old := cam.hex
		cam.hex = max(minHex, min(maxHex, cam.hex*float32(math.Pow(1.12, float64(in.Wheel)))))
		k := cam.hex / old
		// Keep the point under the cursor where it is.
		cam.off.X = in.X - cx - (in.X-cx-cam.off.X)*k
		cam.off.Y = in.Y - cy - (in.Y-cy-cam.off.Y)*k
	}
	extent := cam.hex * 1.8 * float32(rock.Radius+2)
	cam.off.X = max(-extent, min(extent, cam.off.X))
	cam.off.Y = max(-extent*art.Tilt, min(extent*art.Tilt, cam.off.Y))
	origin := gfx.P(cx+cam.off.X, cy+cam.off.Y)
	hex := cam.hex
	at := func(h model.Hex) (float32, float32) {
		p := art.HexPos(origin, h, hex)
		return p.X, p.Y
	}

	// Ships in orbit, laid out on a ring around the rock.
	ships := g.orbitLayout(rock, origin, hex)

	// What is under the cursor?
	hoverHex := art.PickHex(origin, in.X, in.Y, hex)
	tile := rock.TileAt(hoverHex)
	onRock := hover && tile != nil && tile.Terrain.Solid()
	var hoverFleet model.FleetID
	if hover {
		best := float32(14 * 14)
		for _, s := range ships {
			if d := (s.x-in.X)*(s.x-in.X) + (s.y-in.Y)*(s.y-in.Y); d < best {
				best, hoverFleet = d, s.fleet
			}
		}
	}
	hoverB := (*model.Building)(nil)
	if onRock && hoverFleet == 0 {
		hoverB = rock.BuildingAt(hoverHex)
	}

	// Placement preview.
	placing := g.place != model.BuildingNone && rock.Owner == g.me
	var placeErr, costErr error
	if placing && onRock {
		placeErr = sim.CanPlaceIgnoringCost(g.w, g.me, rock.ID, hoverHex, g.place)
		if placeErr == nil {
			costErr = sim.CanPlace(g.w, g.me, rock.ID, hoverHex, g.place)
		}
	}

	if hover && in.Pressed[input.Left] {
		switch {
		case placing && onRock:
			if g.issue(model.BuildCmd(g.me, rock.ID, hoverHex, g.place)) && model.BuildingDefOf(g.place).MaxPerRock == 1 {
				g.place = model.BuildingNone
			}
		case hoverFleet != 0:
			g.sel = selection{kind: selFleet, fleet: hoverFleet}
		case hoverB != nil:
			g.sel = selection{kind: selBuilding, rock: rock.ID, building: hoverB.ID}
		case onRock:
			g.sel = selection{kind: selTile, rock: rock.ID, hex: hoverHex}
		default:
			g.sel = selection{}
		}
	}
	if hover && in.Released[input.Right] && cam.dragged < 6 {
		if placing {
			g.place = model.BuildingNone
		} else {
			g.sel = selection{}
		}
	}

	// Hover hints.
	switch {
	case placing:
	case hoverB != nil:
		def := hoverB.Def()
		tip := &kit.Tip{Title: def.Name}
		if of := g.w.Faction(hoverB.Owner); of != nil && hoverB.Owner != g.me {
			tip.Rows = append(tip.Rows, kit.TipRow{K: "Owner", V: of.Name, Col: th.FactionColor(int(hoverB.Owner))})
		}
		switch {
		case hoverB.Derelict():
			tip.Body = "Derelict."
		case hoverB.Constructing():
			tip.Body = fmt.Sprintf("Under construction: %.0f %%.", hoverB.Progress*100)
		case hoverB.Idle != model.IdleNone && hoverB.Owner == g.me:
			tip.Body = hoverB.Idle.String() + "."
		}
		ui.TipFor(fmt.Sprint("hover/b", hoverB.ID), gfx.R(in.X-2, in.Y-2, 4, 4), tip)
	case onRock:
		if gd, amt, ok := rock.VisibleDeposit(hoverHex, g.me); ok {
			ui.TipFor(fmt.Sprint("hover/d", hoverHex), gfx.R(in.X-2, in.Y-2, 4, 4),
				&kit.Tip{Title: gd.String() + " deposit", Rows: []kit.TipRow{{K: "Left in the ground", V: model.FormatQty(amt)}}})
		}
	}

	// ---- drawing
	me, sel, clock, deposit, place := g.me, g.sel, g.clock, g.deposit || placing && model.BuildingDefOf(g.place).MineRate > 0, g.place
	star := g.w.RockPos(rock.ID)
	fxs := append([]effect(nil), g.fx...)
	a := g.a
	ui.Paint(func(cv *gfx.Canvas) {
		cv.PushClip(area)
		defer cv.PopClip()
		// Emberfall lies toward the belt's centre: light leans in from there.
		dir := model.Vec2{X: -star.X, Y: -star.Y}.Norm()
		a.backdrop.Draw(cv, gfx.BackdropView{X: -cam.off.X * 0.3, Y: -cam.off.Y * 0.3,
			EmberX: cx + float32(dir.X)*1500, EmberY: cy + float32(dir.Y)*1500, EmberRadius: 1500, Time: clock})
		size := max(11, min(22, hex*0.8))
		drawShips := func(near bool) {
			for _, s := range ships {
				if (s.y >= origin.Y) != near {
					continue
				}
				col := th.FactionColor(int(s.owner))
				if sel.kind == selFleet && sel.fleet == s.fleet {
					cv.StrokeCircle(s.x, s.y, size*0.75, 1.5, th.Rule)
				} else if hoverFleet == s.fleet {
					cv.StrokeCircle(s.x, s.y, size*0.75, 1, gfx.Fade(th.Text, 0.5))
				}
				art.Ship(cv, s.kind, s.x, s.y, size, s.heading, col)
			}
		}
		drawShips(false) // ships on the far side of the rock pass behind it
		g.surface.Draw(cv, rock, origin, hex, me, deposit)

		// Reach and coverage of the selected or pending structure.
		ring := func(h model.Hex, radius int, col gfx.Color) {
			for _, x := range h.Spiral(radius) {
				if t := rock.TileAt(x); t != nil && t.Terrain.Solid() {
					px, py := at(x)
					art.TileFill(cv, px, py, hex, col)
				}
			}
		}
		if sel.kind == selBuilding && sel.rock == rock.ID {
			if b := rock.BuildingByID(sel.building); b != nil {
				def := b.Def()
				switch {
				case def.MineRate > 0:
					ring(b.Hex, 1, gfx.Fade(th.Rule, 0.13))
				case def.ShieldRadius > 0:
					ring(b.Hex, def.ShieldRadius, gfx.Fade(th.Info, 0.10))
				case def.RepairRadius > 0:
					ring(b.Hex, def.RepairRadius, gfx.Fade(th.Good, 0.08))
				case def.SurveyRadius > 0:
					ring(b.Hex, def.SurveyRadius+g.w.Mods(b.Owner).SurveyBonus, gfx.Fade(th.Info, 0.07))
				}
			}
		}

		// Back to front, so that nearer structures overlap those behind.
		order := append([]*model.Building(nil), rock.Buildings...)
		sort.SliceStable(order, func(i, j int) bool {
			a, b := order[i].Hex, order[j].Hex
			if a.R != b.R {
				return a.R < b.R
			}
			return a.Q < b.Q
		})
		for _, b := range order {
			px, py := at(b.Hex)
			if !cv.Visible(gfx.R(px-hex, py-2.5*hex, 2*hex, 3.5*hex)) {
				continue
			}
			if sel.kind == selBuilding && sel.building == b.ID {
				art.TileFill(cv, px, py, hex, gfx.Fade(th.Rule, 0.25))
				art.TileStroke(cv, px, py, hex, 2, th.Rule)
			}
			top := art.BuildingTop(b.Kind, hex)
			look := art.BuildingLook{
				Accent: th.FactionColor(int(b.Owner)), Progress: float32(b.Progress), Derelict: b.Derelict(),
				Off:    b.Completed() && !b.Working() && b.Kind != model.Conduit && b.Kind != model.Keystone,
				Damage: float32(1 - b.HPFrac()), Time: clock,
			}
			if b.Constructing() {
				look.Damage = 0
			}
			art.Building(cv, b.Kind, px, py, hex, look)
			if b.Owner == me && b.Completed() && b.Idle != model.IdleNone && b.Idle != model.IdleNoWork && b.Idle != model.IdleNoTarget && hex >= 13 {
				col := th.Warn
				if b.Idle == model.IdleNoPower || b.Idle == model.IdleNoStaff {
					col = th.Alert
				}
				ic := art.IconWarning
				switch b.Idle {
				case model.IdleNoPower:
					ic = art.IconPower
				case model.IdleNoStaff:
					ic = art.IconWorker
				case model.IdleDisabled:
					ic, col = art.IconOnOff, th.TextDim
				case model.IdleStorageFull:
					ic = art.IconStorage
				}
				s := min(14, hex*0.62)
				br := gfx.R(px+hex*0.28, py-top-s*0.4, s, s)
				cv.FillCircle(br.MidX(), br.MidY(), s*0.72, gfx.WithAlpha(th.Well, 0.9))
				art.Icon(cv, ic, br, col)
			}
			if b.Completed() && b.HP < b.MaxHP*0.98 && hex >= 12 {
				w := hex * 1.1
				bar := gfx.R(px-w/2, py+hex*0.68*art.Tilt+1, w, 3)
				cv.FillRect(bar, th.Well)
				col := th.Good
				if f := b.HPFrac(); f < 0.35 {
					col = th.Alert
				} else if f < 0.7 {
					col = th.Warn
				}
				cv.FillRect(gfx.R(bar.X, bar.Y, bar.W*float32(b.HPFrac()), bar.H), col)
			}
		}
		if sel.kind == selTile && sel.rock == rock.ID {
			px, py := at(sel.hex)
			art.TileStroke(cv, px, py, hex, 1.5, th.Text)
		}

		// Placement ghost.
		if placing && onRock {
			px, py := at(hoverHex)
			col := th.Good
			msg := ""
			switch {
			case placeErr != nil:
				col, msg = th.Alert, placeErr.Error()
			case costErr != nil:
				col, msg = th.Warn, costErr.Error()
			}
			def := model.BuildingDefOf(place)
			if def.MineRate > 0 && placeErr == nil {
				ring(hoverHex, 1, gfx.Fade(th.Rule, 0.16))
			}
			if def.ShieldRadius > 0 && placeErr == nil {
				ring(hoverHex, def.ShieldRadius, gfx.Fade(th.Info, 0.10))
			}
			art.TileFill(cv, px, py, hex, gfx.Fade(col, 0.22))
			if placeErr == nil {
				art.Building(cv, place, px, py, hex, art.BuildingLook{Accent: th.FactionColor(int(me)), Progress: 1, Off: true, Time: clock})
			}
			art.TileStroke(cv, px, py, hex, 2, col)
			if msg != "" {
				st := th.Small().WithColor(col)
				w, _ := cv.MeasureText(msg, st)
				box := gfx.R(px-w/2-8, py+hex*art.Tilt+8, w+16, 22)
				cv.Panel(box, 4, gfx.CornersDiag, gfx.WithAlpha(th.Well, 0.92), col)
				cv.TextIn(box, msg, st.Aligned(gfx.AlignCenter))
			}
		} else if onRock && !placing {
			px, py := at(hoverHex)
			art.TileStroke(cv, px, py, hex, 1, gfx.Fade(th.Text, 0.35))
		}

		drawShips(true) // ships on the near side pass in front

		// Effects.
		for _, e := range fxs {
			if e.rock != rock.ID {
				continue
			}
			ph := e.age / e.life
			px, py := at(e.hex)
			orbit := func() (float32, float32) {
				r := hex * 1.732 * orbitRing(rock)
				return origin.X + r*float32(math.Cos(float64(e.angle))), origin.Y + r*float32(math.Sin(float64(e.angle)))*art.Tilt
			}
			switch e.kind {
			case model.FXBuildDestroyed, model.FXSlugImpact:
				art.Explosion(cv, px, py, hex*1.6, ph)
			case model.FXBomb, model.FXMeteorImpact:
				art.Explosion(cv, px, py, hex*0.9, ph)
			case model.FXShipDestroyed:
				ox, oy := orbit()
				art.Explosion(cv, ox, oy, hex*1.2, ph)
			case model.FXBuildDone:
				art.TileStroke(cv, px, py, hex*(1+ph*0.6), 2, gfx.Fade(th.Good, 1-ph))
			case model.FXShieldHit:
				art.TileStroke(cv, px, py, hex*1.1, 2, gfx.Fade(th.Info, 1-ph))
				cv.Glow(px, py, hex*1.4, gfx.Fade(th.Info, 0.5*(1-ph)))
			case model.FXShotFlak, model.FXShotLance:
				ox, oy := orbit()
				if e.kind == model.FXShotLance {
					cv.Line(px, py, ox, oy, 2, gfx.Fade(gfx.RGB(0xC9A0FF), 1-ph))
					cv.Glow(ox, oy, hex*0.8, gfx.Fade(gfx.RGB(0xC9A0FF), 1-ph))
				} else {
					tx, ty := px+(ox-px)*ph, py+(oy-py)*ph
					cv.Line(px+(ox-px)*max(0, ph-0.25), py+(oy-py)*max(0, ph-0.25), tx, ty, 1.5, gfx.Fade(th.Rule, 0.9))
				}
			case model.FXShotShip:
				ox, oy := orbit()
				cv.Glow(ox, oy, hex*0.7, gfx.Fade(e.col, 0.8*(1-ph)))
			}
		}
	})
}

// orbitLayout places every visible ship in orbit of rock on a ring around it.
func (g *Game) orbitLayout(rock *model.Rock, origin gfx.Pt, hex float32) []orbitShip {
	var out []orbitShip
	fleets := g.w.FleetsAt(rock.ID)
	if len(fleets) == 0 {
		return nil
	}
	radius := hex * 1.732 * orbitRing(rock)
	step := max(11, min(22, hex*0.8)) * 1.5 / radius
	base := float64(g.clock) * 0.02
	n := 0
	for fi, fl := range fleets {
		if !sim.Visible(g.w, g.me, fl) {
			continue
		}
		// Each fleet gets its own stretch of the ring.
		start := base + float64(fi)*2.1 + float64(fl.ID%7)*0.31
		for i := range fl.Ships {
			a := start + float64(i)*float64(step)
			out = append(out, orbitShip{
				fleet: fl.ID, owner: fl.Owner, kind: fl.Ships[i].Kind,
				x:       origin.X + radius*float32(math.Cos(a)),
				y:       origin.Y + radius*float32(math.Sin(a))*art.Tilt,
				heading: float32(math.Atan2(math.Cos(a)*art.Tilt, -math.Sin(a))),
			})
			n++
		}
	}
	return out
}
