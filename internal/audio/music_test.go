package audio

import (
	"fmt"
	"math"
	"slices"
	"sync"
	"testing"
)

// musicClip caches one offline render per mood for the tests that only read it.
var musicClip = func() func(m Mood) []float32 {
	var once [NumMoods]sync.Once
	var clips [NumMoods][]float32
	return func(m Mood) []float32 {
		once[m].Do(func() { clips[m] = RenderMusic(m, 40, 11) })
		return clips[m]
	}
}()

func TestMoodSetMatchesSpec(t *testing.T) {
	want := []string{"Menu", "Calm", "Tense", "Battle", "Triumph", "Lament"}
	if NumMoods != len(want) {
		t.Fatalf("NumMoods = %d, want %d", NumMoods, len(want))
	}
	for i, m := range AllMoods() {
		if int(m) != i || !m.Valid() || m.String() != want[i] {
			t.Errorf("mood %d = %q (valid %v), want %q", i, m.String(), m.Valid(), want[i])
		}
	}
	for _, bad := range []Mood{-1, Mood(NumMoods), 99} {
		if bad.Valid() || bad.String() != "Mood(invalid)" {
			t.Errorf("Mood(%d) must be invalid", int(bad))
		}
	}
}

func TestMoodSpecsAreWellFormed(t *testing.T) {
	for _, m := range AllMoods() {
		sp := &moodSpecs[m]
		if sp.mode[0] != 0 || !slices.IsSorted(sp.mode[:]) || sp.mode[6] > 11 {
			t.Errorf("%s: mode %v is not an ascending scale from the tonic", m, sp.mode)
		}
		if !slices.IsSorted(sp.penta[:]) || sp.penta[0] != 0 || sp.penta[4] > 6 {
			t.Errorf("%s: melody subset %v must be ascending scale degrees from the tonic", m, sp.penta)
		}
		if sp.bpm < 40 || sp.bpm > 140 || sp.barsPerChord < 1 {
			t.Errorf("%s: tempo %v bpm / %d bars per chord", m, sp.bpm, sp.barsPerChord)
		}
		if len(sp.progs) < 3 {
			t.Errorf("%s: only %d progressions; the harmony would repeat too soon", m, len(sp.progs))
		}
		for _, prog := range sp.progs {
			if len(prog) == 0 || prog[0] != 0 {
				t.Errorf("%s: progression %v must start on the tonic chord", m, prog)
			}
			for _, degree := range prog {
				c := sp.buildChord(degree, colourNone)
				third := (c.pcs[1] - c.pcs[0] + 12) % 12
				fifth := (c.pcs[2] - c.pcs[0] + 12) % 12
				if fifth != 7 || (third != 3 && third != 4) {
					t.Errorf("%s: degree %d in %v is not a major or minor triad (third %d, fifth %d)",
						m, degree, prog, third, fifth)
				}
			}
		}
		if sp.motifMin < 1 || sp.motifMin > sp.motifMax || sp.motifMax > 8 {
			t.Errorf("%s: motif size %d..%d", m, sp.motifMin, sp.motifMax)
		}
		if sp.padHi-sp.padLo < 20 || pcOf(sp.melBase) != tonicPC {
			t.Errorf("%s: pad register %d..%d or melody base %d is unusable", m, sp.padLo, sp.padHi, sp.melBase)
		}
		if sp.density <= 0 || sp.density > 1 || sp.colour < 0 || sp.colour > 1 || sp.gain <= 0 || sp.gain > 1 {
			t.Errorf("%s: density %v, colour %v or gain %v out of range", m, sp.density, sp.colour, sp.gain)
		}
		// A chord must outlast the pad's release, or released voices pile up.
		chordSec := float64(sp.barsPerChord) * 4 * 60 / sp.bpm
		if sp.padRelease > chordSec {
			t.Errorf("%s: pad release %.1f s exceeds the chord length %.1f s", m, sp.padRelease, chordSec)
		}
	}
	// Calm really is the calmest playing mood, Battle the most driven.
	if moodSpecs[Calm].bpm >= moodSpecs[Tense].bpm || moodSpecs[Tense].bpm >= moodSpecs[Battle].bpm {
		t.Error("tempo should rise from Calm through Tense to Battle")
	}
	if moodSpecs[Calm].density >= moodSpecs[Battle].density {
		t.Error("Battle should be denser than Calm")
	}
	if moodSpecs[Lament].padCutoff >= moodSpecs[Calm].padCutoff || moodSpecs[Calm].padCutoff >= moodSpecs[Triumph].padCutoff {
		t.Error("brightness should rise from Lament through Calm to Triumph")
	}
	for _, m := range AllMoods() {
		pulsed := moodSpecs[m].percGain > 0
		if want := m == Tense || m == Battle || m == Triumph; pulsed != want {
			t.Errorf("%s: pulse layer present = %v, want %v", m, pulsed, want)
		}
	}
}

func TestHarmonyHelpers(t *testing.T) {
	if floorDiv(-1, 7) != -1 || floorDiv(7, 7) != 1 || floorDiv(-7, 7) != -1 || floorDiv(6, 7) != 0 {
		t.Error("floorDiv rounds wrongly")
	}
	if pcOf(-1) != 11 || pcOf(62) != 2 {
		t.Error("pcOf is wrong")
	}
	dorian := &moodSpecs[Calm]
	tests := []struct {
		name      string
		got, want int
	}{
		{"tonic", dorian.degreePC(0), 2},
		{"octave", dorian.degreePC(7), 2},
		{"below", dorian.degreePC(-1), 0},        // C, the flat seventh
		{"dorian sixth", dorian.degreePC(5), 11}, // B natural
		{"melody index 0", dorian.melPitch(0), 62},
		{"melody index 1", dorian.melPitch(1), 65},
		{"melody octave", dorian.melPitch(5), 74},
		{"melody below", dorian.melPitch(-1), 60},
		{"bass D", bassNote(2, 36), 38},
		{"bass C", bassNote(0, 36), 36},
		{"bass B", bassNote(11, 36), 47},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, tc.got, tc.want)
		}
	}
	colours := []struct {
		mood   Mood
		degree int
		colour int
		ok     bool
		why    string
	}{
		{Calm, 0, colourSeventh, true, "Dm7"},
		{Calm, 0, colourNinth, true, "Dm add9"},
		{Calm, 3, colourSeventh, false, "G7 would be a dominant seventh (tritone)"},
		{Calm, 3, colourNinth, true, "G add9"},
		{Menu, 0, colourSeventh, true, "Dmaj7"},
		{Tense, 0, colourNinth, false, "E flat over D minor is a minor ninth"},
		{Tense, 1, colourSeventh, true, "E flat maj7"},
		{Calm, 0, colourNone, false, "no colour requested"},
	}
	for _, tc := range colours {
		if _, ok := moodSpecs[tc.mood].colourPC(tc.degree, tc.colour); ok != tc.ok {
			t.Errorf("%s degree %d colour %d: consonant = %v, want %v (%s)", tc.mood, tc.degree, tc.colour, ok, tc.ok, tc.why)
		}
	}
	dm := dorian.buildChord(0, colourSeventh)
	if dm.n != 4 || !dm.has(60) || dm.inTriad(60) || !dm.inTriad(65) || dm.grates(64) || !dm.grates(70) {
		t.Errorf("Dm7 chord membership is wrong: %+v", dm)
	}
}

// compose runs a generator's sequencer (without rendering audio) for the given number of
// bars and reports every chord change and every bar's notes.
func compose(m Mood, seed uint64, bars int, onChord func(g *musicGen), onBar func(g *musicGen)) {
	g := newMusicGen(m, seed)
	for step := 0; step < bars*16; step++ {
		g.onStep()
		if step%16 == 0 {
			if (step/16)%g.sp.barsPerChord == 0 {
				onChord(g)
			}
			onBar(g)
		}
	}
}

func TestPadVoicingsAreConsonant(t *testing.T) {
	for _, m := range AllMoods() {
		sp := &moodSpecs[m]
		var prev [4]int
		chords, moved, coloured := 0, 0, 0
		for seed := uint64(1); seed <= 5; seed++ {
			first := true
			compose(m, seed, 240, func(g *musicGen) {
				v := g.voicing
				where := fmt.Sprintf("%s seed %d chord %d (degree %d) voicing %v", m, seed, chords, g.ch.degree, v)
				hasThird := false
				for i, n := range v {
					if n < sp.padLo || n > sp.padHi {
						t.Fatalf("%s: note %d outside the register %d..%d", where, n, sp.padLo, sp.padHi)
					}
					if !sp.inMode(n) || !g.ch.has(n) {
						t.Fatalf("%s: note %d is not a tone of the chord in the mode", where, n)
					}
					if pcOf(n) == g.ch.pcs[1] {
						hasThird = true
					}
					if i > 0 && n-v[i-1] < 2 {
						t.Fatalf("%s: voices %d and %d rub or cross", where, v[i-1], n)
					}
					for j := 0; j < i; j++ {
						if (n-v[j])%12 == 1 {
							t.Fatalf("%s: minor ninth between %d and %d", where, v[j], n)
						}
					}
				}
				if !hasThird {
					t.Fatalf("%s: the chord's third is missing", where)
				}
				if g.ch.n == 4 {
					coloured++
					if pc := g.ch.pcs[3]; pcOf(v[0]) == pc {
						t.Fatalf("%s: the colour tone is the lowest voice", where)
					}
					for i := 0; i < 3; i++ {
						if d := (g.ch.pcs[3] - g.ch.pcs[i] + 12) % 12; d == 0 || d == 1 || d == 6 {
							t.Fatalf("%s: colour tone clashes with the triad (interval %d)", where, d)
						}
					}
				}
				bass := bassNote(g.ch.pcs[0], sp.bassLo)
				if got := midiHz(float64(bass)); math.Abs(g.bassTarget-got) > 1e-9 || bass < sp.bassLo || bass > sp.bassLo+11 {
					t.Fatalf("%s: bass target %.2f Hz, want the root %d", where, g.bassTarget, bass)
				}
				if !first {
					for i := range v {
						moved += absInt(v[i] - prev[i])
					}
				}
				prev, first = v, false
				chords++
			}, func(*musicGen) {})
		}
		perVoice := float64(moved) / float64(4*chords)
		t.Logf("%s: %d chords, %d with a colour tone, mean movement %.2f semitones per voice", m, chords, coloured, perVoice)
		if perVoice > 2.5 {
			t.Errorf("%s: voices move %.2f semitones per chord change on average; the voice-leading is not smooth", m, perVoice)
		}
		if sp.colour > 0.2 && coloured < chords/10 {
			t.Errorf("%s: only %d of %d chords carry a colour tone", m, coloured, chords)
		}
	}
}

func TestMelodiesFitTheHarmony(t *testing.T) {
	for _, m := range AllMoods() {
		sp := &moodSpecs[m]
		bars, barsWithNotes, notes := 0, 0, 0
		maxLeap := 0
		for seed := uint64(1); seed <= 5; seed++ {
			compose(m, seed, 240, func(*musicGen) {}, func(g *musicGen) {
				bars++
				count, last := 0, 0
				for step, ev := range g.bar {
					if ev.pitch == 0 {
						continue
					}
					where := fmt.Sprintf("%s seed %d bar %d step %d note %d over degree %d", m, seed, bars, step, ev.pitch, g.ch.degree)
					if !sp.inMode(ev.pitch) {
						t.Fatalf("%s: outside the mode", where)
					}
					if ev.pitch < sp.melBase || ev.pitch > sp.melBase+24 {
						t.Fatalf("%s: outside the melody register", where)
					}
					if step%8 == 0 && !g.ch.inTriad(ev.pitch) {
						t.Fatalf("%s: a strong beat must carry a chord tone", where)
					}
					if g.ch.grates(ev.pitch) {
						t.Fatalf("%s: sits a semitone above a chord tone", where)
					}
					if ev.vel <= 0 || ev.vel > 1.1 {
						t.Fatalf("%s: velocity %v", where, ev.vel)
					}
					if last != 0 {
						maxLeap = max(maxLeap, absInt(ev.pitch-last))
					}
					last = ev.pitch
					count++
				}
				if count > sp.motifMax {
					t.Fatalf("%s bar %d: %d notes, more than the motif size %d", m, bars, count, sp.motifMax)
				}
				if count > 0 {
					barsWithNotes++
				}
				notes += count
			})
		}
		share := float64(barsWithNotes) / float64(bars)
		t.Logf("%s: %.0f%% of bars carry notes, %.1f notes per sounding bar, largest leap %d semitones",
			m, 100*share, float64(notes)/float64(max(barsWithNotes, 1)), maxLeap)
		// Sparse, but never absent and never wall-to-wall (except the driven Battle ostinato).
		if share < 0.2 || (m != Battle && share > 0.8) {
			t.Errorf("%s: %.0f%% of bars carry notes", m, 100*share)
		}
		if maxLeap > 12 {
			t.Errorf("%s: a melodic leap of %d semitones inside a bar, wider than an octave", m, maxLeap)
		}
	}
}

func TestMotifs(t *testing.T) {
	for _, m := range AllMoods() {
		sp := &moodSpecs[m]
		r := newRNG(uint64(m) + 5)
		offbeats := 0
		for i := 0; i < 300; i++ {
			mot := newMotif(&r, sp)
			if mot.n < sp.motifMin || mot.n > sp.motifMax {
				t.Fatalf("%s: motif of %d notes, want %d..%d", m, mot.n, sp.motifMin, sp.motifMax)
			}
			if mot.rel[0] != 0 {
				t.Fatalf("%s: a motif must start on its anchor, got offset %d", m, mot.rel[0])
			}
			for j := 0; j < mot.n; j++ {
				if j > 0 && mot.step[j] <= mot.step[j-1] {
					t.Fatalf("%s: onsets not ascending: %v", m, mot.step[:mot.n])
				}
				if mot.step[j] < 0 || mot.step[j] > 15 || mot.rel[j] < -3 || mot.rel[j] > 4 {
					t.Fatalf("%s: onset %d offset %d out of range", m, mot.step[j], mot.rel[j])
				}
				if mot.step[j]%2 == 1 {
					offbeats++
				}
			}
		}
		if sp.offbeat == 0 && offbeats > 0 {
			t.Errorf("%s: %d off-beat sixteenths in a mood that has none", m, offbeats)
		}
	}
}

func TestMusicProperties(t *testing.T) {
	for _, m := range AllMoods() {
		t.Run(m.String(), func(t *testing.T) {
			buf := musicClip(m)
			if want := 2 * 40 * SampleRate; len(buf) != want {
				t.Fatalf("length %d, want %d (interleaved stereo)", len(buf), want)
			}
			for i, v := range buf {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
					t.Fatalf("sample %d is %v", i, v)
				}
			}
			l, r := splitStereo(buf)
			mid := midOf(buf)
			peak := math.Max(peakOf(l), peakOf(r))
			rms := linToDb(rmsOf(mid))
			t.Logf("peak %.3f, rms %.1f dBFS", peak, rms)
			if peak > 0.8 {
				t.Errorf("peak %.3f: the music must keep headroom", peak)
			}
			if rms < -30 || rms > -22 {
				t.Errorf("rms %.1f dBFS outside the band -30..-22", rms)
			}
			// It never falls silent: every 4 s stretch after the opening fade carries sound.
			win := 4 * SampleRate
			for start := win; start+win <= len(mid); start += win {
				if level := linToDb(rmsOf(mid[start : start+win])); level < rms-8 {
					t.Errorf("%d..%d s: level %.1f dBFS, more than 8 dB under the average", start/SampleRate, (start+win)/SampleRate, level)
				}
			}
			for name, ch := range map[string][]float64{"left": l, "right": r} {
				mean := 0.0
				for _, v := range ch {
					mean += v / float64(len(ch))
				}
				if math.Abs(mean) > 0.001 {
					t.Errorf("%s: DC offset %.5f", name, mean)
				}
				if ch[0] != 0 || math.Abs(ch[len(ch)-1]) > 1e-4 {
					t.Errorf("%s: clip does not begin and end at silence (%v, %v)", name, ch[0], ch[len(ch)-1])
				}
				// Voices start, stop and are reused constantly; none of that may click.
				if ratio, at := kinkOutlier(ch, 0.004); ratio > 14 {
					t.Errorf("%s: click at %.3f s: %.1f× the local roughness", name, float64(at)*invSR, ratio)
				}
			}
		})
	}
}

func TestMusicIsDeterministicPerSeed(t *testing.T) {
	for _, m := range []Mood{Calm, Battle} {
		a := RenderMusic(m, 6, 42)
		b := RenderMusic(m, 6, 42)
		c := RenderMusic(m, 6, 43)
		if !slices.Equal(a, b) {
			t.Errorf("%s: the same seed gave different audio", m)
		}
		if slices.Equal(a, c) {
			t.Errorf("%s: different seeds gave identical audio", m)
		}
	}
	if slices.Equal(RenderMusic(Calm, 6, 42), RenderMusic(Lament, 6, 42)) {
		t.Error("different moods gave identical audio")
	}
	// A shorter render is a prefix of a longer one (apart from the closing fade).
	long, short := RenderMusic(Menu, 5, 9), RenderMusic(Menu, 3, 9)
	n := len(short) - 2*secondsToSamples(0.06)
	if !slices.Equal(long[:n], short[:n]) {
		t.Error("a 3 s render is not the start of the 5 s render with the same seed")
	}
}

func TestRenderMusicArguments(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mood    Mood
		seconds float64
		frames  int
	}{
		{"invalid mood", Mood(-1), 1, 0},
		{"mood out of range", Mood(NumMoods), 1, 0},
		{"zero seconds", Calm, 0, 0},
		{"negative seconds", Calm, -3, 0},
		{"NaN seconds", Calm, math.NaN(), 0},
		{"short", Calm, 0.25, SampleRate / 4},
		{"one second", Battle, 1, SampleRate},
	} {
		if got := len(RenderMusic(tc.mood, tc.seconds, 1)); got != 2*tc.frames {
			t.Errorf("%s: %d samples, want %d", tc.name, got, 2*tc.frames)
		}
	}
}

func TestMusicLoudnessIsConsistentBetweenMoods(t *testing.T) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, m := range AllMoods() {
		l, r := splitStereo(musicClip(m))
		weighted := linToDb((rmsOf(earWeight(l)) + rmsOf(earWeight(r))) / 2)
		plain := linToDb(rmsOf(midOf(musicClip(m))))
		t.Logf("%-8s ear-weighted %.1f dB, plain %.1f dB", m, weighted, plain)
		lo, hi = math.Min(lo, weighted), math.Max(hi, weighted)
	}
	if hi-lo > 2.5 {
		t.Errorf("perceived level differs by %.1f dB between moods, want within 2.5 dB", hi-lo)
	}
	// The music sits under the effects: quieter than even a modest one such as BuildDone.
	if hi > sfxDefs[BuildDone].loudness-3 {
		t.Errorf("music at %.1f dB is not clearly under the effects (BuildDone at %.0f dB)", hi, sfxDefs[BuildDone].loudness)
	}
}

func TestMusicSpectrum(t *testing.T) {
	centroid := make(map[Mood]float64)
	for _, m := range AllMoods() {
		spec := powerSpectrum(midOf(musicClip(m)), 8192)
		centroid[m] = spectralCentroid(spec)
		body := bandEnergy(spec, 60, 2500)
		sub := bandEnergy(spec, 0, 40)
		air := bandEnergy(spec, 2500, 8000)
		top := bandEnergy(spec, 8000, 22050)
		t.Logf("%-8s centroid %.0f Hz; 60 Hz..2.5 kHz %.3f, <40 Hz %.4f, 2.5..8 kHz %.4f, >8 kHz %.5f", m, centroid[m], body, sub, air, top)
		if body < 0.9 {
			t.Errorf("%s: only %.2f of the energy lies between 60 Hz and 2.5 kHz", m, body)
		}
		if sub > 0.03 {
			t.Errorf("%s: %.3f of the energy is sub-sonic rumble below 40 Hz", m, sub)
		}
		if air < 0.0005 || air > 0.08 {
			t.Errorf("%s: %.4f of the energy between 2.5 and 8 kHz; want some air but no harshness", m, air)
		}
		if top > 0.002 {
			t.Errorf("%s: %.4f of the energy above 8 kHz; the music should be soft", m, top)
		}
	}
	if centroid[Lament] >= centroid[Calm] || centroid[Calm] >= centroid[Triumph] {
		t.Errorf("brightness should rise Lament < Calm < Triumph, got %.0f, %.0f, %.0f Hz",
			centroid[Lament], centroid[Calm], centroid[Triumph])
	}
}

// pulseStrength measures how strongly the bass register of a clip pulses at the given period:
// the autocorrelation of its 10 ms level envelope at that lag (1 = perfectly periodic).
func pulseStrength(mid []float64, period float64) float64 {
	var lp svf
	lp.set(160, 0.707)
	hop := secondsToSamples(0.010)
	var env []float64
	acc := 0.0
	for i, v := range mid {
		y := lp.low(v)
		acc += y * y
		if (i+1)%hop == 0 {
			env = append(env, math.Sqrt(acc/float64(hop)))
			acc = 0
		}
	}
	mean := 0.0
	for _, e := range env {
		mean += e / float64(len(env))
	}
	lag := int(math.Round(period / 0.010))
	num, den := 0.0, 0.0
	for i := range env {
		a := env[i] - mean
		den += a * a
		if i+lag < len(env) {
			num += a * (env[i+lag] - mean)
		}
	}
	if den == 0 {
		return 0
	}
	return num / den
}

func TestMusicPulse(t *testing.T) {
	strength := make(map[Mood]float64)
	for _, m := range AllMoods() {
		bar := 4 * 60 / moodSpecs[m].bpm
		strength[m] = pulseStrength(midOf(musicClip(m)), bar)
		t.Logf("%-8s bar-periodicity of the low end: %.2f", m, strength[m])
	}
	for _, m := range []Mood{Tense, Battle} {
		if strength[m] < 0.4 {
			t.Errorf("%s: pulse strength %.2f, want a clear bar-periodic pulse", m, strength[m])
		}
	}
	for _, m := range []Mood{Menu, Calm, Lament} {
		if strength[m] > 0.3 {
			t.Errorf("%s: pulse strength %.2f, want no beat in a still mood", m, strength[m])
		}
	}
}

func TestMusicStereoImage(t *testing.T) {
	for _, m := range AllMoods() {
		l, r := splitStereo(musicClip(m))
		var ll, rr, lr float64
		for i := range l {
			ll += l[i] * l[i]
			rr += r[i] * r[i]
			lr += l[i] * r[i]
		}
		corr := lr / math.Sqrt(ll*rr)
		balance := linToDb(math.Sqrt(ll / rr))
		t.Logf("%-8s L/R correlation %.2f, balance %+.2f dB", m, corr, balance)
		// Gentle width: clearly stereo, but mono-compatible and centred.
		if corr < 0.35 || corr > 0.97 {
			t.Errorf("%s: L/R correlation %.2f, want 0.35..0.97", m, corr)
		}
		if math.Abs(balance) > 1.5 {
			t.Errorf("%s: channels unbalanced by %.2f dB", m, balance)
		}
	}
}

// stemLevel renders one layer of a mood on its own and returns its RMS level in dBFS.
func stemLevel(m Mood, stem string) float64 {
	sp := moodSpecs[m]
	keep := func(name string, gain *float64) {
		if name != stem {
			*gain = 0
		}
	}
	keep("pad", &sp.padGain)
	keep("pluck", &sp.pluckGain)
	keep("bass", &sp.bassGain)
	keep("wind", &sp.windGain)
	keep("pulse", &sp.percGain)
	g := newMusicGenFrom(&sp, m, 3)
	buf := make([]float32, 2*16*SampleRate)
	g.render(buf)
	return linToDb(rmsOf(midOf(buf)))
}

func TestMusicLayerBalance(t *testing.T) {
	for _, m := range AllMoods() {
		pad, pluck, bass := stemLevel(m, "pad"), stemLevel(m, "pluck"), stemLevel(m, "bass")
		wind, pulse := stemLevel(m, "wind"), stemLevel(m, "pulse")
		t.Logf("%-8s pad %.1f, pluck %.1f, bass %.1f, wind %.1f, pulse %.1f dBFS", m, pad, pluck, bass, wind, pulse)
		if pad < pluck || pad < bass || pad < wind {
			t.Errorf("%s: the pad should be the bed of the mix (pad %.1f, pluck %.1f, bass %.1f, wind %.1f)", m, pad, pluck, bass, wind)
		}
		if d := pad - bass; d < 2 || d > 12 {
			t.Errorf("%s: bass is %.1f dB under the pad, want a soft but present sub (2..12 dB)", m, d)
		}
		if d := pad - pluck; d > 14 {
			t.Errorf("%s: plucks are %.1f dB under the pad and would be lost", m, d)
		}
		if d := pad - wind; d < 6 || d > 24 {
			t.Errorf("%s: wind is %.1f dB under the pad, want a gentle but audible bed (6..24 dB)", m, d)
		}
		if hasPulse := moodSpecs[m].percGain > 0; hasPulse && (pad-pulse < -3 || pad-pulse > 14) {
			t.Errorf("%s: pulse is %.1f dB under the pad", m, pad-pulse)
		} else if !hasPulse && pulse > -150 {
			t.Errorf("%s: a pulse layer sounds (%.1f dBFS) in a mood without one", m, pulse)
		}
	}
}

func TestFadeEdges(t *testing.T) {
	buf := make([]float32, 2*100)
	for i := range buf {
		buf[i] = 1
	}
	fadeEdges(buf, 10, 20)
	if buf[0] != 0 || buf[1] != 0 || buf[198] != 0 || buf[199] != 0 {
		t.Errorf("edges not silent: %v %v ... %v %v", buf[0], buf[1], buf[198], buf[199])
	}
	if buf[2*10] != 1 || buf[2*50] != 1 || buf[2*79] != 1 {
		t.Error("the middle of the buffer was altered")
	}
	if buf[2*5] <= 0 || buf[2*5] >= 1 || buf[2*90] <= 0 || buf[2*90] >= 1 {
		t.Error("fades are not gradual")
	}
	fadeEdges(nil, 10, 10) // must not panic
	tiny := []float32{1, 1}
	fadeEdges(tiny, 10, 10)
}

func BenchmarkMusic(b *testing.B) {
	for _, m := range AllMoods() {
		b.Run(m.String(), func(b *testing.B) {
			g := newMusicGen(m, 1)
			buf := make([]float32, 2*SampleRate)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				g.render(buf)
			}
			b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "x-realtime")
		})
	}
}

func BenchmarkRenderAllSfx(b *testing.B) {
	for i := 0; i < b.N; i++ {
		renderAllSfx()
	}
}
