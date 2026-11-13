package audio

import (
	"math"
	"sync"
)

// Sfx identifies one of the synthesised sound effects.
type Sfx int

// The complete sound-effect set. Each is rendered once from a recipe in sfx_recipes.go.
const (
	UIHover        Sfx = iota // pointer enters an interactive widget
	UIClick                   // button press
	UIOpen                    // window or panel opens
	UIClose                   // window or panel closes
	UIError                   // rejected action
	Place                     // structure placed on a tile
	BuildDone                 // construction finished
	Demolish                  // structure demolished or cancelled
	ExplosionSmall            // ship destroyed
	ExplosionLarge            // building destroyed
	Flak                      // Flak Battery burst
	Lance                     // Lance Tower beam
	ShipShot                  // ship gun
	Bomb                      // bombardment hit
	ShieldHit                 // Aegis field absorbs a hit
	Launch                    // fleet departs
	Arrive                    // fleet arrives
	Alert                     // urgent warning
	Notice                    // neutral notification
	Good                      // positive notification
	TechDone                  // research completed
	DirectiveDone             // directive completed
	Coin                      // trade or Scrip income
	SlugFire                  // Mass Driver fires
	SlugImpact                // Mass Driver slug lands
	Meteor                    // micrometeor impact
	FlareStart                // solar flare begins
	ColonyFounded             // new Keystone established
	ColonyFell                // a colony is lost
	Victory                   // game won (stinger)
	Defeat                    // game lost (stinger)
	sfxCount
)

// NumSfx is the number of sound effects; valid values are 0 <= s < NumSfx.
const NumSfx = int(sfxCount)

// sfxDef describes one effect: how it is built and how the mixer treats it.
type sfxDef struct {
	name     string
	loudness float64 // target perceived level, dBFS (see loudnessDB)
	minGap   float64 // minimum seconds between two triggers of this effect
	maxInst  int     // maximum simultaneously sounding instances
	minDur   float64 // design duration range in seconds, verified by the tests
	maxDur   float64
	fadeOut  float64 // seconds of fade applied at the very end
	build    func(r *rng) stereo
}

var sfxDefs = [NumSfx]sfxDef{
	UIHover:        {"UIHover", -34, 0.045, 2, 0.030, 0.080, 0.008, sfxUIHover},
	UIClick:        {"UIClick", -27, 0.040, 2, 0.040, 0.120, 0.010, sfxUIClick},
	UIOpen:         {"UIOpen", -27, 0.080, 2, 0.120, 0.300, 0.020, sfxUIOpen},
	UIClose:        {"UIClose", -28, 0.080, 2, 0.120, 0.300, 0.020, sfxUIClose},
	UIError:        {"UIError", -24, 0.160, 1, 0.180, 0.400, 0.020, sfxUIError},
	Place:          {"Place", -25, 0.060, 3, 0.080, 0.300, 0.015, sfxPlace},
	BuildDone:      {"BuildDone", -21, 0.200, 2, 0.400, 1.200, 0.040, sfxBuildDone},
	Demolish:       {"Demolish", -20, 0.120, 2, 0.400, 1.200, 0.040, sfxDemolish},
	ExplosionSmall: {"ExplosionSmall", -17, 0.090, 4, 0.400, 1.200, 0.050, sfxExplosionSmall},
	ExplosionLarge: {"ExplosionLarge", -15, 0.180, 3, 1.000, 1.900, 0.080, sfxExplosionLarge},
	Flak:           {"Flak", -20, 0.090, 3, 0.150, 0.450, 0.020, sfxFlak},
	Lance:          {"Lance", -20, 0.110, 3, 0.300, 0.800, 0.030, sfxLance},
	ShipShot:       {"ShipShot", -22, 0.070, 4, 0.100, 0.350, 0.020, sfxShipShot},
	Bomb:           {"Bomb", -18, 0.110, 3, 0.500, 1.300, 0.050, sfxBomb},
	ShieldHit:      {"ShieldHit", -21, 0.090, 3, 0.300, 0.800, 0.040, sfxShieldHit},
	Launch:         {"Launch", -21, 0.300, 2, 0.700, 1.400, 0.050, sfxLaunch},
	Arrive:         {"Arrive", -22, 0.300, 2, 0.500, 1.300, 0.050, sfxArrive},
	Alert:          {"Alert", -18, 0.600, 1, 0.500, 1.000, 0.030, sfxAlert},
	Notice:         {"Notice", -24, 0.250, 2, 0.200, 0.600, 0.030, sfxNotice},
	Good:           {"Good", -22, 0.250, 2, 0.350, 0.900, 0.040, sfxGood},
	TechDone:       {"TechDone", -20, 0.800, 1, 0.900, 1.800, 0.080, sfxTechDone},
	DirectiveDone:  {"DirectiveDone", -20, 0.800, 1, 0.800, 1.700, 0.080, sfxDirectiveDone},
	Coin:           {"Coin", -23, 0.080, 3, 0.250, 0.600, 0.030, sfxCoin},
	SlugFire:       {"SlugFire", -18, 0.200, 2, 0.400, 0.900, 0.040, sfxSlugFire},
	SlugImpact:     {"SlugImpact", -15, 0.200, 2, 0.700, 1.500, 0.060, sfxSlugImpact},
	Meteor:         {"Meteor", -17, 0.110, 3, 0.500, 1.200, 0.050, sfxMeteor},
	FlareStart:     {"FlareStart", -20, 1.500, 1, 1.100, 1.900, 0.080, sfxFlareStart},
	ColonyFounded:  {"ColonyFounded", -19, 1.000, 1, 1.100, 2.000, 0.100, sfxColonyFounded},
	ColonyFell:     {"ColonyFell", -19, 1.000, 1, 1.100, 2.000, 0.100, sfxColonyFell},
	Victory:        {"Victory", -16, 3.000, 1, 2.000, 4.000, 0.250, sfxVictory},
	Defeat:         {"Defeat", -18, 3.000, 1, 2.000, 4.000, 0.250, sfxDefeat},
}

// Valid reports whether s names an existing sound effect.
func (s Sfx) Valid() bool { return s >= 0 && int(s) < NumSfx }

// String returns the effect's name as written in the constant list, e.g. "ExplosionLarge".
func (s Sfx) String() string {
	if !s.Valid() {
		return "Sfx(invalid)"
	}
	return sfxDefs[s].name
}

// AllSfx returns every sound effect in declaration order.
func AllSfx() []Sfx {
	all := make([]Sfx, NumSfx)
	for i := range all {
		all[i] = Sfx(i)
	}
	return all
}

// RenderSfx synthesises a sound effect from scratch and returns it as interleaved stereo
// float32 PCM (L, R, L, R, ...) at SampleRate. The result is deterministic: every call returns
// identical samples. An invalid s yields nil.
//
// The buffer begins and ends at silence (short fades are built in), has no DC offset and is
// levelled against the rest of the set, so it can be mixed as is.
func RenderSfx(s Sfx) []float32 {
	if !s.Valid() {
		return nil
	}
	def := &sfxDefs[s]
	r := newRNG(0x5F3759DF + uint64(s)*7919)
	return finalize(def.build(&r), def.loudness, def.fadeOut)
}

// renderAllSfx renders the whole set, one goroutine per effect (each has its own generator,
// so the result does not depend on scheduling).
func renderAllSfx() [NumSfx][]float32 {
	var out [NumSfx][]float32
	var wg sync.WaitGroup
	for i := range out {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = RenderSfx(Sfx(i))
		}()
	}
	wg.Wait()
	return out
}

// ---------------------------------------------------------------------------------------------
// Offline building blocks
// ---------------------------------------------------------------------------------------------

// track is a mono offline buffer. Its methods transform it in place and return it for chaining.
type track []float64

// gen renders sec seconds by evaluating f at each sample time t (seconds).
func gen(sec float64, f func(t float64) float64) track {
	out := make(track, secondsToSamples(sec))
	for i := range out {
		out[i] = f(float64(i) * invSR)
	}
	return out
}

// noise returns sec seconds of white noise.
func noise(r *rng, sec float64) track {
	out := make(track, secondsToSamples(sec))
	for i := range out {
		out[i] = r.bipolar()
	}
	return out
}

// tail fades the end of the track to silence with a raised cosine over at most sec seconds
// (and at most a fifth of the track). Every layer that decays exponentially is finished this
// way, so that it never stops dead in the middle of a sound.
func (a track) tail(sec float64) track {
	n := min(secondsToSamples(sec), len(a)/5)
	for i := 0; i < n; i++ {
		a[len(a)-1-i] *= 0.5 - 0.5*math.Cos(math.Pi*float64(i)/float64(n))
	}
	return a
}

// add mixes b into a starting at the given time, growing a if needed. A layer that does not
// already end in silence is given a short tail first (b is modified).
func (a track) add(b track, at, gain float64) track {
	if n := len(b); n > 0 && math.Abs(b[n-1]) > 1e-4 {
		b.tail(0.012)
	}
	off := secondsToSamples(at)
	if need := off + len(b); need > len(a) {
		a = append(a, make(track, need-len(a))...)
	}
	for i, v := range b {
		a[off+i] += v * gain
	}
	return a
}

// shape multiplies the track by an envelope evaluated at each sample time.
func (a track) shape(env func(t float64) float64) track {
	for i := range a {
		a[i] *= env(float64(i) * invSR)
	}
	return a
}

func (a track) lowpass(cutoff, q float64) track {
	var f svf
	f.set(cutoff, q)
	for i, v := range a {
		a[i] = f.low(v)
	}
	return a
}

func (a track) highpass(cutoff, q float64) track {
	var f svf
	f.set(cutoff, q)
	for i, v := range a {
		a[i] = f.high(v)
	}
	return a
}

func (a track) bandpass(centre, q float64) track {
	var f svf
	f.set(centre, q)
	for i, v := range a {
		a[i] = f.band(v)
	}
	return a
}

// sweepLow low-passes the track while the cutoff glides exponentially from f0 to f1 over its
// whole length.
func (a track) sweepLow(f0, f1, q float64) track {
	var f svf
	for i, v := range a {
		if i&15 == 0 {
			f.set(glide(f0, f1, float64(i)/float64(len(a))), q)
		}
		a[i] = f.low(v)
	}
	return a
}

// sweepBand band-passes the track while the centre frequency glides from f0 to f1.
func (a track) sweepBand(f0, f1, q float64) track {
	var f svf
	for i, v := range a {
		if i&15 == 0 {
			f.set(glide(f0, f1, float64(i)/float64(len(a))), q)
		}
		a[i] = f.band(v)
	}
	return a
}

// drive adds weight with a tanh saturator; the output is rescaled to peak near 1 for
// full-scale input.
func (a track) drive(amount float64) track {
	norm := 1 / math.Tanh(amount)
	for i, v := range a {
		a[i] = math.Tanh(v*amount) * norm
	}
	return a
}

// normalize scales the track so its peak is 1 (silence is left alone). Layers are normalised
// before mixing so recipe gains mean what they say.
func (a track) normalize() track {
	p := peakOf(a)
	if p < 1e-9 {
		return a
	}
	for i := range a {
		a[i] /= p
	}
	return a
}

// perc is a percussive envelope: linear attack over a seconds, then exponential decay with
// time constant d.
func perc(t, a, d float64) float64 {
	if t < 0 {
		return 0
	}
	if t < a {
		return t / a
	}
	return math.Exp(-(t - a) / d)
}

// percEnv returns perc as an envelope function for track.shape.
func percEnv(a, d float64) func(float64) float64 {
	return func(t float64) float64 { return perc(t, a, d) }
}

// trap is a sustained envelope of total length dur with raised-cosine attack a and release r.
func trap(t, dur, a, r float64) float64 {
	if t <= 0 || t >= dur {
		return 0
	}
	g := 1.0
	if t < a {
		g = 0.5 - 0.5*math.Cos(math.Pi*t/a)
	}
	if t > dur-r {
		g *= 0.5 - 0.5*math.Cos(math.Pi*(dur-t)/r)
	}
	return g
}

// bump is a single smooth swell of length dur that peaks at peak·dur (0 < peak < 1).
func bump(t, dur, peak float64) float64 {
	if t <= 0 || t >= dur {
		return 0
	}
	p := peak * dur
	if t < p {
		return 0.5 - 0.5*math.Cos(math.Pi*t/p)
	}
	return 0.5 + 0.5*math.Cos(math.Pi*(t-p)/(dur-p))
}

// glide interpolates geometrically from f0 (x = 0) to f1 (x = 1); musical pitch moves evenly.
func glide(f0, f1, x float64) float64 {
	return f0 * math.Pow(f1/f0, clamp(x, 0, 1))
}

// drop falls exponentially from f0 toward f1 with time constant tau.
func drop(f0, f1, t, tau float64) float64 {
	return f1 + (f0-f1)*math.Exp(-t/tau)
}

// thump is the body of a low impact: a sine whose pitch drops from f0 to f1.
func thump(sec, f0, f1, pitchTau, attack, ampTau float64) track {
	var o osc
	return gen(sec, func(t float64) float64 {
		return o.sine(drop(f0, f1, t, pitchTau)) * perc(t, attack, ampTau)
	}).tail(0.025)
}

// softTone is a rounded sine tone with optional second and third harmonics.
func softTone(sec, freq, attack, tau, h2, h3 float64) track {
	var o osc
	return gen(sec, func(t float64) float64 {
		ph := o.ph
		o.advance(freq)
		return (sinT(ph) + h2*sinT(2*ph) + h3*sinT(3*ph)) * perc(t, attack, tau)
	}).tail(0.025)
}

// fmTone is a two-operator phase-modulation voice. ratio is modulator/carrier frequency and
// index the peak modulation depth in radians, which decays with idxTau so the tone starts
// bright and mellows — the basis of the chimes and bells.
func fmTone(sec, freq, ratio, index, idxTau, attack, ampTau float64) track {
	var carrier, mod float64
	return gen(sec, func(t float64) float64 {
		m := sinT(mod) * index * math.Exp(-t/idxTau) / (2 * math.Pi)
		v := sinT(carrier+m) * perc(t, attack, ampTau)
		carrier += freq * invSR
		mod += freq * ratio * invSR
		return v
	}).tail(0.025)
}

// brassTone is a warm sustained voice: two detuned sawtooths through a low-pass whose cutoff
// opens quickly to cutPeak and settles to cutRest. The amplitude follows a trapezoid.
func brassTone(sec, freq, attack, release, cutPeak, cutRest float64) track {
	var o1, o2 osc
	var f svf
	i := 0
	return gen(sec, func(t float64) float64 {
		if i&15 == 0 {
			open := perc(t, attack*1.5, sec*0.35)
			f.set(cutRest+(cutPeak-cutRest)*open, 0.8)
		}
		i++
		v := o1.saw(freq*0.9972) + o2.saw(freq*1.0028)
		return f.low(v) * 0.5 * trap(t, sec, attack, release)
	})
}

// grains scatters count short noise bursts between t0 and t1, each band-passed somewhere
// between lo and hi Hz and fading as time goes on: debris, crackle, sparks.
func grains(r *rng, sec float64, count int, t0, t1, lo, hi, minTau, maxTau float64) track {
	out := make(track, secondsToSamples(sec))
	for i := 0; i < count; i++ {
		at := r.rangef(t0, t1)
		tau := r.rangef(minTau, maxTau)
		level := r.rangef(0.35, 1) * (1 - 0.7*(at-t0)/(t1-t0))
		g := noise(r, tau*6).bandpass(glide(lo, hi, r.float()), 1.4).shape(percEnv(0.0005, tau))
		out = out.add(g, at, level)
	}
	return out[:secondsToSamples(sec)]
}

// stereo is an offline stereo buffer; both channels always have the same length.
type stereo struct{ l, r track }

// centre places a mono track in the middle of the stereo field.
func centre(m track) stereo {
	r := make(track, len(m))
	copy(r, m)
	return stereo{l: m, r: r}
}

// widen turns a mono track into gentle pseudo-stereo: a delayed, high-passed copy is added to
// one side and subtracted from the other, so the sum (mono) is unchanged.
func widen(m track, amount float64) stereo {
	const delay = 0.011
	d := secondsToSamples(delay)
	side := make(track, len(m)+d)
	copy(side[d:], m)
	side.highpass(350, 0.7)
	out := stereo{l: make(track, len(side)), r: make(track, len(side))}
	for i := range side {
		v := 0.0
		if i < len(m) {
			v = m[i]
		}
		out.l[i] = v + amount*side[i]
		out.r[i] = v - amount*side[i]
	}
	return out
}

// addPan mixes a mono track into the stereo buffer at a time, gain and pan (-1 left .. +1
// right, constant power).
func (s stereo) addPan(m track, at, gain, pan float64) stereo {
	angle := (clamp(pan, -1, 1) + 1) * math.Pi / 4
	s.l = s.l.add(m, at, gain*math.Cos(angle)*math.Sqrt2)
	s.r = s.r.add(m, at, gain*math.Sin(angle)*math.Sqrt2)
	if len(s.l) < len(s.r) {
		s.l = append(s.l, make(track, len(s.r)-len(s.l))...)
	}
	if len(s.r) < len(s.l) {
		s.r = append(s.r, make(track, len(s.l)-len(s.r))...)
	}
	return s
}

// room adds reverberation: wet is the reverb level relative to the dry signal, size and rt60
// describe the space, and tail extends the buffer so the decay is not cut short.
func (s stereo) room(wet, size, rt60, damping, tail float64) stereo {
	n := len(s.l) + secondsToSamples(tail)
	rv := newReverb(size, rt60, damping, 110)
	out := stereo{l: make(track, n), r: make(track, n)}
	for i := 0; i < n; i++ {
		var l, r float64
		if i < len(s.l) {
			l, r = s.l[i], s.r[i]
		}
		wl, wr := rv.tick(l, r)
		out.l[i] = l + wet*wl
		out.r[i] = r + wet*wr
	}
	return out
}

// finalize turns a recipe's raw output into a mix-ready buffer: removes DC and sub-sonic
// content, trims trailing near-silence, fades both ends, levels the sound to the target
// loudness (never letting the peak pass -1 dBFS) and interleaves it.
func finalize(s stereo, targetDB, fadeOut float64) []float32 {
	n := min(len(s.l), len(s.r))
	l, r := s.l[:n], s.r[:n]
	l.highpass(24, 0.707)
	r.highpass(24, 0.707)

	// Trim the tail once it has fallen 60 dB below the peak.
	peak := math.Max(peakOf(l), peakOf(r))
	if peak < 1e-9 {
		return make([]float32, 2*n)
	}
	floor := peak * 0.001
	end := 0
	for i := n - 1; i >= 0; i-- {
		if math.Abs(l[i]) > floor || math.Abs(r[i]) > floor {
			end = i + 1
			break
		}
	}
	n = min(n, end+secondsToSamples(0.004))
	l, r = l[:n], r[:n]

	fadeIn := min(secondsToSamples(0.0006), n/4)
	for i := 0; i < fadeIn; i++ {
		g := 0.5 - 0.5*math.Cos(math.Pi*float64(i)/float64(fadeIn))
		l[i] *= g
		r[i] *= g
	}
	fade := min(secondsToSamples(fadeOut), n/2)
	for i := 0; i < fade; i++ {
		g := 0.5 - 0.5*math.Cos(math.Pi*float64(i)/float64(fade))
		l[n-1-i] *= g
		r[n-1-i] *= g
	}

	gain := dbToLin(targetDB - loudnessDB(l, r))
	const ceiling = 0.89
	if peak = math.Max(peakOf(l), peakOf(r)); peak*gain > ceiling {
		gain = ceiling / peak
	}
	out := make([]float32, 2*n)
	for i := 0; i < n; i++ {
		out[2*i] = float32(l[i] * gain)
		out[2*i+1] = float32(r[i] * gain)
	}
	return out
}
