package art

import (
	"math"

	"drifthold/internal/gfx"
)

// Interface glyphs are authored on a 16 × 16 grid whose coordinates name the
// centre lines of strokes. At draw time the stroke width is chosen as a
// whole number of pixels and every coordinate is snapped so that horizontal
// and vertical strokes cover whole pixels and mirror-image parts stay mirror
// images: the glyphs are sharp at any scale, not just at multiples of the
// grid. Slanted strokes and curves are anti-aliased between snapped end
// points.
//
// Glyphs are rendered once per pixel size in white and tinted when drawn.

// ipen draws one glyph.
type ipen struct {
	cv  *gfx.Canvas
	s   float32 // pixels per point of cv
	upx float32 // pixels per grid unit
	c   float32 // pixel coordinate of the grid centre (8, 8) on both axes
	sw  float32 // stroke width in pixels
	col gfx.Color
}

// px returns the pixel coordinate of the grid coordinate u, snapped to a
// position where a stroke of width sw covers whole pixels.
func (p *ipen) px(u float32) float32 {
	// Distances from the centre are rounded, halves downwards with a little
	// slack, so that 8-d and 8+d land the same distance from the centre
	// even when d·upx is a tie that floating point tips either way.
	d := float64((u - 8) * p.upx)
	if d < 0 {
		return p.c - float32(math.Floor(-d+0.499))
	}
	return p.c + float32(math.Floor(d+0.499))
}

// pt converts a snapped pixel coordinate pair to canvas points.
func (p *ipen) pt(x, y float32) gfx.Pt {
	return gfx.Pt{X: p.px(x) / p.s, Y: p.px(y) / p.s}
}

// rectPx fills the pixel rectangle (x0, y0)-(x1, y1).
func (p *ipen) rectPx(x0, y0, x1, y1 float32) {
	p.cv.FillRect(gfx.R(x0/p.s, y0/p.s, (x1-x0)/p.s, (y1-y0)/p.s), p.col)
}

// hline strokes a horizontal line from x0 to x1 at y, with square ends.
func (p *ipen) hline(x0, x1, y float32) {
	h := p.sw / 2
	p.rectPx(p.px(x0)-h, p.px(y)-h, p.px(x1)+h, p.px(y)+h)
}

// vline strokes a vertical line from y0 to y1 at x, with square ends.
func (p *ipen) vline(x, y0, y1 float32) {
	h := p.sw / 2
	p.rectPx(p.px(x)-h, p.px(y0)-h, p.px(x)+h, p.px(y1)+h)
}

// fill fills the rectangle whose outline strokes would run along the given
// centre lines.
func (p *ipen) fill(x0, y0, x1, y1 float32) {
	h := p.sw / 2
	p.rectPx(p.px(x0)-h, p.px(y0)-h, p.px(x1)+h, p.px(y1)+h)
}

// frame strokes the outline of a rectangle.
func (p *ipen) frame(x0, y0, x1, y1 float32) {
	p.hline(x0, x1, y0)
	p.hline(x0, x1, y1)
	p.vline(x0, y0, y1)
	p.vline(x1, y0, y1)
}

// dot fills a square one stroke wide, grown by extra pixels on every side.
func (p *ipen) dot(x, y, extra float32) {
	h := p.sw/2 + float32(math.Round(float64(extra*p.upx)))
	p.rectPx(p.px(x)-h, p.px(y)-h, p.px(x)+h, p.px(y)+h)
}

// slant returns the width of an anti-aliased stroke that looks as heavy as
// a snapped one.
func (p *ipen) slant() float32 {
	if p.sw <= 1 {
		return 1.3 / p.s
	}
	return p.sw * 1.08 / p.s
}

// path strokes a line through the grid points given as x, y pairs.
func (p *ipen) path(closed bool, xy ...float32) {
	ptsBuf = ptsBuf[:0]
	for i := 0; i+1 < len(xy); i += 2 {
		ptsBuf = append(ptsBuf, p.pt(xy[i], xy[i+1]))
	}
	p.cv.StrokePolyline(ptsBuf, closed, gfx.Stroke{Width: p.slant(), Cap: gfx.CapRound}, p.col)
}

// heavy is path with a stroke half as thick again, for check marks and
// chevrons.
func (p *ipen) heavy(xy ...float32) {
	ptsBuf = ptsBuf[:0]
	for i := 0; i+1 < len(xy); i += 2 {
		ptsBuf = append(ptsBuf, p.pt(xy[i], xy[i+1]))
	}
	p.cv.StrokePolyline(ptsBuf, false, gfx.Stroke{Width: p.slant() * 1.45, Cap: gfx.CapRound}, p.col)
}

// poly fills the polygon through the grid points and strokes its outline,
// so that it is as large as a stroked shape with the same centre lines.
func (p *ipen) poly(xy ...float32) {
	ptsBuf = ptsBuf[:0]
	for i := 0; i+1 < len(xy); i += 2 {
		ptsBuf = append(ptsBuf, p.pt(xy[i], xy[i+1]))
	}
	p.cv.FillPoly(ptsBuf, p.col)
	p.cv.StrokePoly(ptsBuf, p.sw/p.s, p.col)
}

// radius snaps a radius in grid units to whole pixels, in points.
func (p *ipen) radius(r float32) float32 {
	return max(1, float32(math.Round(float64(r*p.upx)))) / p.s
}

// ring strokes a circle.
func (p *ipen) ring(cx, cy, r float32) {
	q := p.pt(cx, cy)
	p.cv.StrokeCircle(q.X, q.Y, p.radius(r), p.slant(), p.col)
}

// disc fills a circle whose rim stroke would have radius r.
func (p *ipen) disc(cx, cy, r float32) {
	q := p.pt(cx, cy)
	p.cv.FillCircle(q.X, q.Y, p.radius(r)+p.sw/2/p.s, p.col)
}

// arc strokes part of a circle with round ends; angles as on a Canvas.
func (p *ipen) arc(cx, cy, r, a0, a1 float32) {
	q := p.pt(cx, cy)
	p.cv.StrokeArc(q.X, q.Y, p.radius(r), a0, a1, gfx.Stroke{Width: p.slant(), Cap: gfx.CapRound}, p.col)
}

// half fills the upper (up = true) or lower half of a disc.
func (p *ipen) half(cx, cy, r float32, up bool) {
	q := p.pt(cx, cy)
	rr := p.radius(r) + p.sw/2/p.s
	if up {
		p.cv.FillSector(q.X, q.Y, rr, pi, tau, p.col)
	} else {
		p.cv.FillSector(q.X, q.Y, rr, 0, pi, p.col)
	}
}

// iconStroke returns the stroke width in pixels for a glyph box of the
// given pixel size.
func iconStroke(box int) float32 {
	return max(1, float32(math.Floor(float64(box)*1.4/16+0.5)))
}

func drawIcon(cv *gfx.Canvas, id IconID, r gfx.Rect, col gfx.Color) {
	if cv == nil || cv.Dst == nil || id < 0 || id >= NumIcons || col.A == 0 || r.Empty() {
		return
	}
	s := canvasScale(cv)
	box := int(math.Round(float64(min(r.W, r.H) * s)))
	if box < 6 {
		box = 6
	}
	img := sprite(domIcon|uint64(id), box, box, s, func(sc *gfx.Canvas) {
		sw := iconStroke(box)
		// The glyph box takes the parity of the stroke so that centred
		// strokes can be centred exactly.
		inner := box
		if (box-int(sw))%2 != 0 {
			inner--
		}
		p := ipen{cv: sc, s: s, upx: float32(box) / 16, c: float32(inner) / 2, sw: sw, col: gfx.White}
		iconArts[id](&p)
	})
	cv.DrawSprite(img, r.MidX(), r.MidY(), &gfx.ImageOpts{Tint: col})
}

var iconArts = [NumIcons]func(p *ipen){
	IconPower:      iconPower,
	IconColonists:  iconColonists,
	IconMorale:     iconMorale,
	IconScrip:      iconScrip,
	IconResearch:   iconResearch,
	IconFleet:      iconFleet,
	IconBelt:       iconBelt,
	IconSurface:    iconSurface,
	IconPause:      iconPause,
	IconPlay:       iconPlay,
	IconFast:       iconFast,
	IconFaster:     iconFaster,
	IconMenu:       iconMenu,
	IconClose:      iconClose,
	IconHelp:       iconHelp,
	IconDirectives: iconDirectives,
	IconExchange:   iconExchange,
	IconHolds:      iconHolds,
	IconWarning:    iconWarning,
	IconHousing:    iconHousing,
	IconAir:        iconAir,
	IconStorage:    iconStorage,
	IconBerth:      iconBerth,
	IconShield:     iconShield,
	IconTarget:     iconTarget,
	IconWorker:     iconWorker,
	IconCycle:      iconCycle,
	IconDeposit:    iconDeposit,
	IconDemolish:   iconDemolish,
	IconPriority:   iconPriority,
	IconOnOff:      iconOnOff,
	IconLog:        iconLog,
	IconSave:       iconSave,
	IconCheck:      iconCheck,
	IconArrowLeft:  func(p *ipen) { p.heavy(10, 3, 5, 8, 10, 13) },
	IconArrowRight: func(p *ipen) { p.heavy(6, 3, 11, 8, 6, 13) },
	IconArrowUp:    func(p *ipen) { p.heavy(3, 10.5, 8, 5.5, 13, 10.5) },
	IconArrowDown:  func(p *ipen) { p.heavy(3, 5.5, 8, 10.5, 13, 5.5) },
	IconLock:       iconLock,
}

func iconPower(p *ipen) {
	p.poly(9.5, 1.5, 4, 9, 7.6, 9, 6.5, 14.5, 12, 7, 8.4, 7)
}

func iconColonists(p *ipen) {
	// The one in front, solid; the one behind, in outline.
	p.disc(6, 5, 2)
	p.half(6, 14, 4, true)
	p.ring(11.5, 4.5, 1.6)
	p.arc(11.2, 14, 3.6, -1.35, -0.12)
}

func iconMorale(p *ipen) {
	p.ring(8, 8, 6)
	p.dot(5.8, 6.3, 0.35)
	p.dot(10.2, 6.3, 0.35)
	p.arc(8, 7.6, 3.4, 0.55, pi-0.55)
}

func iconScrip(p *ipen) {
	p.ring(8, 8, 6)
	p.frame(6.4, 6.4, 9.6, 9.6)
}

func iconResearch(p *ipen) {
	p.hline(5.5, 10.5, 2)
	p.path(false, 6.5, 2, 6.5, 6.2, 2.6, 13.6, 13.4, 13.6, 9.5, 6.2, 9.5, 2)
	p.poly(5, 10.6, 11, 10.6, 12.6, 13.4, 3.4, 13.4)
}

func iconFleet(p *ipen) {
	for _, d := range [3][2]float32{{8, 4.4}, {3.6, 11}, {12.4, 11}} {
		x, y := d[0], d[1]
		p.poly(x, y-2.6, x+1.9, y+2.2, x, y+1.1, x-1.9, y+2.2)
	}
}

func iconBelt(p *ipen) {
	p.disc(8, 8, 0.9)
	// A tilted orbit with one body on it.
	const rx, ry, tilt = 6.8, 3.4, -0.42
	var xy [48]float32
	for i := 0; i < 24; i++ {
		a := tau * float32(i) / 24
		u, v := rx*cosf(a), ry*sinf(a)
		xy[2*i] = 8 + u*cosf(tilt) - v*sinf(tilt)
		xy[2*i+1] = 8 + u*sinf(tilt) + v*cosf(tilt)
	}
	// Not snapped: an ellipse has no straight runs to keep sharp.
	ptsBuf = ptsBuf[:0]
	for i := 0; i < 24; i++ {
		ptsBuf = append(ptsBuf, gfx.Pt{X: (p.c + (xy[2*i]-8)*p.upx) / p.s, Y: (p.c + (xy[2*i+1]-8)*p.upx) / p.s})
	}
	p.cv.StrokePoly(ptsBuf, p.slant(), p.col)
	bx, by := xy[2*21], xy[2*21+1]
	p.cv.FillCircle((p.c+(bx-8)*p.upx)/p.s, (p.c+(by-8)*p.upx)/p.s, (0.9*p.upx+p.sw/2)/p.s, p.col)
}

func iconSurface(p *ipen) {
	hexPath := func(r float32, fill bool) {
		var xy [12]float32
		for i := 0; i < 6; i++ {
			a := -pi/2 + float32(i)*tau/6
			xy[2*i], xy[2*i+1] = 8+r*cosf(a), 8+r*sinf(a)
		}
		if fill {
			p.poly(xy[:]...)
		} else {
			p.path(true, xy[:]...)
		}
	}
	hexPath(6.4, false)
	hexPath(2.3, true)
}

func iconPause(p *ipen) {
	p.fill(4.4, 3.5, 6.2, 12.5)
	p.fill(9.8, 3.5, 11.6, 12.5)
}

func iconPlay(p *ipen) {
	p.poly(4.5, 2.8, 12.8, 8, 4.5, 13.2)
}

func iconFast(p *ipen) {
	p.poly(2.2, 3.8, 7.6, 8, 2.2, 12.2)
	p.poly(8.6, 3.8, 14, 8, 8.6, 12.2)
}

func iconFaster(p *ipen) {
	for _, x := range [3]float32{1.4, 6, 10.6} {
		p.poly(x, 4.2, x+3.9, 8, x, 11.8)
	}
}

func iconMenu(p *ipen) {
	for _, y := range [3]float32{4, 8, 12} {
		p.hline(2.5, 13.5, y)
	}
}

func iconClose(p *ipen) {
	p.heavy(3.8, 3.8, 12.2, 12.2)
	p.heavy(12.2, 3.8, 3.8, 12.2)
}

func iconHelp(p *ipen) {
	p.arc(8, 5.6, 3, pi, tau+0.85)
	p.path(false, 9.98, 7.85, 8, 9.4, 8, 10.4)
	p.dot(8, 13.2, 0.3)
}

func iconDirectives(p *ipen) {
	for i, y := range [3]float32{3.6, 8, 12.4} {
		if i < 2 {
			p.path(false, 2, y, 3.4, y+1.4, 5.6, y-1.6)
		} else {
			p.dot(3.8, y, 0.3)
		}
		p.hline(8, 13.6, y)
	}
}

func iconExchange(p *ipen) {
	p.vline(8, 2.6, 13.4)
	p.hline(5.2, 10.8, 13.6)
	p.hline(2.8, 13.2, 3.6)
	for _, x := range [2]float32{3, 13} {
		p.path(false, x-2.1, 9.2, x, 3.6, x+2.1, 9.2)
		p.half(x, 9.4, 2.3, false)
	}
}

func iconHolds(p *ipen) {
	for _, y := range [3]float32{4, 8, 12} {
		p.dot(3, y, 0.45)
		p.hline(6.4, 13.6, y)
	}
}

func iconWarning(p *ipen) {
	p.path(true, 8, 2, 14.6, 13.4, 1.4, 13.4)
	p.vline(8, 6.4, 9.6)
	p.dot(8, 11.6, 0.1)
}

func iconHousing(p *ipen) {
	p.path(false, 1.6, 8, 8, 2.2, 14.4, 8)
	p.vline(3.6, 8, 13.6)
	p.vline(12.4, 8, 13.6)
	p.hline(3.6, 12.4, 13.6)
	p.fill(7.2, 10.4, 8.8, 13.6)
}

func iconAir(p *ipen) {
	p.hline(2, 8.4, 5.6)
	p.arc(8.4, 3.8, 1.8, pi/2, -pi*0.85)
	p.hline(2, 11.6, 8.6)
	p.arc(11.6, 6.8, 1.8, pi/2, -pi*0.85)
	p.hline(4.4, 9.4, 11.6)
	p.arc(9.4, 13.2, 1.6, -pi/2, pi*0.8)
}

func iconStorage(p *ipen) {
	p.frame(2.6, 4.4, 13.4, 13.4)
	p.hline(2.6, 13.4, 7.4)
	p.fill(6.8, 6.4, 9.2, 9)
}

func iconBerth(p *ipen) {
	p.ring(8, 3.4, 1.6)
	p.vline(8, 5, 13.6)
	p.hline(5.4, 10.6, 7.2)
	p.arc(8, 8.4, 5.2, 0.25, pi-0.25)
	p.path(false, 13.6, 8.4, 13.2, 9.8, 11.8, 9.6)
	p.path(false, 2.4, 8.4, 2.8, 9.8, 4.2, 9.6)
}

func iconShield(p *ipen) {
	p.path(true, 8, 1.8, 13.4, 3.8, 13.4, 8.2, 11.8, 11.6, 8, 14.4, 4.2, 11.6, 2.6, 8.2, 2.6, 3.8)
	p.poly(8, 1.8, 8, 14.4, 4.2, 11.6, 2.6, 8.2, 2.6, 3.8)
}

func iconTarget(p *ipen) {
	p.ring(8, 8, 4.6)
	p.hline(1.4, 4.4, 8)
	p.hline(11.6, 14.6, 8)
	p.vline(8, 1.4, 4.4)
	p.vline(8, 11.6, 14.6)
	p.dot(8, 8, 0.15)
}

func iconWorker(p *ipen) {
	// A hard hat, a face under its brim, shoulders.
	p.half(8, 5.6, 3.3, true)
	p.hline(3.4, 12.6, 5.8)
	p.half(8, 7.4, 2.2, false)
	p.half(8, 14.2, 4.6, true)
}

func iconCycle(p *ipen) {
	p.ring(8, 8, 6)
	p.path(false, 8, 4.2, 8, 8, 10.8, 9.8)
}

func iconDeposit(p *ipen) {
	p.path(true, 4.8, 2.8, 11.2, 2.8, 14.2, 6.6, 8, 14, 1.8, 6.6)
	p.hline(1.8, 14.2, 6.6)
	p.path(false, 5.8, 6.6, 8, 14, 10.2, 6.6)
}

func iconDemolish(p *ipen) {
	p.heavy(2.8, 13.4, 9, 6.4)
	p.poly(14.3, 6.8, 11.9, 9.6, 6.1, 4.5, 8.5, 1.7)
	p.dot(12.6, 12.8, 0.25)
	p.dot(14.2, 10.6, 0.1)
}

func iconPriority(p *ipen) {
	p.heavy(3.6, 7.8, 8, 3.4, 12.4, 7.8)
	p.heavy(3.6, 12.6, 8, 8.2, 12.4, 12.6)
}

func iconOnOff(p *ipen) {
	p.arc(8, 8.6, 5.4, -pi/2+0.72, 3*pi/2-0.72)
	p.vline(8, 1.8, 7.8)
}

func iconLog(p *ipen) {
	p.path(true, 3.4, 2, 9.6, 2, 12.6, 5, 12.6, 14, 3.4, 14)
	p.path(false, 9.6, 2, 9.6, 5, 12.6, 5)
	p.hline(5.6, 10.4, 8)
	p.hline(5.6, 10.4, 11)
}

func iconSave(p *ipen) {
	p.path(true, 2.4, 2.4, 11, 2.4, 13.6, 5, 13.6, 13.6, 2.4, 13.6)
	p.fill(5.2, 2.4, 9.6, 5.4)
	p.frame(4.8, 8.8, 11.2, 13.6)
}

func iconCheck(p *ipen) {
	p.heavy(2.8, 8.6, 6.4, 12.2, 13.2, 4.2)
}

func iconLock(p *ipen) {
	p.arc(8, 6.6, 3, pi, tau)
	p.vline(5, 6.6, 7.6)
	p.vline(11, 6.6, 7.6)
	p.frame(3.4, 7.6, 12.6, 13.6)
	p.fill(7.4, 9.8, 8.6, 11.6)
}
