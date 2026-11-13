// Package emblem draws the Drifthold logo mark entirely in code.
//
// The emblem is a pointy-top hexagonal badge in ink-dark with a thin amber
// rim. Inside it an amber keystone - a wedge-shaped block lit from the upper
// left - rests on a small dark rock, while a thin cream orbit arc carries a
// single cream dot (a drifting rock) around it.
//
// The package is pure Go, has no dependency on the game engine and is fully
// deterministic: the same size always yields the same pixels. It serves the
// game itself (window icon, title screen) as well as tools/winres, which bakes
// the emblem into the Windows executable's icon resources.
package emblem

import (
	"image"
	"math"
)

// MinSize and MaxSize bound the pixel sizes accepted by Render.
const (
	MinSize = 16
	MaxSize = 1024
)

// The Drifthold palette ("lantern-lit drafting table") and the tones derived
// from it for facets and shading.
var (
	inkDark = hex(0x12151F) // badge field (at its centre)
	amber   = hex(0xE8A33D) // rim and keystone
	cream   = hex(0xE9E2D0) // orbit and drifting rock

	inkTop      = hex(0x1F2538) // field, top vertex
	inkBottom   = hex(0x090B11) // field, bottom vertex
	amberLight  = hex(0xF4C063) // lit rim facets, keystone face highlight
	amberPale   = hex(0xFBDD9C) // keystone top facet
	amberDeep   = hex(0xB06E1E) // shaded rim facets
	amberShadow = hex(0x99591A) // keystone right facet
	rockLit     = hex(0x46506B) // rock, lit upper facets
	rockBody    = hex(0x2A3145) // rock, body
	rockShade   = hex(0x1A1F2E) // rock, underside
)

// Sizes returns the icon sizes shipped in the Windows icon, in ascending
// order: 16, 20, 24, 32, 40, 48, 64, 128 and 256 pixels.
func Sizes() []int {
	return []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
}

// Render draws the Drifthold emblem into a square RGBA image of the given
// pixel size, anti-aliased on a transparent background. Sizes outside
// MinSize..MaxSize are clamped to that range.
//
// The drawing adapts to the size: below 32 pixels the orbit arc is dropped,
// outlines are thickened and edges are snapped to the pixel grid so that the
// mark stays legible in a title bar or task bar; from 64 pixels upwards extra
// facets and hairlines are added.
func Render(size int) *image.RGBA {
	size = clampInt(size, MinSize, MaxSize)
	l := newLayout(size)
	c := newCanvas(size)
	l.drawBadge(c)
	if l.orbit {
		l.drawOrbit(c)
	}
	l.drawKeystone(c)
	l.drawRock(c)
	if l.dotRadius > 0 {
		c.fill(solid(cream, 1), disc(l.dot, l.dotRadius))
	}
	return c.image()
}

// layout holds the size-dependent geometry of the emblem, in unit design
// space (see pt).
type layout struct {
	n    int  // pixel size
	fine bool // large enough for hairlines and extra facets (>= 64 px)
	tiny bool // simplified small-icon drawing (< 32 px)

	// Badge.
	radius  float64 // centre to top/bottom vertex
	rim     float64 // rim thickness
	corner  float64 // corner chamfer length
	shading float64 // strength of the per-facet rim shading, 0..1

	// Keystone: a trapezoidal front face extruded along depth.
	keyCentre, keyTop, keyBottom float64
	keyTopHalf, keyBottomHalf    float64
	keyDepth                     pt

	// Rock. Small icons use the plain outline rock; larger ones build a
	// faceted shard from its top edge (left to right), right flank (down to
	// the tip), left flank (tip back up) and the ridge (right to left) that
	// separates the lit top from the flanks. rockRidge[rockKnee] is where
	// the two flanks meet the ridge.
	rock                         []pt
	rockTop, rockRight, rockLeft []pt
	rockRidge                    []pt
	rockTip                      pt
	rockKnee                     int
	rockShadow                   []pt // the keystone's shadow on the rock's top

	// Orbit and drifting rock.
	orbit      bool
	orbitWidth float64
	dot        pt
	dotRadius  float64
}

// px converts a length in pixels to design units.
func (l *layout) px(v float64) float64 { return v / float64(l.n) }

// snap rounds a design coordinate to the nearest pixel boundary.
func (l *layout) snap(v float64) float64 {
	n := float64(l.n)
	return math.Round(v*n) / n
}

// newLayout works out the geometry for an n pixel rendering. There are three
// levels of detail: below 32 pixels a simplified, pixel-snapped drawing with a
// heavier rim and no orbit; from 32 pixels the full composition; and from 64
// pixels (fine) additional hairlines, highlights and shadows.
func newLayout(n int) *layout {
	l := &layout{n: n, fine: n >= 64, tiny: n < 32}
	switch {
	case l.tiny:
		l.radius = 0.5
		l.rim = math.Max(l.px(1.3), 0.074)
		l.corner = 0
		l.shading = 0.25

		l.keyCentre = 0.5 - l.px(0.5)
		l.keyTop = l.snap(0.285)
		l.keyBottom = l.snap(0.635)
		l.keyTopHalf = 0.19
		l.keyBottomHalf = 0.115
		l.keyDepth = pt{l.px(1.1), -l.px(1.0)}

		l.rock = []pt{
			{0.265, 0.668}, {0.335, l.keyBottom}, {0.69, l.keyBottom},
			{0.755, 0.662}, {0.665, 0.745}, {0.555, 0.835}, {0.40, 0.765},
		}
	default:
		l.radius = 0.492
		l.rim = 0.034
		l.corner = 0.016
		l.shading = 0.6
		if !l.fine {
			l.rim = 0.05
			l.corner = 0.012
			l.shading = 0.45
		}

		l.keyCentre = 0.482
		l.keyTop = 0.290
		l.keyBottom = 0.600
		l.keyTopHalf = 0.136
		l.keyBottomHalf = 0.084
		l.keyDepth = pt{0.056, -0.040}

		l.rockTop = []pt{
			{0.250, 0.652}, {0.290, 0.604}, {0.336, 0.560}, {0.372, 0.572},
			{0.418, 0.604}, {0.476, 0.592}, {0.560, 0.600}, {0.636, 0.556},
			{0.700, 0.580}, {0.752, 0.636},
		}
		l.rockRight = []pt{{0.724, 0.688}, {0.666, 0.724}, {0.624, 0.776}}
		l.rockTip = pt{0.546, 0.836}
		l.rockLeft = []pt{{0.480, 0.790}, {0.400, 0.764}, {0.330, 0.720}, {0.282, 0.690}}
		l.rockRidge = []pt{
			{0.672, 0.648}, {0.600, 0.632}, {0.520, 0.660}, {0.430, 0.644}, {0.340, 0.668},
		}
		l.rockKnee = 2
		l.rockShadow = []pt{{0.560, 0.600}, {0.636, 0.556}, {0.692, 0.590}, {0.630, 0.637}}

		l.orbit = true
		l.orbitWidth = math.Max(0.011, l.px(1.1))
		l.dotRadius = 0.030
		if !l.fine {
			l.orbitWidth = math.Max(0.020, l.px(1.3))
			l.dotRadius = 0.042
		}
	}
	return l
}

// hexagon returns the pointy-top badge outline, clockwise from the top
// vertex. Its flat left and right sides are snapped to pixel boundaries so
// that they render crisply at every size.
func (l *layout) hexagon() []pt {
	r := l.radius
	left := l.snap(0.5 - r*math.Sqrt(3)/2)
	right := 1 - left
	return []pt{
		{0.5, 0.5 - r}, {right, 0.5 - r/2}, {right, 0.5 + r/2},
		{0.5, 0.5 + r}, {left, 0.5 + r/2}, {left, 0.5 - r/2},
	}
}

// light is the direction towards the light source (upper left).
var light = pt{-0.55, -0.83}.unit()

// drawBadge paints the hexagonal field and its faceted amber rim.
func (l *layout) drawBadge(c *canvas) {
	hexagon := l.hexagon()
	outer := chamfer(hexagon, l.corner)
	inner := chamfer(inset(hexagon, l.rim), l.corner*0.6)

	// Rim: a base coat, then one flat tone per facet as if the rim were a
	// chamfered metal edge lit from the upper left.
	c.fill(solid(amber, 1), outer)
	for i := range outer {
		j := (i + 1) % len(outer)
		edge := outer[j].sub(outer[i]).unit()
		normal := pt{edge.y, -edge.x} // outward for a clockwise outline
		lit := normal.dot(light) * l.shading
		tone := mix(amber, amberLight, lit)
		if lit < 0 {
			tone = mix(amber, amberDeep, -lit)
		}
		c.fill(solid(tone, 1), []pt{outer[i], outer[j], inner[j], inner[i]})
	}

	// Field: ink-dark at the centre with a subtle vertical gradient,
	// lighter towards the top vertex and darker towards the bottom one.
	c.fill(func(_, y float64) (rgb, float64) {
		t := (y - 0.5) / l.radius
		if t < 0 {
			return mix(inkDark, inkTop, clamp01(-t)), 1
		}
		return mix(inkDark, inkBottom, clamp01(t)), 1
	}, inner)

	if l.fine {
		// A faint lantern glow behind the keystone and a hairline just
		// inside the rim, like a ruled border on a drafting sheet.
		c.fill(glow(pt{0.46, 0.40}, 0.40, amber, 0.10), inner)
		gap, hair := 0.022, 0.006
		ruleOuter := chamfer(inset(hexagon, l.rim+gap), l.corner*0.4)
		ruleInner := chamfer(inset(hexagon, l.rim+gap+hair), l.corner*0.4)
		// The inner outline is wound the other way so that it cuts a
		// hole, leaving only the hairline between the two.
		c.fill(solid(amber, 0.30), ruleOuter, reversed(ruleInner))
	}
}

// drawKeystone paints the wedge-shaped block: an amber front face, a lighter
// top facet and a darker right facet.
func (l *layout) drawKeystone(c *canvas) {
	tl := pt{l.keyCentre - l.keyTopHalf, l.keyTop}
	tr := pt{l.keyCentre + l.keyTopHalf, l.keyTop}
	br := pt{l.keyCentre + l.keyBottomHalf, l.keyBottom}
	bl := pt{l.keyCentre - l.keyBottomHalf, l.keyBottom}
	d := l.keyDepth

	// Base coat over the whole silhouette so that no background shows
	// through the seams between facets.
	c.fill(solid(amber, 1), []pt{tl, tl.add(d), tr.add(d), br.add(d), br, bl})
	c.fill(gradient(tl, amberLight, br, amber), []pt{tl, tr, br, bl})
	c.fill(solid(amberPale, 1), []pt{tl, tl.add(d), tr.add(d), tr})
	c.fill(gradient(tr, amberDeep, br.add(d), amberShadow), []pt{tr, tr.add(d), br.add(d), br})

	if l.fine {
		// Crisp highlight along the lit left edge of the front face.
		w := 0.007
		c.fill(solid(amberPale, 0.55), []pt{tl, tl.add(pt{w * 1.6, 0}), bl.add(pt{w * 1.6, 0}), bl})
	}
	if l.n >= 96 {
		// A chiselled groove following the outline of the front face: a
		// dark cut with a highlight on its far (lit) wall.
		face := []pt{tl, tr, br, bl}
		margin, cut := 0.030, 0.0065
		off := pt{cut * 0.75, cut * 0.75}
		groove := func(shift pt) [][]pt {
			a, b := inset(face, margin), reversed(inset(face, margin+cut))
			for i := range a {
				a[i], b[i] = a[i].add(shift), b[i].add(shift)
			}
			return [][]pt{a, b}
		}
		c.fill(solid(amberPale, 0.60), groove(off)...)
		c.fill(solid(amberShadow, 0.80), groove(pt{})...)
	}
}

// drawRock paints the irregular rock the keystone rests on. It is drawn
// after the keystone so that the block appears bedded into it.
func (l *layout) drawRock(c *canvas) {
	if l.tiny {
		c.fill(gradient(pt{0.5, 0.62}, rockLit, pt{0.5, 0.80}, rockBody), l.rock)
		return
	}
	first, last := l.rockTop[0], l.rockTop[len(l.rockTop)-1]
	knee := l.rockRidge[l.rockKnee]

	outline := join(l.rockTop, l.rockRight, []pt{l.rockTip}, l.rockLeft)
	c.fill(solid(rockBody, 1), outline)

	// Right flank, turned away from the light.
	right := join([]pt{last}, l.rockRight, []pt{l.rockTip, knee}, reversed(l.rockRidge[:l.rockKnee]))
	c.fill(gradient(knee, rockBody, l.rockTip, rockShade), right)
	c.fill(solid(rockShade, 0.55), right)

	// Left flank, in half light.
	left := join([]pt{knee, l.rockTip}, l.rockLeft, []pt{first}, reversed(l.rockRidge[l.rockKnee+1:]))
	c.fill(gradient(first, rockBody, l.rockTip, rockShade), left)

	// Lit top.
	top := join(l.rockTop, l.rockRidge)
	c.fill(gradient(first, rockLit, last, mix(rockLit, rockBody, 0.6)), top)

	if l.fine {
		// The keystone's shadow falling across the top, away from the
		// light, and a thin rim of light along the rock's upper edge.
		c.fill(solid(rockShade, 0.5), l.rockShadow)

		half := make([]float64, len(l.rockTop))
		for i := range half {
			half[i] = 0.0032
		}
		half[0], half[len(half)-1] = 0, 0
		edge := make([]pt, len(l.rockTop))
		for i, v := range l.rockTop {
			edge[i] = v.add(pt{0, 0.0032})
		}
		c.fill(solid(mix(rockLit, cream, 0.45), 0.85), ribbon(edge, half))
	}
}

// join concatenates polygon fragments into one outline.
func join(parts ...[]pt) []pt {
	var out []pt
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// reversed returns a copy of pts in reverse order.
func reversed(pts []pt) []pt {
	out := make([]pt, len(pts))
	for i, v := range pts {
		out[len(pts)-1-i] = v
	}
	return out
}

// drawOrbit paints the thin cream orbit arc and positions the drifting rock
// at its head. The orbit is an ellipse whose long axis runs between the lower
// left and upper right corners of the badge, where the hexagon has most room;
// the arc trails behind the rock and tapers to nothing.
func (l *layout) drawOrbit(c *canvas) {
	const (
		rx, ry = 0.355, 0.315
		tilt   = -30.0 // degrees; negative lifts the long axis up to the right
		from   = 196.0 // tail, degrees along the ellipse
		to     = 356.0 // head
		steps  = 192
	)
	centre := pt{0.5, 0.5}
	sin, cos := math.Sincos(tilt * math.Pi / 180)
	pts := make([]pt, steps+1)
	half := make([]float64, steps+1)
	for i := range pts {
		u := float64(i) / steps
		a := (from + (to-from)*u) * math.Pi / 180
		e := pt{rx * math.Cos(a), ry * math.Sin(a)}
		pts[i] = centre.add(pt{e.x*cos - e.y*sin, e.x*sin + e.y*cos})
		// Taper the tail to a point; full width over the leading part.
		t := clamp01(u / 0.6)
		half[i] = l.orbitWidth / 2 * t * t * (3 - 2*t)
	}
	c.fill(solid(cream, 1), ribbon(pts, half))
	l.dot = pts[steps]
}
