package input

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// describe renders the parts of a State that scripts drive as one short
// line: "x,y" followed by tokens.
//
//	L+ L L-      left button pressed / held / released (R, M likewise)
//	dbl          double click
//	+K K ~K -K   key pressed / held / auto-repeating / released
//	shift ctrl alt
//	wheel=n      wheel movement
//	'c'          typed character
func describe(st State) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%g,%g", st.X, st.Y)
	for i, name := range []string{"L", "R", "M"} {
		switch {
		case st.Pressed[i]:
			b.WriteString(" " + name + "+")
		case st.Released[i]:
			b.WriteString(" " + name + "-")
		case st.Down[i]:
			b.WriteString(" " + name)
		}
		if st.Pressed[i] && !st.Down[i] || st.Released[i] && st.Down[i] {
			b.WriteString("?!") // edges must agree with the level
		}
	}
	if st.DoubleClick {
		b.WriteString(" dbl")
	}
	for k := Key(0); int(k) < numKeys; k++ {
		if !validKey(k) {
			continue
		}
		switch {
		case st.KeyPressed(k):
			b.WriteString(" +" + KeyName(k))
		case st.KeyReleased(k):
			b.WriteString(" -" + KeyName(k))
		case st.KeyRepeated(k):
			b.WriteString(" ~" + KeyName(k))
		case st.KeyDown(k):
			b.WriteString(" " + KeyName(k))
		}
	}
	if st.Shift {
		b.WriteString(" shift")
	}
	if st.Ctrl {
		b.WriteString(" ctrl")
	}
	if st.Alt {
		b.WriteString(" alt")
	}
	if st.Wheel != 0 {
		fmt.Fprintf(&b, " wheel=%g", st.Wheel)
	}
	for _, r := range st.Chars {
		fmt.Fprintf(&b, " %q", r)
	}
	return b.String()
}

// play runs a script to its end and returns one line per frame. A frame that
// reports a screenshot gets " shot=<name>" appended, and the frame on which
// Done turns true gets " done". extra idle frames are polled after the end.
func play(t *testing.T, steps []Step, extra int) []string {
	t.Helper()
	src := NewScript(steps)
	if err := src.Err(); err != nil {
		t.Fatalf("NewScript: %v", err)
	}
	var out []string
	wasDone := src.Done()
	for i := 0; i < 10_000; i++ {
		if src.Done() && extra == 0 {
			return out
		}
		if src.Done() {
			extra--
		}
		st := src.Poll(1600, 900, 1)
		line := describe(st)
		if st.Frame != int64(len(out)+1) || src.Frame() != st.Frame {
			t.Fatalf("frame %d reported as %d (source says %d)", len(out)+1, st.Frame, src.Frame())
		}
		for {
			name, ok := src.TakeShot()
			if !ok {
				break
			}
			line += " shot=" + name
		}
		if src.Done() && !wasDone {
			line += " done"
			wasDone = true
		}
		out = append(out, line)
	}
	t.Fatal("script did not finish within 10000 frames")
	return nil
}

func checkFrames(t *testing.T, got, want []string) {
	t.Helper()
	if reflect.DeepEqual(got, want) {
		return
	}
	var b strings.Builder
	n := max(len(got), len(want))
	for i := 0; i < n; i++ {
		g, w := "<none>", "<none>"
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		mark := "  "
		if g != w {
			mark = "!="
		}
		fmt.Fprintf(&b, "  frame %2d %s got %-34q want %q\n", i+1, mark, g, w)
	}
	t.Errorf("frames differ (%d got, %d want):\n%s", len(got), len(want), b.String())
}

func TestPlaybackSteps(t *testing.T) {
	tests := []struct {
		name  string
		steps []Step
		want  []string
	}{
		{
			// The cursor arrives one frame before the press; press and
			// release are on consecutive frames.
			name:  "click",
			steps: []Step{ClickAt(100, 200)},
			want: []string{
				"100,200",
				"100,200 L+",
				"100,200 L- done",
			},
		},
		{
			name:  "right and middle click",
			steps: []Step{ClickWith(Right, 10, 20), ClickWith(Middle, 30, 40)},
			want: []string{
				"10,20",
				"10,20 R+",
				"10,20 R-",
				"30,40",
				"30,40 M+",
				"30,40 M- done",
			},
		},
		{
			name:  "double click",
			steps: []Step{DblClickAt(50, 60)},
			want: []string{
				"50,60",
				"50,60 L+",
				"50,60 L-",
				"50,60 L+ dbl",
				"50,60 L- done",
			},
		},
		{
			name:  "move then wait",
			steps: []Step{MoveTo(640, 360), Wait(2)},
			want: []string{
				"640,360",
				"640,360",
				"640,360 done",
			},
		},
		{
			name:  "glide",
			steps: []Step{MoveTo(0, 0), GlideTo(100, 40, 4)},
			want: []string{
				"0,0",
				"25,10",
				"50,20",
				"75,30",
				"100,40 done",
			},
		},
		{
			// Move, press at the start, the given number of held moves
			// ending exactly on the target, release there.
			name:  "drag",
			steps: []Step{DragTo(10, 10, 50, 30, 4)},
			want: []string{
				"10,10",
				"10,10 L+",
				"20,15 L",
				"30,20 L",
				"40,25 L",
				"50,30 L",
				"50,30 L- done",
			},
		},
		{
			name:  "right drag",
			steps: []Step{DragWith(Right, 0, 0, 0, 10, 2)},
			want: []string{
				"0,0",
				"0,0 R+",
				"0,5 R",
				"0,10 R",
				"0,10 R- done",
			},
		},
		{
			// press/release let a script do things (here: a screenshot)
			// while the button is held.
			name:  "press, shot, release",
			steps: []Step{PressAt(Left, 10, 10), GlideTo(30, 10, 2), Shot("mid.png"), ReleaseAt(Left, 40, 10)},
			want: []string{
				"10,10",
				"10,10 L+",
				"20,10 L",
				"30,10 L",
				"30,10 L shot=mid.png",
				"40,10 L",
				"40,10 L- done",
			},
		},
		{
			name:  "wheel",
			steps: []Step{MoveTo(5, 5), Scroll(-3), Scroll(1.5)},
			want: []string{
				"5,5",
				"5,5 wheel=-3",
				"5,5 wheel=1.5 done",
			},
		},
		{
			name:  "key taps",
			steps: []Step{Tap(ebiten.KeyR), Tap(ebiten.KeyR), Tap(ebiten.KeyEscape)},
			want: []string{
				"-1000,-1000 +R",
				"-1000,-1000 -R",
				"-1000,-1000 +R",
				"-1000,-1000 -R",
				"-1000,-1000 +Escape",
				"-1000,-1000 -Escape done",
			},
		},
		{
			name:  "key held for three frames",
			steps: []Step{TapFor(ebiten.KeyW, 3)},
			want: []string{
				"-1000,-1000 +W",
				"-1000,-1000 W",
				"-1000,-1000 W",
				"-1000,-1000 -W done",
			},
		},
		{
			// A hold takes no frame itself and overlaps what follows, which
			// is how a shift-click is written. Down for 4 frames, up on the
			// fifth.
			name:  "hold over a click",
			steps: []Step{Hold(ebiten.KeyShiftLeft, 4), ClickAt(7, 8), Wait(3)},
			want: []string{
				"7,8 +ShiftLeft shift",
				"7,8 L+ ShiftLeft shift",
				"7,8 L- ShiftLeft shift",
				"7,8 ShiftLeft shift",
				"7,8 -ShiftLeft",
				"7,8 done",
			},
		},
		{
			name:  "hold with a combined modifier name presses the left variant",
			steps: []Step{{Hold: "Ctrl", Frames: 1}, Tap(ebiten.KeyS), Wait(1)},
			want: []string{
				"-1000,-1000 +S +ControlLeft ctrl",
				"-1000,-1000 -S -ControlLeft",
				"-1000,-1000 done",
			},
		},
		{
			// Without frames a hold lasts until the script ends; the frame
			// after the end lets go of it.
			name:  "open-ended hold",
			steps: []Step{Hold(ebiten.KeyAltLeft, 0), Wait(2)},
			want: []string{
				"-1000,-1000 +AltLeft alt",
				"-1000,-1000 AltLeft alt done",
				"-1000,-1000 -AltLeft",
				"-1000,-1000",
			},
		},
		{
			name:  "renewing a hold extends it without a new press",
			steps: []Step{Hold(ebiten.KeyQ, 1), Wait(1), Hold(ebiten.KeyQ, 2), Wait(4)},
			want: []string{
				"-1000,-1000 +Q",
				"-1000,-1000 Q",
				"-1000,-1000 Q",
				"-1000,-1000 -Q",
				"-1000,-1000 done",
			},
		},
		{
			name:  "text is typed one rune per frame",
			steps: []Step{Type("héy")},
			want: []string{
				"-1000,-1000 'h'",
				"-1000,-1000 'é'",
				"-1000,-1000 'y' done",
			},
		},
		{
			name:  "control characters in text become key taps",
			steps: []Step{Type("a\n\tb\b")},
			want: []string{
				"-1000,-1000 'a'",
				"-1000,-1000 +Enter",
				"-1000,-1000 -Enter",
				"-1000,-1000 +Tab",
				"-1000,-1000 -Tab",
				"-1000,-1000 'b'",
				"-1000,-1000 +Backspace",
				"-1000,-1000 -Backspace done",
			},
		},
		{
			// The shot is reported on its own idle frame, after the frame
			// that delivered the release.
			name:  "shot after a click",
			steps: []Step{ClickAt(1, 2), Shot("after.png"), Quit()},
			want: []string{
				"1,2",
				"1,2 L+",
				"1,2 L-",
				"1,2 shot=after.png done",
			},
		},
		{
			name:  "consecutive shots take one frame each",
			steps: []Step{Shot("a.png"), Shot("b.png")},
			want: []string{
				"-1000,-1000 shot=a.png",
				"-1000,-1000 shot=b.png done",
			},
		},
		{
			name:  "quit ends the script early",
			steps: []Step{MoveTo(1, 1), Quit(), MoveTo(2, 2), Shot("never.png")},
			want: []string{
				"1,1 done",
			},
		},
		{
			name:  "no-op steps take no frame",
			steps: []Step{{}, {Note: "nothing"}, Wait(0), MoveTo(3, 3), {}, Scroll(0)},
			want: []string{
				"3,3 done",
			},
		},
		{
			// After the end everything is released and the cursor stays.
			name:  "frames after the end are idle",
			steps: []Step{PressAt(Right, 9, 9), Hold(ebiten.KeyW, 0), Wait(1)},
			want: []string{
				"9,9",
				"9,9 R+",
				"9,9 R +W done",
				"9,9 R- -W",
				"9,9",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			extra := 0
			switch tc.name {
			case "open-ended hold", "frames after the end are idle":
				extra = 2
			}
			checkFrames(t, play(t, tc.steps, extra), tc.want)
		})
	}
}

// Two separate click steps on one spot are a double click when they are
// close enough in time, exactly as with a real mouse.
func TestPlaybackClicksCloseInTime(t *testing.T) {
	doubles := func(wait int) int {
		n := 0
		src := NewScript([]Step{ClickAt(40, 40), Wait(wait), ClickAt(40, 40)})
		for !src.Done() {
			if src.Poll(800, 600, 1).DoubleClick {
				n++
			}
		}
		return n
	}
	tests := []struct {
		wait int
		want int
	}{
		// Presses are wait+3 frames apart; 20 frames is the last gap inside
		// 350 ms.
		{0, 1}, {5, 1}, {17, 1}, {18, 0}, {22, 0}, {60, 0},
	}
	for _, tc := range tests {
		if got := doubles(tc.wait); got != tc.want {
			t.Errorf("wait %d: %d double clicks, want %d", tc.wait, got, tc.want)
		}
	}

	// Far apart in space is never a double click.
	src := NewScript([]Step{ClickAt(40, 40), ClickAt(46, 40)})
	for !src.Done() {
		if src.Poll(800, 600, 1).DoubleClick {
			t.Error("clicks 6 points apart reported as a double click")
		}
	}
}

func TestPlaybackKeyRepeat(t *testing.T) {
	src := NewScript([]Step{TapFor(ebiten.KeyBackspace, 34)})
	var repeated, pressed []int64
	for !src.Done() {
		st := src.Poll(800, 600, 1)
		if st.KeyRepeated(ebiten.KeyBackspace) {
			repeated = append(repeated, st.Frame)
		}
		if st.KeyPressed(ebiten.KeyBackspace) {
			pressed = append(pressed, st.Frame)
		}
	}
	if want := []int64{1}; !reflect.DeepEqual(pressed, want) {
		t.Errorf("pressed on frames %v, want %v", pressed, want)
	}
	// 400 ms delay = 24 frames after the press, then every 40 ms.
	if want := []int64{1, 25, 28, 30, 33}; !reflect.DeepEqual(repeated, want) {
		t.Errorf("repeated on frames %v, want %v", repeated, want)
	}
	if got := src.Frame(); got != 35 {
		t.Errorf("a 34-frame tap took %d frames, want 35", got)
	}
}

func TestPlaybackDefaults(t *testing.T) {
	// A drag without frames takes DefaultDragFrames held moves.
	got := play(t, []Step{{Drag: []float32{0, 0, 100, 0}}}, 0)
	if want := DefaultDragFrames + 3; len(got) != want {
		t.Fatalf("default drag took %d frames, want %d", len(got), want)
	}
	if got[2] != "10,0 L" || got[len(got)-2] != "100,0 L" {
		t.Errorf("default drag frames: %q", got)
	}
}

func TestPlaybackScreenAndTiming(t *testing.T) {
	tests := []struct {
		pixelW, pixelH int
		scale          float32
		w, h, wantS    float32
	}{
		{1600, 900, 1, 1600, 900, 1},
		{3200, 1800, 2, 1600, 900, 2},
		{2400, 1350, 1.5, 1600, 900, 1.5},
		{1280, 720, 0, 1280, 720, 1},
	}
	for _, tc := range tests {
		src := NewScript([]Step{MoveTo(640, 360), ClickAt(100, 200)})
		st := src.Poll(tc.pixelW, tc.pixelH, tc.scale)
		// Script coordinates are points whatever the scale.
		if st.X != 640 || st.Y != 360 {
			t.Errorf("scale %v: cursor %v,%v, want 640,360", tc.scale, st.X, st.Y)
		}
		if st.W != tc.w || st.H != tc.h || st.Scale != tc.wantS {
			t.Errorf("scale %v: screen %vx%v scale %v, want %vx%v scale %v", tc.scale, st.W, st.H, st.Scale, tc.w, tc.h, tc.wantS)
		}
		if st.Dt != NominalDt || st.Frame != 1 {
			t.Errorf("scale %v: Dt %v Frame %d", tc.scale, st.Dt, st.Frame)
		}
	}
}

func TestPlaybackEmptyScript(t *testing.T) {
	for _, steps := range [][]Step{nil, {}, {{}}, {Quit()}, {Hold(ebiten.KeyW, 5)}} {
		src := NewScript(steps)
		if !src.Done() || src.Err() != nil {
			t.Errorf("script %+v: Done %v Err %v, want done at once", steps, src.Done(), src.Err())
		}
		if _, ok := src.TakeShot(); ok {
			t.Errorf("script %+v: unexpected shot", steps)
		}
		st := src.Poll(1600, 900, 2)
		if got := describe(st); got != "-1000,-1000" {
			t.Errorf("script %+v: idle frame = %q", steps, got)
		}
		if st.W != 800 || st.H != 450 || st.Frame != 1 || st.Dt != NominalDt {
			t.Errorf("script %+v: idle frame data %+v", steps, st)
		}
	}
}

func TestPlaybackInvalidScript(t *testing.T) {
	src := NewScript([]Step{ClickAt(1, 2), {Key: "NoSuchKey"}, Shot("x.png")})
	if src.Err() == nil || !strings.Contains(src.Err().Error(), "step 2") {
		t.Fatalf("Err = %v, want an error naming step 2", src.Err())
	}
	if !src.Done() {
		t.Error("an invalid script must be done at once")
	}
	// Nothing of it is played.
	for i := 0; i < 5; i++ {
		if got := describe(src.Poll(800, 600, 1)); got != "-1000,-1000" {
			t.Errorf("frame %d of an invalid script = %q", i+1, got)
		}
	}
	if _, ok := src.TakeShot(); ok {
		t.Error("an invalid script must not request shots")
	}
}

func TestPlaybackDoneAndShotTiming(t *testing.T) {
	src := NewScript([]Step{ClickAt(10, 10), Shot("one.png"), Wait(1), Shot("two.png"), Hold(ebiten.KeyA, 3), Quit()})
	type event struct {
		frame int64
		shot  string
		done  bool
	}
	var got []event
	if _, ok := src.TakeShot(); ok {
		t.Fatal("shot pending before the first frame")
	}
	for !src.Done() {
		st := src.Poll(800, 600, 1)
		name, _ := src.TakeShot()
		if again, ok := src.TakeShot(); ok {
			t.Fatalf("frame %d: second shot %q pending", st.Frame, again)
		}
		got = append(got, event{st.Frame, name, src.Done()})
	}
	want := []event{
		{1, "", false},        // move
		{2, "", false},        // press
		{3, "", false},        // release
		{4, "one.png", false}, // shot frame: the click has been updated and drawn
		{5, "", false},        // wait
		{6, "two.png", true},  // last frame; the trailing hold and quit take none
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("events:\n got %+v\nwant %+v", got, want)
	}
}

// A host that polls twice before collecting shots still gets every one, in
// order.
func TestPlaybackShotsQueue(t *testing.T) {
	src := NewScript([]Step{Shot("a.png"), Shot("b.png"), Wait(1)})
	src.Poll(800, 600, 1)
	src.Poll(800, 600, 1)
	var got []string
	for {
		name, ok := src.TakeShot()
		if !ok {
			break
		}
		got = append(got, name)
	}
	if want := []string{"a.png", "b.png"}; !reflect.DeepEqual(got, want) {
		t.Errorf("shots = %v, want %v", got, want)
	}
}

func TestPlaybackIsDeterministic(t *testing.T) {
	steps, err := ParseScript([]byte(specScript))
	if err != nil {
		t.Fatal(err)
	}
	record := func() []State {
		src := NewScript(steps)
		var out []State
		for !src.Done() {
			out = append(out, src.Poll(1600, 900, 1.25))
		}
		return out
	}
	a, b := record(), record()
	if !reflect.DeepEqual(a, b) {
		t.Fatal("two runs of the same script differ")
	}
	// wait 30 + move 1 + click 3 + click 3 + dblclick 5 + drag 23 + wheel 1
	// + key 2 + key 2 + hold 0 + text 5 + shot 1 = 76 frames.
	if len(a) != 76 {
		t.Errorf("the specification's example script took %d frames, want 76", len(a))
	}
	// The hold covers the text and the shot: 6 of its 10 frames.
	held := 0
	for _, st := range a {
		if st.KeyDown(ebiten.KeyShiftLeft) {
			held++
			if !st.Shift {
				t.Error("ShiftLeft held without the Shift flag")
			}
		}
	}
	if held != 6 {
		t.Errorf("ShiftLeft held on %d frames, want 6", held)
	}
}

func TestNewScriptCopiesItsSteps(t *testing.T) {
	steps := []Step{ClickAt(10, 10), Wait(1)}
	src := NewScript(steps)
	steps[0] = ClickAt(99, 99)
	steps[1] = Quit()
	got := play(t, []Step{ClickAt(10, 10), Wait(1)}, 0)
	var lines []string
	for !src.Done() {
		lines = append(lines, describe(src.Poll(1600, 900, 1)))
	}
	lines[len(lines)-1] += " done"
	checkFrames(t, lines, got)
}

func TestScriptSourceIsASource(t *testing.T) {
	var _ Source = NewScript(nil)
	var _ Source = (*Device)(nil)
}
