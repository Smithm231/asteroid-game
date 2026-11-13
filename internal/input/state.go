package input

import (
	"math/bits"

	"github.com/hajimehoshi/ebiten/v2"
)

// Button identifies a mouse button. It indexes the Down, Pressed and Released
// arrays of [State].
type Button int

// Mouse buttons.
const (
	Left Button = iota
	Right
	Middle
)

// NumButtons is the number of mouse buttons a State tracks.
const NumButtons = 3

// String returns the lower-case name of the button as used in scripts.
func (b Button) String() string {
	switch b {
	case Left:
		return "left"
	case Right:
		return "right"
	case Middle:
		return "middle"
	}
	return "unknown"
}

// Key is a physical keyboard key (US layout positions), identical to
// Ebitengine's key type so that ebiten.KeyEscape and friends can be used
// directly.
type Key = ebiten.Key

// OffscreenCoord is the cursor coordinate reported by [State.WithoutMouse]:
// far outside any screen so that no hit test can succeed.
const OffscreenCoord float32 = -1e6

// NominalDt is the frame time, in seconds, of the nominal 60 frames per
// second. Scripted playback always reports exactly this value.
const NominalDt float32 = 1.0 / 60

// State is the complete input snapshot of one frame. It is produced by a
// [Source], is a plain value (copy it freely) and is never mutated by the
// source afterwards.
type State struct {
	// X and Y are the cursor position in points, relative to the top-left
	// corner of the screen. They can lie outside the screen.
	X, Y float32
	// Wheel is the vertical wheel movement of this frame; positive values
	// scroll away from the user.
	Wheel float32
	// Down reports the buttons currently held; Pressed and Released are the
	// edges of this frame. All three are indexed by Button.
	Down, Pressed, Released [NumButtons]bool
	// DoubleClick is true on the frame in which the second press of a left
	// double click happens (Pressed[Left] is true on the same frame).
	DoubleClick bool
	// Shift, Ctrl and Alt report whether either variant of the modifier is
	// held.
	Shift, Ctrl, Alt bool
	// Chars holds the text typed this frame, already translated by the
	// keyboard layout. It is nil on most frames and never reused by the
	// source, so it may be retained.
	Chars []rune
	// W and H are the screen size in points.
	W, H float32
	// Scale is the number of device pixels per point.
	Scale float32
	// Frame counts the polls of the source; the first poll reports 1.
	Frame int64
	// Dt is the time since the previous frame in seconds (1/60 nominal).
	Dt float32

	// MouseTaken is set by WithoutMouse: a layer above owns the mouse this
	// frame. Code that reacts to the bare cursor position (edge scrolling,
	// hover highlights) must stay idle while it is set.
	MouseTaken bool
	// KeysTaken is set by WithoutKeys: a layer above owns the keyboard.
	KeysTaken bool

	keysDown     keySet
	keysPressed  keySet
	keysReleased keySet
	keysRepeated keySet
}

// KeyDown reports whether k is currently held. The combined modifier keys
// ebiten.KeyShift, KeyControl, KeyAlt and KeyMeta match either variant.
func (s State) KeyDown(k Key) bool { return s.keysDown.query(k) }

// KeyPressed reports whether k went down this frame. It is a pure edge: it is
// true for one frame only, however long the key stays held. Use KeyRepeated
// for text-editing style auto-repeat.
func (s State) KeyPressed(k Key) bool { return s.keysPressed.query(k) }

// KeyReleased reports whether k went up this frame.
func (s State) KeyReleased(k Key) bool { return s.keysReleased.query(k) }

// KeyRepeated reports whether k went down this frame or, while it stays held,
// fires an auto-repeat this frame: first after KeyRepeatDelay, then every
// KeyRepeatInterval.
func (s State) KeyRepeated(k Key) bool { return s.keysRepeated.query(k) }

// AppendKeysDown appends every key currently held to dst, in ascending key
// order, and returns the extended slice. Combined modifier keys are never
// listed; their left/right variants are.
func (s State) AppendKeysDown(dst []Key) []Key { return s.keysDown.appendTo(dst) }

// AppendKeysPressed appends every key that went down this frame to dst, in
// ascending key order, and returns the extended slice.
func (s State) AppendKeysPressed(dst []Key) []Key { return s.keysPressed.appendTo(dst) }

// CursorInside reports whether the cursor is over the screen and the mouse has
// not been taken by a layer above. Code that reacts to the bare cursor
// position, such as edge scrolling, should check it first: the cursor of a
// real device can be outside the window, and WithoutMouse parks it far away.
func (s State) CursorInside() bool {
	return !s.MouseTaken && s.X >= 0 && s.Y >= 0 && s.X < s.W && s.Y < s.H
}

// WithoutMouse returns a copy of s in which the mouse is neutralised: the
// cursor sits at (OffscreenCoord, OffscreenCoord), no button is down, there
// are no edges, no double click and no wheel movement, and MouseTaken is set.
// A UI layer that owns the mouse hands this copy to the layers beneath it.
// Keyboard state, including the modifier flags, is untouched.
func (s State) WithoutMouse() State {
	s.X, s.Y = OffscreenCoord, OffscreenCoord
	s.Wheel = 0
	s.Down = [NumButtons]bool{}
	s.Pressed = [NumButtons]bool{}
	s.Released = [NumButtons]bool{}
	s.DoubleClick = false
	s.MouseTaken = true
	return s
}

// WithoutKeys returns a copy of s in which the keyboard is neutralised: no
// key is down, pressed, released or repeating, no text was typed, and
// KeysTaken is set. A UI layer that owns the keyboard (a focused text field,
// for instance) hands this copy to the layers beneath it.
//
// The Shift, Ctrl and Alt flags are deliberately kept: they qualify mouse
// actions such as shift-click, which still belong to whoever owns the mouse.
func (s State) WithoutKeys() State {
	s.keysDown = keySet{}
	s.keysPressed = keySet{}
	s.keysReleased = keySet{}
	s.keysRepeated = keySet{}
	s.Chars = nil
	s.KeysTaken = true
	return s
}

// PressKey marks k as having gone down this frame (down, pressed and
// repeated). Together with HoldKey, RepeatKey, ReleaseKey and ClearKey it
// lets tests and UI layers build or edit a State by hand.
func (s *State) PressKey(k Key) { s.setKey(k, true, true, false, true) }

// HoldKey marks k as held without any edge this frame.
func (s *State) HoldKey(k Key) { s.setKey(k, true, false, false, false) }

// RepeatKey marks k as held and firing an auto-repeat this frame.
func (s *State) RepeatKey(k Key) { s.setKey(k, true, false, false, true) }

// ReleaseKey marks k as having gone up this frame.
func (s *State) ReleaseKey(k Key) { s.setKey(k, false, false, true, false) }

// ClearKey removes every trace of k from the state, which is how a layer
// consumes a single key (a handled Escape, for instance) before passing the
// state on.
func (s *State) ClearKey(k Key) { s.setKey(k, false, false, false, false) }

// setKey writes all four bits of k. A combined modifier key addresses its left
// variant (and clears the right one when the key is cleared).
func (s *State) setKey(k Key, down, pressed, released, repeated bool) {
	if l, r, ok := modifierPair(k); ok {
		if !down && !pressed && !released && !repeated {
			s.setKey(r, false, false, false, false)
		}
		k = l
	}
	if !validKey(k) {
		return
	}
	s.keysDown.assign(k, down)
	s.keysPressed.assign(k, pressed)
	s.keysReleased.assign(k, released)
	s.keysRepeated.assign(k, repeated)
	switch k {
	case ebiten.KeyShiftLeft, ebiten.KeyShiftRight:
		s.Shift = s.keysDown.query(ebiten.KeyShift)
	case ebiten.KeyControlLeft, ebiten.KeyControlRight:
		s.Ctrl = s.keysDown.query(ebiten.KeyControl)
	case ebiten.KeyAltLeft, ebiten.KeyAltRight:
		s.Alt = s.keysDown.query(ebiten.KeyAlt)
	}
}

// syncModifiers derives the Shift, Ctrl and Alt flags from the held keys.
func (s *State) syncModifiers() {
	s.Shift = s.keysDown.query(ebiten.KeyShift)
	s.Ctrl = s.keysDown.query(ebiten.KeyControl)
	s.Alt = s.keysDown.query(ebiten.KeyAlt)
}

// numKeys is the number of distinct key codes, combined modifier keys
// included.
const numKeys = int(ebiten.KeyMax) + 1

// keyWords is the number of 64-bit words needed for one bit per key.
const keyWords = (numKeys + 63) / 64

// keySet is a bitset with one bit per key. Combined modifier keys are never
// stored; query resolves them to their variants.
type keySet [keyWords]uint64

// validKey reports whether k is a storable key code: in range and not one of
// the combined modifier keys.
func validKey(k Key) bool {
	if k < 0 || int(k) >= numKeys {
		return false
	}
	_, _, combined := modifierPair(k)
	return !combined
}

// modifierPair returns the left and right variants of a combined modifier key.
func modifierPair(k Key) (left, right Key, ok bool) {
	switch k {
	case ebiten.KeyShift:
		return ebiten.KeyShiftLeft, ebiten.KeyShiftRight, true
	case ebiten.KeyControl:
		return ebiten.KeyControlLeft, ebiten.KeyControlRight, true
	case ebiten.KeyAlt:
		return ebiten.KeyAltLeft, ebiten.KeyAltRight, true
	case ebiten.KeyMeta:
		return ebiten.KeyMetaLeft, ebiten.KeyMetaRight, true
	}
	return 0, 0, false
}

// has reports whether the bit of the plain key k is set.
func (ks *keySet) has(k Key) bool {
	if k < 0 || int(k) >= numKeys {
		return false
	}
	return ks[uint(k)/64]&(1<<(uint(k)%64)) != 0
}

// query is has with combined modifier keys resolved to "either variant".
func (ks *keySet) query(k Key) bool {
	if l, r, ok := modifierPair(k); ok {
		return ks.has(l) || ks.has(r)
	}
	return ks.has(k)
}

// assign sets or clears the bit of k; keys that cannot be stored are ignored.
func (ks *keySet) assign(k Key, on bool) {
	if !validKey(k) {
		return
	}
	if on {
		ks[uint(k)/64] |= 1 << (uint(k) % 64)
	} else {
		ks[uint(k)/64] &^= 1 << (uint(k) % 64)
	}
}

// empty reports whether no bit is set.
func (ks *keySet) empty() bool {
	for _, w := range ks {
		if w != 0 {
			return false
		}
	}
	return true
}

// appendTo appends the keys whose bits are set, in ascending order.
func (ks *keySet) appendTo(dst []Key) []Key {
	for i, w := range ks {
		for w != 0 {
			b := bits.TrailingZeros64(w)
			dst = append(dst, Key(i*64+b))
			w &^= 1 << uint(b)
		}
	}
	return dst
}
