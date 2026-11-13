package input

import (
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// press describes one mouse press fed to the tracker on a given frame.
type press struct {
	frame  int64
	x, y   float32
	button Button
}

// doubleClicks plays the presses through a tracker whose clock is a plain
// frame counter (frameMicros per frame) and returns the frames on which a
// double click was reported.
func doubleClicks(presses []press, frameMicros int64) []int64 {
	var trk tracker
	var out []int64
	byFrame := map[int64]press{}
	var last int64
	for _, p := range presses {
		byFrame[p.frame] = p
		last = max(last, p.frame)
	}
	for f := int64(1); f <= last; f++ {
		var st State
		if p, ok := byFrame[f]; ok {
			st.X, st.Y = p.x, p.y
			st.Down[p.button] = true
			st.Pressed[p.button] = true
		}
		trk.advance(&st, frameMicros)
		if st.DoubleClick {
			out = append(out, f)
		}
	}
	return out
}

func TestDoubleClick(t *testing.T) {
	const us = nominalFrameMicros // 60 frames per second
	tests := []struct {
		name    string
		presses []press
		dt      int64
		want    []int64
	}{
		{"single press", []press{{1, 10, 10, Left}}, us, nil},
		{"quick second press", []press{{1, 10, 10, Left}, {5, 10, 10, Left}}, us, []int64{5}},
		{"second press on the next frame", []press{{1, 10, 10, Left}, {2, 10, 10, Left}}, us, []int64{2}},
		// 20 frames = 333.34 ms, 21 frames = 350.007 ms.
		{"just inside the time limit", []press{{1, 10, 10, Left}, {21, 10, 10, Left}}, us, []int64{21}},
		{"just outside the time limit", []press{{1, 10, 10, Left}, {22, 10, 10, Left}}, us, nil},
		{"exactly 350 ms", []press{{1, 0, 0, Left}, {36, 0, 0, Left}}, 10_000, []int64{36}},
		{"one frame over 350 ms", []press{{1, 0, 0, Left}, {37, 0, 0, Left}}, 10_000, nil},
		{"exactly 4 points away", []press{{1, 10, 10, Left}, {5, 14, 10, Left}}, us, []int64{5}},
		{"diagonal within 4 points", []press{{1, 10, 10, Left}, {5, 12.5, 13, Left}}, us, []int64{5}},
		{"diagonal beyond 4 points", []press{{1, 10, 10, Left}, {5, 13, 13, Left}}, us, nil},
		{"too far", []press{{1, 10, 10, Left}, {5, 14.5, 10, Left}}, us, nil},
		{"right button never double clicks", []press{{1, 10, 10, Right}, {3, 10, 10, Right}}, us, nil},
		{"middle button never double clicks", []press{{1, 10, 10, Middle}, {3, 10, 10, Middle}}, us, nil},
		{"right press in between breaks it", []press{{1, 10, 10, Left}, {3, 10, 10, Right}, {5, 10, 10, Left}}, us, nil},
		{"triple click is one double click", []press{{1, 10, 10, Left}, {4, 10, 10, Left}, {7, 10, 10, Left}}, us, []int64{4}},
		{"four clicks are two double clicks", []press{{1, 5, 5, Left}, {4, 5, 5, Left}, {7, 5, 5, Left}, {10, 5, 5, Left}}, us, []int64{4, 10}},
		{"slow then quick", []press{{1, 5, 5, Left}, {40, 5, 5, Left}, {45, 5, 5, Left}}, us, []int64{45}},
		{"far press re-arms at the new spot", []press{{1, 5, 5, Left}, {4, 50, 50, Left}, {7, 50, 50, Left}}, us, []int64{7}},
		// At 144 Hz the same wall-clock rule spans more frames.
		{"144 Hz inside", []press{{1, 5, 5, Left}, {51, 5, 5, Left}}, 6_944, []int64{51}},
		{"144 Hz outside", []press{{1, 5, 5, Left}, {52, 5, 5, Left}}, 6_944, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := doubleClicks(tc.presses, tc.dt)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("double clicks on frames %v, want %v", got, tc.want)
			}
		})
	}
}

// repeats holds key k from frame 1 for the given number of frames and returns
// the frames on which KeyRepeated was true.
func repeats(frames int, frameMicros int64) []int {
	const k = ebiten.KeyBackspace
	var trk tracker
	var out []int
	for f := 1; f <= frames; f++ {
		var st State
		st.keysDown.assign(k, true)
		if f == 1 {
			st.keysPressed.assign(k, true)
		}
		trk.advance(&st, frameMicros)
		if st.KeyRepeated(k) {
			out = append(out, f)
		}
		if st.KeyPressed(k) != (f == 1) {
			panic("tracker changed the pressed edge")
		}
	}
	return out
}

func TestKeyRepeat(t *testing.T) {
	tests := []struct {
		name   string
		frames int
		dt     int64
		want   []int
	}{
		// Press on frame 1, so "n frames held" is frame n+1.
		// 24 frames = 400.008 ms is the first repeat; then every 40 ms.
		{"60 Hz", 38, nominalFrameMicros, []int{1, 25, 28, 30, 33, 35, 37}},
		{"short hold never repeats", 24, nominalFrameMicros, []int{1}},
		// 10 ms frames: delay after exactly 40 frames, then every 4 frames.
		{"100 Hz", 54, 10_000, []int{1, 41, 45, 49, 53}},
		// Frames longer than the interval repeat once per frame at most.
		{"10 Hz", 8, 100_000, []int{1, 5, 6, 7, 8}},
		{"2 Hz", 4, 500_000, []int{1, 2, 3, 4}},
		{"time standing still", 50, 0, []int{1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := repeats(tc.frames, tc.dt)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("repeats on frames %v, want %v", got, tc.want)
			}
		})
	}
}

func TestKeyRepeatRestartsAndIsPerKey(t *testing.T) {
	const a, b = ebiten.KeyA, ebiten.KeyB
	var trk tracker
	// A is held throughout; B is held from frame 10, let go on frame 40 and
	// pressed again on frame 41.
	var gotA, gotB []int
	wasA, wasB := false, false
	for f := 1; f <= 70; f++ {
		aDown, bDown := true, f >= 10 && f != 40
		var st State
		if aDown {
			st.keysDown.assign(a, true)
			if !wasA {
				st.keysPressed.assign(a, true)
			}
		}
		if bDown {
			st.keysDown.assign(b, true)
			if !wasB {
				st.keysPressed.assign(b, true)
			}
		}
		wasA, wasB = aDown, bDown
		trk.advance(&st, nominalFrameMicros)
		if st.KeyRepeated(a) {
			gotA = append(gotA, f)
		}
		if st.KeyRepeated(b) {
			gotB = append(gotB, f)
		}
	}
	wantA := []int{1, 25, 28, 30, 33, 35, 37, 40, 42, 45, 47, 49, 52, 54, 57, 59, 61, 64, 66, 69}
	// B: pressed on 10 (repeat from 34), released on 40, pressed again on 41
	// (delay restarts: next repeat 24 frames later, on 65).
	wantB := []int{10, 34, 37, 39, 41, 65, 68, 70}
	if !reflect.DeepEqual(gotA, wantA) {
		t.Errorf("A repeats on %v, want %v", gotA, wantA)
	}
	if !reflect.DeepEqual(gotB, wantB) {
		t.Errorf("B repeats on %v, want %v", gotB, wantB)
	}
}

// A key that is first seen already held (no press edge, e.g. after the window
// regains focus) starts its delay then, instead of repeating at once.
func TestKeyRepeatWithoutPressEdge(t *testing.T) {
	const k = ebiten.KeyW
	var trk tracker
	// Hold and release once so that stale hold time exists for the key.
	for f := 1; f <= 40; f++ {
		var st State
		st.keysDown.assign(k, true)
		trk.advance(&st, nominalFrameMicros)
	}
	var idle State
	trk.advance(&idle, nominalFrameMicros)

	var got []int
	for f := 1; f <= 26; f++ {
		var st State
		st.keysDown.assign(k, true)
		trk.advance(&st, nominalFrameMicros)
		if st.KeyRepeated(k) {
			got = append(got, f)
		}
	}
	if want := []int{25}; !reflect.DeepEqual(got, want) {
		t.Errorf("repeats on %v, want %v", got, want)
	}
}

func TestRepeatCount(t *testing.T) {
	tests := []struct {
		held int64
		want int64
	}{
		{0, 0}, {399_999, 0}, {400_000, 1}, {439_999, 1}, {440_000, 2}, {480_000, 3}, {1_000_000, 16},
	}
	for _, tc := range tests {
		if got := repeatCount(tc.held); got != tc.want {
			t.Errorf("repeatCount(%d) = %d, want %d", tc.held, got, tc.want)
		}
	}
}
