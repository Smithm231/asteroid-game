package model

import "math"

// Vec2 is a point or direction in the belt plane (units "u", Emberfall at the
// origin) or in screen space when used by the hex/pixel helpers.
type Vec2 struct {
	X float64 // horizontal component
	Y float64 // vertical component
}

// Polar returns the vector of length r at the given angle (radians, measured
// from the +X axis toward +Y).
func Polar(r, angle float64) Vec2 {
	return Vec2{X: r * math.Cos(angle), Y: r * math.Sin(angle)}
}

// Add returns a + b.
func (a Vec2) Add(b Vec2) Vec2 { return Vec2{a.X + b.X, a.Y + b.Y} }

// Sub returns a - b.
func (a Vec2) Sub(b Vec2) Vec2 { return Vec2{a.X - b.X, a.Y - b.Y} }

// Scale returns a multiplied by the scalar k.
func (a Vec2) Scale(k float64) Vec2 { return Vec2{a.X * k, a.Y * k} }

// Dot returns the dot product of a and b.
func (a Vec2) Dot(b Vec2) float64 { return a.X*b.X + a.Y*b.Y }

// LenSq returns the squared length of a.
func (a Vec2) LenSq() float64 { return a.X*a.X + a.Y*a.Y }

// Len returns the length of a.
func (a Vec2) Len() float64 { return math.Sqrt(a.LenSq()) }

// DistSq returns the squared distance between a and b.
func (a Vec2) DistSq(b Vec2) float64 { return a.Sub(b).LenSq() }

// Dist returns the distance between a and b.
func (a Vec2) Dist(b Vec2) float64 { return a.Sub(b).Len() }

// Norm returns the unit vector in the direction of a, or the zero vector when
// a has zero length.
func (a Vec2) Norm() Vec2 {
	l := a.Len()
	if l == 0 {
		return Vec2{}
	}
	return Vec2{a.X / l, a.Y / l}
}

// Angle returns the direction of a in radians in (-pi, pi], measured from the
// +X axis toward +Y. The zero vector has angle 0.
func (a Vec2) Angle() float64 { return math.Atan2(a.Y, a.X) }

// Rotate returns a rotated by angle radians (from +X toward +Y).
func (a Vec2) Rotate(angle float64) Vec2 {
	s, c := math.Sin(angle), math.Cos(angle)
	return Vec2{a.X*c - a.Y*s, a.X*s + a.Y*c}
}

// Lerp returns the point a + (b-a)*t; t = 0 gives a, t = 1 gives b.
func (a Vec2) Lerp(b Vec2, t float64) Vec2 {
	return Vec2{a.X + (b.X-a.X)*t, a.Y + (b.Y-a.Y)*t}
}

// Toward returns a moved straight toward b by at most step. When b is no
// farther away than step the result is exactly b (no overshoot, no rounding
// residue); a step that is not positive leaves a where it is.
func (a Vec2) Toward(b Vec2, step float64) Vec2 {
	d := b.Sub(a)
	l := d.Len()
	if l <= step || l == 0 {
		return b
	}
	if step <= 0 {
		return a
	}
	return a.Add(d.Scale(step / l))
}

// Clamp returns x limited to the range [lo, hi].
func Clamp(x, lo, hi float64) float64 {
	if x != x { // NaN
		return lo
	}
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// Clamp01 returns x limited to the range [0, 1].
func Clamp01(x float64) float64 { return Clamp(x, 0, 1) }
