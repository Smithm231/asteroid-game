// Package kit is the small immediate-mode widget toolkit of Drifthold.
//
// Frame protocol: call Begin with the frame's input, declare every widget
// (each call both handles input and records how to draw itself), call End,
// and replay the recorded drawing with Draw as often as needed.
//
//	ctx.Begin(in)
//	if ctx.Button("ok", r, "OK", kit.Btn{}) { ... }
//	ctx.End()
//	...
//	ctx.Draw(screen)
//
// All geometry is in points. Widgets are identified by string ids that must be
// unique within a frame. Drawing is grouped in layers (world, panels,
// windows, popups, tooltips); a widget only reacts to the mouse when nothing
// in a higher layer covered that spot in the previous frame.
package kit

import (
	"hash/fnv"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"drifthold/internal/gfx"
	"drifthold/internal/input"
)

// Layers, bottom to top.
const (
	LayerWorld = iota
	LayerPanel
	LayerWindow
	LayerPopup
	LayerTip
	numLayers
)

// Sound is a cue the toolkit asks the application to play.
type Sound int

// Interface sounds.
const (
	SoundHover Sound = iota
	SoundClick
	SoundOpen
	SoundClose
	SoundError
)

type blocker struct {
	r     gfx.Rect
	layer int
}

// Ctx is the toolkit state. Create one with New and keep it for the life of
// the application.
type Ctx struct {
	// In is the input of the current frame.
	In input.State
	// Th is the theme every widget draws with.
	Th *gfx.Theme
	// OnSound, when set, receives interface sound cues.
	OnSound func(Sound)
	// Time is the running clock in seconds, for animations.
	Time float32

	layer    int
	cmds     [numLayers][]func(cv *gfx.Canvas)
	blockers []blocker
	prev     []blocker
	clip     gfx.Rect
	clips    []gfx.Rect

	active  uint64 // widget holding the mouse
	focus   uint64 // text input holding the keyboard
	lastHot uint64 // widget under the cursor in the previous frame
	hot     uint64
	scroll  map[uint64]float32
	layers  []int

	tip      *Tip
	tipRect  gfx.Rect
	tipID    uint64
	tipSince float32

	caret float32
}

// New returns a toolkit context using the given theme.
func New(th *gfx.Theme) *Ctx {
	return &Ctx{Th: th, scroll: map[uint64]float32{}}
}

func hashID(id string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(id))
	return h.Sum64() | 1
}

// Begin starts a frame.
func (c *Ctx) Begin(in input.State) {
	c.In = in
	c.Time += in.Dt
	c.caret += in.Dt
	for i := range c.cmds {
		c.cmds[i] = c.cmds[i][:0]
	}
	c.prev, c.blockers = c.blockers, c.prev[:0]
	c.layer = LayerPanel
	c.clip = gfx.R(0, 0, in.W, in.H)
	c.clips = c.clips[:0]
	c.layers = c.layers[:0]
	c.lastHot, c.hot = c.hot, 0
	c.tip = nil
	if !in.Down[input.Left] && !in.Released[input.Left] {
		c.active = 0
	}
	if in.Pressed[input.Left] {
		// A click anywhere drops the keyboard focus; a text input under the
		// cursor takes it back in the same frame.
		c.focus = 0
	}
}

// End finishes a frame.
func (c *Ctx) End() {
	if c.tip != nil {
		c.tipSince += c.In.Dt
		if c.tipSince > 0.35 {
			c.drawTip()
		}
	} else {
		c.tipSince = 0
		c.tipID = 0
	}
	if c.In.Released[input.Left] {
		c.active = 0
	}
}

// Draw replays the frame's drawing onto dst.
func (c *Ctx) Draw(dst *ebiten.Image) {
	cv := gfx.NewCanvas(dst, c.In.Scale)
	for l := range c.cmds {
		for _, fn := range c.cmds[l] {
			fn(cv)
		}
	}
}

// SetLayer selects the layer for the widgets that follow and returns the
// previous one.
func (c *Ctx) SetLayer(l int) int {
	old := c.layer
	c.layer = l
	return old
}

// Paint records custom drawing in the current layer.
func (c *Ctx) Paint(fn func(cv *gfx.Canvas)) {
	c.cmds[c.layer] = append(c.cmds[c.layer], fn)
}

// Block marks r as covered by the current layer: widgets and world views in
// lower layers ignore the mouse there from the next frame on.
func (c *Ctx) Block(r gfx.Rect) {
	c.blockers = append(c.blockers, blocker{r, c.layer})
}

// covered reports whether a higher layer covered the cursor position.
func (c *Ctx) covered(layer int) bool {
	for _, b := range c.prev {
		if b.layer > layer && b.r.Contains(c.In.X, c.In.Y) {
			return true
		}
	}
	return false
}

// Hovered reports whether the cursor is over r and free for a widget of the
// current layer: inside the active clip, not covered by a higher layer and
// not captured by another widget.
func (c *Ctx) Hovered(r gfx.Rect) bool {
	return r.Contains(c.In.X, c.In.Y) && c.clip.Contains(c.In.X, c.In.Y) && !c.covered(c.layer)
}

// WantMouse reports whether the interface owns the mouse this frame: the
// cursor is over a panel or window, or a widget is being held. World views
// must ignore the mouse while it is true.
func (c *Ctx) WantMouse() bool {
	return c.active != 0 || c.covered(LayerWorld)
}

// WantKeys reports whether a text input has the keyboard. Hotkeys must be
// ignored while it is true.
func (c *Ctx) WantKeys() bool { return c.focus != 0 }

func (c *Ctx) sound(s Sound) {
	if c.OnSound != nil {
		c.OnSound(s)
	}
}

// press runs the common click logic of a widget and reports (hovered, held,
// clicked).
func (c *Ctx) press(id uint64, r gfx.Rect, enabled bool) (hover, held, clicked bool) {
	hover = c.Hovered(r) && (c.active == 0 || c.active == id)
	if hover {
		c.hot = id
		if enabled && c.lastHot != id && c.active == 0 {
			c.sound(SoundHover)
		}
	}
	if !enabled {
		if hover && c.In.Pressed[input.Left] {
			c.sound(SoundError)
		}
		return hover, false, false
	}
	if hover && c.In.Pressed[input.Left] {
		c.active = id
	}
	held = c.active == id && c.In.Down[input.Left]
	if c.active == id && c.In.Released[input.Left] && hover {
		clicked = true
		c.sound(SoundClick)
	}
	return hover, held, clicked
}

// ------------------------------------------------------------------ statics

// Panel draws a standard chamfered panel and blocks the world beneath it.
func (c *Ctx) Panel(r gfx.Rect) {
	c.Block(r)
	th := c.Th
	c.Paint(func(cv *gfx.Canvas) {
		cv.Panel(r, th.Chamfer, gfx.CornersDiag, th.Panel, th.RuleDim)
	})
}

// Header draws a section heading with an amber rule under it, at the top of
// r, and returns the area below.
func (c *Ctx) Header(r gfx.Rect, title string) gfx.Rect {
	th := c.Th
	bar := r.CutTop(22)
	c.Paint(func(cv *gfx.Canvas) {
		cv.TextIn(bar, title, th.Label().WithColor(th.Text))
		cv.FillRect(gfx.R(bar.X, bar.Bottom()-1, bar.W, th.Hairline), th.RuleDim)
	})
	r.CutTop(4)
	return r
}

// Label draws one line of text inside r, shortened with an ellipsis if needed.
func (c *Ctx) Label(r gfx.Rect, text string, st gfx.TextStyle) {
	c.Paint(func(cv *gfx.Canvas) { cv.TextFit(r, text, st) })
}

// Text draws word-wrapped text from the top of r and returns the height used.
func (c *Ctx) Text(r gfx.Rect, text string, st gfx.TextStyle) float32 {
	lines := gfx.WrapText(text, r.W, st, c.In.Scale)
	lh := gfx.LineHeight(st, c.In.Scale)
	c.Paint(func(cv *gfx.Canvas) { cv.TextLines(lines, anchorX(r, st), r.Y, st) })
	return float32(len(lines)) * lh
}

func anchorX(r gfx.Rect, st gfx.TextStyle) float32 {
	switch st.Align {
	case gfx.AlignCenter:
		return r.MidX()
	case gfx.AlignRight:
		return r.Right()
	}
	return r.X
}

// Progress draws a progress bar filled to frac (0..1) with an optional label
// centred on it.
func (c *Ctx) Progress(r gfx.Rect, frac float32, col gfx.Color, label string) {
	th := c.Th
	frac = float32(math.Max(0, math.Min(1, float64(frac))))
	c.Paint(func(cv *gfx.Canvas) {
		cv.FillRect(r, th.Well)
		if frac > 0 {
			cv.FillRect(gfx.R(r.X, r.Y, r.W*frac, r.H), col)
		}
		cv.StrokeRect(r, th.Hairline, th.Border)
		if label != "" {
			cv.TextIn(r, label, th.Small().Aligned(gfx.AlignCenter).WithColor(th.Text))
		}
	})
}

// ------------------------------------------------------------------ buttons

// Btn are the options of a button.
type Btn struct {
	// Disabled greys the button out; Why, when set, is shown as its tooltip.
	Disabled bool
	Why      string
	// Primary draws the button in amber: the main action of a dialog.
	Primary bool
	// Selected draws the button as the chosen one of a group.
	Selected bool
	// Tip is the tooltip shown while hovering.
	Tip *Tip
	// Hotkey triggers the button from the keyboard.
	Hotkey input.Key
	// Icon, when set, draws a glyph left of the label (or centred when the
	// label is empty).
	Icon func(cv *gfx.Canvas, r gfx.Rect, col gfx.Color)
	// Align places the label; the default is centred.
	Align gfx.Align
	// Left makes Align apply (AlignLeft is the zero value).
	Left bool
	// Quiet draws no frame until hovered (toolbar style).
	Quiet bool
	// Tight uses minimal side padding, for narrow tabs.
	Tight bool
}

// Button declares a push button and reports whether it was clicked.
func (c *Ctx) Button(id string, r gfx.Rect, label string, o Btn) bool {
	h := hashID(id)
	hover, held, clicked := c.press(h, r, !o.Disabled)
	if o.Hotkey != 0 && !o.Disabled && !c.WantKeys() && !c.In.KeysTaken && c.In.KeyPressed(o.Hotkey) && !c.In.Ctrl && !c.In.Alt {
		clicked = true
		c.sound(SoundClick)
	}
	if hover {
		switch {
		case o.Disabled && o.Why != "":
			c.Tooltip(h, r, &Tip{Body: o.Why})
		case o.Tip != nil:
			c.Tooltip(h, r, o.Tip)
		}
	}
	th := c.Th
	c.Paint(func(cv *gfx.Canvas) {
		fill, border, text := th.PanelHi, th.Border, th.Text
		switch {
		case o.Disabled:
			fill, text = th.Panel, th.TextFaint
		case o.Primary:
			fill, border, text = th.Rule, th.Rule, th.Ink
		case o.Selected:
			border = th.Rule
		}
		if o.Quiet && !hover && !o.Selected {
			fill, border = gfx.Color{}, gfx.Color{}
		}
		cv.Panel(r, 4, gfx.CornersDiag, fill, border)
		if !o.Disabled {
			if held {
				cv.FillChamferC(r, 4, gfx.CornersDiag, th.Press)
			} else if hover {
				cv.FillChamferC(r, 4, gfx.CornersDiag, th.Hover)
			}
		}
		if o.Selected && !o.Primary {
			cv.FillRect(gfx.R(r.X+4, r.Bottom()-2, r.W-8, 2), th.Rule)
			text = th.Rule
		}
		inner := r.InsetXY(8, 0)
		if o.Tight {
			inner = r.InsetXY(2, 0)
		}
		if o.Icon != nil {
			if label == "" {
				s := min(r.W, r.H) - 8
				o.Icon(cv, r.Center(s, s), text)
				return
			}
			s := min(r.H-8, 18)
			ir := inner.CutLeft(s + 6)
			o.Icon(cv, gfx.R(ir.X, r.MidY()-s/2, s, s), text)
		}
		st := th.Body().WithColor(text).Aligned(gfx.AlignCenter)
		if o.Left {
			st = st.Aligned(o.Align)
		}
		if o.Primary || o.Selected {
			st = st.Weighted(gfx.Medium)
		}
		cv.TextFit(inner, label, st)
	})
	return clicked
}

// Tabs declares a row of tabs and reports whether the selection changed.
func (c *Ctx) Tabs(id string, r gfx.Rect, labels []string, sel *int) bool {
	changed := false
	n := len(labels)
	for i, l := range labels {
		if c.Button(id+"/"+l, r.Col(i, n, 2), l, Btn{Selected: *sel == i, Quiet: true}) && *sel != i {
			*sel = i
			changed = true
		}
	}
	th := c.Th
	c.Paint(func(cv *gfx.Canvas) {
		cv.FillRect(gfx.R(r.X, r.Bottom()-1, r.W, th.Hairline), th.RuleDim)
	})
	return changed
}

// Check declares a check box with a label and reports whether it was toggled.
func (c *Ctx) Check(id string, r gfx.Rect, label string, v *bool) bool {
	h := hashID(id)
	hover, _, clicked := c.press(h, r, true)
	if clicked {
		*v = !*v
	}
	on := *v
	th := c.Th
	c.Paint(func(cv *gfx.Canvas) {
		box := gfx.R(r.X, r.MidY()-8, 16, 16)
		cv.FillRect(box, th.Well)
		col := th.Border
		if hover {
			col = th.Rule
		}
		cv.StrokeRect(box, th.Hairline, col)
		if on {
			cv.FillRect(box.Inset(4), th.Rule)
		}
		cv.TextFit(gfx.R(r.X+24, r.Y, r.W-24, r.H), label, th.Body())
	})
	return clicked
}

// Slider declares a horizontal slider for *v in [lo, hi] and reports whether
// the value changed.
func (c *Ctx) Slider(id string, r gfx.Rect, v *float64, lo, hi float64) bool {
	h := hashID(id)
	hover, held, _ := c.press(h, r, true)
	old := *v
	if held && r.W > 16 {
		t := float64((c.In.X - r.X - 8) / (r.W - 16))
		*v = lo + (hi-lo)*math.Max(0, math.Min(1, t))
	}
	frac := float32(0)
	if hi > lo {
		frac = float32((*v - lo) / (hi - lo))
	}
	th := c.Th
	c.Paint(func(cv *gfx.Canvas) {
		track := gfx.R(r.X+8, r.MidY()-2, r.W-16, 4)
		cv.FillRect(track, th.Well)
		cv.FillRect(gfx.R(track.X, track.Y, track.W*frac, track.H), th.RuleDim)
		kx := track.X + track.W*frac
		col := th.Text
		if hover || held {
			col = th.Rule
		}
		cv.FillRect(gfx.R(kx-4, r.MidY()-8, 8, 16), col)
	})
	return *v != old
}

// TextInput declares a single-line text field editing *s and reports whether
// the text changed. allow, when not nil, filters typed characters.
func (c *Ctx) TextInput(id string, r gfx.Rect, s *string, maxLen int, allow func(rune) bool) bool {
	h := hashID(id)
	hover := c.Hovered(r)
	if hover && c.In.Pressed[input.Left] {
		c.focus = h
		c.caret = 0
	}
	focused := c.focus == h
	old := *s
	if focused {
		for _, ch := range c.In.Chars {
			if ch < 32 || (allow != nil && !allow(ch)) || len([]rune(*s)) >= maxLen {
				continue
			}
			*s += string(ch)
		}
		if c.In.KeyRepeated(ebiten.KeyBackspace) {
			if rs := []rune(*s); len(rs) > 0 {
				*s = string(rs[:len(rs)-1])
			}
		}
		if c.In.KeyPressed(ebiten.KeyEnter) || c.In.KeyPressed(ebiten.KeyEscape) || c.In.KeyPressed(ebiten.KeyTab) {
			c.focus = 0
		}
	}
	text := *s
	blink := focused && math.Mod(float64(c.caret), 1) < 0.55
	th := c.Th
	scale := c.In.Scale
	c.Paint(func(cv *gfx.Canvas) {
		cv.FillRect(r, th.Well)
		col := th.Border
		if focused || hover {
			col = th.Rule
		}
		cv.StrokeRect(r, th.Hairline, col)
		st := th.Body()
		inner := r.InsetXY(8, 0)
		cv.TextFit(inner, text, st)
		if blink {
			w, _ := gfx.MeasureText(text, st, scale)
			x := min(inner.X+w+1, inner.Right())
			cv.FillRect(gfx.R(x, r.Y+5, 1.5, r.H-10), th.Text)
		}
	})
	return *s != old
}

// ------------------------------------------------------------------- scroll

// BeginScroll starts a vertically scrolling area showing content of height
// contentH inside view. It returns the rectangle to lay the content out in:
// as wide as the view (minus the scroll bar), contentH tall and already
// shifted by the scroll position. Widgets declared until EndScroll are clipped
// to the view.
func (c *Ctx) BeginScroll(id string, view gfx.Rect, contentH float32) gfx.Rect {
	h := hashID(id)
	maxOff := max(0, contentH-view.H)
	off := c.scroll[h]
	if c.Hovered(view) && c.In.Wheel != 0 {
		off -= c.In.Wheel * 40
	}
	// Draggable thumb.
	bar := gfx.R(view.Right()-8, view.Y, 8, view.H)
	thumbH := view.H
	if contentH > view.H {
		thumbH = max(24, view.H*view.H/contentH)
		_, held, _ := c.press(h, bar, true)
		if held && view.H > thumbH {
			off = (c.In.Y - view.Y - thumbH/2) / (view.H - thumbH) * maxOff
		}
	}
	off = max(0, min(maxOff, off))
	c.scroll[h] = off
	c.clips = append(c.clips, c.clip)
	c.clip = c.clip.Intersect(view)
	th := c.Th
	c.Paint(func(cv *gfx.Canvas) {
		if contentH > view.H {
			cv.FillRect(bar, th.Well)
			ty := view.Y
			if maxOff > 0 {
				ty += off / maxOff * (view.H - thumbH)
			}
			cv.FillRect(gfx.R(bar.X+2, ty, 4, thumbH), th.RuleDim)
		}
		cv.PushClip(view)
	})
	w := view.W
	if contentH > view.H {
		w -= 12
	}
	return gfx.R(view.X, view.Y-off, w, contentH)
}

// EndScroll ends the scrolling area started by the matching BeginScroll.
func (c *Ctx) EndScroll() {
	if n := len(c.clips); n > 0 {
		c.clip = c.clips[n-1]
		c.clips = c.clips[:n-1]
	}
	c.Paint(func(cv *gfx.Canvas) { cv.PopClip() })
}

// Visible reports whether r intersects the active clip: rows of a long
// scrolling list can skip their work when it is false.
func (c *Ctx) Visible(r gfx.Rect) bool { return c.clip.Overlaps(r) }

// ------------------------------------------------------------------ windows

// Window draws a window frame with a title bar in the window layer, blocks
// everything beneath it, and returns the content area. closed is true when
// the player clicked the close button or pressed Escape. Call EndWindow when
// the content has been declared.
func (c *Ctx) Window(id, title string, r gfx.Rect) (content gfx.Rect, closed bool) {
	c.clips = append(c.clips, c.clip)
	c.layers = append(c.layers, c.SetLayer(LayerWindow))
	c.Block(r)
	th := c.Th
	bar := gfx.R(r.X, r.Y, r.W, 34)
	c.Paint(func(cv *gfx.Canvas) {
		cv.Shadow(r.Translate(0, 6), 18, th.Shadow)
		cv.Panel(r, th.Chamfer+2, gfx.CornersDiag, th.Panel, th.Rule)
		cv.FillChamferC(bar.Inset(1), th.Chamfer+1, gfx.CornerTL, th.PanelHi)
		cv.FillRect(gfx.R(r.X+1, bar.Bottom()-1, r.W-2, th.Hairline), th.Rule)
		cv.TextIn(gfx.R(bar.X+14, bar.Y, bar.W-60, bar.H), title, th.Head().WithColor(th.Rule))
	})
	x := gfx.R(bar.Right()-32, bar.Y+5, 24, 24)
	closed = c.Button(id+"/close", x, "", Btn{Quiet: true, Icon: func(cv *gfx.Canvas, r gfx.Rect, col gfx.Color) {
		m := r.Inset(4)
		cv.Line(m.X, m.Y, m.Right(), m.Bottom(), 1.5, col)
		cv.Line(m.Right(), m.Y, m.X, m.Bottom(), 1.5, col)
	}})
	if !c.WantKeys() && !c.In.KeysTaken && c.In.KeyPressed(ebiten.KeyEscape) {
		closed = true
	}
	if closed {
		c.sound(SoundClose)
	}
	return gfx.R(r.X, bar.Bottom(), r.W, r.H-bar.H).Inset(th.Pad + 4), closed
}

// EndWindow ends the window started by the matching Window call.
func (c *Ctx) EndWindow() {
	if n := len(c.layers); n > 0 {
		c.SetLayer(c.layers[n-1])
		c.layers = c.layers[:n-1]
	}
	if n := len(c.clips); n > 0 {
		c.clip = c.clips[n-1]
		c.clips = c.clips[:n-1]
	}
}

// Scrim dims the whole screen beneath the current layer and blocks it, for
// modal dialogs.
func (c *Ctx) Scrim() {
	full := gfx.R(0, 0, c.In.W, c.In.H)
	c.Block(full)
	th := c.Th
	c.Paint(func(cv *gfx.Canvas) { cv.FillRect(full, th.Scrim) })
}

// ----------------------------------------------------------------- tooltips

// TipRow is a key/value line of a tooltip.
type TipRow struct {
	K, V string
	Col  gfx.Color
}

// Tip is the content of a tooltip.
type Tip struct {
	Title string
	Body  string
	Rows  []TipRow
	Foot  string
}

// Tooltip shows tip while the cursor rests on r. id identifies the anchor so
// that moving to another anchor restarts the delay.
func (c *Ctx) Tooltip(id uint64, r gfx.Rect, tip *Tip) {
	if tip == nil || !c.Hovered(r) || c.active != 0 {
		return
	}
	if c.tipID != id {
		c.tipID = id
		c.tipSince = 0
	}
	c.tip = tip
	c.tipRect = r
}

// TipFor is Tooltip for an anchor that is not a widget: the id is derived
// from the given string.
func (c *Ctx) TipFor(id string, r gfx.Rect, tip *Tip) { c.Tooltip(hashID(id), r, tip) }

func (c *Ctx) drawTip() {
	tip, th, scale := c.tip, c.Th, c.In.Scale
	w := float32(300)
	pad := float32(10)
	if tip.Body == "" && tip.Foot == "" && len(tip.Rows) == 0 {
		// A bare title: make the box fit it.
		tw, _ := gfx.MeasureText(tip.Title, th.Body().Weighted(gfx.Bold), scale)
		w = min(w, tw+2*pad+2)
	}
	inner := w - 2*pad
	body, foot := th.Small().WithColor(th.Text), th.Small().WithColor(th.TextDim)
	var bodyLines, footLines []string
	h := pad
	if tip.Title != "" {
		h += 20
	}
	if tip.Body != "" {
		bodyLines = gfx.WrapText(tip.Body, inner, body, scale)
		h += float32(len(bodyLines))*gfx.LineHeight(body, scale) + 4
	}
	h += float32(len(tip.Rows)) * 17
	if len(tip.Rows) > 0 {
		h += 4
	}
	if tip.Foot != "" {
		footLines = gfx.WrapText(tip.Foot, inner, foot, scale)
		h += float32(len(footLines))*gfx.LineHeight(foot, scale) + 4
	}
	h += pad - 4
	x := c.In.X + 16
	y := c.In.Y + 18
	if x+w > c.In.W-4 {
		x = c.In.X - w - 12
	}
	if y+h > c.In.H-4 {
		y = c.In.Y - h - 10
	}
	x, y = max(4, x), max(4, y)
	r := gfx.R(x, y, w, h)
	old := c.SetLayer(LayerTip)
	c.Paint(func(cv *gfx.Canvas) {
		cv.Shadow(r.Translate(0, 4), 12, th.Shadow)
		cv.Panel(r, 5, gfx.CornersDiag, th.Well, th.Rule)
		cy := r.Y + pad
		if tip.Title != "" {
			cv.Text(tip.Title, r.X+pad, cy, th.Body().Weighted(gfx.Bold).WithColor(th.Rule))
			cy += 20
		}
		if len(bodyLines) > 0 {
			cy += cv.TextLines(bodyLines, r.X+pad, cy, body) + 4
		}
		for _, row := range tip.Rows {
			col := row.Col
			if col == (gfx.Color{}) {
				col = th.Text
			}
			cv.Text(row.K, r.X+pad, cy, th.Small().WithColor(th.TextDim))
			cv.Text(row.V, r.Right()-pad, cy, th.Small().WithColor(col).Aligned(gfx.AlignRight))
			cy += 17
		}
		if len(tip.Rows) > 0 {
			cy += 4
		}
		if len(footLines) > 0 {
			cv.TextLines(footLines, r.X+pad, cy, foot)
		}
	})
	c.SetLayer(old)
}

// ------------------------------------------------------------------ helpers

// Upper returns s in upper case; headings use it.
func Upper(s string) string { return strings.ToUpper(s) }

// Pulse returns a value swinging between 0 and 1 with the given period in
// seconds, for attention-seeking highlights.
func (c *Ctx) Pulse(period float32) float32 {
	return float32(0.5 + 0.5*math.Sin(float64(c.Time)*2*math.Pi/float64(period)))
}
