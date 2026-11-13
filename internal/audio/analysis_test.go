package audio

import (
	"math"
	"math/cmplx"
)

// Measurement helpers for the tests: levels, spectra and discontinuity detection.

// rmsOf returns the root-mean-square level of x (0 for an empty slice).
func rmsOf(x []float64) float64 {
	if len(x) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range x {
		sum += v * v
	}
	return math.Sqrt(sum / float64(len(x)))
}

// fft computes the discrete Fourier transform of x in place; len(x) must be a power of two.
func fft(x []complex128) {
	n := len(x)
	if n < 2 {
		return
	}
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

// powerSpectrum returns the averaged power spectrum of x using Hann-windowed frames of the
// given size (a power of two) with 50 % overlap. Bin k covers frequency k·SampleRate/size.
func powerSpectrum(x []float64, size int) []float64 {
	spec := make([]float64, size/2+1)
	if len(x) == 0 {
		return spec
	}
	frame := make([]complex128, size)
	frames := 0
	for start := 0; start == 0 || start+size <= len(x); start += size / 2 {
		for i := range frame {
			v := 0.0
			if start+i < len(x) {
				v = x[start+i]
			}
			w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(size))
			frame[i] = complex(v*w, 0)
		}
		fft(frame)
		for k := range spec {
			re, im := real(frame[k]), imag(frame[k])
			spec[k] += re*re + im*im
		}
		frames++
	}
	for k := range spec {
		spec[k] /= float64(frames)
	}
	return spec
}

// bandEnergy returns the fraction (0..1) of a power spectrum's energy between lo and hi Hz.
func bandEnergy(spec []float64, lo, hi float64) float64 {
	size := (len(spec) - 1) * 2
	total, band := 0.0, 0.0
	for k, p := range spec {
		f := float64(k) * sr / float64(size)
		total += p
		if f >= lo && f < hi {
			band += p
		}
	}
	if total <= 0 {
		return 0
	}
	return band / total
}

// spectralCentroid returns the energy-weighted mean frequency of a power spectrum, a simple
// measure of brightness.
func spectralCentroid(spec []float64) float64 {
	size := (len(spec) - 1) * 2
	total, weighted := 0.0, 0.0
	for k, p := range spec {
		total += p
		weighted += p * float64(k) * sr / float64(size)
	}
	if total <= 0 {
		return 0
	}
	return weighted / total
}

// goertzel returns the amplitude of the sinusoidal component of x at freq Hz.
func goertzel(x []float64, freq float64) float64 {
	if len(x) == 0 {
		return 0
	}
	w := 2 * math.Pi * freq * invSR
	coef := 2 * math.Cos(w)
	var s1, s2 float64
	for _, v := range x {
		s0 := v + coef*s1 - s2
		s2, s1 = s1, s0
	}
	power := s1*s1 + s2*s2 - coef*s1*s2
	if power < 0 {
		power = 0
	}
	return 2 * math.Sqrt(power) / float64(len(x))
}

// splitStereo separates an interleaved stereo buffer into its two channels.
func splitStereo(interleaved []float32) (l, r []float64) {
	n := len(interleaved) / 2
	l, r = make([]float64, n), make([]float64, n)
	for i := 0; i < n; i++ {
		l[i] = float64(interleaved[2*i])
		r[i] = float64(interleaved[2*i+1])
	}
	return l, r
}

// midOf returns the mono (mid) sum of an interleaved stereo buffer.
func midOf(interleaved []float32) []float64 {
	n := len(interleaved) / 2
	m := make([]float64, n)
	for i := 0; i < n; i++ {
		m[i] = (float64(interleaved[2*i]) + float64(interleaved[2*i+1])) / 2
	}
	return m
}

// maxKink returns the largest second difference |x[i] - 2x[i-1] + x[i-2]| and where it occurs.
// A smooth waveform has tiny second differences; a click (a step or a corner in the waveform)
// shows up as one large value, however quiet the surrounding signal is.
func maxKink(x []float64) (kink float64, at int) {
	for i := 2; i < len(x); i++ {
		if d := math.Abs(x[i] - 2*x[i-1] + x[i-2]); d > kink {
			kink, at = d, i
		}
	}
	return kink, at
}

// kinkOutlier returns the largest ratio between a second difference and the RMS second
// difference of the 20 ms around it (excluding the immediate neighbourhood), ignoring kinks
// below floor. It finds isolated discontinuities in otherwise smooth material.
func kinkOutlier(x []float64, floor float64) (ratio float64, at int) {
	n := len(x)
	d2 := make([]float64, n)
	for i := 2; i < n; i++ {
		d2[i] = x[i] - 2*x[i-1] + x[i-2]
	}
	prefix := make([]float64, n+1)
	for i, v := range d2 {
		prefix[i+1] = prefix[i] + v*v
	}
	const half, guard = 441, 8
	for i := half; i < n-half; i++ {
		a := math.Abs(d2[i])
		if a < floor {
			continue
		}
		sum := (prefix[i+half] - prefix[i-half]) - (prefix[i+guard] - prefix[i-guard])
		local := math.Sqrt(sum / float64(2*(half-guard)))
		if r := a / (local + 1e-9); r > ratio {
			ratio, at = r, i
		}
	}
	return ratio, at
}
