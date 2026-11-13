package input

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// fakeDevice is a Device whose platform reader, clock and tick rate are under
// the test's control.
type fakeDevice struct {
	*Device
	in  rawInput  // what the next Poll reads
	now time.Time // what the clock says
	tps int       // what the tick rate is
}

func newFakeDevice(tps int) *fakeDevice {
	f := &fakeDevice{now: time.Unix(1_000, 0), tps: tps}
	f.Device = &Device{
		read: func(raw *rawInput) {
			// Copy like the real reader does: into the reused buffers.
			raw.cursorX, raw.cursorY, raw.wheelY = f.in.cursorX, f.in.cursorY, f.in.wheelY
			raw.down, raw.pressed, raw.released = f.in.down, f.in.pressed, f.in.released
			raw.keysDown = append(raw.keysDown[:0], f.in.keysDown...)
			raw.keysPressed = append(raw.keysPressed[:0], f.in.keysPressed...)
			raw.keysReleased = append(raw.keysReleased[:0], f.in.keysReleased...)
			raw.chars = append(raw.chars[:0], f.in.chars...)
		},
		clock: func() time.Time { return f.now },
		tps:   func() int { return f.tps },
	}
	return f
}

// step advances the fake clock and polls.
func (f *fakeDevice) step(elapsed time.Duration, pixelW, pixelH int, scale float32) State {
	f.now = f.now.Add(elapsed)
	return f.Poll(pixelW, pixelH, scale)
}

func TestNewDeviceIsASource(t *testing.T) {
	src := NewDevice()
	d, ok := src.(*Device)
	if !ok {
		t.Fatalf("NewDevice returned %T, want *Device", src)
	}
	if d.read == nil || d.clock == nil || d.tps == nil {
		t.Fatal("NewDevice left a dependency unset")
	}
}

func TestDevicePixelsToPoints(t *testing.T) {
	tests := []struct {
		name           string
		cursorX        float64
		cursorY        float64
		pixelW, pixelH int
		scale          float32
		x, y, w, h     float32
		wantScale      float32
	}{
		{"scale 1", 640, 360, 1600, 900, 1, 640, 360, 1600, 900, 1},
		{"scale 2", 640, 360, 3200, 1800, 2, 320, 180, 1600, 900, 2},
		{"scale 1.5", 600, 450, 2400, 1350, 1.5, 400, 300, 1600, 900, 1.5},
		{"scale 0.75", 300, 150, 1200, 675, 0.75, 400, 200, 1600, 900, 0.75},
		{"fractional pixel", 100.5, 0.25, 800, 600, 2, 50.25, 0.125, 400, 300, 2},
		{"outside the window", -40, 2000, 3200, 1800, 2, -20, 1000, 1600, 900, 2},
		{"zero scale falls back to 1", 640, 360, 1600, 900, 0, 640, 360, 1600, 900, 1},
		{"negative scale falls back to 1", 640, 360, 1600, 900, -2, 640, 360, 1600, 900, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeDevice(60)
			f.in.cursorX, f.in.cursorY = tc.cursorX, tc.cursorY
			st := f.step(0, tc.pixelW, tc.pixelH, tc.scale)
			if st.X != tc.x || st.Y != tc.y {
				t.Errorf("cursor = %v,%v points, want %v,%v", st.X, st.Y, tc.x, tc.y)
			}
			if st.W != tc.w || st.H != tc.h {
				t.Errorf("screen = %vx%v points, want %vx%v", st.W, st.H, tc.w, tc.h)
			}
			if st.Scale != tc.wantScale {
				t.Errorf("Scale = %v, want %v", st.Scale, tc.wantScale)
			}
		})
	}
}

func TestDeviceButtonsWheelAndFrame(t *testing.T) {
	f := newFakeDevice(60)

	f.in.down = [NumButtons]bool{Left: true}
	f.in.pressed = [NumButtons]bool{Left: true}
	f.in.wheelY = 1.5
	st := f.step(0, 800, 600, 1)
	if !st.Down[Left] || !st.Pressed[Left] || st.Released[Left] || st.Down[Right] || st.Down[Middle] {
		t.Errorf("frame 1 buttons: down %v pressed %v released %v", st.Down, st.Pressed, st.Released)
	}
	if st.Wheel != 1.5 {
		t.Errorf("Wheel = %v, want 1.5 (positive = away from the user)", st.Wheel)
	}
	if st.Frame != 1 {
		t.Errorf("first Frame = %d, want 1", st.Frame)
	}

	f.in = rawInput{released: [NumButtons]bool{Left: true}, down: [NumButtons]bool{Right: true}, pressed: [NumButtons]bool{Right: true}}
	st = f.step(0, 800, 600, 1)
	if st.Down[Left] || !st.Released[Left] || !st.Down[Right] || !st.Pressed[Right] {
		t.Errorf("frame 2 buttons: down %v pressed %v released %v", st.Down, st.Pressed, st.Released)
	}
	if st.Wheel != 0 || st.Frame != 2 {
		t.Errorf("frame 2: wheel %v frame %d", st.Wheel, st.Frame)
	}
}

func TestDeviceKeysAndChars(t *testing.T) {
	f := newFakeDevice(60)

	// Ebitengine lists the combined modifier next to the variant that is
	// held; the combined one must not be stored separately.
	f.in.keysDown = []Key{ebiten.KeyR, ebiten.KeyShiftLeft, ebiten.KeyShift}
	f.in.keysPressed = []Key{ebiten.KeyR, ebiten.KeyShiftLeft}
	f.in.chars = []rune("R")
	st1 := f.step(0, 800, 600, 1)

	if !st1.KeyDown(ebiten.KeyR) || !st1.KeyPressed(ebiten.KeyR) || !st1.KeyRepeated(ebiten.KeyR) {
		t.Error("R should be down, pressed and repeated on its first frame")
	}
	if !st1.Shift || st1.Ctrl || st1.Alt || !st1.KeyDown(ebiten.KeyShift) || st1.KeyDown(ebiten.KeyShiftRight) {
		t.Errorf("modifiers: shift %v ctrl %v alt %v", st1.Shift, st1.Ctrl, st1.Alt)
	}
	if got := st1.AppendKeysDown(nil); len(got) != 2 {
		t.Errorf("keys down = %v, want R and ShiftLeft only", got)
	}
	if string(st1.Chars) != "R" {
		t.Errorf("Chars = %q, want %q", string(st1.Chars), "R")
	}

	f.in.keysDown = []Key{ebiten.KeyControlRight, ebiten.KeyControl, ebiten.KeyAltLeft, ebiten.KeyAlt}
	f.in.keysPressed = []Key{ebiten.KeyControlRight, ebiten.KeyAltLeft}
	f.in.keysReleased = []Key{ebiten.KeyR, ebiten.KeyShiftLeft}
	f.in.chars = []rune("xy")
	st2 := f.step(0, 800, 600, 1)

	if st2.KeyDown(ebiten.KeyR) || !st2.KeyReleased(ebiten.KeyR) || st2.KeyPressed(ebiten.KeyR) || st2.KeyRepeated(ebiten.KeyR) {
		t.Error("R should be released on the second frame")
	}
	if st2.Shift || !st2.Ctrl || !st2.Alt {
		t.Errorf("modifiers: shift %v ctrl %v alt %v", st2.Shift, st2.Ctrl, st2.Alt)
	}
	if string(st2.Chars) != "xy" {
		t.Errorf("Chars = %q, want %q", string(st2.Chars), "xy")
	}
	// The first frame's snapshot must not have been overwritten by the
	// second poll: States are retained by callers.
	if string(st1.Chars) != "R" || !st1.KeyDown(ebiten.KeyR) {
		t.Error("a later Poll modified an earlier State")
	}

	f.in = rawInput{}
	st3 := f.step(0, 800, 600, 1)
	if st3.Chars != nil {
		t.Errorf("Chars = %q on a frame without typing, want nil", string(st3.Chars))
	}
}

func TestDeviceFrameTime(t *testing.T) {
	ms := time.Millisecond
	tests := []struct {
		name    string
		tps     int
		elapsed []time.Duration // wall-clock time before each poll
		want    []float32       // reported Dt
	}{
		{"fixed 60 ticks ignores the clock", 60, []time.Duration{0, 3 * ms, 250 * ms}, []float32{0.016667, 0.016667, 0.016667}},
		{"fixed 120 ticks", 120, []time.Duration{0, 50 * ms}, []float32{0.008333, 0.008333}},
		{"fixed 30 ticks", 30, []time.Duration{0, 0}, []float32{0.033333, 0.033333}},
		{"synced: first frame is nominal, then measured", ebiten.SyncWithFPS, []time.Duration{0, 7 * ms, 20 * ms}, []float32{0.016667, 0.007, 0.020}},
		{"synced: a stall is capped at 100 ms", ebiten.SyncWithFPS, []time.Duration{0, 3 * time.Second}, []float32{0.016667, 0.1}},
		{"synced: a zero-length frame is raised to 1 ms", ebiten.SyncWithFPS, []time.Duration{0, 0}, []float32{0.016667, 0.001}},
		{"zero ticks is treated as synced", 0, []time.Duration{0, 12 * ms}, []float32{0.016667, 0.012}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeDevice(tc.tps)
			for i, el := range tc.elapsed {
				st := f.step(el, 800, 600, 1)
				if d := st.Dt - tc.want[i]; d < -1e-6 || d > 1e-6 {
					t.Errorf("poll %d: Dt = %v, want %v", i+1, st.Dt, tc.want[i])
				}
			}
		})
	}
}

// clickAfter runs two left presses through a device, the second one after the
// given wall-clock gap, and reports whether it counted as a double click.
// The gap is split over as many polls as needed, because one frame never
// counts for more than 100 ms.
func clickAfter(t *testing.T, tps int, gap time.Duration, dx float64) bool {
	t.Helper()
	f := newFakeDevice(tps)
	press := func(x float64) {
		f.in = rawInput{cursorX: x, cursorY: 50, down: [NumButtons]bool{Left: true}, pressed: [NumButtons]bool{Left: true}}
	}
	idle := func(x float64) { f.in = rawInput{cursorX: x, cursorY: 50} }

	press(100)
	if st := f.step(10*time.Millisecond, 800, 600, 1); st.DoubleClick {
		t.Fatal("the first press cannot be a double click")
	}
	const tick = 10 * time.Millisecond
	for ; gap > tick; gap -= tick {
		idle(100)
		if st := f.step(tick, 800, 600, 1); st.DoubleClick {
			t.Fatal("double click reported without a press")
		}
	}
	press(100 + dx)
	return f.step(gap, 800, 600, 1).DoubleClick
}

func TestDeviceDoubleClickUsesTheInjectedClock(t *testing.T) {
	ms := time.Millisecond
	tests := []struct {
		name string
		gap  time.Duration
		dx   float64 // pixels at scale 1, i.e. points
		want bool
	}{
		{"fast", 120 * ms, 0, true},
		{"at the limit", 350 * ms, 0, true},
		{"just over the limit", 351 * ms, 0, false},
		{"slow", 900 * ms, 0, false},
		{"fast but 4 points away", 120 * ms, 4, true},
		{"fast but 5 points away", 120 * ms, 5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := clickAfter(t, ebiten.SyncWithFPS, tc.gap, tc.dx); got != tc.want {
				t.Errorf("double click = %v, want %v", got, tc.want)
			}
		})
	}
}

// The distance rule is in points: at scale 2, six pixels are three points.
func TestDeviceDoubleClickDistanceIsInPoints(t *testing.T) {
	tests := []struct {
		scale float32
		dxPx  float64
		want  bool
	}{
		{1, 6, false},
		{2, 6, true},
		{2, 8, true},
		{2, 9, false},
	}
	for _, tc := range tests {
		f := newFakeDevice(60)
		f.in = rawInput{cursorX: 200, cursorY: 200, down: [NumButtons]bool{Left: true}, pressed: [NumButtons]bool{Left: true}}
		f.step(0, 1600, 900, tc.scale)
		f.in = rawInput{cursorX: 200, cursorY: 200}
		f.step(0, 1600, 900, tc.scale)
		f.in = rawInput{cursorX: 200 + tc.dxPx, cursorY: 200, down: [NumButtons]bool{Left: true}, pressed: [NumButtons]bool{Left: true}}
		if got := f.step(0, 1600, 900, tc.scale).DoubleClick; got != tc.want {
			t.Errorf("scale %v, %v px apart: double click = %v, want %v", tc.scale, tc.dxPx, got, tc.want)
		}
	}
}

func TestDeviceKeyRepeat(t *testing.T) {
	f := newFakeDevice(60)
	var got []int
	for frame := 1; frame <= 30; frame++ {
		f.in = rawInput{keysDown: []Key{ebiten.KeyArrowLeft}}
		if frame == 1 {
			f.in.keysPressed = []Key{ebiten.KeyArrowLeft}
		}
		st := f.step(0, 800, 600, 1)
		if st.KeyRepeated(ebiten.KeyArrowLeft) {
			got = append(got, frame)
		}
		if st.KeyPressed(ebiten.KeyArrowLeft) != (frame == 1) {
			t.Fatalf("frame %d: KeyPressed must be a single-frame edge", frame)
		}
	}
	want := []int{1, 25, 28, 30}
	if len(got) != len(want) {
		t.Fatalf("repeats on frames %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("repeats on frames %v, want %v", got, want)
		}
	}
}
