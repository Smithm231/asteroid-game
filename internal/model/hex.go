package model

import (
	"math"
	"strconv"
)

// Hex is an axial coordinate on a pointy-top hex grid. Every rock has its own
// grid with (0,0) at the centre. The implied third cube coordinate is
// S = -Q-R.
type Hex struct {
	Q int // column axis (grows to the east)
	R int // row axis (grows to the south-east)
}

// hexDirs lists the six neighbour offsets in the canonical order.
var hexDirs = [6]Hex{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}

// HexDirections returns the six neighbour offsets in the canonical order
// (+1,0), (+1,-1), (0,-1), (-1,0), (-1,+1), (0,+1): east, north-east,
// north-west, west, south-west, south-east on a pointy-top grid drawn with +Y
// down.
func HexDirections() [6]Hex { return hexDirs }

// HexDir returns neighbour offset number dir; dir is taken modulo 6, so
// negative and large values wrap around.
func HexDir(dir int) Hex { return hexDirs[((dir%6)+6)%6] }

// Add returns h + o.
func (h Hex) Add(o Hex) Hex { return Hex{h.Q + o.Q, h.R + o.R} }

// Sub returns h - o.
func (h Hex) Sub(o Hex) Hex { return Hex{h.Q - o.Q, h.R - o.R} }

// Scale returns h with both coordinates multiplied by k.
func (h Hex) Scale(k int) Hex { return Hex{h.Q * k, h.R * k} }

// S returns the third cube coordinate, -Q-R.
func (h Hex) S() int { return -h.Q - h.R }

// Neighbor returns the adjacent hex in direction dir (taken modulo 6; see
// HexDirections for the order).
func (h Hex) Neighbor(dir int) Hex { return h.Add(HexDir(dir)) }

// Neighbors returns the six adjacent hexes in the canonical direction order.
func (h Hex) Neighbors() [6]Hex {
	var n [6]Hex
	for i, d := range hexDirs {
		n[i] = Hex{h.Q + d.Q, h.R + d.R}
	}
	return n
}

// Len returns the hex distance from the origin (0,0) to h.
func (h Hex) Len() int {
	return (absInt(h.Q) + absInt(h.R) + absInt(h.Q+h.R)) / 2
}

// Dist returns the hex distance between h and o:
// (|dq| + |dr| + |dq+dr|) / 2.
func (h Hex) Dist(o Hex) int { return h.Sub(o).Len() }

// Adjacent reports whether h and o are neighbours (distance exactly 1).
func (h Hex) Adjacent(o Hex) bool { return h.Dist(o) == 1 }

// String formats the hex as "(q,r)".
func (h Hex) String() string {
	return "(" + strconv.Itoa(h.Q) + "," + strconv.Itoa(h.R) + ")"
}

// Ring returns the hexes at distance exactly radius from h. The order is
// deterministic: it starts at h + radius*(-1,+1) and walks the ring through
// the six canonical directions. Ring(0) is just h; a negative radius yields
// nil. A ring of radius r > 0 has 6*r hexes.
func (h Hex) Ring(radius int) []Hex {
	if radius < 0 {
		return nil
	}
	if radius == 0 {
		return []Hex{h}
	}
	out := make([]Hex, 0, 6*radius)
	cur := h.Add(hexDirs[4].Scale(radius))
	for d := 0; d < 6; d++ {
		for i := 0; i < radius; i++ {
			out = append(out, cur)
			cur = cur.Add(hexDirs[d])
		}
	}
	return out
}

// Spiral returns every hex within radius of h ordered by increasing
// distance: h itself, then Ring(1), Ring(2), ... each in Ring order. Use it
// for "nearest first" searches. A negative radius yields nil.
func (h Hex) Spiral(radius int) []Hex {
	if radius < 0 {
		return nil
	}
	out := make([]Hex, 0, HexCount(radius))
	out = append(out, h)
	for r := 1; r <= radius; r++ {
		cur := h.Add(hexDirs[4].Scale(r))
		for d := 0; d < 6; d++ {
			for i := 0; i < r; i++ {
				out = append(out, cur)
				cur = cur.Add(hexDirs[d])
			}
		}
	}
	return out
}

// WithinRadius returns every hex within radius of h in row-major order
// (increasing R, then increasing Q). For h = (0,0) this is exactly the order
// of increasing Rock tile index. A negative radius yields nil.
func (h Hex) WithinRadius(radius int) []Hex {
	if radius < 0 {
		return nil
	}
	out := make([]Hex, 0, HexCount(radius))
	for dr := -radius; dr <= radius; dr++ {
		lo := max(-radius, -dr-radius)
		hi := min(radius, -dr+radius)
		for dq := lo; dq <= hi; dq++ {
			out = append(out, Hex{h.Q + dq, h.R + dr})
		}
	}
	return out
}

// HexCount returns the number of hexes within radius of a centre hex:
// 3*r*(r+1)+1, or 0 for a negative radius.
func HexCount(radius int) int {
	if radius < 0 {
		return 0
	}
	return 3*radius*(radius+1) + 1
}

// sqrt3 is the square root of three, used by the pointy-top layout.
const sqrt3 = 1.7320508075688772935274463415058723669428052538103806

// HexToPixel returns the centre of hex h on a pointy-top layout where size is
// the distance from a hex centre to any of its corners. Hex (0,0) is at the
// origin, +Q points along +X and +R points down-right:
//
//	x = size * sqrt(3) * (q + r/2)
//	y = size * 3/2 * r
func HexToPixel(h Hex, size float64) Vec2 {
	q, r := float64(h.Q), float64(h.R)
	return Vec2{
		X: size * sqrt3 * (q + r/2),
		Y: size * 1.5 * r,
	}
}

// PixelToHex returns the hex containing point p for the layout used by
// HexToPixel (same size and origin), using cube rounding. A size that is not
// positive yields (0,0).
func PixelToHex(p Vec2, size float64) Hex {
	if !(size > 0) {
		return Hex{}
	}
	q := (sqrt3/3*p.X - p.Y/3) / size
	r := (2.0 / 3.0 * p.Y) / size
	return HexRound(q, r)
}

// HexRound rounds fractional axial coordinates to the nearest hex using cube
// rounding (the coordinate with the largest rounding error is recomputed from
// the other two so that q+r+s stays 0).
func HexRound(q, r float64) Hex {
	s := -q - r
	rq, rr, rs := math.Round(q), math.Round(r), math.Round(s)
	dq, dr, ds := math.Abs(rq-q), math.Abs(rr-r), math.Abs(rs-s)
	switch {
	case dq > dr && dq > ds:
		rq = -rr - rs
	case dr > ds:
		rr = -rq - rs
	}
	return Hex{Q: int(rq), R: int(rr)}
}

// HexCorners returns the six corners of a pointy-top hex centred at center
// with the given size (centre-to-corner distance). Corner 0 is the upper-right
// one (at -30 degrees with +Y down) and the rest follow clockwise on screen:
// lower-right, bottom, lower-left, upper-left, top.
func HexCorners(center Vec2, size float64) [6]Vec2 {
	w := size * sqrt3 / 2
	h := size / 2
	return [6]Vec2{
		{center.X + w, center.Y - h},
		{center.X + w, center.Y + h},
		{center.X, center.Y + size},
		{center.X - w, center.Y + h},
		{center.X - w, center.Y - h},
		{center.X, center.Y - size},
	}
}

// absInt returns the absolute value of x.
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
