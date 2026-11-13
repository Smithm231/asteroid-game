package emblem

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"slices"
	"sync"
	"testing"
)

// testSizes covers every shipped icon size plus the extremes and a few odd
// sizes that exercise the pixel snapping.
func testSizes() []int {
	return append(Sizes(), MinSize, 17, 31, 33, 63, 96, 100, 512, MaxSize)
}

func TestSizes(t *testing.T) {
	want := []int{16, 20, 24, 32, 40, 48, 64, 128, 256}
	got := Sizes()
	if !slices.Equal(got, want) {
		t.Fatalf("Sizes() = %v, want %v", got, want)
	}
	// The caller owns the returned slice: changing it must not affect
	// later calls.
	got[0] = 999
	if again := Sizes(); !slices.Equal(again, want) {
		t.Fatalf("Sizes() after mutation = %v, want %v", again, want)
	}
}

func TestRenderBounds(t *testing.T) {
	for _, size := range testSizes() {
		img := Render(size)
		if want := image.Rect(0, 0, size, size); img.Bounds() != want {
			t.Errorf("Render(%d).Bounds() = %v, want %v", size, img.Bounds(), want)
		}
		if img.Stride != 4*size || len(img.Pix) != 4*size*size {
			t.Errorf("Render(%d): stride %d, %d bytes of pixels", size, img.Stride, len(img.Pix))
		}
	}
}

func TestRenderClampsSize(t *testing.T) {
	for _, tc := range []struct{ size, want int }{
		{-5, MinSize}, {0, MinSize}, {15, MinSize}, {16, 16}, {1024, 1024}, {1025, MaxSize}, {1 << 20, MaxSize},
	} {
		if got := Render(tc.size).Bounds().Dx(); got != tc.want {
			t.Errorf("Render(%d) is %d px wide, want %d", tc.size, got, tc.want)
		}
	}
}

func TestRenderDeterministic(t *testing.T) {
	for _, size := range testSizes() {
		a, b := Render(size), Render(size)
		if !bytes.Equal(a.Pix, b.Pix) {
			t.Errorf("Render(%d) produced different pixels on two calls", size)
		}
	}
}

func TestRenderConcurrent(t *testing.T) {
	want := Render(48).Pix
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !bytes.Equal(Render(48).Pix, want) {
				t.Error("concurrent Render(48) differs from the sequential result")
			}
		}()
	}
	wg.Wait()
}

func TestRenderAlpha(t *testing.T) {
	for _, size := range testSizes() {
		img := Render(size)
		last := size - 1
		for _, p := range []image.Point{{0, 0}, {last, 0}, {0, last}, {last, last}} {
			if c := img.RGBAAt(p.X, p.Y); c != (color.RGBA{}) {
				t.Errorf("Render(%d): corner %v = %v, want fully transparent", size, p, c)
			}
		}
		if a := img.RGBAAt(size/2, size/2).A; a != 0xFF {
			t.Errorf("Render(%d): centre alpha = %d, want 255", size, a)
		}

		var opaque, partial int
		for i := 0; i < len(img.Pix); i += 4 {
			r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
			if r > a || g > a || b > a {
				t.Fatalf("Render(%d): pixel %d (%d,%d,%d,%d) is not premultiplied", size, i/4, r, g, b, a)
			}
			switch {
			case a == 0xFF:
				opaque++
			case a > 0:
				partial++
			}
		}
		// A regular pointy-top hexagon inscribed in the square covers
		// 3*sqrt(3)/8 = 65% of it.
		cover := float64(opaque) / float64(size*size)
		if cover < 0.52 || cover > 0.68 {
			t.Errorf("Render(%d): %.0f%% of the pixels are opaque, want roughly 65%%", size, cover*100)
		}
		if partial == 0 {
			t.Errorf("Render(%d): no partially transparent pixels, edges are not anti-aliased", size)
		}
	}
}

// near reports whether an opaque pixel is within tol of the given colour on
// every channel.
func near(c color.RGBA, want rgb, tol int) bool {
	if c.A != 0xFF {
		return false
	}
	d := func(got uint8, w float64) bool {
		diff := int(got) - int(w*255+0.5)
		return diff >= -tol && diff <= tol
	}
	return d(c.R, want.r) && d(c.G, want.g) && d(c.B, want.b)
}

// count returns the number of pixels of img for which match returns true.
func count(img *image.RGBA, match func(color.RGBA) bool) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if match(img.RGBAAt(x, y)) {
				n++
			}
		}
	}
	return n
}

func TestRenderPalette(t *testing.T) {
	for _, size := range testSizes() {
		img := Render(size)
		total := size * size

		// Amber: the rim and the keystone's front face.
		amberish := count(img, func(c color.RGBA) bool { return near(c, amber, 28) })
		if amberish < total/20 {
			t.Errorf("Render(%d): only %d of %d pixels are amber", size, amberish, total)
		}

		// Ink: the field must stay dark. (At 16 px the rim and keystone
		// leave it less than a fifth of the image.)
		dark := count(img, func(c color.RGBA) bool {
			return c.A == 0xFF && c.R < 0x40 && c.G < 0x48 && c.B < 0x60
		})
		if dark < total/7 {
			t.Errorf("Render(%d): only %d of %d pixels are ink-dark", size, dark, total)
		}

		// Cream: the orbit and the drifting rock, dropped below 32 px.
		creamy := count(img, func(c color.RGBA) bool { return near(c, cream, 20) })
		switch {
		case size < 32 && creamy != 0:
			t.Errorf("Render(%d): %d cream pixels, want none (orbit is dropped below 32 px)", size, creamy)
		case size >= 32 && creamy == 0:
			t.Errorf("Render(%d): no cream pixels, want the orbit dot", size)
		}
	}
}

// TestRenderKeystone checks the composition: the keystone's amber front face
// sits at the centre of the badge above a darker rock, with ink field on
// either side of it.
func TestRenderKeystone(t *testing.T) {
	for _, size := range testSizes() {
		img := Render(size)
		at := func(x, y float64) color.RGBA {
			return img.RGBAAt(int(x*float64(size)), int(y*float64(size)))
		}
		if c := at(0.47, 0.45); !(c.R > 0xC0 && c.G > 0x70 && c.B < 0x90) {
			t.Errorf("Render(%d): keystone face = %v, want amber", size, c)
		}
		for _, x := range []float64{0.20, 0.80} {
			if c := at(x, 0.45); !(c.A == 0xFF && c.R < 0x50 && c.G < 0x50 && c.B < 0x60) {
				t.Errorf("Render(%d): field beside the keystone at x=%.2f = %v, want ink-dark", size, x, c)
			}
		}
		if c := at(0.50, 0.71); !(c.A == 0xFF && c.R < 0x70 && c.B > c.R && c.B >= 0x28) {
			t.Errorf("Render(%d): rock = %v, want a dark slate tone", size, c)
		}
	}
}

func TestHexagonSnapsToPixels(t *testing.T) {
	for _, size := range testSizes() {
		l := newLayout(size)
		h := l.hexagon()
		for _, i := range []int{1, 2, 4, 5} {
			x := h[i].x * float64(size)
			if math.Abs(x-math.Round(x)) > 1e-9 {
				t.Errorf("size %d: hexagon side vertex %d at x=%v px, want a whole pixel", size, i, x)
			}
		}
		if h[0].y < 0 || h[3].y > 1 {
			t.Errorf("size %d: hexagon exceeds the image vertically: %v..%v", size, h[0].y, h[3].y)
		}
	}
}

func TestInsetAndChamfer(t *testing.T) {
	square := []pt{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	in := inset(square, 0.25)
	want := []pt{{0.25, 0.25}, {0.75, 0.25}, {0.75, 0.75}, {0.25, 0.75}}
	for i := range want {
		if in[i].sub(want[i]).length() > 1e-9 {
			t.Fatalf("inset(square, 0.25) = %v, want %v", in, want)
		}
	}
	// Winding must not matter.
	in = inset(reversed(square), 0.25)
	for _, v := range in {
		if v.x < 0.25-1e-9 || v.x > 0.75+1e-9 || v.y < 0.25-1e-9 || v.y > 0.75+1e-9 {
			t.Fatalf("inset of a counter-clockwise square grew outwards: %v", in)
		}
	}

	ch := chamfer(square, 0.1)
	if len(ch) != 8 {
		t.Fatalf("chamfer(square) has %d vertices, want 8", len(ch))
	}
	if ch[0].sub(pt{0, 0.1}).length() > 1e-9 || ch[1].sub(pt{0.1, 0}).length() > 1e-9 {
		t.Fatalf("chamfer(square, 0.1) starts %v %v, want (0,0.1) (0.1,0)", ch[0], ch[1])
	}
	if got := chamfer(square, 0); len(got) != 4 {
		t.Fatalf("chamfer with zero length changed the polygon: %v", got)
	}
}

func BenchmarkRender256(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Render(256)
	}
}
