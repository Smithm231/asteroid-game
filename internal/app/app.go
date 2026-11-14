// Package app is the Drifthold application: it owns the window contents, the
// running game session, the interface and the sound, and ties them to the
// simulation.
package app

import (
	"fmt"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/ai"
	"drifthold/internal/art"
	"drifthold/internal/audio"
	"drifthold/internal/gfx"
	"drifthold/internal/input"
	"drifthold/internal/model"
	"drifthold/internal/save"
	"drifthold/internal/sim"
	"drifthold/internal/ui/kit"
)

// Version is the game version shown on the title screen.
const Version = "1.0.0"

// Options configure the application at start-up.
type Options struct {
	// Mute disables all sound output (also required on machines without a
	// sound device).
	Mute bool
	// NewGame starts a game straight away with these settings instead of
	// showing the title screen.
	NewGame *model.GameSettings
	// Load starts by loading this save file.
	Load string
	// Portable keeps saves and settings in this directory instead of the
	// user's settings folder.
	Portable string
	// FixedStep makes every frame advance the clock by exactly 1/60 s, for
	// reproducible scripted runs.
	FixedStep bool
}

type screenID int

const (
	screenTitle screenID = iota
	screenSetup
	screenGame
)

// App is the whole application. Feed it one input snapshot per frame with
// Frame and let it paint with Draw.
type App struct {
	opts     Options
	th       *gfx.Theme
	ui       *kit.Ctx
	snd      *audio.Engine
	settings save.Settings
	backdrop *gfx.Backdrop

	screen screenID
	game   *Game
	title  titleState
	// dialog is the modal dialog open on top of the current screen.
	dialog dialogID
	dlg    dialogState

	in       input.State
	quit     bool
	fullscr  bool
	notice   string // one-line message on the title screen (errors)
	lastSave time.Time
}

// New creates the application.
func New(o Options) *App {
	if o.Portable != "" {
		save.SetRoot(o.Portable)
	}
	a := &App{opts: o, th: gfx.Default(), backdrop: gfx.NewBackdrop(0x5eed)}
	a.settings = save.LoadSettings()
	a.ui = kit.New(a.th)
	if o.Mute {
		a.snd = audio.NewSilent()
	} else {
		a.snd = audio.New()
	}
	a.applyVolume()
	a.snd.SetMood(audio.Menu)
	a.ui.OnSound = func(s kit.Sound) {
		switch s {
		case kit.SoundHover:
			a.snd.PlayAt(audio.UIHover, 0.5, 0)
		case kit.SoundClick:
			a.snd.Play(audio.UIClick)
		case kit.SoundOpen:
			a.snd.Play(audio.UIOpen)
		case kit.SoundClose:
			a.snd.Play(audio.UIClose)
		case kit.SoundError:
			a.snd.Play(audio.UIError)
		}
	}
	a.fullscr = a.settings.Fullscreen
	a.title.setup = a.settings.LastSetup
	a.title.seedText = fmt.Sprint(a.title.setup.Seed)
	switch {
	case o.Load != "":
		if err := a.loadGame(o.Load); err != nil {
			a.notice = err.Error()
		}
	case o.NewGame != nil:
		a.startGame(*o.NewGame)
	}
	return a
}

func (a *App) applyVolume() {
	a.snd.SetVolume(float32(a.settings.MasterVolume), float32(a.settings.SfxVolume), float32(a.settings.MusicVolume))
}

// Quit reports whether the player asked to leave the game.
func (a *App) Quit() bool { return a.quit }

// Fullscreen reports whether the player wants the window to be full screen.
func (a *App) Fullscreen() bool { return a.fullscr }

// UIScale returns the player's interface scale factor (0 = automatic).
func (a *App) UIScale() float64 { return a.settings.UIScale }

// startGame generates a world and enters it.
func (a *App) startGame(st model.GameSettings) {
	w, err := sim.NewGame(st)
	if err != nil {
		a.notice = "Could not create the world: " + err.Error()
		return
	}
	a.settings.LastSetup = st.Normalized()
	save.SaveSettings(a.settings)
	a.enter(w)
}

// enter makes w the running game.
func (a *App) enter(w *model.World) {
	a.game = newGame(a, w)
	a.screen = screenGame
	a.dialog = dialogNone
	a.notice = ""
	a.lastSave = time.Now()
	a.snd.SetMood(audio.Calm)
}

func (a *App) loadGame(path string) error {
	w, _, err := save.Read(path)
	if err != nil {
		return err
	}
	a.enter(w)
	return nil
}

func (a *App) saveGame(name string) error {
	if a.game == nil {
		return nil
	}
	_, err := save.Write(name, a.game.w)
	if err == nil {
		a.lastSave = time.Now()
	}
	return err
}

// toTitle leaves the running game.
func (a *App) toTitle() {
	if a.game != nil {
		a.game.surface.Release()
	}
	a.game = nil
	a.screen = screenTitle
	a.dialog = dialogNone
	a.snd.SetMood(audio.Menu)
}

// Frame advances the application by one frame using the given input.
func (a *App) Frame(in input.State) {
	if a.opts.FixedStep {
		in.Dt = 1.0 / 60
	}
	a.in = in
	a.snd.Update()
	a.ui.Begin(in)
	if !a.ui.WantKeys() {
		if in.KeyPressed(ebiten.KeyF11) || (in.Alt && in.KeyPressed(ebiten.KeyEnter)) {
			a.fullscr = !a.fullscr
			a.settings.Fullscreen = a.fullscr
			save.SaveSettings(a.settings)
		}
	}
	switch a.screen {
	case screenTitle, screenSetup:
		a.titleFrame()
	case screenGame:
		a.game.frame()
	}
	a.dialogFrame()
	a.ui.End()

	// Autosave.
	if a.game != nil && a.settings.AutosaveMinutes > 0 && !a.game.w.GameOver &&
		time.Since(a.lastSave) > time.Duration(a.settings.AutosaveMinutes)*time.Minute {
		if err := a.saveGame(save.AutosaveName); err != nil {
			a.game.toast(model.SevWarn, "Autosave failed: "+err.Error())
		}
		a.lastSave = time.Now()
	}
}

// Draw paints the current frame.
func (a *App) Draw(dst *ebiten.Image) {
	dst.Fill(a.th.Space)
	a.ui.Draw(dst)
}

// ------------------------------------------------------------------ session

type viewMode int

const (
	viewSurface viewMode = iota
	viewBelt
)

type selKind int

const (
	selNone selKind = iota
	selBuilding
	selTile
	selFleet
	selRock
)

type selection struct {
	kind     selKind
	rock     model.RockID
	building model.BuildingID
	hex      model.Hex
	fleet    model.FleetID
}

type toast struct {
	text string
	sev  model.Severity
	rock model.RockID
	age  float32
}

// effect is a short-lived visual cue in one of the two views.
type effect struct {
	kind  model.FXKind
	rock  model.RockID
	hex   model.Hex
	a, b  model.Vec2
	age   float32
	life  float32
	angle float32
	col   gfx.Color
}

// Game is a running game session: the simulation plus everything the player
// has selected and opened.
type Game struct {
	a   *App
	sim *sim.Sim
	w   *model.World
	me  model.FactionID

	speed  int // 0 paused, 1, 2 or 4
	acc    float64
	clock  float32 // seconds of real time, for animation
	view   viewMode
	rock   model.RockID // rock shown in the surface view
	sel    selection
	place  model.BuildingKind
	tab    int // build palette tab
	window windowID
	win    windowState

	surface *art.Surface
	scam    surfaceCam
	bcam    beltCam
	deposit bool // deposit overlay

	toasts  []toast
	logSeen int
	fx      []effect
	orders  orderPick // fleet order waiting for a target rock
	menu    *ctxMenu  // belt right-click menu
	include [model.NumShipKinds]bool
	incFor  model.FleetID
	battle  float32 // seconds since combat last touched the player
	tense   float32
	logOpen bool
	ended   bool
}

func newGame(a *App, w *model.World) *Game {
	g := &Game{a: a, w: w, me: model.PlayerFaction, speed: 1, place: model.BuildingNone,
		surface: art.NewSurface(), battle: 99, tense: 99}
	g.sim = sim.New(w)
	g.sim.AI = ai.Step
	if h := w.Human(); h != nil {
		g.rock = h.HomeRock
		if w.Rock(g.rock) == nil || w.Rock(g.rock).Owner != g.me {
			if rocks := w.RocksOf(g.me); len(rocks) > 0 {
				g.rock = rocks[0].ID
			}
		}
	}
	g.logSeen = w.LogSeq
	g.scam.hex = 22
	g.centerOnKeystone()
	g.bcam.zoom = 0
	return g
}

func (g *Game) faction() *model.Faction { return g.w.Faction(g.me) }

// cur returns the rock shown in the surface view.
func (g *Game) cur() *model.Rock { return g.w.Rock(g.rock) }

// issue sends a command from the player. It is executed at once so that the
// interface can report a refusal directly.
func (g *Game) issue(c model.Command) bool {
	if g.w.GameOver {
		return false
	}
	if err := sim.Exec(g.w, c); err != nil {
		g.toast(model.SevWarn, err.Error()+".")
		g.a.snd.Play(audio.UIError)
		return false
	}
	return true
}

func (g *Game) toast(sev model.Severity, text string) {
	g.toasts = append(g.toasts, toast{text: text, sev: sev, rock: model.NoRock})
	if len(g.toasts) > 6 {
		g.toasts = g.toasts[len(g.toasts)-6:]
	}
}

// gotoRock shows a rock in the surface view.
func (g *Game) gotoRock(id model.RockID) {
	if g.w.Rock(id) == nil {
		return
	}
	if id != g.rock {
		g.rock = id
		g.sel = selection{}
		g.place = model.BuildingNone
		g.centerOnKeystone()
	}
	g.view = viewSurface
}

func (g *Game) centerOnKeystone() {
	g.scam.off = gfx.Pt{}
	if r := g.cur(); r != nil {
		if ks := r.Keystone(); ks != nil {
			p := model.HexToPixel(ks.Hex, float64(g.scam.hex))
			g.scam.off = gfx.P(-float32(p.X), -float32(p.Y))
		}
	}
}

// cycleRock moves the surface view to the player's next or previous colony.
func (g *Game) cycleRock(dir int) {
	rocks := g.w.RocksOf(g.me)
	if len(rocks) == 0 {
		return
	}
	idx := 0
	for i, r := range rocks {
		if r.ID == g.rock {
			idx = i
		}
	}
	g.gotoRock(rocks[(idx+dir+len(rocks))%len(rocks)].ID)
}

// frame runs one frame of the game screen.
func (g *Game) frame() {
	in := g.a.in
	ui := g.a.ui
	g.clock += in.Dt

	g.hotkeys()
	g.advance(float64(in.Dt))
	g.collectLog()
	g.music(in.Dt)

	full := gfx.R(0, 0, in.W, in.H)
	main := g.layout(full) // declares HUD panels, returns the world area
	old := ui.SetLayer(kit.LayerWorld)
	if g.view == viewSurface {
		g.surfaceFrame(main)
	} else {
		g.beltFrame(main)
	}
	ui.SetLayer(old)
	g.windowFrame(full)
	g.overlayFrame(full)

	for i := 0; i < len(g.fx); {
		g.fx[i].age += in.Dt
		if g.fx[i].age >= g.fx[i].life {
			g.fx = append(g.fx[:i], g.fx[i+1:]...)
			continue
		}
		i++
	}
}

// advance steps the simulation according to the game speed.
func (g *Game) advance(dt float64) {
	if g.w.GameOver {
		if !g.ended {
			g.ended = true
			g.window = windowNone
			if g.w.Winner == g.me {
				g.a.snd.Play(audio.Victory)
				g.a.snd.SetMood(audio.Triumph)
			} else {
				g.a.snd.Play(audio.Defeat)
				g.a.snd.SetMood(audio.Lament)
			}
		}
		return
	}
	if g.speed == 0 || g.a.dialog != dialogNone {
		return
	}
	g.acc += dt * float64(g.speed) * model.TicksPerSecond
	steps := int(g.acc)
	if steps > 12 {
		steps = 12 // never spiral after a long frame
		g.acc = 0
	} else {
		g.acc -= float64(steps)
	}
	for i := 0; i < steps && !g.w.GameOver; i++ {
		g.sim.Step()
		for _, fx := range g.w.TakeFX() {
			g.onFX(fx)
		}
	}
}

// collectLog turns new log entries addressed to the player into toasts.
func (g *Game) collectLog() {
	if g.w.LogSeq == g.logSeen {
		return
	}
	for _, e := range g.w.Log {
		if e.Seq <= g.logSeen || !e.VisibleTo(g.me) {
			continue
		}
		g.toasts = append(g.toasts, toast{text: e.Text, sev: e.Sev, rock: e.Rock})
		switch e.Sev {
		case model.SevAlert:
			g.a.snd.Play(audio.Alert)
		case model.SevWarn:
			g.a.snd.PlayAt(audio.Notice, 0.8, 0)
		}
	}
	if len(g.toasts) > 6 {
		g.toasts = g.toasts[len(g.toasts)-6:]
	}
	g.logSeen = g.w.LogSeq
}

// music picks the music mood from what is happening to the player.
func (g *Game) music(dt float32) {
	if g.w.GameOver {
		return
	}
	g.battle += dt
	g.tense += dt
	for _, fl := range g.w.Fleets {
		r := g.w.Rock(fl.At)
		switch {
		case fl.Engaged && (fl.Owner == g.me || (r != nil && r.Owner == g.me)):
			g.battle = 0
		case fl.Owner != g.me && fl.InTransit() && fl.Order == model.OrderAttack:
			if d := g.w.Rock(fl.Dest); d != nil && d.Owner == g.me && sim.Visible(g.w, g.me, fl) {
				g.tense = 0
			}
		case fl.Owner == g.me && fl.Order == model.OrderAttack:
			g.tense = 0
		}
	}
	switch {
	case g.battle < 8:
		g.a.snd.SetMood(audio.Battle)
	case g.tense < 6:
		g.a.snd.SetMood(audio.Tense)
	default:
		g.a.snd.SetMood(audio.Calm)
	}
}

// onFX reacts to a cue from the simulation with sound and a visual effect.
func (g *Game) onFX(fx model.FX) {
	mine := fx.Faction == g.me
	here := fx.Rock == g.rock && g.view == viewSurface
	r := g.w.Rock(fx.Rock)
	onMine := r != nil && r.Owner == g.me
	vol := float32(0.35)
	if here {
		vol = 1
	} else if mine || onMine {
		vol = 0.6
	}
	play := func(s audio.Sfx, v float32) {
		if v > 0 {
			g.a.snd.PlayAt(s, v, 0)
		}
	}
	add := func(life float32) {
		g.fx = append(g.fx, effect{kind: fx.Kind, rock: fx.Rock, hex: fx.Hex, a: fx.A, b: fx.B, life: life,
			angle: float32(math.Mod(float64(len(g.fx))*2.399+float64(fx.Hex.Q*7+fx.Hex.R*13), 6.283)),
			col:   g.a.th.FactionColor(int(fx.Faction))})
	}
	relevant := here || mine || onMine
	switch fx.Kind {
	case model.FXBuildPlaced:
		if mine {
			play(audio.Place, 1)
		}
	case model.FXBuildDone:
		if mine {
			play(audio.BuildDone, vol)
			add(0.8)
		}
	case model.FXBuildDestroyed:
		if relevant {
			play(audio.ExplosionLarge, vol)
		}
		add(1.0)
	case model.FXShipBuilt:
		if mine {
			play(audio.Good, vol)
		}
	case model.FXFleetLaunch:
		if mine {
			play(audio.Launch, vol)
		}
	case model.FXFleetArrive:
		if mine {
			play(audio.Arrive, vol)
		}
	case model.FXShotFlak:
		if relevant {
			play(audio.Flak, vol*0.7)
		}
		add(0.25)
	case model.FXShotLance:
		if relevant {
			play(audio.Lance, vol*0.7)
		}
		add(0.3)
	case model.FXShotShip:
		if relevant {
			play(audio.ShipShot, vol*0.5)
		}
		add(0.25)
	case model.FXBomb:
		if relevant {
			play(audio.Bomb, vol*0.7)
		}
		add(0.5)
	case model.FXShipDestroyed:
		if relevant {
			play(audio.ExplosionSmall, vol)
		}
		add(0.8)
	case model.FXShieldHit:
		if relevant {
			play(audio.ShieldHit, vol*0.6)
		}
		add(0.4)
	case model.FXSlugFired:
		if relevant {
			play(audio.SlugFire, vol)
		}
	case model.FXSlugImpact:
		if relevant {
			play(audio.SlugImpact, vol)
		}
		add(0.6)
	case model.FXMeteorImpact:
		if relevant {
			play(audio.Meteor, vol*0.8)
		}
		add(0.6)
	case model.FXColonyFounded:
		if mine {
			play(audio.ColonyFounded, 1)
		}
	case model.FXColonyFell:
		play(audio.ColonyFell, max(vol, 0.6))
	case model.FXTechDone:
		if mine {
			play(audio.TechDone, 1)
		}
	case model.FXDirectiveDone:
		if mine {
			play(audio.DirectiveDone, 1)
		}
	case model.FXTrade:
		if mine {
			play(audio.Coin, 1)
		}
	case model.FXFlareStart:
		play(audio.FlareStart, 0.8)
	}
}

// hotkeys handles the keyboard shortcuts of the game screen.
func (g *Game) hotkeys() {
	in := g.a.in
	if g.a.ui.WantKeys() || in.KeysTaken || g.a.dialog != dialogNone || in.Ctrl || in.Alt {
		return
	}
	key := in.KeyPressed
	switch {
	case key(ebiten.KeySpace):
		if g.speed == 0 {
			g.speed = 1
		} else {
			g.speed = 0
		}
	case key(ebiten.KeyDigit1):
		g.speed = 1
	case key(ebiten.KeyDigit2):
		g.speed = 2
	case key(ebiten.KeyDigit3):
		g.speed = 4
	case key(ebiten.KeyG) && g.view != viewSurface:
		g.deposit = !g.deposit
	case key(ebiten.KeyComma):
		g.cycleRock(-1)
	case key(ebiten.KeyPeriod):
		g.cycleRock(1)
	case key(ebiten.KeyHome):
		g.centerOnKeystone()
	case key(ebiten.KeyF5):
		if err := g.a.saveGame("quicksave"); err != nil {
			g.toast(model.SevWarn, err.Error())
		} else {
			g.toast(model.SevInfo, "Game saved.")
		}
	case key(ebiten.KeyDelete):
		if g.sel.kind == selBuilding && g.window == windowNone {
			g.issue(model.DemolishCmd(g.me, g.sel.rock, g.sel.building))
			g.a.snd.Play(audio.Demolish)
			g.sel = selection{}
		}
	case key(ebiten.KeyEscape):
		switch {
		case g.menu != nil:
			g.menu = nil
		case g.orders.active:
			g.orders.active = false
		case g.place != model.BuildingNone:
			g.place = model.BuildingNone
		case g.window != windowNone:
			// the window closes itself on Escape
		case g.sel.kind != selNone:
			g.sel = selection{}
		default:
			g.a.openDialog(dialogMenu)
		}
		// The key has done its job: nothing else may react to it this frame
		// (a dialog just opened would close again at once).
		g.a.in.ClearKey(ebiten.KeyEscape)
		g.a.ui.In.ClearKey(ebiten.KeyEscape)
	}
}

func (g *Game) toggleView() {
	if g.view == viewSurface {
		g.view = viewBelt
	} else {
		g.view = viewSurface
	}
	g.place = model.BuildingNone
	g.menu = nil
}

// fmtQty formats an amount compactly for the resource bar.
func fmtQty(v float64) string {
	switch {
	case v >= 10000:
		return fmt.Sprintf("%.0fk", v/1000)
	case v >= 1000:
		return fmt.Sprintf("%.1fk", v/1000)
	}
	return fmt.Sprintf("%.0f", math.Floor(v+1e-6))
}

// fmtRate formats a per-cycle rate with its sign.
func fmtRate(v float64) string {
	if math.Abs(v) < 0.05 {
		return "±0"
	}
	if math.Abs(v) >= 10 {
		return fmt.Sprintf("%+.0f", v)
	}
	return fmt.Sprintf("%+.1f", v)
}

// fmtTime formats game-seconds as m:ss.
func fmtTime(s float64) string {
	if s < 0 {
		s = 0
	}
	return fmt.Sprintf("%d:%02d", int(s)/60, int(s)%60)
}
