// Package uitest is a tiny host for visual tests. It opens a window, drives a
// UI with scripted input from package input, and writes the screenshots the
// script asks for, so that a demo program can be run headlessly and its
// output inspected:
//
//	xvfb-run -a -s "-screen 0 3840x2160x24" go run ./internal/<pkg>/demo -out <dir> -scale 2
//
// A demo's main looks like this:
//
//	func main() {
//		o := uitest.Flags()              // -out -scale -w -h -script -frames
//		if o.Script == nil {
//			o.Script = builtinScript()   // []input.Step
//		}
//		err := uitest.Run(o,
//			func(in input.State) { /* update: declare widgets, react to in */ },
//			func(dst *ebiten.Image) { /* draw; dst is W*Scale x H*Scale pixels */ })
//		if err != nil {
//			log.Fatal(err)
//		}
//	}
//
// Run never creates an audio context; a machine without a sound card is fine.
package uitest

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/input"
)

// Defaults applied by Run to zero Options fields.
const (
	DefaultW      = 1600
	DefaultH      = 900
	DefaultScale  = 1
	DefaultOutDir = "shots"
	DefaultTitle  = "Drifthold UI test"
)

// Options configures a Run.
type Options struct {
	// W and H are the window size in points (default 1600×900).
	W, H int
	// Scale is the number of pixels per point (default 1). The window, the
	// image handed to draw and the screenshots measure W×Scale by H×Scale
	// pixels.
	Scale float32
	// Script is the input to play back. Run returns when it has finished.
	Script []input.Step
	// OutDir is the directory that receives the PNGs requested by "shot"
	// steps (default "shots"). It is created if missing.
	OutDir string
	// Frames, when positive, stops the run after that many frames even if
	// the script has not finished.
	Frames int
	// Title is the window title (default "Drifthold UI test").
	Title string
}

// withDefaults returns o with every unset field replaced by its default.
func (o Options) withDefaults() Options {
	if o.W <= 0 {
		o.W = DefaultW
	}
	if o.H <= 0 {
		o.H = DefaultH
	}
	if !(o.Scale > 0) {
		o.Scale = DefaultScale
	}
	if o.OutDir == "" {
		o.OutDir = DefaultOutDir
	}
	if o.Title == "" {
		o.Title = DefaultTitle
	}
	return o
}

// PixelSize returns the size in device pixels of the screen image that Run
// hands to draw for these options: points × scale, rounded to whole pixels.
func (o Options) PixelSize() (w, h int) {
	o = o.withDefaults()
	return scaled(o.W, o.Scale), scaled(o.H, o.Scale)
}

func scaled(points int, scale float32) int {
	return max(1, int(math.Round(float64(points)*float64(scale))))
}

// stepper is the frame logic of a run, kept free of any window so that it can
// be tested: it keeps update and draw in lock step, collects screenshots after
// the draw of the frame that requested them, and decides when to stop.
type stepper struct {
	src       *input.ScriptSource
	pixelW    int
	pixelH    int
	scale     float32
	maxFrames int
	update    func(in input.State)

	frames   int  // frames polled so far
	pending  bool // the latest frame has been updated but not drawn yet
	finished bool
	err      error
}

// tick is called for every game update. It polls one frame of input and runs
// the update callback, unless the previous frame is still waiting to be drawn
// or the run is over.
func (s *stepper) tick() {
	if s.finished || s.pending || s.err != nil {
		return
	}
	in := s.src.Poll(s.pixelW, s.pixelH, s.scale)
	s.frames++
	s.pending = true
	if s.update != nil {
		s.update(in)
	}
}

// drawn is called after every draw. For the first draw of a frame it saves
// the screenshots that frame requested and then checks whether the run is
// complete.
func (s *stepper) drawn(save func(name string) error) {
	if !s.pending {
		return
	}
	s.pending = false
	for {
		name, ok := s.src.TakeShot()
		if !ok {
			break
		}
		if err := save(name); err != nil && s.err == nil {
			s.err = err
		}
	}
	if s.src.Done() || (s.maxFrames > 0 && s.frames >= s.maxFrames) {
		s.finished = true
	}
}

// game adapts a stepper to Ebitengine.
type game struct {
	step   stepper
	draw   func(dst *ebiten.Image)
	outDir string
	report io.Writer // receives one line per screenshot; nil for silence
	pix    []byte
}

// Update implements ebiten.Game.
func (g *game) Update() error {
	if g.step.err != nil {
		return g.step.err
	}
	if g.step.finished {
		return ebiten.Termination
	}
	g.step.tick()
	return nil
}

// Draw implements ebiten.Game.
func (g *game) Draw(screen *ebiten.Image) {
	if g.draw != nil {
		g.draw(screen)
	}
	g.step.drawn(func(name string) error {
		b := screen.Bounds()
		if n := 4 * b.Dx() * b.Dy(); len(g.pix) != n {
			g.pix = make([]byte, n)
		}
		screen.ReadPixels(g.pix)
		path, err := writeShot(g.outDir, name, g.pix, b.Dx(), b.Dy())
		if err != nil {
			return err
		}
		if g.report != nil {
			fmt.Fprintf(g.report, "uitest: frame %d: saved %s (%d×%d px)\n", g.step.frames, path, b.Dx(), b.Dy())
		}
		return nil
	})
}

// Layout implements ebiten.Game. The screen always has the configured pixel
// size, so that one pixel is 1/Scale points whatever the window does.
func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return g.step.pixelW, g.step.pixelH
}

// Run opens a window of o.W × o.H points at o.Scale pixels per point and
// plays o.Script. Every frame it polls the scripted input, calls update with
// it, calls draw with the screen image, and then writes the screenshots the
// script requested on that frame into o.OutDir, printing one line per file to
// standard output. It returns when the script has finished (or after
// o.Frames frames), with the first error encountered.
//
// Update and draw alternate strictly, one draw per update, and the reported
// frame time is always 1/60 s, so a run is reproducible however fast the
// machine renders. Either callback may be nil.
//
// Run must be called from the main goroutine (from main), at most once per
// process.
func Run(o Options, update func(in input.State), draw func(dst *ebiten.Image)) error {
	o = o.withDefaults()
	if err := input.Validate(o.Script); err != nil {
		return fmt.Errorf("uitest: %w", err)
	}
	for i, st := range o.Script {
		if st.Shot == "" {
			continue
		}
		if _, err := shotPath(o.OutDir, st.Shot); err != nil {
			return fmt.Errorf("uitest: script step %d: %w", i+1, err)
		}
	}
	if err := os.MkdirAll(o.OutDir, 0o755); err != nil {
		return fmt.Errorf("uitest: creating output directory: %w", err)
	}

	pw, ph := o.PixelSize()
	g := &game{
		step: stepper{
			src:       input.NewScript(o.Script),
			pixelW:    pw,
			pixelH:    ph,
			scale:     o.Scale,
			maxFrames: o.Frames,
			update:    update,
		},
		draw:   draw,
		outDir: o.OutDir,
		report: os.Stdout,
	}

	// The window is sized so that it shows the screen image 1:1 in physical
	// pixels on the monitor it opens on.
	device := 1.0
	if m := ebiten.Monitor(); m != nil {
		if f := m.DeviceScaleFactor(); f > 0 {
			device = f
		}
	}
	ebiten.SetWindowSize(
		max(1, int(math.Ceil(float64(pw)/device))),
		max(1, int(math.Ceil(float64(ph)/device))))
	ebiten.SetWindowTitle(o.Title)
	// A scripted run must not depend on window focus or on the display's
	// refresh rate: update once per drawn frame, as fast as frames render.
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetVsyncEnabled(false)
	ebiten.SetTPS(ebiten.SyncWithFPS)

	err := ebiten.RunGame(g)
	if g.step.err != nil {
		// A screenshot could not be written; that is the error RunGame
		// was stopped with.
		return g.step.err
	}
	if err != nil && !errors.Is(err, ebiten.Termination) {
		return fmt.Errorf("uitest: %w", err)
	}
	return nil
}
