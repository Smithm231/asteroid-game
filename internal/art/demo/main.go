// Command demo renders the contact sheets of package art. It is the visual
// regression test of the package: run it, then look at the pictures.
//
//	xvfb-run -a -s "-screen 0 3840x2160x24" go run ./internal/art/demo -out <dir> -scale 1
//
// writes one PNG per sheet into <dir>:
//
//	structures               every kind at hex sizes 52, 30, 22 and 14 pt
//	structures_states_small  every kind × running, off, 30 % and 70 % built,
//	                         damaged, derelict, at 14 and 22 pt
//	structures_states_30     the same states at 30 pt
//	structures_states_a, _b  the same states at 44 pt
//	structures_motion        every kind at thirteen moments of its animation
//	structures_bare          every kind on a flat field, for silhouette checks
//	structure_icons          BuildingIcon at 60, 48 and 36 pt, and BuildingTop
//	colony_zooms             a grown colony at hex sizes 14 and 22 pt, with
//	                         and without the deposit overlay
//	colony_close             the same colony at 30 pt
//	colony_close_deposits    ... fully surveyed, with the deposit overlay
//	colony_outlines          the colony under the tile outlines and picking
//	                         marks the interface draws, at three zooms
//	surface_classes          one rock of each class, surveyed around a landing
//	                         and all over, the latter with the deposit overlay
//	surface_extreme          a close-up at 110 pt, past the largest sprites
//	ships                    every kind in the four faction accents, at 48, 24,
//	                         14 and 10 pt and many headings, with its icon
//	goods_icons              goods and interface glyphs at 12, 16, 20, 24 pt
//	                         and larger
//	misc                     rock badges, Emberfall, an explosion strip
//
// Run it at -scale 1 and -scale 2. With -only <name> a single sheet is
// rendered. With -bench N the program instead draws a busy colony (a full
// rock surface and 150 running structures; -bench-buildings changes the
// number) for N frames and prints what a frame costs.
package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/ai"
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
	seed   = flag.Uint64("seed", 42, "world seed for the rocks shown")
	cycles = flag.Int("cycles", 70, "cycles the colony shown has been growing for")
	benchN = flag.Int("bench-buildings", 150, "number of structures in the -bench scene")
)

// framesPerSheet is how long each sheet stays up; the screenshot is taken on
// its sixth frame, when sprite and surface caches have settled.
const framesPerSheet = 8

type sheet struct {
	name string
	draw func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32)
}

var sheets = []sheet{
	{"structures", (*demo).gallery},
	{"structures_states_small", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.states(cv, r, t, []float32{14, 22}, 0, 25) }},
	{"structures_states_30", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.states(cv, r, t, []float32{30}, 0, 25) }},
	{"structures_states_a", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.states(cv, r, t, []float32{44}, 0, 13) }},
	{"structures_states_b", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.states(cv, r, t, []float32{44}, 13, 25) }},
	{"structures_motion", (*demo).motion},
	{"structures_bare", (*demo).bare},
	{"structure_icons", (*demo).icons},
	{"colony_zooms", (*demo).colonyZooms},
	{"colony_close", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.colonyClose(cv, r, t, false) }},
	{"colony_close_deposits", func(d *demo, cv *gfx.Canvas, r gfx.Rect, t float32) { d.colonyClose(cv, r, t, true) }},
	{"colony_outlines", (*demo).colonyOutlines},
	{"surface_classes", (*demo).surfaceClasses},
	{"surface_extreme", (*demo).surfaceExtreme},
	{"ships", (*demo).ships},
	{"goods_icons", (*demo).goodsIcons},
	{"misc", (*demo).misc},
}

func main() {
	o := uitest.Flags()
	o.Title = "Drifthold art contact sheets"
	d := &demo{th: gfx.Default(), sky: gfx.NewBackdrop(0xA57), surf: art.NewSurface()}
	if err := d.prepare(); err != nil {
		log.Fatal(err)
	}

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
		fmt.Printf("bench: %d structures on a %d-tile rock at hex size %.0f pt, scale %.2f, tilt %.2f\n",
			d.benchBuildings, d.benchTiles, benchHex, d.in.Scale, art.Tilt)
		fmt.Printf("bench: draw callback %.3f ms/frame (surface %.3f ms, structures %.3f ms) over %d frames; first frame %.1f ms; %d sprites cached\n",
			ms(d.benchTotal)/float64(d.benchFrames), ms(d.benchSurface)/float64(d.benchFrames),
			ms(d.benchBuild)/float64(d.benchFrames), d.benchFrames, ms(d.benchFirst), art.CachedSprites())
		fmt.Printf("bench: whole frame, backdrop and presentation included, %.2f ms on this machine's renderer\n",
			ms(d.benchSpan)/float64(d.benchFrames))
		fmt.Printf("bench: painting the static layer of the rock afresh (once per zoom level): %.1f ms at hex size %.0f, %.1f ms at hex size 30\n",
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
	explored *model.Rock // the home rock, fully surveyed
	classes  [model.NumRockClasses]*model.Rock
	partial  [model.NumRockClasses]*model.Rock // the same, surveyed around one landing
	surveyed [model.NumRockClasses]*model.Rock // the same, surveyed all over

	benchTotal, benchSurface, benchBuild, benchFirst time.Duration
	benchSpan                                        time.Duration
	benchPaint                                       [2]time.Duration
	benchLast                                        time.Time
	benchFrames, benchBuildings, benchTiles          int
}

func (d *demo) update(in input.State) { d.in = in }

// prepare grows a colony the way tools/demosave does — a rival brain plays
// the human Hold for a while — and then roughs it up a little so that every
// state a structure can be in appears on the rock.
func (d *demo) prepare() error {
	st := model.DefaultSettings(*seed)
	w, err := sim.NewGame(st)
	if err != nil {
		return err
	}
	h := w.Human()
	h.Personality, h.IsAI = model.PersonalityMeridian, true
	s := sim.New(w)
	s.AI = ai.Step
	s.Run(*cycles * model.TicksPerCycle)
	d.world = w
	d.home = w.Rock(h.HomeRock)
	rock := d.home

	// Damage, switch off and abandon a few structures, and start some new
	// ones.
	for i, b := range rock.Buildings {
		if b.Kind == model.Keystone || !b.Completed() {
			continue
		}
		switch i % 9 {
		case 3:
			b.HP = b.MaxHP * 0.5
		case 5:
			b.Enabled, b.Eff = false, 0
		case 7:
			b.HP = b.MaxHP * 0.2
		}
	}
	extra := []model.BuildingKind{model.Laboratory, model.FlakBattery, model.Greenhouse, model.LanceTower, model.Silo, model.MassDriver}
	n := 0
	for _, hx := range rock.Hexes() {
		if n >= len(extra) {
			break
		}
		t := rock.TileAt(hx)
		if !t.Buildable() || !rock.AdjacentToOwned(hx, h.ID) || (hx.Q*7+hx.R*13)%3 != 0 {
			continue
		}
		b := model.NewBuilding(w.NewBuildingID(), extra[n], h.ID, hx, 1)
		b.Progress = []float64{0.15, 0.4, 0.65, 0.85, 0.5, 0.3}[n]
		if n == 4 {
			b.Complete()
			b.HP = b.MaxHP * 0.6
			b.MakeDerelict()
		}
		if rock.AddBuilding(b) == nil {
			n++
		}
	}
	nr := 0
	for _, hx := range rock.Hexes() {
		t := rock.TileAt(hx)
		if t.Buildable() && (hx.Q*5+hx.R*3)%11 == 1 && nr < 5 {
			t.Rubble = true
			nr++
		}
	}

	// A fully surveyed twin of the home rock for the deposit overlay.
	cp := *rock
	cp.Tiles = append([]model.Tile(nil), rock.Tiles...)
	cp.ID = 9001 // a rock in its own right as far as caches are concerned
	cp.RevealAll(model.PlayerFaction)
	d.explored = &cp

	for _, r := range w.Rocks {
		if c := int(r.Class); c >= 0 && c < len(d.classes) && d.classes[c] == nil && r != rock {
			d.classes[c] = r
		}
	}
	if d.classes[rock.Class] == nil {
		d.classes[rock.Class] = rock
	}
	for i, r := range d.classes {
		if r == nil {
			continue
		}
		// What a newcomer sees: the ground around a landing surveyed, the
		// rest still under the veil.
		part := *r
		part.Tiles = append([]model.Tile(nil), r.Tiles...)
		part.Buildings = nil
		part.ID = model.RockID(9200 + i)
		part.SurveyedBy.Clear(model.PlayerFaction)
		for j := range part.Tiles {
			part.Tiles[j].Known.Clear(model.PlayerFaction)
		}
		hexes := part.BlobHexes()
		part.RevealWithin(hexes[len(hexes)*2/5], 3, model.PlayerFaction)
		d.partial[i] = &part
		all := part
		all.Tiles = append([]model.Tile(nil), r.Tiles...)
		all.ID = model.RockID(9300 + i)
		all.RevealAll(model.PlayerFaction)
		d.surveyed[i] = &all
	}
	return nil
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
	cv.TextIn(head, fmt.Sprintf("scale %.2f   tilt %.2f   %d sprites cached", in.Scale, art.Tilt, art.CachedSprites()),
		d.th.NumSmall().Aligned(gfx.AlignRight).WithColor(d.th.TextDim))
	cv.FillRect(gfx.R(screen.X, head.Bottom(), screen.W, 1), d.th.RuleDim)
	screen.CutTop(8)
	sheets[i].draw(d, cv, screen, t)
}

func (d *demo) caption(cv *gfx.Canvas, s string, x, y float32) {
	cv.Text(s, x, y, d.th.Small().WithColor(d.th.TextDim))
}

// ground paints one rock-coloured tile with its ground centre at (x, y).
func (d *demo) ground(cv *gfx.Canvas, x, y, size float32, class int) {
	base := d.th.RockColor(class)
	art.TileFill(cv, x, y, size*1.04, base)
	art.TileStroke(cv, x, y, size*1.03, 1, gfx.WithAlpha(gfx.Darken(base, 0.5), 0.5))
}

// reach returns how far a structure drawn at the given hex size extends
// above and below the ground centre of its tile.
func reach(size float32) (above, below float32) {
	return size * 1.66, size * (art.Tilt + 0.12)
}

// ---------------------------------------------------------------------------
// Structures
// ---------------------------------------------------------------------------

func (d *demo) gallery(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	const cols, rows = 7, 4
	for i, kind := range model.AllBuildingKinds() {
		cell := r.Cell(i%cols, i/cols, cols, rows, 6)
		cv.Panel(cell, 5, gfx.CornersDiag, gfx.WithAlpha(th.Panel, 0.85), th.Border)
		label := cell.CutBottom(20)
		cv.TextIn(label, kind.String(), th.Small().Aligned(gfx.AlignCenter))
		class := i % 4
		look := art.BuildingLook{Accent: th.FactionColor(1 + (i/cols)%4), Progress: 1, Time: t}
		put := func(x, y, size float32) {
			d.ground(cv, x, y, size, class)
			art.Building(cv, kind, x, y, size, look)
		}
		up, down := reach(52)
		put(cell.X+64, cell.Y+(cell.H+up-down)/2, 52)
		up30, down30 := reach(30)
		top := cell.Y + (cell.H-(up30+down30)-(up30+down30)*22/30-10)/2
		put(cell.X+162, top+up30, 30)
		put(cell.X+204, top+up30+down30+10+up30*22/30, 14)
		put(cell.X+156, top+up30+down30+10+up30*22/30, 22)
	}
}

type lookState struct {
	name string
	look art.BuildingLook
}

func (d *demo) lookStates(accent gfx.Color, t float32) []lookState {
	return []lookState{
		{"running", art.BuildingLook{Accent: accent, Progress: 1, Time: t}},
		{"off", art.BuildingLook{Accent: accent, Progress: 1, Off: true, Time: t}},
		{"built 30 %", art.BuildingLook{Accent: accent, Progress: 0.3, Time: t}},
		{"built 70 %", art.BuildingLook{Accent: accent, Progress: 0.7, Time: t}},
		{"damaged", art.BuildingLook{Accent: accent, Progress: 1, Damage: 0.75, Time: t}},
		{"derelict", art.BuildingLook{Accent: d.th.FactionColor(0), Progress: 1, Derelict: true, Time: t}},
	}
}

// states draws the kinds from..to in every state, one block of rows per
// size.
func (d *demo) states(cv *gfx.Canvas, r gfx.Rect, t float32, sizes []float32, from, to int) {
	th := d.th
	kinds := model.AllBuildingKinds()
	cw := (r.W - 80) / float32(to-from)
	head := r.CutTop(16)
	if cw > 70 {
		for i := from; i < to; i++ {
			cv.TextIn(gfx.R(r.X+80+float32(i-from)*cw, head.Y, cw, head.H), kinds[i].String(), th.Small().Aligned(gfx.AlignCenter))
		}
	}
	y := r.Y
	for bi, size := range sizes {
		states := d.lookStates(th.FactionColor(1+(bi+from/13)%4), t)
		up, down := reach(size)
		ch := up + down + 4
		if len(sizes) == 1 {
			ch = r.H / float32(len(states))
		}
		d.caption(cv, fmt.Sprintf("hex size %.0f pt", size), r.X, y-2)
		for si, st := range states {
			gy := y + (ch+up-down)/2
			cv.TextIn(gfx.R(r.X, gy-8, 76, 16), st.name, th.Small().WithColor(th.TextDim))
			for i := from; i < to; i++ {
				gx := r.X + 80 + (float32(i-from)+0.5)*cw
				d.ground(cv, gx, gy, size, (i+si)%4)
				art.Building(cv, kinds[i], gx, gy, size, st.look)
			}
			y += ch
		}
		y += 14
	}
}

// motion shows every kind at a run of moments, to check what moves: one row
// per kind, time running to the right.
func (d *demo) motion(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	kinds := model.AllBuildingKinds()
	const size, frames, step = 19, 13, 0.21
	rows := (len(kinds) + 1) / 2
	rowH := r.H / float32(rows)
	up, down := reach(size)
	for i, kind := range kinds {
		col := gfx.R(r.X+float32(i/rows)*r.W/2, r.Y+float32(i%rows)*rowH, r.W/2-8, rowH)
		cv.TextIn(col.CutLeft(104), kind.String(), th.Small().WithColor(th.TextDim))
		cw := col.W / frames
		for f := 0; f < frames; f++ {
			x := col.X + (float32(f)+0.5)*cw
			y := col.Y + (col.H+up-down)/2
			d.ground(cv, x, y, size, i%4)
			look := art.BuildingLook{Accent: th.FactionColor(1 + i%4), Progress: 1, Time: float32(f) * step}
			art.Building(cv, kind, x, y, size, look)
		}
	}
}

// bare draws every kind on a flat field with nothing around it, at the
// sizes the game uses most. Thresholding this sheet against its background
// gives the silhouettes, which is how to check that each kind can be told
// from the others by outline alone.
func (d *demo) bare(cv *gfx.Canvas, r gfx.Rect, t float32) {
	cv.FillRect(gfx.R(0, 0, d.in.W, d.in.H), gfx.RGB(0x202020))
	kinds := model.AllBuildingKinds()
	y := r.Y + 6
	cw := r.W / float32(len(kinds))
	for _, size := range []float32{14, 18, 22, 30, 44} {
		up, down := reach(size)
		for i, kind := range kinds {
			x := r.X + (float32(i)+0.5)*cw
			art.Building(cv, kind, x, y+up, size, art.BuildingLook{Accent: d.th.FactionColor(1), Progress: 1, Off: true})
		}
		y += up + down + 10
	}
	for i, kind := range kinds {
		cv.TextIn(gfx.R(r.X+float32(i)*cw, y, cw, 16), kind.String(), d.th.Small().Sized(9).Aligned(gfx.AlignCenter))
	}
}

// icons shows BuildingIcon the way the build palette and the codex use it.
func (d *demo) icons(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	kinds := model.AllBuildingKinds()
	const cols = 5
	rows := (len(kinds) + cols - 1) / cols
	for i, kind := range kinds {
		cell := r.Cell(i%cols, i/cols, cols, rows, 8)
		cv.Panel(cell, 5, gfx.CornersDiag, th.Panel, th.Border)
		in := cell.Inset(8)
		accent := th.FactionColor(1 + i%4)
		x := in.X
		for _, s := range []float32{60, 48, 36} {
			box := gfx.R(x, in.Y+(60-s)/2, s, s)
			cv.FillRect(box, th.Well)
			cv.StrokeRect(box, 1, th.Border)
			art.BuildingIcon(cv, kind, box, accent)
			x += s + 10
		}
		// As a row of the build palette.
		row := gfx.R(in.X, in.Bottom()-44, in.W, 44)
		cv.Panel(row, 4, gfx.CornersDiag, th.PanelHi, th.Border)
		art.BuildingIcon(cv, kind, gfx.R(row.X+4, row.Y+4, 36, 36), accent)
		cv.TextIn(row.Pad(48, 0, 4, 18), kind.String(), th.Body())
		cv.TextIn(row.Pad(48, 20, 4, 0), fmt.Sprintf("top %.2f", art.BuildingTop(kind, 1)), th.Small().WithColor(th.TextDim))
	}
}

// ---------------------------------------------------------------------------
// Surfaces and the colony
// ---------------------------------------------------------------------------

// rockBox returns the bounds of the rock's tiles on screen, relative to the
// position of its centre hex.
func rockBox(rock *model.Rock, hex float32) gfx.Rect {
	var box gfx.Rect
	first := true
	for _, h := range rock.BlobHexes() {
		p := art.HexPos(gfx.Pt{}, h, hex)
		t := gfx.R(p.X-hex*0.87, p.Y-hex*art.Tilt, hex*1.74, 2*hex*art.Tilt)
		if first {
			box, first = t, false
		} else {
			box = box.Union(t)
		}
	}
	return box
}

// centred returns where to put the rock's centre hex so that the rock sits
// in the middle of r, a little high to leave room for its underside.
func centred(rock *model.Rock, hex float32, r gfx.Rect) gfx.Pt {
	b := rockBox(rock, hex)
	drop := hex * 1.5
	return gfx.P(r.MidX()-b.MidX(), r.MidY()-b.MidY()-drop)
}

// structures draws the rock's buildings the way the game does: back to
// front, each in the state the simulation has it in.
func (d *demo) structures(cv *gfx.Canvas, rock *model.Rock, origin gfx.Pt, hex, t float32, marks bool) {
	th := d.th
	order := append([]*model.Building(nil), rock.Buildings...)
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i].Hex, order[j].Hex
		if a.R != b.R {
			return a.R < b.R
		}
		return a.Q < b.Q
	})
	for _, b := range order {
		p := art.HexPos(origin, b.Hex, hex)
		look := art.BuildingLook{
			Accent: th.FactionColor(int(b.Owner)), Progress: float32(b.Progress), Derelict: b.Derelict(),
			Off:    b.Completed() && !b.Working() && b.Kind != model.Conduit && b.Kind != model.Keystone,
			Damage: float32(1 - b.HPFrac()), Time: t + float32(b.ID)*0.37,
		}
		if b.Constructing() {
			look.Damage = 0
		}
		art.Building(cv, b.Kind, p.X, p.Y, hex, look)
		if marks && look.Off && hex >= 13 {
			// The status mark the game hangs over an idle structure.
			s := min(14, hex*0.62)
			br := gfx.R(p.X+hex*0.28, p.Y-art.BuildingTop(b.Kind, hex)-s*0.4, s, s)
			cv.FillCircle(br.MidX(), br.MidY(), s*0.72, gfx.WithAlpha(th.Well, 0.9))
			art.Icon(cv, art.IconOnOff, br, th.TextDim)
		}
	}
}

func (d *demo) colonyView(cv *gfx.Canvas, r gfx.Rect, rock *model.Rock, hex, t float32, deposits bool, label string) gfx.Pt {
	cv.StrokeChamfer(r, 5, 1, d.th.Border)
	cv.PushClip(r.Inset(1))
	origin := centred(rock, hex, r)
	d.surf.Draw(cv, rock, origin, hex, model.PlayerFaction, deposits)
	d.structures(cv, rock, origin, hex, t, true)
	cv.PopClip()
	d.caption(cv, label, r.X+8, r.Y+6)
	return origin
}

func (d *demo) colonyZooms(cv *gfx.Canvas, r gfx.Rect, t float32) {
	left := r.CutLeft(r.W * 0.38)
	r.CutLeft(8)
	a := left.CutTop(left.H/2 - 4)
	left.CutTop(8)
	d.colonyView(cv, a, d.home, 14, t, false, "hex size 14 · as surveyed")
	d.colonyView(cv, left, d.explored, 14, t, true, "hex size 14 · deposit overlay · fully surveyed")
	b := r.CutTop(r.H/2 - 4)
	r.CutTop(8)
	d.colonyView(cv, b, d.home, 22, t, false, "hex size 22 · as surveyed")
	d.colonyView(cv, r, d.home, 22, t, true, "hex size 22 · deposit overlay · as surveyed")
}

func (d *demo) colonyClose(cv *gfx.Canvas, r gfx.Rect, t float32, deposits bool) {
	rock, label := d.home, "hex size 30 · as surveyed"
	if deposits {
		rock, label = d.explored, "hex size 30 · deposit overlay · fully surveyed"
	}
	d.colonyView(cv, r, rock, 30, t, deposits, label)
}

// colonyOutlines lays the interface's own marks over the surface: the
// outline of every tile (TileStroke), a filled tile here and there
// (TileFill), and a dot at the centre of every tile that is green when
// PickHex finds the tile again from that point. Tiles and outlines must
// coincide at every zoom.
func (d *demo) colonyOutlines(cv *gfx.Canvas, r gfx.Rect, t float32) {
	th := d.th
	cells := []struct {
		hex        float32
		structures bool
	}{{14, false}, {22, false}, {30, false}, {22, true}}
	for i, c := range cells {
		cell := r.Cell(i%2, i/2, 2, 2, 8)
		rock := d.home
		cv.StrokeChamfer(cell, 5, 1, th.Border)
		cv.PushClip(cell.Inset(1))
		origin := centred(rock, c.hex, cell)
		d.surf.Draw(cv, rock, origin, c.hex, model.PlayerFaction, false)
		for _, h := range rock.BlobHexes() {
			p := art.HexPos(origin, h, c.hex)
			if (h.Q+2*h.R)%5 == 0 {
				art.TileFill(cv, p.X, p.Y, c.hex, gfx.WithAlpha(th.Info, 0.22))
			}
			art.TileStroke(cv, p.X, p.Y, c.hex, 1, gfx.WithAlpha(th.Rule, 0.8))
			col := th.Good
			if art.PickHex(origin, p.X, p.Y, c.hex) != h ||
				art.PickHex(origin, p.X+c.hex*0.5, p.Y+c.hex*0.3*art.Tilt, c.hex) != h ||
				art.PickHex(origin, p.X-c.hex*0.5, p.Y-c.hex*0.3*art.Tilt, c.hex) != h {
				col = th.Alert
			}
			cv.FillCircle(p.X, p.Y, 1.5, col)
		}
		if c.structures {
			d.structures(cv, rock, origin, c.hex, t, true)
		}
		cv.PopClip()
		label := fmt.Sprintf("hex size %.0f · TileStroke on every tile, TileFill on some", c.hex)
		if c.structures {
			label += " · structures on top"
		}
		d.caption(cv, label, cell.X+8, cell.Y+6)
	}
}

func (d *demo) surfaceView(cv *gfx.Canvas, r gfx.Rect, rock *model.Rock, hex float32, deposits bool, label string) {
	cv.StrokeChamfer(r, 5, 1, d.th.Border)
	cv.PushClip(r.Inset(1))
	d.surf.Draw(cv, rock, centred(rock, hex, r), hex, model.PlayerFaction, deposits)
	cv.PopClip()
	d.caption(cv, label, r.X+8, r.Y+6)
}

func (d *demo) surfaceClasses(cv *gfx.Canvas, r gfx.Rect, t float32) {
	for i, rock := range d.classes {
		if rock == nil {
			continue
		}
		cell := r.Cell(i%2, i/2, 2, 2, 8)
		half := cell.CutLeft(cell.W / 2)
		d.surfaceView(cv, half, d.partial[i], 13, false, fmt.Sprintf("%s · %s · surveyed around a landing", rock.Name, rock.Class))
		d.surfaceView(cv, cell, d.surveyed[i], 13, true, "surveyed · deposit overlay")
	}
}

// surfaceExtreme zooms in further than any sprite or surface layer is
// rendered, where the art falls back to stretching.
func (d *demo) surfaceExtreme(cv *gfx.Canvas, r gfx.Rect, t float32) {
	const hex = 110
	cv.StrokeChamfer(r, 5, 1, d.th.Border)
	cv.PushClip(r.Inset(1))
	centre := model.Hex{}
	if k := d.home.Keystone(); k != nil {
		centre = k.Hex
	}
	p := art.HexPos(gfx.Pt{}, centre, hex)
	origin := gfx.P(r.MidX()-p.X, r.MidY()-p.Y+60)
	d.surf.Draw(cv, d.explored, origin, hex, model.PlayerFaction, true)
	d.structures(cv, d.home, origin, hex, t, false)
	cv.PopClip()
	d.caption(cv, "hex size 110 · deposit overlay", r.X+8, r.Y+6)
}

// ---------------------------------------------------------------------------
// Benchmark
// ---------------------------------------------------------------------------

const benchHex float32 = 22

func (d *demo) benchScene(cv *gfx.Canvas, t float32) {
	start := time.Now()
	in := d.in
	d.sky.Draw(cv, gfx.BackdropView{Time: t})
	rock := d.home
	c := centred(rock, benchHex, gfx.R(0, 0, in.W, in.H))
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
			p := art.HexPos(c, h, benchHex)
			// Later passes stack further structures beside the first so
			// that small rocks still carry the full number.
			look := art.BuildingLook{Accent: d.th.FactionColor(1 + n%4), Progress: 1, Time: t + float32(n)*0.37}
			switch {
			case n%19 == 7:
				look.Progress = 0.5
			case n%23 == 5:
				look.Off = true
			case n%17 == 3:
				look.Damage = 0.8
			}
			art.Building(cv, kinds[n%len(kinds)], p.X+float32(pass)*3, p.Y+float32(pass)*2, benchHex, look)
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
		for i, hex := range [2]float32{benchHex, 30} {
			fresh := art.NewSurface()
			t0 := time.Now()
			fresh.Draw(cv, rock, c, hex, model.PlayerFaction, false)
			d.benchPaint[i] = time.Since(t0)
			fresh.Release()
		}
	}
	d.benchLast = time.Now()
	d.benchBuildings, d.benchTiles = n, tiles
	cv.Text(fmt.Sprintf("%d structures · %d sprites cached", n, art.CachedSprites()), 12, 12, d.th.Body())
}
