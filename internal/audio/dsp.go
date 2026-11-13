package audio

import "math"

// SampleRate is the sample rate, in Hz, of everything this package renders or plays.
const SampleRate = 44100

const (
	sr    = float64(SampleRate)
	invSR = 1 / sr
)

// ---------------------------------------------------------------------------------------------
// Small numeric helpers
// ---------------------------------------------------------------------------------------------

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

// dbToLin converts decibels to a linear amplitude factor.
func dbToLin(db float64) float64 { return math.Pow(10, db/20) }

// linToDb converts a linear amplitude to decibels; silence maps to -200 dB.
func linToDb(lin float64) float64 {
	if lin < 1e-10 {
		return -200
	}
	return 20 * math.Log10(lin)
}

// midiHz returns the frequency of a MIDI note number (69 = A4 = 440 Hz).
func midiHz(note float64) float64 { return 440 * math.Pow(2, (note-69)/12) }

// secondsToSamples converts a duration to a whole number of samples.
func secondsToSamples(sec float64) int {
	if sec <= 0 {
		return 0
	}
	return int(math.Round(sec * sr))
}

// decayCoef returns the per-sample multiplier of an exponential decay with time constant tau
// seconds (the value falls to 1/e after tau).
func decayCoef(tau float64) float64 {
	if tau <= 0 {
		return 0
	}
	return math.Exp(-1 / (tau * sr))
}

// smoothCoef returns the per-sample coefficient of a one-pole smoother with time constant tau.
func smoothCoef(tau float64) float64 { return 1 - decayCoef(tau) }

// ---------------------------------------------------------------------------------------------
// PRNG
// ---------------------------------------------------------------------------------------------

// rng is a SplitMix64 generator. The audio package owns its randomness so that renders are
// reproducible and never touch math/rand's global state.
type rng struct{ s uint64 }

func newRNG(seed uint64) rng {
	r := rng{s: seed*0x9E3779B97F4A7C15 + 0xD1B54A32D192ED03}
	r.next()
	return r
}

func (r *rng) next() uint64 {
	r.s += 0x9E3779B97F4A7C15
	z := r.s
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// float returns a uniform value in [0, 1).
func (r *rng) float() float64 { return float64(r.next()>>11) / (1 << 53) }

// bipolar returns a uniform value in [-1, 1); used as white noise.
func (r *rng) bipolar() float64 { return r.float()*2 - 1 }

// rangef returns a uniform value in [lo, hi).
func (r *rng) rangef(lo, hi float64) float64 { return lo + (hi-lo)*r.float() }

// intn returns a uniform integer in [0, n); n <= 0 yields 0.
func (r *rng) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

// chance reports true with probability p.
func (r *rng) chance(p float64) bool { return r.float() < p }

// pick returns an index drawn with the given non-negative weights (0 if all are zero).
func (r *rng) pick(weights []float64) int {
	total := 0.0
	for _, w := range weights {
		if w > 0 {
			total += w
		}
	}
	if total <= 0 {
		return 0
	}
	x := r.float() * total
	for i, w := range weights {
		if w <= 0 {
			continue
		}
		if x < w {
			return i
		}
		x -= w
	}
	return len(weights) - 1
}

// ---------------------------------------------------------------------------------------------
// Oscillators
// ---------------------------------------------------------------------------------------------

const (
	sineBits = 12
	sineLen  = 1 << sineBits
)

var sineTable = func() (t [sineLen + 1]float64) {
	for i := range t {
		t[i] = math.Sin(2 * math.Pi * float64(i) / sineLen)
	}
	return t
}()

// sinT returns sin(2π·turns) from a linearly interpolated table (error below 1e-6). The phase
// is measured in turns, so 1.0 is a full cycle; any finite value is accepted.
func sinT(turns float64) float64 {
	turns -= float64(int64(turns))
	if turns < 0 {
		turns++
	}
	f := turns * sineLen
	i := int(f)
	frac := f - float64(i)
	i &= sineLen - 1
	a := sineTable[i]
	return a + (sineTable[i+1]-a)*frac
}

// polyBlep is the two-sample polynomial band-limited step residual for a phase ph in [0,1)
// advancing by dt per sample. Subtracting it from a naive sawtooth removes most aliasing.
func polyBlep(ph, dt float64) float64 {
	if ph < dt {
		x := ph / dt
		return x + x - x*x - 1
	}
	if ph > 1-dt {
		x := (ph - 1) / dt
		return x*x + x + x + 1
	}
	return 0
}

// osc is a phase accumulator with several band-limited or soft waveforms. The zero value is
// ready to use. Frequencies must be in [0, SampleRate/2).
type osc struct{ ph float64 }

func (o *osc) advance(freq float64) {
	o.ph += freq * invSR
	if o.ph >= 1 {
		o.ph -= 1
	}
}

// sine returns the next sample of a sine wave.
func (o *osc) sine(freq float64) float64 {
	v := sinT(o.ph)
	o.advance(freq)
	return v
}

// saw returns the next sample of a band-limited (polyBLEP) rising sawtooth.
func (o *osc) saw(freq float64) float64 {
	dt := freq * invSR
	v := 2*o.ph - 1 - polyBlep(o.ph, dt)
	o.advance(freq)
	return v
}

// pulse returns the next sample of a band-limited pulse wave with the given duty cycle
// (0.5 is a square wave). It is built as the difference of two band-limited sawtooths half a
// duty cycle apart, which keeps both edges consistent however the phase rounds.
func (o *osc) pulse(freq, width float64) float64 {
	dt := freq * invSR
	ph2 := o.ph + 1 - width
	if ph2 >= 1 {
		ph2 -= 1
	}
	s1 := 2*o.ph - 1 - polyBlep(o.ph, dt)
	s2 := 2*ph2 - 1 - polyBlep(ph2, dt)
	o.advance(freq)
	return s2 - s1 + 2*width - 1
}

// tri returns the next sample of a triangle wave (its harmonics fall off fast enough that no
// band-limiting is needed at the pitches used here).
func (o *osc) tri(freq float64) float64 {
	v := 4*math.Abs(o.ph-0.5) - 1
	o.advance(freq)
	return v
}

// ---------------------------------------------------------------------------------------------
// Envelopes
// ---------------------------------------------------------------------------------------------

const (
	envIdle = iota
	envAttack
	envDecay
	envSustain
	envRelease
)

// adsr is a streaming attack/decay/sustain/release envelope: linear attack, exponential decay
// and release. It is used by the music voices; the offline SFX recipes use the closed-form
// helpers in sfx.go instead.
type adsr struct {
	attackStep  float64
	decayCoef   float64
	sustain     float64
	releaseCoef float64
	stage       int
	level       float64
}

// set configures the envelope times (seconds) and sustain level (0..1).
func (e *adsr) set(attack, decay, sustain, release float64) {
	e.attackStep = 1 / math.Max(1, attack*sr)
	e.decayCoef = decayCoef(decay / 3)
	e.sustain = clamp(sustain, 0, 1)
	// The release is "done" at -60 dB after the given time.
	e.releaseCoef = decayCoef(release / 6.9)
}

// gate opens (true) or releases (false) the envelope. Opening an already sounding envelope
// continues from its current level, so re-gating never clicks.
func (e *adsr) gate(on bool) {
	if on {
		e.stage = envAttack
	} else if e.stage != envIdle {
		e.stage = envRelease
	}
}

// idle reports whether the envelope has finished and produces silence.
func (e *adsr) idle() bool { return e.stage == envIdle }

// next advances the envelope by one sample and returns its level.
func (e *adsr) next() float64 {
	switch e.stage {
	case envAttack:
		e.level += e.attackStep
		if e.level >= 1 {
			e.level = 1
			e.stage = envDecay
		}
	case envDecay:
		e.level = e.sustain + (e.level-e.sustain)*e.decayCoef
		if e.level-e.sustain < 1e-4 {
			e.level = e.sustain
			e.stage = envSustain
		}
	case envRelease:
		e.level *= e.releaseCoef
		if e.level < 1e-4 {
			e.level = 0
			e.stage = envIdle
		}
	}
	return e.level
}

// ---------------------------------------------------------------------------------------------
// Filters
// ---------------------------------------------------------------------------------------------

// onePole is a 6 dB/octave low-pass; the matching high-pass is the input minus its output.
type onePole struct{ a, z float64 }

func (f *onePole) set(cutoff float64) {
	f.a = 1 - math.Exp(-2*math.Pi*clamp(cutoff, 1, 0.49*sr)*invSR)
}

func (f *onePole) low(x float64) float64 {
	f.z += (x - f.z) * f.a
	return f.z
}

func (f *onePole) high(x float64) float64 { return x - f.low(x) }

// svf is a 12 dB/octave state-variable filter in the trapezoidal (zero-delay feedback) form.
// It stays stable while its cutoff is modulated, which the swept SFX and the music pads rely on.
type svf struct {
	g, k       float64
	a1, a2, a3 float64
	ic1, ic2   float64
}

// set tunes the filter. q is the resonance (0.707 is maximally flat).
func (f *svf) set(cutoff, q float64) {
	cutoff = clamp(cutoff, 8, 0.46*sr)
	if q < 0.3 {
		q = 0.3
	}
	f.g = math.Tan(math.Pi * cutoff * invSR)
	f.k = 1 / q
	f.a1 = 1 / (1 + f.g*(f.g+f.k))
	f.a2 = f.g * f.a1
	f.a3 = f.g * f.a2
}

func (f *svf) step(x float64) (v1, v2 float64) {
	v3 := x - f.ic2
	v1 = f.a1*f.ic1 + f.a2*v3
	v2 = f.ic2 + f.a2*f.ic1 + f.a3*v3
	f.ic1 = 2*v1 - f.ic1
	f.ic2 = 2*v2 - f.ic2
	return v1, v2
}

// low returns the low-pass output for the next input sample.
func (f *svf) low(x float64) float64 {
	_, v2 := f.step(x)
	return v2
}

// band returns the band-pass output, normalised to unity gain at the centre frequency.
func (f *svf) band(x float64) float64 {
	v1, _ := f.step(x)
	return v1 * f.k
}

// high returns the high-pass output.
func (f *svf) high(x float64) float64 {
	v1, v2 := f.step(x)
	return x - f.k*v1 - v2
}

// biquad is a direct-form-I second-order section, used for the shelving filter of the loudness
// meter.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

// setHighShelf configures a high shelf of the given gain (dB) around the corner frequency.
func (f *biquad) setHighShelf(corner, gainDB float64) {
	a := math.Pow(10, gainDB/40)
	w := 2 * math.Pi * corner * invSR
	cs, sn := math.Cos(w), math.Sin(w)
	alpha := sn / 2 * math.Sqrt2
	sq := 2 * math.Sqrt(a) * alpha
	a0 := (a + 1) - (a-1)*cs + sq
	f.b0 = a * ((a + 1) + (a-1)*cs + sq) / a0
	f.b1 = -2 * a * ((a - 1) + (a+1)*cs) / a0
	f.b2 = a * ((a + 1) + (a-1)*cs - sq) / a0
	f.a1 = 2 * ((a - 1) - (a+1)*cs) / a0
	f.a2 = ((a + 1) - (a-1)*cs - sq) / a0
}

func (f *biquad) tick(x float64) float64 {
	y := f.b0*x + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2
	f.x2, f.x1 = f.x1, x
	f.y2, f.y1 = f.y1, y
	return y
}

// ---------------------------------------------------------------------------------------------
// Delay line
// ---------------------------------------------------------------------------------------------

// delayLine is a power-of-two ring buffer with integer and interpolated taps.
type delayLine struct {
	buf  []float64
	mask int
	w    int
}

// newDelayLine allocates a line able to delay by at least maxSamples.
func newDelayLine(maxSamples int) delayLine {
	n := 2
	for n < maxSamples+2 {
		n <<= 1
	}
	return delayLine{buf: make([]float64, n), mask: n - 1}
}

// write stores the newest sample.
func (d *delayLine) write(x float64) {
	d.buf[d.w] = x
	d.w = (d.w + 1) & d.mask
}

// tap returns the sample written n writes ago (n >= 1).
func (d *delayLine) tap(n int) float64 { return d.buf[(d.w-n)&d.mask] }

// tapFrac returns a linearly interpolated tap at a fractional delay (n >= 1).
func (d *delayLine) tapFrac(n float64) float64 {
	i := int(n)
	frac := n - float64(i)
	a := d.buf[(d.w-i)&d.mask]
	b := d.buf[(d.w-i-1)&d.mask]
	return a + (b-a)*frac
}

// ---------------------------------------------------------------------------------------------
// Dynamics
// ---------------------------------------------------------------------------------------------

const (
	limiterThreshold = 0.84 // the limiter holds peaks at about -1.5 dBFS ...
	clipKnee         = 0.86 // ... so the soft clipper above it normally passes audio untouched
	clipCeiling      = 0.98 // absolute bound on any sample this package emits
)

// softClip is a transparent-below-the-knee saturator whose output never exceeds clipCeiling.
func softClip(x float64) float64 {
	a := math.Abs(x)
	if a <= clipKnee {
		return x
	}
	y := clipKnee + (clipCeiling-clipKnee)*math.Tanh((a-clipKnee)/(clipCeiling-clipKnee))
	return math.Copysign(y, x)
}

// limiter is a stereo-linked look-ahead peak limiter followed by a soft clipper. The look-ahead
// lets the gain fall before a peak arrives, so loud moments are turned down rather than
// distorted; the clipper is the hard guarantee that the output stays inside ±clipCeiling.
type limiter struct {
	l, r      []float64
	pos       int
	env       float64
	hold      int
	gain      float64
	attack    float64
	release   float64
	envDecay  float64
	lookahead int
}

func newLimiter() *limiter {
	n := secondsToSamples(0.005)
	return &limiter{
		l:         make([]float64, n),
		r:         make([]float64, n),
		gain:      1,
		attack:    smoothCoef(0.005 / 4),
		release:   smoothCoef(0.180),
		envDecay:  decayCoef(0.060),
		lookahead: n,
	}
}

// tick processes one stereo frame. The output is delayed by the look-ahead (5 ms).
func (m *limiter) tick(inL, inR float64) (float64, float64) {
	peak := math.Max(math.Abs(inL), math.Abs(inR))
	switch {
	case peak >= m.env:
		m.env = peak
		m.hold = m.lookahead
	case m.hold > 0:
		m.hold--
	default:
		m.env = peak + (m.env-peak)*m.envDecay
	}
	target := 1.0
	if m.env > limiterThreshold {
		target = limiterThreshold / m.env
	}
	if target < m.gain {
		m.gain += (target - m.gain) * m.attack
	} else {
		m.gain += (target - m.gain) * m.release
	}
	outL, outR := m.l[m.pos], m.r[m.pos]
	m.l[m.pos], m.r[m.pos] = inL, inR
	m.pos++
	if m.pos == m.lookahead {
		m.pos = 0
	}
	return softClip(outL * m.gain), softClip(outR * m.gain)
}
