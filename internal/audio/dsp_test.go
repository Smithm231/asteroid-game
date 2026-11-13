package audio

import (
	"math"
	"testing"
)

func TestSinT(t *testing.T) {
	for _, turns := range []float64{0, 0.25, 0.5, 0.75, 1, 0.1234, 7.77, -0.3, -12.625, 1e6 + 0.375} {
		got, want := sinT(turns), math.Sin(2*math.Pi*turns)
		if math.Abs(got-want) > 2e-6 {
			t.Errorf("sinT(%v) = %v, want %v", turns, got, want)
		}
	}
}

func TestConversions(t *testing.T) {
	tests := []struct {
		name      string
		got, want float64
	}{
		{"A4", midiHz(69), 440},
		{"A5", midiHz(81), 880},
		{"D4", midiHz(62), 293.6648},
		{"0 dB", dbToLin(0), 1},
		{"-6 dB", dbToLin(-6.0206), 0.5},
		{"half", linToDb(0.5), -6.0206},
		{"silence", linToDb(0), -200},
		{"1 s in samples", float64(secondsToSamples(1)), SampleRate},
		{"negative time", float64(secondsToSamples(-1)), 0},
		{"decay to 1/e", math.Pow(decayCoef(0.5), 0.5*sr), 1 / math.E},
	}
	for _, tc := range tests {
		if math.Abs(tc.got-tc.want) > 1e-3*math.Max(1, math.Abs(tc.want)) {
			t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
		}
	}
}

func TestRNG(t *testing.T) {
	a, b, c := newRNG(42), newRNG(42), newRNG(43)
	same, differ := true, false
	sum := 0.0
	const n = 20000
	for i := 0; i < n; i++ {
		x, y, z := a.float(), b.float(), c.float()
		if x != y {
			same = false
		}
		if x != z {
			differ = true
		}
		if x < 0 || x >= 1 {
			t.Fatalf("float() = %v, outside [0,1)", x)
		}
		sum += x
	}
	if !same {
		t.Error("equal seeds produced different sequences")
	}
	if !differ {
		t.Error("different seeds produced the same sequence")
	}
	if mean := sum / n; math.Abs(mean-0.5) > 0.01 {
		t.Errorf("mean of float() = %v, want about 0.5", mean)
	}
	r := newRNG(7)
	var hits [5]int
	for i := 0; i < 5000; i++ {
		k := r.intn(5)
		if k < 0 || k >= 5 {
			t.Fatalf("intn(5) = %d", k)
		}
		hits[k]++
		if v := r.bipolar(); v < -1 || v >= 1 {
			t.Fatalf("bipolar() = %v", v)
		}
		if v := r.rangef(3, 4); v < 3 || v >= 4 {
			t.Fatalf("rangef(3,4) = %v", v)
		}
		if k := r.pick([]float64{0, 2, 0, 1}); k != 1 && k != 3 {
			t.Fatalf("pick chose index %d, which has zero weight", k)
		}
	}
	for k, h := range hits {
		if h < 800 || h > 1200 {
			t.Errorf("intn(5) hit %d %d times out of 5000", k, h)
		}
	}
	if r.intn(0) != 0 || r.intn(-3) != 0 {
		t.Error("intn of a non-positive bound must be 0")
	}
	if r.pick(nil) != 0 || r.pick([]float64{0, 0}) != 0 {
		t.Error("pick with no positive weight must be 0")
	}
}

func TestOscillators(t *testing.T) {
	tests := []struct {
		name  string
		wave  func(o *osc, f float64) float64
		freqs []float64
	}{
		{"sine", (*osc).sine, []float64{55, 441, 2205, 8820}},
		{"saw", (*osc).saw, []float64{55, 441, 2205, 8820}},
		{"square", func(o *osc, f float64) float64 { return o.pulse(f, 0.5) }, []float64{55, 441, 2205, 8820}},
		// The triangle is not band-limited and is only used for low notes.
		{"tri", (*osc).tri, []float64{55, 441, 882}},
	}
	for _, tc := range tests {
		for _, freq := range tc.freqs {
			var o osc
			// A whole number of cycles, so the mean measures DC exactly.
			n := int(math.Round(sr/freq)) * 200
			x := make([]float64, n)
			for i := range x {
				x[i] = tc.wave(&o, freq)
			}
			// Band-limiting legitimately lowers the peak of a wave with few harmonics left.
			if p := peakOf(x); p > 1.3 || p < 0.5 || math.IsNaN(p) {
				t.Errorf("%s %.0f Hz: peak %v, want about 1", tc.name, freq, p)
			}
			mean := 0.0
			for _, v := range x {
				mean += v / float64(n)
			}
			if math.Abs(mean) > 0.01 {
				t.Errorf("%s %.0f Hz: DC offset %v", tc.name, freq, mean)
			}
			fund := goertzel(x, freq)
			off := goertzel(x, freq*1.37)
			if fund < 0.5 || off > fund/20 {
				t.Errorf("%s %.0f Hz: fundamental %v, off-pitch %v", tc.name, freq, fund, off)
			}
		}
	}
}

func TestPulseWidth(t *testing.T) {
	// The duty cycle sets the fraction of time spent high, and so the mean.
	for _, width := range []float64{0.2, 0.3, 0.5, 0.75} {
		var o osc
		n := 441 * 100
		mean, peak := 0.0, 0.0
		for i := 0; i < n; i++ {
			v := o.pulse(100, width)
			mean += v / float64(n)
			peak = math.Max(peak, math.Abs(v))
		}
		if want := 2*width - 1; math.Abs(mean-want) > 0.01 {
			t.Errorf("width %.2f: mean %v, want %v", width, mean, want)
		}
		if peak > 1.3 {
			t.Errorf("width %.2f: peak %v, want about 1", width, peak)
		}
	}
}

// A naive sawtooth at 4 kHz has harmonics far above the Nyquist frequency, which fold back to
// frequencies that are not harmonics of the note. The band-limited sawtooth must be nearly free
// of them.
func TestSawIsBandLimited(t *testing.T) {
	const freq = 4009.0
	const n = 1 << 15
	render := func(blep bool) []float64 {
		var o osc
		x := make([]float64, n)
		for i := range x {
			if blep {
				x[i] = o.saw(freq)
			} else {
				x[i] = 2*o.ph - 1
				o.advance(freq)
			}
		}
		return x
	}
	inharmonic := func(x []float64) float64 {
		spec := powerSpectrum(x, 4096)
		total, alias := 0.0, 0.0
		for k, p := range spec {
			f := float64(k) * sr / 4096
			total += p
			h := math.Round(f / freq)
			if h < 1 || math.Abs(f-h*freq) > 60 {
				alias += p
			}
		}
		return alias / total
	}
	naive, limited := inharmonic(render(false)), inharmonic(render(true))
	t.Logf("energy outside the harmonics: naive %.4f, polyBLEP %.4f", naive, limited)
	if limited > naive/4 {
		t.Errorf("polyBLEP saw aliases too much: %.4f of its energy (naive: %.4f)", limited, naive)
	}
}

func TestADSR(t *testing.T) {
	var e adsr
	e.set(0.01, 0.05, 0.6, 0.1)
	if !e.idle() || e.next() != 0 {
		t.Fatal("a fresh envelope must be idle and silent")
	}
	e.gate(true)
	prev, samples := 0.0, 0
	for prev < 1 && samples < SampleRate {
		v := e.next()
		if v < prev {
			t.Fatalf("attack is not monotonic at sample %d", samples)
		}
		prev = v
		samples++
	}
	if want := secondsToSamples(0.01); samples < want-1 || samples > want+1 {
		t.Fatalf("attack took %d samples, want %d", samples, want)
	}
	for i := 0; i < secondsToSamples(0.3); i++ {
		prev = e.next()
	}
	if math.Abs(prev-0.6) > 1e-3 {
		t.Fatalf("sustain level = %v, want 0.6", prev)
	}
	e.gate(false)
	for i := 0; i < secondsToSamples(0.05); i++ {
		prev = e.next()
	}
	if prev <= 0 || prev > 0.1 {
		t.Fatalf("level half-way through the release = %v", prev)
	}
	// Re-opening continues from the current level: no jump.
	e.gate(true)
	if v := e.next(); math.Abs(v-prev) > 0.01 {
		t.Fatalf("re-gating jumped from %v to %v", prev, v)
	}
	e.gate(false)
	for i := 0; i < secondsToSamples(0.5); i++ {
		prev = e.next()
	}
	if prev != 0 || !e.idle() {
		t.Fatalf("envelope did not finish: level %v idle %v", prev, e.idle())
	}
}

func sineGain(freq float64, process func(x float64) float64) float64 {
	n := secondsToSamples(0.5)
	out := make([]float64, n)
	for i := range out {
		out[i] = process(math.Sin(2 * math.Pi * freq * float64(i) * invSR))
	}
	return goertzel(out[n/2:], freq)
}

func TestFilters(t *testing.T) {
	tests := []struct {
		name     string
		make     func() func(float64) float64
		freq     float64
		min, max float64
	}{
		{"svf low passes lows", func() func(float64) float64 { var f svf; f.set(1000, 0.707); return f.low }, 100, 0.97, 1.03},
		{"svf low cuts highs", func() func(float64) float64 { var f svf; f.set(1000, 0.707); return f.low }, 8000, 0, 0.03},
		{"svf low is -3 dB at cutoff", func() func(float64) float64 { var f svf; f.set(1000, 0.707); return f.low }, 1000, 0.68, 0.74},
		{"svf high passes highs", func() func(float64) float64 { var f svf; f.set(1000, 0.707); return f.high }, 8000, 0.95, 1.05},
		{"svf high cuts lows", func() func(float64) float64 { var f svf; f.set(1000, 0.707); return f.high }, 100, 0, 0.02},
		{"svf band is unity at centre", func() func(float64) float64 { var f svf; f.set(1000, 4); return f.band }, 1000, 0.97, 1.03},
		{"svf band rejects far below", func() func(float64) float64 { var f svf; f.set(1000, 4); return f.band }, 100, 0, 0.05},
		{"one-pole low passes lows", func() func(float64) float64 { var f onePole; f.set(1000); return f.low }, 50, 0.97, 1.01},
		{"one-pole low cuts highs", func() func(float64) float64 { var f onePole; f.set(1000); return f.low }, 10000, 0, 0.2},
		{"one-pole high cuts lows", func() func(float64) float64 { var f onePole; f.set(1000); return f.high }, 50, 0, 0.08},
		{"shelf leaves lows", func() func(float64) float64 { var f biquad; f.setHighShelf(1500, 4); return f.tick }, 100, 0.98, 1.03},
		{"shelf lifts highs by 4 dB", func() func(float64) float64 { var f biquad; f.setHighShelf(1500, 4); return f.tick }, 10000, 1.52, 1.64},
	}
	for _, tc := range tests {
		if g := sineGain(tc.freq, tc.make()); g < tc.min || g > tc.max {
			t.Errorf("%s: gain at %.0f Hz = %.3f, want %.2f..%.2f", tc.name, tc.freq, g, tc.min, tc.max)
		}
	}
}

func TestSVFStableUnderModulation(t *testing.T) {
	var f svf
	r := newRNG(1)
	for i := 0; i < 200000; i++ {
		if i%7 == 0 {
			f.set(r.rangef(20, 18000), r.rangef(0.3, 8))
		}
		v := f.low(r.bipolar())
		if math.IsNaN(v) || math.Abs(v) > 50 {
			t.Fatalf("filter blew up at sample %d: %v", i, v)
		}
	}
}

func TestDelayLine(t *testing.T) {
	d := newDelayLine(100)
	for i := 1; i <= 500; i++ {
		d.write(float64(i))
	}
	for _, n := range []int{1, 2, 50, 100} {
		if got, want := d.tap(n), float64(501-n); got != want {
			t.Errorf("tap(%d) = %v, want %v", n, got, want)
		}
	}
	if got := d.tapFrac(10.25); math.Abs(got-490.75) > 1e-9 {
		t.Errorf("tapFrac(10.25) = %v, want 490.75", got)
	}
}

func TestReverb(t *testing.T) {
	for _, tc := range []struct{ size, rt60 float64 }{{0.5, 0.6}, {1, 1.5}, {1.15, 3.8}} {
		rv := newReverb(tc.size, tc.rt60, 0.3, 100)
		n := secondsToSamples(tc.rt60 * 2.5)
		l, r := make([]float64, n), make([]float64, n)
		for i := range l {
			in := 0.0
			if i == 0 {
				in = 1
			}
			l[i], r[i] = rv.tick(in, in)
			if math.IsNaN(l[i]) || math.IsNaN(r[i]) {
				t.Fatalf("rt60 %.1f: NaN at sample %d", tc.rt60, i)
			}
		}
		// Schroeder backward integration gives the energy decay curve; the time it takes to
		// fall from -5 to -35 dB, doubled, estimates RT60.
		total := 0.0
		for _, v := range l {
			total += v * v
		}
		rem := total
		t5, t35 := -1, -1
		for i, v := range l {
			rem -= v * v
			db := 10 * math.Log10(math.Max(rem, 1e-30)/total)
			if t5 < 0 && db <= -5 {
				t5 = i
			}
			if t35 < 0 && db <= -35 {
				t35 = i
				break
			}
		}
		if t5 < 0 || t35 < 0 {
			t.Fatalf("rt60 %.1f: the tail never decayed by 35 dB", tc.rt60)
		}
		measured := 2 * float64(t35-t5) * invSR
		if measured < tc.rt60*0.5 || measured > tc.rt60*1.3 {
			t.Errorf("configured rt60 %.2f s, measured %.2f s", tc.rt60, measured)
		}
		same := true
		for i := range l {
			if l[i] != r[i] {
				same = false
				break
			}
		}
		if same {
			t.Errorf("rt60 %.1f: left and right tails are identical, want a stereo tail", tc.rt60)
		}
		if tailPeak := peakOf(l[n-secondsToSamples(0.1):]); tailPeak > 0.002*peakOf(l) {
			t.Errorf("rt60 %.1f: tail still at %.5f of the peak after 2.5×rt60", tc.rt60, tailPeak/peakOf(l))
		}
	}
}

func TestReverbLevelIndependentOfDecay(t *testing.T) {
	// The wet level of a steady input should not depend much on the decay time.
	level := func(rt60 float64) float64 {
		rv := newReverb(1, rt60, 0.2, 60)
		r := newRNG(5)
		n := secondsToSamples(rt60*2 + 2)
		out := make([]float64, n)
		for i := range out {
			out[i], _ = rv.tick(r.bipolar(), r.bipolar())
		}
		return rmsOf(out[n-secondsToSamples(1):])
	}
	short, long := level(0.5), level(4)
	if ratio := linToDb(long / short); math.Abs(ratio) > 4 {
		t.Errorf("wet level differs by %.1f dB between rt60 0.5 s and 4 s", ratio)
	}
}

func TestSoftClip(t *testing.T) {
	prev := -1.0
	for x := -6.0; x <= 6; x += 0.001 {
		y := softClip(x)
		if math.Abs(y) > clipCeiling {
			t.Fatalf("softClip(%v) = %v exceeds the ceiling", x, y)
		}
		if y < prev {
			t.Fatalf("softClip is not monotonic at %v", x)
		}
		if math.Abs(x) <= clipKnee && y != x {
			t.Fatalf("softClip(%v) = %v, want identity below the knee", x, y)
		}
		if math.Abs(softClip(-x)+y) > 1e-12 {
			t.Fatalf("softClip is not odd at %v", x)
		}
		prev = y
	}
}

func TestLimiter(t *testing.T) {
	lim := newLimiter()
	// A quiet signal passes untouched, delayed by the look-ahead.
	quiet := func(i int) float64 { return 0.5 * math.Sin(2*math.Pi*440*float64(i)*invSR) }
	for i := 0; i < 5000; i++ {
		l, r := lim.tick(quiet(i), -quiet(i))
		if i >= lim.lookahead {
			if want := quiet(i - lim.lookahead); l != want || r != -want {
				t.Fatalf("quiet signal altered at %d: got %v/%v, want ±%v", i, l, r, want)
			}
		}
	}
	// A burst far above full scale is held under the ceiling and mostly by gain reduction,
	// not by clipping: after the attack the output peak sits near the threshold.
	lim = newLimiter()
	peak, settled := 0.0, 0.0
	n := secondsToSamples(0.5)
	for i := 0; i < n; i++ {
		x := 4 * math.Sin(2*math.Pi*220*float64(i)*invSR)
		l, r := lim.tick(x, x*0.5)
		a := math.Max(math.Abs(l), math.Abs(r))
		peak = math.Max(peak, a)
		if i > n/2 {
			settled = math.Max(settled, a)
		}
	}
	if peak > clipCeiling {
		t.Errorf("limiter let a peak of %v through (ceiling %v)", peak, clipCeiling)
	}
	if settled > limiterThreshold+0.01 || settled < limiterThreshold-0.05 {
		t.Errorf("settled peak = %v, want about the threshold %v", settled, limiterThreshold)
	}
	// After the burst the gain recovers.
	for i := 0; i < secondsToSamples(1.5); i++ {
		lim.tick(0, 0)
	}
	if lim.gain < 0.99 {
		t.Errorf("gain after 1.5 s of silence = %v, want it recovered to 1", lim.gain)
	}
}
