package main

import (
	"encoding/binary"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"drifthold/internal/audio"
)

func TestWriteWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.wav")
	pcm := []float32{0, 1, -1, 0.5, 2, -2} // three stereo frames; the last one over range
	if err := writeWAV(path, pcm); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 44+len(pcm)*2 {
		t.Fatalf("file is %d bytes, want %d", len(raw), 44+len(pcm)*2)
	}
	u16 := func(off int) int { return int(binary.LittleEndian.Uint16(raw[off:])) }
	u32 := func(off int) int { return int(binary.LittleEndian.Uint32(raw[off:])) }
	header := []struct {
		name      string
		got, want any
	}{
		{"RIFF tag", string(raw[0:4]), "RIFF"},
		{"RIFF size", u32(4), len(raw) - 8},
		{"WAVE tag", string(raw[8:16]), "WAVEfmt "},
		{"fmt size", u32(16), 16},
		{"format", u16(20), 1},
		{"channels", u16(22), 2},
		{"sample rate", u32(24), audio.SampleRate},
		{"byte rate", u32(28), audio.SampleRate * 4},
		{"block align", u16(32), 4},
		{"bits", u16(34), 16},
		{"data tag", string(raw[36:40]), "data"},
		{"data size", u32(40), len(pcm) * 2},
	}
	for _, h := range header {
		if h.got != h.want {
			t.Errorf("%s = %v, want %v", h.name, h.got, h.want)
		}
	}
	want := []int16{0, 32767, -32767, 16384, 32767, -32768}
	for i, w := range want {
		if got := int16(binary.LittleEndian.Uint16(raw[44+2*i:])); got != w {
			t.Errorf("sample %d = %d, want %d", i, got, w)
		}
	}
}

func TestRunWritesEverything(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "out") // run must create it
	if err := run(dir, 0.5, 3, false); err != nil {
		t.Fatal(err)
	}
	check := func(name string, wantFrames int) {
		t.Helper()
		info, err := os.Stat(filepath.Join(dir, name+".wav"))
		if err != nil {
			t.Errorf("missing %s.wav: %v", name, err)
			return
		}
		frames := int(info.Size()-44) / 4
		if wantFrames > 0 && frames != wantFrames {
			t.Errorf("%s.wav holds %d frames, want %d", name, frames, wantFrames)
		}
		if frames < audio.SampleRate/50 {
			t.Errorf("%s.wav is only %d frames long", name, frames)
		}
	}
	for _, s := range audio.AllSfx() {
		check("sfx_"+s.String(), 0)
	}
	for _, m := range audio.AllMoods() {
		check("music_"+m.String(), audio.SampleRate/2)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := audio.NumSfx + audio.NumMoods; len(entries) != want {
		t.Errorf("%d files written, want %d", len(entries), want)
	}
}

func TestWriteSpectrogram(t *testing.T) {
	// A steady 1 kHz tone must draw one bright horizontal line at the height of 1 kHz.
	const frames = audio.SampleRate / 2
	pcm := make([]float32, 2*frames)
	for i := 0; i < frames; i++ {
		v := float32(0.5 * math.Sin(2*math.Pi*1000*float64(i)/audio.SampleRate))
		pcm[2*i], pcm[2*i+1] = v, v
	}
	path := filepath.Join(t.TempDir(), "tone.png")
	if err := writeSpectrogram(path, pcm); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() != 1200 || b.Dy() != 400 {
		t.Fatalf("image is %d×%d, want 1200×400", b.Dx(), b.Dy())
	}
	brightest, at := uint32(0), 0
	for y := 0; y < b.Dy(); y++ {
		r, g, bl, _ := img.At(b.Dx()/2, y).RGBA()
		if sum := r + g + bl; sum > brightest {
			brightest, at = sum, y
		}
	}
	// Rows map logarithmically from 16 kHz (top) to 30 Hz (bottom).
	want := int(400 * (1 - math.Log(1000.0/30)/math.Log(16000.0/30)))
	if at < want-6 || at > want+6 {
		t.Errorf("brightest row is %d, want about %d (1 kHz)", at, want)
	}
	if r, g, bl, _ := img.At(b.Dx()/2, 20).RGBA(); r+g+bl > brightest/4 {
		t.Error("the top of the spectrogram should be dark for a 1 kHz tone")
	}
}

func TestRunRejectsBadArguments(t *testing.T) {
	for _, seconds := range []float64{0, -1, math.NaN()} {
		if err := run(t.TempDir(), seconds, 1, false); err == nil {
			t.Errorf("run with seconds = %v succeeded, want an error", seconds)
		}
	}
	// A path that cannot be a directory.
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(file, 1, 1, false); err == nil {
		t.Error("run into a regular file succeeded, want an error")
	}
}

func TestHelpers(t *testing.T) {
	peak, rms := levels([]float32{0.5, -0.5, 0.5, -0.5})
	if math.Abs(peak+6.02) > 0.01 || math.Abs(rms+6.02) > 0.01 {
		t.Errorf("levels = %.2f, %.2f; want -6.02, -6.02", peak, rms)
	}
	if peak, rms := levels(nil); peak != -200 || rms != -200 {
		t.Errorf("levels of nothing = %v, %v; want -200, -200", peak, rms)
	}
	// The FFT of a pure cosine has all its energy in one bin (and its mirror).
	const n = 64
	x := make([]complex128, n)
	for i := range x {
		x[i] = complex(math.Cos(2*math.Pi*5*float64(i)/n), 0)
	}
	fft(x)
	for k := range x {
		mag := math.Hypot(real(x[k]), imag(x[k]))
		want := 0.0
		if k == 5 || k == n-5 {
			want = n / 2
		}
		if math.Abs(mag-want) > 1e-9 {
			t.Errorf("bin %d: magnitude %v, want %v", k, mag, want)
		}
	}
	if lo, hi := heat(-1), heat(2); lo != heat(0) || hi != heat(1) {
		t.Error("heat does not clamp its input")
	}
	if a, b := heat(0), heat(1); a.R >= b.R || a.A != 255 || b.A != 255 {
		t.Errorf("heat ramp: %v .. %v", a, b)
	}
}
