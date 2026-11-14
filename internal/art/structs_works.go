package art

import (
	"math"

	"drifthold/internal/gfx"
)

// ---------------------------------------------------------------------------
// Bore Rig: a four-legged derrick over a bore hole, its drive housing at
// the foot, a spoil chute off one side, the drill string turning inside.
// ---------------------------------------------------------------------------

const boreTop = 1.0

func boreRigVol(b *bpen) {
	b.shadowG(0, 0, 0.72, 0.7)
	// Spoil heap at the end of the chute.
	b.frustum(matOf(gfx.RGB(0x8E5B45)), 0.6, 0.3, 0.14, 0, 0, 0.13)
	b.disc(colPit, 0, 0, 0.004, 0.17)
	b.hoop(inkLine, b.ow(), 1.2, 0, 0, 0.004, 0.17)
	b.hoop(matIron.mid, 0.03, 1, 0, 0, 0.004, 0.21)
	feet := ibox{0, 0, 0.3, 0.3}
	head := ibox{0, 0, 0.07, 0.07}
	leg := func(a, c float32) {
		x0, y0 := feet.pt(a, c)
		x1, y1 := head.pt(a, c)
		b.beam(matSteel.mid, 0.055, x0, y0, 0, x1, y1, boreTop)
	}
	brace := func(a0, c0, a1, c1, t0, t1 float32) {
		// A strut between two legs: from height fraction t0 on the first
		// to t1 on the second.
		ax0, ay0 := feet.pt(a0, c0)
		ax1, ay1 := head.pt(a0, c0)
		bx0, by0 := feet.pt(a1, c1)
		bx1, by1 := head.pt(a1, c1)
		b.beam(matIron.hi, 0.028,
			lerpf(ax0, ax1, t0), lerpf(ay0, ay1, t0), boreTop*t0,
			lerpf(bx0, bx1, t1), lerpf(by0, by1, t1), boreTop*t1)
	}
	leg(-1, -1)
	// The drive housing behind the string.
	drive := ibox{-0.3, -0.16, 0.12, 0.15}
	b.box(matSteel, drive, 0, 0.26)
	b.topQ(b.acc.mid, drive, -0.7, 0.7, -0.7, 0.7, 0.262)
	// The drill string.
	b.beam(matDark.hi, 0.05, 0, 0, 0, 0, 0, boreTop)
	leg(-1, 1)
	leg(1, -1)
	if !b.tiny() {
		brace(-1, 1, 1, 1, 0.12, 0.42)
		brace(1, 1, -1, 1, 0.12, 0.42)
		brace(1, 1, 1, -1, 0.12, 0.42)
		brace(1, -1, 1, 1, 0.12, 0.42)
		brace(-1, 1, 1, 1, 0.42, 0.42)
		brace(1, 1, 1, -1, 0.42, 0.42)
		brace(-1, 1, 1, 1, 0.72, 0.72)
		brace(1, 1, 1, -1, 0.72, 0.72)
	}
	leg(1, 1)
	// The chute.
	b.tube(matIron, 0.12, 0.08, 0.26, 0.52, 0.26, 0.12, 0.05)
	// The crown block.
	crown := ibox{0, 0, 0.1, 0.1}
	b.box(matSteel, crown, boreTop, boreTop+0.08)
	b.topQ(b.acc.mid, crown, -0.6, 0.6, -0.6, 0.6, boreTop+0.082)
	b.hit(-0.3, 0.0, 0.14)
	b.hit(0.1, 0.3, 0.3)
	b.hit(0, 0, 0.7)
}

func boreRigLive(b *bpen) {
	// The string turns: bright flights wind down it. The motor on the
	// crown block throbs.
	if b.lit {
		for i := 0; i < 4; i++ {
			f := frac(b.t*0.9 + float32(i)/4)
			h := boreTop * (1 - f) * 0.96
			b.rod(matSteel.hi, 0.07, 1.4, -0.006, 0, h, 0.006, 0, h+0.035)
		}
	}
	r := float32(0.05)
	if b.lit {
		r += 0.012 * sinf(b.t*5.2)
	}
	b.knobAt(0, 0, boreTop+0.11, r, b.acc.hi)
}

// ---------------------------------------------------------------------------
// Smelter: a heavy furnace hall with a chimney and an ore hopper on its
// roof, glowing vents in its wall and an open crucible of molten metal at
// its foot.
// ---------------------------------------------------------------------------

const (
	smelterPotGX = 0.38
	smelterPotGY = 0.36
	smelterPotH  = 0.22
)

func smelterVol(b *bpen) {
	b.shadowG(0, 0.02, 0.84, 0.78)
	q := ibox{-0.14, -0.08, 0.3, 0.26}
	b.box(matPlate, q, 0, 0.46)
	// Vents in the left wall, glowing with the heat inside.
	for _, h := range [2]float32{0.12, 0.26} {
		b.wallL(inkLine, q, -0.72, 0.62, h-0.012, h+0.062)
		b.wallL(b.lamp(colHot), q, -0.68, 0.58, h, h+0.05)
	}
	b.wallR(b.acc.mid, q, 1, -1, 0.36, 0.42)
	if b.lit && !b.tiny() {
		b.glowAt(-0.4, 0.2, 0.2, 0.4, alpha(colHot, 0.28))
	}
	// On the roof: the ore hopper and the chimney.
	hx, hy := q.pt(0.35, 0.3)
	hop := ibox{hx, hy, 0.1, 0.1}
	b.box(matIron, hop, 0.46, 0.6)
	b.topQ(gfx.RGB(0xC0714F), hop, -0.75, 0.75, -0.75, 0.75, 0.602)
	cx, cy := q.pt(-0.45, -0.3)
	b.frustum(matIron, cx, cy, 0.1, 0.075, 0.46, 1.06)
	b.band(b.acc.mid, cx, cy, 0.082, 0.9, 0.96)
	b.disc(colPit, cx, cy, 1.06, 0.045)
	// The crucible and the spout that feeds it.
	b.tube(matIron, 0.16, 0.2, 0.3, smelterPotGX-0.06, smelterPotGY-0.08, smelterPotH+0.04, 0.045)
	b.frustum(matDark, smelterPotGX, smelterPotGY, 0.2, 0.24, 0, smelterPotH)
	slag := gfx.RGB(0x2A2C35)
	if b.lit {
		slag = colHot
	}
	b.disc(slag, smelterPotGX, smelterPotGY, smelterPotH+0.002, 0.18)
	b.hoop(inkLine, b.ow(), 1, smelterPotGX, smelterPotGY, smelterPotH+0.002, 0.18)
	b.hit(-0.42, 0.24, 0.3)
	b.hit(0.1, 0.24, 0.22)
	b.hit(-0.14, -0.08, 0.47)
}

func smelterLive(b *bpen) {
	if !b.lit {
		b.disc(gfx.RGB(0x3A3C46), smelterPotGX, smelterPotGY, smelterPotH+0.003, 0.09)
		return
	}
	v := pulse(b.t, 1.9, 0, 1)
	b.glowAt(smelterPotGX, smelterPotGY, smelterPotH, 0.62, alpha(colHot, 0.4+0.25*v))
	b.disc(colHotCore, smelterPotGX+0.02*sinf(b.t*0.9), smelterPotGY+0.02*cosf(b.t*1.1), smelterPotH+0.003, 0.09+0.035*v)
	// Sparks jumping from the pot.
	for i := 0; i < 2; i++ {
		f := frac(b.t*0.9 + float32(i)*0.53)
		a := 3.9 + float32(i)*1.7 + wrap(float32(math.Floor(float64(b.t*0.9+float32(i)*0.53)))*2.4, tau)
		d := 0.1 + 0.25*f
		b.dotAt(smelterPotGX+d*cosf(a), smelterPotGY+d*sinf(a), smelterPotH+0.4*f*(1-f)*2, max(0.022, 1.1*b.px), alpha(colHotCore, 1-f))
	}
	// Chimney smoke.
	cx, cy := ibox{-0.14, -0.08, 0.3, 0.26}.pt(-0.45, -0.3)
	for k := 0; k < 2; k++ {
		d := frac(b.t*0.3 + float32(k)/2)
		b.puffAt(cx+0.18*d, cy, 1.1+0.42*d, 0.05+0.09*d, gfx.Color{R: 0xC8, G: 0xB8, B: 0xB0, A: uint8(64 * (1 - d))})
	}
}

// ---------------------------------------------------------------------------
// Cracker: a tall distillation column and a cascade of two fuel tanks on
// a skid, a flare burning at the end of a boom.
// ---------------------------------------------------------------------------

const (
	crackerColX, crackerColY = -0.26, -0.2
	crackerFlareGX           = 0.02
	crackerFlareGY           = -0.42
	crackerFlareH            = 1.34
)

func crackerVol(b *bpen) {
	b.shadowG(0.02, 0.06, 0.82, 0.76)
	skid := ibox{0.06, 0.1, 0.4, 0.2}
	b.box(matPad, skid, 0, 0.06)
	// The ice intake.
	ix, iy := skid.pt(-0.75, 0.55)
	b.box(matIce, ibox{ix, iy, 0.08, 0.08}, 0.06, 0.24)
	// The column.
	b.cyl(matSteel, crackerColX, crackerColY, 0.15, 0.06, 1.12)
	b.band(matIce.mid, crackerColX, crackerColY, 0.15, 0.2, 0.28)
	for _, h := range [2]float32{0.52, 0.84} {
		b.band(matIron.lo, crackerColX, crackerColY, 0.185, h-0.012, h+0.04)
		b.band(matIron.hi, crackerColX, crackerColY, 0.185, h+0.022, h+0.04)
	}
	b.domeUp(matSteel, crackerColX, crackerColY, 0.15, 1.12, 0.09)
	// The flare boom.
	b.beam(matIron.hi, 0.035, crackerColX, crackerColY, 1.16, crackerFlareGX, crackerFlareGY, crackerFlareH)
	b.dotAt(crackerFlareGX, crackerFlareGY, crackerFlareH, 0.05, matDark.mid)
	// The tanks and the pipe run.
	b.tube(matCopper, crackerColX+0.1, crackerColY+0.1, 0.5, 0.1, 0.14, 0.5, 0.04)
	b.cyl(matCream, 0.1, 0.14, 0.2, 0.06, 0.42)
	b.band(matFuel.mid, 0.1, 0.14, 0.2, 0.2, 0.28)
	b.domeUp(matCream, 0.1, 0.14, 0.2, 0.42, 0.09)
	b.tube(matCopper, 0.2, 0.24, 0.32, 0.42, 0.42, 0.3, 0.035)
	b.cyl(matCream, 0.42, 0.42, 0.14, 0.06, 0.3)
	b.band(matFuel.mid, 0.42, 0.42, 0.14, 0.14, 0.2)
	b.domeUp(matCream, 0.42, 0.42, 0.14, 0.3, 0.07)
	b.hit(0.1, 0.34, 0.2)
	b.hit(crackerColX, crackerColY+0.15, 0.6)
	b.hit(0.42, 0.56, 0.16)
}

func crackerLive(b *bpen) {
	if !b.lit {
		return
	}
	f := 0.6 + 0.25*sinf(b.t*17) + 0.15*sinf(b.t*7.1+1)
	b.glowAt(crackerFlareGX, crackerFlareGY, crackerFlareH+0.04, 0.34, alpha(colHot, 0.6*f))
	r := 0.045 + 0.02*f
	p := b.at(crackerFlareGX, crackerFlareGY, crackerFlareH)
	// A flame: a drop standing on the tip of the boom.
	b.cv.FillPoly([]gfx.Pt{
		{X: p.X - r*b.k, Y: p.Y - 0.02*b.k}, {X: p.X, Y: p.Y - (0.12+0.07*f)*b.k}, {X: p.X + r*b.k, Y: p.Y - 0.02*b.k}, {X: p.X, Y: p.Y + 0.03*b.k},
	}, colHot)
	b.puffAt(crackerFlareGX, crackerFlareGY, crackerFlareH+0.03, r*0.6, colHotCore)
}

// ---------------------------------------------------------------------------
// Fabricator: a works under a saw-tooth roof, its glazed lights lit from
// inside, with a delivery pad where an arm sets down finished parts.
// ---------------------------------------------------------------------------

var (
	fabHall = ibox{-0.12, -0.08, 0.36, 0.28}
	fabPad  = ibox{0.4, 0.4, 0.15, 0.13}
)

func fabricatorVol(b *bpen) {
	b.shadowG(0, 0.02, 0.84, 0.78)
	q := fabHall
	const wall, peak = 0.3, 0.5
	b.box(matSteel, q, 0, wall)
	b.wallL(b.acc.mid, q, -1, 1, 0.04, 0.1)
	// A loading door in the right wall.
	b.wallR(inkLine, q, 0.5, -0.3, 0, 0.21)
	b.wallR(matDark.mid, q, 0.45, -0.25, 0, 0.19)
	// The saw-tooth roof: three bays, each a slope rising to a glazed
	// face that looks to the right.
	glass := gfx.RGB(0xECDDAE)
	if !b.lit {
		glass = gfx.RGB(0x2A3346)
	}
	const bays = 3
	for i := 0; i < bays; i++ {
		a0 := -1 + 2*float32(i)/bays
		a1 := a0 + 2.0/bays
		lx0, ly0 := q.pt(a0, 1)
		lx1, ly1 := q.pt(a1, 1)
		rx0, ry0 := q.pt(a0, -1)
		rx1, ry1 := q.pt(a1, -1)
		b.face(matSteel.hi, lx0, ly0, wall, lx1, ly1, peak, rx1, ry1, peak, rx0, ry0, wall)
		b.face(matSteel.mid, lx0, ly0, wall, lx1, ly1, wall, lx1, ly1, peak)
		b.face(glass, lx1, ly1, wall, rx1, ry1, wall, rx1, ry1, peak, lx1, ly1, peak)
		if !b.tiny() {
			for _, c := range [2]float32{0.34, -0.34} {
				mx, my := q.pt(a1, c)
				b.edge(matSteel.lo, 0.02, 0.7, mx, my, wall, mx, my, peak)
			}
		}
		b.edge(gfx.Lighten(matSteel.hi, 0.45), b.hairline(), 0, lx0, ly0, wall, lx1, ly1, peak, rx1, ry1, peak)
		b.loop(inkLine, b.ow(), 1, lx0, ly0, wall, lx1, ly1, wall, rx1, ry1, wall, rx1, ry1, peak, rx0, ry0, wall, lx0, ly0, wall)
	}
	if b.lit && !b.tiny() {
		b.glowAt(0.1, 0.1, 0.4, 0.5, alpha(colWindow, 0.16))
	}
	// The delivery pad and the parts waiting on it.
	b.box(matPad, fabPad, 0, 0.05)
	if !b.tiny() {
		pc := matOf(gfx.RGB(0x7D95C9))
		for _, c := range [2][2]float32{{0.25, -0.4}, {0.3, 0.45}} {
			x, y := fabPad.pt(c[0], c[1])
			b.box(pc, ibox{x, y, 0.045, 0.045}, 0.05, 0.13)
		}
	}
	b.hit(-0.42, 0.22, 0.18)
	b.hit(0.08, 0.3, 0.14)
	b.hit(-0.12, -0.08, 0.42)
}

func fabricatorLive(b *bpen) {
	// The arm stands at the back of the pad and swings between the works
	// and the pad.
	u := smooth(0.5 + 0.5*sinf(b.t*1.3))
	if !b.lit {
		u = 0.25
	}
	sx, sy := fabPad.pt(-0.8, -0.2)
	hx, hy := fabPad.pt(lerpf(-0.3, 0.5, u), lerpf(-0.6, 0.5, u))
	hh := float32(0.16) + 0.1*sinf(u*pi)
	ex, ey, eh := (sx+hx)/2, (sy+hy)/2, float32(0.42)
	b.beam(matSteel.hi, 0.045, sx, sy, 0.05, ex, ey, eh)
	b.beam(matSteel.hi, 0.035, ex, ey, eh, hx, hy, hh)
	b.knobAt(sx, sy, 0.06, 0.035, b.acc.mid)
	b.dotAt(ex, ey, eh, 0.03, b.acc.mid)
	b.dotAt(hx, hy, hh, 0.035, b.lamp(gfx.RGB(0xBFE9FF)))
	if b.lit && u > 0.9 {
		b.glowAt(hx, hy, hh, 0.2, alpha(gfx.RGB(0xBFE9FF), 0.7))
	}
}

// ---------------------------------------------------------------------------
// Laboratory: a domed hall with a glowing oculus at the end of a long
// bench wing, an aerial on the roof and a sample tank at the door.
// ---------------------------------------------------------------------------

const (
	labDomeGX, labDomeGY = -0.26, -0.22
	labDrumH             = 0.36
	labEyeH              = 0.62
	labGlowCol           = 0x8FE3FF
)

func laboratoryVol(b *bpen) {
	b.shadowG(0.02, 0.04, 0.84, 0.78)
	b.cyl(matCream, labDomeGX, labDomeGY, 0.3, 0, labDrumH)
	b.band(b.acc.mid, labDomeGX, labDomeGY, 0.3, labDrumH-0.08, labDrumH-0.03)
	wing := ibox{0.12, 0.14, 0.32, 0.2}
	b.box(matSteel, wing, 0, 0.28)
	b.wallL(b.acc.mid, wing, -1, 1, 0.04, 0.09)
	b.winL(gfx.RGB(labGlowCol), wing, 3, -0.6, 0.85, 0.14, 0.22)
	b.winR(gfx.RGB(labGlowCol), wing, 2, 0.75, -0.6, 0.14, 0.22)
	if !b.tiny() {
		// Skylights.
		for _, a := range [2]float32{0.0, 0.5} {
			b.topQ(inkLine, wing, a-0.02, a+0.3, -0.52, 0.52, 0.281)
			b.topQ(b.lamp(gfx.RGB(labGlowCol)), wing, a+0.02, a+0.26, -0.44, 0.44, 0.282)
		}
	}
	b.domeUp(matCream, labDomeGX, labDomeGY, 0.3, labDrumH, 0.27)
	// The oculus.
	p := b.at(labDomeGX-0.03, labDomeGY, labEyeH)
	b.cv.FillCircle(p.X, p.Y, 0.11*b.k, b.col(inkLine))
	b.cv.FillCircle(p.X, p.Y, 0.085*b.k, b.col(b.lamp(gfx.RGB(labGlowCol))))
	// The aerial.
	ax, ay := wing.pt(0.7, -0.5)
	b.beam(matIron.hi, 0.025, ax, ay, 0.28, ax, ay, 0.74)
	b.beam(matIron.hi, 0.02, ax-0.08, ay, 0.62, ax+0.08, ay, 0.62)
	// The sample tank on its stand.
	b.beam(matIron.mid, 0.03, 0.5, 0.56, 0, 0.5, 0.56, 0.1)
	b.ball(matIce, 0.5, 0.56, 0.17, 0.095)
	b.hit(-0.2, 0.2, 0.14)
	b.hit(0.4, 0.36, 0.16)
	b.hit(-0.3, 0.06, 0.24)
}

func laboratoryLive(b *bpen) {
	if !b.lit {
		return
	}
	v := pulse(b.t, 3.1, 0, 1)
	b.glowAt(labDomeGX-0.03, labDomeGY, labEyeH, 0.46, alpha(gfx.RGB(labGlowCol), 0.3+0.3*v))
	b.dotAt(labDomeGX-0.03, labDomeGY, labEyeH, 0.04+0.02*v, gfx.RGB(0xF2FCFF))
	// Two motes circling the dome, like a diagram of an atom.
	for i := 0; i < 2; i++ {
		a := b.t*2.1*(1-0.35*float32(i)) + float32(i)*2
		r := 0.36 + 0.06*float32(i)
		b.puffAt(labDomeGX+r*cosf(a), labDomeGY+r*sinf(a), labEyeH-0.06+0.1*float32(i), max(0.028, 1.1*b.px), gfx.RGB(0xDFF7FF))
	}
}

// ---------------------------------------------------------------------------
// Launch Cradle: a catapult rail climbing between two gantry towers, a
// hull on the slip, lights running up the rail.
// ---------------------------------------------------------------------------

// The rail runs from its foot to its head; rail returns the point a
// fraction t along it and s to the side.
func cradleRail(t, s float32) (x, y, h float32) {
	x0, y0 := diag(0, 0, 0, 0.44)
	x1, y1 := diag(0, 0, 0, -0.44)
	sx, sy := diag(0, 0, s, 0)
	return lerpf(x0, x1, t) + sx, lerpf(y0, y1, t) + sy, lerpf(0.1, 0.8, t)
}

func launchCradleVol(b *bpen) {
	b.shadowG(0, 0, 0.82, 0.78)
	tx0, ty0 := diag(0, 0, -0.32, 0)
	tx1, ty1 := diag(0, 0, 0.32, 0)
	// The far tower.
	back := ibox{tx0, ty0, 0.08, 0.1}
	b.box(matSteel, back, 0, 0.84)
	b.wallL(b.acc.mid, back, -1, 1, 0.7, 0.76)
	// The blast pit at the foot of the rail.
	px, py := diag(0, 0, 0, 0.5)
	b.box(matDark, ibox{px, py, 0.14, 0.09}, 0, 0.08)
	// Props under the rail.
	for _, t := range [2]float32{0.45, 0.85} {
		x, y, h := cradleRail(t, 0)
		b.beam(matIron.mid, 0.04, x, y, 0, x, y, h-0.04)
	}
	// The rail: a sloping bed, its side, two bright rails and chevrons.
	const w = 0.085
	ax, ay, ah := cradleRail(0, -w)
	bx, by, bh := cradleRail(1, -w)
	cx, cy, ch := cradleRail(1, w)
	dx, dy, dh := cradleRail(0, w)
	b.face(matIron.lo, dx, dy, dh-0.07, cx, cy, ch-0.07, cx, cy, ch, dx, dy, dh)
	b.face(matIron.mid, ax, ay, ah, bx, by, bh, cx, cy, ch, dx, dy, dh)
	b.loop(inkLine, b.ow(), 1, ax, ay, ah, bx, by, bh, cx, cy, ch, cx, cy, ch-0.07, dx, dy, dh-0.07, dx, dy, dh)
	for _, s := range [2]float32{-0.05, 0.05} {
		x0, y0, h0 := cradleRail(0.02, s)
		x1, y1, h1 := cradleRail(0.98, s)
		b.edge(matSteel.hi, 0.022, 0.8, x0, y0, h0+0.004, x1, y1, h1+0.004)
	}
	for _, t := range [2]float32{0.8, 0.9} {
		x0, y0, h0 := cradleRail(t-0.06, -0.06)
		x1, y1, h1 := cradleRail(t, 0)
		x2, y2, h2 := cradleRail(t-0.06, 0.06)
		b.edge(b.acc.hi, 0.03, 1, x0, y0, h0+0.006, x1, y1, h1+0.006, x2, y2, h2+0.006)
	}
	// The hull on the slip.
	hull := func(c gfx.Color, lift float32, pts ...float32) {
		var xyh [30]float32
		n := 0
		for i := 0; i+1 < len(pts); i += 2 {
			x, y, h := cradleRail(pts[i], pts[i+1])
			xyh[n], xyh[n+1], xyh[n+2] = x, y, h+lift
			n += 3
		}
		b.face(c, xyh[:n]...)
	}
	hull(matCream.lo, 0.02, 0.24, -0.04, 0.3, -0.07, 0.56, -0.07, 0.74, 0, 0.56, 0.07, 0.3, 0.07, 0.24, 0.04)
	hull(matCream.mid, 0.085, 0.24, -0.04, 0.3, -0.07, 0.56, -0.07, 0.74, 0, 0.56, 0.07, 0.3, 0.07, 0.24, 0.04)
	hull(matCream.hi, 0.085, 0.24, -0.04, 0.3, -0.07, 0.56, -0.07, 0.74, 0, 0.24, 0)
	{
		x0, y0, h0 := cradleRail(0.24, -0.04)
		x1, y1, h1 := cradleRail(0.3, -0.07)
		x2, y2, h2 := cradleRail(0.56, -0.07)
		x3, y3, h3 := cradleRail(0.74, 0)
		x4, y4, h4 := cradleRail(0.56, 0.07)
		x5, y5, h5 := cradleRail(0.3, 0.07)
		x6, y6, h6 := cradleRail(0.24, 0.04)
		b.loop(inkLine, b.ow(), 1, x0, y0, h0+0.085, x1, y1, h1+0.085, x2, y2, h2+0.085, x3, y3, h3+0.085,
			x4, y4, h4+0.085, x5, y5, h5+0.085, x6, y6, h6+0.085)
	}
	// The gantry beam and the near tower.
	b.beam(matSteel.mid, 0.07, tx0, ty0, 0.8, tx1, ty1, 0.8)
	front := ibox{tx1, ty1, 0.08, 0.1}
	b.box(matSteel, front, 0, 0.84)
	b.wallL(b.acc.mid, front, -1, 1, 0.7, 0.76)
	if !b.tiny() {
		b.winL(colWindow, front, 1, -0.5, 0.5, 0.5, 0.6)
	}
	b.hit(0.24, 0.5, 0.3)
	b.hit(-0.1, 0.3, 0.2)
	b.hit(0.3, 0.3, 0.7)
}

func launchCradleLive(b *bpen) {
	for i := 0; i < 3; i++ {
		f := frac(b.t*0.55 + float32(i)/3)
		if !b.lit {
			f = (float32(i) + 0.5) / 3
		}
		x, y, h := cradleRail(0.05+0.9*f, 0.1)
		if !b.lit {
			b.dotAt(x, y, h, 0.03, colWindowOff)
			continue
		}
		b.dotAt(x, y, h, 0.035, alpha(colLamp, sinf(f*pi)))
	}
	x, y := diag(0, 0, 0.32, 0)
	if b.lit && frac(b.t*0.8) < 0.3 {
		b.glowAt(x, y, 0.9, 0.24, alpha(gfx.RGB(0xFF6A4A), 0.8))
		b.dotAt(x, y, 0.9, 0.04, gfx.RGB(0xFFC4B0))
	} else {
		b.dotAt(x, y, 0.9, 0.03, gfx.RGB(0x5A2A22))
	}
}

// ---------------------------------------------------------------------------
// Dock Ring: a mooring ring floating over its anchor on three tethers,
// clamps on its rim and marker lights going round.
// ---------------------------------------------------------------------------

const (
	dockRingR = 0.6
	dockRingH = 0.72
)

func dockRingVol(b *bpen) {
	// The ring hangs well above the ground: its shadow lies loose below.
	b.hoop(inkShadow, 0.1, 2, 0.12, 0.08, 0, dockRingR)
	// Three landing pads and the anchor block.
	for i := 0; i < 3; i++ {
		a := pi/2 + float32(i)*tau/3
		x, y := 0.5*cosf(a), 0.5*sinf(a)
		b.disc(matPad.mid, x, y, 0.003, 0.15)
		b.hoop(inkLine, b.ow(), 1, x, y, 0.003, 0.15)
		b.hoop(alpha(matCream.hi, 0.7), 0.02, 0.7, x, y, 0.004, 0.09)
	}
	b.shadowG(0, 0, 0.2, 0.2)
	ring := func(a0, a1 float32) {
		b.arcG(inkLine, 0.13+2*b.ow(), 3.2, 0, 0, dockRingH, dockRingR, a0, a1)
		b.arcG(matSteel.mid, 0.13, 2, 0, 0, dockRingH, dockRingR, a0, a1)
		b.arcG(matSteel.hi, 0.04, 0.8, 0, 0, dockRingH+0.04, dockRingR, a0, a1)
		b.arcG(matSteel.lo, 0.035, 0.8, 0, 0, dockRingH-0.045, dockRingR, a0, a1)
	}
	clamp := func(a float32) {
		x, y := dockRingR*cosf(a), dockRingR*sinf(a)
		b.box(b.acc, ibox{x, y, 0.055, 0.055}, dockRingH-0.075, dockRingH+0.085)
	}
	ring(pi, tau)
	clamp(3 * pi / 2)
	b.box(matIron, ibox{0, 0, 0.1, 0.1}, 0, 0.18)
	b.topQ(b.acc.mid, ibox{0, 0, 0.1, 0.1}, -0.6, 0.6, -0.6, 0.6, 0.182)
	for i := 0; i < 3; i++ {
		a := -pi/2 + float32(i)*tau/3 + tau/6
		b.edge(alpha(matCream.hi, 0.85), 0.022, 0.8, 0, 0, 0.18, dockRingR*cosf(a), dockRingR*sinf(a), dockRingH)
	}
	ring(0, pi)
	clamp(pi / 6)
	clamp(5 * pi / 6)
	b.hit(0, 0.6, dockRingH)
	b.hit(0, 0.1, 0.1)
	b.hit(-0.5, 0.33, dockRingH)
}

func dockRingLive(b *bpen) {
	for i := 0; i < 3; i++ {
		a := b.t*0.7 + float32(i)*tau/3
		if !b.lit {
			a = float32(i)*tau/3 + 0.5
		}
		x, y := dockRingR*cosf(a), dockRingR*sinf(a)
		b.dotAt(x, y, dockRingH+0.03, 0.04, b.lamp(gfx.RGB(0xFFF2CC)))
	}
	if b.lit {
		a := b.t * 0.7
		b.glowAt(dockRingR*cosf(a), dockRingR*sinf(a), dockRingH+0.03, 0.2, alpha(colLamp, 0.6))
	}
}

// ---------------------------------------------------------------------------
// Exchange Dock: a painted landing pad with a control tower at its back
// corner and a stack of trade containers at its side.
// ---------------------------------------------------------------------------

var exchangePad = ibox{-0.12, 0.06, 0.32, 0.32}

func exchangeDockVol(b *bpen) {
	b.shadowG(0, 0.03, 0.84, 0.78)
	q := exchangePad
	b.box(matPad, q, 0, 0.07)
	const top = 0.072
	{
		x0, y0 := q.pt(-0.72, 0.72)
		x1, y1 := q.pt(0.72, 0.72)
		x2, y2 := q.pt(0.72, -0.72)
		x3, y3 := q.pt(-0.72, -0.72)
		b.loop(alpha(matCream.hi, 0.85), 0.028, 1, x0, y0, top, x1, y1, top, x2, y2, top, x3, y3, top)
	}
	b.hoop(colHazard, 0.04, 1.2, q.x, q.y, top, 0.15)
	b.disc(colHazard, q.x, q.y, top, 0.045)
	// The control tower: a shaft, a glazed cab, an accent roof, an aerial.
	tw := ibox{0.44, -0.34, 0.075, 0.075}
	b.box(matSteel, tw, 0, 0.6)
	cab := ibox{tw.x, tw.y, 0.11, 0.11}
	b.box(mat(0x4D6A7C, 0x2C4657, 0x1B2E3B), cab, 0.6, 0.76)
	b.winL(gfx.RGB(0xBFE9FF), cab, 2, -0.8, 0.8, 0.64, 0.72)
	b.winR(gfx.RGB(0xBFE9FF), cab, 2, 0.8, -0.8, 0.64, 0.72)
	b.box(b.acc, ibox{tw.x, tw.y, 0.125, 0.125}, 0.76, 0.8)
	b.beam(matIron.hi, 0.022, tw.x, tw.y, 0.8, tw.x, tw.y, 0.95)
	// Containers, in the colours of what the Exchange trades.
	crate := func(x, y, h float32, c uint32, long bool) {
		q := ibox{x, y, 0.13, 0.075}
		if !long {
			q.da, q.db = 0.075, 0.13
		}
		b.box(matOf(gfx.RGB(c)), q, h, h+0.13)
	}
	crate(0.5, 0.2, 0, 0xC0714F, true)
	crate(0.5, 0.2, 0.13, 0x9FD8EE, true)
	crate(0.36, 0.5, 0, 0xD9C89A, false)
	crate(0.56, 0.42, 0, 0x7D95C9, true)
	b.hit(-0.3, 0.4, 0.06)
	b.hit(0.5, 0.36, 0.14)
	b.hit(0.44, -0.2, 0.4)
}

func exchangeDockLive(b *bpen) {
	q := exchangePad
	on := int(wrap(b.t*2.5, 4)) % 4
	for i, c := range [4][2]float32{{-0.86, 0.86}, {0.86, 0.86}, {0.86, -0.86}, {-0.86, -0.86}} {
		x, y := q.pt(c[0], c[1])
		if b.lit && i == on {
			b.glowAt(x, y, 0.08, 0.18, alpha(b.acc.hi, 0.8))
			b.dotAt(x, y, 0.08, 0.04, gfx.Lighten(b.acc.hi, 0.4))
			continue
		}
		col := b.acc.lo
		if !b.lit {
			col = colWindowOff
		}
		b.dotAt(x, y, 0.08, 0.032, col)
	}
	if b.lit {
		v := pulse(b.t, 2.2, 0.2, 1)
		b.dotAt(0.44, -0.34, 0.96, 0.03, alpha(gfx.RGB(0xFF8A6A), v))
	}
}

// ---------------------------------------------------------------------------
// Farseer Array: one great dish turning on a tall mount, a small fixed
// dish and a relay hut at its foot.
// ---------------------------------------------------------------------------

const (
	farseerH = 0.98 // height of the dish's centre
	farseerR = 0.52
)

func farseerVol(b *bpen) {
	b.shadowG(0, 0, 0.6, 0.56)
	hut := ibox{-0.44, 0.2, 0.13, 0.11}
	b.box(matSteel, hut, 0, 0.2)
	b.wallL(b.acc.mid, hut, -1, 1, 0.13, 0.17)
	if !b.tiny() {
		b.winL(colWindow, hut, 1, -0.4, 0.4, 0.04, 0.1)
	}
	// The small dish: a tilted bowl on a post.
	b.beam(matIron.mid, 0.04, 0.48, 0.34, 0, 0.48, 0.34, 0.2)
	p := b.at(0.48, 0.34, 0.26)
	bowl := func(rx, ry, dx, dy float32, c gfx.Color) {
		var buf [16]gfx.Pt
		pts := buf[:0]
		for i := 0; i < 16; i++ {
			a := tau * float32(i) / 16
			u, v := rx*cosf(a), ry*sinf(a)
			// Turned a little anticlockwise, to look up to the left.
			pts = append(pts, gfx.Pt{X: p.X + (dx+u*0.9+v*0.42)*b.k, Y: p.Y + (dy-u*0.42+v*0.9)*b.k})
		}
		b.cv.FillPoly(pts, b.col(c))
		if c == matSteel.lo {
			b.cv.StrokePoly(pts, b.w(b.ow(), 1), b.col(inkLine))
		}
	}
	bowl(0.17, 0.11, 0, 0, matSteel.lo)
	bowl(0.13, 0.08, 0.01, 0.01, matSteel.mid)
	bowl(0.07, 0.045, 0.02, 0.02, matSteel.hi)
	// The mount.
	b.frustum(matIron, 0, 0, 0.2, 0.1, 0, 0.56)
	b.band(b.acc.mid, 0, 0, 0.165, 0.16, 0.22)
	b.box(matSteel, ibox{0, 0, 0.09, 0.09}, 0.56, 0.7)
	b.hit(0, 0.16, 0.2)
	b.hit(-0.44, 0.32, 0.1)
	b.hit(0, 0.05, 0.6)
}

func farseerLive(b *bpen) {
	az := b.t * 0.32
	if !b.lit {
		az = -0.9
	}
	const el = 0.7
	ca, sa := cosf(az), sinf(az)
	ce, se := cosf(el), sinf(el)
	nx, ny, nh := ca*ce, sa*ce, se // the way the dish looks
	e1 := [3]float32{-sa, ca, 0}   // across its face
	e2 := [3]float32{-ca * se, -sa * se, ce}
	// A round of radius r, set back by off along the axis.
	round := func(c gfx.Color, casing bool, r, off float32) {
		b.ovalAt(c, casing, nx*off, ny*off, farseerH+nh*off,
			[3]float32{e1[0] * r, e1[1] * r, 0}, [3]float32{e2[0] * r, e2[1] * r, e2[2] * r})
	}
	// The yoke, then the dish: rim, bowl and the light in its middle.
	b.beam(matIron.mid, 0.06, 0, 0, 0.68, -nx*0.1, -ny*0.1, farseerH-nh*0.1)
	round(matSteel.lo, true, farseerR, 0)
	round(matSteel.mid, false, farseerR*0.84, -0.03)
	round(matSteel.hi, false, farseerR*0.5, -0.07)
	// The feed on its three struts.
	fx, fy, fh := nx*0.3, ny*0.3, farseerH+nh*0.3
	for _, t := range [3]float32{0.6, 2.7, 4.8} {
		u, v := farseerR*0.92*cosf(t), farseerR*0.92*sinf(t)
		b.rod(matIron.lo, 0.022, 0.8, u*e1[0]+v*e2[0], u*e1[1]+v*e2[1], farseerH+v*e2[2], fx, fy, fh)
	}
	b.knobAt(fx, fy, fh, 0.042, b.acc.mid)
	if b.lit {
		b.dotAt(fx, fy, fh, 0.02, gfx.RGB(0xFFF2CC))
	}
}

// ---------------------------------------------------------------------------
// Mender Hub: an arch-shaped depot — two bays under a long lintel — over
// a landing court marked with the repair cross, its drones circling out.
// ---------------------------------------------------------------------------

func menderHubVol(b *bpen) {
	b.shadowG(0, 0, 0.82, 0.76)
	// The landing court in front of the arch.
	cx, cy := diag(0, 0, 0, 0.3)
	b.disc(matPad.lo, cx, cy, 0.003, 0.25)
	b.hoop(alpha(matCream.hi, 0.85), 0.03, 1, cx, cy, 0.004, 0.2)
	cross := func(c gfx.Color, w, l float32) {
		q := ibox{cx, cy, w, l}
		b.topQ(c, q, -1, 1, -1, 1, 0.005)
		q = ibox{cx, cy, l, w}
		b.topQ(c, q, -1, 1, -1, 1, 0.005)
	}
	cross(inkLine, 0.045, 0.13)
	cross(colRepair, 0.03, 0.115)
	pillar := func(a float32) {
		x, y := diag(0, 0, a, -0.06)
		q := ibox{x, y, 0.12, 0.15}
		b.box(matSteel, q, 0, 0.5)
		// A drone bay in its face.
		b.wallL(inkLine, q, -0.6, 0.6, 0.08, 0.3)
		b.wallL(matDark.mid, q, -0.5, 0.5, 0.09, 0.28)
		if !b.tiny() {
			b.wallL(b.lamp(colRepair), q, -0.12, 0.12, 0.34, 0.38)
		}
	}
	pillar(-0.3)
	pillar(0.3)
	lx, ly := diag(0, 0, 0, -0.06)
	lintel := ibox{lx, ly, 0.43, 0.16}
	b.box(matSteel, lintel, 0.5, 0.72)
	b.wallL(b.acc.mid, lintel, -0.94, 0.94, 0.6, 0.66)
	b.wallR(b.acc.lo, lintel, 0.9, -0.9, 0.6, 0.66)
	if !b.tiny() {
		b.topQ(matSteel.lo, lintel, -0.6, -0.2, -0.4, 0.4, 0.722)
		b.topQ(matSteel.lo, lintel, 0.2, 0.6, -0.4, 0.4, 0.722)
	}
	b.hit(-0.3, 0.0, 0.24)
	b.hit(0.2, 0.4, 0.3)
	b.hit(0, -0.06, 0.74)
}

func menderHubLive(b *bpen) {
	cx, cy := diag(0, 0, 0, 0.3)
	for i := 0; i < 3; i++ {
		a := b.t*0.9 + float32(i)*tau/3
		x, y, h := 0.7*cosf(a), 0.1+0.66*sinf(a), 0.86+0.1*sinf(a*2+float32(i))
		if !b.lit {
			// Parked on the court.
			x, y, h = cx-0.12+0.12*float32(i), cy+0.02, 0.03
		}
		p := b.soft(x, y, h) // a drone is no part of the hub
		s := 0.06 * b.k
		if b.live {
			// A drone in flight: a pale lozenge with its lamp.
			b.stampOval(p, s, s*0.62, 0, 0.028*b.k, matCream.hi)
			b.stampOval(p, max(0.02, 0.8*b.px)*b.k, max(0.02, 0.8*b.px)*b.k, 0, 0, b.lamp(colRepair))
			continue
		}
		o := 0.028 * b.k
		b.cv.FillPoly([]gfx.Pt{{X: p.X - s - o, Y: p.Y}, {X: p.X, Y: p.Y - s*0.6 - o}, {X: p.X + s + o, Y: p.Y}, {X: p.X, Y: p.Y + s*0.6 + o}}, b.col(inkLine))
		b.cv.FillPoly([]gfx.Pt{{X: p.X - s, Y: p.Y}, {X: p.X, Y: p.Y - s*0.6}, {X: p.X + s, Y: p.Y}, {X: p.X, Y: p.Y + s*0.6}}, b.col(matCream.hi))
		b.cv.FillCircle(p.X, p.Y, max(0.02, 0.8*b.px)*b.k, b.col(b.lamp(colRepair)))
	}
}

// ---------------------------------------------------------------------------
// Flak Battery: an armoured eight-sided emplacement carrying a turret with
// twin barrels that sweep the sky.
// ---------------------------------------------------------------------------

const flakTurretH = 0.46

func flakVol(b *bpen) {
	b.shadowG(0, 0, 0.78, 0.76)
	var buf [16]float32
	b.prism(matArmor, 0, 0.2, ngon(buf[:0], 0, 0, 0.66, 8, pi/8)...)
	b.hoop(b.acc.mid, 0.06, 1.4, 0, 0, 0.202, 0.46)
	if !b.tiny() {
		// Ammunition lockers on the deck.
		b.box(matDark, ibox{-0.36, 0.06, 0.07, 0.1}, 0.2, 0.3)
	}
	b.cyl(matSteel, 0, 0, 0.25, 0.2, flakTurretH)
	// The mantlet the barrels turn on.
	b.disc(inkLine, 0, 0, flakTurretH+0.004, 0.17)
	b.disc(b.acc.mid, 0, 0, flakTurretH+0.006, 0.135)
	b.hit(-0.3, 0.5, 0.1)
	b.hit(0.36, 0.44, 0.12)
	b.hit(0, 0.25, 0.34)
}

func flakLive(b *bpen) {
	az := -0.9 + 0.75*sinf(b.t*0.45)
	if !b.lit {
		az = -0.4
	}
	const el = 0.62
	ca, sa := cosf(az), sinf(az)
	dx, dy, dh := ca*cosf(el), sa*cosf(el), sinf(el)
	// The two barrels on the mantlet, each with its muzzle brake.
	for _, o := range [2]float32{-0.085, 0.085} {
		x0, y0 := -sa*o, ca*o
		const l = 0.62
		b.beam(matIron.mid, 0.07, x0, y0, flakTurretH+0.04, x0+dx*l, y0+dy*l, flakTurretH+0.04+dh*l)
		b.beam(matDark.mid, 0.09, x0+dx*(l-0.1), y0+dy*(l-0.1), flakTurretH+0.04+dh*(l-0.1), x0+dx*l, y0+dy*l, flakTurretH+0.04+dh*l)
	}
	b.knobAt(0, 0, flakTurretH+0.05, 0.062, matSteel.mid)
}

// ---------------------------------------------------------------------------
// Lance Tower: a three-cornered footing with a capacitor at each corner
// and a tall slim tower carrying a beam emitter that slowly hunts.
// ---------------------------------------------------------------------------

const lanceHeadH = 1.14

func lanceVol(b *bpen) {
	b.shadowG(0, 0, 0.76, 0.72)
	var base [12]float32
	for i := 0; i < 3; i++ {
		a := pi/2 + float32(i)*tau/3
		for k, o := range [2]float32{-0.22, 0.22} {
			base[4*i+2*k], base[4*i+2*k+1] = 0.7*cosf(a+o), 0.7*sinf(a+o)
		}
	}
	b.prism(matArmor, 0, 0.13, base[:]...)
	capAt := func(i int) {
		a := pi/2 + float32(i)*tau/3
		x, y := 0.5*cosf(a), 0.5*sinf(a)
		b.cyl(matDark, x, y, 0.1, 0.13, 0.32)
		b.disc(b.lamp(colLumen), x, y, 0.322, 0.06)
		if b.lit {
			b.glowAt(x, y, 0.33, 0.16, alpha(colLumen, 0.4))
		}
	}
	capAt(1)
	capAt(2)
	b.frustum(matIron, 0, 0, 0.17, 0.09, 0.13, lanceHeadH-0.06)
	b.band(b.acc.mid, 0, 0, 0.15, 0.3, 0.36)
	if !b.tiny() {
		b.band(matIron.lo, 0, 0, 0.125, 0.7, 0.73)
		b.band(matIron.lo, 0, 0, 0.11, 0.9, 0.93)
	}
	capAt(0)
	b.hit(0, 0.17, 0.4)
	b.hit(-0.4, 0.3, 0.1)
	b.hit(0, 0.1, 0.8)
}

func lanceLive(b *bpen) {
	az := -1.0 + 0.8*sinf(b.t*0.3)
	if !b.lit {
		az = -0.5
	}
	const el = 0.45
	dx, dy, dh := cosf(az)*cosf(el), sinf(az)*cosf(el), sinf(el)
	at := func(l float32) (float32, float32, float32) { return dx * l, dy * l, lanceHeadH + dh*l }
	// The emitter: a heavy breech, a long tapering lance, focus rings.
	x0, y0, h0 := at(-0.16)
	x1, y1, h1 := at(0.12)
	x2, y2, h2 := at(0.5)
	b.beam(matSteel.mid, 0.045, x1, y1, h1, x2, y2, h2)
	b.beam(matSteel.mid, 0.1, x0, y0, h0, x1, y1, h1)
	for _, l := range [2]float32{0.24, 0.36} {
		x, y, h := at(l)
		b.knobAt(x, y, h, 0.032, b.acc.mid)
	}
	b.knobAt(0, 0, lanceHeadH, 0.065, matSteel.mid)
	b.dotAt(x2, y2, h2, 0.04, b.lamp(colLumenHi))
	if b.lit {
		v := pulse(b.t, 1.6, 0.4, 1)
		b.glowAt(x2, y2, h2, 0.28, alpha(colLumen, 0.75*v))
	}
}

// ---------------------------------------------------------------------------
// Aegis Projector: three emitter blades leaning in round a shield dome
// that throws out ring after ring.
// ---------------------------------------------------------------------------

func aegisVol(b *bpen) {
	b.shadowG(0, 0, 0.78, 0.76)
	blade := func(a float32) {
		c, s := cosf(a), sinf(a)
		tx, ty := -s, c
		quad := func(r0, r1, w float32) [8]float32 {
			return [8]float32{
				r0*c - w*tx, r0*s - w*ty, r1*c - w*tx, r1*s - w*ty,
				r1*c + w*tx, r1*s + w*ty, r0*c + w*tx, r0*s + w*ty,
			}
		}
		b.hexa(matSteel, 0.1, 0.5, quad(0.5, 0.7, 0.15), quad(0.46, 0.6, 0.12))
		b.hexa(matSteel, 0.5, 0.82, quad(0.46, 0.6, 0.12), quad(0.3, 0.4, 0.08))
		// The accent strip up its outer face, for the blades that show it.
		if s > 0 {
			b.face(b.acc.mid, 0.7*c-0.05*tx, 0.7*s-0.05*ty, 0.14, 0.7*c+0.05*tx, 0.7*s+0.05*ty, 0.14,
				0.6*c+0.04*tx, 0.6*s+0.04*ty, 0.48, 0.6*c-0.04*tx, 0.6*s-0.04*ty, 0.48)
		}
	}
	blade(-pi / 2)
	b.cyl(matIron, 0, 0, 0.5, 0, 0.12)
	b.hoop(b.acc.mid, 0.05, 1.2, 0, 0, 0.122, 0.4)
	m := mat3{gfx.RGB(0xDDF6FF), colShield, gfx.RGB(0x2F7FA6)}
	if !b.lit {
		m = mat(0x61798A, 0x3C5161, 0x22313D)
	}
	b.domeUp(m, 0, 0, 0.3, 0.12, 0.34)
	blade(5 * pi / 6)
	blade(pi / 6)
	b.hit(-0.5, 0.3, 0.3)
	b.hit(0.5, 0.32, 0.26)
	b.hit(0, 0.3, 0.2)
}

func aegisLive(b *bpen) {
	if !b.lit {
		return
	}
	const period = 2.6
	v := pulse(b.t, period, 0, 1)
	b.glowAt(0, 0, 0.36, 0.72, alpha(colShield, 0.22+0.25*v))
	f := frac(b.t / period)
	b.hoop(alpha(gfx.RGB(0xC8F1FF), 0.7*(1-f)*(1-f)), 0.035, 1.2, 0, 0, 0.3+0.5*f, 0.3+0.5*f)
	b.dotAt(-0.07, 0, 0.42, 0.045+0.02*v, gfx.RGB(0xF4FDFF))
}

// ---------------------------------------------------------------------------
// Mass Driver: a rail gun slanting up across the tile from a breech on a
// turntable, coil after coil lighting up along the barrel as it charges.
// ---------------------------------------------------------------------------

var massDriverCoilT = [5]float32{0.18, 0.36, 0.54, 0.72, 0.9}

// driverBarrel returns the point a fraction t along the barrel, from the
// breech to the muzzle.
func driverBarrel(t float32) (x, y, h float32) {
	x0, y0 := diag(0, 0, 0, 0.2)
	x1, y1 := diag(0, 0, 0, -0.54)
	return lerpf(x0, x1, t), lerpf(y0, y1, t), lerpf(0.32, 1.1, t)
}

func massDriverVol(b *bpen) {
	b.shadowG(0, 0.02, 0.8, 0.74)
	tx, ty := diag(0, 0, 0, 0.26)
	b.cyl(matIron, tx, ty, 0.36, 0, 0.12)
	b.hoop(b.acc.mid, 0.05, 1.2, tx, ty, 0.122, 0.28)
	// The prop under the barrel.
	px, py, ph := driverBarrel(0.55)
	b.beam(matIron.mid, 0.045, px+0.16, py+0.16, 0, px, py, ph-0.06)
	b.beam(matIron.mid, 0.045, px-0.16, py-0.16, 0, px, py, ph-0.06)
	// The barrel: a dark channel between two bright rails, ringed by
	// coils.
	x0, y0, h0 := driverBarrel(0)
	x1, y1, h1 := driverBarrel(1)
	b.beam(matDark.mid, 0.13, x0, y0, h0, x1, y1, h1)
	for _, s := range [2]float32{-0.045, 0.045} {
		sx, sy := diag(0, 0, s, 0)
		b.edge(matSteel.hi, 0.028, 0.8, x0+sx, y0+sy, h0, x1+sx, y1+sy, h1)
	}
	for _, t := range massDriverCoilT {
		driverCoil(b, t, matCopper.mid)
	}
	// The breech.
	breech := ibox{tx, ty, 0.13, 0.19}
	b.box(matArmor, breech, 0.12, 0.42)
	b.wallL(b.acc.mid, breech, -0.8, 0.8, 0.3, 0.36)
	if !b.tiny() {
		// Slugs racked beside the turntable.
		for i := 0; i < 3; i++ {
			x, y := diag(tx, ty, 0.3, -0.12+0.11*float32(i))
			b.box(matOf(gfx.RGB(0xAEB8C6)), ibox{x, y, 0.035, 0.035}, 0, 0.1)
		}
	}
	b.hit(tx-0.1, ty+0.2, 0.26)
	b.hit(0.1, 0.2, 0.08)
	b.hit(0.1, -0.1, 0.6)
}

// driverCoil draws the coil a fraction t along the barrel: a band across
// it.
func driverCoil(b *bpen, t float32, c gfx.Color) {
	x, y, h := driverBarrel(t)
	sx, sy := diag(0, 0, 0.1, 0)
	if b.live {
		b.beam(c, 0.05, x-sx*0.86, y-sy*0.86, h, x+sx*0.86, y+sy*0.86, h)
		return
	}
	b.edge(inkLine, 0.085, 2.2, x-sx, y-sy, h, x+sx, y+sy, h)
	b.edge(c, 0.05, 1, x-sx*0.86, y-sy*0.86, h, x+sx*0.86, y+sy*0.86, h)
}

func massDriverLive(b *bpen) {
	tx, ty := diag(0, 0, 0, 0.26)
	if !b.lit {
		b.dotAt(tx, ty+0.1, 0.44, 0.04, colWindowOff)
		return
	}
	// Charge runs up the barrel, then the breech lamp flashes.
	const period = 3.2
	u := frac(b.t/period) * 7
	i := int(u)
	if i >= 0 && i < len(massDriverCoilT) {
		driverCoil(b, massDriverCoilT[i], gfx.RGB(0xFFE9A8))
		x, y, h := driverBarrel(massDriverCoilT[i])
		b.glowAt(x, y, h, 0.28, alpha(colHazard, 0.7))
	}
	c := gfx.RGB(0x7A5A2A)
	if i >= len(massDriverCoilT) {
		c = gfx.RGB(0xFFF2CC)
		b.glowAt(tx, ty+0.1, 0.44, 0.28, alpha(colLamp, 0.7))
	}
	b.dotAt(tx, ty+0.1, 0.44, 0.04, c)
}
