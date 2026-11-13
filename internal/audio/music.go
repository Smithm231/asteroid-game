package audio

import "math"

// The sounding half of the music engine. A musicGen is a self-contained streaming synthesiser
// for one mood: a sixteenth-note clock drives the composer in music_harmony.go, whose decisions
// trigger pad, pluck, bass and pulse voices; a wind bed, an echo and a reverb complete the mix.
// Everything is driven by the generator's own seeded PRNG, so a given (mood, seed) pair always
// produces the same audio.

const (
	padSlots   = 10 // four sounding pad notes plus releasing ones
	pluckSlots = 6
	drumSlots  = 3
	tickSlots  = 3
	ctlPeriod  = 64 // samples between control-rate updates (LFOs, filter retuning)
)

// Layer levels before the per-mood gains; set so the layers balance at unity mood gains.
const (
	padLevel   = 0.105
	pluckLevel = 0.24
	bassLevel  = 0.045
	windLevel  = 0.30
	drumLevel  = 0.16
	tickLevel  = 0.045
)

// The presence shelf applied to the finished music: +4.6 dB above roughly 2 kHz.
const (
	airCorner = 2000.0
	airLift   = 0.7
)

type padVoice struct {
	pitch      int
	gate       bool
	env        adsr
	p1, p2, p3 float64 // phases: two detuned sawtooths and a sine
	d1, d2, d3 float64 // phase increments
	gl, gr     float64
}

type pluckVoice struct {
	live           bool
	carrier, mod   float64
	dCarrier, dMod float64
	amp, ampCoef   float64
	idx, idxCoef   float64
	rise, riseStep float64
	gl, gr         float64
}

type drumVoice struct {
	live           bool
	ph             float64
	freq, rest     float64 // current and resting pitch
	freqCoef       float64
	amp, ampCoef   float64
	rise, riseStep float64
}

type tickVoice struct {
	live         bool
	amp, ampCoef float64
	bp           svf
	gl, gr       float64
}

// pulseEnv is a retriggerable attack/decay envelope used to pulse the bass.
type pulseEnv struct {
	level, peak float64
	rising      bool
	step, coef  float64
}

func (e *pulseEnv) trigger(peak float64) {
	if e.level < peak {
		e.peak = peak
		e.rising = true
	}
}

func (e *pulseEnv) next() float64 {
	if e.rising {
		e.level += e.step
		if e.level >= e.peak {
			e.level = e.peak
			e.rising = false
		}
	} else {
		e.level *= e.coef
	}
	return e.level
}

type noteEvent struct {
	pitch int // 0 = no note on this step
	vel   float64
}

// musicGen generates the music of one mood.
type musicGen struct {
	mood Mood
	sp   *moodSpec
	r    rng // composition and performance choices
	nz   rng // audio-rate noise, kept apart so it never perturbs the composition

	// Clock.
	stepLen   float64 // samples per sixteenth note
	untilStep float64
	step      int

	// Composer state.
	prog        []int
	progIdx     int
	progPos     int
	phrase      int
	ch          chord
	voicing     [4]int
	haveVoicing bool
	mot         motif
	lastMel     int
	restBars    int
	bar         [16]noteEvent

	// Voices.
	pads   [padSlots]padVoice
	plucks [pluckSlots]pluckVoice
	drums  [drumSlots]drumVoice
	ticks  [tickSlots]tickVoice

	// Bass.
	bassPh, bassFreq, bassTarget float64
	bassAmp                      float64
	bassEnv                      pulseEnv
	bassGlide, bassFade          float64

	// Buses and effects.
	padLP              [2]svf
	padLFO             [2]float64 // phases of the two cutoff LFOs
	chorus             [2]delayLine
	chorusPh           [2]float64
	echoL, echoR       delayLine
	echoLP             [2]onePole
	echoTime           int
	windLP             [2]onePole
	windBP             [2]svf
	windPh             [2][3]float64 // per channel: two sweep LFOs and one gust LFO
	windAmp, windAmpTo [2]float64
	air                [2]onePole // splits off the top octaves for the presence lift
	rv                 *reverb
	ctl                int
}

// Slow LFO rates in Hz; mutually unrelated so the texture never audibly repeats.
var (
	padLFORates  = [2]float64{0.021, 0.053}
	windLFORates = [2][3]float64{{0.043, 0.071, 0.057}, {0.037, 0.083, 0.049}}
	chorusRates  = [2]float64{0.23, 0.19}
)

// newMusicGen creates a generator for a mood. The mood must be valid.
func newMusicGen(m Mood, seed uint64) *musicGen {
	return newMusicGenFrom(&moodSpecs[m], m, seed)
}

// newMusicGenFrom creates a generator from an explicit parameter set (the tests use this to
// audition single layers).
func newMusicGenFrom(sp *moodSpec, m Mood, seed uint64) *musicGen {
	g := &musicGen{
		mood:    m,
		sp:      sp,
		r:       newRNG(seed*2654435761 + uint64(m)*40503 + 17),
		nz:      newRNG(seed ^ 0xA5A5A5A55A5A5A5A + uint64(m)),
		stepLen: sr * 60 / (sp.bpm * 4),
		progIdx: -1,
		lastMel: melodySpan / 2,
	}
	for i := range g.padLFO {
		g.padLFO[i] = g.r.float()
	}
	for c := range g.windPh {
		for i := range g.windPh[c] {
			g.windPh[c][i] = g.r.float()
		}
		g.windLP[c].set(2200)
		g.chorus[c] = newDelayLine(secondsToSamples(0.030))
		g.echoLP[c].set(2600)
		g.air[c].set(airCorner)
	}
	g.chorusPh[1] = 0.37
	// The echo repeats every dotted eighth (three sixteenths).
	g.echoTime = int(3 * g.stepLen)
	g.echoL = newDelayLine(g.echoTime + 4)
	g.echoR = newDelayLine(g.echoTime + 4)
	g.bassGlide = smoothCoef(0.030)
	g.bassFade = smoothCoef(0.8)
	g.bassEnv.step = 1 / (0.006 * sr)
	if sp.bassStyle == bassDrive {
		g.bassEnv.coef = decayCoef(0.13)
	} else {
		g.bassEnv.coef = decayCoef(0.30)
	}
	g.rv = newReverb(1.15, sp.reverbTime, 0.45, 180)
	g.control()
	return g
}

// ---------------------------------------------------------------------------------------------
// Sequencing
// ---------------------------------------------------------------------------------------------

// onStep runs once per sixteenth note.
func (g *musicGen) onStep() {
	s := g.step % 16
	bar := g.step / 16
	if s == 0 {
		barInChord := bar % g.sp.barsPerChord
		if barInChord == 0 {
			g.advanceChord()
		}
		g.planBar(barInChord)
	}
	if ev := g.bar[s]; ev.pitch != 0 {
		g.pluck(ev.pitch, ev.vel)
	}
	g.bassStep(s)
	g.pulseStep(s, bar)
	g.step++
}

// advanceChord moves to the next chord of the progression (choosing a new progression, and
// sometimes a new motif, when the current one is finished) and re-voices the pads and bass.
func (g *musicGen) advanceChord() {
	sp := g.sp
	if g.progPos >= len(g.prog) {
		idx := g.r.intn(len(sp.progs))
		if len(sp.progs) > 1 && idx == g.progIdx {
			idx = (idx + 1 + g.r.intn(len(sp.progs)-1)) % len(sp.progs)
		}
		g.progIdx, g.prog, g.progPos = idx, sp.progs[idx], 0
		if g.phrase == 0 || g.r.chance(0.5) {
			g.mot = newMotif(&g.r, sp)
		}
		g.phrase++
	}
	degree := g.prog[g.progPos]
	g.progPos++

	colour := colourNone
	if g.r.chance(sp.colour) {
		first, second := colourSeventh, colourNinth
		if g.r.chance(0.5) {
			first, second = second, first
		}
		if _, ok := sp.colourPC(degree, first); ok {
			colour = first
		} else if _, ok := sp.colourPC(degree, second); ok {
			colour = second
		}
	}
	g.ch = sp.buildChord(degree, colour)
	g.setVoicing(voiceChord(g.ch, g.voicing, g.haveVoicing, sp.padLo, sp.padHi))
	g.bassTarget = midiHz(float64(bassNote(g.ch.pcs[0], sp.bassLo)))
	if g.bassFreq == 0 {
		g.bassFreq = g.bassTarget
	}
}

// planBar decides which plucked notes the coming bar carries by hanging the current motif on
// the current chord. Bars are often left empty: the music breathes.
func (g *musicGen) planBar(barInChord int) {
	sp := g.sp
	g.bar = [16]noteEvent{}
	p := sp.density
	if barInChord > 0 {
		p *= 0.55 // the first bar of a chord states the motif; later bars only sometimes answer
	}
	if g.restBars >= 2 {
		p = math.Min(1, p*1.8)
	}
	if !g.r.chance(p) {
		g.restBars++
		return
	}
	g.restBars = 0
	anchor := sp.anchorIndex(&g.ch, g.lastMel)
	last, lastRel := -1, 0
	for i := 0; i < g.mot.n; i++ {
		if i > 0 && g.r.chance(0.12) {
			continue // drop a note now and then so repeats are never literal
		}
		rel := g.mot.rel[i]
		k := int(clamp(float64(anchor+rel), 0, melodySpan-1))
		step := g.mot.step[i]
		dir, avoid := 0, -1
		if last >= 0 && rel != lastRel {
			avoid = last // the motif moves here, so the fitted note must move too
			dir = 1
			if rel < lastRel {
				dir = -1
			}
		}
		// Notes on beats one and three belong to the chord; the rest only must not grate.
		k = sp.fitIndex(&g.ch, k, step%8 == 0, dir, avoid)
		// Keep the line singable: fold any leap wider than an octave back by an octave
		// (five melody indices), which leaves the note's place in the chord unchanged.
		for last >= 0 && absInt(sp.melPitch(k)-sp.melPitch(last)) > 12 {
			if k > last {
				k -= 5
			} else {
				k += 5
			}
		}
		g.bar[step] = noteEvent{pitch: sp.melPitch(k), vel: g.mot.vel[i] * g.r.rangef(0.9, 1.05)}
		last, lastRel = k, rel
	}
	if last >= 0 {
		g.lastMel = last
	}
}

// setVoicing gates the pad voices to a new four-note voicing. Notes shared with the previous
// chord simply keep sounding; the rest cross-fade through their envelopes.
func (g *musicGen) setVoicing(v [4]int) {
	sp := g.sp
	wanted := func(pitch int) bool {
		return pitch == v[0] || pitch == v[1] || pitch == v[2] || pitch == v[3]
	}
	for i := range g.pads {
		if pv := &g.pads[i]; pv.gate && !wanted(pv.pitch) {
			pv.gate = false
			pv.env.gate(false)
		}
	}
	for idx, pitch := range v {
		slot := -1
		quietest := -1
		held := false
		for i := range g.pads {
			pv := &g.pads[i]
			switch {
			case pv.pitch == pitch && !pv.env.idle():
				if pv.gate {
					held = true
				} else {
					slot = i // still releasing at this pitch: reopen it
				}
			case pv.env.idle():
				if slot < 0 {
					slot = i
				}
			case !pv.gate && (quietest < 0 || pv.env.level < g.pads[quietest].env.level):
				quietest = i
			}
			if held {
				break
			}
		}
		if held {
			continue
		}
		if slot < 0 {
			slot = quietest
		}
		if slot < 0 {
			continue // every slot is held open, which four notes over ten slots cannot cause
		}
		pv := &g.pads[slot]
		if pv.pitch != pitch || pv.env.idle() {
			freq := midiHz(float64(pitch))
			cents := 5 + float64(pitch%3)
			pv.d1 = freq * math.Pow(2, -cents/1200) * invSR
			pv.d2 = freq * math.Pow(2, cents/1200) * invSR
			pv.d3 = freq * invSR
			pv.p1, pv.p2, pv.p3 = g.r.float(), g.r.float(), g.r.float()
			pv.pitch = pitch
			// Voices are spread across the stereo field by their place in the chord. A voice
			// that is reopened keeps its position: moving it while it sounds would click.
			pan := [4]float64{-0.45, 0.3, -0.15, 0.5}[idx]
			angle := (pan + 1) * math.Pi / 4
			pv.gl, pv.gr = math.Cos(angle), math.Sin(angle)
		}
		pv.env.set(sp.padAttack, sp.padAttack, 0.85, sp.padRelease)
		pv.gate = true
		pv.env.gate(true)
	}
	g.voicing, g.haveVoicing = v, true
}

// pluck starts a plucked note on the quietest voice.
func (g *musicGen) pluck(pitch int, vel float64) {
	sp := g.sp
	slot := 0
	for i := range g.plucks {
		if !g.plucks[i].live {
			slot = i
			break
		}
		if g.plucks[i].amp < g.plucks[slot].amp {
			slot = i
		}
	}
	// Higher notes are played softer, duller and shorter, as on a real plucked instrument.
	height := float64(pitch-62) / 24
	freq := midiHz(float64(pitch))
	tau := sp.pluckDecay / 3 * (1 - 0.3*height)
	pan := g.r.rangef(-0.45, 0.45)
	angle := (pan + 1) * math.Pi / 4
	g.plucks[slot] = pluckVoice{
		live:     true,
		dCarrier: freq * invSR,
		dMod:     freq * sp.pluckRatio * invSR,
		amp:      vel * (1 - 0.3*height),
		ampCoef:  decayCoef(tau),
		idx:      sp.pluckIndex * (0.55 + 0.45*vel) * (1 - 0.25*height) / (2 * math.Pi),
		idxCoef:  decayCoef(tau * 0.6),
		riseStep: 1 / (0.003 * sr),
		gl:       math.Cos(angle),
		gr:       math.Sin(angle),
	}
}

// bassStep re-strikes the bass according to the mood's bass style.
func (g *musicGen) bassStep(s int) {
	switch g.sp.bassStyle {
	case bassDrive:
		if s%2 == 0 {
			g.bassEnv.trigger([8]float64{1, 0.55, 0.7, 0.55, 0.9, 0.55, 0.7, 0.6}[s/2])
		}
	case bassBeat:
		if s%4 == 0 {
			g.bassEnv.trigger([4]float64{1, 0.7, 0.85, 0.7}[s/4])
		}
	}
}

// pulseStep plays the percussion-like pulse pattern of the mood, if it has one.
func (g *musicGen) pulseStep(s, bar int) {
	sp := g.sp
	if sp.percGain <= 0 {
		return
	}
	if v := sp.drumLow[s]; v > 0 {
		if s == 0 && bar%4 == 0 {
			v *= 1.15 // a slightly heavier downbeat marks every fourth bar
		}
		g.drum(58, v*g.r.rangef(0.9, 1.05), 0.16)
	}
	if v := sp.drumHigh[s]; v > 0 {
		g.drum(98, v*g.r.rangef(0.85, 1.05), 0.10)
	}
	if v := sp.tick[s]; v > 0 {
		g.tick(v * g.r.rangef(0.8, 1.1))
	}
}

func (g *musicGen) drum(freq, vel, tau float64) {
	slot := 0
	for i := range g.drums {
		if !g.drums[i].live {
			slot = i
			break
		}
		if g.drums[i].amp < g.drums[slot].amp {
			slot = i
		}
	}
	g.drums[slot] = drumVoice{
		live:     true,
		freq:     freq * 2.4,
		rest:     freq,
		freqCoef: decayCoef(0.020),
		amp:      vel,
		ampCoef:  decayCoef(tau),
		riseStep: 1 / (0.002 * sr),
	}
}

func (g *musicGen) tick(vel float64) {
	slot := 0
	for i := range g.ticks {
		if !g.ticks[i].live {
			slot = i
			break
		}
		if g.ticks[i].amp < g.ticks[slot].amp {
			slot = i
		}
	}
	t := &g.ticks[slot]
	t.live = true
	t.amp = vel
	t.ampCoef = decayCoef(0.018)
	t.bp.set(g.r.rangef(3600, 5200), 1.6)
	pan := g.r.rangef(-0.5, 0.5)
	angle := (pan + 1) * math.Pi / 4
	t.gl, t.gr = math.Cos(angle), math.Sin(angle)
}

// control runs every ctlPeriod samples: it advances the slow LFOs and retunes the pad filter
// and the wind.
func (g *musicGen) control() {
	sp := g.sp
	dt := float64(ctlPeriod) * invSR
	for i := range g.padLFO {
		g.padLFO[i] += padLFORates[i] * dt
		if g.padLFO[i] >= 1 {
			g.padLFO[i] -= 1
		}
	}
	sweep := 0.6*sinT(g.padLFO[0]) + 0.4*sinT(g.padLFO[1])
	cut := sp.padCutoff * math.Pow(2, sp.padMove*sweep)
	g.padLP[0].set(cut, 0.62)
	g.padLP[1].set(cut*1.04, 0.62)

	for c := range g.windPh {
		ph := &g.windPh[c]
		for i := range ph {
			ph[i] += windLFORates[c][i] * dt
			if ph[i] >= 1 {
				ph[i] -= 1
			}
		}
		pos := 0.5 + 0.5*(0.6*sinT(ph[0])+0.4*sinT(ph[1]))
		g.windBP[c].set(glide(sp.windLo, sp.windHi, pos), 0.9)
		gust := 0.5 + 0.5*sinT(ph[2])
		g.windAmpTo[c] = 0.35 + 0.65*gust*gust
	}
}

// ---------------------------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------------------------

// render fills out (interleaved stereo) with the next len(out)/2 frames.
func (g *musicGen) render(out []float32) {
	sp := g.sp
	padGain := padLevel * sp.padGain
	pluckGain := pluckLevel * sp.pluckGain
	bassGain := bassLevel * sp.bassGain
	windGain := windLevel * sp.windGain
	drumGain := drumLevel * sp.percGain
	tickGain := tickLevel * sp.percGain
	echoFb := sp.echo
	wet := sp.reverbWet

	for i := 0; i+1 < len(out); i += 2 {
		if g.untilStep <= 0 {
			g.onStep()
			g.untilStep += g.stepLen
		}
		g.untilStep--
		if g.ctl <= 0 {
			g.control()
			g.ctl = ctlPeriod
		}
		g.ctl--

		// Pads.
		var padL, padR float64
		for v := range g.pads {
			pv := &g.pads[v]
			if pv.env.idle() {
				continue
			}
			e := pv.env.next()
			s1 := 2*pv.p1 - 1 - polyBlep(pv.p1, pv.d1)
			s2 := 2*pv.p2 - 1 - polyBlep(pv.p2, pv.d2)
			s3 := sinT(pv.p3)
			if pv.p1 += pv.d1; pv.p1 >= 1 {
				pv.p1 -= 1
			}
			if pv.p2 += pv.d2; pv.p2 >= 1 {
				pv.p2 -= 1
			}
			if pv.p3 += pv.d3; pv.p3 >= 1 {
				pv.p3 -= 1
			}
			padL += e * pv.gl * (0.8*s1 + 0.3*s2 + 0.6*s3)
			padR += e * pv.gr * (0.3*s1 + 0.8*s2 + 0.6*s3)
		}
		padL = g.padLP[0].low(padL) * padGain
		padR = g.padLP[1].low(padR) * padGain
		// A slow stereo chorus thickens the pad and widens it a little.
		for c := range g.chorusPh {
			if g.chorusPh[c] += chorusRates[c] * invSR; g.chorusPh[c] >= 1 {
				g.chorusPh[c] -= 1
			}
		}
		g.chorus[0].write(padL)
		g.chorus[1].write(padR)
		padL = 0.75*padL + 0.5*g.chorus[0].tapFrac((0.014+0.004*sinT(g.chorusPh[0]))*sr)
		padR = 0.75*padR + 0.5*g.chorus[1].tapFrac((0.016+0.004*sinT(g.chorusPh[1]))*sr)

		// Plucks.
		var plL, plR float64
		for v := range g.plucks {
			pv := &g.plucks[v]
			if !pv.live {
				continue
			}
			if pv.rise < 1 {
				pv.rise = math.Min(1, pv.rise+pv.riseStep)
			}
			s := sinT(pv.carrier+pv.idx*sinT(pv.mod)) * pv.amp * pv.rise
			if pv.carrier += pv.dCarrier; pv.carrier >= 1 {
				pv.carrier -= 1
			}
			if pv.mod += pv.dMod; pv.mod >= 1 {
				pv.mod -= 1
			}
			pv.amp *= pv.ampCoef
			pv.idx *= pv.idxCoef
			if pv.amp < 2e-5 {
				pv.live = false
			}
			plL += s * pv.gl
			plR += s * pv.gr
		}
		plL *= pluckGain
		plR *= pluckGain
		// Ping-pong echo: left repeats feed the right line and back.
		echoL := g.echoL.tap(g.echoTime)
		echoR := g.echoR.tap(g.echoTime)
		g.echoL.write((plL+plR)*0.5 + g.echoLP[0].low(echoR)*echoFb)
		g.echoR.write(g.echoLP[1].low(echoL) * echoFb)
		plL += echoL * echoFb * 1.2
		plR += echoR * echoFb * 1.2

		// Bass: a sine with two soft overtones so it still reads on small speakers.
		g.bassFreq += (g.bassTarget - g.bassFreq) * g.bassGlide
		g.bassAmp += (1 - g.bassAmp) * g.bassFade
		if g.bassPh += g.bassFreq * invSR; g.bassPh >= 1 {
			g.bassPh -= 1
		}
		bassAmp := g.bassAmp
		switch sp.bassStyle {
		case bassBeat:
			bassAmp *= 0.6 + 0.5*g.bassEnv.next()
		case bassDrive:
			bassAmp *= 0.22 + 0.95*g.bassEnv.next()
		}
		bass := (sinT(g.bassPh) + 0.30*sinT(2*g.bassPh) + 0.10*sinT(3*g.bassPh)) * bassAmp * bassGain

		// Pulse: pitched drum thumps (centre) and noise ticks.
		var drum, tkL, tkR float64
		for v := range g.drums {
			dv := &g.drums[v]
			if !dv.live {
				continue
			}
			if dv.rise < 1 {
				dv.rise = math.Min(1, dv.rise+dv.riseStep)
			}
			drum += sinT(dv.ph) * dv.amp * dv.rise
			dv.freq = dv.rest + (dv.freq-dv.rest)*dv.freqCoef
			if dv.ph += dv.freq * invSR; dv.ph >= 1 {
				dv.ph -= 1
			}
			dv.amp *= dv.ampCoef
			if dv.amp < 2e-5 {
				dv.live = false
			}
		}
		drum *= drumGain
		for v := range g.ticks {
			tv := &g.ticks[v]
			if !tv.live {
				continue
			}
			s := tv.bp.band(g.nz.bipolar()) * tv.amp
			tv.amp *= tv.ampCoef
			if tv.amp < 2e-5 {
				tv.live = false
			}
			tkL += s * tv.gl
			tkR += s * tv.gr
		}
		tkL *= tickGain
		tkR *= tickGain

		// Wind: two independent noise bands, one per side.
		var wind [2]float64
		for c := range wind {
			g.windAmp[c] += (g.windAmpTo[c] - g.windAmp[c]) * 0.0004
			wind[c] = g.windBP[c].band(g.windLP[c].low(g.nz.bipolar())) * g.windAmp[c] * windGain
		}

		// Reverb send and final sum.
		revL, revR := g.rv.tick(
			0.5*padL+0.6*plL+0.2*drum+0.3*tkL+0.25*wind[0],
			0.5*padR+0.6*plR+0.2*drum+0.3*tkR+0.25*wind[1],
		)
		outL := padL + plL + bass + drum + tkL + wind[0] + wet*revL
		outR := padR + plR + bass + drum + tkR + wind[1] + wet*revR
		// A gentle high shelf restores some air: the layers are all deliberately soft.
		outL += airLift * g.air[0].high(outL)
		outR += airLift * g.air[1].high(outR)
		out[i] = float32(outL * sp.gain)
		out[i+1] = float32(outR * sp.gain)
	}
}

// RenderMusic renders the generative music of a mood offline and returns it as interleaved
// stereo float32 PCM (L, R, L, R, ...) at SampleRate, passed through the same limiter as live
// playback. The output is fully determined by (m, seconds, seed). A 20 ms fade-in and a 50 ms
// fade-out make the clip self-contained. seconds is capped at 600; an invalid mood or a
// non-positive duration yields nil.
func RenderMusic(m Mood, seconds float64, seed uint64) []float32 {
	if !m.Valid() || !(seconds > 0) {
		return nil
	}
	frames := secondsToSamples(math.Min(seconds, 600))
	g := newMusicGen(m, seed)
	lim := newLimiter()
	out := make([]float32, 2*frames)
	// The limiter delays by its look-ahead; render that much extra and drop the lead-in.
	lead := lim.lookahead
	block := make([]float32, 2*1024)
	done := 0
	for done < frames+lead {
		g.render(block)
		for i := 0; i+1 < len(block) && done < frames+lead; i += 2 {
			l, r := lim.tick(float64(block[i]), float64(block[i+1]))
			if done >= lead {
				out[2*(done-lead)] = float32(l)
				out[2*(done-lead)+1] = float32(r)
			}
			done++
		}
	}
	fadeEdges(out, secondsToSamples(0.020), secondsToSamples(0.050))
	return out
}

// fadeEdges applies raised-cosine fades of the given lengths (in frames) to both ends of an
// interleaved stereo buffer.
func fadeEdges(buf []float32, in, out int) {
	frames := len(buf) / 2
	in = min(in, frames/2)
	out = min(out, frames/2)
	for i := 0; i < in; i++ {
		g := float32(0.5 - 0.5*math.Cos(math.Pi*float64(i)/float64(in)))
		buf[2*i] *= g
		buf[2*i+1] *= g
	}
	for i := 0; i < out; i++ {
		g := float32(0.5 - 0.5*math.Cos(math.Pi*float64(i)/float64(out)))
		buf[2*(frames-1-i)] *= g
		buf[2*(frames-1-i)+1] *= g
	}
}
