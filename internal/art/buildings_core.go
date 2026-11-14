package art

import (
	"math"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

func init() {
	buildingArts = [model.NumBuildingKinds]buildingArt{
		model.Keystone:       {keystoneBody, keystoneMotion},
		model.Conduit:        {conduitBody, conduitMotion},
		model.Silo:           {siloBody, siloMotion},
		model.SurveyMast:     {surveyMastBody, surveyMastMotion},
		model.Sunvane:        {sunvaneBody, sunvaneMotion},
		model.ThermalStack:   {thermalStackBody, thermalStackMotion},
		model.LumenCore:      {lumenCoreBody, lumenCoreMotion},
		model.Habitat:        {habitatBody, habitatMotion},
		model.Aerator:        {aeratorBody, aeratorMotion},
		model.Greenhouse:     {greenhouseBody, greenhouseMotion},
		model.Commons:        {commonsBody, commonsMotion},
		model.BoreRig:        {boreRigBody, boreRigMotion},
		model.Smelter:        {smelterBody, smelterMotion},
		model.Cracker:        {crackerBody, crackerMotion},
		model.Fabricator:     {fabricatorBody, fabricatorMotion},
		model.Laboratory:     {laboratoryBody, laboratoryMotion},
		model.LaunchCradle:   {launchCradleBody, launchCradleMotion},
		model.DockRing:       {dockRingBody, dockRingMotion},
		model.ExchangeDock:   {exchangeDockBody, exchangeDockMotion},
		model.FarseerArray:   {farseerBody, farseerMotion},
		model.MenderHub:      {menderHubBody, menderHubMotion},
		model.FlakBattery:    {flakBody, flakMotion},
		model.LanceTower:     {lanceBody, lanceMotion},
		model.AegisProjector: {aegisBody, aegisMotion},
		model.MassDriver:     {massDriverBody, massDriverMotion},
	}
}

// More materials, used by a few kinds each.
var (
	matArmor = mat(0xB3BCC9, 0x7C8696, 0x4B5462) // armour plate
	matPlate = mat(0xC3CAD5, 0x8D97A6, 0x58616F) // heavy industry
	matFuel  = mat(0xF7B083, 0xEE7B45, 0x9A4622) // fuel tanks
	matIce   = mat(0xE2F5FC, 0x9FD8EE, 0x5A93AB) // air and ice tanks
	matLeaf  = mat(0x9ED3A0, 0x57A070, 0x2E6B4E) // greenhouse glazing
	matPad   = mat(0x77818F, 0x464F5E, 0x2A303C) // landing pads
)

// frame returns b with its frame turned by a radians.
func (b *bpen) frame(a float32) *bpen {
	c := *b
	c.pen = b.pen.rotated(a)
	return &c
}

// hexXY returns the corners of a regular hexagon as x, y pairs; a0 is the
// angle of the first corner (0 for flat-top, -π/2 for pointy-top).
func hexXY(cx, cy, r, a0 float32) [12]float32 {
	var xy [12]float32
	for i := 0; i < 6; i++ {
		a := a0 + float32(i)*tau/6
		xy[2*i], xy[2*i+1] = cx+r*cosf(a), cy+r*sinf(a)
	}
	return xy
}

// ngonXY appends the corners of a regular n-gon as x, y pairs.
func ngonXY(dst []float32, cx, cy, r float32, n int, a0 float32) []float32 {
	for i := 0; i < n; i++ {
		a := a0 + float32(i)*tau/float32(n)
		dst = append(dst, cx+r*cosf(a), cy+r*sinf(a))
	}
	return dst
}

// facetTone picks the tone of a facet whose outward normal points along the
// screen angle a.
func facetTone(m mat3, a float32) gfx.Color {
	d := cosf(a)*lightX + sinf(a)*lightY
	switch {
	case d > 0.35:
		return m.hi
	case d < -0.35:
		return m.lo
	}
	return m.mid
}

// pyramid draws a faceted n-sided roof rising to a point at (cx, cy).
func (b *bpen) pyramid(m mat3, cx, cy, r float32, n int, a0 float32) {
	for i := 0; i < n; i++ {
		a := a0 + float32(i)*tau/float32(n)
		a1 := a + tau/float32(n)
		b.poly(facetTone(m, (a+a1)/2+b.ang), cx, cy, cx+r*cosf(a), cy+r*sinf(a), cx+r*cosf(a1), cy+r*sinf(a1))
	}
	b.ngonLine(cx, cy, r, n, a0, b.ow(), 1, inkLine)
}

// strut draws a beam between two points: a dark casing with a lighter core.
func (b *bpen) strut(x0, y0, x1, y1, width float32, c gfx.Color) {
	b.line(x0, y0, x1, y1, width+2*b.ow(), 2.4, inkLine)
	b.line(x0, y0, x1, y1, width, 1, c)
}

// ---------------------------------------------------------------------------
// Keystone: a hexagonal citadel with a faceted accent roof, three landing
// lugs reaching for the tile corners and a beacon at its peak.
// ---------------------------------------------------------------------------

func keystoneBody(b *bpen) {
	for i := 0; i < 3; i++ {
		f := b.frame(-pi/2 + float32(i)*tau/3)
		f.slab(matIron, 0.05, 0.56, -0.19, 0.93, -0.11, 0.93, 0.11, 0.56, 0.19)
		if !b.tiny() {
			f.poly(b.acc.mid, 0.78, -0.07, 0.88, -0.06, 0.88, 0.06, 0.78, 0.07)
		}
	}
	lower := hexXY(0, 0, 0.78, 0)
	b.slab(matSteel, 0.11, lower[:]...)
	if !b.tiny() {
		for i := 0; i < 6; i++ {
			a := float32(i) * tau / 6
			b.line(0.54*cosf(a), 0.54*sinf(a), 0.73*cosf(a), 0.73*sinf(a), 0.03, 1, alpha(matSteel.lo, 0.9))
		}
	}
	if b.big() {
		for i := 0; i < 6; i++ {
			a := float32(i)*tau/6 + tau/12
			nx, ny := cosf(a), sinf(a)
			for _, o := range [2]float32{-0.13, 0.13} {
				x, y := 0.585*nx-o*ny, 0.585*ny+o*nx
				b.lampDot(x, y, 0.032, colWindow)
			}
		}
	}
	// Upper tier: the accent roof.
	upper := hexXY(0, 0, 0.5, 0)
	b.shadowPoly(0.09, upper[:]...)
	b.pyramid(b.acc, 0, 0, 0.5, 6, 0)
	b.drum(matDark, 0, 0, 0, 0.17)
	b.ring(0, 0, 0.115, 0.035, 1, matCream.mid)
}

func keystoneMotion(b *bpen) {
	// A double flash every 2.4 s.
	ph := float32(math.Mod(float64(b.t), 2.4))
	on := ph < 0.14 || (ph > 0.32 && ph < 0.46)
	if !b.lit {
		b.circle(0, 0, 0.06, colWindowOff)
		return
	}
	if on {
		b.glow(0, 0, 0.62, alpha(gfx.RGB(0xFFE7B0), 0.75))
		b.circle(0, 0, 0.075, gfx.RGB(0xFFF6DC))
	} else {
		b.glow(0, 0, 0.3, alpha(colLamp, 0.3))
		b.circle(0, 0, 0.06, colLamp)
	}
}

// ---------------------------------------------------------------------------
// Conduit: a low junction plate with trenches running to the six
// neighbouring tiles.
// ---------------------------------------------------------------------------

func conduitBody(b *bpen) {
	for i := 0; i < 6; i++ {
		a := float32(i) * tau / 6
		c, s := cosf(a), sinf(a)
		b.line(0.2*c, 0.2*s, 0.868*c, 0.868*s, 0.21, 3, alpha(inkLine, 0.9))
		b.line(0.2*c, 0.2*s, 0.868*c, 0.868*s, 0.12, 1.6, matDark.mid)
		b.line(0.2*c, 0.2*s, 0.868*c, 0.868*s, 0.04, 0.9, matIron.hi)
	}
	node := hexXY(0, 0, 0.34, -pi/2)
	b.slab(matIron, 0.04, node[:]...)
	b.ngonLine(0, 0, 0.2, 6, -pi/2, 0.04, 1, b.acc.mid)
}

func conduitMotion(b *bpen) {
	if !b.lit {
		b.circle(0, 0, 0.08, colWindowOff)
		return
	}
	v := pulse(b.t, 3.2, 0.45, 1)
	b.circle(0, 0, 0.085, gfx.Lerp(b.acc.lo, b.acc.hi, v))
}

// ---------------------------------------------------------------------------
// Silo: three pressure lids in a trefoil around a manifold.
// ---------------------------------------------------------------------------

func siloBody(b *bpen) {
	mf := hexXY(0, 0, 0.3, 0)
	b.slab(matIron, 0.04, mf[:]...)
	for i := 0; i < 3; i++ {
		a := -pi/2 + float32(i)*tau/3
		x, y := 0.4*cosf(a), 0.4*sinf(a)
		b.drum(matSteel, 0.1, x, y, 0.36)
		b.ring(x, y, 0.235, 0.075, 1.2, b.acc.mid)
		b.drum(matIron, 0, x, y, 0.1)
		if b.big() {
			for k := 0; k < 4; k++ {
				ka := pi/4 + float32(k)*tau/4
				b.line(x+0.3*cosf(ka), y+0.3*sinf(ka), x+0.335*cosf(ka), y+0.335*sinf(ka), 0.035, 1, matSteel.lo)
			}
		}
	}
	b.drum(matDark, 0, 0, 0, 0.1)
}

func siloMotion(b *bpen) {
	// Three level lamps taking turns.
	for i := 0; i < 3; i++ {
		a := -pi/2 + float32(i)*tau/3
		x, y := 0.4*cosf(a), 0.4*sinf(a)
		if !b.lit {
			b.circle(x, y, 0.045, colWindowOff)
			continue
		}
		v := pulse(b.t+float32(i)*1.3, 3.9, 0.25, 1)
		b.circle(x, y, 0.045, gfx.Lerp(gfx.RGB(0x3E5A3C), gfx.RGB(0xA8F0A0), v))
	}
}

// ---------------------------------------------------------------------------
// Survey Mast: a tall mast on a tripod inside its sounding circle, with a
// sensor arm sweeping round and pings running out through the rock.
// ---------------------------------------------------------------------------

func surveyMastBody(b *bpen) {
	// The sounding circle scratched into the ground.
	for i := 0; i < 12; i++ {
		a := float32(i) * tau / 12
		b.arc(0, 0, 0.7, a+0.07, a+tau/12-0.07, 0.035, 1, alpha(matCream.hi, 0.55))
	}
	for i := 0; i < 3; i++ {
		a := pi/2 + float32(i)*tau/3
		x, y := 0.56*cosf(a), 0.56*sinf(a)
		b.strut(0, 0, x, y, 0.07, matIron.hi)
		b.drum(matDark, 0.03, x, y, 0.095)
	}
	// The mast is tall: its shadow is long.
	b.line(0.05, 0.07, 0.42, 0.56, 0.09, 1.2, inkShadow)
	b.drum(matSteel, 0, 0, 0, 0.2)
	b.ring(0, 0, 0.115, 0.05, 1, b.acc.mid)
}

func surveyMastMotion(b *bpen) {
	a := b.t * 1.1
	c, s := cosf(a), sinf(a)
	if b.lit {
		f := float32(math.Mod(float64(b.t), 2.6)) / 2.6
		b.ring(0, 0, 0.2+0.62*f, 0.035, 1, alpha(gfx.RGB(0xBFE9FF), 0.6*(1-f)))
	}
	b.strut(0.1*c, 0.1*s, 0.42*c, 0.42*s, 0.05, matSteel.hi)
	// The sensor head: a short bar across the end of the arm.
	x, y := 0.44*c, 0.44*s
	b.line(x+0.13*s, y-0.13*c, x-0.13*s, y+0.13*c, 0.13, 2.4, inkLine)
	b.line(x+0.11*s, y-0.11*c, x-0.11*s, y+0.11*c, 0.07, 1.2, matCream.hi)
	b.circle(0, 0, 0.055, b.lamp(gfx.RGB(0xBFE9FF)))
}

// ---------------------------------------------------------------------------
// Sunvane: four collector vanes set as a pinwheel around a hub.
// ---------------------------------------------------------------------------

// sunvaneRects are the four vanes: x, y, w, h.
var sunvaneRects = [4][4]float32{
	{0.09, -0.42, 0.66, 0.33},
	{0.09, 0.09, 0.33, 0.66},
	{-0.75, 0.09, 0.66, 0.33},
	{-0.42, -0.75, 0.33, 0.66},
}

func sunvaneBody(b *bpen) {
	for i, r := range sunvaneRects {
		x, y, w, h := r[0], r[1], r[2], r[3]
		b.block(matSolar, 0.05, x, y, w, h)
		// Cell joints.
		long := w > h
		jc := alpha(inkLine, 0.55)
		if long {
			if !b.tiny() {
				b.rect(x+w/3-0.012, y+0.04, 0.024, h-0.08, jc)
				b.rect(x+2*w/3-0.012, y+0.04, 0.024, h-0.08, jc)
			}
		} else if !b.tiny() {
			b.rect(x+0.04, y+h/3-0.012, w-0.08, 0.024, jc)
			b.rect(x+0.04, y+2*h/3-0.012, w-0.08, 0.024, jc)
		}
		// Accent on the outer end.
		const e = 0.075
		switch i {
		case 0:
			b.rect(x+w-e-0.03, y+0.035, e, h-0.07, b.acc.mid)
		case 1:
			b.rect(x+0.035, y+h-e-0.03, w-0.07, e, b.acc.mid)
		case 2:
			b.rect(x+0.03, y+0.035, e, h-0.07, b.acc.mid)
		case 3:
			b.rect(x+0.035, y+0.03, w-0.07, e, b.acc.mid)
		}
	}
	b.drum(matIron, 0.05, 0, 0, 0.17)
	b.circle(0, 0, 0.08, b.acc.hi)
}

func sunvaneMotion(b *bpen) {
	if !b.lit {
		return
	}
	// The red sun's reflection slides along every vane in step.
	f := float32(math.Mod(float64(b.t), 6)) / 6
	if f > 0.5 {
		return
	}
	u := f * 2
	a := 0.55 * sinf(u*pi)
	c := gfx.Color{R: 0xFF, G: 0xD9, B: 0xC2, A: uint8(255 * a)}
	for i, r := range sunvaneRects {
		x, y, w, h := r[0]+0.04, r[1]+0.04, r[2]-0.08, r[3]-0.08
		const band = 0.07
		switch i {
		case 0, 2:
			px := x + (w-band)*u
			if i == 2 {
				px = x + (w-band)*(1-u)
			}
			b.rect(px, y, band, h, c)
		default:
			py := y + (h-band)*u
			if i == 3 {
				py = y + (h-band)*(1-u)
			}
			b.rect(x, py, w, band, c)
		}
	}
}

// ---------------------------------------------------------------------------
// Thermal Stack: a generator house fed from a fuel tank, with two burner
// stacks glowing and smoking at one end.
// ---------------------------------------------------------------------------

var thermalStacks = [2][2]float32{{0.44, -0.21}, {0.44, 0.25}}

func thermalStackBody(b *bpen) {
	b.pill(matFuel, 0.07, -0.52, -0.56, -0.12, -0.56, 0.13)
	b.strut(-0.3, -0.46, -0.3, -0.34, 0.06, matCopper.mid)
	b.block(matIron, 0.05, 0.14, -0.46, 0.58, 0.92)
	b.block(matSteel, 0.13, -0.66, -0.36, 0.84, 0.8)
	if !b.tiny() {
		for i := 0; i < 4; i++ {
			y := -0.22 + float32(i)*0.15
			b.rect(-0.44, y, 0.54, 0.03, matSteel.lo)
		}
	}
	b.rect(-0.6, -0.3, 0.1, 0.68, b.acc.mid)
	for _, s := range thermalStacks {
		b.drum(matIron, 0.3, s[0], s[1], 0.2)
		b.circle(s[0], s[1], 0.125, inkLine)
	}
}

func thermalStackMotion(b *bpen) {
	for i, s := range thermalStacks {
		if !b.lit {
			b.circle(s[0], s[1], 0.09, gfx.RGB(0x15171D))
			continue
		}
		v := pulse(b.t+float32(i)*0.7, 1.7, 0, 1)
		b.glow(s[0], s[1], 0.42, alpha(colHot, 0.45+0.25*v))
		b.circle(s[0], s[1], 0.1, gfx.Lerp(colHot, colHotCore, v))
		// Exhaust drifting up and to the right, thinning as it goes.
		for k := 0; k < 3; k++ {
			d := float32(math.Mod(float64(b.t*0.32+float32(k)/3+float32(i)*0.17), 1))
			b.circle(s[0]+0.3*d, s[1]-0.62*d, 0.07+0.1*d, gfx.Color{R: 0xC8, G: 0xB8, B: 0xB0, A: uint8(70 * (1 - d))})
		}
	}
}

// ---------------------------------------------------------------------------
// Lumen Core: a round containment vessel ringed by six pylons, with a
// violet crystal burning in its well.
// ---------------------------------------------------------------------------

func lumenCoreBody(b *bpen) {
	for i := 0; i < 6; i++ {
		f := b.frame(float32(i) * tau / 6)
		f.slab(matSteel, 0.06, 0.6, -0.1, 0.85, -0.075, 0.85, 0.075, 0.6, 0.1)
	}
	b.drum(matIron, 0.12, 0, 0, 0.7)
	b.ring(0, 0, 0.575, 0.06, 1.2, b.acc.mid)
	b.circle(0, 0, 0.47, inkLine)
	b.circle(0, 0, 0.44, matDark.lo)
	if !b.tiny() {
		for i := 0; i < 6; i++ {
			a := float32(i)*tau/6 + tau/12
			b.line(0.22*cosf(a), 0.22*sinf(a), 0.43*cosf(a), 0.43*sinf(a), 0.04, 1, alpha(b.lamp(colLumen), 0.7))
		}
	}
	m := mat3{colLumenHi, colLumen, gfx.RGB(0x6B4FB8)}
	if !b.lit {
		m = mat(0x6F6690, 0x4A416B, 0x2C2744)
	}
	b.pyramid(m, 0, 0, 0.25, 6, -pi/2)
}

func lumenCoreMotion(b *bpen) {
	if !b.lit {
		return
	}
	v := pulse(b.t, 2.2, 0, 1)
	b.glow(0, 0, 0.85, alpha(colLumen, 0.3+0.3*v))
	b.glowCore(0, 0, 0.5, alpha(colLumenHi, 0.5+0.4*v))
	a := b.t * 0.9
	for i := 0; i < 2; i++ {
		aa := a + float32(i)*pi
		b.arc(0, 0, 0.35, aa, aa+1.0, 0.045, 1.2, alpha(colLumenHi, 0.85))
	}
}

// ---------------------------------------------------------------------------
// Habitat: two pressure cabins lying side by side, rows of lit windows.
// ---------------------------------------------------------------------------

func habitatBody(b *bpen) {
	b.block(matIron, 0.05, -0.15, -0.14, 0.3, 0.28)
	for i, y := range [2]float32{-0.33, 0.33} {
		b.pill(matCream, 0.1, -0.42, y, 0.42, y, 0.25)
		// Accent collar, at opposite ends of the two cabins.
		x := float32(-0.5)
		if i == 1 {
			x = 0.38
		}
		b.quad(x, y-0.225, 0.12, 0.45, b.acc.mid)
		b.quad(x, y-0.225, 0.12, 0.09, b.acc.hi)
		b.quad(x, y+0.135, 0.12, 0.09, b.acc.lo)
		// Windows.
		n, w := 4, float32(0.085)
		if b.tiny() {
			n, w = 3, 0.11
		}
		x0 := float32(-0.3)
		if i == 1 {
			x0 = -0.42
		}
		for k := 0; k < n; k++ {
			wx := x0 + (float32(k)+0.5)*0.72/float32(n)
			b.window(wx, y, w, 0.1, colWindow)
		}
	}
}

func habitatMotion(b *bpen) {
	// The airlock lamp between the cabins.
	if !b.lit {
		b.circle(0, 0, 0.05, colWindowOff)
		return
	}
	v := pulse(b.t, 4, 0.5, 1)
	b.circle(0, 0, 0.05, gfx.Lerp(gfx.RGB(0x7A5A2A), colLamp, v))
}

// ---------------------------------------------------------------------------
// Aerator: a chamfered plant with one great fan and two air tanks.
// ---------------------------------------------------------------------------

func aeratorBody(b *bpen) {
	b.drum(matIce, 0.1, 0.6, -0.26, 0.15)
	b.drum(matIce, 0.1, 0.6, 0.1, 0.15)
	const x0, x1, y0, y1, c = -0.7, 0.42, -0.56, 0.56, 0.17
	b.slab(matSteel, 0.1,
		x0+c, y0, x1-c, y0, x1, y0+c, x1, y1-c, x1-c, y1, x0+c, y1, x0, y1-c, x0, y0+c)
	b.rect(-0.5, 0.415, 0.72, 0.075, b.acc.mid)
	b.circle(-0.14, -0.04, 0.42, inkLine)
	b.circle(-0.14, -0.04, 0.385, matDark.lo)
	b.ring(-0.14, -0.04, 0.385, 0.04, 1, matIron.mid)
}

func aeratorMotion(b *bpen) {
	const cx, cy = -0.14, -0.04
	a := b.t * 4.2
	for i := 0; i < 4; i++ {
		aa := a + float32(i)*tau/4
		c, s := cosf(aa), sinf(aa)
		// A blade: narrow at the hub, broad at the tip.
		x0, y0 := cx+0.07*c, cy+0.07*s
		x1, y1 := cx+0.33*c-0.13*s, cy+0.33*s+0.13*c
		x2, y2 := cx+0.35*c+0.05*s, cy+0.35*s-0.05*c
		b.poly(matSteel.mid, x0, y0, x1, y1, x2, y2)
	}
	b.circle(cx, cy, 0.1, inkLine)
	b.circle(cx, cy, 0.075, b.acc.mid)
}

// ---------------------------------------------------------------------------
// Greenhouse: a long glasshouse with a ridge, green under the panes.
// ---------------------------------------------------------------------------

func greenhouseBody(b *bpen) {
	b.slab(matSteel, 0.08,
		-0.74, -0.2, -0.54, -0.42, 0.54, -0.42, 0.74, -0.2, 0.74, 0.2, 0.54, 0.42, -0.54, 0.42, -0.74, 0.2)
	m := matLeaf
	if !b.lit {
		m = mat(0x587A66, 0x35574A, 0x20372F)
	}
	// Two glazed slopes either side of the ridge.
	b.poly(m.mid, -0.66, -0.17, -0.5, -0.345, 0.5, -0.345, 0.66, -0.17, 0.66, -0.02, -0.66, -0.02)
	b.poly(m.lo, -0.66, 0.02, 0.66, 0.02, 0.66, 0.17, 0.5, 0.345, -0.5, 0.345, -0.66, 0.17)
	if !b.tiny() {
		// Rows of plants show through.
		for i := 0; i < 7; i++ {
			x := -0.48 + float32(i)*0.16
			b.rect(x-0.035, -0.28, 0.07, 0.2, alpha(m.hi, 0.75))
			b.rect(x-0.035, 0.08, 0.07, 0.2, alpha(m.mid, 0.9))
		}
	}
	for _, x := range [3]float32{-0.33, 0, 0.33} {
		b.rect(x-0.02, -0.36, 0.04, 0.72, matSteel.hi)
	}
	b.rect(-0.7, -0.035, 1.4, 0.07, b.acc.mid)
	b.rect(-0.7, -0.035, 1.4, 0.025, b.acc.hi)
	if b.lit && !b.tiny() {
		for _, x := range [4]float32{-0.5, -0.165, 0.165, 0.5} {
			b.glow(x, -0.18, 0.16, alpha(gfx.RGB(0xF4FFD0), 0.35))
			b.glow(x, 0.18, 0.16, alpha(gfx.RGB(0xF4FFD0), 0.3))
		}
	}
}

func greenhouseMotion(b *bpen) {
	if !b.lit {
		return
	}
	// Grow-lamps breathing.
	v := pulse(b.t, 5, 0, 1)
	b.glow(0, 0, 0.7, alpha(gfx.RGB(0xC8F5A0), 0.10+0.14*v))
}

// ---------------------------------------------------------------------------
// Commons: a fan-shaped hall of stepped tiers around a stage, its rim hung
// with lanterns.
// ---------------------------------------------------------------------------

const commonsCY = 0.38

// fanXY returns a half disc (flat side down) as x, y pairs.
func fanXY(dst []float32, cx, cy, r float32, n int) []float32 {
	for i := 0; i <= n; i++ {
		a := pi + pi*float32(i)/float32(n)
		dst = append(dst, cx+r*cosf(a), cy+r*sinf(a))
	}
	return dst
}

func commonsBody(b *bpen) {
	b.block(matIron, 0.05, -0.24, 0.34, 0.48, 0.28)
	b.window(0, 0.54, 0.2, 0.08, colWindow)
	var buf [40]float32
	outer := fanXY(buf[:0], 0, commonsCY, 0.78, 16)
	b.slab(matCream, 0.12, outer...)
	mid := fanXY(buf[:0], 0, commonsCY-0.02, 0.56, 14)
	b.poly(matCream.hi, mid...)
	b.outline(alpha(matCream.lo, 0.9), 0.03, 1, mid...)
	if !b.tiny() {
		for i := 1; i < 6; i++ {
			a := pi + pi*float32(i)/6
			c, s := cosf(a), sinf(a)
			b.line(0.36*c, commonsCY+0.36*s, 0.74*c, commonsCY+0.74*s, 0.03, 1, alpha(matCream.lo, 0.8))
		}
	}
	stage := fanXY(buf[:0], 0, commonsCY-0.02, 0.3, 10)
	b.poly(b.acc.mid, stage...)
	b.outline(inkLine, b.ow(), 1, stage...)
	b.arc(0, commonsCY-0.02, 0.3-b.ow()/2-b.bw()/2, litAngle-0.8, litAngle+0.6, b.bw(), 1, b.acc.hi)
	for i := 0; i < 6; i++ {
		x, y := commonsLantern(i)
		b.lampDot(x, y, 0.04, colLamp)
	}
}

func commonsLantern(i int) (x, y float32) {
	a := pi + pi*(float32(i)+0.5)/6
	return 0.66 * cosf(a), commonsCY + 0.66*sinf(a)
}

func commonsMotion(b *bpen) {
	if !b.lit {
		return
	}
	// One lantern after another flares a little.
	i := int(math.Mod(float64(b.t*1.5), 6))
	x, y := commonsLantern(i)
	b.glow(x, y, 0.22, alpha(colLamp, 0.7))
	b.circle(x, y, 0.05, gfx.RGB(0xFFF0C8))
}
