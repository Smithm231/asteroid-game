package audio

// Mood selects the character of the generative music.
type Mood int

// The music moods. All share the tonic D, so cross-fades between them stay related.
const (
	Menu    Mood = iota // title and menus: floating, luminous (D Lydian)
	Calm                // ordinary play: slow and contemplative (D Dorian)
	Tense               // a threat is near: darker, with a slow heartbeat (D Phrygian)
	Battle              // combat: driving pulse and ostinato (D Aeolian)
	Triumph             // victory: bright and moving (D Ionian)
	Lament              // defeat or loss: very slow, dark (D Aeolian)
	moodCount
)

// NumMoods is the number of moods; valid values are 0 <= m < NumMoods.
const NumMoods = int(moodCount)

// Valid reports whether m names an existing mood.
func (m Mood) Valid() bool { return m >= 0 && int(m) < NumMoods }

// String returns the mood's name as written in the constant list, e.g. "Battle".
func (m Mood) String() string {
	if !m.Valid() {
		return "Mood(invalid)"
	}
	return moodSpecs[m].name
}

// AllMoods returns every mood in declaration order.
func AllMoods() []Mood {
	all := make([]Mood, NumMoods)
	for i := range all {
		all[i] = Mood(i)
	}
	return all
}

// Scale modes as semitone offsets of the seven degrees above the tonic.
var (
	modeLydian   = [7]int{0, 2, 4, 6, 7, 9, 11}
	modeIonian   = [7]int{0, 2, 4, 5, 7, 9, 11}
	modeDorian   = [7]int{0, 2, 3, 5, 7, 9, 10}
	modeAeolian  = [7]int{0, 2, 3, 5, 7, 8, 10}
	modePhrygian = [7]int{0, 1, 3, 5, 7, 8, 10}
)

// Five-note subsets (as scale degrees) that melodies draw from: every note of these sits well
// over every chord of its mode, which is what keeps the improvised lines from sounding random.
var (
	pentaMajor = [5]int{0, 1, 2, 4, 5} // 1 2 3 5 6
	pentaMinor = [5]int{0, 2, 3, 4, 6} // 1 b3 4 5 b7
)

// Bass behaviours.
const (
	bassSustain = iota // one long note per chord, gliding between roots
	bassBeat           // sustained, swelling gently on every beat
	bassDrive          // re-struck every eighth note
)

// moodSpec is the full parameter set of one mood: harmony, tempo, the sound of each layer and
// the layer balance.
type moodSpec struct {
	name string

	// Harmony and time.
	mode         [7]int
	penta        [5]int
	progs        [][]int // chord progressions as scale degrees; none uses the diminished triad
	bpm          float64
	barsPerChord int
	colour       float64 // probability of adding a seventh or ninth to a chord

	// Pad: slow chords from detuned sawtooths and sines through a drifting low-pass.
	padLo, padHi          int     // MIDI register of the voicing
	padAttack, padRelease float64 // seconds
	padCutoff             float64 // Hz, centre of the filter sweep (brightness)
	padMove               float64 // octaves the cutoff drifts either way
	padGain               float64

	// Pluck: sparse FM motifs with a ping-pong echo.
	melBase            int     // MIDI note of melody index 0 (a tonic)
	density            float64 // probability that a bar carries the motif
	motifMin, motifMax int     // notes per motif
	offbeat            float64 // weight of sixteenth-note off-beats in motif rhythms
	pluckRatio         float64 // FM modulator ratio (timbre)
	pluckIndex         float64 // FM depth at the attack (brightness)
	pluckDecay         float64 // seconds
	pluckGain          float64
	echo               float64 // echo level and feedback (0..0.6)

	// Sub-bass on chord roots.
	bassLo    int // lowest MIDI note; roots are placed in the octave above it
	bassStyle int
	bassGain  float64

	// Wind: band-passed noise wandering between two frequencies.
	windLo, windHi float64
	windGain       float64

	// Pulse: per-sixteenth velocities of the low drum, the high drum and the soft noise tick.
	drumLow, drumHigh, tick [16]float64
	percGain                float64

	reverbWet, reverbTime float64
	gain                  float64 // output trim that levels the moods against each other
}

var moodSpecs = [NumMoods]moodSpec{
	Menu: {
		name: "Menu", mode: modeLydian, penta: pentaMajor,
		progs: [][]int{{0, 1, 0, 5}, {0, 5, 1, 0}, {0, 2, 1, 0}, {0, 1, 5, 4}},
		bpm:   66, barsPerChord: 2, colour: 0.6,
		padLo: 50, padHi: 74, padAttack: 2.2, padRelease: 3.6, padCutoff: 1250, padMove: 0.5, padGain: 1,
		melBase: 62, density: 0.5, motifMin: 2, motifMax: 4, offbeat: 0,
		pluckRatio: 3.5, pluckIndex: 1.4, pluckDecay: 1.2, pluckGain: 1, echo: 0.42,
		bassLo: 36, bassStyle: bassSustain, bassGain: 1,
		windLo: 300, windHi: 1100, windGain: 0.8,
		reverbWet: 0.32, reverbTime: 3.6, gain: 0.69,
	},
	Calm: {
		name: "Calm", mode: modeDorian, penta: pentaMinor,
		progs: [][]int{{0, 3, 0, 6}, {0, 6, 3, 0}, {0, 2, 3, 0}, {0, 3, 6, 3}, {0, 4, 3, 0}},
		bpm:   58, barsPerChord: 2, colour: 0.55,
		padLo: 50, padHi: 74, padAttack: 2.4, padRelease: 3.8, padCutoff: 1000, padMove: 0.55, padGain: 1,
		melBase: 62, density: 0.42, motifMin: 2, motifMax: 4, offbeat: 0,
		pluckRatio: 2, pluckIndex: 2.0, pluckDecay: 1.4, pluckGain: 1.4, echo: 0.42,
		bassLo: 36, bassStyle: bassSustain, bassGain: 1,
		windLo: 260, windHi: 900, windGain: 1,
		reverbWet: 0.32, reverbTime: 3.8, gain: 0.70,
	},
	Tense: {
		name: "Tense", mode: modePhrygian, penta: pentaMinor,
		progs: [][]int{{0, 1, 0, 6}, {0, 1, 2, 1}, {0, 6, 1, 0}, {0, 3, 1, 0}},
		bpm:   84, barsPerChord: 2, colour: 0.25,
		padLo: 48, padHi: 72, padAttack: 1.6, padRelease: 2.8, padCutoff: 800, padMove: 0.6, padGain: 1,
		melBase: 62, density: 0.42, motifMin: 2, motifMax: 4, offbeat: 0.15,
		pluckRatio: 3, pluckIndex: 1.9, pluckDecay: 0.9, pluckGain: 0.9, echo: 0.45,
		bassLo: 36, bassStyle: bassBeat, bassGain: 1.3,
		windLo: 220, windHi: 800, windGain: 1.1,
		drumLow:  [16]float64{0: 1, 3: 0.55, 8: 0.8, 11: 0.45},
		tick:     [16]float64{2: 0.25, 4: 0.6, 6: 0.25, 10: 0.25, 12: 0.6, 14: 0.25},
		percGain: 1, reverbWet: 0.3, reverbTime: 3.2, gain: 0.72,
	},
	Battle: {
		name: "Battle", mode: modeAeolian, penta: pentaMinor,
		progs: [][]int{{0, 5, 6, 0}, {0, 5, 2, 6}, {0, 3, 5, 6}, {0, 6, 5, 6}},
		bpm:   112, barsPerChord: 2, colour: 0.15,
		padLo: 50, padHi: 74, padAttack: 0.6, padRelease: 1.4, padCutoff: 1500, padMove: 0.45, padGain: 0.9,
		melBase: 62, density: 0.85, motifMin: 5, motifMax: 7, offbeat: 0.5,
		pluckRatio: 2, pluckIndex: 2.6, pluckDecay: 0.32, pluckGain: 1.2, echo: 0.3,
		bassLo: 36, bassStyle: bassDrive, bassGain: 1.8,
		windLo: 300, windHi: 1000, windGain: 0.6,
		drumLow:  [16]float64{0: 1, 6: 0.7, 8: 0.9, 14: 0.6},
		drumHigh: [16]float64{3: 0.6, 11: 0.6, 12: 0.45},
		tick: [16]float64{0.35, 0.16, 0.35, 0.16, 0.6, 0.16, 0.35, 0.16,
			0.35, 0.16, 0.35, 0.16, 0.6, 0.16, 0.35, 0.2},
		percGain: 1, reverbWet: 0.22, reverbTime: 2.2, gain: 0.66,
	},
	Triumph: {
		name: "Triumph", mode: modeIonian, penta: pentaMajor,
		progs: [][]int{{0, 3, 5, 4}, {0, 4, 5, 3}, {0, 2, 3, 4}, {0, 3, 0, 4}},
		bpm:   96, barsPerChord: 1, colour: 0.4,
		padLo: 52, padHi: 76, padAttack: 0.5, padRelease: 1.6, padCutoff: 1900, padMove: 0.4, padGain: 0.95,
		melBase: 62, density: 0.75, motifMin: 3, motifMax: 5, offbeat: 0.1,
		pluckRatio: 2, pluckIndex: 2.4, pluckDecay: 0.6, pluckGain: 1, echo: 0.36,
		bassLo: 36, bassStyle: bassBeat, bassGain: 1,
		windLo: 400, windHi: 1300, windGain: 0.5,
		drumLow:  [16]float64{0: 0.9, 8: 0.7, 10: 0.4},
		tick:     [16]float64{2: 0.2, 4: 0.5, 6: 0.2, 10: 0.2, 12: 0.5, 14: 0.2},
		percGain: 0.8, reverbWet: 0.28, reverbTime: 2.8, gain: 0.62,
	},
	Lament: {
		name: "Lament", mode: modeAeolian, penta: pentaMinor,
		progs: [][]int{{0, 3, 5, 4}, {0, 5, 3, 0}, {0, 6, 5, 4}, {0, 4, 5, 3}},
		bpm:   50, barsPerChord: 2, colour: 0.5,
		padLo: 47, padHi: 71, padAttack: 3, padRelease: 4.2, padCutoff: 680, padMove: 0.5, padGain: 1,
		melBase: 50, density: 0.34, motifMin: 2, motifMax: 3, offbeat: 0,
		pluckRatio: 1, pluckIndex: 1.5, pluckDecay: 2.2, pluckGain: 1, echo: 0.45,
		bassLo: 36, bassStyle: bassSustain, bassGain: 1.1,
		windLo: 200, windHi: 700, windGain: 1.2,
		reverbWet: 0.4, reverbTime: 4.6, gain: 0.77,
	},
}
