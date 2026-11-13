package uitest

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShotPath(t *testing.T) {
	out := filepath.Join("some", "out")
	tests := []struct {
		name    string
		want    string // relative to out, slash-separated; "" for an error
		wantErr string
	}{
		{"name.png", "name.png", ""},
		{"name", "name.png", ""},
		{"NAME.PNG", "NAME.PNG", ""},
		{"01_idle", "01_idle.png", ""},
		{"v1.2", "v1.2.png", ""},
		{"panel/open.png", "panel/open.png", ""},
		{"panel/open", "panel/open.png", ""},
		{"  padded.png  ", "padded.png", ""},
		{"", "", "empty"},
		{"   ", "", "empty"},
		{"../escape.png", "", "inside the output directory"},
		{"a/../../escape", "", "inside the output directory"},
		{"/abs/shot.png", "", "inside the output directory"},
	}
	for _, tc := range tests {
		got, err := shotPath(out, tc.name)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("shotPath(%q) = %q, %v; want an error mentioning %q", tc.name, got, err, tc.wantErr)
			}
			continue
		}
		if err != nil {
			t.Errorf("shotPath(%q): %v", tc.name, err)
			continue
		}
		if want := filepath.Join(out, filepath.FromSlash(tc.want)); got != want {
			t.Errorf("shotPath(%q) = %q, want %q", tc.name, got, want)
		}
	}
}

// testPixels returns a w×h premultiplied RGBA buffer with a recognisable
// pattern: an opaque gradient, one half-transparent and one fully transparent
// pixel.
func testPixels(w, h int) []byte {
	pix := make([]byte, 4*w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := 4 * (y*w + x)
			pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(10*x), byte(20*y), byte(x+y), 0xff
		}
	}
	// (1,0): half-transparent red, premultiplied. (2,0): untouched screen.
	copy(pix[4:8], []byte{0x80, 0, 0, 0x80})
	copy(pix[8:12], []byte{0, 0, 0, 0})
	return pix
}

func TestWriteShot(t *testing.T) {
	dir := t.TempDir()
	const w, h = 7, 5
	path, err := writeShot(dir, "sub/dir/shot", testPixels(w, h), w, h)
	if err != nil {
		t.Fatalf("writeShot: %v", err)
	}
	if want := filepath.Join(dir, "sub", "dir", "shot.png"); path != want {
		t.Errorf("writeShot reported %q, want %q", path, want)
	}

	f, err := os.Open(filepath.Join(dir, "sub", "dir", "shot.png"))
	if err != nil {
		t.Fatalf("the screenshot was not written where expected: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("the screenshot is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != w || b.Dy() != h {
		t.Fatalf("size = %d×%d, want %d×%d", b.Dx(), b.Dy(), w, h)
	}

	at := func(x, y int) color.RGBA {
		r, g, b, a := img.At(x, y).RGBA()
		return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
	}
	tests := []struct {
		x, y int
		want color.RGBA
	}{
		{0, 0, color.RGBA{0, 0, 0, 0xff}},
		{3, 2, color.RGBA{30, 40, 5, 0xff}},
		{6, 4, color.RGBA{60, 80, 10, 0xff}},
		// Translucent and untouched pixels are flattened onto black, as the
		// window shows them.
		{1, 0, color.RGBA{0x80, 0, 0, 0xff}},
		{2, 0, color.RGBA{0, 0, 0, 0xff}},
	}
	for _, tc := range tests {
		if got := at(tc.x, tc.y); got != tc.want {
			t.Errorf("pixel (%d,%d) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestWriteShotOverwrites(t *testing.T) {
	dir := t.TempDir()
	if _, err := writeShot(dir, "a.png", testPixels(4, 4), 4, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := writeShot(dir, "a.png", testPixels(9, 3), 9, 3); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "a.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 9 || cfg.Height != 3 {
		t.Errorf("second shot is %d×%d, want 9×3", cfg.Width, cfg.Height)
	}
}

func TestWriteShotErrors(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		shot    string
		pix     []byte
		w, h    int
		wantErr string
	}{
		{"short buffer", "a.png", make([]byte, 10), 4, 4, "buffer"},
		{"zero size", "a.png", nil, 0, 0, "buffer"},
		{"escaping name", "../a.png", testPixels(2, 2), 2, 2, "inside the output directory"},
		{"empty name", "", testPixels(2, 2), 2, 2, "empty"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path, err := writeShot(dir, tc.shot, tc.pix, tc.w, tc.h)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("writeShot error = %v, want one mentioning %q", err, tc.wantErr)
			}
			if path != "" {
				t.Errorf("writeShot reported path %q alongside an error", path)
			}
		})
	}

	// A file where a directory is needed.
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeShot(blocker, "a.png", testPixels(2, 2), 2, 2); err == nil {
		t.Error("writeShot into a path below a regular file should fail")
	}
}
