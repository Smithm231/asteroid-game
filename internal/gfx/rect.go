package gfx

// Pt is a point (or a vector) in points.
type Pt struct{ X, Y float32 }

// P returns the point (x, y). Like R, it spares packages outside gfx from
// writing keyed struct literals.
func P(x, y float32) Pt { return Pt{x, y} }

// Rect is an axis-aligned rectangle in points: its top-left corner and its
// size. The zero Rect is empty.
//
// Layout follows the rect-cut idiom: start from the area to fill and slice
// pieces off its edges with the Cut methods, which shrink the receiver and
// return the piece that was removed.
//
//	body := panel.Inset(th.Pad)
//	title := body.CutTop(th.RowH) // body is now what is left below the title
//	body.CutTop(th.Gap)
//	icon := title.CutLeft(th.RowH)
type Rect struct{ X, Y, W, H float32 }

// R returns the rectangle with top-left corner (x, y) and size w × h.
func R(x, y, w, h float32) Rect { return Rect{x, y, w, h} }

// RectLTRB returns the rectangle spanning the edges left, top, right and
// bottom. Edges given in the wrong order are swapped.
func RectLTRB(left, top, right, bottom float32) Rect {
	if right < left {
		left, right = right, left
	}
	if bottom < top {
		top, bottom = bottom, top
	}
	return Rect{left, top, right - left, bottom - top}
}

// Right returns the x coordinate of the right edge.
func (r Rect) Right() float32 { return r.X + r.W }

// Bottom returns the y coordinate of the bottom edge.
func (r Rect) Bottom() float32 { return r.Y + r.H }

// MidX returns the x coordinate of the centre.
func (r Rect) MidX() float32 { return r.X + r.W/2 }

// MidY returns the y coordinate of the centre.
func (r Rect) MidY() float32 { return r.Y + r.H/2 }

// Mid returns the centre point.
func (r Rect) Mid() Pt { return Pt{r.X + r.W/2, r.Y + r.H/2} }

// Empty reports whether the rectangle has no area.
func (r Rect) Empty() bool { return !(r.W > 0 && r.H > 0) }

// Contains reports whether the point (x, y) lies inside the rectangle. The
// left and top edges are inside, the right and bottom edges are not, so
// adjacent rectangles never both contain a point.
func (r Rect) Contains(x, y float32) bool {
	return x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

// Overlaps reports whether r and o share any area.
func (r Rect) Overlaps(o Rect) bool {
	return !r.Empty() && !o.Empty() &&
		r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}

// Intersect returns the area common to r and o, or the zero Rect if they do
// not overlap.
func (r Rect) Intersect(o Rect) Rect {
	x0, y0 := max(r.X, o.X), max(r.Y, o.Y)
	x1, y1 := min(r.X+r.W, o.X+o.W), min(r.Y+r.H, o.Y+o.H)
	if !(x1 > x0 && y1 > y0) {
		return Rect{}
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Union returns the smallest rectangle containing both r and o. An empty
// rectangle contributes nothing.
func (r Rect) Union(o Rect) Rect {
	if r.Empty() {
		return o
	}
	if o.Empty() {
		return r
	}
	x0, y0 := min(r.X, o.X), min(r.Y, o.Y)
	x1, y1 := max(r.X+r.W, o.X+o.W), max(r.Y+r.H, o.Y+o.H)
	return Rect{x0, y0, x1 - x0, y1 - y0}
}

// Translate returns r moved by (dx, dy).
func (r Rect) Translate(dx, dy float32) Rect {
	return Rect{r.X + dx, r.Y + dy, r.W, r.H}
}

// Inset returns r shrunk by d on every side. A negative d grows it. The
// result never has a negative size: a rectangle that is too small collapses
// onto its centre line.
func (r Rect) Inset(d float32) Rect { return r.Pad(d, d, d, d) }

// InsetXY returns r shrunk by dx on the left and right and by dy on the top
// and bottom.
func (r Rect) InsetXY(dx, dy float32) Rect { return r.Pad(dx, dy, dx, dy) }

// Expand returns r grown by d on every side; it is Inset(-d).
func (r Rect) Expand(d float32) Rect { return r.Pad(-d, -d, -d, -d) }

// Pad returns r shrunk by the given amount on each side (negative amounts
// grow that side). If the paddings exceed the size along an axis, the result
// has zero size along it and sits where the two padded edges would meet in
// proportion to the paddings.
func (r Rect) Pad(left, top, right, bottom float32) Rect {
	x, w := padAxis(r.X, r.W, left, right)
	y, h := padAxis(r.Y, r.H, top, bottom)
	return Rect{x, y, w, h}
}

func padAxis(pos, size, lo, hi float32) (float32, float32) {
	if rest := size - lo - hi; rest >= 0 {
		return pos + lo, rest
	}
	// Over-padded: collapse to the point that splits the size in the ratio
	// of the two paddings.
	if sum := lo + hi; sum > 0 {
		return pos + size*lo/sum, 0
	}
	return pos + size/2, 0
}

// Center returns the rectangle of size w × h centred on r. It may be larger
// than r.
func (r Rect) Center(w, h float32) Rect {
	return Rect{r.X + (r.W-w)/2, r.Y + (r.H-h)/2, w, h}
}

// CutTop removes a strip of height n from the top of r and returns it. n is
// clamped to the range 0..r.H.
func (r *Rect) CutTop(n float32) Rect {
	n = clampf(n, 0, max(0, r.H))
	out := Rect{r.X, r.Y, r.W, n}
	r.Y += n
	r.H -= n
	return out
}

// CutBottom removes a strip of height n from the bottom of r and returns it.
// n is clamped to the range 0..r.H.
func (r *Rect) CutBottom(n float32) Rect {
	n = clampf(n, 0, max(0, r.H))
	r.H -= n
	return Rect{r.X, r.Y + r.H, r.W, n}
}

// CutLeft removes a strip of width n from the left of r and returns it. n is
// clamped to the range 0..r.W.
func (r *Rect) CutLeft(n float32) Rect {
	n = clampf(n, 0, max(0, r.W))
	out := Rect{r.X, r.Y, n, r.H}
	r.X += n
	r.W -= n
	return out
}

// CutRight removes a strip of width n from the right of r and returns it. n
// is clamped to the range 0..r.W.
func (r *Rect) CutRight(n float32) Rect {
	n = clampf(n, 0, max(0, r.W))
	r.W -= n
	return Rect{r.X + r.W, r.Y, n, r.H}
}

// Col returns column i of n equal-width columns laid side by side across r,
// left to right, separated by gap. It allocates nothing. It returns the zero
// Rect if n < 1 or i is out of range.
func (r Rect) Col(i, n int, gap float32) Rect {
	if n < 1 || i < 0 || i >= n {
		return Rect{}
	}
	w := max(0, (r.W-gap*float32(n-1))/float32(n))
	return Rect{r.X + float32(i)*(w+gap), r.Y, w, r.H}
}

// Row returns row i of n equal-height rows stacked down r, top to bottom,
// separated by gap. It allocates nothing. It returns the zero Rect if n < 1
// or i is out of range.
func (r Rect) Row(i, n int, gap float32) Rect {
	if n < 1 || i < 0 || i >= n {
		return Rect{}
	}
	h := max(0, (r.H-gap*float32(n-1))/float32(n))
	return Rect{r.X, r.Y + float32(i)*(h+gap), r.W, h}
}

// Cell returns the cell at column col, row row of a cols × rows grid laid
// over r with gap between cells. It allocates nothing. It returns the zero
// Rect if the grid is empty or the cell is out of range.
func (r Rect) Cell(col, row, cols, rows int, gap float32) Rect {
	if cols < 1 || rows < 1 || col < 0 || col >= cols || row < 0 || row >= rows {
		return Rect{}
	}
	c := r.Col(col, cols, gap)
	rr := r.Row(row, rows, gap)
	return Rect{c.X, rr.Y, c.W, rr.H}
}

// SplitH splits r into n equal columns side by side, left to right, with no
// gap. It returns nil if n < 1. Use Col to avoid the allocation.
func (r Rect) SplitH(n int) []Rect {
	if n < 1 {
		return nil
	}
	out := make([]Rect, n)
	for i := range out {
		out[i] = r.Col(i, n, 0)
	}
	return out
}

// SplitV splits r into n equal rows stacked top to bottom, with no gap. It
// returns nil if n < 1. Use Row to avoid the allocation.
func (r Rect) SplitV(n int) []Rect {
	if n < 1 {
		return nil
	}
	out := make([]Rect, n)
	for i := range out {
		out[i] = r.Row(i, n, 0)
	}
	return out
}

// Grid splits r into cols × rows equal cells separated by gap and returns
// them row by row (index = row*cols + col). It returns nil if cols or rows
// is less than 1. Use Cell to avoid the allocation.
func (r Rect) Grid(cols, rows int, gap float32) []Rect {
	if cols < 1 || rows < 1 {
		return nil
	}
	out := make([]Rect, 0, cols*rows)
	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			out = append(out, r.Cell(col, row, cols, rows, gap))
		}
	}
	return out
}

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
