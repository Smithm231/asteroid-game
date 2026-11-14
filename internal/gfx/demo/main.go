// Command demo renders the gfx contact sheet: the palette, every Canvas
// primitive, chamfered panels, text styles, alignment, wrapping and
// truncation, clipping, glows, paths, sprites and the backdrop, all on one
// screen. It is the visual regression test of package gfx.
//
// Run it headlessly with
//
//	xvfb-run -a -s "-screen 0 3840x2160x24" go run ./internal/gfx/demo -out <dir> -scale 1.5
//
// which writes sheet.png (the sheet at rest) and sheet_pan.png (later, with
// the camera moved, to show the parallax of the backdrop). With -perf N it
// additionally draws N extra primitives per frame and prints how long the
// draw callback takes.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
	"drifthold/internal/input"
	"drifthold/internal/uitest"
)

var perf = flag.Int("perf", 0, "draw this many extra primitives per frame and report the draw time")

func main() {
	o := uitest.Flags()
	if o.Script == nil {
		o.Script = []input.Step{
			input.Wait(2),
			input.Shot("sheet.png"),
			input.Wait(40),
			input.Shot("sheet_pan.png"),
			input.Quit(),
		}
	}
	o.Title = "Drifthold gfx contact sheet"

	d := &demo{
		th:      gfx.Default(),
		sky:     gfx.NewBackdrop(0xD21F7),
		sprites: gfx.NewSpriteCache(64),
	}
	if err := uitest.Run(o, d.update, d.draw); err != nil {
		log.Fatal(err)
	}
	if *perf > 0 && d.frames > 0 {
		fmt.Printf("perf: %d extra primitives, draw callback %.2f ms/frame over %d frames\n",
			*perf, float64(d.drawTime.Microseconds())/1000/float64(d.frames), d.frames)
	}
}

type demo struct {
	th      *gfx.Theme
	sky     *gfx.Backdrop
	sprites *gfx.SpriteCache
	in      input.State

	drawTime time.Duration
	frames   int
}

func (d *demo) update(in input.State) { d.in = in }

func (d *demo) draw(dst *ebiten.Image) {
	start := time.Now()
	in := d.in
	if in.Scale == 0 {
		return
	}
	cv := gfx.NewCanvas(dst, in.Scale)
	th := d.th
	t := float32(in.Frame) / 60

	// The backdrop fills the screen; the camera starts moving after the
	// first screenshot so that the second one shows the layers shifted.
	cam := float32(max(0, in.Frame-4)) * 60
	d.sky.Draw(cv, gfx.BackdropView{
		X: cam, Y: cam * 0.4,
		EmberX: in.W - 150, EmberY: in.H + 60, EmberRadius: 760,
		Time: t,
	})

	screen := gfx.R(0, 0, in.W, in.H).Inset(12)
	header := screen.CutTop(48)
	d.header(cv, header)
	screen.CutTop(8)

	rowA := screen.CutTop(324)
	screen.CutTop(8)
	rowB := screen.CutTop(292)
	screen.CutTop(8)
	rowC := screen

	palette := rowA.CutLeft(500)
	rowA.CutLeft(8)
	glow := rowA.CutRight(372)
	rowA.CutRight(8)
	d.palette(cv, section(cv, th, palette, "Palette"))
	d.primitives(cv, section(cv, th, rowA, "Primitives"), t)
	d.glows(cv, section(cv, th, glow, "Glow, images, sprites"), t)

	panels := rowB.CutLeft(400)
	rowB.CutLeft(8)
	styles := rowB.CutLeft(520)
	rowB.CutLeft(8)
	d.panels(cv, section(cv, th, panels, "Chamfered panels"))
	d.styles(cv, section(cv, th, styles, "Text styles"))
	d.layout(cv, section(cv, th, rowB, "Text layout"))

	clip := rowC.CutLeft(300)
	rowC.CutLeft(8)
	paths := rowC.CutLeft(520)
	rowC.CutLeft(8)
	d.clips(cv, section(cv, th, clip, "Clip stack"))
	d.paths(cv, section(cv, th, paths, "Paths"))
	d.backdrop(cv, rowC)

	if *perf > 0 {
		d.stress(cv, *perf)
	}
	d.drawTime += time.Since(start)
	d.frames++
}

// section draws a titled panel and returns the area left for its content.
func section(cv *gfx.Canvas, th *gfx.Theme, r gfx.Rect, title string) gfx.Rect {
	cv.Shadow(r.Translate(0, 3), 10, th.Shadow)
	cv.Panel(r, th.Chamfer, gfx.CornersAll, th.Panel, th.RuleDim)
	body := r.Inset(th.Pad)
	bar := body.CutTop(20)
	cv.TextIn(bar, title, th.Head().Sized(13))
	cv.FillRect(gfx.R(bar.X, bar.Bottom()+1, bar.W, th.Hairline), th.Rule)
	body.CutTop(8)
	return body
}

// caption writes a small dim label with its top-left corner at (x, y).
func (d *demo) caption(cv *gfx.Canvas, s string, x, y float32) {
	cv.Text(s, x, y, d.th.Small().WithColor(d.th.TextDim))
}

func (d *demo) header(cv *gfx.Canvas, r gfx.Rect) {
	th := d.th
	cv.PanelV(r, th.Chamfer, gfx.CornersDiag, th.PanelHi, th.Panel, th.Rule, 1)
	body := r.InsetXY(14, 0)
	cv.TextIn(body, "Drifthold", th.Title().WithColor(th.Rule))
	w, _ := cv.MeasureText("Drifthold", th.Title())
	body.CutLeft(w + 14)
	cv.TextIn(body, "gfx contact sheet", th.Head().WithColor(th.TextDim))
	info := fmt.Sprintf("%.0f × %.0f pt   scale %.2f   %d × %d px",
		d.in.W, d.in.H, d.in.Scale, cv.Dst.Bounds().Dx(), cv.Dst.Bounds().Dy())
	cv.TextIn(body, info, th.Num().Aligned(gfx.AlignRight).WithColor(th.TextDim))
}

func (d *demo) palette(cv *gfx.Canvas, r gfx.Rect) {
	th := d.th
	type sw struct {
		name string
		c    gfx.Color
	}
	rows := []struct {
		label string
		sws   []sw
	}{
		{"Surface", []sw{{"Space", th.Space}, {"Well", th.Well}, {"Panel", th.Panel}, {"PanelHi", th.PanelHi}, {"Border", th.Border}, {"RuleDim", th.RuleDim}, {"Rule", th.Rule}}},
		{"Text", []sw{{"Text", th.Text}, {"TextDim", th.TextDim}, {"TextFaint", th.TextFaint}, {"Ink", th.Ink}}},
		{"Status", []sw{{"Good", th.Good}, {"Warn", th.Warn}, {"Alert", th.Alert}, {"Info", th.Info}}},
		{"Faction", []sw{{"0 neutral", th.FactionColor(0)}, {"1 amber", th.FactionColor(1)}, {"2 teal", th.FactionColor(2)}, {"3 magenta", th.FactionColor(3)}, {"4 lime", th.FactionColor(4)}}},
		{"Goods", []sw{{"Ore", th.GoodColor(0)}, {"Ice", th.GoodColor(1)}, {"Silica", th.GoodColor(2)}, {"Lumen", th.GoodColor(3)}, {"Alloy", th.GoodColor(4)}, {"Fuel", th.GoodColor(5)}, {"Parts", th.GoodColor(6)}, {"Food", th.GoodColor(7)}}},
		{"Rock", []sw{{"Ferric", th.RockColor(0)}, {"Stony", th.RockColor(1)}, {"Frost", th.RockColor(2)}, {"Glint", th.RockColor(3)}}},
	}
	for i, row := range rows {
		line := r.Row(i, len(rows), 4)
		label := line.CutLeft(54)
		cv.TextIn(label.Pad(0, 0, 0, 14), row.label, th.Label())
		for j, s := range row.sws {
			cell := gfx.R(line.X+float32(j)*53.5, line.Y, 49, line.H)
			chip := cell.CutTop(cell.H - 14)
			cv.Panel(chip, 4, gfx.CornersDiag, s.c, th.Border)
			cv.TextIn(cell, s.name, th.Small().Sized(10).WithColor(th.TextDim).Aligned(gfx.AlignCenter))
		}
	}
}

func (d *demo) primitives(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	amber, cream, teal := th.Rule, th.Text, th.FactionColor(2)
	rows := 5
	rowH := r.H / float32(rows)

	// Row 1: rectangles.
	y := r.Y
	d.caption(cv, "FillRect / V / H", r.X, y)
	cv.FillRect(gfx.R(r.X, y+16, 44, 26), amber)
	cv.FillRectV(gfx.R(r.X+50, y+16, 44, 26), th.PanelHi, th.Info)
	cv.FillRectH(gfx.R(r.X+100, y+16, 44, 26), th.Alert, th.Warn)
	d.caption(cv, "StrokeRect 1 / 2 / 3", r.X+160, y)
	for i := 0; i < 3; i++ {
		cv.StrokeRect(gfx.R(r.X+160+float32(i)*50, y+16, 44, 26), float32(i+1), cream)
	}
	d.caption(cv, "1 pt rules at x+0, +.25, +.5, +.75", r.X+320, y)
	for i := 0; i < 16; i++ {
		x := r.X + 320 + float32(i)*6.25
		cv.Line(x, y+16, x, y+42, 1, amber)
	}
	for i := 0; i < 5; i++ {
		yy := y + 16 + float32(i)*6.25
		cv.Line(r.X+430, yy, r.X+520, yy, 1, cream)
	}
	cv.Shadow(gfx.R(r.X+540, y+18, 56, 20), 8, gfx.RGBA(0x000000FF))
	cv.FillRect(gfx.R(r.X+540, y+14, 56, 20), th.PanelHi)
	d.caption(cv, "Shadow", r.X+548, y+17)

	// Row 2: lines.
	y += rowH
	d.caption(cv, "Line 0.5 / 1 / 2 / 4", r.X, y)
	for i, w := range []float32{0.5, 1, 2, 4} {
		x := r.X + float32(i)*30
		cv.Line(x, y+44, x+34, y+16, w, cream)
	}
	d.caption(cv, "caps: butt, round, square", r.X+160, y)
	for i, c := range []gfx.Cap{gfx.CapButt, gfx.CapRound, gfx.CapSquare} {
		yy := y + 20 + float32(i)*10
		cv.StrokeLine(r.X+170, yy, r.X+250, yy+3, gfx.Stroke{Width: 7, Cap: c}, amber)
		cv.Line(r.X+170, yy, r.X+250, yy+3, 0.75, th.Ink)
	}
	d.caption(cv, "dashes (marching)", r.X+320, y)
	cv.StrokeLine(r.X+320, y+20, r.X+470, y+20, gfx.Stroke{Width: 1.5, Dash: 8, Gap: 5, Phase: -t * 20}, teal)
	cv.StrokeLine(r.X+320, y+44, r.X+470, y+26, gfx.Stroke{Width: 3, Dash: 2, Gap: 6, Cap: gfx.CapRound}, cream)
	cv.StrokeArc(r.X+510, y+30, 16, 0, 2*math.Pi, gfx.Stroke{Width: 1, Dash: 5, Gap: 4, Phase: t * 12}, amber)
	cv.StrokeArc(r.X+560, y+30, 16, -math.Pi/2, math.Pi, gfx.Stroke{Width: 4, Cap: gfx.CapRound}, th.Good)

	// Row 3: polylines and polygons.
	y += rowH
	d.caption(cv, "Polyline / StrokePoly", r.X, y)
	zig := make([]gfx.Pt, 0, 8)
	for i := 0; i < 8; i++ {
		zig = append(zig, gfx.P(r.X+float32(i)*12, y+22+float32(i%2)*20))
	}
	cv.Polyline(zig, 2, amber)
	tri := []gfx.Pt{gfx.P(r.X+110, y+44), gfx.P(r.X+128, y+16), gfx.P(r.X+146, y+44)}
	cv.StrokePoly(tri, 3, cream)
	d.caption(cv, "FillPoly convex / V / concave", r.X+160, y)
	pent := regular(r.X+182, y+31, 15, 5, -math.Pi/2)
	cv.FillPoly(pent, teal)
	cv.StrokePoly(pent, 1, cream)
	cv.FillPolyV(regular(r.X+222, y+31, 15, 7, -math.Pi/2), th.Warn, th.Alert)
	star := starPoly(r.X+264, y+31, 16, 7, 5)
	cv.FillPoly(star, amber)
	cv.StrokePoly(star, 1, th.Ink)
	arrow := []gfx.Pt{
		gfx.P(r.X+290, y+26), gfx.P(r.X+312, y+26), gfx.P(r.X+312, y+18), gfx.P(r.X+330, y+31),
		gfx.P(r.X+312, y+44), gfx.P(r.X+312, y+36), gfx.P(r.X+290, y+36),
	}
	cv.FillPolyV(arrow, th.Info, th.PanelHi)
	d.caption(cv, "FillHex / StrokeHex (pointy-top)", r.X+350, y)
	const hs = 9.0
	for q := -2; q <= 2; q++ {
		for rr := -1; rr <= 1; rr++ {
			hx := r.X + 470 + hs*float32(math.Sqrt(3))*(float32(q)+float32(rr)/2)
			hy := y + 31 + hs*1.5*float32(rr)
			base := th.RockColor((q + rr + 8) % 4)
			cv.FillHexV(hx, hy, hs+0.4, gfx.Lighten(base, 0.12), gfx.Darken(base, 0.18))
			cv.StrokeHex(hx, hy, hs, 0.75, gfx.WithAlpha(th.Space, 0.6))
		}
	}
	cv.StrokeHex(r.X+470, y+31, hs, 1.5, amber)

	// Row 4: circles.
	y += rowH
	d.caption(cv, "FillCircle r = 0.5 … 12", r.X, y)
	x := r.X + 4
	for _, rad := range []float32{0.5, 1, 1.5, 2, 3, 5, 8, 12} {
		cv.FillCircle(x+rad, y+31, rad, cream)
		x += 2*rad + 7
	}
	d.caption(cv, "StrokeCircle 0.5 / 1 / 2 / 5", r.X+160, y)
	for i, w := range []float32{0.5, 1, 2, 5} {
		cv.StrokeCircle(r.X+176+float32(i)*36, y+31, 13, w, amber)
	}
	d.caption(cv, "Arc / FillSector", r.X+320, y)
	cv.Arc(r.X+336, y+31, 13, -math.Pi/2, math.Pi/3, 2, cream)
	cv.Arc(r.X+336, y+31, 8, math.Pi, 0, 3, teal)
	for i, f := range []float32{0.2, 0.45, 0.8} {
		cx := r.X + 380 + float32(i)*36
		cv.FillCircle(cx, y+31, 14, th.Well)
		cv.FillSector(cx, y+31, 13, -math.Pi/2, -math.Pi/2+2*math.Pi*f, th.Good)
		cv.StrokeCircle(cx, y+31, 14, 1, th.Border)
	}
	cv.FillCircleV(r.X+510, y+31, 14, th.Info, th.PanelHi)
	cv.FillCircleV(r.X+550, y+31, 14, th.Warn, th.Alert)

	// Row 5: the same shapes turning slowly, to show the anti-aliasing of
	// slanted edges.
	y += rowH
	d.caption(cv, "anti-aliasing at shallow angles", r.X, y)
	for i := 0; i < 12; i++ {
		a := float64(i)*math.Pi/24 + float64(t)*0.05
		cx, cy := r.X+20+float32(i)*26, y+32
		dx, dy := 12*float32(math.Cos(a)), 12*float32(math.Sin(a))
		cv.Line(cx-dx, cy-dy, cx+dx, cy+dy, 1, cream)
	}
	for i := 0; i < 6; i++ {
		a := float64(i) * math.Pi / 36
		cv.FillPoly(regular(r.X+350+float32(i)*42, y+32, 15, 4, math.Pi/4+a), gfx.Lerp(amber, th.Alert, float32(i)/5))
	}
}

func (d *demo) glows(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	well := r.CutTop(118)
	cv.Panel(well, 4, gfx.CornersAll, th.Space, th.Border)
	cv.PushClip(well.Inset(1))
	cv.Glow(well.X+70, well.MidY(), 62, th.FactionColor(1))
	cv.Glow(well.X+125, well.MidY()-14, 62, th.FactionColor(2))
	cv.Glow(well.X+105, well.MidY()+26, 62, th.FactionColor(3))
	for i := 0; i < 5; i++ {
		cv.GlowCore(well.X+205+float32(i)*30, well.Y+34, 18, gfx.Lerp(th.Info, th.Warn, float32(i)/4))
	}
	// A lit disc casting a soft shadow.
	cv.Blob(well.X+262, well.Y+97, 46, 9, gfx.RGBA(0x000000C0))
	cv.FillCircleV(well.X+262, well.Y+78, 18, gfx.Lighten(th.RockColor(2), 0.2), gfx.Darken(th.RockColor(2), 0.4))
	pulse := 0.6 + 0.4*float32(math.Sin(float64(t)*3))
	cv.Glow(well.X+322, well.Y+82, 24, gfx.Fade(th.Alert, pulse))
	cv.FillCircle(well.X+322, well.Y+82, 3, th.Text)
	cv.PopClip()
	d.caption(cv, "Glow (additive)", well.X+8, well.Y+4)
	d.caption(cv, "GlowCore", well.X+200, well.Y+4)
	d.caption(cv, "Blob", well.X+244, well.Y+104)

	r.CutTop(8)
	d.caption(cv, "SpriteCache → DrawSprite: plain, tint, rotated, additive, DrawImage ×2", r.X, r.Y)
	r.CutTop(18)
	ship := d.sprites.Get(1, 40, 40, cv.Scale, func(sc *gfx.Canvas) { drawShip(sc, th) })
	cy := r.Y + 24
	cv.DrawSprite(ship, r.X+24, cy, nil)
	cv.DrawSprite(ship, r.X+74, cy, &gfx.ImageOpts{Tint: th.FactionColor(3)})
	cv.DrawSprite(ship, r.X+124, cy, &gfx.ImageOpts{Rotation: math.Pi/4 + t*0.5})
	cv.Glow(r.X+174, cy, 26, gfx.Fade(th.Info, 0.5))
	cv.DrawSprite(ship, r.X+174, cy, &gfx.ImageOpts{Additive: true})
	cv.DrawSprite(ship, r.X+224, cy, &gfx.ImageOpts{Tint: gfx.Opacity(0.35)})
	cv.DrawImage(ship, gfx.R(r.X+262, cy-40, 80, 80), nil)

	r.CutTop(72)
	d.caption(cv, "Local canvas: one drawing on a 40-unit grid at 3 sizes", r.X, r.Y)
	x := r.X
	for _, size := range []float32{20, 40, 64} {
		box := gfx.R(x, r.Y+18, size, size)
		cv.StrokeRect(box.Expand(1), 1, th.Border)
		drawShip(cv.Local(box, 40), th)
		x += size + 14
	}
}

// drawShip paints a small craft on a 40 × 40 unit grid.
func drawShip(cv *gfx.Canvas, th *gfx.Theme) {
	hull := []gfx.Pt{gfx.P(20, 3), gfx.P(31, 30), gfx.P(20, 25), gfx.P(9, 30)}
	cv.FillPolyV(hull, gfx.Lighten(th.Goods[4], 0.25), gfx.Darken(th.Goods[4], 0.45))
	cv.StrokePoly(hull, 1, th.Ink)
	cv.FillPoly([]gfx.Pt{gfx.P(20, 9), gfx.P(24, 22), gfx.P(16, 22)}, th.FactionColor(1))
	cv.FillCircle(20, 31, 3, th.Warn)
	cv.Glow(20, 33, 8, gfx.Fade(th.Goods[5], 0.9))
}

func (d *demo) panels(cv *gfx.Canvas, r gfx.Rect) {
	th := d.th
	top := r.CutTop(92)
	for i, c := range []struct {
		name    string
		corners gfx.Corners
	}{
		{"CornersAll", gfx.CornersAll}, {"CornersDiag", gfx.CornersDiag},
		{"CornersTop", gfx.CornersTop}, {"CornerTR", gfx.CornerTR},
	} {
		cell := top.Col(i, 4, 8)
		box := cell.CutTop(cell.H - 16)
		cv.Panel(box, th.Chamfer, c.corners, th.PanelHi, th.Rule)
		inner := box.Inset(8)
		cv.StrokeChamferC(inner, 4, 1, c.corners, th.Border)
		cv.FillChamferC(inner.Inset(6), 3, c.corners, gfx.WithAlpha(th.Rule, 0.25))
		cv.TextIn(cell, c.name, th.Small().Sized(10).WithColor(th.TextDim).Aligned(gfx.AlignCenter))
	}

	r.CutTop(8)
	// A window with a title bar, as the interface will draw them.
	win := r.CutLeft(214)
	cv.Shadow(win.Translate(0, 4), 12, th.Shadow)
	cv.Panel(win, th.Chamfer, gfx.CornersAll, th.Panel, th.Rule)
	bar := win.Inset(1).TakeTop(th.RowH)
	cv.FillChamferV(bar, th.Chamfer-1, gfx.CornersTop, gfx.Lighten(th.PanelHi, 0.04), th.PanelHi)
	cv.FillRect(gfx.R(win.X+1, bar.Bottom(), win.W-2, 1), th.RuleDim)
	cv.TextIn(bar.InsetXY(10, 0), "Launch Cradle", th.Head().Sized(13))
	cv.TextIn(bar.InsetXY(10, 0), "×", th.Head().Aligned(gfx.AlignRight).WithColor(th.TextDim))
	body := win.Inset(10)
	body.CutTop(th.RowH)
	for i, row := range []struct {
		k, v string
		c    gfx.Color
	}{{"Power", "−15", th.Warn}, {"Staff", "4 / 4", th.Good}, {"Queue", "2 of 5", th.Text}} {
		line := body.CutTop(22)
		if i%2 == 1 {
			cv.FillRect(line.InsetXY(-4, 0), gfx.WithAlpha(th.PanelHi, 0.7))
		}
		cv.TextIn(line, row.k, th.Body().WithColor(th.TextDim))
		cv.TextIn(line, row.v, th.Num().Aligned(gfx.AlignRight).WithColor(row.c))
	}
	body.CutTop(8)
	track := body.CutTop(10)
	cv.FillChamfer(track, 3, th.Well)
	cv.FillChamferV(track.Pad(0, 0, track.W*0.38, 0), 3, gfx.CornersAll, gfx.Lighten(th.Rule, 0.15), gfx.Darken(th.Rule, 0.2))
	cv.StrokeChamfer(track, 3, 1, th.Border)

	r.CutLeft(12)
	// Button states.
	d.caption(cv, "button states", r.X, r.Y)
	r.CutTop(16)
	for _, b := range []struct {
		label       string
		top, bottom gfx.Color
		border, ink gfx.Color
	}{
		{"Normal", th.PanelHi, th.PanelHi, th.RuleDim, th.Text},
		{"Hover", gfx.Lighten(th.PanelHi, 0.08), gfx.Lighten(th.PanelHi, 0.04), th.Rule, th.Text},
		{"Pressed", gfx.Darken(th.PanelHi, 0.25), gfx.Darken(th.PanelHi, 0.15), th.Rule, th.Text},
		{"Primary", gfx.Lighten(th.Rule, 0.1), gfx.Darken(th.Rule, 0.12), th.Rule, th.Ink},
		{"Disabled", th.Panel, th.Panel, th.Border, th.TextFaint},
	} {
		btn := r.CutTop(th.RowH)
		r.CutTop(5)
		cv.PanelV(btn, 5, gfx.CornersDiag, b.top, b.bottom, b.border, 1)
		cv.TextIn(btn, b.label, th.Body().Weighted(gfx.Medium).WithColor(b.ink).Aligned(gfx.AlignCenter))
	}
}

func (d *demo) styles(cv *gfx.Canvas, r gfx.Rect) {
	th := d.th
	for _, s := range []struct {
		name string
		st   gfx.TextStyle
		text string
	}{
		{"Display 40", th.Display(), "Drifthold"},
		{"Title 22", th.Title(), "The Shoal of Emberfall"},
		{"Head 16", th.Head(), "Free Charter of Ansel"},
		{"Label 11", th.Label(), "Storage capacity"},
		{"Body 13", th.Body(), "Bore Rigs mine 36 units per cycle from the veins around them."},
		{"Body medium", th.Body().Weighted(gfx.Medium), "Smelter: 30 Ore → 20 Alloy per cycle."},
		{"Body bold", th.Body().Weighted(gfx.Bold), "Keystone under attack at Glint-7!"},
		{"Small 11", th.Small(), "Deposits are hidden until surveyed · sensor range 260 u"},
		{"Num 13", th.Num(), "0123456789  1,204.5  −15  +160  ×4"},
		{"Num bold", th.Num().Weighted(gfx.Bold), "Cycle 12   Scrip 4,310"},
	} {
		h := cv.LineHeight(s.st)
		line := r.CutTop(max(h, 16))
		r.CutTop(3)
		tag := line.CutLeft(86)
		cv.TextIn(tag, s.name, th.Small().WithColor(th.TextDim))
		cv.TextIn(line, s.text, s.st)
	}
}

func (d *demo) layout(cv *gfx.Canvas, r gfx.Rect) {
	th := d.th
	left := r.CutLeft(250)
	r.CutLeft(14)

	// Alignment against a guide.
	d.caption(cv, "Align: left, centre, right of the guide", left.X, left.Y)
	gx := left.X + 125
	cv.Line(gx, left.Y+16, gx, left.Y+74, 1, th.RuleDim)
	cv.Text("Left of anchor", gx, left.Y+18, th.Body())
	cv.Text("Centred on anchor", gx, left.Y+36, th.Body().Aligned(gfx.AlignCenter))
	cv.Text("Right of anchor", gx, left.Y+54, th.Body().Aligned(gfx.AlignRight))
	left.CutTop(82)

	// TextIn: vertical centring in rows.
	d.caption(cv, "TextIn: centred on the capital height", left.X, left.Y)
	left.CutTop(16)
	for i, a := range []gfx.Align{gfx.AlignLeft, gfx.AlignCenter, gfx.AlignRight} {
		row := left.CutTop(th.RowH)
		left.CutTop(3)
		cv.Panel(row, 4, gfx.CornersDiag, th.PanelHi, th.Border)
		cv.Line(row.X, row.MidY(), row.Right(), row.MidY(), 1, gfx.WithAlpha(th.Rule, 0.25))
		text := []string{"Aerator", "Greenhouse Hxgjpq", "1,204"}[i]
		st := []gfx.TextStyle{th.Body(), th.Head().Sized(13), th.Num()}[i]
		cv.TextIn(row.InsetXY(8, 0), text, st.Aligned(a))
	}
	left.CutTop(5)

	// Tracking.
	d.caption(cv, "Tracking 0 / 1 / 3 pt", left.X, left.Y)
	for i, tr := range []float32{0, 1, 3} {
		st := th.Label().WithColor(th.Text)
		st.Tracking = tr
		cv.Text("Orbital flight", left.X, left.Y+16+float32(i)*15, st)
	}
	// Number table, right-aligned monospace.
	for i, v := range []string{"1,204.5", "36.0", "−0.25"} {
		cv.Text(v, left.Right(), left.Y+16+float32(i)*15, th.NumSmall().Aligned(gfx.AlignRight))
	}

	// Wrapping.
	const para = "Micrometeor squall inbound: 8–14 impacts over ten seconds. Aegis Projectors absorb the damage while their pool lasts.\nExtraordinarilylongunbreakableword splits."
	d.caption(cv, "WrapText at 190 and 120 pt", r.X, r.Y)
	wrap := r.CutTop(150)
	wrap.CutTop(16)
	for _, w := range []float32{190, 120} {
		box := wrap.CutLeft(w)
		wrap.CutLeft(12)
		cv.FillRect(box, th.Well)
		cv.Line(box.Right(), box.Y, box.Right(), box.Bottom(), 1, th.RuleDim)
		st := th.Small()
		cv.PushClip(box.Expand(1))
		cv.TextLines(cv.WrapText(para, w, st), box.X, box.Y, st)
		cv.PopClip()
	}

	// Ellipsis.
	r.CutTop(6)
	d.caption(cv, "TextFit (Ellipsize) at 220, 150, 90, 30 pt", r.X, r.Y)
	r.CutTop(16)
	for _, w := range []float32{220, 150, 90, 30} {
		row := r.CutTop(19)
		r.CutTop(3)
		box := row.CutLeft(w)
		cv.FillRect(box, th.Well)
		cv.Line(box.Right(), box.Y, box.Right(), box.Bottom(), 1, th.RuleDim)
		cv.TextFit(box, "Meridian Lattice claims another rock", th.Body())
	}
}

func (d *demo) clips(cv *gfx.Canvas, r gfx.Rect) {
	th := d.th
	outer := r.Inset(4)
	cv.StrokeRect(outer.Expand(1), 1, th.Border)
	cv.PushClip(outer)
	cv.FillRect(outer, th.Well)
	for i := 0; i < 7; i++ {
		cv.FillCircle(outer.X+float32(i)*48, outer.Y+float32(i)*14, 34, gfx.WithAlpha(th.FactionColor(1+i%4), 0.55))
	}
	cv.Text("Clipped text runs off the right edge of the outer clip", outer.X+90, outer.Y+8, th.Body())

	inner := outer.Center(outer.W*0.55, outer.H*0.5).Translate(20, 14)
	cv.PushClip(inner)
	cv.FillRect(outer, th.PanelHi) // only the inner clip gets painted
	for i := 0; i < 9; i++ {
		x := inner.X - 30 + float32(i)*26
		cv.Line(x, inner.Y-10, x+60, inner.Bottom()+10, 3, th.Rule)
	}
	cv.Text("NESTED CLIP — cut on all four sides, wherever it falls", inner.X-24, inner.MidY()-9, th.Head())
	cv.PopClip()

	// Back in the outer clip: this would be cut by the inner one.
	cv.StrokeRect(inner.Expand(2), 1, th.Text)
	sub := cv.Sub(gfx.R(outer.X+8, outer.Bottom()-30, 96, 22))
	sub.FillRect(outer, gfx.WithAlpha(th.Good, 0.3))
	sub.Text("Sub(rect) canvas clips too", outer.X+12, outer.Bottom()-27, th.Small())
	cv.PopClip()
}

func (d *demo) paths(cv *gfx.Canvas, r gfx.Rect) {
	th := d.th
	d.caption(cv, "DrawPath: fill, V-gradient, stroke joins, even-odd hole, transforms", r.X, r.Y)
	y := r.Y + 22
	var p gfx.Path

	// A curved hull, drawn three ways from one path via the transform.
	p.MoveTo(0, -30)
	p.CubicTo(22, -20, 26, 10, 14, 30)
	p.QuadTo(0, 22, -14, 30)
	p.CubicTo(-26, 10, -22, -20, 0, -30)
	p.Close()
	cv.DrawPath(&p, gfx.PathStyle{Fill: th.Goods[4], X: r.X + 34, Y: y + 36})
	cv.DrawPath(&p, gfx.PathStyle{
		Fill: gfx.Lighten(th.Goods[4], 0.3), FillTo: gfx.Darken(th.Goods[4], 0.55),
		Stroke: th.Ink, Width: 1.5, Join: gfx.JoinRound, X: r.X + 100, Y: y + 36,
	})
	cv.DrawPath(&p, gfx.PathStyle{
		Stroke: th.Rule, Width: 2, X: r.X + 166, Y: y + 36, Scale: 0.8, Rotation: math.Pi / 5,
	})

	// Joins on a thick zigzag.
	for i, j := range []gfx.Join{gfx.JoinMiter, gfx.JoinRound, gfx.JoinBevel} {
		p.Reset()
		x := r.X + 214 + float32(i)*62
		p.Polyline([]gfx.Pt{gfx.P(x, y+62), gfx.P(x+16, y+12), gfx.P(x+32, y+62), gfx.P(x+48, y+12)})
		cv.DrawPath(&p, gfx.PathStyle{Stroke: th.Info, Width: 7, Join: j, Cap: gfx.Cap(i)})
	}

	// A ring with a hole (even-odd) and a rounded rectangle.
	p.Reset()
	p.Circle(r.X+430, y+36, 30)
	p.Circle(r.X+430, y+36, 16)
	p.Hex(r.X+430, y+36, 9)
	cv.DrawPath(&p, gfx.PathStyle{Fill: th.Rule, FillTo: th.Alert, EvenOdd: true})

	p.Reset()
	p.RoundRect(gfx.R(r.X+474, y+8, 40, 56), 10)
	cv.DrawPath(&p, gfx.PathStyle{Fill: th.Well, Stroke: th.Good, Width: 1.5})
	p.Reset()
	p.Ellipse(r.X+494, y+36, 12, 20)
	cv.FillPathV(&p, th.Good, gfx.WithAlpha(th.Good, 0))

	// Concave fills through FillPoly, at small sizes.
	d.caption(cv, "concave FillPoly at 6 … 20 pt", r.X, y+78)
	x := r.X + 14
	for _, size := range []float32{6, 9, 12, 16, 20} {
		cv.FillPoly(starPoly(x, y+112, size, size*0.42, 5), th.Warn)
		x += size*2 + 12
	}
	d.caption(cv, "Path.Chamfer stroked round", r.X+260, y+78)
	p.Reset()
	p.Chamfer(gfx.R(r.X+262, y+98, 120, 30), 8, gfx.CornersDiag)
	cv.DrawPath(&p, gfx.PathStyle{Fill: th.PanelHi, Stroke: th.Rule, Width: 3, Join: gfx.JoinRound})
}

func (d *demo) backdrop(cv *gfx.Canvas, r gfx.Rect) {
	th := d.th
	// No fill: the backdrop itself shows through this frame.
	cv.StrokeChamfer(r, th.Chamfer, 1, th.RuleDim)
	cv.TextIn(r.Inset(th.Pad).TakeTop(20), "Backdrop (seeded starfield, 3 parallax layers, Emberfall glow)", th.Head().Sized(13))
}

// stress draws n extra primitives of mixed kinds, for timing.
func (d *demo) stress(cv *gfx.Canvas, n int) {
	th := d.th
	for i := 0; i < n; i++ {
		x := float32(20 + (i%80)*19)
		y := float32(70 + (i/80)*19)
		c := gfx.WithAlpha(th.FactionColor(i%5), 0.5)
		switch i % 6 {
		case 0:
			cv.FillRect(gfx.R(x, y, 14, 14), c)
		case 1:
			cv.Panel(gfx.R(x, y, 16, 16), 4, gfx.CornersAll, th.PanelHi, c)
		case 2:
			cv.FillCircle(x+8, y+8, 7, c)
		case 3:
			cv.Line(x, y, x+15, y+12, 1.5, c)
		case 4:
			cv.FillHex(x+8, y+8, 8, c)
		case 5:
			cv.Text("Ab1", x, y, th.Small())
		}
	}
}

// regular returns a regular n-gon.
func regular(cx, cy, r float32, n int, a0 float64) []gfx.Pt {
	pts := make([]gfx.Pt, n)
	for i := range pts {
		sin, cos := math.Sincos(a0 + 2*math.Pi*float64(i)/float64(n))
		pts[i] = gfx.P(cx+r*float32(cos), cy+r*float32(sin))
	}
	return pts
}

// starPoly returns a star with n points.
func starPoly(cx, cy, outer, inner float32, n int) []gfx.Pt {
	pts := make([]gfx.Pt, 2*n)
	for i := range pts {
		r := outer
		if i%2 == 1 {
			r = inner
		}
		sin, cos := math.Sincos(-math.Pi/2 + math.Pi*float64(i)/float64(n))
		pts[i] = gfx.P(cx+r*float32(cos), cy+r*float32(sin))
	}
	return pts
}
