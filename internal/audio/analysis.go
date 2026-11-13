package audio

import "math"

// Measurement helpers used at render time to level the sound effects against each other.

// peakOf returns the largest absolute sample value.
func peakOf(x []float64) float64 {
	p := 0.0
	for _, v := range x {
		if a := math.Abs(v); a > p {
			p = a
		}
	}
	return p
}

// earWeight returns a perceptually weighted copy of x: a gentle high shelf (+4 dB above about
// 1.5 kHz) followed by a 120 Hz high-pass (-6 dB at 120 Hz, -12 dB at 60 Hz). It approximates
// the ear's reduced sensitivity to bass at moderate listening levels, so that a sound made
// mostly of sub-bass is not mistaken for a loud one.
func earWeight(x []float64) []float64 {
	var shelf biquad
	shelf.setHighShelf(1500, 4)
	var hp svf
	hp.set(120, 0.5)
	out := make([]float64, len(x))
	for i, v := range x {
		out[i] = hp.high(shelf.tick(v))
	}
	return out
}

// loudnessDB estimates how loud a stereo sound is perceived: the highest mean-square level of
// the ear-weighted signal over any 100 ms window, in dBFS. Sounds shorter than the window are
// measured against the full window, so a brief tick reads quieter than a sustained tone of the
// same amplitude — just as it is heard.
func loudnessDB(l, r []float64) float64 {
	wl, wr := earWeight(l), earWeight(r)
	n := min(len(wl), len(wr))
	if n == 0 {
		return -200
	}
	win := secondsToSamples(0.100)
	hop := secondsToSamples(0.010)
	// Prefix sums of the per-frame energy make every window O(1).
	prefix := make([]float64, n+1)
	for i := 0; i < n; i++ {
		prefix[i+1] = prefix[i] + (wl[i]*wl[i]+wr[i]*wr[i])/2
	}
	best := 0.0
	for start := 0; ; start += hop {
		end := min(start+win, n)
		if e := (prefix[end] - prefix[start]) / float64(win); e > best {
			best = e
		}
		if end == n {
			break
		}
	}
	return linToDb(math.Sqrt(best))
}
