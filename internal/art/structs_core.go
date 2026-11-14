package art

import (
	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

func init() {
	structArts = [model.NumBuildingKinds]structArt{
		model.Keystone:       {keystoneVol, keystoneLive, 1.1},
		model.Conduit:        {conduitVol, conduitLive, 0.32},
		model.Silo:           {siloVol, siloLive, 0.78},
		model.SurveyMast:     {surveyMastVol, surveyMastLive, 1.52},
		model.Sunvane:        {sunvaneVol, sunvaneLive, 0.68},
		model.ThermalStack:   {thermalStackVol, thermalStackLive, 1.58},
		model.LumenCore:      {lumenCoreVol, lumenCoreLive, 1},
		model.Habitat:        {habitatVol, habitatLive, 0.64},
		model.Aerator:        {aeratorVol, aeratorLive, 0.76},
		model.Greenhouse:     {greenhouseVol, greenhouseLive, 0.72},
		model.Commons:        {commonsVol, commonsLive, 0.62},
		model.BoreRig:        {boreRigVol, boreRigLive, 1.2},
		model.Smelter:        {smelterVol, smelterLive, 1.26},
		model.Cracker:        {crackerVol, crackerLive, 1.56},
		model.Fabricator:     {fabricatorVol, fabricatorLive, 0.76},
		model.Laboratory:     {laboratoryVol, laboratoryLive, 0.74},
		model.LaunchCradle:   {launchCradleVol, launchCradleLive, 1.1},
		model.DockRing:       {dockRingVol, dockRingLive, 1.18},
		model.ExchangeDock:   {exchangeDockVol, exchangeDockLive, 1.14},
		model.FarseerArray:   {farseerVol, farseerLive, 1.42},
		model.MenderHub:      {menderHubVol, menderHubLive, 1.06},
		model.FlakBattery:    {flakVol, flakLive, 1.12},
		model.LanceTower:     {lanceVol, lanceLive, 1.6},
		model.AegisProjector: {aegisVol, aegisLive, 1.04},
		model.MassDriver:     {massDriverVol, massDriverLive, 1.4},
	}
}

// The diagonals a box's edges run along, as ground vectors of unit length
// per unit of screen x: A to the lower right, B to the lower left.
//
// diag returns the ground point a along A and c along B from (x, y).
func diag(x, y, a, c float32) (float32, float32) { return x + a - c, y + a + c }

// ---------------------------------------------------------------------------
// Keystone: a hexagonal bastion in two tiers under a faceted accent roof,
// with buttresses towards the corners of its tile and a beacon on a mast.
// ---------------------------------------------------------------------------

func keystoneVol(b *bpen) {
	b.shadowG(0, 0, 0.86, 0.84)
	lug := func(a float32) {
		c, s := cosf(a), sinf(a)
		nx, ny := -s, c
		b.prism(matIron, 0, 0.15,
			0.5*c-0.2*nx, 0.5*s-0.2*ny, 0.9*c-0.1*nx, 0.9*s-0.1*ny,
			0.9*c+0.1*nx, 0.9*s+0.1*ny, 0.5*c+0.2*nx, 0.5*s+0.2*ny)
		b.face(b.acc.mid, 0.68*c-0.09*nx, 0.68*s-0.09*ny, 0.155, 0.85*c-0.06*nx, 0.85*s-0.06*ny, 0.155,
			0.85*c+0.06*nx, 0.85*s+0.06*ny, 0.155, 0.68*c+0.09*nx, 0.68*s+0.09*ny, 0.155)
	}
	lug(-pi / 2)
	var buf [12]float32
	lower := ngon(buf[:0], 0, 0, 0.72, 6, 0)
	b.prism(matSteel, 0, 0.3, lower...)
	if !b.tiny() {
		// A dark course round the foot of the wall and a row of lit ports.
		for i := 1; i <= 3; i++ {
			x0, y0, x1, y1 := lower[2*i-2], lower[2*i-1], lower[2*i], lower[2*i+1]
			b.wallQ(alpha(matSteel.lo, 0.6), x0, y0, x1, y1, 0, 1, 0, 0.05)
			for _, u := range [2]float32{0.3, 0.62} {
				b.wallQ(b.lamp(colWindow), x0, y0, x1, y1, u, u+0.1, 0.14, 0.2)
			}
		}
	}
	lug(pi / 6)
	lug(5 * pi / 6)
	var buf2 [12]float32
	upper := ngon(buf2[:0], 0, 0, 0.5, 6, 0)
	b.prism(matCream, 0.3, 0.56, upper...)
	for i := 1; i <= 3; i++ {
		x0, y0, x1, y1 := upper[2*i-2], upper[2*i-1], upper[2*i], upper[2*i+1]
		b.wallQ(colWindowOff, x0, y0, x1, y1, 0.16, 0.84, 0.39, 0.48)
		n := 3
		if b.tiny() {
			n = 2
		}
		for k := 0; k < n; k++ {
			u := 0.2 + 0.6*float32(k)/float32(n)
			b.wallQ(b.lamp(colWindow), x0, y0, x1, y1, u+0.03, u+0.6/float32(n)-0.03, 0.4, 0.47)
		}
	}
	var buf3 [12]float32
	b.spire(b.acc, 0.56, 0.9, 0, 0, ngon(buf3[:0], 0, 0, 0.56, 6, 0)...)
	b.beam(matIron.hi, 0.035, 0, 0, 0.88, 0, 0, 1.06)
	b.hit(-0.54, 0.31, 0.16)
	b.hit(0.2, 0.43, 0.42)
	b.hit(0.5, 0.3, 0.12)
	b.hit(-0.15, 0.1, 0.7)
}

func keystoneLive(b *bpen) {
	// A double flash every 2.4 s.
	ph := wrap(b.t, 2.4)
	on := ph < 0.14 || (ph > 0.32 && ph < 0.46)
	switch {
	case !b.lit:
		b.dotAt(0, 0, 1.08, 0.05, colWindowOff)
	case on:
		b.glowAt(0, 0, 1.08, 0.5, alpha(gfx.RGB(0xFFE7B0), 0.7))
		b.dotAt(0, 0, 1.08, 0.07, gfx.RGB(0xFFF6DC))
	default:
		b.glowAt(0, 0, 1.08, 0.24, alpha(colLamp, 0.3))
		b.dotAt(0, 0, 1.08, 0.05, colLamp)
	}
}

// ---------------------------------------------------------------------------
// Conduit: trenches to the six neighbouring tiles, meeting in a low
// junction box.
// ---------------------------------------------------------------------------

func conduitVol(b *bpen) {
	for i := 0; i < 6; i++ {
		a := float32(i) * tau / 6
		c, s := cosf(a), sinf(a)
		b.edge(alpha(inkLine, 0.8), 0.1, 2.4, 0.2*c, 0.2*s, 0, 0.868*c, 0.868*s, 0)
		b.edge(matIron.lo, 0.05, 1.1, 0.2*c, 0.2*s, 0, 0.868*c, 0.868*s, 0)
		if !b.tiny() {
			b.edge(matSteel.mid, 0.02, 0.7, 0.2*c, 0.2*s, 0, 0.868*c, 0.868*s, 0)
		}
	}
	b.shadowG(0, 0, 0.36, 0.36)
	var buf [12]float32
	b.prism(matIron, 0, 0.07, ngon(buf[:0], 0, 0, 0.32, 6, 0)...)
	q := ibox{0, 0, 0.11, 0.11}
	b.box(matSteel, q, 0.07, 0.2)
	b.topQ(b.acc.mid, q, -0.6, 0.6, -0.6, 0.6, 0.202)
	b.hit(0, 0.12, 0.12)
	b.hit(-0.3, 0.2, 0.02)
	b.hit(0.4, 0.1, 0.02)
}

func conduitLive(b *bpen) {
	if !b.lit {
		b.dotAt(0, 0, 0.21, 0.04, colWindowOff)
		return
	}
	v := pulse(b.t, 3.2, 0.3, 1)
	b.dotAt(0, 0, 0.21, 0.045, gfx.Lerp(b.acc.lo, gfx.Lighten(b.acc.hi, 0.3), v))
}

// ---------------------------------------------------------------------------
// Silo: three squat pressure tanks in a trefoil, joined by a manifold.
// ---------------------------------------------------------------------------

var siloTanks = [3][2]float32{{0, -0.34}, {-0.32, 0.2}, {0.32, 0.2}}

func siloVol(b *bpen) {
	b.shadowG(0, 0, 0.74, 0.7)
	for i, t := range siloTanks {
		x, y := t[0], t[1]
		if i == 1 {
			// The manifold lies between the tanks.
			b.tube(matIron, -0.2, 0.05, 0.08, 0.2, 0.05, 0.08, 0.05)
		}
		b.cyl(matSteel, x, y, 0.3, 0, 0.44)
		b.band(b.acc.mid, x, y, 0.3, 0.27, 0.35)
		if !b.tiny() {
			b.band(alpha(matSteel.lo, 0.7), x, y, 0.3, 0, 0.04)
		}
		b.domeUp(matSteel, x, y, 0.3, 0.44, 0.15)
		b.dotAt(x, y, 0.6, 0.045, matIron.lo)
	}
	b.hit(-0.32, 0.5, 0.2)
	b.hit(0.4, 0.48, 0.14)
	b.hit(0.05, -0.2, 0.5)
}

func siloLive(b *bpen) {
	// A level lamp on the front of each tank, taking turns.
	for i, t := range siloTanks {
		x, y := t[0], t[1]+0.3
		if !b.lit {
			b.dotAt(x, y, 0.16, 0.035, colWindowOff)
			continue
		}
		v := pulse(b.t+float32(i)*1.3, 3.9, 0.25, 1)
		b.dotAt(x, y, 0.16, 0.035, gfx.Lerp(gfx.RGB(0x3E5A3C), gfx.RGB(0xA8F0A0), v))
	}
}

// ---------------------------------------------------------------------------
// Survey Mast: a tall lattice mast on a tripod inside its sounding circle,
// a sensor arm turning at its head, pings running out through the rock.
// ---------------------------------------------------------------------------

func surveyMastVol(b *bpen) {
	// The sounding circle scratched into the ground.
	for i := 0; i < 12; i++ {
		a := float32(i) * tau / 12
		b.arcG(alpha(matCream.hi, 0.55), 0.03, 1, 0, 0, 0, 0.72, a+0.07, a+tau/12-0.07)
	}
	b.shadowG(0, 0, 0.3, 0.3)
	// The mast is tall: its shadow is long.
	b.edge(inkShadow, 0.07, 1.2, 0.05, 0.03, 0, 0.62, 0.3, 0)
	leg := func(a float32) {
		x, y := 0.5*cosf(a), 0.5*sinf(a)
		b.beam(matSteel.mid, 0.06, x, y, 0, 0, 0, 0.44)
		b.disc(matIron.lo, x, y, 0.01, 0.09)
		b.hoop(inkLine, b.ow(), 1, x, y, 0.01, 0.09)
	}
	leg(7 * pi / 6)
	leg(11 * pi / 6)
	leg(pi / 2)
	b.cyl(matSteel, 0, 0, 0.1, 0.36, 0.52)
	b.band(b.acc.mid, 0, 0, 0.1, 0.42, 0.47)
	// The mast: a pole with spars.
	b.beam(matSteel.hi, 0.045, 0, 0, 0.5, 0, 0, 1.3)
	if !b.tiny() {
		for _, h := range [2]float32{0.78, 1.04} {
			b.beam(matIron.hi, 0.025, -0.11, 0, h, 0.11, 0, h)
		}
		b.edge(alpha(matIron.hi, 0.7), 0.014, 0.6, -0.11, 0, 0.78, 0, 0, 1.3)
		b.edge(alpha(matIron.hi, 0.7), 0.014, 0.6, 0.11, 0, 0.78, 0, 0, 1.3)
	}
	b.hit(0, 0.1, 0.44)
	b.hit(0, 0.5, 0.03)
	b.hit(0, 0, 0.9)
}

func surveyMastLive(b *bpen) {
	if b.lit {
		f := wrap(b.t, 2.6) / 2.6
		b.hoop(alpha(gfx.RGB(0xBFE9FF), 0.6*(1-f)), 0.03, 1, 0, 0, 0, 0.2+0.6*f)
	}
	// The sensor arm swings round the head of the mast.
	a := b.t * 1.1
	c, s := cosf(a), sinf(a)
	const h = 1.32
	b.beam(matSteel.hi, 0.04, -0.14*c, -0.14*s, h, 0.34*c, 0.34*s, h)
	x, y := 0.36*c, 0.36*s
	b.beam(matCream.hi, 0.07, x+0.1*s, y-0.1*c, h, x-0.1*s, y+0.1*c, h)
	b.knobAt(0, 0, h+0.08, 0.035, b.lamp(gfx.RGB(0xBFE9FF)))
	if b.lit {
		b.glowAt(0, 0, h+0.08, 0.16, alpha(gfx.RGB(0xBFE9FF), 0.4))
	}
}

// ---------------------------------------------------------------------------
// Sunvane: four collector panels set as a pinwheel on a short post, lying
// almost flat to catch the red sun.
// ---------------------------------------------------------------------------

// sunvanePanels are the four vanes, set round the hub like the sails of a
// pinwheel: centre and half extents along the two diagonals, far ones
// first.
var sunvanePanels = [4]ibox{
	{-0.2, -0.4, 0.22, 0.115}, // far
	{0.4, -0.2, 0.115, 0.22},  // right
	{-0.4, 0.2, 0.115, 0.22},  // left
	{0.2, 0.4, 0.22, 0.115},   // near
}

const sunvaneH = 0.3

func sunvaneVol(b *bpen) {
	for _, q := range sunvanePanels {
		// The vane's shadow on the ground.
		s := q
		s.x += 0.09
		s.y += 0.045
		lx, ly := s.pt(-1, 1)
		fx, fy := s.pt(1, 1)
		rx, ry := s.pt(1, -1)
		kx, ky := s.pt(-1, -1)
		b.face(inkShadow, lx, ly, 0, fx, fy, 0, rx, ry, 0, kx, ky, 0)
	}
	b.shadowG(0, 0, 0.2, 0.2)
	b.frustum(matIron, 0, 0, 0.16, 0.1, 0, 0.07)
	panel := func(i int) {
		q := sunvanePanels[i]
		long := q.da > q.db
		// The arm that carries the vane, and the vane: a frame with the
		// cells set into it.
		b.beam(matIron.mid, 0.04, 0, 0, sunvaneH-0.07, q.x*0.8, q.y*0.8, sunvaneH-0.05)
		b.box(matSteel, q, sunvaneH-0.045, sunvaneH)
		b.topQ(matSolar.hi, q, -0.9, 0.9, -0.86, 0.86, sunvaneH+0.001)
		// The half of every vane nearer the hub lies in the light a little
		// less.
		if long {
			b.topQ(matSolar.mid, q, -0.9, 0, -0.86, 0.86, sunvaneH+0.002)
		} else {
			b.topQ(matSolar.mid, q, -0.9, 0.9, -0.86, 0, sunvaneH+0.002)
		}
		if !b.tiny() {
			jc := alpha(inkLine, 0.55)
			for _, u := range [3]float32{-0.45, 0, 0.45} {
				if long {
					x0, y0 := q.pt(u, -0.86)
					x1, y1 := q.pt(u, 0.86)
					b.edge(jc, 0.02, 0.7, x0, y0, sunvaneH, x1, y1, sunvaneH)
				} else {
					x0, y0 := q.pt(-0.9, u)
					x1, y1 := q.pt(0.9, u)
					b.edge(jc, 0.02, 0.7, x0, y0, sunvaneH, x1, y1, sunvaneH)
				}
			}
			if long {
				x0, y0 := q.pt(-0.9, 0)
				x1, y1 := q.pt(0.9, 0)
				b.edge(jc, 0.02, 0.7, x0, y0, sunvaneH, x1, y1, sunvaneH)
			} else {
				x0, y0 := q.pt(0, -0.86)
				x1, y1 := q.pt(0, 0.86)
				b.edge(jc, 0.02, 0.7, x0, y0, sunvaneH, x1, y1, sunvaneH)
			}
		}
	}
	panel(0)
	panel(1)
	b.cyl(matIron, 0, 0, 0.075, 0.05, sunvaneH+0.1)
	b.disc(b.acc.hi, 0, 0, sunvaneH+0.105, 0.055)
	panel(2)
	panel(3)
	// The owner's colour on the outer end of every vane.
	b.topQ(b.acc.mid, sunvanePanels[0], -0.96, -0.76, -0.86, 0.86, sunvaneH+0.003)
	b.topQ(b.acc.mid, sunvanePanels[1], -0.9, 0.9, -0.96, -0.76, sunvaneH+0.003)
	b.topQ(b.acc.mid, sunvanePanels[2], -0.9, 0.9, 0.76, 0.96, sunvaneH+0.003)
	b.topQ(b.acc.mid, sunvanePanels[3], 0.76, 0.96, -0.86, 0.86, sunvaneH+0.003)
	b.hit(-0.4, 0.2, sunvaneH)
	b.hit(0.2, 0.4, sunvaneH)
	b.hit(0.3, -0.15, sunvaneH)
}

func sunvaneLive(b *bpen) {
	if !b.lit {
		return
	}
	// The sun's reflection slides along every vane in step, from the hub
	// outwards.
	f := wrap(b.t, 6) / 6
	if f > 0.5 {
		return
	}
	u := f*4 - 1 // -1..1 along the vane
	c := gfx.Color{R: 0xFF, G: 0xE6, B: 0xD6, A: uint8(255 * 0.7 * sinf(f*2*pi))}
	for i, q := range sunvanePanels {
		v := u
		if i == 0 || i == 1 {
			v = -u
		}
		if q.da > q.db {
			b.topQ(c, q, max(-0.9, v-0.14), min(0.9, v+0.14), -0.86, 0.86, sunvaneH+0.004)
		} else {
			b.topQ(c, q, -0.9, 0.9, max(-0.86, v-0.14), min(0.86, v+0.14), sunvaneH+0.004)
		}
	}
}

// ---------------------------------------------------------------------------
// Thermal Stack: a generator house with a fuel tank on its roof and two
// burner stacks, one very tall, glowing and smoking at the top.
// ---------------------------------------------------------------------------

// thermalChimneys are the two chimneys: ground position, foot radius, height.
var thermalChimneys = [2][4]float32{{0.42, -0.22, 0.13, 1.42}, {0.52, 0.2, 0.11, 0.98}}

func thermalStackVol(b *bpen) {
	b.shadowG(-0.05, 0.05, 0.8, 0.74)
	stack := func(s [4]float32) {
		x, y, r, h := s[0], s[1], s[2], s[3]
		b.frustum(matIron, x, y, r, r*0.74, 0, h)
		b.band(b.acc.mid, x, y, r*0.79, h-0.2, h-0.13)
		if !b.tiny() {
			b.band(alpha(matIron.lo, 0.8), x, y, r*0.98, 0.08, 0.14)
		}
		b.disc(colPit, x, y, h, r*0.5)
	}
	stack(thermalChimneys[0])
	q := ibox{-0.2, 0.08, 0.26, 0.22}
	b.box(matSteel, q, 0, 0.4)
	b.wallL(b.acc.mid, q, -1, 1, 0.3, 0.36)
	if !b.tiny() {
		for i := 0; i < 4; i++ {
			u := -0.7 + 0.45*float32(i)
			b.wallL(alpha(matSteel.lo, 0.8), q, u, u+0.12, 0.06, 0.24)
		}
		b.winR(colWindow, q, 2, 0.7, -0.5, 0.14, 0.26)
	}
	// The fuel tank lying along the roof.
	x0, y0 := q.pt(-0.6, -0.2)
	x1, y1 := q.pt(0.5, -0.2)
	b.tube(matFuel, x0, y0, 0.49, x1, y1, 0.49, 0.085)
	stack(thermalChimneys[1])
	b.hit(-0.44, 0.34, 0.2)
	b.hit(0.06, 0.34, 0.16)
	b.hit(-0.2, 0.05, 0.42)
}

func thermalStackLive(b *bpen) {
	for i, s := range thermalChimneys {
		x, y, r, h := s[0], s[1], s[2], s[3]
		if !b.lit {
			continue
		}
		v := pulse(b.t+float32(i)*0.7, 1.7, 0, 1)
		b.glowAt(x, y, h, 0.4, alpha(colHot, 0.4+0.25*v))
		b.disc(gfx.Lerp(colHot, colHotCore, v), x, y, h, r*0.42)
		// Exhaust rising and drifting off to the right, thinning as it goes.
		n := 3 - i // the tall chimney smokes more
		for k := 0; k < n; k++ {
			d := frac(b.t*0.3 + float32(k)/float32(n) + float32(i)*0.17)
			b.puffAt(x+0.2*d, y, h+0.08+0.5*d, 0.06+0.1*d, gfx.Color{R: 0xC8, G: 0xB8, B: 0xB0, A: uint8(70 * (1 - d))})
		}
	}
}

// ---------------------------------------------------------------------------
// Lumen Core: a round containment vessel ringed by six pylons, a violet
// crystal hanging over its well.
// ---------------------------------------------------------------------------

var lumenMat = mat3{colLumenHi, colLumen, gfx.RGB(0x6B4FB8)}

func lumenCoreVol(b *bpen) {
	b.shadowG(0, 0, 0.8, 0.78)
	pylon := func(i int) {
		a := float32(i)*tau/6 + tau/12
		x, y := 0.64*cosf(a), 0.64*sinf(a)
		b.box(matSteel, ibox{x, y, 0.06, 0.06}, 0, 0.5)
		b.dotAt(x, y+0.04, 0.4, 0.03, b.lamp(colLumen))
	}
	for _, i := range [3]int{3, 4, 5} {
		pylon(i)
	}
	b.cyl(matIron, 0, 0, 0.52, 0, 0.32)
	b.band(b.acc.mid, 0, 0, 0.52, 0.19, 0.26)
	b.disc(matDark.lo, 0, 0, 0.32, 0.38)
	b.hoop(inkLine, b.ow(), 1, 0, 0, 0.32, 0.38)
	if b.lit {
		b.glowAt(0, 0, 0.36, 0.5, alpha(colLumen, 0.35))
	}
	// The crystal: two six-sided points base to base.
	m := lumenMat
	if !b.lit {
		m = mat(0x6F6690, 0x4A416B, 0x2C2744)
	}
	var buf [12]float32
	gem := ngon(buf[:0], 0, 0, 0.17, 6, pi/6)
	b.spire(mat3{m.mid, m.lo, gfx.Darken(m.lo, 0.3)}, 0.66, 0.42, 0, 0, gem...)
	b.spire(m, 0.66, 0.98, 0, 0, gem...)
	for _, i := range [3]int{0, 1, 2} {
		pylon(i)
	}
	b.hit(-0.3, 0.42, 0.14)
	b.hit(0.34, 0.4, 0.12)
	b.hit(0, 0.2, 0.33)
}

func lumenCoreLive(b *bpen) {
	if !b.lit {
		return
	}
	v := pulse(b.t, 2.2, 0, 1)
	b.glowAt(0, 0, 0.68, 0.8, alpha(colLumen, 0.28+0.28*v))
	p := b.at(0, 0, 0.7)
	b.cv.GlowCore(p.X, p.Y, 0.44*b.k, alpha(colLumenHi, 0.45+0.4*v))
	a := b.t * 0.9
	for i := 0; i < 2; i++ {
		// A streak of light on its way round the crystal.
		a0 := a + float32(i)*pi
		a1 := a0 + 0.9
		b.rod(alpha(colLumenHi, 0.85), 0.04, 1.2, 0.3*cosf(a0), 0.3*sinf(a0), 0.66, 0.3*cosf(a1), 0.3*sinf(a1), 0.66)
	}
}

// ---------------------------------------------------------------------------
// Habitat: three pressure cabins, two side by side and one stacked on
// them, with a row of lit ports along each.
// ---------------------------------------------------------------------------

func habitatVol(b *bpen) {
	b.shadowG(0, 0.02, 0.8, 0.72)
	const r = 0.17
	cabin := func(c, h, a0, a1 float32, collar float32) {
		x0, y0 := diag(0, 0, a0, c)
		x1, y1 := diag(0, 0, a1, c)
		b.tube(matCream, x0, y0, h, x1, y1, h, r)
		// The accent collar.
		cx0, cy0 := diag(0, 0, collar-0.035, c)
		cx1, cy1 := diag(0, 0, collar+0.035, c)
		p, q := b.at(cx0, cy0, h), b.at(cx1, cy1, h)
		b.cv.Line(p.X, p.Y, q.X, q.Y, 2*r*b.k, b.col(b.acc.mid))
		b.cv.Line(p.X-0.3*r*b.k, p.Y-0.45*r*b.k, q.X-0.3*r*b.k, q.Y-0.45*r*b.k, 0.6*r*b.k, b.col(b.acc.hi))
		// The glazing band along the flank and the cabin lights in it.
		w0, w1 := a0+0.1, a1-0.06
		if collar < 0 {
			w0 = collar + 0.07
		} else {
			w1 = collar - 0.07
		}
		gx0, gy0 := diag(0, 0, w0, c)
		gx1, gy1 := diag(0, 0, w1, c)
		const drop = 0.035
		b.edge(colWindowOff, 0.085, 1.6, gx0, gy0, h-drop, gx1, gy1, h-drop)
		n := 5
		if b.tiny() {
			n = 3
		}
		for k := 0; k < n; k++ {
			u0 := (float32(k) + 0.2) / float32(n)
			u1 := (float32(k) + 0.8) / float32(n)
			b.edge(b.lamp(gfx.RGB(0xFFCF70)), 0.055, 1,
				lerpf(gx0, gx1, u0), lerpf(gy0, gy1, u0), h-drop, lerpf(gx0, gx1, u1), lerpf(gy0, gy1, u1), h-drop)
		}
		if b.lit && !b.tiny() {
			b.glowAt((gx0+gx1)/2, (gy0+gy1)/2, h-drop, 0.3, alpha(colWindow, 0.14))
		}
		// The hatch on the near end.
		b.dotAt(x1, y1, h, r*0.62, matCream.lo)
		b.dotAt(x1, y1, h, r*0.36, matIron.lo)
	}
	cabin(-0.17, 0.17, -0.4, 0.4, -0.26)
	cabin(0.17, 0.17, -0.4, 0.4, 0.22)
	cabin(0, 0.45, -0.34, 0.34, -0.2)
	b.hit(-0.2, 0.14, 0.2)
	b.hit(0.3, 0.44, 0.16)
	b.hit(0.05, 0.05, 0.5)
}

func habitatLive(b *bpen) {
	// The airlock lamp on the top cabin's hatch.
	x, y := diag(0, 0, 0.34, 0)
	if !b.lit {
		b.dotAt(x, y, 0.45, 0.035, colWindowOff)
		return
	}
	v := pulse(b.t, 4, 0.4, 1)
	b.dotAt(x, y, 0.45, 0.035, gfx.Lerp(gfx.RGB(0x7A5A2A), colLamp, v))
}

// ---------------------------------------------------------------------------
// Aerator: a plant with one great fan in its roof and two air tanks beside
// it.
// ---------------------------------------------------------------------------

const aeratorH = 0.4

var aeratorBox = ibox{-0.1, -0.04, 0.32, 0.32}

func aeratorVol(b *bpen) {
	b.shadowG(0, 0.02, 0.82, 0.76)
	q := aeratorBox
	b.box(matSteel, q, 0, aeratorH)
	b.wallL(b.acc.mid, q, -0.8, 0.8, 0.06, 0.12)
	if !b.tiny() {
		for i := 0; i < 4; i++ {
			h := 0.18 + 0.05*float32(i)
			b.wallR(alpha(matDark.mid, 0.9), q, 0.7, -0.7, h, h+0.025)
		}
	}
	// The fan well in the roof.
	b.disc(matDark.lo, q.x, q.y, aeratorH+0.001, 0.36)
	b.hoop(inkLine, b.ow(), 1.2, q.x, q.y, aeratorH+0.001, 0.36)
	b.hoop(matIron.mid, 0.03, 1, q.x, q.y, aeratorH+0.001, 0.32)
	for _, t := range [2][2]float32{{0.6, 0.3}, {0.42, 0.5}} {
		b.cyl(matIce, t[0], t[1], 0.1, 0, 0.46)
		b.domeUp(matIce, t[0], t[1], 0.1, 0.46, 0.07)
	}
	b.hit(-0.4, 0.3, 0.2)
	b.hit(0.2, 0.26, 0.2)
	b.hit(-0.1, 0.0, aeratorH)
}

func aeratorLive(b *bpen) {
	q := aeratorBox
	const h = aeratorH + 0.004
	a := b.t * 4.2
	for i := 0; i < 2; i++ {
		// Two blades right across the hub make the four of the fan.
		c, s := cosf(a+float32(i)*pi/2), sinf(a+float32(i)*pi/2)
		b.rod(matSteel.mid, 0.1, 1.6, q.x-0.3*c, q.y-0.3*s, h, q.x+0.3*c, q.y+0.3*s, h)
	}
	b.disc(inkLine, q.x, q.y, h, 0.09)
	b.disc(b.acc.mid, q.x, q.y, h+0.01, 0.065)
}

// ---------------------------------------------------------------------------
// Greenhouse: a long glasshouse under a ridged roof, green behind the
// panes.
// ---------------------------------------------------------------------------

func greenhouseVol(b *bpen) {
	b.shadowG(0, 0.02, 0.82, 0.76)
	q := ibox{0, 0, 0.42, 0.23}
	b.box(matSteel, q, 0, 0.1)
	m := matLeaf
	if !b.lit {
		m = mat(0x587A66, 0x35574A, 0x20372F)
	}
	const eave, ridge = 0.25, 0.5
	p := func(a, c float32) (float32, float32) { return q.pt(a*0.96, c*0.9) }
	lx, ly := p(-1, 1)
	fx, fy := p(1, 1)
	rx, ry := p(1, -1)
	bx, by := p(-1, 0) // back end of the ridge
	tx, ty := p(1, 0)  // near end of the ridge
	// The left wall, the gable end and the roof slope over the left wall.
	b.face(m.mid, lx, ly, 0.1, fx, fy, 0.1, fx, fy, eave, lx, ly, eave)
	b.face(m.lo, fx, fy, 0.1, rx, ry, 0.1, rx, ry, eave, tx, ty, ridge, fx, fy, eave)
	b.face(m.hi, lx, ly, eave, fx, fy, eave, tx, ty, ridge, bx, by, ridge)
	if !b.tiny() {
		// Rows of plants show through the roof and the wall.
		for i := 0; i < 6; i++ {
			a := -0.8 + 0.32*float32(i)
			x0, y0 := p(a, 0.82)
			x1, y1 := p(a, 0.22)
			b.edge(alpha(m.mid, 0.9), 0.07, 1.2, x0, y0, lerpf(eave, ridge, 0.18), x1, y1, lerpf(eave, ridge, 0.78))
			b.wallQ(alpha(m.lo, 0.8), lx, ly, fx, fy, (a+1)/2-0.03, (a+1)/2+0.05, 0.12, 0.22)
		}
	}
	// Glazing bars.
	bars := 5
	if b.tiny() {
		bars = 3
	}
	hl := max(0.025, 0.9*b.px)
	for i := 0; i <= bars; i++ {
		a := -1 + 2*float32(i)/float32(bars)
		x0, y0 := p(a, 1)
		x1, y1 := p(a, 0)
		b.edge(matSteel.hi, hl, 0, x0, y0, 0.1, x0, y0, eave, x1, y1, ridge)
	}
	b.edge(matSteel.hi, hl, 0, lx, ly, eave, fx, fy, eave, rx, ry, eave)
	b.edge(matSteel.hi, hl, 0, tx, ty, ridge, tx, ty, 0.1)
	b.edge(b.acc.mid, 0.05, 1.4, bx, by, ridge, tx, ty, ridge)
	b.loop(inkLine, b.ow(), 1, lx, ly, 0.1, fx, fy, 0.1, rx, ry, 0.1, rx, ry, eave, tx, ty, ridge, bx, by, ridge, lx, ly, eave)
	if b.lit && !b.tiny() {
		b.glowAt(-0.1, 0.2, 0.3, 0.5, alpha(gfx.RGB(0xF4FFD0), 0.16))
	}
	b.hit(-0.2, 0.3, 0.2)
	b.hit(0.35, 0.3, 0.2)
	b.hit(-0.1, 0.1, 0.42)
}

func greenhouseLive(b *bpen) {
	if !b.lit {
		return
	}
	// Grow-lamps breathing.
	v := pulse(b.t, 5, 0, 1)
	b.glowAt(-0.08, 0.16, 0.32, 0.6, alpha(gfx.RGB(0xC8F5A0), 0.05+0.11*v))
}

// ---------------------------------------------------------------------------
// Commons: a half-round hall in three stepped tiers behind a tall flat
// front, its rim hung with lanterns.
// ---------------------------------------------------------------------------

// The hall's flat front runs along the diagonal A through commonsC; the
// round of the hall lies behind it.
var commonsC = [2]float32{-0.13, 0.13}

// halfRound returns the ground outline of a half disc of radius r whose
// straight side runs along A through (cx, cy), the round on the far side.
func halfRound(dst []float32, cx, cy, r float32, n int) []float32 {
	for i := 0; i <= n; i++ {
		a := 5*pi/4 + pi*float32(i)/float32(n)
		dst = append(dst, cx+r*cosf(a), cy+r*sinf(a))
	}
	return dst
}

// commonsLamp returns where the i-th of the six lanterns hangs.
func commonsLamp(i int) (x, y, h float32) {
	a := 5*pi/4 + pi*(float32(i)+0.5)/6
	return commonsC[0] + 0.6*cosf(a), commonsC[1] + 0.6*sinf(a), 0.3
}

func commonsVol(b *bpen) {
	b.shadowG(0.04, 0, 0.82, 0.78)
	cx, cy := commonsC[0], commonsC[1]
	var buf [40]float32
	const k = 0.7071
	tier := func(m mat3, r, h0, h1 float32) {
		b.prism(m, h0, h1, halfRound(buf[:0], cx, cy, r, 12)...)
	}
	tier(matCream, 0.66, 0, 0.27)
	// The front: a dark plinth, tall lit windows and the doors.
	x0, y0 := cx-0.66*k, cy-0.66*k
	x1, y1 := cx+0.66*k, cy+0.66*k
	b.wallQ(alpha(matCream.lo, 0.7), x0, y0, x1, y1, 0, 1, 0, 0.04)
	n := 6
	if b.tiny() {
		n = 4
	}
	for i := 0; i < n; i++ {
		u := 0.08 + 0.84*float32(i)/float32(n)
		w := 0.84 / float32(n) * 0.5
		b.wallQ(colWindowOff, x0, y0, x1, y1, u-0.008, u+w+0.008, 0.075, 0.215)
		b.wallQ(b.lamp(colWindow), x0, y0, x1, y1, u, u+w, 0.085, 0.205)
	}
	// Lanterns round the rim of the lowest tier.
	for i := 0; i < 6; i++ {
		x, y, h := commonsLamp(i)
		b.dotAt(x, y, h, 0.05, alpha(inkLine, 0.8))
		b.lampAt(x, y, h, 0.032, gfx.RGB(0xFFB54A))
	}
	tier(matCream, 0.46, 0.27, 0.43)
	xa, ya := cx-0.46*k, cy-0.46*k
	xb, yb := cx+0.46*k, cy+0.46*k
	b.wallQ(colWindowOff, xa, ya, xb, yb, 0.12, 0.88, 0.31, 0.39)
	b.wallQ(b.lamp(colWindow), xa, ya, xb, yb, 0.15, 0.85, 0.32, 0.38)
	tier(b.acc, 0.26, 0.43, 0.54)
	// The entrance: tall lit doors under an awning in the owner's colour.
	b.wallQ(inkLine, x0, y0, x1, y1, 0.38, 0.62, 0, 0.2)
	b.wallQ(b.lamp(colWindow), x0, y0, x1, y1, 0.4, 0.49, 0, 0.185)
	b.wallQ(b.lamp(colWindow), x0, y0, x1, y1, 0.51, 0.6, 0, 0.185)
	{
		const out, hi, lo = 0.13, 0.27, 0.19
		wx0, wy0 := lerpf(x0, x1, 0.3), lerpf(y0, y1, 0.3)
		wx1, wy1 := lerpf(x0, x1, 0.7), lerpf(y0, y1, 0.7)
		b.face(b.acc.lo, wx1, wy1, hi, wx1-out, wy1+out, lo, wx1, wy1, lo)
		b.face(b.acc.hi, wx0, wy0, hi, wx1, wy1, hi, wx1-out, wy1+out, lo, wx0-out, wy0+out, lo)
		b.face(b.acc.mid, wx0-out, wy0+out, lo, wx1-out, wy1+out, lo, wx1-out, wy1+out, lo-0.03, wx0-out, wy0+out, lo-0.03)
		b.loop(inkLine, 0.03, 1, wx0, wy0, hi, wx1, wy1, hi, wx1-out, wy1+out, lo, wx1-out, wy1+out, lo-0.03, wx0-out, wy0+out, lo-0.03, wx0-out, wy0+out, lo)
	}
	if b.lit && !b.tiny() {
		b.glowAt(cx-0.1, cy+0.1, 0.14, 0.5, alpha(colWindow, 0.14))
	}
	b.hit(-0.3, 0.14, 0.14)
	b.hit(0.2, 0.5, 0.14)
	b.hit(0.1, -0.1, 0.3)
}

func commonsLive(b *bpen) {
	if !b.lit {
		return
	}
	// One lantern after another flares a little.
	i := int(wrap(b.t*1.5, 6)) % 6
	x, y, h := commonsLamp(i)
	b.glowAt(x, y, h, 0.2, alpha(colLamp, 0.7))
	b.dotAt(x, y, h, 0.04, gfx.RGB(0xFFF0C8))
}
