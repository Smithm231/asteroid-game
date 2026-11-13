package input

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Source produces one State per frame. pixelW and pixelH are the size of the
// screen image in device pixels and scale is the number of pixels per point;
// the source reports cursor position and screen size in points.
//
// Poll must be called exactly once per game Update, because edges (Pressed,
// Released, KeyPressed, ...) describe the change since the previous poll.
type Source interface {
	Poll(pixelW, pixelH int, scale float32) State
}

// Limits for the measured frame time used when the game does not run at a
// fixed tick rate: a stalled frame must not fast-forward key repeat, and a
// zero-length frame must not stall it.
const (
	minFrameMicros = 1_000
	maxFrameMicros = 100_000
)

// rawInput is everything a Device needs from the platform for one frame.
// Cursor coordinates are in device pixels. The slices are reused between
// frames.
type rawInput struct {
	cursorX, cursorY        float64
	wheelY                  float64
	down, pressed, released [NumButtons]bool
	keysDown                []Key
	keysPressed             []Key
	keysReleased            []Key
	chars                   []rune
}

// ebitenButtons maps Button to Ebitengine's mouse button codes.
var ebitenButtons = [NumButtons]ebiten.MouseButton{
	Left:   ebiten.MouseButtonLeft,
	Right:  ebiten.MouseButtonRight,
	Middle: ebiten.MouseButtonMiddle,
}

// readEbiten fills raw from Ebitengine's input functions. It is only valid
// inside a game's Update.
func readEbiten(raw *rawInput) {
	raw.cursorX, raw.cursorY = ebiten.CursorPositionF()
	_, raw.wheelY = ebiten.Wheel()
	for b, eb := range ebitenButtons {
		raw.down[b] = ebiten.IsMouseButtonPressed(eb)
		raw.pressed[b] = inpututil.IsMouseButtonJustPressed(eb)
		raw.released[b] = inpututil.IsMouseButtonJustReleased(eb)
	}
	raw.keysDown = inpututil.AppendPressedKeys(raw.keysDown[:0])
	raw.keysPressed = inpututil.AppendJustPressedKeys(raw.keysPressed[:0])
	raw.keysReleased = inpututil.AppendJustReleasedKeys(raw.keysReleased[:0])
	raw.chars = ebiten.AppendInputChars(raw.chars[:0])
}

// Device is the Source backed by the real mouse and keyboard. Create it with
// NewDevice.
type Device struct {
	read  func(*rawInput)  // platform reader
	clock func() time.Time // wall clock, used only without a fixed tick rate
	tps   func() int       // ticks per second; <= 0 means "one update per drawn frame"

	raw     rawInput
	trk     tracker
	frame   int64
	last    time.Time
	hasLast bool
}

// NewDevice returns the Source that reads the real mouse and keyboard through
// Ebitengine. Its Poll must be called from the game's Update, once per call.
//
// Frame time is 1/TPS while the game runs at a fixed tick rate (Ebitengine's
// default) and is measured with the wall clock, clamped to 1..100 ms, when
// ticks are synchronised with the display.
//
// Key and button levels and edges are Ebitengine's. In particular a modifier
// key (Shift, Control, Alt, Meta) that is released during a frame still
// counts as down for that frame, so that a chord whose keys are let go
// together is not lost.
func NewDevice() Source {
	return &Device{read: readEbiten, clock: time.Now, tps: ebiten.TPS}
}

// Poll implements Source.
func (d *Device) Poll(pixelW, pixelH int, scale float32) State {
	if !(scale > 0) {
		scale = 1
	}
	d.read(&d.raw)
	d.frame++
	dt := d.frameMicros()

	raw := &d.raw
	st := State{
		X:        float32(raw.cursorX / float64(scale)),
		Y:        float32(raw.cursorY / float64(scale)),
		Wheel:    float32(raw.wheelY),
		Down:     raw.down,
		Pressed:  raw.pressed,
		Released: raw.released,
		W:        float32(pixelW) / scale,
		H:        float32(pixelH) / scale,
		Scale:    scale,
		Frame:    d.frame,
		Dt:       float32(float64(dt) / 1e6),
	}
	// assign ignores the combined modifier keys Ebitengine lists next to
	// their left/right variants.
	for _, k := range raw.keysDown {
		st.keysDown.assign(k, true)
	}
	for _, k := range raw.keysPressed {
		st.keysPressed.assign(k, true)
	}
	for _, k := range raw.keysReleased {
		st.keysReleased.assign(k, true)
	}
	st.syncModifiers()
	if len(raw.chars) > 0 {
		st.Chars = append([]rune(nil), raw.chars...)
	}
	d.trk.advance(&st, dt)
	return st
}

// frameMicros returns the duration of the frame being polled, in
// microseconds.
func (d *Device) frameMicros() int64 {
	now := d.clock()
	last, hadLast := d.last, d.hasLast
	d.last, d.hasLast = now, true

	if tps := d.tps(); tps > 0 {
		return (1_000_000 + int64(tps)/2) / int64(tps)
	}
	if !hadLast {
		return nominalFrameMicros
	}
	dt := now.Sub(last).Microseconds()
	if dt < minFrameMicros {
		dt = minFrameMicros
	}
	if dt > maxFrameMicros {
		dt = maxFrameMicros
	}
	return dt
}
