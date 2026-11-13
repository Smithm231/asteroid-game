package input

import "time"

// Timing rules shared by every source.
const (
	// DoubleClickTime is the longest gap between the two presses of a double
	// click.
	DoubleClickTime = 350 * time.Millisecond
	// DoubleClickDist is the farthest, in points, that the second press of a
	// double click may be from the first.
	DoubleClickDist float32 = 4
	// KeyRepeatDelay is how long a key must be held before it starts to
	// auto-repeat.
	KeyRepeatDelay = 400 * time.Millisecond
	// KeyRepeatInterval is the time between two auto-repeats.
	KeyRepeatInterval = 40 * time.Millisecond
)

// Internal time is counted in whole microseconds so that playback at a fixed
// frame time is exact and reproducible.
const (
	nominalFrameMicros   = 16667 // 1/60 s, rounded
	doubleClickMicros    = int64(DoubleClickTime / time.Microsecond)
	repeatDelayMicros    = int64(KeyRepeatDelay / time.Microsecond)
	repeatIntervalMicros = int64(KeyRepeatInterval / time.Microsecond)
)

// tracker derives the time-dependent parts of a State — double clicks and key
// auto-repeat — from the raw per-frame facts. It owns no clock: the caller
// tells it how much time each frame took, which is what makes it testable and
// lets scripted playback count frames instead of reading the wall clock.
type tracker struct {
	now int64 // microseconds elapsed, including the current frame

	// Double-click detection: the last unpaired left press.
	armed          bool
	clickAt        int64
	clickX, clickY float32

	// Key repeat: which keys were held last frame and for how long each key
	// has been held.
	wasDown keySet
	held    [numKeys]int64
}

// advance moves time forward by dtMicros and completes st, whose cursor,
// button and key down/pressed/released fields must already be final: it sets
// DoubleClick and the repeated keys.
func (t *tracker) advance(st *State, dtMicros int64) {
	if dtMicros < 0 {
		dtMicros = 0
	}
	t.now += dtMicros

	st.DoubleClick = t.click(st)

	st.keysRepeated = st.keysPressed
	for i, w := range st.keysDown {
		for b := 0; w != 0; b, w = b+1, w>>1 {
			if w&1 == 0 {
				continue
			}
			k := Key(i*64 + b)
			if st.keysPressed.has(k) || !t.wasDown.has(k) {
				// A fresh press (or a key first seen already held, e.g.
				// after the window regained focus) restarts the delay.
				t.held[k] = 0
				continue
			}
			before := repeatCount(t.held[k])
			t.held[k] += dtMicros
			if repeatCount(t.held[k]) > before {
				st.keysRepeated.assign(k, true)
			}
		}
	}
	t.wasDown = st.keysDown
}

// click updates the double-click state machine with this frame's presses and
// reports whether a left double click completed.
func (t *tracker) click(st *State) bool {
	if st.Pressed[Right] || st.Pressed[Middle] {
		// Another button in between breaks the sequence.
		t.armed = false
	}
	if !st.Pressed[Left] {
		return false
	}
	if t.armed && t.now-t.clickAt <= doubleClickMicros {
		dx, dy := st.X-t.clickX, st.Y-t.clickY
		if dx*dx+dy*dy <= DoubleClickDist*DoubleClickDist {
			// Disarm so that a third quick press starts a new sequence
			// instead of reporting a second double click.
			t.armed = false
			return true
		}
	}
	t.armed = true
	t.clickAt = t.now
	t.clickX, t.clickY = st.X, st.Y
	return false
}

// repeatCount returns how many auto-repeats a key held for the given time has
// fired so far.
func repeatCount(heldMicros int64) int64 {
	if heldMicros < repeatDelayMicros {
		return 0
	}
	return 1 + (heldMicros-repeatDelayMicros)/repeatIntervalMicros
}
