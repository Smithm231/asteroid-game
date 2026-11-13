package model

import "math"

// Orbit and sunlight constants of the belt.
const (
	// OrbitRMin is the smallest orbit radius of a rock, in u.
	OrbitRMin = 380.0
	// OrbitRMax is the largest orbit radius of a rock, in u.
	OrbitRMax = 950.0
	// OrbitRefRadius is the orbit radius, in u, whose period is
	// OrbitRefPeriod.
	OrbitRefRadius = 400.0
	// OrbitRefPeriod is the orbital period, in game-seconds, at
	// OrbitRefRadius (18 minutes).
	OrbitRefPeriod = 1080.0
	// OrbitMinGap is the minimum radial separation, in u, between two rocks
	// that are not well separated in phase.
	OrbitMinGap = 35.0

	// SolarRefRadius is the orbit radius, in u, at which the solar factor is
	// exactly 1.
	SolarRefRadius = 520.0
	// SolarExponent is the exponent of the solar falloff.
	SolarExponent = 1.2
	// SolarMin is the lower clamp of the solar factor.
	SolarMin = 0.45
	// SolarMax is the upper clamp of the solar factor.
	SolarMax = 1.6
)

// OrbitOmega returns the angular speed, in radians per game-second, of a
// circular prograde orbit of radius r (in u):
//
//	omega(r) = 2*pi / (OrbitRefPeriod * (r/OrbitRefRadius)^1.5)
//
// A radius that is not positive yields 0.
func OrbitOmega(r float64) float64 {
	if !(r > 0) {
		return 0
	}
	x := r / OrbitRefRadius
	// x^1.5 computed as x*sqrt(x): exact rounding of sqrt keeps this
	// reproducible everywhere.
	return 2 * math.Pi / (OrbitRefPeriod * x * math.Sqrt(x))
}

// OrbitPeriod returns the orbital period, in game-seconds, of an orbit of
// radius r (in u), or 0 when r is not positive.
func OrbitPeriod(r float64) float64 {
	w := OrbitOmega(r)
	if w == 0 {
		return 0
	}
	return 2 * math.Pi / w
}

// OrbitAngle returns the orbital angle, in radians, at the given tick of a
// body with orbit radius r and phase phase0 at tick 0:
//
//	angle = phase0 + omega(r) * tick / TicksPerSecond
func OrbitAngle(r, phase0 float64, tick int) float64 {
	return phase0 + OrbitOmega(r)*float64(tick)/TicksPerSecond
}

// OrbitPos returns the belt position at the given tick of a body with orbit
// radius r and phase phase0 at tick 0: (r*cos(angle), r*sin(angle)).
func OrbitPos(r, phase0 float64, tick int) Vec2 {
	return Polar(r, OrbitAngle(r, phase0, tick))
}

// SolarFactor returns the sunlight multiplier at orbit radius r (in u):
//
//	clamp((SolarRefRadius/r)^SolarExponent, SolarMin, SolarMax)
//
// A radius that is not positive yields SolarMax.
func SolarFactor(r float64) float64 {
	if !(r > 0) {
		return SolarMax
	}
	return Clamp(math.Pow(SolarRefRadius/r, SolarExponent), SolarMin, SolarMax)
}
