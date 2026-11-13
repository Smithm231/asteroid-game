package model

import (
	"math"
	"testing"
)

func TestHexDirectionsOrder(t *testing.T) {
	want := [6]Hex{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}
	if got := HexDirections(); got != want {
		t.Fatalf("HexDirections() = %v, want %v", got, want)
	}
	for i := -12; i < 12; i++ {
		if got, w := HexDir(i), want[((i%6)+6)%6]; got != w {
			t.Errorf("HexDir(%d) = %v, want %v", i, got, w)
		}
	}
	// Opposite directions cancel.
	for i := 0; i < 6; i++ {
		if sum := HexDir(i).Add(HexDir(i + 3)); sum != (Hex{}) {
			t.Errorf("direction %d and its opposite sum to %v", i, sum)
		}
	}
}

func TestHexArithmetic(t *testing.T) {
	a, b := Hex{2, -3}, Hex{-1, 4}
	if got := a.Add(b); got != (Hex{1, 1}) {
		t.Errorf("Add = %v", got)
	}
	if got := a.Sub(b); got != (Hex{3, -7}) {
		t.Errorf("Sub = %v", got)
	}
	if got := a.Scale(3); got != (Hex{6, -9}) {
		t.Errorf("Scale = %v", got)
	}
	if got := a.S(); got != 1 {
		t.Errorf("S = %d", got)
	}
	if got := a.String(); got != "(2,-3)" {
		t.Errorf("String = %q", got)
	}
}

func TestHexDist(t *testing.T) {
	tests := []struct {
		a, b Hex
		want int
	}{
		{Hex{0, 0}, Hex{0, 0}, 0},
		{Hex{0, 0}, Hex{1, 0}, 1},
		{Hex{0, 0}, Hex{1, -1}, 1},
		{Hex{0, 0}, Hex{1, 1}, 2},
		{Hex{0, 0}, Hex{-1, -1}, 2},
		{Hex{0, 0}, Hex{3, -3}, 3},
		{Hex{0, 0}, Hex{-2, 5}, 5},
		{Hex{2, -1}, Hex{-3, 4}, 5},
		{Hex{-4, 2}, Hex{4, -2}, 8},
		{Hex{7, 0}, Hex{0, 7}, 7},
	}
	for _, tc := range tests {
		if got := tc.a.Dist(tc.b); got != tc.want {
			t.Errorf("%v.Dist(%v) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
		if got := tc.b.Dist(tc.a); got != tc.want {
			t.Errorf("distance not symmetric for %v, %v", tc.a, tc.b)
		}
		if got := tc.b.Sub(tc.a).Len(); got != tc.want {
			t.Errorf("Len of difference = %d, want %d", got, tc.want)
		}
	}
}

func TestHexNeighbors(t *testing.T) {
	for _, h := range []Hex{{0, 0}, {3, -2}, {-5, 5}} {
		ns := h.Neighbors()
		seen := map[Hex]bool{}
		for i, n := range ns {
			if n != h.Neighbor(i) {
				t.Errorf("Neighbors()[%d] = %v, Neighbor(%d) = %v", i, n, i, h.Neighbor(i))
			}
			if n != h.Add(HexDir(i)) {
				t.Errorf("neighbour %d of %v is %v", i, h, n)
			}
			if !h.Adjacent(n) || h.Dist(n) != 1 {
				t.Errorf("neighbour %v of %v is not at distance 1", n, h)
			}
			if seen[n] {
				t.Errorf("duplicate neighbour %v", n)
			}
			seen[n] = true
		}
		if h.Adjacent(h) {
			t.Errorf("%v is adjacent to itself", h)
		}
	}
}

func TestHexRing(t *testing.T) {
	c := Hex{2, -1}
	if got := c.Ring(-1); got != nil {
		t.Errorf("Ring(-1) = %v, want nil", got)
	}
	if got := c.Ring(0); len(got) != 1 || got[0] != c {
		t.Errorf("Ring(0) = %v, want [%v]", got, c)
	}
	for r := 1; r <= 12; r++ {
		ring := c.Ring(r)
		if len(ring) != 6*r {
			t.Fatalf("Ring(%d) has %d hexes, want %d", r, len(ring), 6*r)
		}
		seen := map[Hex]bool{}
		for i, h := range ring {
			if c.Dist(h) != r {
				t.Errorf("Ring(%d)[%d] = %v is at distance %d", r, i, h, c.Dist(h))
			}
			if seen[h] {
				t.Errorf("Ring(%d) repeats %v", r, h)
			}
			seen[h] = true
			// Consecutive ring hexes are neighbours (the ring is a walk).
			if next := ring[(i+1)%len(ring)]; !h.Adjacent(next) {
				t.Errorf("Ring(%d): %v and %v are not adjacent", r, h, next)
			}
		}
		if want := c.Add(Hex{-r, r}); ring[0] != want {
			t.Errorf("Ring(%d) starts at %v, want %v", r, ring[0], want)
		}
	}
}

func TestHexSpiral(t *testing.T) {
	c := Hex{-3, 4}
	if got := c.Spiral(-1); got != nil {
		t.Errorf("Spiral(-1) = %v, want nil", got)
	}
	for r := 0; r <= 11; r++ {
		sp := c.Spiral(r)
		if len(sp) != HexCount(r) {
			t.Fatalf("Spiral(%d) has %d hexes, want %d", r, len(sp), HexCount(r))
		}
		if sp[0] != c {
			t.Errorf("Spiral(%d) starts at %v", r, sp[0])
		}
		seen := map[Hex]bool{}
		prev := 0
		for _, h := range sp {
			d := c.Dist(h)
			if d < prev {
				t.Errorf("Spiral(%d): distance decreases at %v", r, h)
			}
			prev = d
			if d > r {
				t.Errorf("Spiral(%d) contains %v at distance %d", r, h, d)
			}
			if seen[h] {
				t.Errorf("Spiral(%d) repeats %v", r, h)
			}
			seen[h] = true
		}
	}
	// Spiral is the concatenation of the rings.
	var want []Hex
	for r := 0; r <= 4; r++ {
		want = append(want, c.Ring(r)...)
	}
	got := c.Spiral(4)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Spiral(4)[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestHexWithinRadius(t *testing.T) {
	c := Hex{5, -2}
	if got := c.WithinRadius(-1); got != nil {
		t.Errorf("WithinRadius(-1) = %v, want nil", got)
	}
	for r := 0; r <= 11; r++ {
		disc := c.WithinRadius(r)
		if len(disc) != HexCount(r) {
			t.Fatalf("WithinRadius(%d) has %d hexes, want %d", r, len(disc), HexCount(r))
		}
		inSpiral := map[Hex]bool{}
		for _, h := range c.Spiral(r) {
			inSpiral[h] = true
		}
		for i, h := range disc {
			if !inSpiral[h] {
				t.Errorf("WithinRadius(%d) contains %v, which Spiral does not", r, h)
			}
			if i > 0 {
				p := disc[i-1]
				if !(p.R < h.R || (p.R == h.R && p.Q < h.Q)) {
					t.Errorf("WithinRadius(%d) not in row-major order at %v -> %v", r, p, h)
				}
			}
		}
	}
}

func TestHexCount(t *testing.T) {
	want := map[int]int{-1: 0, 0: 1, 1: 7, 2: 19, 3: 37, 7: 169, 11: 397}
	for r, n := range want {
		if got := HexCount(r); got != n {
			t.Errorf("HexCount(%d) = %d, want %d", r, got, n)
		}
	}
}

func TestHexToPixelKnownPoints(t *testing.T) {
	const size = 10.0
	tests := []struct {
		h    Hex
		x, y float64
	}{
		{Hex{0, 0}, 0, 0},
		{Hex{1, 0}, size * math.Sqrt(3), 0},
		{Hex{0, 1}, size * math.Sqrt(3) / 2, size * 1.5},
		{Hex{-1, 1}, -size * math.Sqrt(3) / 2, size * 1.5},
		{Hex{1, -1}, size * math.Sqrt(3) / 2, -size * 1.5},
		{Hex{2, 2}, size * math.Sqrt(3) * 3, size * 3},
	}
	for _, tc := range tests {
		p := HexToPixel(tc.h, size)
		if math.Abs(p.X-tc.x) > 1e-9 || math.Abs(p.Y-tc.y) > 1e-9 {
			t.Errorf("HexToPixel(%v) = %v, want (%v,%v)", tc.h, p, tc.x, tc.y)
		}
	}
	// All six neighbours are exactly sqrt(3)*size away from the centre.
	for _, n := range (Hex{}).Neighbors() {
		if d := HexToPixel(n, size).Len(); math.Abs(d-size*math.Sqrt(3)) > 1e-9 {
			t.Errorf("neighbour %v is %v px away", n, d)
		}
	}
}

func TestPixelHexRoundTrip(t *testing.T) {
	rng := NewRNG(42)
	for _, size := range []float64{1, 7.5, 16, 24, 63.25} {
		inner := size * math.Sqrt(3) / 2 // centre-to-edge distance
		for _, h := range (Hex{}).WithinRadius(14) {
			centre := HexToPixel(h, size)
			if got := PixelToHex(centre, size); got != h {
				t.Fatalf("size %v: centre of %v maps to %v", size, h, got)
			}
			// Any point inside the inscribed circle belongs to the hex.
			for i := 0; i < 8; i++ {
				p := centre.Add(Polar(rng.Range(0, inner*0.98), rng.Range(0, 2*math.Pi)))
				if got := PixelToHex(p, size); got != h {
					t.Fatalf("size %v: point %v inside %v maps to %v", size, p, h, got)
				}
			}
		}
	}
}

func TestPixelToHexNearestCentre(t *testing.T) {
	// For arbitrary points the chosen hex must be the one whose centre is
	// nearest (that is what makes cube rounding "correct").
	const size = 12.0
	rng := NewRNG(7)
	for i := 0; i < 5000; i++ {
		p := Vec2{rng.Range(-300, 300), rng.Range(-300, 300)}
		h := PixelToHex(p, size)
		best := HexToPixel(h, size).Dist(p)
		for _, n := range h.Neighbors() {
			if d := HexToPixel(n, size).Dist(p); d < best-1e-9 {
				t.Fatalf("point %v mapped to %v (%.4f) but neighbour %v is closer (%.4f)", p, h, best, n, d)
			}
		}
	}
	if got := PixelToHex(Vec2{100, 100}, 0); got != (Hex{}) {
		t.Errorf("PixelToHex with size 0 = %v, want (0,0)", got)
	}
	if got := PixelToHex(Vec2{100, 100}, -5); got != (Hex{}) {
		t.Errorf("PixelToHex with negative size = %v, want (0,0)", got)
	}
}

func TestHexRound(t *testing.T) {
	tests := []struct {
		q, r float64
		want Hex
	}{
		{0, 0, Hex{0, 0}},
		{0.3, 0.3, Hex{0, 0}},
		{0.9, 0.05, Hex{1, 0}},
		{-0.9, 0.95, Hex{-1, 1}},
		{2.2, -3.1, Hex{2, -3}},
		{0.7, 0.7, Hex{1, 1}},
		{0.5, 0.1, Hex{1, 0}},
		{-4.49, 2.2, Hex{-4, 2}},
	}
	for _, tc := range tests {
		if got := HexRound(tc.q, tc.r); got != tc.want {
			t.Errorf("HexRound(%v,%v) = %v, want %v", tc.q, tc.r, got, tc.want)
		}
	}
}

func TestHexCorners(t *testing.T) {
	const size = 20.0
	centre := Vec2{100, -50}
	cs := HexCorners(centre, size)
	for i, c := range cs {
		if d := c.Dist(centre); math.Abs(d-size) > 1e-9 {
			t.Errorf("corner %d is %v from the centre, want %v", i, d, size)
		}
		// Adjacent corners are one side length (= size) apart.
		if d := c.Dist(cs[(i+1)%6]); math.Abs(d-size) > 1e-9 {
			t.Errorf("corners %d and %d are %v apart, want %v", i, (i+1)%6, d, size)
		}
	}
	// Pointy-top: there is a corner straight above and straight below.
	if cs[5].X != centre.X || cs[5].Y != centre.Y-size {
		t.Errorf("corner 5 = %v, want straight above the centre", cs[5])
	}
	if cs[2].X != centre.X || cs[2].Y != centre.Y+size {
		t.Errorf("corner 2 = %v, want straight below the centre", cs[2])
	}
	// Every corner is shared with two neighbours: it is equidistant from
	// the centre and from two neighbouring hex centres.
	origin := HexCorners(Vec2{}, size)
	for i, c := range origin {
		shared := 0
		for _, n := range (Hex{}).Neighbors() {
			if math.Abs(HexToPixel(n, size).Dist(c)-size) < 1e-9 {
				shared++
			}
		}
		if shared != 2 {
			t.Errorf("corner %d touches %d neighbouring hexes, want 2", i, shared)
		}
	}
}

func TestVec2(t *testing.T) {
	a, b := Vec2{3, 4}, Vec2{-1, 2}
	if got := a.Add(b); got != (Vec2{2, 6}) {
		t.Errorf("Add = %v", got)
	}
	if got := a.Sub(b); got != (Vec2{4, 2}) {
		t.Errorf("Sub = %v", got)
	}
	if got := a.Scale(2); got != (Vec2{6, 8}) {
		t.Errorf("Scale = %v", got)
	}
	if got := a.Len(); got != 5 {
		t.Errorf("Len = %v", got)
	}
	if got := a.LenSq(); got != 25 {
		t.Errorf("LenSq = %v", got)
	}
	if got := a.Dot(b); got != 5 {
		t.Errorf("Dot = %v", got)
	}
	if got := a.Dist(Vec2{0, 0}); got != 5 {
		t.Errorf("Dist = %v", got)
	}
	if got := a.Norm(); math.Abs(got.Len()-1) > 1e-12 {
		t.Errorf("Norm length = %v", got.Len())
	}
	if got := (Vec2{}).Norm(); got != (Vec2{}) {
		t.Errorf("Norm of zero = %v", got)
	}
	if got := a.Lerp(b, 0.5); got != (Vec2{1, 3}) {
		t.Errorf("Lerp = %v", got)
	}
	if got := Polar(2, math.Pi/2); math.Abs(got.X) > 1e-12 || math.Abs(got.Y-2) > 1e-12 {
		t.Errorf("Polar = %v", got)
	}
	if got := (Vec2{0, 3}).Angle(); math.Abs(got-math.Pi/2) > 1e-12 {
		t.Errorf("Angle = %v", got)
	}
	if got := (Vec2{1, 0}).Rotate(math.Pi / 2); math.Abs(got.X) > 1e-12 || math.Abs(got.Y-1) > 1e-12 {
		t.Errorf("Rotate = %v", got)
	}

	// Toward: partial step, overshoot, and degenerate steps.
	from, to := Vec2{0, 0}, Vec2{10, 0}
	if got := from.Toward(to, 4); got != (Vec2{4, 0}) {
		t.Errorf("Toward partial = %v", got)
	}
	if got := from.Toward(to, 10); got != to {
		t.Errorf("Toward exact = %v", got)
	}
	if got := from.Toward(to, 25); got != to {
		t.Errorf("Toward overshoot = %v", got)
	}
	if got := from.Toward(to, 0); got != from {
		t.Errorf("Toward zero step = %v", got)
	}
	if got := from.Toward(to, -3); got != from {
		t.Errorf("Toward negative step = %v", got)
	}
	if got := to.Toward(to, 1); got != to {
		t.Errorf("Toward self = %v", got)
	}

	if Clamp(5, 0, 3) != 3 || Clamp(-1, 0, 3) != 0 || Clamp(2, 0, 3) != 2 {
		t.Error("Clamp is wrong")
	}
	if Clamp01(1.5) != 1 || Clamp01(-0.5) != 0 || Clamp01(0.25) != 0.25 {
		t.Error("Clamp01 is wrong")
	}
}

func TestOrbitMaths(t *testing.T) {
	// r = 400 has an 18-minute period.
	if got, want := OrbitOmega(400), 2*math.Pi/1080; math.Abs(got-want) > 1e-15 {
		t.Errorf("OrbitOmega(400) = %v, want %v", got, want)
	}
	if got := OrbitPeriod(400); math.Abs(got-1080) > 1e-9 {
		t.Errorf("OrbitPeriod(400) = %v, want 1080", got)
	}
	// Period scales with r^1.5: four times the radius, eight times the
	// period.
	if got := OrbitPeriod(1600); math.Abs(got-8*1080) > 1e-6 {
		t.Errorf("OrbitPeriod(1600) = %v, want %v", got, 8*1080.0)
	}
	// Outer rocks are slower.
	prev := math.Inf(1)
	for r := OrbitRMin; r <= OrbitRMax; r += 10 {
		w := OrbitOmega(r)
		if !(w > 0) || w >= prev {
			t.Fatalf("OrbitOmega(%v) = %v is not positive and decreasing", r, w)
		}
		want := 2 * math.Pi / (1080 * math.Pow(r/400, 1.5))
		if math.Abs(w-want) > 1e-15 {
			t.Fatalf("OrbitOmega(%v) = %v, want %v", r, w, want)
		}
		prev = w
	}
	if OrbitOmega(0) != 0 || OrbitOmega(-5) != 0 || OrbitPeriod(0) != 0 {
		t.Error("non-positive radius must give omega 0 and period 0")
	}

	rock := &Rock{OrbitR: 520, Phase0: 1.25}
	if got := rock.Omega(); got != OrbitOmega(520) {
		t.Errorf("Rock.Omega = %v", got)
	}
	if got := rock.AngleAt(0); got != 1.25 {
		t.Errorf("AngleAt(0) = %v, want Phase0", got)
	}
	p0 := rock.PosAt(0)
	if want := Polar(520, 1.25); p0.Dist(want) > 1e-9 {
		t.Errorf("PosAt(0) = %v, want %v", p0, want)
	}
	// angle = Phase0 + omega * t / 10.
	if got, want := rock.AngleAt(600), 1.25+OrbitOmega(520)*60; math.Abs(got-want) > 1e-12 {
		t.Errorf("AngleAt(600) = %v, want %v", got, want)
	}
	// The orbit is circular and prograde (angle increases).
	for _, tick := range []int{0, 1, 599, 600, 12345, 1000000} {
		if d := rock.PosAt(tick).Len(); math.Abs(d-520) > 1e-9 {
			t.Errorf("PosAt(%d) is %v from Emberfall, want 520", tick, d)
		}
	}
	if !(rock.AngleAt(100) > rock.AngleAt(0)) {
		t.Error("orbit is not prograde")
	}
	// After one full period the rock is back where it started.
	periodTicks := int(math.Round(OrbitPeriod(400) * TicksPerSecond))
	r400 := &Rock{OrbitR: 400, Phase0: 0.3}
	if d := r400.PosAt(periodTicks).Dist(r400.PosAt(0)); d > 1e-6 {
		t.Errorf("rock is %v u from its start after one period", d)
	}
	var nilRock *Rock
	if nilRock.PosAt(5) != (Vec2{}) || nilRock.Omega() != 0 {
		t.Error("nil rock must have zero position and omega")
	}
}

func TestSolarFactor(t *testing.T) {
	tests := []struct {
		r, want float64
	}{
		{520, 1.0},
		{380, math.Pow(520.0/380.0, 1.2)},
		{950, math.Pow(520.0/950.0, 1.2)},
		{100, 1.6},   // clamped high
		{340, 1.6},   // (520/340)^1.2 = 1.665 -> clamped
		{2000, 0.45}, // clamped low
		{0, 1.6},
		{-1, 1.6},
	}
	for _, tc := range tests {
		if got := SolarFactor(tc.r); math.Abs(got-tc.want) > 1e-12 {
			t.Errorf("SolarFactor(%v) = %v, want %v", tc.r, got, tc.want)
		}
	}
	for r := OrbitRMin; r <= OrbitRMax; r += 5 {
		s := SolarFactor(r)
		if s < SolarMin || s > SolarMax {
			t.Errorf("SolarFactor(%v) = %v outside [%v,%v]", r, s, SolarMin, SolarMax)
		}
	}
}
