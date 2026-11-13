// Command demo exercises packages input and uitest end to end: it shows the
// cursor, buttons, modifiers, held keys, wheel, typed text and an event log,
// all driven by a built-in input script, and saves screenshots along the way.
//
// Run it headlessly with
//
//	xvfb-run -a -s "-screen 0 3840x2160x24" go run ./internal/uitest/demo -out <dir> -w 960 -h 600 -scale 2
//
// Everything on screen is laid out in points over a labelled 100-point grid,
// so the same script must produce the same picture at every scale, only
// sharper.
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"

	"drifthold/internal/input"
	"drifthold/internal/uitest"
)

// Palette of the game's presentation specification.
var (
	colSpace  = color.RGBA{0x0B, 0x0D, 0x14, 0xff}
	colPanel  = color.RGBA{0x12, 0x15, 0x1F, 0xff}
	colPanel2 = color.RGBA{0x1A, 0x1E, 0x2B, 0xff}
	colGrid   = color.RGBA{0x1B, 0x20, 0x2E, 0xff}
	colGrid2  = color.RGBA{0x2A, 0x31, 0x45, 0xff}
	colAmber  = color.RGBA{0xE8, 0xA3, 0x3D, 0xff}
	colCream  = color.RGBA{0xE9, 0xE2, 0xD0, 0xff}
	colDim    = color.RGBA{0x8C, 0x8A, 0x80, 0xff}
	colGood   = color.RGBA{0x7B, 0xC9, 0x6F, 0xff}
	colAlert  = color.RGBA{0xE5, 0x53, 0x3D, 0xff}
	colInfo   = color.RGBA{0x6F, 0xB7, 0xE6, 0xff}
	colTeal   = color.RGBA{0x3D, 0xC9, 0xB3, 0xff}
)

// Fixed layout, in points.
var (
	statusRect = rect{20, 20, 440, 132}
	targetRect = [3]rect{{20, 180, 100, 70}, {140, 180, 100, 70}, {260, 180, 100, 70}}
	trackRect  = rect{20, 280, 440, 80}
	wheelRect  = rect{20, 384, 440, 24}
	fieldRect  = rect{20, 440, 440, 34}
	logRect    = rect{500, 20, 440, 560}
)

const boxSize = 60

// builtinScript drives every feature of the demo. Coordinates are points and
// refer to the layout above.
func builtinScript() []input.Step {
	return []input.Step{
		input.Wait(3),
		input.Shot("01_idle.png"),

		// Hover the first target.
		input.MoveTo(70, 215),
		input.Shot("02_hover.png"),

		// Two separate left clicks (the wait keeps them from being a double
		// click), a right click and a double click.
		input.ClickAt(70, 215),
		input.Wait(25),
		input.ClickAt(70, 215),
		input.ClickWith(input.Right, 190, 215),
		input.DblClickAt(310, 215),
		input.Shot("03_clicks.png"),

		// Drag the box 300 points to the right.
		input.DragTo(50, 320, 350, 320, 20),
		input.Shot("04_dragged.png"),

		// Grab it again and take a picture while the button is still held.
		input.PressAt(input.Left, 350, 320),
		input.GlideTo(200, 345, 8),
		input.Shot("05_mid_drag.png"),
		input.ReleaseAt(input.Left, 150, 320),

		// Wheel: -3, -2, +1 leaves -4.
		input.MoveTo(240, 396),
		input.Scroll(-3),
		input.Scroll(-2),
		input.Scroll(1),
		input.Shot("06_wheel.png"),

		// Focus the text field and type.
		input.ClickAt(240, 457),
		input.Type("Hello, Shoal!"),
		input.Shot("07_typed.png"),

		// Hold Backspace long enough for four auto-repeats: five runes go.
		input.TapFor(ebiten.KeyBackspace, 34),
		input.Shot("08_backspace_repeat.png"),

		// Shift-click: the hold overlaps the click and the screenshot.
		input.Hold(ebiten.KeyShiftLeft, 6),
		input.ClickAt(70, 215),
		input.Shot("09_shift_click.png"),
		input.Wait(4),

		// A handful of plain keys for the log, then a chord held for the
		// final picture.
		input.Tap(ebiten.KeyR),
		input.Tap(ebiten.KeyEscape),
		input.Tap(ebiten.KeyF1),
		input.Tap(ebiten.KeyDigit1),
		input.Tap(ebiten.KeyArrowLeft),
		input.Hold(ebiten.KeyControlLeft, 0),
		input.Hold(ebiten.KeyAltRight, 0),
		input.Hold(ebiten.KeyW, 0),
		input.MoveTo(430, 520),
		input.Shot("10_keys.png"),
		input.Quit(),
	}
}

func main() {
	o := uitest.Flags()
	if o.Script == nil {
		o.Script = builtinScript()
	}
	o.Title = "Drifthold input demo"

	d, err := newDemo()
	if err != nil {
		log.Fatal(err)
	}
	if err := uitest.Run(o, d.update, d.draw); err != nil {
		log.Fatal(err)
	}
}

type rect struct{ x, y, w, h float32 }

func (r rect) contains(x, y float32) bool {
	return x >= r.x && y >= r.y && x < r.x+r.w && y < r.y+r.h
}

// mark is a spot where a mouse button went down or up.
type mark struct {
	x, y   float32
	button input.Button
	up     bool
}

type point struct{ x, y float32 }

type demo struct {
	regular, mono *text.GoTextFaceSource
	faces         map[faceKey]*text.GoTextFace

	in          input.State
	counts      [3]int // left clicks, right clicks, double clicks
	shiftClicks int

	boxX, boxY     float32
	dragging       bool
	grabDX, grabDY float32
	trail          []point

	wheel   float32
	focus   bool
	typed   []rune
	entered string

	marks []mark
	log   []string
	keys  []input.Key
}

type faceKey struct {
	mono bool
	px   float64
}

func newDemo() (*demo, error) {
	regular, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, fmt.Errorf("loading Go Regular: %w", err)
	}
	mono, err := text.NewGoTextFaceSource(bytes.NewReader(gomono.TTF))
	if err != nil {
		return nil, fmt.Errorf("loading Go Mono: %w", err)
	}
	return &demo{
		regular: regular,
		mono:    mono,
		faces:   map[faceKey]*text.GoTextFace{},
		boxX:    trackRect.x,
		boxY:    trackRect.y + (trackRect.h-boxSize)/2,
	}, nil
}

func (d *demo) logf(format string, args ...any) {
	line := fmt.Sprintf("f%-4d ", d.in.Frame) + fmt.Sprintf(format, args...)
	d.log = append(d.log, line)
	const keep = 24
	if len(d.log) > keep {
		d.log = d.log[len(d.log)-keep:]
	}
}

// update reacts to one frame of input. All hit tests are in points.
func (d *demo) update(in input.State) {
	d.in = in

	for b := input.Left; b < input.NumButtons; b++ {
		if in.Pressed[b] {
			d.marks = append(d.marks, mark{in.X, in.Y, b, false})
			d.logf("press   %-6s at %.0f, %.0f", b, in.X, in.Y)
		}
		if in.Released[b] {
			d.marks = append(d.marks, mark{in.X, in.Y, b, true})
			d.logf("release %-6s at %.0f, %.0f", b, in.X, in.Y)
		}
	}
	if in.DoubleClick {
		d.logf("double click")
	}
	if in.Wheel != 0 {
		d.wheel += in.Wheel
		d.logf("wheel %+g", in.Wheel)
	}
	d.keys = in.AppendKeysPressed(d.keys[:0])
	for _, k := range d.keys {
		d.logf("key down %s", input.KeyName(k))
	}
	if len(in.Chars) > 0 {
		d.logf("typed %q", string(in.Chars))
	}

	// Click targets.
	if in.Pressed[input.Left] && targetRect[0].contains(in.X, in.Y) {
		d.counts[0]++
		if in.Shift {
			d.shiftClicks++
		}
	}
	if in.Pressed[input.Right] && targetRect[1].contains(in.X, in.Y) {
		d.counts[1]++
	}
	if in.DoubleClick && targetRect[2].contains(in.X, in.Y) {
		d.counts[2]++
	}

	// Draggable box.
	box := rect{d.boxX, d.boxY, boxSize, boxSize}
	if in.Pressed[input.Left] && box.contains(in.X, in.Y) {
		d.dragging = true
		d.grabDX, d.grabDY = in.X-d.boxX, in.Y-d.boxY
		d.trail = d.trail[:0]
	}
	if d.dragging && in.Down[input.Left] {
		d.boxX = clamp(in.X-d.grabDX, trackRect.x, trackRect.x+trackRect.w-boxSize)
		d.boxY = clamp(in.Y-d.grabDY, trackRect.y, trackRect.y+trackRect.h-boxSize)
		d.trail = append(d.trail, point{in.X, in.Y})
	}
	if in.Released[input.Left] {
		d.dragging = false
	}

	// Text field.
	if in.Pressed[input.Left] {
		d.focus = fieldRect.contains(in.X, in.Y)
	}
	if d.focus {
		d.typed = append(d.typed, in.Chars...)
		if in.KeyRepeated(ebiten.KeyBackspace) && len(d.typed) > 0 {
			d.typed = d.typed[:len(d.typed)-1]
		}
		if in.KeyPressed(ebiten.KeyEnter) {
			d.entered = string(d.typed)
			d.typed = d.typed[:0]
		}
	}
}

func clamp(v, lo, hi float32) float32 {
	return min(max(v, lo), hi)
}

// canvas draws in points onto a pixel image.
type canvas struct {
	d   *demo
	dst *ebiten.Image
	s   float32 // pixels per point
}

func (c canvas) fill(r rect, col color.Color) {
	vector.FillRect(c.dst, r.x*c.s, r.y*c.s, r.w*c.s, r.h*c.s, col, false)
}

func (c canvas) stroke(r rect, width float32, col color.Color) {
	c.fill(rect{r.x, r.y, r.w, width}, col)
	c.fill(rect{r.x, r.y + r.h - width, r.w, width}, col)
	c.fill(rect{r.x, r.y, width, r.h}, col)
	c.fill(rect{r.x + r.w - width, r.y, width, r.h}, col)
}

func (c canvas) line(x0, y0, x1, y1, width float32, col color.Color) {
	vector.StrokeLine(c.dst, x0*c.s, y0*c.s, x1*c.s, y1*c.s, width*c.s, col, true)
}

func (c canvas) ring(x, y, r, width float32, col color.Color) {
	vector.StrokeCircle(c.dst, x*c.s, y*c.s, r*c.s, width*c.s, col, true)
}

func (c canvas) dot(x, y, r float32, col color.Color) {
	vector.FillCircle(c.dst, x*c.s, y*c.s, r*c.s, col, true)
}

// text draws one line with its top-left corner at (x, y); size is in points.
func (c canvas) text(str string, x, y, size float32, mono bool, col color.Color) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(x*c.s), float64(y*c.s))
	op.ColorScale.ScaleWithColor(col)
	text.Draw(c.dst, str, c.d.face(mono, float64(size*c.s)), op)
}

// face returns the font face of the given pixel size, creating it on first
// use, so that text is rasterised at native resolution for every scale.
func (d *demo) face(mono bool, px float64) *text.GoTextFace {
	key := faceKey{mono, px}
	if f, ok := d.faces[key]; ok {
		return f
	}
	src := d.regular
	if mono {
		src = d.mono
	}
	f := &text.GoTextFace{Source: src, Size: px}
	d.faces[key] = f
	return f
}

var buttonColor = [input.NumButtons]color.RGBA{
	input.Left:   colAmber,
	input.Right:  colTeal,
	input.Middle: colInfo,
}

func (d *demo) draw(dst *ebiten.Image) {
	in := d.in
	if in.Scale == 0 {
		return // no frame polled yet
	}
	c := canvas{d: d, dst: dst, s: in.Scale}
	dst.Fill(colSpace)

	// Point grid with labels every 100 points.
	for x := float32(0); x < in.W; x += 50 {
		col, major := colGrid, int(x)%100 == 0
		if major {
			col = colGrid2
		}
		c.fill(rect{x, 0, 1, in.H}, col)
		if major && x > 0 {
			c.text(fmt.Sprintf("%d", int(x)), x+3, 2, 10, true, colDim)
		}
	}
	for y := float32(0); y < in.H; y += 50 {
		col, major := colGrid, int(y)%100 == 0
		if major {
			col = colGrid2
		}
		c.fill(rect{0, y, in.W, 1}, col)
		if major && y > 0 {
			c.text(fmt.Sprintf("%d", int(y)), 3, y+2, 10, true, colDim)
		}
	}

	d.drawStatus(c)
	d.drawTargets(c)
	d.drawTrack(c)
	d.drawWheel(c)
	d.drawField(c)
	d.drawLog(c)

	// Where buttons went down (rings) and up (dots).
	for _, m := range d.marks {
		col := buttonColor[m.button]
		if m.up {
			c.dot(m.x, m.y, 2.5, col)
		} else {
			c.ring(m.x, m.y, 7, 1.5, col)
		}
	}

	// The cursor: a crosshair with its position.
	if in.CursorInside() {
		c.line(in.X-12, in.Y, in.X-3, in.Y, 1.5, colCream)
		c.line(in.X+3, in.Y, in.X+12, in.Y, 1.5, colCream)
		c.line(in.X, in.Y-12, in.X, in.Y-3, 1.5, colCream)
		c.line(in.X, in.Y+3, in.X, in.Y+12, 1.5, colCream)
		c.text(fmt.Sprintf("%.0f, %.0f", in.X, in.Y), in.X+10, in.Y+10, 11, true, colCream)
	}
}

func (d *demo) panel(c canvas, r rect, title string) {
	c.fill(r, colPanel)
	c.stroke(r, 1, colPanel2)
	c.fill(rect{r.x, r.y, r.w, 1}, colAmber)
	if title != "" {
		c.text(strings.ToUpper(title), r.x+8, r.y-15, 10, false, colDim)
	}
}

// lamp draws a labelled indicator that lights up when on.
func (d *demo) lamp(c canvas, x, y float32, label string, on bool, col color.Color) float32 {
	const w, h = 46, 18
	r := rect{x, y, w, h}
	if on {
		c.fill(r, col)
		c.text(label, x+6, y+3, 11, false, colSpace)
	} else {
		c.fill(r, colPanel2)
		c.text(label, x+6, y+3, 11, false, colDim)
	}
	return x + w + 6
}

func (d *demo) drawStatus(c canvas) {
	in := d.in
	r := statusRect
	d.panel(c, r, "")
	x, y := r.x+10, r.y+9

	cursor := fmt.Sprintf("cursor  %7.1f, %7.1f pt", in.X, in.Y)
	if !in.CursorInside() {
		cursor = "cursor  outside the window"
	}
	c.text(cursor, x, y, 13, true, colCream)
	c.text(fmt.Sprintf("frame %d", in.Frame), r.x+r.w-100, y, 13, true, colDim)
	y += 20
	c.text(fmt.Sprintf("screen  %.0f x %.0f pt   scale %g   %.0f x %.0f px",
		in.W, in.H, in.Scale, in.W*in.Scale, in.H*in.Scale), x, y, 13, true, colCream)
	y += 24

	lx := x
	lx = d.lamp(c, lx, y, "left", in.Down[input.Left], buttonColor[input.Left])
	lx = d.lamp(c, lx, y, "right", in.Down[input.Right], buttonColor[input.Right])
	lx = d.lamp(c, lx, y, "mid", in.Down[input.Middle], buttonColor[input.Middle])
	lx += 14
	lx = d.lamp(c, lx, y, "shift", in.Shift, colGood)
	lx = d.lamp(c, lx, y, "ctrl", in.Ctrl, colGood)
	d.lamp(c, lx, y, "alt", in.Alt, colGood)
	y += 28

	d.keys = in.AppendKeysDown(d.keys[:0])
	names := make([]string, len(d.keys))
	for i, k := range d.keys {
		names[i] = input.KeyName(k)
	}
	held := "none"
	if len(names) > 0 {
		held = strings.Join(names, "  ")
	}
	c.text("keys down", x, y, 11, false, colDim)
	c.text(held, x+70, y-1, 13, true, colCream)
	y += 20
	c.text("wheel sum", x, y, 11, false, colDim)
	c.text(fmt.Sprintf("%+g", d.wheel), x+70, y-1, 13, true, colCream)
}

func (d *demo) drawTargets(c canvas) {
	in := d.in
	labels := [3]string{"LEFT CLICK", "RIGHT CLICK", "DOUBLE CLICK"}
	for i, r := range targetRect {
		bg := colPanel
		hover := r.contains(in.X, in.Y)
		if hover {
			bg = colPanel2
		}
		c.fill(r, bg)
		edge := color.Color(colPanel2)
		if hover {
			edge = colAmber
		}
		c.stroke(r, 1, edge)
		c.text(labels[i], r.x+8, r.y+8, 10, false, colDim)
		c.text(fmt.Sprintf("%d", d.counts[i]), r.x+8, r.y+24, 28, true, colCream)
		if i == 0 {
			c.text(fmt.Sprintf("with shift %d", d.shiftClicks), r.x+8, r.y+r.h-16, 10, false, colDim)
		}
	}
}

func (d *demo) drawTrack(c canvas) {
	d.panel(c, trackRect, "drag the box")
	for _, p := range d.trail {
		c.dot(p.x, p.y, 1.5, colInfo)
	}
	box := rect{d.boxX, d.boxY, boxSize, boxSize}
	col := colTeal
	if d.dragging {
		col = colAmber
	}
	c.fill(box, col)
	c.text(fmt.Sprintf("x %.0f", d.boxX), box.x+6, box.y+6, 11, true, colSpace)
	c.text(fmt.Sprintf("y %.0f", d.boxY), box.x+6, box.y+22, 11, true, colSpace)
}

func (d *demo) drawWheel(c canvas) {
	r := wheelRect
	d.panel(c, r, "wheel")
	// One notch per 20 points, zero in the middle.
	mid := r.x + r.w/2
	c.fill(rect{mid, r.y, 1, r.h}, colDim)
	w := clamp(d.wheel*20, -r.w/2, r.w/2)
	if w >= 0 {
		c.fill(rect{mid, r.y + 6, w, r.h - 12}, colGood)
	} else {
		c.fill(rect{mid + w, r.y + 6, -w, r.h - 12}, colAlert)
	}
}

func (d *demo) drawField(c canvas) {
	r := fieldRect
	d.panel(c, r, "text field (click to focus)")
	if d.focus {
		c.stroke(r, 1, colAmber)
	}
	str := string(d.typed)
	c.text(str, r.x+10, r.y+9, 14, false, colCream)
	if d.focus {
		w, _ := text.Measure(str, d.face(false, float64(14*c.s)), 0)
		c.fill(rect{r.x + 11 + float32(w)/c.s, r.y + 8, 1.5, r.h - 16}, colAmber)
	}
	if d.entered != "" {
		c.text("entered: "+d.entered, r.x+10, r.y+r.h+8, 11, false, colDim)
	}
}

func (d *demo) drawLog(c canvas) {
	r := logRect
	if r.x+r.w > d.in.W {
		return // window too narrow for the log
	}
	d.panel(c, r, "")
	c.text("EVENT LOG", r.x+10, r.y+8, 10, false, colDim)
	y := r.y + 28
	for _, line := range d.log {
		if y+16 > r.y+r.h {
			break
		}
		c.text(line, r.x+10, y, 12, true, colCream)
		y += 21
	}
}
