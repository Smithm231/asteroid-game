// Command demo renders the contact sheets of package art. It is the visual
// regression test of the package: run it, then look at the pictures.
//
//	xvfb-run -a -s "-screen 0 3840x2160x24" go run ./internal/art/demo -out <dir> -scale 1
//
// writes one PNG per sheet into <dir>:
//
//	buildings               every kind at hex sizes 56, 28, 18, 14 and 10 pt
//	buildings_states_small  every kind × running, off, 30 % and 70 % built,
//	                        damaged, derelict, at 14, 18 and 28 pt
//	buildings_states_a, _b  the same states at 44 pt
//	buildings_motion        every kind at thirteen moments of its animation
//	buildings_bare          every kind on a flat field, for silhouette checks
//	ships                   every kind in the four faction accents, at 48, 24,
//	                        14 and 10 pt and many headings, with its icon
//	goods_icons             goods and interface glyphs at 12, 16, 20, 24 pt
//	                        and larger
//	surface_zooms           a colony rock at hex sizes 10 and 16 pt, with and
//	                        without the deposit overlay
//	surface_close           the same rock at 28 pt with its colony
//	surface_close_deposits  ... fully surveyed, deposit overlay, rubble
//	surface_classes         one rock of each class, unsurveyed and surveyed
//	surface_extreme         a close-up at 150 pt, past the largest sprites
//	misc                    rock badges, Emberfall, an explosion strip
//
// Run it at -scale 1 and -scale 2. With -only <name> a single sheet is
// rendered. With -bench N the program instead draws a busy colony (a full
// rock surface and 150 running buildings; -bench-buildings changes the
// number) for N frames and prints what a frame costs.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/art"
	"drifthold/internal/gfx"
	"drifthold/internal/input"
	"drifthold/internal/model"
	"drifthold/internal/sim"
	"drifthold/internal/uitest"
)

var (
	only   = flag.String("only", "", "render only the sheet with this name")
	bench  = flag.Int("bench", 0, "time this many frames of a busy colony instead of rendering the sheets")
	seed   = flag.Uint64("seed", 7, "world seed for the rocks shown")
	benchN = flag.Int("bench-buildings", 150, "number of buildings in the -bench scene")
)

// framesPerSheet is how long each sheet stays up; the screenshot is taken on
// its sixth frame, when sprite and surface caches have settled.
const framesPerSheet = 8

type sheet struct {
	name string
	draw func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32)
}

var sheets = []sheet{
	{"buildings", (*demo).buildingGallery},
	{"buildings_states_small", (*demo).buildingStatesSmall},
	{"buildings_states_a", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.buildingStatesLarge(cv, r, t, 0, 13) }},
	{"buildings_states_b", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.buildingStatesLarge(cv, r, t, 13, 25) }},
	{"buildings_motion", (*demo).buildingMotion},
	{"buildings_bare", (*demo).buildingBare},
	{"ships", (*demo).ships},
	{"goods_icons", (*demo).goodsIcons},
	{"surface_zooms", (*demo).surfaceZooms},
	{"surface_close", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.surfaceClose(cv, r, t, false) }},
	{"surface_close_deposits", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.surfaceClose(cv, r, t, true) }},
	{"surface_classes", (*demo).surfaceClasses},
	{"surface_extreme", (*demo).surfaceExtreme},
	{"misc", (*demo).misc},
}

func main() {
	o := uitest.Flags()
	o.Title = "Drifthold art contact sheets"
	w, err := sim.NewGame(model.DefaultSettings(*seed))
	if err != nil {
		log.Fatal(err)
	}
	d := &demo{th: gfx.Default(), world: w, sky: gfx.NewBackdrop(0xA57), surf: art.NewSurface()}
	d.prepare()

	if *only != "" {
		var keep []sheet
		for _, s := range sheets {
			if s.name == *only {
				keep = append(keep, s)
			}
		}
		if len(keep) == 0 {
			log.Fatalf("no sheet named %q", *only)
		}
		sheets = keep
	}
	if *bench > 0 {
		o.Script = []input.Step{input.Wait(*bench), input.Quit()}
	} else if o.Script == nil {
		for i, s := range sheets {
			wait := framesPerSheet - 1
			if i == 0 {
				wait = framesPerSheet - 3
			}
			o.Script = append(o.Script, input.Wait(wait), input.Shot(s.name+".png"))
		}
		o.Script = append(o.Script, input.Quit())
	}
	if err := uitest.Run(o, d.update, d.draw); err != nil {
		log.Fatal(err)
	}
	if *bench > 0 && d.benchFrames > 0 {
		fmt.Printf("bench: %d buildings on a %d-tile rock at hex size %.0f pt, scale %.2f\n",
			d.benchBuildings, d.benchTiles, benchHex, d.in.Scale)
		fmt.Printf("bench: draw callback %.3f ms/frame (surface %.3f ms, buildings %.3f ms) over %d frames; first frame %.1f ms; %d sprites cached\n",
			ms(d.benchTotal)/float64(d.benchFrames), ms(d.benchSurface)/float64(d.benchFrames),
			ms(d.benchBuild)/float64(d.benchFrames), d.benchFrames, ms(d.benchFirst), art.CachedSprites())
		fmt.Printf("bench: whole frame, backdrop and presentation included, %.2f ms on this machine's renderer\n",
			ms(d.benchSpan)/float64(d.benchFrames))
		fmt.Printf("bench: painting the static layer of the rock afresh (once per zoom level): %.1f ms at hex size %.0f, %.1f ms at hex size 28\n",
			ms(d.benchPaint[0]), benchHex, ms(d.benchPaint[1]))
	}
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

type demo struct {
	th    *gfx.Theme
	world *model.World
	sky   *gfx.Backdrop
	surf  *art.Surface
	in    input.State

	home     *model.Rock
	explored *model.Rock // a copy of the home rock, fully surveyed
	placed   []placedBuilding
	classes  [model.NumRockClasses]*model.Rock

	benchTotal, benchSurface, benchBuild, benchFirst time.Duration
	benchSpan                                        time.Duration
	benchPaint                                       [2]time.Duration
	benchLast                                        time.Time
	benchFrames, benchBuildings, benchTiles          int
}

type placedBuilding struct {
	hex  model.Hex
	kind model.BuildingKind
	look art.BuildingLook
}

func (d *demo) update(in input.State) { d.in = in }

// prepare picks the rocks to show and lays out a colony on the home rock.
func (d *demo) prepare() {
	w := d.world
	d.home = w.Rock(w.Human().HomeRock)
	for _, r := range w.Rocks {
		if c := int(r.Class); c >= 0 && c < len(d.classes) && d.classes[c] == nil && r != d.home {
			d.classes[c] = r
		}
	}
	if d.classes[d.home.Class] == nil {
		d.classes[d.home.Class] = d.home
	}

	// A fully surveyed twin of the home rock, with some rubble and a few
	// half-mined deposits.
	cp := *d.home
	cp.Tiles = append([]model.Tile(nil), d.home.Tiles...)
	cp.ID = 9001 // a rock in its own right as far as caches are concerned
	d.explored = &cp
	d.explored.RevealAll(model.PlayerFaction)
	n := 0
	for i := range d.explored.Tiles {
		t := &d.explored.Tiles[i]
		if t.HasDeposit() {
			n++
			switch n % 4 {
			case 1:
				t.Amount *= 0.15
			case 2:
				t.Amount *= 0.5
			}
		}
	}

	// The colony: every kind once, spiralling out from the Keystone, in a
	// mix of states.
	centre := model.Hex{}
	if k := d.home.Keystone(); k != nil {
		centre = k.Hex
	}
	amber := d.th.FactionColor(1)
	d.placed = append(d.placed, placedBuilding{centre, model.Keystone, art.BuildingLook{Accent: amber, Progress: 1}})
	kind := model.BuildingKind(1)
	extra := 0
	for _, h := range centre.Spiral(6)[1:] {
		t := d.home.TileAt(h)
		if t == nil || t.Terrain != model.TerrainRegolith {
			continue
		}
		if (h.Q*7+h.R*13)%5 == 0 {
			continue // leave some ground open
		}
		look := art.BuildingLook{Accent: amber, Progress: 1}
		k := kind
		if kind < model.NumBuildingKinds {
			kind++
		} else {
			// Round the colony off with conduits and a few special cases.
			if extra >= 18 {
				break
			}
			switch extra % 7 {
			case 0:
				k, look.Progress = model.Habitat, 0.35
			case 1:
				k, look.Off = model.Smelter, true
			case 2:
				k, look.Damage = model.Sunvane, 0.8
			case 3:
				k, look.Derelict, look.Accent = model.Silo, true, d.th.FactionColor(0)
			case 4:
				k, look.Progress = model.BoreRig, 0.7
			default:
				k = model.Conduit
			}
			extra++
		}
		d.placed = append(d.placed, placedBuilding{h, k, look})
	}
	// Rubble on a few free tiles of the explored twin.
	used := map[model.Hex]bool{}
	for _, p := range d.placed {
		used[p.hex] = true
	}
	nr := 0
	for _, h := range centre.Spiral(7) {
		t := d.explored.TileAt(h)
		if t != nil && t.Terrain == model.TerrainRegolith && !used[h] && (h.Q*5+h.R*3)%4 == 1 {
			t.Rubble = true
			if o := d.home.TileAt(h); o != nil {
				o.Rubble = true
			}
			nr++
			if nr >= 6 {
				break
			}
		}
	}
}

func (d *demo) draw(dst *ebiten.Image) {
	in := d.in
	if in.Scale == 0 {
		return
	}
	cv := gfx.NewCanvas(dst, in.Scale)
	t := float32(in.Frame) / 60
	if *bench > 0 {
		d.benchScene(cv, t)
		return
	}
	i := min(len(sheets)-1, int(in.Frame-1)/framesPerSheet)
	// Every sheet is photographed at the same moment of the animation.
	t = 1.37 + float32(int(in.Frame-1)%framesPerSheet)/60
	d.sky.Draw(cv, gfx.BackdropView{EmberX: in.W + 200, EmberY: in.H + 300, EmberRadius: 900, Time: t})
	screen := gfx.R(0, 0, in.W, in.H).Inset(10)
	head := screen.CutTop(30)
	cv.TextIn(head, "Drifthold art · "+sheets[i].name, d.th.Head().WithColor(d.th.Rule))
	cv.TextIn(head, fmt.Sprintf("scale %.2f   %d sprites cached", in.Scale, art.CachedSprites()),
		d.th.NumSmall().Aligned(gfx.AlignRight).WithColor(d.th.TextDim))
	cv.FillRect(gfx.R(screen.X, head.Bottom(), screen.W, 1), d.th.RuleDim)
	screen.CutTop(8)
	sheets[i].draw(d, cv, screen, t)
}

func (d *demo) caption(cv *gfx.Canvas, s string, x, y float32) {
	cv.Text(s, x, y, d.th.Small().WithColor(d.th.TextDim))
}

// ground paints one rock-coloured tile for a building to stand on.
func (d *demo) ground(cv *gfx.Canvas, x, y, size float32, class int) {
	base := d.th.RockColor(class)
	cv.FillHexV(x, y, size, gfx.Lighten(base, 0.03), gfx.Darken(base, 0.06))
	cv.StrokeHex(x, y, size, 1, gfx.WithAlpha(gfx.Darken(base, 0.5), 0.5))
}

// ---------------------------------------------------------------------------
// Buildings
// ---------------------------------------------------------------------------

func (d *demo) buildingGallery(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	const cols, rows = 7, 4
	for i, kind := range model.AllBuildingKinds() {
		cell := r.Cell(i%cols, i/cols, cols, rows, 6)
		cv.Panel(cell, 5, gfx.CornersDiag, gfx.WithAlpha(th.Panel, 0.85), th.Border)
		label := cell.CutBottom(20)
		cv.TextIn(label, kind.String(), th.Small().Aligned(gfx.AlignCenter))
		class := i % 4
		look := art.BuildingLook{Accent: th.FactionColor(1 + (i/cols)%4), Progress: 1, Time: t}
		x := cell.X + 62
		y := cell.MidY()
		d.ground(cv, x, y, 56, class)
		art.Building(cv, kind, x, y, 56, look)
		x += 56 + 38
		d.ground(cv, x, y-26, 28, class)
		art.Building(cv, kind, x, y-26, 28, look)
		d.ground(cv, x-8, y+40, 18, class)
		art.Building(cv, kind, x-8, y+40, 18, look)
		d.ground(cv, x+28, y+34, 14, class)
		art.Building(cv, kind, x+28, y+34, 14, look)
		d.ground(cv, x+26, y+66, 10, class)
		art.Building(cv, kind, x+26, y+66, 10, look)
	}
}

type lookState struct {
	name string
	look art.BuildingLook
}

func (d *demo) states(accent gfx.Color, t float32) []lookState {
	return []lookState{
		{"running", art.BuildingLook{Accent: accent, Progress: 1, Time: t}},
		{"off", art.BuildingLook{Accent: accent, Progress: 1, Off: true, Time: t}},
		{"built 30 %", art.BuildingLook{Accent: accent, Progress: 0.3, Time: t}},
		{"built 70 %", art.BuildingLook{Accent: accent, Progress: 0.7, Time: t}},
		{"damaged", art.BuildingLook{Accent: accent, Progress: 1, Damage: 0.75, Time: t}},
		{"derelict", art.BuildingLook{Accent: d.th.FactionColor(0), Progress: 1, Derelict: true, Time: t}},
	}
}

func (d *demo) buildingStatesSmall(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	kinds := model.AllBuildingKinds()
	y := r.Y
	for bi, size := range []float32{14, 18, 28} {
		accent := th.FactionColor(1 + bi)
		states := d.states(accent, t)
		cw := (r.W - 80) / float32(len(kinds))
		ch := size*2 + 2
		d.caption(cv, fmt.Sprintf("hex size %.0f pt", size), r.X, y)
		y += 16
		for si, st := range states {
			cy := y + ch/2
			cv.TextIn(gfx.R(r.X, cy-8, 76, 16), st.name, th.Small().WithColor(th.TextDim))
			for i, kind := range kinds {
				cx := r.X + 80 + (float32(i)+0.5)*cw
				d.ground(cv, cx, cy, size, (i+si)%4)
				art.Building(cv, kind, cx, cy, size, st.look)
			}
			y += ch
		}
		y += 6
	}
}

func (d *demo) buildingStatesLarge(cv *gfx.Canvas, r gfx.Rect, t float32, from, to int) {
	th := d.th
	kinds := model.AllBuildingKinds()
	const size = 44
	states := d.states(th.FactionColor(3), t)
	if from > 0 {
		states = d.states(th.FactionColor(4), t)
	}
	n := to - from
	cw := (r.W - 80) / float32(n)
	head := r.CutTop(18)
	for i := from; i < to; i++ {
		cv.TextIn(gfx.R(r.X+80+float32(i-from)*cw, head.Y, cw, head.H), kinds[i].String(), th.Small().Aligned(gfx.AlignCenter))
	}
	ch := r.H / float32(len(states))
	for si, st := range states {
		cy := r.Y + (float32(si)+0.5)*ch
		cv.TextIn(gfx.R(r.X, cy-8, 76, 16), st.name, th.Small().WithColor(th.TextDim))
		for i := from; i < to; i++ {
			cx := r.X + 80 + (float32(i-from)+0.5)*cw
			d.ground(cv, cx, cy, size, (i+si)%4)
			art.Building(cv, kinds[i], cx, cy, size, st.look)
		}
	}
}

// buildingBare draws every kind on a flat field with nothing around it, at
// the sizes the game uses most. Thresholding this sheet against its
// background gives the silhouettes, which is how to check that each kind can
// be told from the others by outline alone.
func (d *demo) buildingBare(cv *gfx.Canvas, r gfx.Rect, t float32) {
	cv.FillRect(gfx.R(0, 0, d.in.W, d.in.H), gfx.RGB(0x202020))
	kinds := model.AllBuildingKinds()
	y := r.Y + 30
	for _, size := range []float32{14, 16, 18, 28, 44} {
		cw := r.W / float32(len(kinds))
		for i, kind := range kinds {
			x := r.X + (float32(i)+0.5)*cw
			art.Building(cv, kind, x, y+size, size, art.BuildingLook{Accent: d.th.FactionColor(1), Progress: 1, Off: true})
		}
		y += size*2 + 30
	}
	for i, kind := range kinds {
		cw := r.W / float32(len(kinds))
		cv.TextIn(gfx.R(r.X+float32(i)*cw, y, cw, 16), kind.String(), d.th.Small().Sized(9).Aligned(gfx.AlignCenter))
	}
}

// buildingMotion shows every kind at a run of moments, to check what moves:
// one row per kind, time running to the right.
func (d *demo) buildingMotion(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	kinds := model.AllBuildingKinds()
	const size, frames, step = 21, 13, 0.21
	rows := (len(kinds) + 1) / 2
	rowH := r.H / float32(rows)
	for i, kind := range kinds {
		col := gfx.R(r.X+float32(i/rows)*r.W/2, r.Y+float32(i%rows)*rowH, r.W/2-8, rowH)
		cv.TextIn(col.CutLeft(104), kind.String(), th.Small().WithColor(th.TextDim))
		cw := col.W / frames
		for f := 0; f < frames; f++ {
			x := col.X + (float32(f)+0.5)*cw
			y := col.MidY()
			d.ground(cv, x, y, size, i%4)
			look := art.BuildingLook{Accent: th.FactionColor(1 + i%4), Progress: 1, Time: float32(f) * step}
			art.Building(cv, kind, x, y, size, look)
		}
	}
}

// ---------------------------------------------------------------------------
// Ships
// ---------------------------------------------------------------------------

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
// Surfaces
// ---------------------------------------------------------------------------

// rockExtent returns the size in points of a rock's tile disc at hexSize.
func rockExtent(rock *model.Rock, hexSize float32) (w, h float32) {
	n := float32(2*rock.Radius + 1)
	return n * hexSize * 1.7320508, (1.5*n + 0.5) * hexSize
}

// colony draws the demo colony on the home rock.
func (d *demo) colony(cv *gfx.Canvas, centre gfx.Pt, hexSize, t float32) {
	for _, p := range d.placed {
		v := model.HexToPixel(p.hex, float64(hexSize))
		look := p.look
		look.Time = t
		art.Building(cv, p.kind, centre.X+float32(v.X), centre.Y+float32(v.Y), hexSize, look)
	}
}

func (d *demo) surfaceView(cv *gfx.Canvas, r gfx.Rect, rock *model.Rock, hexSize, t float32, deposits, colony bool, label string) {
	cv.StrokeChamfer(r, 5, 1, d.th.Border)
	cv.PushClip(r.Inset(1))
	c := r.Mid()
	d.surf.Draw(cv, rock, c, hexSize, model.PlayerFaction, deposits)
	if colony {
		d.colony(cv, c, hexSize, t)
	}
	cv.PopClip()
	d.caption(cv, label, r.X+8, r.Y+6)
}

func (d *demo) surfaceZooms(cv *gfx.Canvas, r gfx.Rect, t float32) {
	left := r.CutLeft(r.W * 0.36)
	r.CutLeft(8)
	a := left.CutTop(left.H/2 - 4)
	left.CutTop(8)
	d.surfaceView(cv, a, d.home, 10, t, false, true, "hex size 10 · colony · partly surveyed")
	d.surfaceView(cv, left, d.explored, 10, t, true, false, "hex size 10 · deposit overlay · fully surveyed")
	b := r.CutLeft(r.W/2 - 4)
	r.CutLeft(8)
	d.surfaceView(cv, b, d.home, 16, t, false, true, "hex size 16 · colony · partly surveyed")
	d.surfaceView(cv, r, d.home, 16, t, true, false, "hex size 16 · deposit overlay · partly surveyed")
}

func (d *demo) surfaceClose(cv *gfx.Canvas, r gfx.Rect, t float32, deposits bool) {
	rock := d.home
	label := "hex size 28 · colony · partly surveyed"
	if deposits {
		rock = d.explored
		label = "hex size 28 · deposit overlay · fully surveyed, with rubble and half-mined veins"
	}
	d.surfaceView(cv, r, rock, 28, t, deposits, !deposits, label)
}

// surfaceExtreme zooms in further than any sprite or surface layer is
// rendered, where the art falls back to stretching.
func (d *demo) surfaceExtreme(cv *gfx.Canvas, r gfx.Rect, t float32) {
	const hex = 150
	cv.StrokeChamfer(r, 5, 1, d.th.Border)
	cv.PushClip(r.Inset(1))
	centre := model.Hex{}
	if k := d.home.Keystone(); k != nil {
		centre = k.Hex
	}
	v := model.HexToPixel(centre, hex)
	c := gfx.P(r.MidX()-float32(v.X)-200, r.MidY()-float32(v.Y))
	d.surf.Draw(cv, d.explored, c, hex, model.PlayerFaction, true)
	d.colony(cv, c, hex, t)
	cv.PopClip()
	d.caption(cv, "hex size 150 · deposit overlay", r.X+8, r.Y+6)
}

func (d *demo) surfaceClasses(cv *gfx.Canvas, r gfx.Rect, t float32) {
	for i, rock := range d.classes {
		if rock == nil {
			continue
		}
		cell := r.Cell(i%2, i/2, 2, 2, 8)
		half := cell.CutLeft(cell.W / 2)
		d.surfaceView(cv, half, rock, 13, t, false, false, fmt.Sprintf("%s · %s · unsurveyed", rock.Name, rock.Class))
		cp := *rock
		cp.ID += 9100
		cp.SurveyedBy.Set(model.PlayerFaction)
		// A different Surface value would be the usual way to show the same
		// rock twice; sharing one exercises its cache instead.
		d.surfaceView(cv, cell, &cp, 13, t, true, false, "surveyed · deposit overlay")
	}
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
		x := r.X + 60 + float32(i)*150
		y := r.Y + 450
		for q := -2; q <= 2; q++ {
			for rr := -1; rr <= 1; rr++ {
				v := model.HexToPixel(model.Hex{Q: q, R: rr}, 22)
				d.ground(cv, x+float32(v.X), y+float32(v.Y), 22, 1)
			}
		}
		art.Building(cv, model.Habitat, x, y, 22, art.BuildingLook{Accent: th.FactionColor(2), Progress: 1, Damage: float32(i) / 5, Time: t})
		art.Explosion(cv, x, y, 30, float32(i)/6)
	}
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

const benchHex float32 = 16

func (d *demo) benchScene(cv *gfx.Canvas, t float32) {
	start := time.Now()
	in := d.in
	d.sky.Draw(cv, gfx.BackdropView{Time: t})
	rock := d.home
	c := gfx.P(in.W/2, in.H/2)
	t0 := time.Now()
	d.surf.Draw(cv, rock, c, benchHex, model.PlayerFaction, true)
	surface := time.Since(t0)
	t0 = time.Now()
	n, tiles := 0, 0
	kinds := model.AllBuildingKinds()
	for pass := 0; pass < 4 && n < *benchN; pass++ {
		for i := range rock.Tiles {
			h, ok := rock.HexAt(i)
			if !ok || rock.Tiles[i].Terrain != model.TerrainRegolith {
				continue
			}
			if pass == 0 {
				tiles++
			}
			if n >= *benchN {
				continue
			}
			v := model.HexToPixel(h, float64(benchHex))
			// Later passes stack further buildings beside the rock so that
			// small rocks still carry 150.
			x := c.X + float32(v.X) + float32(pass)*3
			y := c.Y + float32(v.Y) + float32(pass)*2
			look := art.BuildingLook{Accent: d.th.FactionColor(1 + n%4), Progress: 1, Time: t + float32(n)*0.37}
			switch {
			case n%19 == 7:
				look.Progress = 0.5
			case n%23 == 5:
				look.Off = true
			case n%17 == 3:
				look.Damage = 0.8
			}
			art.Building(cv, kinds[n%len(kinds)], x, y, benchHex, look)
			n++
		}
	}
	build := time.Since(t0)
	total := time.Since(start)
	if in.Frame <= 1 {
		d.benchFirst = total
	}
	// Skip the frames in which the caches fill.
	if in.Frame > 10 {
		if !d.benchLast.IsZero() {
			d.benchSpan += time.Since(d.benchLast)
		}
		d.benchTotal += total
		d.benchSurface += surface
		d.benchBuild += build
		d.benchFrames++
	}
	if in.Frame == 6 {
		// What a change of zoom costs: a fresh Surface has to paint the
		// rock's static layer.
		for i, hex := range [2]float32{benchHex, 28} {
			fresh := art.NewSurface()
			t0 := time.Now()
			fresh.Draw(cv, rock, c, hex, model.PlayerFaction, false)
			d.benchPaint[i] = time.Since(t0)
			fresh.Release()
		}
	}
	d.benchLast = time.Now()
	d.benchBuildings, d.benchTiles = n, tiles
	cv.Text(fmt.Sprintf("%d buildings · %d sprites cached", n, art.CachedSprites()), 12, 12, d.th.Body())
}
