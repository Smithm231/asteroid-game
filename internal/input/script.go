package input

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Step is one entry of an input script. Exactly one action field is set per
// step (a step with none is an accepted no-op); Button and Frames modify the
// action. Coordinates are in points. The package documentation lists every
// action with its JSON form and frame count.
type Step struct {
	// Wait idles for this many frames.
	Wait int `json:"wait,omitempty"`
	// Move places the cursor at [x, y]. With Frames > 1 the cursor glides
	// there from its current position.
	Move []float32 `json:"move,omitempty"`
	// Click moves to [x, y], presses on the next frame and releases on the
	// one after.
	Click []float32 `json:"click,omitempty"`
	// DblClick is a click immediately followed by a second press and release
	// at the same [x, y].
	DblClick []float32 `json:"dblclick,omitempty"`
	// Drag presses at [x0, y0], moves to [x1, y1] over Frames frames with
	// the button held, and releases there.
	Drag []float32 `json:"drag,omitempty"`
	// Press moves to [x, y] and presses the button, leaving it held for the
	// following steps.
	Press []float32 `json:"press,omitempty"`
	// Release moves to [x, y] (a held button makes this a drag motion) and
	// releases the button.
	Release []float32 `json:"release,omitempty"`
	// Wheel reports this wheel movement for one frame; positive scrolls away
	// from the user.
	Wheel float32 `json:"wheel,omitempty"`
	// Key taps the named key: down for Frames frames (default 1), then up.
	Key string `json:"key,omitempty"`
	// Hold keeps the named key down during the next Frames frames while the
	// following steps run; without Frames, until the script ends.
	Hold string `json:"hold,omitempty"`
	// Text types the string, one rune per frame.
	Text string `json:"text,omitempty"`
	// Shot asks the host to save a screenshot under this file name once
	// everything before it has been delivered and drawn.
	Shot string `json:"shot,omitempty"`
	// Quit ends the script.
	Quit bool `json:"quit,omitempty"`

	// Button selects the mouse button of Click, DblClick, Drag, Press and
	// Release: "left" (default), "right" or "middle".
	Button string `json:"button,omitempty"`
	// Frames is the duration of Drag, Key, Hold and Move.
	Frames int `json:"frames,omitempty"`
	// Note is free text for the reader of the script; playback ignores it.
	Note string `json:"note,omitempty"`
}

// DefaultDragFrames is the number of frames a drag takes when its step does
// not say.
const DefaultDragFrames = 10

// Wait returns a step that idles for the given number of frames.
func Wait(frames int) Step { return Step{Wait: frames} }

// MoveTo returns a step that places the cursor at (x, y).
func MoveTo(x, y float32) Step { return Step{Move: []float32{x, y}} }

// GlideTo returns a step that moves the cursor to (x, y) over the given
// number of frames.
func GlideTo(x, y float32, frames int) Step {
	return Step{Move: []float32{x, y}, Frames: frames}
}

// ClickAt returns a step that clicks the left button at (x, y).
func ClickAt(x, y float32) Step { return Step{Click: []float32{x, y}} }

// ClickWith returns a step that clicks button b at (x, y).
func ClickWith(b Button, x, y float32) Step {
	return Step{Click: []float32{x, y}, Button: buttonField(b)}
}

// DblClickAt returns a step that double-clicks the left button at (x, y).
func DblClickAt(x, y float32) Step { return Step{DblClick: []float32{x, y}} }

// DragTo returns a step that drags with the left button from (x0, y0) to
// (x1, y1) over the given number of frames.
func DragTo(x0, y0, x1, y1 float32, frames int) Step {
	return Step{Drag: []float32{x0, y0, x1, y1}, Frames: frames}
}

// DragWith is DragTo with an explicit button.
func DragWith(b Button, x0, y0, x1, y1 float32, frames int) Step {
	return Step{Drag: []float32{x0, y0, x1, y1}, Frames: frames, Button: buttonField(b)}
}

// PressAt returns a step that presses button b at (x, y) and keeps it held.
func PressAt(b Button, x, y float32) Step {
	return Step{Press: []float32{x, y}, Button: buttonField(b)}
}

// ReleaseAt returns a step that moves to (x, y) and releases button b.
func ReleaseAt(b Button, x, y float32) Step {
	return Step{Release: []float32{x, y}, Button: buttonField(b)}
}

// Scroll returns a step that turns the wheel by delta for one frame.
func Scroll(delta float32) Step { return Step{Wheel: delta} }

// Tap returns a step that presses and releases k.
func Tap(k Key) Step { return Step{Key: KeyName(k)} }

// TapFor returns a step that holds k down for the given number of frames and
// then releases it, blocking the script meanwhile.
func TapFor(k Key, frames int) Step { return Step{Key: KeyName(k), Frames: frames} }

// Hold returns a step that keeps k down during the next frames frames while
// later steps run; frames <= 0 holds it until the script ends.
func Hold(k Key, frames int) Step {
	if frames < 0 {
		frames = 0
	}
	return Step{Hold: KeyName(k), Frames: frames}
}

// Type returns a step that types text, one rune per frame.
func Type(text string) Step { return Step{Text: text} }

// Shot returns a step that asks the host for a screenshot named name.
func Shot(name string) Step { return Step{Shot: name} }

// Quit returns the step that ends the script.
func Quit() Step { return Step{Quit: true} }

// buttonField returns the script spelling of b, leaving the default button
// implicit.
func buttonField(b Button) string {
	if b == Left {
		return ""
	}
	return b.String()
}

// stepPlan is a validated step with names resolved.
type stepPlan struct {
	action string // "" for a no-op
	button Button
	key    Key
}

// plan validates the step and resolves its key and button names.
func (st Step) plan() (stepPlan, error) {
	var p stepPlan
	var actions []string
	add := func(set bool, name string) {
		if set {
			actions = append(actions, name)
		}
	}
	add(st.Wait != 0, "wait")
	add(st.Move != nil, "move")
	add(st.Click != nil, "click")
	add(st.DblClick != nil, "dblclick")
	add(st.Drag != nil, "drag")
	add(st.Press != nil, "press")
	add(st.Release != nil, "release")
	add(st.Wheel != 0, "wheel")
	add(st.Key != "", "key")
	add(st.Hold != "", "hold")
	add(st.Text != "", "text")
	add(st.Shot != "", "shot")
	add(st.Quit, "quit")

	if len(actions) > 1 {
		return p, fmt.Errorf("more than one action in a step (%s)", strings.Join(actions, ", "))
	}
	if len(actions) == 1 {
		p.action = actions[0]
	}

	coords := func(name string, v []float32, n int) error {
		if len(v) != n {
			return fmt.Errorf("%q needs %d numbers, got %d", name, n, len(v))
		}
		return nil
	}
	var err error
	switch p.action {
	case "wait":
		if st.Wait < 0 {
			err = fmt.Errorf(`"wait" must not be negative, got %d`, st.Wait)
		}
	case "move":
		err = coords("move", st.Move, 2)
	case "click":
		err = coords("click", st.Click, 2)
	case "dblclick":
		err = coords("dblclick", st.DblClick, 2)
	case "drag":
		err = coords("drag", st.Drag, 4)
	case "press":
		err = coords("press", st.Press, 2)
	case "release":
		err = coords("release", st.Release, 2)
	case "key":
		p.key, err = ParseKey(st.Key)
	case "hold":
		p.key, err = ParseKey(st.Hold)
	}
	if err != nil {
		return p, err
	}

	if st.Button != "" {
		switch p.action {
		case "click", "dblclick", "drag", "press", "release":
		default:
			return p, fmt.Errorf(`"button" only applies to click, dblclick, drag, press and release`)
		}
	}
	if p.button, err = ParseButton(st.Button); err != nil {
		return p, err
	}
	if st.Frames < 0 {
		return p, fmt.Errorf(`"frames" must not be negative, got %d`, st.Frames)
	}
	if st.Frames != 0 {
		switch p.action {
		case "drag", "key", "hold", "move":
		default:
			return p, fmt.Errorf(`"frames" only applies to drag, key, hold and move`)
		}
	}
	// The combined modifier keys are played back as their left variants.
	if l, _, ok := modifierPair(p.key); ok {
		p.key = l
	}
	return p, nil
}

// Validate reports the first problem in the step, or nil.
func (st Step) Validate() error {
	_, err := st.plan()
	return err
}

// Validate checks every step of a script and reports the first problem,
// naming the offending step by its position (starting at 1).
func Validate(steps []Step) error {
	for i, st := range steps {
		if err := st.Validate(); err != nil {
			return fmt.Errorf("input: script step %d: %w", i+1, err)
		}
	}
	return nil
}

// ParseScript decodes a script from JSON: an array of step objects as listed
// in the package documentation. Unknown fields and invalid steps are errors.
func ParseScript(data []byte) ([]Step, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var steps []Step
	if err := dec.Decode(&steps); err != nil {
		return nil, fmt.Errorf("input: parsing script: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("input: parsing script: unexpected data after the step list")
	}
	if err := Validate(steps); err != nil {
		return nil, err
	}
	return steps, nil
}

// LoadScript reads and parses a JSON script file.
func LoadScript(path string) ([]Step, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("input: loading script: %w", err)
	}
	steps, err := ParseScript(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return steps, nil
}
