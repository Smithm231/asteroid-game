package uitest

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/input"
)

func TestOptionsDefaults(t *testing.T) {
	got := Options{}.withDefaults()
	want := Options{W: 1600, H: 900, Scale: 1, OutDir: "shots", Title: "Drifthold UI test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("defaults = %+v, want %+v", got, want)
	}

	// Explicit values survive; nonsense falls back.
	script := []input.Step{input.Wait(1)}
	in := Options{W: 1280, H: 720, Scale: 1.5, Script: script, OutDir: "x", Frames: 9, Title: "T"}
	if got := in.withDefaults(); !reflect.DeepEqual(got, in) {
		t.Errorf("explicit options changed: %+v", got)
	}
	bad := Options{W: -3, H: 0, Scale: -1}.withDefaults()
	if bad.W != DefaultW || bad.H != DefaultH || bad.Scale != DefaultScale {
		t.Errorf("invalid sizes not replaced: %+v", bad)
	}
}

func TestPixelSize(t *testing.T) {
	tests := []struct {
		w, h   int
		scale  float32
		pw, ph int
	}{
		{0, 0, 0, 1600, 900},
		{1600, 900, 1, 1600, 900},
		{1600, 900, 2, 3200, 1800},
		{1600, 900, 1.5, 2400, 1350},
		{1280, 720, 0.75, 960, 540},
		{960, 600, 2, 1920, 1200},
		{961, 601, 1.5, 1442, 902}, // 1441.5 and 901.5 round up
		{1001, 3, 1.25, 1251, 4},   // 1251.25 and 3.75
		{1, 1, 0.25, 1, 1},         // never below one pixel
	}
	for _, tc := range tests {
		pw, ph := Options{W: tc.w, H: tc.h, Scale: tc.scale}.PixelSize()
		if pw != tc.pw || ph != tc.ph {
			t.Errorf("%d×%d pt at scale %v = %d×%d px, want %d×%d", tc.w, tc.h, tc.scale, pw, ph, tc.pw, tc.ph)
		}
	}
}

// host simulates the game loop around a stepper and records what happens.
type host struct {
	step    stepper
	updates []int64  // frame numbers passed to update
	saved   []string // "frame:name" for every screenshot
	saveErr error
}

func newHost(steps []input.Step, maxFrames int) *host {
	h := &host{}
	h.step = stepper{
		src:       input.NewScript(steps),
		pixelW:    3200,
		pixelH:    1800,
		scale:     2,
		maxFrames: maxFrames,
		update: func(in input.State) {
			h.updates = append(h.updates, in.Frame)
		},
	}
	return h
}

func (h *host) draw() {
	h.step.drawn(func(name string) error {
		h.saved = append(h.saved, fmt.Sprintf("%d:%s", h.step.frames, name))
		return h.saveErr
	})
}

// run alternates update and draw until the stepper finishes.
func (h *host) run(t *testing.T) {
	t.Helper()
	for i := 0; i < 10_000; i++ {
		if h.step.finished {
			return
		}
		h.step.tick()
		h.draw()
	}
	t.Fatal("run did not finish")
}

func TestStepperRunsScriptAndSavesShots(t *testing.T) {
	h := newHost([]input.Step{
		input.ClickAt(10, 10),   // frames 1-3
		input.Shot("click.png"), // frame 4
		input.Wait(2),           // frames 5-6
		input.Shot("later.png"), // frame 7
		input.Quit(),
	}, 0)
	h.run(t)

	if want := []int64{1, 2, 3, 4, 5, 6, 7}; !reflect.DeepEqual(h.updates, want) {
		t.Errorf("updates = %v, want %v", h.updates, want)
	}
	// Each shot is saved after the draw of its own frame, which is the frame
	// after the input it documents.
	if want := []string{"4:click.png", "7:later.png"}; !reflect.DeepEqual(h.saved, want) {
		t.Errorf("saved = %v, want %v", h.saved, want)
	}
	if h.step.err != nil {
		t.Errorf("err = %v", h.step.err)
	}
}

func TestStepperPassesScreenSizeInPoints(t *testing.T) {
	var got input.State
	s := stepper{
		src:    input.NewScript([]input.Step{input.MoveTo(100, 50)}),
		pixelW: 3200, pixelH: 1800, scale: 2,
		update: func(in input.State) { got = in },
	}
	s.tick()
	if got.W != 1600 || got.H != 900 || got.Scale != 2 || got.X != 100 || got.Y != 50 {
		t.Errorf("state = %+v", got)
	}
}

// Updates and draws stay in lock step whatever order the engine calls them
// in: an update without a draw in between is held back, and a second draw of
// the same frame saves nothing twice.
func TestStepperLockStep(t *testing.T) {
	h := newHost([]input.Step{input.Shot("a.png"), input.Shot("b.png"), input.Wait(1)}, 0)

	h.draw() // a draw before the first update does nothing
	if len(h.saved) != 0 || h.step.finished {
		t.Fatal("draw before any update had an effect")
	}

	h.step.tick()
	h.step.tick() // held back: frame 1 has not been drawn
	h.step.tick()
	if want := []int64{1}; !reflect.DeepEqual(h.updates, want) {
		t.Fatalf("updates = %v, want %v", h.updates, want)
	}

	h.draw()
	h.draw() // repeated draw of frame 1
	if want := []string{"1:a.png"}; !reflect.DeepEqual(h.saved, want) {
		t.Fatalf("saved = %v, want %v", h.saved, want)
	}

	h.step.tick()
	h.draw()
	h.step.tick()
	if h.step.finished {
		t.Fatal("finished before the last frame was drawn")
	}
	h.draw()
	if !h.step.finished {
		t.Fatal("not finished after the last frame was drawn")
	}
	if want := []string{"1:a.png", "2:b.png"}; !reflect.DeepEqual(h.saved, want) {
		t.Errorf("saved = %v, want %v", h.saved, want)
	}

	// Once finished, nothing moves any more.
	h.step.tick()
	h.draw()
	if want := []int64{1, 2, 3}; !reflect.DeepEqual(h.updates, want) {
		t.Errorf("updates = %v, want %v", h.updates, want)
	}
}

func TestStepperFrameLimit(t *testing.T) {
	tests := []struct {
		name      string
		steps     []input.Step
		maxFrames int
		frames    int
		saved     []string
	}{
		{"limit before the script ends", []input.Step{input.Wait(100), input.Shot("never.png")}, 5, 5, nil},
		{"script ends before the limit", []input.Step{input.Wait(3)}, 50, 3, nil},
		{"shot on the last allowed frame", []input.Step{input.Wait(2), input.Shot("edge.png"), input.Wait(10)}, 3, 3, []string{"3:edge.png"}},
		{"no limit", []input.Step{input.Wait(40)}, 0, 40, nil},
		// An empty script still shows one frame.
		{"empty script", nil, 0, 1, nil},
		{"empty script with a limit", nil, 7, 1, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(tc.steps, tc.maxFrames)
			h.run(t)
			if h.step.frames != tc.frames || len(h.updates) != tc.frames {
				t.Errorf("ran %d frames (%d updates), want %d", h.step.frames, len(h.updates), tc.frames)
			}
			if !reflect.DeepEqual(h.saved, tc.saved) {
				t.Errorf("saved = %v, want %v", h.saved, tc.saved)
			}
		})
	}
}

func TestStepperStopsOnSaveError(t *testing.T) {
	h := newHost([]input.Step{input.Shot("a.png"), input.Wait(5), input.Shot("b.png")}, 0)
	h.saveErr = errors.New("disk full")
	for i := 0; i < 20; i++ {
		h.step.tick()
		h.draw()
	}
	if !errors.Is(h.step.err, h.saveErr) {
		t.Fatalf("err = %v, want the save error", h.step.err)
	}
	// The failed frame is the last one: no further updates, no second shot.
	if want := []int64{1}; !reflect.DeepEqual(h.updates, want) {
		t.Errorf("updates = %v, want %v", h.updates, want)
	}
	if want := []string{"1:a.png"}; !reflect.DeepEqual(h.saved, want) {
		t.Errorf("saved = %v, want %v", h.saved, want)
	}
}

func TestStepperNilUpdate(t *testing.T) {
	s := stepper{src: input.NewScript([]input.Step{input.Wait(2)}), pixelW: 10, pixelH: 10, scale: 1}
	for i := 0; i < 5 && !s.finished; i++ {
		s.tick()
		s.drawn(func(string) error { return nil })
	}
	if !s.finished || s.frames != 2 {
		t.Errorf("finished %v after %d frames, want true after 2", s.finished, s.frames)
	}
}

// The game adapter reports the configured pixel size whatever the window
// size is, and turns the stepper's outcome into Ebitengine's conventions.
func TestGameLayoutAndTermination(t *testing.T) {
	g := &game{step: stepper{
		src:    input.NewScript(nil),
		pixelW: 3200, pixelH: 1800, scale: 2,
	}}
	for _, outside := range [][2]int{{3200, 1800}, {800, 600}, {1, 1}} {
		if w, h := g.Layout(outside[0], outside[1]); w != 3200 || h != 1800 {
			t.Errorf("Layout(%d, %d) = %d×%d, want 3200×1800", outside[0], outside[1], w, h)
		}
	}

	if err := g.Update(); err != nil {
		t.Fatalf("first Update = %v", err)
	}
	g.step.drawn(func(string) error { return nil })
	if err := g.Update(); !errors.Is(err, ebiten.Termination) {
		t.Errorf("Update after the script ended = %v, want ebiten.Termination", err)
	}

	boom := errors.New("boom")
	g.step.err = boom
	if err := g.Update(); !errors.Is(err, boom) {
		t.Errorf("Update with a pending error = %v, want it returned", err)
	}
}

// Run rejects bad input before it opens a window, so these cases are safe to
// exercise without a display.
func TestRunRejectsBadScripts(t *testing.T) {
	tests := []struct {
		name  string
		steps []input.Step
		want  string
	}{
		{"invalid step", []input.Step{input.Wait(1), {Key: "NoSuchKey"}}, "step 2"},
		{"two actions", []input.Step{{Wait: 1, Quit: true}}, "more than one action"},
		{"shot outside the output directory", []input.Step{input.Shot("../up.png")}, "inside the output directory"},
		{"absolute shot path", []input.Step{input.Wait(1), input.Shot("/etc/x.png")}, "step 2"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			err := Run(Options{Script: tc.steps, OutDir: dir}, nil, nil)
			if err == nil {
				t.Fatal("Run accepted the script")
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.HasPrefix(err.Error(), "uitest: ") {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}
