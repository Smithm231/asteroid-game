package input

import (
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestKeySetCoversEveryKey(t *testing.T) {
	if keyWords*64 < numKeys {
		t.Fatalf("keySet holds %d bits, need %d", keyWords*64, numKeys)
	}
	var ks keySet
	if !ks.empty() {
		t.Fatal("zero keySet should be empty")
	}
	stored := 0
	for k := Key(0); int(k) < numKeys; k++ {
		ks.assign(k, true)
		if validKey(k) {
			stored++
			if !ks.has(k) {
				t.Errorf("key %d (%s) not stored", int(k), k)
			}
		} else if ks.has(k) {
			t.Errorf("combined key %d (%s) must not be stored", int(k), k)
		}
	}
	if got := len(ks.appendTo(nil)); got != stored {
		t.Errorf("appendTo listed %d keys, want %d", got, stored)
	}
	for k := Key(0); int(k) < numKeys; k++ {
		ks.assign(k, false)
	}
	if !ks.empty() {
		t.Error("keySet should be empty after clearing every key")
	}
	// Out-of-range keys are ignored, never a panic.
	ks.assign(Key(-1), true)
	ks.assign(Key(numKeys), true)
	ks.assign(Key(1<<20), true)
	if !ks.empty() || ks.has(Key(-1)) || ks.has(Key(numKeys)) || ks.query(Key(1<<20)) {
		t.Error("out-of-range keys must be ignored")
	}
}

func TestStateKeyQueries(t *testing.T) {
	var s State
	s.PressKey(ebiten.KeyR)
	s.HoldKey(ebiten.KeyW)
	s.RepeatKey(ebiten.KeyBackspace)
	s.ReleaseKey(ebiten.KeyEscape)

	tests := []struct {
		key                               Key
		down, pressed, released, repeated bool
	}{
		{ebiten.KeyR, true, true, false, true},
		{ebiten.KeyW, true, false, false, false},
		{ebiten.KeyBackspace, true, false, false, true},
		{ebiten.KeyEscape, false, false, true, false},
		{ebiten.KeyQ, false, false, false, false},
		{Key(-5), false, false, false, false},
		{Key(numKeys + 3), false, false, false, false},
	}
	for _, tc := range tests {
		if got := s.KeyDown(tc.key); got != tc.down {
			t.Errorf("KeyDown(%s) = %v, want %v", tc.key, got, tc.down)
		}
		if got := s.KeyPressed(tc.key); got != tc.pressed {
			t.Errorf("KeyPressed(%s) = %v, want %v", tc.key, got, tc.pressed)
		}
		if got := s.KeyReleased(tc.key); got != tc.released {
			t.Errorf("KeyReleased(%s) = %v, want %v", tc.key, got, tc.released)
		}
		if got := s.KeyRepeated(tc.key); got != tc.repeated {
			t.Errorf("KeyRepeated(%s) = %v, want %v", tc.key, got, tc.repeated)
		}
	}

	wantDown := []Key{ebiten.KeyR, ebiten.KeyW, ebiten.KeyBackspace}
	if got := s.AppendKeysDown(nil); !reflect.DeepEqual(got, wantDown) {
		t.Errorf("AppendKeysDown = %v, want %v", got, wantDown)
	}
	if got := s.AppendKeysPressed([]Key{ebiten.KeyA}); !reflect.DeepEqual(got, []Key{ebiten.KeyA, ebiten.KeyR}) {
		t.Errorf("AppendKeysPressed = %v", got)
	}

	s.ClearKey(ebiten.KeyR)
	if s.KeyDown(ebiten.KeyR) || s.KeyPressed(ebiten.KeyR) || s.KeyRepeated(ebiten.KeyR) {
		t.Error("ClearKey left traces of the key")
	}

	// The queries also work through a pointer, as the specification's
	// signatures promise.
	p := &s
	if !p.KeyDown(ebiten.KeyW) || p.KeyPressed(ebiten.KeyW) || p.KeyRepeated(ebiten.KeyW) {
		t.Error("queries through *State disagree")
	}
}

func TestStateModifiers(t *testing.T) {
	tests := []struct {
		name             string
		key              Key
		combined         Key
		shift, ctrl, alt bool
	}{
		{"left shift", ebiten.KeyShiftLeft, ebiten.KeyShift, true, false, false},
		{"right shift", ebiten.KeyShiftRight, ebiten.KeyShift, true, false, false},
		{"left ctrl", ebiten.KeyControlLeft, ebiten.KeyControl, false, true, false},
		{"right ctrl", ebiten.KeyControlRight, ebiten.KeyControl, false, true, false},
		{"left alt", ebiten.KeyAltLeft, ebiten.KeyAlt, false, false, true},
		{"right alt", ebiten.KeyAltRight, ebiten.KeyAlt, false, false, true},
		{"left meta", ebiten.KeyMetaLeft, ebiten.KeyMeta, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var s State
			s.PressKey(tc.key)
			if !s.KeyDown(tc.combined) || !s.KeyPressed(tc.combined) || !s.KeyRepeated(tc.combined) {
				t.Errorf("combined key %s does not follow %s", tc.combined, tc.key)
			}
			if s.Shift != tc.shift || s.Ctrl != tc.ctrl || s.Alt != tc.alt {
				t.Errorf("flags = %v %v %v, want %v %v %v", s.Shift, s.Ctrl, s.Alt, tc.shift, tc.ctrl, tc.alt)
			}
			s.ReleaseKey(tc.key)
			if s.KeyDown(tc.combined) || !s.KeyReleased(tc.combined) {
				t.Errorf("release of %s not visible through %s", tc.key, tc.combined)
			}
			if s.Shift || s.Ctrl || s.Alt {
				t.Error("modifier flag still set after release")
			}
		})
	}

	// A combined key addresses the left variant when written.
	var s State
	s.PressKey(ebiten.KeyShift)
	if !s.KeyDown(ebiten.KeyShiftLeft) || s.KeyDown(ebiten.KeyShiftRight) || !s.Shift {
		t.Error("PressKey(KeyShift) should press ShiftLeft")
	}
	s.HoldKey(ebiten.KeyShiftRight)
	s.ClearKey(ebiten.KeyShift)
	if s.KeyDown(ebiten.KeyShift) || s.Shift {
		t.Error("ClearKey(KeyShift) should clear both variants")
	}

	// Both variants held: releasing one keeps the flag.
	s = State{}
	s.HoldKey(ebiten.KeyControlLeft)
	s.HoldKey(ebiten.KeyControlRight)
	s.ReleaseKey(ebiten.KeyControlLeft)
	if !s.Ctrl || !s.KeyDown(ebiten.KeyControl) {
		t.Error("Ctrl should stay set while the right variant is held")
	}
}

func fullState() State {
	s := State{
		X: 120, Y: 80, Wheel: -2,
		Down:        [NumButtons]bool{true, false, true},
		Pressed:     [NumButtons]bool{true, false, false},
		Released:    [NumButtons]bool{false, true, false},
		DoubleClick: true,
		Chars:       []rune("ab"),
		W:           1600, H: 900, Scale: 2, Frame: 42, Dt: NominalDt,
	}
	s.PressKey(ebiten.KeyR)
	s.HoldKey(ebiten.KeyShiftLeft)
	s.ReleaseKey(ebiten.KeyEscape)
	return s
}

func TestWithoutMouse(t *testing.T) {
	orig := fullState()
	got := orig.WithoutMouse()

	if got.X != OffscreenCoord || got.Y != OffscreenCoord {
		t.Errorf("cursor = %v,%v, want off-screen", got.X, got.Y)
	}
	if got.Wheel != 0 || got.DoubleClick || !got.MouseTaken {
		t.Errorf("wheel %v, double click %v, taken %v", got.Wheel, got.DoubleClick, got.MouseTaken)
	}
	var none [NumButtons]bool
	if got.Down != none || got.Pressed != none || got.Released != none {
		t.Errorf("buttons survive: %v %v %v", got.Down, got.Pressed, got.Released)
	}
	// Keyboard and frame data are untouched.
	if !got.KeyPressed(ebiten.KeyR) || !got.Shift || !got.KeyReleased(ebiten.KeyEscape) || got.KeysTaken {
		t.Error("WithoutMouse must not touch the keyboard")
	}
	if string(got.Chars) != "ab" || got.W != 1600 || got.H != 900 || got.Scale != 2 || got.Frame != 42 || got.Dt != NominalDt {
		t.Error("WithoutMouse must not touch text or frame data")
	}
	// The original is a value and stays as it was.
	if orig.X != 120 || !orig.Down[Left] || orig.MouseTaken || !orig.DoubleClick {
		t.Error("WithoutMouse modified its receiver")
	}
}

func TestWithoutKeys(t *testing.T) {
	orig := fullState()
	got := orig.WithoutKeys()

	for _, k := range []Key{ebiten.KeyR, ebiten.KeyShiftLeft, ebiten.KeyShift, ebiten.KeyEscape} {
		if got.KeyDown(k) || got.KeyPressed(k) || got.KeyReleased(k) || got.KeyRepeated(k) {
			t.Errorf("key %s survives WithoutKeys", k)
		}
	}
	if got.Chars != nil || !got.KeysTaken {
		t.Errorf("chars %q, taken %v", string(got.Chars), got.KeysTaken)
	}
	if len(got.AppendKeysDown(nil)) != 0 {
		t.Error("no key may be listed as down")
	}
	// Modifier flags are kept on purpose (they qualify mouse actions) and
	// the mouse is untouched.
	if !got.Shift {
		t.Error("WithoutKeys must keep the modifier flags")
	}
	if got.X != 120 || !got.Down[Left] || !got.Pressed[Left] || !got.DoubleClick || got.Wheel != -2 || got.MouseTaken {
		t.Error("WithoutKeys must not touch the mouse")
	}
	if !orig.KeyPressed(ebiten.KeyR) || string(orig.Chars) != "ab" || orig.KeysTaken {
		t.Error("WithoutKeys modified its receiver")
	}

	// The two filters chain.
	both := orig.WithoutMouse().WithoutKeys()
	if !both.MouseTaken || !both.KeysTaken || both.Down[Left] || both.KeyDown(ebiten.KeyR) {
		t.Error("chained filters lost something")
	}
}

func TestCursorInside(t *testing.T) {
	tests := []struct {
		x, y  float32
		taken bool
		want  bool
	}{
		{0, 0, false, true},
		{799.5, 599.5, false, true},
		{400, 300, false, true},
		{800, 300, false, false}, // the right and bottom edges are outside
		{400, 600, false, false},
		{-0.5, 300, false, false},
		{400, -1, false, false},
		{-1000, -1000, false, false},
		{400, 300, true, false},
	}
	for _, tc := range tests {
		s := State{X: tc.x, Y: tc.y, W: 800, H: 600, MouseTaken: tc.taken}
		if got := s.CursorInside(); got != tc.want {
			t.Errorf("cursor %v,%v taken=%v: CursorInside = %v, want %v", tc.x, tc.y, tc.taken, got, tc.want)
		}
	}
	inside := State{X: 10, Y: 10, W: 800, H: 600}
	if !inside.CursorInside() || inside.WithoutMouse().CursorInside() {
		t.Error("WithoutMouse must take the cursor off the screen")
	}
}
