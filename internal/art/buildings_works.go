package art

import (
	"math"

	"drifthold/internal/gfx"
)

// wrap returns v reduced to [0, period): the phase of a cycle, also for
// negative v.
func wrap(v, period float32) float32 {
	w := float64(v) - float64(period)*math.Floor(float64(v)/float64(period))
	if !(w >= 0) || w >= float64(period) {
		return 0
	}
	return float32(w)
}

// frac returns the fractional part of v, in [0, 1).
func frac(v float32) float32 { return wrap(v, 1) }

// ---------------------------------------------------------------------------
// Bore Rig: a square drill frame on four splayed feet over a bore hole, a
// spoil chute off one side, the cutter head turning in the middle.
// ---------------------------------------------------------------------------

func boreRigBody(b *bpen) {
	b.block(matIron, 0.04, 0.4, -0.1, 0.4, 0.2)
	if !b.tiny() {
		for i := 0; i < 3; i++ {
			b.rect(0.5+float32(i)*0.1, -0.06, 0.035, 0.12, matDark.lo)
		}
	}
	for _, sx := range [2]float32{-1, 1} {
		for _, sy := range [2]float32{-1, 1} {
			b.strut(sx*0.34, sy*0.34, sx*0.61, sy*0.5, 0.09, matIron.mid)
			b.drum(matDark, 0.03, sx*0.62, sy*0.5, 0.1)
		}
	}
	b.block(matSteel, 0.14, -0.44, -0.44, 0.88, 0.88)
	b.rect(-0.295, -0.295, 0.59, 0.59, matDark.lo)
	b.outline(inkLine, b.ow(), 1.2, -0.295, -0.295, 0.295, -0.295, 0.295, 0.295, -0.295, 0.295)
	for _, sx := range [2]float32{-1, 1} {
		for _, sy := range [2]float32{-1, 1} {
			b.rect(sx*0.37-0.045, sy*0.37-0.045, 0.09, 0.09, b.acc.mid)
		}
	}
	b.line(-0.28, -0.28, 0.28, 0.28, 0.05, 1, matIron.lo)
	b.line(-0.28, 0.28, 0.28, -0.28, 0.05, 1, matIron.lo)
	b.well(0, 0, 0.22, colPit)
}

func boreRigMotion(b *bpen) {
	a := b.t * 2.6
	for i := 0; i < 3; i++ {
		aa := a + float32(i)*tau/3
		c, s := cosf(aa), sinf(aa)
		b.poly(matSteel.mid, 0.05*c, 0.05*s, 0.2*c-0.1*s, 0.2*s+0.1*c, 0.21*c+0.05*s, 0.21*s-0.05*c)
	}
	// The drive piston pumps as the head turns.
	r := 0.085 + 0.02*sinf(b.t*5.2)
	if !b.lit {
		r = 0.085
	}
	b.circle(0, 0, r+0.025, inkLine)
	b.circle(0, 0, r, b.acc.mid)
}

// ---------------------------------------------------------------------------
// Smelter: a heavy furnace hall with an ore hopper, a chimney, and an open
// crucible of molten metal at its side.
// ---------------------------------------------------------------------------

const (
	smelterPotX = 0.43
	smelterPotY = 0.17
)

func smelterBody(b *bpen) {
	b.drum(matIron, 0.28, 0.45, -0.43, 0.14)
	b.well(0.45, -0.43, 0.08, colPit)
	b.block(matPlate, 0.14, -0.7, -0.5, 0.92, 1.0)
	// The ore hopper on the roof.
	b.poly(gfx.RGB(0xC0714F), -0.61, -0.415, -0.27, -0.415, -0.32, -0.15, -0.56, -0.15)
	b.outline(inkLine, b.ow(), 1.2, -0.61, -0.415, -0.27, -0.415, -0.32, -0.15, -0.56, -0.15)
	if !b.tiny() {
		b.circle(-0.5, -0.3, 0.035, gfx.RGB(0x8A4E36))
		b.circle(-0.39, -0.26, 0.03, gfx.RGB(0xDA8E6C))
	}
	// Roof vents, glowing with the heat below.
	for _, y := range [2]float32{-0.02, 0.17} {
		b.rect(-0.56, y, 0.62, 0.07, inkLine)
		b.rect(-0.54, y+0.015, 0.58, 0.04, b.lamp(colHot))
	}
	if b.lit && !b.tiny() {
		b.glow(-0.25, 0.1, 0.4, alpha(colHot, 0.3))
	}
	b.rect(-0.64, 0.345, 0.8, 0.08, b.acc.mid)
	b.drum(matDark, 0.1, smelterPotX, smelterPotY, 0.36)
	slag := gfx.RGB(0x2A2C35)
	if b.lit {
		slag = colHot
	}
	b.well(smelterPotX, smelterPotY, 0.255, slag)
}

func smelterMotion(b *bpen) {
	if !b.lit {
		b.circle(smelterPotX, smelterPotY, 0.13, gfx.RGB(0x3A3C46))
		return
	}
	v := pulse(b.t, 1.9, 0, 1)
	b.glow(smelterPotX, smelterPotY, 0.7, alpha(colHot, 0.4+0.25*v))
	b.circle(smelterPotX+0.02*sinf(b.t*0.9), smelterPotY+0.02*cosf(b.t*1.1), 0.13+0.04*v, colHotCore)
	// Sparks jumping from the pot.
	for i := 0; i < 2; i++ {
		f := frac(b.t*0.9 + float32(i)*0.53)
		a := 3.9 + float32(i)*1.7 + wrap(float32(math.Floor(float64(b.t*0.9+float32(i)*0.53)))*2.4, tau)
		d := 0.2 + 0.4*f
		b.circle(smelterPotX+d*cosf(a), smelterPotY+d*sinf(a)-0.15*f, max(0.025, 1.1*b.px), alpha(colHotCore, 1-f))
	}
	// Chimney smoke.
	for k := 0; k < 2; k++ {
		d := frac(b.t*0.3 + float32(k)/2)
		b.circle(0.45+0.26*d, -0.43-0.5*d, 0.06+0.09*d, gfx.Color{R: 0xC8, G: 0xB8, B: 0xB0, A: uint8(64 * (1 - d))})
	}
}

// ---------------------------------------------------------------------------
// Cracker: a distillation column and a cascade of two smaller fuel tanks
// along a pipe run, with a flare burning at the end of a boom.
// ---------------------------------------------------------------------------

const (
	crackerFlareX = 0.2
	crackerFlareY = -0.62
)

func crackerBody(b *bpen) {
	// The skid under the fuel tanks and the ice intake beside the column.
	b.slab(matPad, 0.03, -0.14, -0.1, 0.3, -0.2, 0.74, 0.34, 0.6, 0.62, 0.16, 0.46, -0.18, 0.2)
	b.strut(-0.56, 0.1, -0.44, -0.02, 0.07, matIron.mid)
	b.block(matIce, 0.06, -0.74, 0.06, 0.22, 0.17)
	b.strut(-0.38, -0.24, 0.16, 0.12, 0.13, matCopper.mid)
	b.strut(0.16, 0.12, 0.52, 0.4, 0.1, matCopper.mid)
	// The flare boom and its shadow.
	b.line(-0.2, -0.26, crackerFlareX+0.1, crackerFlareY+0.14, 0.05, 1, inkShadow)
	b.strut(-0.3, -0.34, crackerFlareX, crackerFlareY, 0.045, matIron.hi)
	b.drum(matDark, 0, crackerFlareX, crackerFlareY, 0.065)

	b.drum(matSteel, 0.3, -0.38, -0.24, 0.34)
	b.ring(-0.38, -0.24, 0.235, 0.05, 1, matIce.mid)
	b.drum(matIron, 0, -0.38, -0.24, 0.13)
	b.circle(-0.38, -0.24, 0.06, b.acc.mid)

	b.drum(matCream, 0.14, 0.16, 0.12, 0.245)
	b.ring(0.16, 0.12, 0.15, 0.065, 1.2, matFuel.mid)
	b.drum(matCream, 0.1, 0.52, 0.4, 0.165)
	b.ring(0.52, 0.4, 0.09, 0.055, 1.2, matFuel.mid)
}

func crackerMotion(b *bpen) {
	if !b.lit {
		return
	}
	f := 0.6 + 0.25*sinf(b.t*17) + 0.15*sinf(b.t*7.1+1)
	b.glow(crackerFlareX, crackerFlareY, 0.34, alpha(colHot, 0.6*f))
	r := 0.05 + 0.02*f
	b.circle(crackerFlareX+0.02, crackerFlareY-0.03, r+0.02, colHot)
	b.circle(crackerFlareX+0.01, crackerFlareY-0.015, r*0.6, colHotCore)
}

// ---------------------------------------------------------------------------
// Fabricator: a saw-tooth-roofed works with a delivery pad where an arm
// sets down finished parts.
// ---------------------------------------------------------------------------

func fabricatorBody(b *bpen) {
	b.block(matPad, 0.03, 0.3, -0.24, 0.5, 0.56)
	if !b.tiny() {
		pc := gfx.RGB(0x7D95C9)
		b.rect(0.44, 0.14, 0.1, 0.1, pc)
		b.rect(0.58, 0.14, 0.1, 0.1, pc)
		b.rect(0.44, 0.02, 0.1, 0.1, gfx.Lighten(pc, 0.2))
	}
	b.block(matSteel, 0.14, -0.72, -0.52, 1.0, 1.04)
	// Saw-tooth roof: a glazed north light and a shaded slope per bay.
	glass := gfx.RGB(0xECDDAE)
	if !b.lit {
		glass = gfx.RGB(0x2A3346)
	}
	const bay = 0.232
	for i := 0; i < 4; i++ {
		x := -0.68 + float32(i)*bay
		b.rect(x, -0.47, 0.085, 0.8, glass)
		b.rect(x+bay-0.05, -0.47, 0.05, 0.8, matSteel.lo)
	}
	b.rect(-0.68, 0.36, 0.92, 0.09, b.acc.mid)
	if b.lit && !b.tiny() {
		b.glow(-0.25, -0.1, 0.55, alpha(colWindow, 0.14))
	}
}

func fabricatorMotion(b *bpen) {
	// The arm swings between the works and the pad.
	u := smooth(0.5 + 0.5*sinf(b.t*1.3))
	if !b.lit {
		u = 0.2
	}
	const sx, sy = 0.36, -0.16
	hx, hy := lerpf(0.4, 0.66, u), lerpf(0.0, 0.2, u)
	// Elbow: off the straight line, so the arm visibly bends.
	mx, my := (sx+hx)/2, (sy+hy)/2
	dx, dy := hx-sx, hy-sy
	l := float32(math.Hypot(float64(dx), float64(dy)))
	bend := 0.5 * float32(math.Sqrt(float64(max(0.04, 0.2*0.2-l*l/4))))
	ex, ey := mx+dy/l*bend, my-dx/l*bend
	b.strut(sx, sy, ex, ey, 0.05, matSteel.hi)
	b.strut(ex, ey, hx, hy, 0.04, matSteel.hi)
	b.circle(sx, sy, 0.07, inkLine)
	b.circle(sx, sy, 0.05, b.acc.mid)
	b.circle(hx, hy, 0.04, b.lamp(gfx.RGB(0xBFE9FF)))
	if b.lit && u > 0.9 {
		b.glow(hx, hy, 0.22, alpha(gfx.RGB(0xBFE9FF), 0.7))
	}
}

// ---------------------------------------------------------------------------
// Laboratory: a domed hall with a glowing oculus on a long bench wing, the
// outline of a keyhole.
// ---------------------------------------------------------------------------

const (
	labDomeY = -0.3
	labGlow  = 0x8FE3FF
)

func laboratoryBody(b *bpen) {
	b.strut(0.3, 0.36, 0.42, 0.36, 0.05, matIron.mid)
	b.drum(matSteel, 0.08, 0.53, 0.36, 0.14)
	b.circle(0.53, 0.36, 0.06, b.acc.mid)
	b.block(matSteel, 0.1, -0.3, -0.05, 0.6, 0.76)
	b.rect(-0.24, 0.57, 0.48, 0.075, b.acc.mid)
	for _, x := range [2]float32{-0.13, 0.13} {
		b.rect(x-0.085, 0.3, 0.17, 0.13, inkLine)
		b.window(x, 0.365, 0.12, 0.08, gfx.RGB(labGlow))
	}
	b.dome(matCream, 0.14, 0, labDomeY, 0.47)
	b.well(0, labDomeY, 0.195, b.lamp(gfx.RGB(labGlow)))
	if !b.lit {
		b.circle(-0.03, labDomeY-0.04, 0.09, gfx.RGB(0x2B3550))
	}
}

func laboratoryMotion(b *bpen) {
	if !b.lit {
		return
	}
	v := pulse(b.t, 3.1, 0, 1)
	b.glow(0, labDomeY, 0.5, alpha(gfx.RGB(labGlow), 0.3+0.3*v))
	b.circle(0, labDomeY, 0.07+0.03*v, gfx.RGB(0xF2FCFF))
	// Two motes circling the oculus, like a diagram of an atom.
	for i := 0; i < 2; i++ {
		a := b.t*2.1*(1-0.35*float32(i)) + float32(i)*2
		r := 0.3 + 0.07*float32(i)
		b.circle(r*cosf(a), labDomeY+r*sinf(a), max(0.03, 1.1*b.px), gfx.RGB(0xDFF7FF))
	}
}

// ---------------------------------------------------------------------------
// Launch Cradle: a catapult rail running up the tile between two gantry
// towers, a hull on the slip, running lights up the towers.
// ---------------------------------------------------------------------------

func launchCradleBody(b *bpen) {
	b.block(matDark, 0.03, -0.25, 0.5, 0.5, 0.3)
	b.block(matIron, 0.06, -0.2, -0.86, 0.4, 1.42)
	b.rect(-0.115, -0.82, 0.04, 1.34, matSteel.hi)
	b.rect(0.075, -0.82, 0.04, 1.34, matSteel.hi)
	// Launch chevrons at the head of the rail.
	for i := 0; i < 2; i++ {
		y := -0.72 + float32(i)*0.15
		b.polyline(b.acc.hi, 0.05, 1.2, -0.11, y+0.08, 0, y, 0.11, y+0.08)
	}
	for _, y := range [3]float32{-0.3, 0.02, 0.34} {
		b.block(matIron, 0.08, -0.46, y-0.05, 0.92, 0.1)
	}
	for _, x := range [2]float32{-0.62, 0.45} {
		b.block(matSteel, 0.16, x, -0.44, 0.17, 0.9)
		b.rect(x+0.04, -0.4, 0.09, 0.07, b.acc.mid)
		b.rect(x+0.04, 0.35, 0.09, 0.07, b.acc.mid)
	}
	// The hull on the slip.
	hull := [...]float32{0, -0.42, 0.12, -0.14, 0.12, 0.2, 0.07, 0.31, -0.07, 0.31, -0.12, 0.2, -0.12, -0.14}
	b.shadowPoly(0.06, hull[:]...)
	b.poly(matCream.mid, hull[:]...)
	b.poly(matCream.hi, 0, -0.42, 0, 0.31, -0.07, 0.31, -0.12, 0.2, -0.12, -0.14)
	b.outline(inkLine, b.ow(), 1, hull[:]...)
	if !b.tiny() {
		b.rect(-0.1, -0.04, 0.2, 0.03, matCream.lo)
		b.rect(-0.1, 0.1, 0.2, 0.03, matCream.lo)
	}
}

func launchCradleMotion(b *bpen) {
	for _, x := range [2]float32{-0.535, 0.535} {
		for i := 0; i < 3; i++ {
			f := frac(b.t*0.55 + float32(i)/3)
			y := 0.3 - 0.6*f
			if !b.lit {
				b.circle(x, 0.24-0.24*float32(i), 0.035, colWindowOff)
				continue
			}
			b.circle(x, y, 0.04, alpha(colLamp, sinf(f*pi)))
		}
	}
	if b.lit && frac(b.t*0.8) < 0.3 {
		b.glow(0, -0.8, 0.26, alpha(gfx.RGB(0xFF6A4A), 0.8))
		b.circle(0, -0.8, 0.04, gfx.RGB(0xFFC4B0))
	}
}

// ---------------------------------------------------------------------------
// Dock Ring: a mooring ring floating over its anchor on three tethers,
// with clamps towards the tile corners and marker lights going round.
// ---------------------------------------------------------------------------

func dockRingBody(b *bpen) {
	b.drum(matIron, 0.05, 0, 0, 0.2)
	b.circle(0, 0, 0.1, b.acc.mid)
	for i := 0; i < 3; i++ {
		a := -pi/2 + float32(i)*tau/3 + tau/6
		b.line(0.17*cosf(a), 0.17*sinf(a), 0.6*cosf(a), 0.6*sinf(a), 0.03, 1, alpha(matCream.hi, 0.85))
	}
	const r, w = 0.64, 0.16
	// The ring hangs well above the ground: its shadow lies far off.
	b.ring(0.16, 0.2, r, w, 2, inkShadow)
	for i := 0; i < 3; i++ {
		f := b.frame(-pi/2 + float32(i)*tau/3)
		f.slab(b.acc, 0, 0.68, -0.11, 0.9, -0.075, 0.9, 0.075, 0.68, 0.11)
	}
	b.ring(0, 0, r, w+2*b.ow(), 3, inkLine)
	b.ring(0, 0, r, w, 2, matSteel.mid)
	b.arc(0, 0, r+w/2-b.bw()/2, litAngle-1.1, litAngle+1.1, b.bw(), 1, matSteel.hi)
	b.arc(0, 0, r-w/2+b.bw()/2, litAngle-1.1, litAngle+1.1, b.bw(), 1, matSteel.lo)
	b.arc(0, 0, r+w/2-b.bw()/2, shadeAngle-1.1, shadeAngle+1.1, b.bw(), 1, matSteel.lo)
	if !b.tiny() {
		for i := 0; i < 6; i++ {
			a := float32(i)*tau/6 + tau/12
			b.line((r-w/2)*cosf(a), (r-w/2)*sinf(a), (r+w/2)*cosf(a), (r+w/2)*sinf(a), 0.03, 1, alpha(inkLine, 0.6))
		}
	}
}

func dockRingMotion(b *bpen) {
	for i := 0; i < 3; i++ {
		a := b.t*0.7 + float32(i)*tau/3
		if !b.lit {
			a = float32(i)*tau/3 + 0.5
		}
		x, y := 0.64*cosf(a), 0.64*sinf(a)
		b.circle(x, y, 0.045, b.lamp(gfx.RGB(0xFFF2CC)))
	}
	if b.lit {
		a := b.t * 0.7
		b.glow(0.64*cosf(a), 0.64*sinf(a), 0.22, alpha(colLamp, 0.6))
	}
}

// ---------------------------------------------------------------------------
// Exchange Dock: a diamond landing pad with painted marks, a control tower
// and a stack of trade containers.
// ---------------------------------------------------------------------------

var exchangeLights = [4][2]float32{{-0.6, -0.04}, {-0.1, -0.54}, {0.4, -0.04}, {-0.1, 0.46}}

func exchangeDockBody(b *bpen) {
	b.slab(matPad, 0.05, -0.74, -0.04, -0.1, -0.68, 0.54, -0.04, -0.1, 0.6)
	b.outline(alpha(matCream.hi, 0.85), 0.035, 1, -0.5, -0.04, -0.1, -0.44, 0.3, -0.04, -0.1, 0.36)
	b.ring(-0.1, -0.04, 0.15, 0.05, 1.2, colHazard)
	b.circle(-0.1, -0.04, 0.045, colHazard)
	// Containers, in the colours of what the Exchange trades.
	crates := [...]struct {
		x, y float32
		c    uint32
	}{{0.3, 0.2, 0xC0714F}, {0.52, 0.2, 0x9FD8EE}, {0.3, 0.35, 0xD9C89A}, {0.52, 0.35, 0x7D95C9}}
	for _, cr := range crates {
		b.block(matOf(gfx.RGB(cr.c)), 0.06, cr.x, cr.y, 0.2, 0.13)
	}
	b.drum(matSteel, 0.22, 0.47, -0.45, 0.14)
	b.circle(0.47, -0.45, 0.075, b.acc.mid)
}

func exchangeDockMotion(b *bpen) {
	on := int(wrap(b.t*2.5, 4)) % 4
	for i, l := range exchangeLights {
		if b.lit && i == on {
			b.glow(l[0], l[1], 0.2, alpha(b.acc.hi, 0.8))
			b.circle(l[0], l[1], 0.05, gfx.Lighten(b.acc.hi, 0.4))
			continue
		}
		c := b.acc.lo
		if !b.lit {
			c = colWindowOff
		}
		b.circle(l[0], l[1], 0.04, c)
	}
}

// ---------------------------------------------------------------------------
// Farseer Array: one great dish turning on a mount, two small fixed dishes
// and a relay hut.
// ---------------------------------------------------------------------------

const farseerY = -0.12

// ellipseXY returns an ellipse with semi-axes a (across) and c (along the
// direction ang) as x, y pairs.
func ellipseXY(dst []float32, cx, cy, a, c, ang float32, n int) []float32 {
	sn, cs := sinf(ang), cosf(ang)
	for i := 0; i < n; i++ {
		t := tau * float32(i) / float32(n)
		u, v := c*cosf(t), a*sinf(t)
		dst = append(dst, cx+u*cs-v*sn, cy+u*sn+v*cs)
	}
	return dst
}

// smallDish draws a fixed bowl: shaded on the side nearest the light, as a
// hollow is.
func (b *bpen) smallDish(x, y, r float32) {
	b.drum(matIron, 0.04, x-0.02, y+0.03, r*0.5)
	b.shadowCircle(x, y, r, 0.1)
	b.circle(x, y, r, matSteel.lo)
	b.circle(x-lightX*r*0.16, y-lightY*r*0.16, r*0.74, matSteel.mid)
	b.circle(x-lightX*r*0.3, y-lightY*r*0.3, r*0.4, matSteel.hi)
	b.ring(x, y, r, b.ow(), 1, inkLine)
	b.circle(x, y, max(r*0.16, 1.1*b.px), b.acc.mid)
}

func farseerBody(b *bpen) {
	b.block(matSteel, 0.08, -0.18, 0.5, 0.36, 0.22)
	b.rect(-0.13, 0.63, 0.26, 0.05, b.acc.mid)
	b.smallDish(0.53, 0.38, 0.2)
	b.smallDish(-0.56, 0.3, 0.15)
	b.drum(matIron, 0.06, 0, farseerY, 0.34)
}

func farseerMotion(b *bpen) {
	ang := b.t * 0.32
	if !b.lit {
		ang = -0.9
	}
	const a, c = 0.6, 0.47
	var buf [48]float32
	sh := ellipseXY(buf[:0], 0.2*shadowDX, farseerY+0.2*shadowDY, a, c, ang, 24)
	b.poly(inkShadow, sh...)
	b.poly(matSteel.lo, ellipseXY(buf[:0], 0, farseerY, a, c, ang, 24)...)
	// The hollow of the bowl catches the light on its far side.
	b.poly(matSteel.mid, ellipseXY(buf[:0], -lightX*0.07, farseerY-lightY*0.07, a*0.82, c*0.82, ang, 24)...)
	b.poly(matSteel.hi, ellipseXY(buf[:0], -lightX*0.15, farseerY-lightY*0.15, a*0.5, c*0.5, ang, 20)...)
	b.outline(inkLine, b.ow(), 1, ellipseXY(buf[:0], 0, farseerY, a, c, ang, 24)...)
	// Feed horn on its struts, leaning the way the dish looks.
	fx, fy := 0.2*cosf(ang), farseerY+0.2*sinf(ang)
	sn, cs := sinf(ang), cosf(ang)
	for _, t := range [3]float32{2.2, -2.2, 0} {
		u, v := c*0.9*cosf(t), a*0.9*sinf(t)
		b.line(u*cs-v*sn, farseerY+u*sn+v*cs, fx, fy, 0.03, 1, matIron.lo)
	}
	b.circle(fx, fy, 0.085, inkLine)
	b.circle(fx, fy, 0.06, b.acc.mid)
	if b.lit {
		b.circle(fx, fy, 0.028, gfx.RGB(0xFFF2CC))
	}
}

// ---------------------------------------------------------------------------
// Mender Hub: an arch-shaped depot around a landing court marked with a
// repair cross, its drones circling out.
// ---------------------------------------------------------------------------

func menderHubBody(b *bpen) {
	// The court is open ground: a painted landing circle and the repair
	// cross on a low plate.
	b.ring(0, 0.24, 0.24, 0.04, 1, alpha(matCream.hi, 0.85))
	b.rect(-0.075, 0.07, 0.15, 0.34, inkLine)
	b.rect(-0.17, 0.165, 0.34, 0.15, inkLine)
	b.rect(-0.045, 0.1, 0.09, 0.28, colRepair)
	b.rect(-0.14, 0.195, 0.28, 0.09, colRepair)
	// The depot: two arms and the bar that joins them.
	for _, x := range [2]float32{-0.66, 0.34} {
		b.block(matSteel, 0.12, x, -0.2, 0.32, 0.76)
		b.rect(x+0.07, 0.04, 0.18, 0.13, inkLine)
		b.rect(x+0.07, 0.3, 0.18, 0.13, inkLine)
		if !b.tiny() {
			b.rect(x+0.09, 0.06, 0.14, 0.09, matDark.mid)
			b.rect(x+0.09, 0.32, 0.14, 0.09, matDark.mid)
		}
	}
	b.block(matSteel, 0.12, -0.66, -0.6, 1.32, 0.44)
	b.rect(-0.6, -0.48, 1.2, 0.09, b.acc.mid)
	if !b.tiny() {
		b.rect(-0.5, -0.32, 0.3, 0.07, matSteel.lo)
		b.rect(0.2, -0.32, 0.3, 0.07, matSteel.lo)
	}
}

func menderHubMotion(b *bpen) {
	for i := 0; i < 3; i++ {
		a := b.t*0.9 + float32(i)*tau/3
		x, y := 0.74*cosf(a), 0.1+0.66*sinf(a)
		if !b.lit {
			// Parked on the court.
			x, y = -0.15+0.15*float32(i), 0.52
		}
		const s = 0.065
		b.poly(inkLine, x-s-0.03, y, x, y-s-0.03, x+s+0.03, y, x, y+s+0.03)
		b.poly(matCream.hi, x-s, y, x, y-s, x+s, y, x, y+s)
		b.circle(x, y, max(0.022, 0.8*b.px), b.lamp(colRepair))
	}
}

// ---------------------------------------------------------------------------
// Flak Battery: an armoured octagon carrying a twin-barrelled turret that
// sweeps the sky.
// ---------------------------------------------------------------------------

func flakBody(b *bpen) {
	var buf [16]float32
	base := ngonXY(buf[:0], 0, 0, 0.7, 8, pi/8)
	b.slab(matArmor, 0.1, base...)
	if !b.tiny() {
		b.block(matDark, 0.03, -0.52, 0.1, 0.17, 0.26)
	}
	b.ring(0, 0, 0.47, 0.07, 1.2, b.acc.mid)
	b.well(0, 0, 0.385, matDark.mid)
}

func flakMotion(b *bpen) {
	ang := -0.75 + 0.7*sinf(b.t*0.45)
	if !b.lit {
		ang = -0.2
	}
	f := b.frame(ang)
	b.shadowCircle(0, 0, 0.27, 0.1)
	for _, y := range [2]float32{-0.12, 0.12} {
		f.line(0.1, y, 0.78, y, 0.17, 2.8, inkLine)
		f.line(0.1, y, 0.75, y, 0.1, 1.2, matIron.mid)
		f.line(0.1, y-0.025, 0.62, y-0.025, 0.04, 0.8, matSteel.hi)
		f.line(0.64, y, 0.75, y, 0.13, 1.6, matDark.mid)
	}
	house := [...]float32{-0.25, -0.25, 0.22, -0.25, 0.28, -0.155, 0.28, 0.155, 0.22, 0.25, -0.25, 0.25}
	f.poly(matSteel.mid, house[:]...)
	f.edges(matSteel, house[:]...)
	f.outline(inkLine, max(b.ow()*1.3, 0.045), 1.4, house[:]...)
	f.circle(-0.03, 0, 0.11, inkLine)
	f.circle(-0.03, 0, 0.08, b.acc.mid)
}

// ---------------------------------------------------------------------------
// Lance Tower: a three-cornered footing with a capacitor at each corner
// and a long beam emitter on top, slowly hunting.
// ---------------------------------------------------------------------------

func lanceBody(b *bpen) {
	var base [12]float32
	for i := 0; i < 3; i++ {
		a := -pi/2 + float32(i)*tau/3
		for k, o := range [2]float32{-0.21, 0.21} {
			base[4*i+2*k], base[4*i+2*k+1] = 0.72*cosf(a+o), 0.72*sinf(a+o)
		}
	}
	b.slab(matArmor, 0.1, base[:]...)
	for i := 0; i < 3; i++ {
		a := -pi/2 + float32(i)*tau/3
		x, y := 0.5*cosf(a), 0.5*sinf(a)
		b.drum(matDark, 0.06, x, y, 0.135)
		b.lampDot(x, y, 0.06, colLumen)
	}
	b.ring(0, 0, 0.3, 0.06, 1.2, b.acc.mid)
	b.drum(matIron, 0.36, 0, 0, 0.24)
}

func lanceMotion(b *bpen) {
	ang := -0.85 + 0.75*sinf(b.t*0.3)
	if !b.lit {
		ang = -0.35
	}
	f := b.frame(ang)
	lance := [...]float32{-0.14, -0.1, 0.56, -0.055, 0.86, 0, 0.56, 0.055, -0.14, 0.1}
	f.shadowPoly(0.12, lance[:]...)
	f.poly(matSteel.mid, lance[:]...)
	f.poly(matSteel.hi, -0.14, -0.1, 0.56, -0.055, 0.86, 0, -0.14, 0)
	f.outline(inkLine, b.ow(), 1, lance[:]...)
	for _, x := range [2]float32{0.2, 0.4} {
		f.line(x, -0.12, x, 0.12, 0.06, 1.2, inkLine)
		f.line(x, -0.09, x, 0.09, 0.03, 0.8, b.acc.mid)
	}
	f.drum(matSteel, 0, 0, 0, 0.135)
	f.circle(0.62, 0, 0.05, b.lamp(colLumenHi))
	if b.lit {
		v := pulse(b.t, 1.6, 0.4, 1)
		f.glow(0.66, 0, 0.3, alpha(colLumen, 0.75*v))
	}
}

// ---------------------------------------------------------------------------
// Aegis Projector: three emitter petals around a shield dome that throws
// out ring after ring.
// ---------------------------------------------------------------------------

func aegisBody(b *bpen) {
	b.drum(matIron, 0.06, 0, 0, 0.46)
	const r, w, half = 0.64, 0.17, 0.72
	for i := 0; i < 3; i++ {
		a := -pi/2 + float32(i)*tau/3
		b.strut(0.4*cosf(a), 0.4*sinf(a), 0.6*cosf(a), 0.6*sinf(a), 0.09, matIron.mid)
		b.arc(0.1*shadowDX, 0.1*shadowDY, r, a-half, a+half, w, 2, inkShadow)
	}
	for i := 0; i < 3; i++ {
		a := -pi/2 + float32(i)*tau/3
		o := b.ow()
		b.arc(0, 0, r, a-half-o/r, a+half+o/r, w+2*o, 3, inkLine)
		b.arc(0, 0, r, a-half, a+half, w, 2, matSteel.mid)
		// Lit and shaded rims, by which way the petal faces.
		d := cosf(a)*lightX + sinf(a)*lightY
		outer, inner := matSteel.hi, matSteel.lo
		if d < 0 {
			outer, inner = matSteel.lo, matSteel.hi
		}
		if d > 0.3 || d < -0.3 {
			b.arc(0, 0, r+w/2-b.bw()/2, a-half, a+half, b.bw(), 1, outer)
			b.arc(0, 0, r-w/2+b.bw()/2, a-half, a+half, b.bw(), 1, inner)
		}
		b.arc(0, 0, r, a-half+0.12, a+half-0.12, 0.05, 1, b.acc.mid)
	}
	m := mat3{gfx.RGB(0xDDF6FF), colShield, gfx.RGB(0x2F7FA6)}
	if !b.lit {
		m = mat(0x61798A, 0x3C5161, 0x22313D)
	}
	b.dome(m, 0.1, 0, 0, 0.3)
}

func aegisMotion(b *bpen) {
	if !b.lit {
		return
	}
	const period = 2.6
	v := pulse(b.t, period, 0, 1)
	b.glow(0, 0, 0.75, alpha(colShield, 0.22+0.25*v))
	f := frac(b.t / period)
	b.ring(0, 0, 0.3+0.6*f, 0.04, 1.2, alpha(gfx.RGB(0xC8F1FF), 0.7*(1-f)*(1-f)))
	b.circle(lightX*0.1, lightY*0.1, 0.06+0.02*v, gfx.RGB(0xF4FDFF))
}

// ---------------------------------------------------------------------------
// Mass Driver: a rail gun laid corner to corner across the tile, coil
// after coil lighting up along the barrel as it charges.
// ---------------------------------------------------------------------------

const massDriverAngle = -pi / 6

var massDriverCoils = [5]float32{0.05, 0.24, 0.43, 0.62, 0.81}

func massDriverBody(b *bpen) {
	f := b.frame(massDriverAngle)
	f.drum(matIron, 0.05, -0.36, 0, 0.4)
	f.ring(-0.36, 0, 0.31, 0.05, 1, b.acc.mid)
	f.slab(matIron, 0.03, 0.34, -0.22, 0.54, -0.22, 0.54, 0.22, 0.34, 0.22)
	f.slab(matDark, 0.14, -0.14, -0.14, 0.92, -0.105, 0.92, 0.105, -0.14, 0.14)
	f.line(-0.1, -0.075, 0.9, -0.06, 0.045, 1, matSteel.hi)
	f.line(-0.1, 0.075, 0.9, 0.06, 0.045, 1, matSteel.mid)
	for _, x := range massDriverCoils {
		f.line(x, -0.185, x, 0.185, 0.1, 2.4, inkLine)
		f.line(x, -0.155, x, 0.155, 0.055, 1, matCopper.mid)
	}
	f.slab(matArmor, 0.12, -0.66, -0.27, -0.14, -0.27, -0.14, 0.27, -0.66, 0.27)
	f.poly(b.acc.mid, -0.6, -0.21, -0.5, -0.21, -0.5, 0.21, -0.6, 0.21)
	if !b.tiny() {
		// Slugs racked beside the breech.
		for i := 0; i < 3; i++ {
			x := -0.56 + float32(i)*0.13
			f.poly(inkLine, x-0.015, 0.32, x+0.105, 0.32, x+0.105, 0.44, x-0.015, 0.44)
			f.poly(gfx.RGB(0xAEB8C6), x, 0.335, x+0.09, 0.335, x+0.09, 0.425, x, 0.425)
		}
	}
}

func massDriverMotion(b *bpen) {
	f := b.frame(massDriverAngle)
	if !b.lit {
		f.circle(-0.3, 0, 0.05, colWindowOff)
		return
	}
	// Charge runs up the barrel, then the breech lamp flashes.
	const period = 3.2
	u := frac(b.t/period) * 7
	i := int(u)
	if i >= 0 && i < len(massDriverCoils) {
		x := massDriverCoils[i]
		f.line(x, -0.155, x, 0.155, 0.055, 1, gfx.RGB(0xFFE9A8))
		f.glow(x, 0, 0.3, alpha(colHazard, 0.7))
	}
	c := gfx.RGB(0x7A5A2A)
	if i >= len(massDriverCoils) {
		c = gfx.RGB(0xFFF2CC)
		f.glow(-0.3, 0, 0.3, alpha(colLamp, 0.7))
	}
	f.circle(-0.3, 0, 0.05, c)
}
