package audio

// The composing half of the music engine: scales, chords, voice-leading and motifs. Nothing in
// this file makes sound; music.go turns its decisions into audio.

// tonicPC is the pitch class of the home note of every mood (D).
const tonicPC = 2

// melodySpan is the number of melody-scale indices a line may roam (two octaves and a tonic).
const melodySpan = 11

// floorDiv divides rounding toward negative infinity.
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func absInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// pcOf returns the pitch class (0..11) of a MIDI note.
func pcOf(note int) int { return ((note % 12) + 12) % 12 }

// degreePC returns the pitch class of a scale degree (any integer; 7 is the octave).
func (sp *moodSpec) degreePC(degree int) int {
	return pcOf(tonicPC + sp.mode[degree-7*floorDiv(degree, 7)])
}

// inMode reports whether a MIDI note belongs to the mood's scale.
func (sp *moodSpec) inMode(note int) bool {
	pc := pcOf(note)
	for d := 0; d < 7; d++ {
		if sp.degreePC(d) == pc {
			return true
		}
	}
	return false
}

// melPitch maps a melody-scale index to a MIDI note: index 0 is melBase and every five indices
// climb an octave through the mood's pentatonic subset.
func (sp *moodSpec) melPitch(k int) int {
	oct := floorDiv(k, 5)
	return sp.melBase + 12*oct + sp.mode[sp.penta[k-5*oct]]
}

// Chord colours.
const (
	colourNone = iota
	colourSeventh
	colourNinth
)

// chord is a diatonic chord as pitch classes: root, third, fifth and optionally one colour tone.
type chord struct {
	degree int
	pcs    [4]int
	n      int // 3 for a triad, 4 with a colour tone
}

// colourPC returns the pitch class a colour would add to the triad on degree, and whether it
// is consonant there: it must not double a chord tone, sit a semitone above one (the harsh
// minor ninth) or form a tritone with one (which would turn the chord into a restless dominant).
func (sp *moodSpec) colourPC(degree, colour int) (int, bool) {
	var pc int
	switch colour {
	case colourSeventh:
		pc = sp.degreePC(degree + 6)
	case colourNinth:
		pc = sp.degreePC(degree + 1)
	default:
		return 0, false
	}
	for _, step := range []int{0, 2, 4} {
		switch (pc - sp.degreePC(degree+step) + 12) % 12 {
		case 0, 1, 6:
			return pc, false
		}
	}
	return pc, true
}

// buildChord returns the triad on a scale degree, with the colour tone when it is consonant.
func (sp *moodSpec) buildChord(degree, colour int) chord {
	c := chord{degree: degree, n: 3}
	c.pcs[0] = sp.degreePC(degree)
	c.pcs[1] = sp.degreePC(degree + 2)
	c.pcs[2] = sp.degreePC(degree + 4)
	if pc, ok := sp.colourPC(degree, colour); ok {
		c.pcs[3] = pc
		c.n = 4
	}
	return c
}

// has reports whether a note is one of the chord's tones (including the colour).
func (c *chord) has(note int) bool {
	pc := pcOf(note)
	for i := 0; i < c.n; i++ {
		if c.pcs[i] == pc {
			return true
		}
	}
	return false
}

// inTriad reports whether a note is the chord's root, third or fifth.
func (c *chord) inTriad(note int) bool {
	pc := pcOf(note)
	return pc == c.pcs[0] || pc == c.pcs[1] || pc == c.pcs[2]
}

// grates reports whether a note sits a semitone above one of the triad's tones — the classic
// "avoid note" (such as the fourth over a major chord), which sounds wrong when it lingers.
func (c *chord) grates(note int) bool {
	pc := pcOf(note)
	for i := 0; i < 3; i++ {
		if (pc-c.pcs[i]+12)%12 == 1 {
			return true
		}
	}
	return false
}

// voiceChord chooses four pad notes for a chord inside the register lo..hi. It searches every
// combination of chord tones and keeps the one with the lowest cost, where the cost rewards
// complete chords, open and clean spacing (no semitone rubs, no low clusters) and — given the
// previous voicing — the smallest possible movement of each voice. The result is ascending.
func voiceChord(c chord, prev [4]int, havePrev bool, lo, hi int) [4]int {
	var cand []int
	for n := lo; n <= hi; n++ {
		if c.has(n) {
			cand = append(cand, n)
		}
	}
	if len(cand) < 4 {
		// A register too narrow to voice in; stack the triad from lo instead.
		var v [4]int
		base := lo
		for pcOf(base) != c.pcs[0] {
			base++
		}
		for i := range v {
			v[i] = base
			for step := 1; step <= 12; step++ {
				if c.inTriad(base + step) {
					base += step
					break
				}
			}
		}
		return v
	}
	best := [4]int{cand[0], cand[1], cand[2], cand[3]}
	bestCost := 1e18
	for a := 0; a < len(cand)-3; a++ {
		for b := a + 1; b < len(cand)-2; b++ {
			for d := b + 1; d < len(cand)-1; d++ {
				for e := d + 1; e < len(cand); e++ {
					v := [4]int{cand[a], cand[b], cand[d], cand[e]}
					if cost := voicingCost(c, v, prev, havePrev, lo, hi); cost < bestCost {
						best, bestCost = v, cost
					}
				}
			}
		}
	}
	return best
}

// voicingCost scores an ascending four-note voicing; lower is better.
func voicingCost(c chord, v, prev [4]int, havePrev bool, lo, hi int) float64 {
	cost := 0.0
	var count [4]int
	for _, n := range v {
		for i := 0; i < c.n; i++ {
			if c.pcs[i] == pcOf(n) {
				count[i]++
			}
		}
	}
	if count[1] == 0 {
		cost += 40 // the third gives the chord its character
	}
	if count[0] == 0 {
		cost += 12
	}
	if c.n == 4 && count[3] == 0 {
		cost += 6
	}
	if count[1] > 1 {
		cost += 3
	}
	if count[3] > 1 {
		cost += 10
	}
	if c.n == 4 {
		// A colour tone belongs in the upper voices; at the bottom it muddies the bass.
		switch c.pcs[3] {
		case pcOf(v[0]):
			cost += 60 // never: it would sound like a different chord
		case pcOf(v[1]):
			cost += 3
		}
	}
	for i := 0; i < 3; i++ {
		gap := v[i+1] - v[i]
		switch {
		case gap == 1:
			cost += 100 // semitone rub
		case gap == 2 && v[i] < 60:
			cost += 8 // a whole-tone cluster is muddy in the low register
		case gap > 12:
			cost += 3 * float64(gap-12)
		}
		if gap < 4 && v[i] < 52 {
			cost += 10
		}
	}
	for i := 0; i < 4; i++ {
		for j := i + 1; j < 4; j++ {
			switch (v[j] - v[i]) % 12 {
			case 1:
				cost += 30 // minor ninth
			case 11:
				cost += 4 // major seventh: allowed, but not for free
			}
		}
	}
	if span := v[3] - v[0]; span < 10 {
		cost += 2 * float64(10-span)
	} else if span > 22 {
		cost += 2 * float64(span-22)
	}
	if havePrev {
		for i := range v {
			cost += float64(absInt(v[i] - prev[i]))
		}
		cost += 0.5 * float64(absInt(v[3]-prev[3])) // a smooth top line matters most
	} else {
		centre := float64(lo+hi) / 2
		mean := float64(v[0]+v[1]+v[2]+v[3]) / 4
		if mean > centre {
			cost += 0.5 * (mean - centre)
		} else {
			cost += 0.5 * (centre - mean)
		}
	}
	return cost
}

// bassNote places a root pitch class in the octave starting at lo.
func bassNote(rootPC, lo int) int {
	n := lo
	for pcOf(n) != rootPC {
		n++
	}
	return n
}

// motif is a short melodic cell: onsets within one bar of sixteen steps, each with a melodic
// offset (in melody-scale indices, relative to an anchor chosen per chord) and a velocity. A
// motif is reused over several chords, which is what makes the plucked lines sound composed.
type motif struct {
	n    int
	step [8]int
	rel  [8]int
	vel  [8]float64
}

// motifWeights are the base probabilities of an onset on each sixteenth of the bar: beats
// first, then the eighths between them; odd sixteenths get the mood's off-beat weight.
var motifWeights = [16]float64{3, 0, 1.2, 0, 2, 0, 1.5, 0, 2.5, 0, 1.2, 0, 2, 0, 1.5, 0}

// newMotif invents a motif for the mood.
func newMotif(r *rng, sp *moodSpec) motif {
	var m motif
	m.n = sp.motifMin + r.intn(sp.motifMax-sp.motifMin+1)
	if m.n > len(m.step) {
		m.n = len(m.step)
	}
	weights := motifWeights
	for i := 1; i < 16; i += 2 {
		weights[i] = sp.offbeat
	}
	var used [16]bool
	for i := 0; i < m.n; i++ {
		s := r.pick(weights[:])
		weights[s] = 0
		used[s] = true
	}
	i := 0
	rel := 0
	for s, on := range used {
		if !on {
			continue
		}
		m.step[i] = s
		if i > 0 {
			// Mostly steps, some thirds, the occasional leap; pulled back toward the anchor
			// so a line never runs away.
			delta := []int{-2, -1, 1, 2, 3, -3}[r.pick([]float64{1.2, 3, 3, 1.2, 0.4, 0.4})]
			if rel+delta > 4 || rel+delta < -3 {
				delta = -delta
			}
			rel += delta
		}
		m.rel[i] = rel
		m.vel[i] = r.rangef(0.62, 0.85)
		if s%4 == 0 {
			m.vel[i] += 0.15
		}
		i++
	}
	m.n = i
	return m
}

// anchorIndex picks the melody index a motif is hung from for a chord: the chord tone nearest
// to the line's last note, nudged toward the middle of the range when the line has drifted to
// an edge.
func (sp *moodSpec) anchorIndex(c *chord, last int) int {
	order := [7]int{0, -1, 1, -2, 2, -3, 3}
	if last < melodySpan/2 {
		order = [7]int{0, 1, -1, 2, -2, 3, -3}
	}
	lo, hi := 2, melodySpan-4
	for _, d := range order {
		k := last + d
		if k < lo || k > hi {
			continue
		}
		if c.inTriad(sp.melPitch(k)) {
			return k
		}
	}
	return int(clamp(float64(last), float64(lo), float64(hi)))
}

// fitIndex adjusts a melody index so its note suits the chord. On a strong beat the note must
// belong to the triad; anywhere else it only has to avoid grating (see chord.grates). When k
// itself does not fit, the neighbours are tried, first in the direction dir the line is moving
// (-1 down, +1 up), so a correction keeps the contour instead of flattening it. avoid, when
// inside the range, is an index the result should differ from (the previous note, so that a
// line that is meant to move does not stall on a repeated pitch).
func (sp *moodSpec) fitIndex(c *chord, k int, strong bool, dir, avoid int) int {
	fits := func(j int) bool {
		if j < 0 || j >= melodySpan || j == avoid {
			return false
		}
		note := sp.melPitch(j)
		if strong {
			return c.inTriad(note)
		}
		return !c.grates(note)
	}
	if fits(k) {
		return k
	}
	first := 1
	if dir < 0 {
		first = -1
	}
	for _, d := range [6]int{first, -first, 2 * first, -2 * first, 3 * first, -3 * first} {
		if fits(k + d) {
			return k + d
		}
	}
	if avoid >= 0 {
		// Nothing else fits nearby: repeating the previous note beats a wrong one.
		return sp.fitIndex(c, k, strong, dir, -1)
	}
	return int(clamp(float64(k), 0, melodySpan-1))
}
