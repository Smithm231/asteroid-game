// Command drifthold is the game.
//
// Run it without arguments to play. The flags exist for testing:
//
//	drifthold -mute                     no sound (needed without a sound device)
//	drifthold -new 42                   skip the title screen: new game with seed 42
//	drifthold -load path/to/save.drift  start from a save file
//	drifthold -script steps.json -out shots/   play scripted input, save screenshots
package main

import (
	"flag"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/app"
	"drifthold/internal/emblem"
	"drifthold/internal/input"
	"drifthold/internal/model"
	"drifthold/internal/uitest"
)

type game struct {
	a     *app.App
	src   input.Source
	scale float64
	full  bool
}

func (g *game) Update() error {
	w, h := ebiten.WindowSize()
	dev := ebiten.Monitor().DeviceScaleFactor()
	if ebiten.IsFullscreen() {
		w, h = ebiten.Monitor().Size()
	}
	g.scale = dev * uiScale(g.a.UIScale(), h)
	pw, ph := int(float64(w)*dev), int(float64(h)*dev)
	g.a.Frame(g.src.Poll(pw, ph, float32(g.scale)))
	if g.a.Fullscreen() != g.full {
		g.full = g.a.Fullscreen()
		ebiten.SetFullscreen(g.full)
	}
	if g.a.Quit() {
		return ebiten.Termination
	}
	return nil
}

// uiScale returns the interface scale: the player's choice, or one that keeps
// the layout comfortable for the window height.
func uiScale(user float64, winH int) float64 {
	if user > 0 {
		return user
	}
	switch {
	case winH >= 1700:
		return 1.5
	case winH >= 1300:
		return 1.25
	case winH < 700:
		return 0.75
	case winH < 800:
		return 0.85
	}
	return 1
}

func (g *game) Draw(screen *ebiten.Image) { g.a.Draw(screen) }

func (g *game) Layout(w, h int) (int, int) { panic("unused: LayoutF is implemented") }

func (g *game) LayoutF(w, h float64) (float64, float64) {
	dev := ebiten.Monitor().DeviceScaleFactor()
	return w * dev, h * dev
}

func main() {
	mute := flag.Bool("mute", false, "disable sound")
	newSeed := flag.Int64("new", -1, "start a new game with this seed, skipping the title screen")
	rivals := flag.Int("rivals", 2, "number of rivals for -new (1-3)")
	load := flag.String("load", "", "start by loading this save file")
	portable := flag.String("portable", "", "keep saves and settings in this directory")
	script := flag.String("script", "", "play scripted input from this JSON file (implies -mute)")
	out := flag.String("out", "shots", "directory for screenshots taken by a script")
	sw := flag.Int("w", 1600, "window width in points for -script")
	sh := flag.Int("h", 900, "window height in points for -script")
	sscale := flag.Float64("scale", 1, "pixels per point for -script")
	flag.Parse()

	opts := app.Options{Mute: *mute || *script != "", Load: *load, Portable: *portable}
	if *newSeed >= 0 {
		st := model.DefaultSettings(uint64(*newSeed))
		st.Rivals = *rivals
		opts.NewGame = &st
	}

	if *script != "" {
		steps, err := input.LoadScript(*script)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		opts.FixedStep = true
		if opts.Portable == "" {
			opts.Portable = filepath.Join(*out, "profile")
		}
		a := app.New(opts)
		err = uitest.Run(uitest.Options{W: *sw, H: *sh, Scale: float32(*sscale), Script: steps, OutDir: *out, Title: "Drifthold"},
			func(in input.State) { a.Frame(in) }, a.Draw)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	a := app.New(opts)
	g := &game{a: a, src: input.NewDevice(), full: a.Fullscreen()}
	ebiten.SetWindowTitle("Drifthold")
	ebiten.SetWindowSizeLimits(1100, 640, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// Open at a comfortable size, or maximised when the screen is too small
	// for it (a 1080p laptop at 150 % scaling has only 1280x720 to offer).
	if mw, mh := ebiten.Monitor().Size(); mw >= 1700 && mh >= 1000 {
		ebiten.SetWindowSize(1600, 900)
	} else {
		ebiten.SetWindowSize(min(1600, max(1100, mw-80)), min(900, max(640, mh-120)))
		ebiten.MaximizeWindow()
	}
	ebiten.SetFullscreen(g.full)
	ebiten.SetVsyncEnabled(true)
	var icons []image.Image
	for _, s := range []int{16, 32, 48, 64, 128, 256} {
		icons = append(icons, emblem.Render(s))
	}
	ebiten.SetWindowIcon(icons)
	if err := ebiten.RunGameWithOptions(g, &ebiten.RunGameOptions{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
