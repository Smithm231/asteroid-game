package app

import (
	"fmt"
	"strconv"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/emblem"
	"drifthold/internal/gfx"
	"drifthold/internal/model"
	"drifthold/internal/save"
	"drifthold/internal/ui/kit"
)

type titleState struct {
	setup    model.GameSettings
	seedText string
	logo     *ebiten.Image
}

type dialogID int

const (
	dialogNone dialogID = iota
	dialogMenu
	dialogSave
	dialogLoad
	dialogOptions
)

type dialogState struct {
	saves    []save.Entry
	saveName string
	back     dialogID // dialog to return to
	err      string
}

func (a *App) openDialog(d dialogID) {
	a.dlg.back = a.dialog
	if a.dialog == dialogNone {
		a.dlg.back = dialogNone
	}
	a.dialog = d
	a.dlg.err = ""
	if d == dialogSave || d == dialogLoad {
		a.dlg.saves, _ = save.List()
		if d == dialogSave && a.dlg.saveName == "" && a.game != nil {
			a.dlg.saveName = fmt.Sprintf("Cycle %d", a.game.w.Cycle())
		}
	}
	a.ui.OnSound(kit.SoundOpen)
}

func (a *App) closeDialog() {
	a.dialog = a.dlg.back
	a.dlg.back = dialogNone
}

// titleFrame declares the title screen or the new-game setup.
func (a *App) titleFrame() {
	ui, th, in := a.ui, a.th, a.in
	full := gfx.R(0, 0, in.W, in.H)
	if a.title.logo == nil {
		a.title.logo = ebiten.NewImageFromImage(emblem.Render(512))
	}
	logo := a.title.logo
	t := ui.Time
	old := ui.SetLayer(kit.LayerWorld)
	ui.Paint(func(cv *gfx.Canvas) {
		a.backdrop.Draw(cv, gfx.BackdropView{X: t * 6, Y: t * 2, EmberX: full.W * 0.82, EmberY: full.H * 0.9, EmberRadius: 900, Time: t})
	})
	ui.SetLayer(old)

	col := full.Center(420, min(full.H-40, 620))
	if a.screen == screenSetup {
		a.setupFrame(full)
		return
	}
	head := col.CutTop(250)
	ui.Paint(func(cv *gfx.Canvas) {
		cv.DrawImage(logo, gfx.R(head.MidX()-70, head.Y, 140, 140), &gfx.ImageOpts{Smooth: true})
		st := th.Display().Aligned(gfx.AlignCenter).WithColor(th.Rule)
		st.Tracking = 8
		cv.Text("DRIFTHOLD", head.MidX(), head.Y+150, st)
		cv.Text("STAKE YOUR CLAIM IN THE SHOAL OF EMBERFALL", head.MidX(), head.Y+208, th.Label().WithColor(th.TextDim).Aligned(gfx.AlignCenter))
		cv.Text("v"+Version, full.Right()-14, full.Bottom()-24, th.Small().WithColor(th.TextFaint).Aligned(gfx.AlignRight))
	})
	if a.dialog != dialogNone {
		return
	}
	f := &flow{ui: ui, th: th, r: col.Center(300, col.H)}
	saves, _ := save.List()
	if len(saves) > 0 {
		s := saves[0]
		if f.button("continue", "Continue", kit.Btn{Primary: true, Tip: &kit.Tip{Title: s.Name, Rows: []kit.TipRow{
			{K: "Hold", V: s.Hold}, {K: "Cycle", V: fmt.Sprint(s.Cycle)}, {K: "Saved", V: s.SavedAt.Format("2 Jan 2006, 15:04")}}}}) {
			if err := a.loadGame(s.Path); err != nil {
				a.notice = err.Error()
			}
			return
		}
	}
	if f.button("new", "New game", kit.Btn{Primary: len(saves) == 0}) {
		a.screen = screenSetup
		a.title.setup.Seed = uint64(time.Now().UnixNano()) % 1000000
		a.title.seedText = fmt.Sprint(a.title.setup.Seed)
	}
	if f.button("load", "Load game", kit.Btn{Disabled: len(saves) == 0, Why: "There are no saved games yet."}) {
		a.openDialog(dialogLoad)
	}
	if f.button("options", "Options", kit.Btn{}) {
		a.openDialog(dialogOptions)
	}
	if f.button("quit", "Quit", kit.Btn{}) {
		a.quit = true
	}
	if a.notice != "" {
		f.gap(10)
		f.text(a.notice, th.Small().WithColor(th.Alert).Aligned(gfx.AlignCenter))
	}
}

func (a *App) setupFrame(full gfx.Rect) {
	ui, th := a.ui, a.th
	st := &a.title.setup
	box := full.Center(560, 560)
	ui.Block(box)
	ui.Paint(func(cv *gfx.Canvas) {
		cv.Shadow(box.Translate(0, 8), 24, th.Shadow)
		cv.Panel(box, 10, gfx.CornersDiag, th.Panel, th.Rule)
	})
	f := &flow{ui: ui, th: th, r: box.Inset(28)}
	f.ui.Label(f.row(34), "NEW CHARTER", th.Title().WithColor(th.Rule))
	f.gap(8)
	label := func(s string) { f.ui.Label(f.row(20), s, th.Label().WithColor(th.TextDim)) }
	choice := func(id string, names []string, sel int, tips []string) int {
		row := f.row(th.RowH + 2)
		f.gap(10)
		for i, n := range names {
			o := kit.Btn{Selected: sel == i}
			if tips != nil {
				o.Tip = &kit.Tip{Title: n, Body: tips[i]}
			}
			if ui.Button(id+n, row.Col(i, len(names), 6), n, o) {
				sel = i
			}
		}
		return sel
	}
	label("NAME OF YOUR HOLD")
	ui.TextInput("holdname", f.row(th.RowH+2), &st.PlayerName, 32, nil)
	f.gap(10)
	label("SIZE OF THE BELT")
	st.BeltSize = model.BeltSize(choice("size", []string{"Small · 10 rocks", "Medium · 13 rocks", "Large · 16 rocks"}, int(st.BeltSize), nil))
	label("RIVAL HOLDS")
	st.Rivals = 1 + choice("rivals", []string{"One", "Two", "Three"}, st.Rivals-1, nil)
	label("DIFFICULTY")
	st.Difficulty = model.Difficulty(choice("diff", []string{"Easy", "Normal", "Hard"}, int(st.Difficulty), []string{
		"Rivals produce 20 % less and attack later, with smaller fleets.",
		"Rivals play by the same rules as you.",
		"Rivals produce 25 % more, attack sooner and single you out.",
	}))
	label("WORLD SEED")
	row := f.row(th.RowH + 2)
	f.gap(10)
	if ui.Button("reseed", row.CutRight(120), "Randomise", kit.Btn{}) {
		st.Seed = uint64(time.Now().UnixNano()) % 1000000
		a.title.seedText = fmt.Sprint(st.Seed)
	}
	row.CutRight(8)
	if ui.TextInput("seed", row, &a.title.seedText, 9, func(r rune) bool { return r >= '0' && r <= '9' }) {
		st.Seed, _ = strconv.ParseUint(a.title.seedText, 10, 64)
	}
	row = f.row(th.RowH)
	f.gap(14)
	ui.Check("events", row.Col(0, 2, 10), "Random belt events", &st.Events)
	ui.Check("directives", row.Col(1, 2, 10), "Directives (guided goals)", &st.Directives)
	row = f.row(th.RowH + 6)
	if ui.Button("setup/back", row.Col(0, 2, 10), "Back", kit.Btn{Hotkey: ebiten.KeyEscape}) {
		a.screen = screenTitle
	}
	if ui.Button("setup/start", row.Col(1, 2, 10), "Found the Hold", kit.Btn{Primary: true}) {
		if st.PlayerName == "" {
			st.PlayerName = model.DefaultPlayerName
		}
		a.startGame(*st)
	}
}

// dialogFrame declares the modal dialog that is open, if any.
func (a *App) dialogFrame() {
	if a.dialog == dialogNone {
		return
	}
	ui, th, in := a.ui, a.th, a.in
	full := gfx.R(0, 0, in.W, in.H)
	old := ui.SetLayer(kit.LayerWindow)
	ui.Scrim()
	ui.SetLayer(old)
	switch a.dialog {
	case dialogMenu:
		r, closed := ui.Window("dlg/menu", "Drifthold", full.Center(340, 330))
		f := &flow{ui: ui, th: th, r: r}
		if f.button("m/resume", "Resume", kit.Btn{Primary: true}) || closed {
			a.dialog = dialogNone
		}
		if f.button("m/save", "Save game", kit.Btn{Disabled: a.game == nil || a.game.w.GameOver}) {
			a.openDialog(dialogSave)
		}
		if f.button("m/load", "Load game", kit.Btn{}) {
			a.openDialog(dialogLoad)
		}
		if f.button("m/options", "Options", kit.Btn{}) {
			a.openDialog(dialogOptions)
		}
		f.gap(8)
		if f.button("m/title", "Quit to the title screen", kit.Btn{}) {
			a.toTitle()
		}
		if f.button("m/quit", "Quit to the desktop", kit.Btn{}) {
			a.quit = true
		}
		ui.EndWindow()
	case dialogSave:
		r, closed := ui.Window("dlg/save", "Save game", full.Center(520, 460))
		f := &flow{ui: ui, th: th, r: r}
		f.ui.Label(f.row(20), "NAME", th.Label().WithColor(th.TextDim))
		row := f.row(th.RowH + 2)
		f.gap(8)
		doSave := ui.Button("s/ok", row.CutRight(110), "Save", kit.Btn{Primary: true, Hotkey: ebiten.KeyEnter})
		row.CutRight(8)
		ui.TextInput("s/name", row, &a.dlg.saveName, 40, nil)
		if a.dlg.err != "" {
			f.text(a.dlg.err, th.Small().WithColor(th.Alert))
		}
		f.ui.Label(f.row(20), "EXISTING SAVES (CLICK TO OVERWRITE)", th.Label().WithColor(th.TextDim))
		content := ui.BeginScroll("s/list", f.r, float32(len(a.dlg.saves))*30)
		for i, s := range a.dlg.saves {
			if ui.Button(fmt.Sprint("s/e", i), content.CutTop(30).Pad(0, 0, 0, 3), fmt.Sprintf("%s  ·  cycle %d  ·  %s", s.Name, s.Cycle, s.SavedAt.Format("2 Jan 15:04")),
				kit.Btn{Quiet: true, Left: true, Align: gfx.AlignLeft}) {
				a.dlg.saveName = s.Name
			}
		}
		ui.EndScroll()
		ui.EndWindow()
		if doSave {
			if err := a.saveGame(a.dlg.saveName); err != nil {
				a.dlg.err = err.Error()
			} else {
				if a.game != nil {
					a.game.toast(model.SevInfo, "Game saved as “"+save.CleanName(a.dlg.saveName)+"”.")
				}
				a.dialog = dialogNone
			}
		} else if closed {
			a.closeDialog()
		}
	case dialogLoad:
		r, closed := ui.Window("dlg/load", "Load game", full.Center(600, 480))
		if len(a.dlg.saves) == 0 {
			ui.Text(r, "There are no saved games yet.", th.Body().WithColor(th.TextDim))
		}
		if a.dlg.err != "" {
			h := ui.Text(r, a.dlg.err, th.Small().WithColor(th.Alert))
			r.CutTop(h + 6)
		}
		content := ui.BeginScroll("l/list", r, float32(len(a.dlg.saves))*50)
		for i, s := range a.dlg.saves {
			row := content.CutTop(50).Pad(0, 0, 0, 4)
			del := row.CutRight(76)
			row.CutRight(6)
			if ui.Button(fmt.Sprint("l/e", i), row, "", kit.Btn{}) {
				if err := a.loadGame(s.Path); err != nil {
					a.dlg.err = err.Error()
				}
				ui.EndScroll()
				ui.EndWindow()
				return
			}
			s := s
			ui.Paint(func(cv *gfx.Canvas) {
				cv.TextFit(gfx.R(row.X+12, row.Y+4, row.W-180, 20), s.Name, th.Body().Weighted(gfx.Medium))
				cv.TextFit(gfx.R(row.X+12, row.Y+24, row.W-24, 18),
					fmt.Sprintf("%s  ·  cycle %d  ·  %d colonies, %d colonists  ·  %d rivals, %s", s.Hold, s.Cycle, s.Colonies, s.Pop, s.Rivals, s.Difficulty),
					th.Small().WithColor(th.TextDim))
				cv.TextFit(gfx.R(row.Right()-170, row.Y+4, 160, 20), s.SavedAt.Format("2 Jan 2006, 15:04"), th.Small().WithColor(th.TextDim).Aligned(gfx.AlignRight))
			})
			if ui.Button(fmt.Sprint("l/d", i), del.Inset(6), "Delete", kit.Btn{Quiet: true}) {
				save.Delete(s.Path)
				a.dlg.saves, _ = save.List()
				break
			}
		}
		ui.EndScroll()
		ui.EndWindow()
		if closed {
			a.closeDialog()
		}
	case dialogOptions:
		r, closed := ui.Window("dlg/options", "Options", full.Center(520, 420))
		f := &flow{ui: ui, th: th, r: r}
		s := &a.settings
		slider := func(name string, v *float64) {
			row := f.row(th.RowH)
			f.gap(6)
			ui.Label(row.CutLeft(150), name, th.Body())
			ui.Label(row.CutRight(50), fmt.Sprintf("%.0f %%", *v*100), th.Num().Aligned(gfx.AlignRight))
			if ui.Slider("opt/"+name, row, v, 0, 1) {
				a.applyVolume()
			}
		}
		f.header("Sound")
		slider("Overall volume", &s.MasterVolume)
		slider("Effects", &s.SfxVolume)
		slider("Music", &s.MusicVolume)
		f.gap(6)
		f.header("Display")
		row := f.row(th.RowH)
		f.gap(8)
		ui.Label(row.CutLeft(150), "Interface size", th.Body())
		scales := []float64{0, 0.75, 1, 1.25, 1.5, 2}
		for i, sc := range scales {
			name := "Auto"
			if sc > 0 {
				name = fmt.Sprintf("%.0f %%", sc*100)
			}
			if ui.Button("opt/scale"+name, row.Col(i, len(scales), 4), name, kit.Btn{Selected: s.UIScale == sc}) {
				s.UIScale = sc
			}
		}
		row = f.row(th.RowH)
		f.gap(8)
		if ui.Check("opt/full", row, "Full screen (F11)", &a.fullscr) {
			s.Fullscreen = a.fullscr
		}
		f.header("Game")
		row = f.row(th.RowH)
		f.gap(12)
		ui.Label(row.CutLeft(150), "Autosave", th.Body())
		mins := []int{0, 2, 5, 10}
		for i, m := range mins {
			name := "Off"
			if m > 0 {
				name = fmt.Sprintf("%d min", m)
			}
			if ui.Button("opt/auto"+name, row.Col(i, len(mins), 4), name, kit.Btn{Selected: s.AutosaveMinutes == m}) {
				s.AutosaveMinutes = m
			}
		}
		done := f.button("opt/done", "Done", kit.Btn{Primary: true})
		ui.EndWindow()
		if done || closed {
			save.SaveSettings(a.settings)
			a.closeDialog()
		}
	}
}
