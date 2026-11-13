package audio

import "math"

// reverbLines are the base delay lengths (samples at 44.1 kHz, size 1.0) of the feedback delay
// network. They are distinct primes between roughly 20 and 47 ms so that echoes never line up.
var reverbLines = [8]int{887, 1009, 1163, 1319, 1493, 1657, 1861, 2053}

// reverbDiffusers are the base lengths of the four input all-pass filters (two per channel).
var reverbDiffusers = [4]int{149, 211, 173, 229}

// reverb is a small stereo feedback delay network: two all-pass diffusers per input channel
// feed eight delay lines that are mixed through a Hadamard matrix, with a one-pole low-pass in
// every loop for high-frequency damping. It is cheap enough to run in the real-time music path
// and is also used offline to give sound effects a tail.
type reverb struct {
	line  [8][]float64
	pos   [8]int
	gain  [8]float64
	lp    [8]float64
	damp  float64
	ap    [4][]float64
	apPos [4]int
	lowL  onePole
	lowR  onePole
	norm  float64
}

// newReverb builds a reverb. size scales the room (0.3 = small booth, 1 = hall, up to 2),
// rt60 is the decay time in seconds, damping (0..1) darkens the tail and lowCut removes
// rumble from the input so bass does not smear.
func newReverb(size, rt60, damping, lowCut float64) *reverb {
	size = clamp(size, 0.2, 2)
	rt60 = clamp(rt60, 0.05, 20)
	r := &reverb{damp: clamp(damping, 0, 0.95)}
	meanSq := 0.0
	for i, base := range reverbLines {
		n := int(float64(base)*size) | 1 // odd lengths keep the lines mutually out of step
		n += 2 * i
		r.line[i] = make([]float64, n)
		r.gain[i] = math.Pow(10, -3*float64(n)/(rt60*sr))
		meanSq += r.gain[i] * r.gain[i] / 8
	}
	for i, base := range reverbDiffusers {
		n := int(float64(base)*size) | 1
		r.ap[i] = make([]float64, n)
	}
	r.lowL.set(lowCut)
	r.lowR.set(lowCut)
	// Scale the wet signal so a steady input comes back at roughly its own level whatever the
	// decay time is; callers then choose a wet amount without worrying about rt60.
	r.norm = 0.5 * math.Sqrt(1-meanSq)
	return r
}

func (r *reverb) allpass(i int, x float64) float64 {
	const g = 0.6
	buf := r.ap[i]
	p := r.apPos[i]
	d := buf[p]
	v := x - g*d
	buf[p] = v
	p++
	if p == len(buf) {
		p = 0
	}
	r.apPos[i] = p
	return d + g*v
}

// tick feeds one stereo frame and returns the wet (reverberated) frame only.
func (r *reverb) tick(inL, inR float64) (float64, float64) {
	inL = r.allpass(1, r.allpass(0, r.lowL.high(inL)))
	inR = r.allpass(3, r.allpass(2, r.lowR.high(inR)))

	var x [8]float64
	keep := 1 - r.damp
	for i := range x {
		v := r.line[i][r.pos[i]]
		r.lp[i] += (v - r.lp[i]) * keep
		x[i] = r.lp[i] * r.gain[i]
	}
	outL := (x[0] - x[2] + x[4] - x[6]) * r.norm
	outR := (x[1] - x[3] + x[5] - x[7]) * r.norm

	// 8×8 Hadamard mix (fast Walsh–Hadamard transform, orthonormal after scaling).
	a0, a1, a2, a3 := x[0]+x[1], x[0]-x[1], x[2]+x[3], x[2]-x[3]
	a4, a5, a6, a7 := x[4]+x[5], x[4]-x[5], x[6]+x[7], x[6]-x[7]
	b0, b1, b2, b3 := a0+a2, a1+a3, a0-a2, a1-a3
	b4, b5, b6, b7 := a4+a6, a5+a7, a4-a6, a5-a7
	const s = 0.35355339059327373 // 1/sqrt(8)
	x[0], x[1], x[2], x[3] = (b0+b4)*s, (b1+b5)*s, (b2+b6)*s, (b3+b7)*s
	x[4], x[5], x[6], x[7] = (b0-b4)*s, (b1-b5)*s, (b2-b6)*s, (b3-b7)*s

	for i := range x {
		in := inL
		if i&1 == 1 {
			in = inR
		}
		p := r.pos[i]
		r.line[i][p] = in + x[i]
		p++
		if p == len(r.line[i]) {
			p = 0
		}
		r.pos[i] = p
	}
	return outL, outR
}
