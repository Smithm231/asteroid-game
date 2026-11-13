// Command wavdump renders every sound effect and a stretch of every music mood to WAV files,
// for listening to and inspecting the synthesiser's output outside the game. It needs no audio
// device.
//
//	go run ./internal/audio/wavdump -out /tmp/drifthold-audio
//	go run ./internal/audio/wavdump -out /tmp/drifthold-audio -seconds 60 -seed 7 -spectrograms
//
// Files written: sfx_<Name>.wav for each effect and music_<Mood>.wav for each mood (16-bit
// stereo, 44.1 kHz). With -spectrograms a PNG spectrogram is written next to each WAV: time
// runs left to right, frequency bottom to top on a logarithmic scale from 30 Hz to 16 kHz, and
// brightness shows level over a 72 dB range.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/cmplx"
	"os"
	"path/filepath"

	"drifthold/internal/audio"
)

func main() {
	out := flag.String("out", "", "directory to write the WAV files to (required; created if missing)")
	seconds := flag.Float64("seconds", 30, "length of each music render in seconds")
	seed := flag.Uint64("seed", 1, "seed of the music generator")
	spectro := flag.Bool("spectrograms", false, "also write a PNG spectrogram for every file")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "wavdump: -out is required")
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*out, *seconds, *seed, *spectro); err != nil {
		fmt.Fprintln(os.Stderr, "wavdump:", err)
		os.Exit(1)
	}
}

func run(dir string, seconds float64, seed uint64, spectro bool) error {
	if !(seconds > 0) {
		return fmt.Errorf("-seconds must be positive, got %v", seconds)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	write := func(name string, pcm []float32) error {
		path := filepath.Join(dir, name+".wav")
		if err := writeWAV(path, pcm); err != nil {
			return err
		}
		peak, rms := levels(pcm)
		fmt.Printf("%-28s %6.2f s  peak %5.1f dBFS  rms %5.1f dBFS\n",
			name+".wav", float64(len(pcm)/2)/audio.SampleRate, peak, rms)
		if spectro {
			return writeSpectrogram(filepath.Join(dir, name+".png"), pcm)
		}
		return nil
	}
	for _, s := range audio.AllSfx() {
		if err := write("sfx_"+s.String(), audio.RenderSfx(s)); err != nil {
			return err
		}
	}
	for _, m := range audio.AllMoods() {
		if err := write("music_"+m.String(), audio.RenderMusic(m, seconds, seed)); err != nil {
			return err
		}
	}
	return nil
}

// levels returns the peak and RMS level of interleaved PCM in dBFS.
func levels(pcm []float32) (peak, rms float64) {
	sum := 0.0
	for _, v := range pcm {
		a := math.Abs(float64(v))
		peak = math.Max(peak, a)
		sum += a * a
	}
	if len(pcm) > 0 {
		rms = math.Sqrt(sum / float64(len(pcm)))
	}
	db := func(x float64) float64 {
		if x < 1e-10 {
			return -200
		}
		return 20 * math.Log10(x)
	}
	return db(peak), db(rms)
}

// writeWAV stores interleaved stereo float PCM as a 16-bit stereo RIFF/WAVE file.
func writeWAV(path string, pcm []float32) error {
	const (
		channels      = 2
		bitsPerSample = 16
		blockAlign    = channels * bitsPerSample / 8
	)
	dataLen := len(pcm) * 2
	buf := make([]byte, 44+dataLen)
	copy(buf[0:], "RIFF")
	binary.LittleEndian.PutUint32(buf[4:], uint32(36+dataLen))
	copy(buf[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(buf[16:], 16) // fmt chunk size
	binary.LittleEndian.PutUint16(buf[20:], 1)  // PCM
	binary.LittleEndian.PutUint16(buf[22:], channels)
	binary.LittleEndian.PutUint32(buf[24:], audio.SampleRate)
	binary.LittleEndian.PutUint32(buf[28:], audio.SampleRate*blockAlign)
	binary.LittleEndian.PutUint16(buf[32:], blockAlign)
	binary.LittleEndian.PutUint16(buf[34:], bitsPerSample)
	copy(buf[36:], "data")
	binary.LittleEndian.PutUint32(buf[40:], uint32(dataLen))
	for i, v := range pcm {
		s := math.Round(float64(v) * 32767)
		s = math.Max(-32768, math.Min(32767, s))
		binary.LittleEndian.PutUint16(buf[44+2*i:], uint16(int16(s)))
	}
	return os.WriteFile(path, buf, 0o644)
}

// writeSpectrogram draws a log-frequency spectrogram of the mono sum of pcm.
func writeSpectrogram(path string, pcm []float32) error {
	const (
		fftSize = 4096
		width   = 1200
		height  = 400
		loHz    = 30.0
		hiHz    = 16000.0
		rangeDB = 72.0
	)
	frames := len(pcm) / 2
	mono := make([]float64, frames)
	for i := range mono {
		mono[i] = (float64(pcm[2*i]) + float64(pcm[2*i+1])) / 2
	}
	window := make([]float64, fftSize)
	for i := range window {
		window[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/fftSize)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	buf := make([]complex128, fftSize)
	mags := make([]float64, fftSize/2)
	for x := 0; x < width; x++ {
		centre := 0
		if width > 1 {
			centre = x * (frames - 1) / (width - 1)
		}
		for i := range buf {
			j := centre - fftSize/2 + i
			v := 0.0
			if j >= 0 && j < frames {
				v = mono[j]
			}
			buf[i] = complex(v*window[i], 0)
		}
		fft(buf)
		for k := range mags {
			mags[k] = cmplx.Abs(buf[k]) / (fftSize / 4) // a full-scale sine reads 0 dB
		}
		for y := 0; y < height; y++ {
			// Each pixel row spans a band of frequencies; show the strongest bin inside it.
			f0 := loHz * math.Pow(hiHz/loHz, float64(height-1-y)/float64(height))
			f1 := loHz * math.Pow(hiHz/loHz, float64(height-y)/float64(height))
			k0 := int(f0 * fftSize / audio.SampleRate)
			k1 := int(f1*fftSize/audio.SampleRate) + 1
			peak := 0.0
			for k := k0; k < k1 && k < len(mags); k++ {
				peak = math.Max(peak, mags[k])
			}
			level := 0.0
			if peak > 0 {
				level = (20*math.Log10(peak) + rangeDB) / rangeDB
			}
			img.SetRGBA(x, y, heat(level))
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// heat maps a level in 0..1 to a dark-blue → amber → white ramp.
func heat(v float64) color.RGBA {
	v = math.Max(0, math.Min(1, v))
	stops := [...][3]float64{{8, 10, 24}, {40, 50, 120}, {200, 90, 60}, {240, 180, 70}, {255, 250, 235}}
	pos := v * float64(len(stops)-1)
	i := int(pos)
	if i >= len(stops)-1 {
		i = len(stops) - 2
	}
	t := pos - float64(i)
	mix := func(c int) uint8 { return uint8(stops[i][c] + (stops[i+1][c]-stops[i][c])*t) }
	return color.RGBA{R: mix(0), G: mix(1), B: mix(2), A: 255}
}

// fft computes the discrete Fourier transform of x in place; len(x) must be a power of two.
func fft(x []complex128) {
	n := len(x)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			x[i], x[j] = x[j], x[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		step := cmplx.Rect(1, -2*math.Pi/float64(size))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := 0; k < size/2; k++ {
				a := x[start+k]
				b := x[start+k+size/2] * w
				x[start+k] = a + b
				x[start+k+size/2] = a - b
				w *= step
			}
		}
	}
}
