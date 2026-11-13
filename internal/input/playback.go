package input

import "github.com/hajimehoshi/ebiten/v2"

// scriptStartCoord is where the cursor sits before the first step that
// positions it: outside the screen, so nothing is hovered.
const scriptStartCoord float32 = -1000

// frameOp is what a script does during one frame. The zero value is an idle
// frame.
type frameOp struct {
	move    bool
	x, y    float32
	btnDown uint8 // bit b: Button(b) goes down
	btnUp   uint8 // bit b: Button(b) goes up
	keyDown bool  // key goes down
	keyUp   bool  // key goes up
	key     Key
	wheel   float32
	char    rune // typed character, 0 for none
	shot    string
}

// heldKey is a key kept down by a "hold" step.
type heldKey struct {
	key  Key
	left int // frames remaining; negative means until the script ends
}

// ScriptSource is the Source that replays a script. Playback is purely
// frame-driven — one Poll is one frame — so the same script always yields
// the same sequence of States. It never calls into Ebitengine and therefore
// works without a window.
type ScriptSource struct {
	steps []Step
	next  int // index of the next step to expand
	queue []frameOp
	qi    int // index of the next op in queue
	done  bool
	err   error

	frame   int64
	x, y    float32
	buttons [NumButtons]bool
	keys    keySet
	holds   []heldKey
	shots   []string
	trk     tracker
}

// NewScript returns a source that plays the given steps in order. The slice
// is copied. If the script is invalid nothing is played: Err reports the
// problem and Done is true from the start (use Validate or ParseScript to
// check a script beforehand).
func NewScript(steps []Step) *ScriptSource {
	s := &ScriptSource{
		steps: append([]Step(nil), steps...),
		x:     scriptStartCoord,
		y:     scriptStartCoord,
	}
	if err := Validate(s.steps); err != nil {
		s.err = err
		s.done = true
		return s
	}
	s.fill()
	return s
}

// Err reports why the script could not be played, or nil.
func (s *ScriptSource) Err() error { return s.err }

// Frame returns the number of frames polled so far.
func (s *ScriptSource) Frame() int64 { return s.frame }

// Done reports whether the script has finished: every frame of every step has
// been returned by Poll, or a quit step was reached. It becomes true with the
// Poll that delivers the last frame, so a host should still draw that frame
// (and collect its screenshot) before stopping. Polling a finished script
// yields idle frames with everything released.
func (s *ScriptSource) Done() bool {
	return s.done && s.qi >= len(s.queue)
}

// TakeShot returns the name of a screenshot requested by the frame most
// recently polled, removing it from the pending list; ok is false when
// nothing is pending. The host calls it after drawing the frame. A shot step
// occupies an otherwise idle frame that follows all earlier steps, so by the
// time it is reported their effects have been through an update and a draw.
func (s *ScriptSource) TakeShot() (name string, ok bool) {
	if len(s.shots) == 0 {
		return "", false
	}
	name = s.shots[0]
	s.shots = s.shots[1:]
	return name, true
}

// Poll implements Source: it advances the script by one frame.
func (s *ScriptSource) Poll(pixelW, pixelH int, scale float32) State {
	if !(scale > 0) {
		scale = 1
	}
	s.frame++

	// NewScript and the previous Poll have already queued this frame's op.
	prevButtons, prevKeys := s.buttons, s.keys
	var op frameOp
	if s.qi < len(s.queue) {
		op = s.queue[s.qi]
		s.qi++
		s.apply(op)
	} else {
		// The script is over: let go of everything.
		s.buttons = [NumButtons]bool{}
		s.keys = keySet{}
		s.holds = s.holds[:0]
	}

	st := State{
		X:     s.x,
		Y:     s.y,
		Wheel: op.wheel,
		Down:  s.buttons,
		W:     float32(pixelW) / scale,
		H:     float32(pixelH) / scale,
		Scale: scale,
		Frame: s.frame,
		Dt:    NominalDt,
	}
	for b := range s.buttons {
		st.Pressed[b] = s.buttons[b] && !prevButtons[b]
		st.Released[b] = !s.buttons[b] && prevButtons[b]
	}
	st.keysDown = s.keys
	for i := range s.keys {
		st.keysPressed[i] = s.keys[i] &^ prevKeys[i]
		st.keysReleased[i] = prevKeys[i] &^ s.keys[i]
	}
	st.syncModifiers()
	if op.char != 0 {
		st.Chars = []rune{op.char}
	}
	s.trk.advance(&st, nominalFrameMicros)

	if op.shot != "" {
		s.shots = append(s.shots, op.shot)
	}
	// Look ahead so that Done is already accurate for this frame and "hold"
	// steps are armed before the frame they first affect.
	s.fill()
	return st
}

// apply performs one frame of the script on the cursor, buttons and keys.
func (s *ScriptSource) apply(op frameOp) {
	if op.move {
		s.x, s.y = op.x, op.y
	}
	for b := range s.buttons {
		if op.btnUp&(1<<uint(b)) != 0 {
			s.buttons[b] = false
		}
		if op.btnDown&(1<<uint(b)) != 0 {
			s.buttons[b] = true
		}
	}
	if op.keyUp {
		s.keys.assign(op.key, false)
	}
	if op.keyDown {
		s.keys.assign(op.key, true)
	}
	kept := s.holds[:0]
	for _, h := range s.holds {
		if h.left == 0 {
			s.keys.assign(h.key, false)
			continue
		}
		s.keys.assign(h.key, true)
		if h.left > 0 {
			h.left--
		}
		kept = append(kept, h)
	}
	s.holds = kept
}

// fill expands steps until at least one frame is queued or the script ends.
// Steps that take no frame (hold, quit, no-ops) are carried out on the way.
func (s *ScriptSource) fill() {
	for s.qi >= len(s.queue) && !s.done {
		if s.next >= len(s.steps) {
			s.done = true
			break
		}
		st := s.steps[s.next]
		s.next++
		s.queue = s.queue[:0]
		s.qi = 0
		s.expand(st)
	}
}

// expand turns one step into its frames, appended to the queue.
func (s *ScriptSource) expand(st Step) {
	p, err := st.plan()
	if err != nil {
		// Unreachable: NewScript validated every step.
		return
	}
	btn := uint8(1) << uint(p.button)
	push := func(op frameOp) { s.queue = append(s.queue, op) }
	at := func(x, y float32) frameOp { return frameOp{move: true, x: x, y: y} }

	switch p.action {
	case "wait":
		for i := 0; i < st.Wait; i++ {
			push(frameOp{})
		}

	case "move":
		n := max(st.Frames, 1)
		x1, y1 := st.Move[0], st.Move[1]
		for i := 1; i <= n; i++ {
			f := float32(i) / float32(n)
			push(at(lerp(s.x, x1, f), lerp(s.y, y1, f)))
		}

	case "click", "dblclick":
		pos := st.Click
		presses := 1
		if p.action == "dblclick" {
			pos = st.DblClick
			presses = 2
		}
		push(at(pos[0], pos[1]))
		for i := 0; i < presses; i++ {
			down, up := at(pos[0], pos[1]), at(pos[0], pos[1])
			down.btnDown = btn
			up.btnUp = btn
			push(down)
			push(up)
		}

	case "drag":
		n := st.Frames
		if n == 0 {
			n = DefaultDragFrames
		}
		x0, y0, x1, y1 := st.Drag[0], st.Drag[1], st.Drag[2], st.Drag[3]
		push(at(x0, y0))
		down := at(x0, y0)
		down.btnDown = btn
		push(down)
		for i := 1; i <= n; i++ {
			f := float32(i) / float32(n)
			push(at(lerp(x0, x1, f), lerp(y0, y1, f)))
		}
		up := at(x1, y1)
		up.btnUp = btn
		push(up)

	case "press":
		push(at(st.Press[0], st.Press[1]))
		down := at(st.Press[0], st.Press[1])
		down.btnDown = btn
		push(down)

	case "release":
		push(at(st.Release[0], st.Release[1]))
		up := at(st.Release[0], st.Release[1])
		up.btnUp = btn
		push(up)

	case "wheel":
		push(frameOp{wheel: st.Wheel})

	case "key":
		s.pushTap(p.key, max(st.Frames, 1))

	case "hold":
		left := st.Frames
		if left == 0 {
			left = -1
		}
		for i := range s.holds {
			if s.holds[i].key == p.key {
				s.holds[i].left = left
				return
			}
		}
		s.holds = append(s.holds, heldKey{key: p.key, left: left})

	case "text":
		for _, r := range st.Text {
			switch r {
			case '\n':
				s.pushTap(ebiten.KeyEnter, 1)
			case '\t':
				s.pushTap(ebiten.KeyTab, 1)
			case '\b':
				s.pushTap(ebiten.KeyBackspace, 1)
			default:
				push(frameOp{char: r})
			}
		}

	case "shot":
		push(frameOp{shot: st.Shot})

	case "quit":
		s.done = true
	}
}

// pushTap queues a key that goes down, stays down for the given number of
// frames and goes up on the frame after.
func (s *ScriptSource) pushTap(k Key, frames int) {
	s.queue = append(s.queue, frameOp{keyDown: true, key: k})
	for i := 1; i < frames; i++ {
		s.queue = append(s.queue, frameOp{})
	}
	s.queue = append(s.queue, frameOp{keyUp: true, key: k})
}

// lerp interpolates from a to b, returning b exactly at f = 1.
func lerp(a, b, f float32) float32 {
	if f >= 1 {
		return b
	}
	return a + (b-a)*f
}
