package art

import (
	"math"

	"drifthold/internal/gfx"
	"drifthold/internal/model"
)

// ---------------------------------------------------------------------------
// Rock badge
// ---------------------------------------------------------------------------

// badgeHalf is the half extent of a badge sprite relative to its radius.
const badgeHalf = 1.25

// drawRockBadge draws the silhouette of the rock's actual blob, scaled to
// fit a circle of the given radius: the dark skirt all round, the plateau in
// the class colour on top of it, a strip of light along the upper left.
func drawRockBadge(cv *gfx.Canvas, rock *model.Rock, cx, cy, radius float32) {
	if cv == nil || cv.Dst == nil || rock == nil || !(radius > 0) || len(rock.Tiles) == 0 {
		return
	}
	s := canvasScale(cv)
	unitPx := bucketPx(radius * s)
	half := int(math.Ceil(float64(badgeHalf * unitPx)))
	ext := float32(half) / s
	x, y := snapCentre(cv, cx, cy)
	if !cv.Visible(gfx.R(x-ext, y-ext, 2*ext, 2*ext)) {
		return
	}
	terr, _, _ := rockState(rock, model.NoFaction)
	id := domBadge | (mix64(rock.ShapeSeed^terr^uint64(rock.Class)<<8^uint64(rock.ID)<<20) >> 4)
	img := sprite(id, 2*half, 2*half, s, func(sc *gfx.Canvas) {
		c := float32(half) / s
		paintBadge(sc, rock, c, c, unitPx/s)
	})
	if rs := radius * s; rs > unitPx*1.04 {
		e := ext * rs / unitPx
		cv.DrawImage(img, gfx.R(x-e, y-e, 2*e, 2*e), nil)
		return
	}
	cv.DrawSprite(img, x, y, nil)
}

// badgeSides is the number of corners of a badge's outline.
const badgeSides = 14

// badgeOutline reduces the rock's blob to a polygon of badgeSides corners
// around its centre, each as far out as the blob reaches in that direction,
// normalised so that the farthest corner is at distance 1.
func badgeOutline(rock *model.Rock) (xy [2 * badgeSides]float32, ok bool) {
	loops := blobOutline(rock)
	if len(loops) == 0 {
		return xy, false
	}
	// The centre of the blob's bounding box.
	minX, minY := float32(math.MaxFloat32), float32(math.MaxFloat32)
	maxX, maxY := float32(-math.MaxFloat32), float32(-math.MaxFloat32)
	for _, loop := range loops {
		for _, v := range loop {
			minX, maxX = min(minX, v.x), max(maxX, v.x)
			minY, maxY = min(minY, v.y), max(maxY, v.y)
		}
	}
	mx, my := (minX+maxX)/2, (minY+maxY)/2
	var reach [badgeSides]float32
	for _, loop := range loops {
		for _, v := range loop {
			dx, dy := v.x-mx, v.y-my
			a := math.Atan2(float64(dy), float64(dx))
			k := int(math.Floor((a+math.Pi)/(2*math.Pi)*badgeSides+0.5)) % badgeSides
			reach[k] = max(reach[k], float32(math.Hypot(float64(dx), float64(dy))))
		}
	}
	// Fill directions nothing fell into from their neighbours, then take
	// the sharpest spikes off.
	for pass := 0; pass < badgeSides; pass++ {
		for k := range reach {
			if reach[k] == 0 {
				reach[k] = max(reach[(k+1)%badgeSides], reach[(k+badgeSides-1)%badgeSides])
			}
		}
	}
	var soft [badgeSides]float32
	var mean float32
	for k := range reach {
		soft[k] = 0.15*reach[(k+badgeSides-1)%badgeSides] + 0.7*reach[k] + 0.15*reach[(k+1)%badgeSides]
		mean += soft[k] / badgeSides
	}
	if mean <= 0 {
		return xy, false
	}
	// At a few pixels across, the real unevenness of a blob would be lost:
	// exaggerate it so that each rock keeps a shape of its own.
	var far float32
	for k := range soft {
		soft[k] = max(0.55*mean, mean+1.5*(soft[k]-mean))
		far = max(far, soft[k])
	}
	for k := range soft {
		a := -math.Pi + 2*math.Pi*float64(k)/badgeSides
		r := soft[k] / far
		xy[2*k], xy[2*k+1] = r*float32(math.Cos(a)), r*float32(math.Sin(a))
	}
	return xy, true
}

// paintBadge draws the rock as the surface view shows it, in miniature: a
// plateau in the class colour ringed by cliff facets that are light towards
// the upper left and dark towards the lower right.
func paintBadge(cv *gfx.Canvas, rock *model.Rock, cx, cy, radius float32) {
	outer, ok := badgeOutline(rock)
	if !ok {
		return
	}
	pal := paletteFor(rock.Class)
	// A badge is seen small and against the dark of space or of a panel:
	// its tones are pushed apart further than the surface's.
	pal.base = tone(pal.base, 0.12)
	pal.cliffLit = gfx.Lerp(gfx.Lighten(pal.base, 0.4), gfx.RGB(0xFFF1D6), 0.2)
	pal.cliffMid = tone(pal.base, -0.3)
	p := newPen(cv, cx, cy, radius)
	seed := rock.ShapeSeed
	// The plateau: the outline drawn in, a little towards the light.
	const inset = 0.64
	var top [2 * badgeSides]float32
	for i := 0; i < badgeSides; i++ {
		top[2*i] = outer[2*i]*inset + lightX*0.09
		top[2*i+1] = outer[2*i+1]*inset + lightY*0.09
	}
	p.poly(pal.cliffMid, outer[:]...)
	for i := 0; i < badgeSides; i++ {
		j := (i + 1) % badgeSides
		ex, ey := outer[2*j]-outer[2*i], outer[2*j+1]-outer[2*i+1]
		l := float32(math.Hypot(float64(ex), float64(ey)))
		if l == 0 {
			continue
		}
		f := facing(ey/l, -ex/l) + (unit(mix64(seed+uint64(i)*977))-0.5)*0.3
		var c gfx.Color
		if f >= 0 {
			c = gfx.Lerp(pal.cliffMid, pal.cliffLit, min(1, f*1.2))
		} else {
			c = gfx.Lerp(pal.cliffMid, pal.cliffDark, min(1, -f*1.2))
		}
		p.poly(c, outer[2*i], outer[2*i+1], outer[2*j], outer[2*j+1], top[2*j], top[2*j+1], top[2*i], top[2*i+1])
	}
	p.outline(pal.outline, 0.05, 1, outer[:]...)
	p.polyV(tone(pal.base, 0.09), tone(pal.base, -0.07), top[:]...)

	// Larger badges have room for one crater on the plateau.
	if radius*canvasScale(cv) >= 9 {
		r := newRnd(seed ^ 0xBAD6E)
		a, d := r.in(0, tau), r.in(0.1, 0.26)
		x, y := lightX*0.09+d*cosf(a), lightY*0.09+d*sinf(a)
		rad := r.in(0.1, 0.15)
		p.circle(x, y, rad, tone(pal.base, -0.17))
		p.circle(x-lightX*rad*0.3, y-lightY*rad*0.3, rad*0.7, tone(pal.base, -0.04))
	}
}

// ---------------------------------------------------------------------------
// Emberfall
// ---------------------------------------------------------------------------

var (
	starCorona = gfx.RGB(0xE5533D)
	starHalo   = gfx.RGB(0xF2814A)
	starRim    = gfx.RGB(0xC23B24)
	starBody   = gfx.RGB(0xE8663A)
	starWarm   = gfx.RGB(0xF59A52)
	starHot    = gfx.RGB(0xFFC680)
	starSpot   = gfx.RGB(0xA02D1C)
)

// drawStar draws the red dwarf: a banded disc with slow spots turning across
// it, inside a corona that breathes and flickers.
func drawStar(cv *gfx.Canvas, cx, cy, r, t float32) {
	if cv == nil || cv.Dst == nil || !(r > 0) {
		return
	}
	if !cv.Visible(gfx.R(cx-5*r, cy-5*r, 10*r, 10*r)) {
		return
	}
	t = finite(t)
	// Slow breathing with a faster, fainter flicker on top.
	f := 0.9 + 0.06*sinf(t*0.7) + 0.03*sinf(t*2.3+1.1) + 0.015*sinf(t*6.1+0.4)
	cv.Glow(cx, cy, r*4.6, alpha(starCorona, 0.34*f))
	cv.Glow(cx, cy, r*2.5, alpha(starHalo, 0.5*f))

	// An uneven corona: slow swells of light hugging the limb.
	for i := 0; i < 5; i++ {
		a := float32(i)*1.2566 + 0.3*sinf(t*0.11+float32(i)*2.1) + t*0.015
		l := 0.5 + 0.5*sinf(t*0.37+float32(i)*1.7)
		reach := r * (0.82 + 0.14*l)
		cv.Glow(cx+reach*cosf(a), cy+reach*sinf(a), r*(0.75+0.3*l), alpha(starWarm, (0.1+0.2*l)*f))
	}

	cv.FillCircle(cx, cy, r, starRim)
	cv.FillCircle(cx, cy, r*0.93, starBody)
	cv.FillCircle(cx-r*0.02, cy-r*0.03, r*0.74, gfx.Lerp(starBody, starWarm, 0.75))
	cv.FillCircle(cx-r*0.04, cy-r*0.06, r*0.5, starWarm)

	// Spots carried round by the star's turning: they cross the disc from
	// left to right, foreshortened towards the limb.
	if r*canvasScale(cv) >= 9 {
		for i := 0; i < 6; i++ {
			lat := -0.9 + 1.8*unit(mix64(uint64(i)*7+3))
			lon := wrap(t*0.045+float32(i)*1.13, tau) - pi
			if lon < -1.35 || lon > 1.35 {
				continue
			}
			size := r * (0.07 + 0.08*unit(mix64(uint64(i)*13+5)))
			x := cx + r*0.86*cosf(lat)*sinf(lon)
			y := cy + r*0.86*sinf(lat)
			edge := cosf(lon)
			cv.Blob(x, y, size*1.9*edge+size*0.3, size*1.9, alpha(starSpot, 0.55*edge))
		}
	}
	cv.GlowCore(cx-r*0.05, cy-r*0.08, r*1.2, alpha(starHot, 0.6*f))
}

// ---------------------------------------------------------------------------
// Explosion
// ---------------------------------------------------------------------------

// easeOut decelerates from 0 to 1.
func easeOut(t float32) float32 {
	t = 1 - clamp01(t)
	return 1 - t*t*t
}

// drawExplosion draws one frame of a blast: a white flash, a fireball that
// swells and cools, a shock ring racing outwards, embers thrown clear and a
// little smoke left behind.
func drawExplosion(cv *gfx.Canvas, cx, cy, r, ph float32) {
	if cv == nil || cv.Dst == nil || !(r > 0) || !(ph >= 0) || ph >= 1 {
		return
	}
	if !cv.Visible(gfx.R(cx-2.2*r, cy-2.2*r, 4.4*r, 4.4*r)) {
		return
	}
	px := 1 / canvasScale(cv)
	fade := 1 - ph

	// Smoke, underneath everything, late in the blast.
	if ph > 0.3 {
		a := smooth((ph-0.3)/0.25) * fade * 0.5
		for i := 0; i < 3; i++ {
			ang := float32(i)*2.3 + 0.7
			d := r * (0.18 + 0.3*ph)
			rad := r * (0.34 + 0.34*ph)
			cv.Blob(cx+d*cosf(ang), cy+d*sinf(ang)-r*0.18*ph, rad, rad, gfx.Color{R: 0x2A, G: 0x26, B: 0x2A, A: uint8(255 * a)})
		}
	}

	// Light thrown on the surroundings.
	cv.Glow(cx, cy, r*(1.2+1.1*ph), alpha(colHot, 0.7*fade*fade))

	// The shock ring.
	ringR := r * (0.2 + 0.82*easeOut(ph))
	cv.StrokeCircle(cx, cy, ringR, max(r*0.09*fade, px), alpha(gfx.RGB(0xFFE9CC), 0.8*fade*fade))

	// The fireball: a ragged cloud of lobes around a hot heart, cooling
	// from yellow through orange to a dull red as it thins.
	if ph < 0.75 {
		grow := easeOut(ph / 0.5)
		a := 1 - smooth((ph-0.3)/0.45)
		outer := gfx.Lerp(colHot, gfx.RGB(0x7A2418), smooth((ph-0.15)/0.5))
		mid := gfx.Lerp(gfx.RGB(0xFFB55A), gfx.RGB(0xC2491F), smooth((ph-0.15)/0.5))
		inner := gfx.Lerp(gfx.RGB(0xFFF1C4), colHot, smooth((ph-0.1)/0.45))
		for i := 0; i < 5; i++ {
			ang := float32(i)*1.2566 + 0.4
			k := 0.75 + 0.5*unit(mix64(uint64(i)*17+3))
			d := r * 0.26 * grow * k
			cv.FillCircle(cx+d*cosf(ang), cy+d*sinf(ang), r*0.3*grow*(1.5-k*0.5), alpha(outer, a))
		}
		for i := 0; i < 3; i++ {
			ang := float32(i)*2.094 + 1.3
			d := r * 0.13 * grow
			cv.FillCircle(cx+d*cosf(ang), cy+d*sinf(ang), r*0.25*grow, alpha(mid, a))
		}
		cv.FillCircle(cx, cy, r*0.24*grow*(1-0.6*ph), alpha(inner, a))
	}

	// Embers thrown clear of the fireball, each trailing a short streak.
	for i := 0; i < 11; i++ {
		ang := float32(i)*2.39996 + 0.2
		speed := 0.75 + 0.55*unit(mix64(uint64(i)*29+11))
		d := r * speed * easeOut(ph*0.9)
		x, y := cx+d*cosf(ang), cy+d*sinf(ang)
		tail := r * 0.22 * fade * fade
		c := gfx.Lerp(colHotCore, colHot, ph)
		cv.Line(x-tail*cosf(ang), y-tail*sinf(ang), x, y, max(r*0.035*fade, px), alpha(c, fade))
		cv.FillCircle(x, y, max(r*0.04*fade, 0.7*px), alpha(gfx.RGB(0xFFF3D0), fade))
	}

	// The flash.
	if ph < 0.22 {
		f := 1 - ph/0.22
		cv.GlowCore(cx, cy, r*(1.0+2.6*ph), alpha(gfx.RGB(0xFFF6E0), f))
		cv.FillCircle(cx, cy, r*0.34*f, alpha(gfx.White, f))
	}
}
