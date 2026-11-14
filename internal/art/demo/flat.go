package main

import (
	"fmt"
	"math"

	"drifthold/internal/art"
	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// The sheets in this file show the art that is seen from straight above
// whatever the surface view does: ships, goods, interface glyphs, rock
// badges, the star and explosions.

func (d *demo) ships(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	kinds := model.AllShipKinds()
	rowH := r.H / float32(len(kinds))
	for ki, kind := range kinds {
		row := gfx.R(r.X, r.Y+float32(ki)*rowH, r.W, rowH-4)
		cv.Panel(row, 5, gfx.CornersDiag, gfx.WithAlpha(th.Panel, 0.55), th.Border)
		name := row.CutLeft(92)
		cv.TextIn(name.Inset(8), kind.String(), th.Body())
		icon := row.CutLeft(rowH - 4).Inset(6)
		cv.Panel(icon, 4, gfx.CornersAll, th.Well, th.Border)
		art.ShipIcon(cv, kind, icon.Inset(4), th.FactionColor(1))
		row.CutLeft(10)
		bw := row.W / 4
		for a := 0; a < 4; a++ {
			blk := gfx.R(row.X+float32(a)*bw, row.Y, bw, row.H)
			accent := th.FactionColor(1 + a)
			x := blk.X + 34
			cy := blk.MidY()
			for i := 0; i < 2; i++ {
				h := float32(a)*0.7 + float32(i)*2.3 - 1.2
				art.Ship(cv, kind, x, cy, 48, h, accent)
				x += 62
			}
			for i := 0; i < 4; i++ {
				h := float32(a)*0.4 + float32(i)*math.Pi/2*1.1
				art.Ship(cv, kind, x, cy-18+float32(i%2)*36, 24, h, accent)
				x += 22
			}
			x += 12
			for i := 0; i < 6; i++ {
				h := float32(a)*0.5 + float32(i)*math.Pi/3
				art.Ship(cv, kind, x+float32(i/2)*22, cy-24+float32(i%2)*22, 14, h, accent)
			}
			for i := 0; i < 6; i++ {
				h := float32(a)*0.5 + float32(i)*math.Pi/3 + 0.4
				art.Ship(cv, kind, x+float32(i)*13, cy+30, 10, h, accent)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Goods and glyphs
// ---------------------------------------------------------------------------

func (d *demo) goodsIcons(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	top := r.CutTop(170)
	cv.Panel(top, 5, gfx.CornersDiag, th.Panel, th.RuleDim)
	top = top.Inset(10)
	cv.Text("Goods at 12, 16, 20, 24 and 56 pt", top.X, top.Y, th.Label())
	goods := model.AllGoods()
	gw := top.W / float32(len(goods))
	for i, g := range goods {
		x := top.X + float32(i)*gw
		y := top.Y + 24
		art.Good(cv, g, gfx.R(x, y, 56, 56))
		xx := x + 68
		for _, s := range []float32{24, 20, 16, 12} {
			art.Good(cv, g, gfx.R(xx, y+56-s, s, s))
			xx += s + 6
		}
		// As it appears in a resource bar.
		bar := gfx.R(x, y+72, 120, 26)
		cv.Panel(bar, 4, gfx.CornersDiag, th.PanelHi, th.Border)
		art.Good(cv, g, gfx.R(bar.X+6, bar.Y+5, 16, 16))
		cv.TextIn(bar.Pad(28, 0, 6, 0), fmt.Sprintf("%d", 120+i*347), th.Num())
		cv.TextIn(bar.Pad(28, 0, 6, 0), g.String(), th.Small().WithColor(th.TextDim).Aligned(gfx.AlignRight))
	}

	r.CutTop(10)
	cv.Panel(r, 5, gfx.CornersDiag, th.Panel, th.RuleDim)
	r = r.Inset(10)
	cv.Text("Interface glyphs at 12, 16, 20, 24 and 40 pt, in cream, amber and dim", r.X, r.Y, th.Label())
	r.CutTop(22)
	const cols = 8
	rows := (int(art.NumIcons) + cols - 1) / cols
	for i := 0; i < int(art.NumIcons); i++ {
		cell := r.Cell(i%cols, i/cols, cols, rows, 4)
		id := art.IconID(i)
		y := cell.Y + 6
		art.Icon(cv, id, gfx.R(cell.X+4, y, 40, 40), th.Text)
		x := cell.X + 54
		for k, s := range []float32{24, 20, 16, 12} {
			col := th.Text
			if k == 1 {
				col = th.Rule
			}
			if k == 3 {
				col = th.TextDim
			}
			art.Icon(cv, id, gfx.R(x, y+40-s, s, s), col)
			x += s + 7
		}
		// In a button, as the interface will use it.
		btn := gfx.R(x+2, y+14, 26, 26)
		cv.Panel(btn, 5, gfx.CornersDiag, th.PanelHi, th.RuleDim)
		art.Icon(cv, id, btn.Inset(5), th.Text)
		cv.Text(iconNames[i], cell.X+4, y+46, th.Small().WithColor(th.TextDim))
	}
}

var iconNames = [art.NumIcons]string{
	"Power", "Colonists", "Morale", "Scrip", "Research", "Fleet", "Belt", "Surface",
	"Pause", "Play", "Fast", "Faster", "Menu", "Close", "Help", "Directives",
	"Exchange", "Holds", "Warning", "Housing", "Air", "Storage", "Berth", "Shield",
	"Target", "Worker", "Cycle", "Deposit", "Demolish", "Priority", "OnOff", "Log",
	"Save", "Check", "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown", "Lock",
}

// ---------------------------------------------------------------------------
// Badges, the star, an explosion
// ---------------------------------------------------------------------------

func (d *demo) misc(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	top := r.CutTop(250)
	cv.Text("Rock badges at radius 6, 8, 10, 14 and 24 pt", top.X, top.Y, th.Label())
	rocks := d.world.Rocks
	cw := top.W / float32(len(rocks))
	for i, rock := range rocks {
		x := top.X + (float32(i)+0.5)*cw
		y := top.Y + 34
		for _, rad := range []float32{6, 8, 10, 14, 24} {
			y += rad
			art.RockBadge(cv, rock, x, y, rad)
			y += rad + 12
		}
		cv.Text(rock.Name, x, y, th.Small().Sized(9).WithColor(th.TextDim).Aligned(gfx.AlignCenter))
	}
	r.CutTop(8)
	starArea := r.CutLeft(520)
	cv.Text("Emberfall at radius 60, 26 and 10 pt, at three moments", starArea.X, starArea.Y, th.Label())
	art.Star(cv, starArea.X+170, starArea.Y+250, 60, t)
	art.Star(cv, starArea.X+400, starArea.Y+130, 26, t+2.1)
	art.Star(cv, starArea.X+420, starArea.Y+330, 10, t+4.4)
	cv.Text("Explosion, phase 0 … 0.95, radius 44 and 18 pt", r.X, r.Y, th.Label())
	const n = 12
	for i := 0; i < n; i++ {
		ph := float32(i) / n * 0.96
		x := r.X + 50 + float32(i)*(r.W-90)/(n-1)
		art.Explosion(cv, x, r.Y+130, 44, ph)
		art.Explosion(cv, x, r.Y+280, 18, ph)
		d.caption(cv, fmt.Sprintf("%.2f", ph), x-10, r.Y+330)
	}
	// A blast over a lit surface, where it will be seen.
	cv.Text("Over ground", r.X, r.Y+370, th.Label())
	for i := 0; i < 6; i++ {
		c := gfx.P(r.X+60+float32(i)*150, r.Y+450)
		for q := -2; q <= 2; q++ {
			for rr := -1; rr <= 1; rr++ {
				p := art.HexPos(c, model.Hex{Q: q, R: rr}, 22)
				d.ground(cv, p.X, p.Y, 22, 1)
			}
		}
		art.Building(cv, model.Habitat, c.X, c.Y, 22, art.BuildingLook{Accent: th.FactionColor(2), Progress: 1, Damage: float32(i) / 5, Time: t})
		art.Explosion(cv, c.X, c.Y, 30, float32(i)/6)
	}
}
