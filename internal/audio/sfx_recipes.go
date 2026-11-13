package audio

import "math"

// The recipes. Each builds its sound from a few normalised layers; finalize then levels the
// result, so the gains below only set the balance between layers. Pitched material is tuned
// to D (the music's home note) and its fifth so effects never fight the score.

// Equal-tempered pitches used by the tonal effects (Hz).
const (
	hzD2  = 73.416
	hzA2  = 110.0
	hzD3  = 146.832
	hzF3  = 174.614
	hzA3  = 220.0
	hzC4  = 261.626
	hzD4  = 293.665
	hzF4  = 349.228
	hzFs4 = 369.994
	hzG4  = 391.995
	hzA4  = 440.0
	hzD5  = 587.330
	hzE5  = 659.255
	hzFs5 = 739.989
	hzA5  = 880.0
	hzB5  = 987.767
	hzD6  = 1174.659
	hzFs6 = 1479.978
	hzA6  = 1760.0
	hzD7  = 2349.318
	hzA7  = 3520.0
)

// sfxUIHover: a barely-there high tick.
func sfxUIHover(*rng) stereo {
	return centre(softTone(0.045, 1318.5, 0.003, 0.009, 0.2, 0))
}

// sfxUIClick: a soft wooden "tock" — a quick downward blip with a whisper of noise.
func sfxUIClick(r *rng) stereo {
	var o osc
	body := gen(0.07, func(t float64) float64 {
		return o.sine(drop(980, 540, t, 0.012)) * perc(t, 0.001, 0.014)
	})
	click := noise(r, 0.02).bandpass(3200, 1.2).shape(percEnv(0.0003, 0.004)).normalize()
	return centre(body.add(click, 0, 0.3))
}

// sfxUIOpen: two rising notes (D, A) with a small upward breath of air.
func sfxUIOpen(r *rng) stereo {
	m := softTone(0.12, hzD5, 0.004, 0.036, 0.18, 0.05)
	m = m.add(softTone(0.125, hzA5, 0.004, 0.040, 0.18, 0.05), 0.062, 0.9)
	air := noise(r, 0.17).sweepBand(1500, 4600, 1.5).normalize().
		shape(func(t float64) float64 { return bump(t, 0.17, 0.6) })
	return centre(m.add(air, 0, 0.07))
}

// sfxUIClose: the mirror image of UIOpen — falling notes, air sweeping down.
func sfxUIClose(r *rng) stereo {
	m := softTone(0.11, hzA5, 0.004, 0.032, 0.15, 0.04)
	m = m.add(softTone(0.115, hzD5, 0.004, 0.038, 0.15, 0.04), 0.058, 0.9)
	air := noise(r, 0.15).sweepBand(4200, 1200, 1.5).normalize().
		shape(func(t float64) float64 { return bump(t, 0.15, 0.3) })
	return centre(m.add(air, 0, 0.06))
}

// sfxUIError: two dull low pulses falling a minor third, roughened by a 29 Hz beat.
func sfxUIError(*rng) stereo {
	pulse := func(freq float64) track {
		var o1, o2, o3 osc
		return gen(0.105, func(t float64) float64 {
			v := o1.tri(freq) + 0.45*o2.pulse(freq/2, 0.3) + 0.3*o3.sine(freq+29)
			return v * trap(t, 0.105, 0.005, 0.035)
		}).lowpass(1100, 0.7)
	}
	m := pulse(233.08)
	m = m.add(pulse(196.0), 0.125, 0.95)
	return centre(m)
}

// sfxPlace: a structure set down — a low thunk under a woody knock, a small click and a faint
// locking tone.
func sfxPlace(r *rng) stereo {
	m := thump(0.14, 190, 85, 0.02, 0.001, 0.045)
	m = m.add(thump(0.08, 430, 215, 0.008, 0.001, 0.02), 0, 0.6)
	click := noise(r, 0.03).bandpass(2400, 1).shape(percEnv(0.0002, 0.005)).normalize()
	m = m.add(click, 0, 0.35)
	m = m.add(softTone(0.07, 622, 0.001, 0.02, 0.1, 0), 0.028, 0.2)
	return centre(m)
}

// sfxBuildDone: a bright three-note rising chime.
func sfxBuildDone(*rng) stereo {
	m := fmTone(0.45, hzD5, 2, 1.2, 0.08, 0.002, 0.16)
	m = m.add(fmTone(0.50, hzA5, 2, 1.2, 0.08, 0.002, 0.20), 0.11, 0.9)
	m = m.add(fmTone(0.40, hzD6, 2, 0.8, 0.06, 0.002, 0.16), 0.22, 0.45)
	return centre(m).room(0.22, 0.5, 0.7, 0.4, 0.25)
}

// sfxDemolish: a thud followed by crumbling debris and a couple of metallic clanks.
func sfxDemolish(r *rng) stereo {
	m := thump(0.4, 120, 42, 0.05, 0.002, 0.11)
	crumble := grains(r, 0.62, 16, 0, 0.5, 500, 3200, 0.012, 0.045).
		sweepLow(4200, 500, 0.7).normalize()
	m = m.add(crumble, 0.01, 0.8)
	for i := 0; i < 3; i++ {
		clank := fmTone(0.2, r.rangef(320, 820), 2.76, 2, 0.03, 0.001, 0.05)
		m = m.add(clank, r.rangef(0.05, 0.32), 0.16)
	}
	return widen(m, 0.25).room(0.12, 0.5, 0.5, 0.5, 0.15)
}

// explosion is the shared skeleton of the blast effects: a pitch-dropping body, a noise burst
// that darkens as it fades, a band of mid-range grit (what carries the blast on small
// speakers) and a short high snap, saturated for weight.
func explosion(r *rng, sec, f0, f1, pitchTau, bodyTau, noiseTau, cut0, cut1, drive float64) track {
	body := thump(sec, f0, f1, pitchTau, 0.0015, bodyTau)
	burst := noise(r, sec).shape(percEnv(0.0006, noiseTau)).sweepLow(cut0, cut1, 0.7).normalize()
	grit := noise(r, sec).bandpass(4*f0, 0.6).shape(percEnv(0.001, noiseTau*0.8)).normalize()
	snap := noise(r, 0.08).highpass(1800, 0.7).shape(percEnv(0.0002, 0.012)).normalize()
	m := body.add(burst, 0, 1.1).add(grit, 0, 0.7).add(snap, 0, 0.4)
	return m.normalize().drive(drive)
}

// sfxExplosionSmall: a compact blast with a short tail (a ship breaking up).
func sfxExplosionSmall(r *rng) stereo {
	m := explosion(r, 0.5, 160, 42, 0.07, 0.15, 0.13, 5000, 500, 1.8)
	return widen(m, 0.3).room(0.2, 0.7, 0.6, 0.5, 0.25)
}

// sfxExplosionLarge: a double boom with long rumble, crackle and a wide tail (a building).
func sfxExplosionLarge(r *rng) stereo {
	m := explosion(r, 1.2, 100, 38, 0.16, 0.42, 0.35, 3500, 320, 2.2)
	m = m.add(thump(1.0, 72, 34, 0.2, 0.004, 0.5), 0.06, 0.35)
	rumble := noise(r, 1.3).lowpass(260, 0.8).shape(percEnv(0.03, 0.5)).normalize()
	m = m.add(rumble, 0, 0.5)
	crackle := grains(r, 1.0, 34, 0.04, 0.85, 900, 2600, 0.004, 0.02).normalize()
	m = m.add(crackle, 0, 0.16)
	return widen(m.normalize(), 0.35).room(0.3, 1.1, 1.3, 0.55, 0.3)
}

// sfxFlak: three dry, tight thumps in quick succession.
func sfxFlak(r *rng) stereo {
	var m track
	at := 0.0
	for i, tune := range []float64{1, 0.92, 1.06} {
		pop := thump(0.09, 230*tune, 95, 0.012, 0.0005, 0.022)
		puff := noise(r, 0.09).bandpass(1300*tune, 0.9).shape(percEnv(0.0003, 0.014)).normalize()
		pop = pop.add(puff, 0, 0.6)
		m = m.add(pop, at, []float64{1, 0.85, 0.95}[i])
		at += 0.064 + r.rangef(-0.004, 0.004)
	}
	return widen(m.normalize().drive(1.5), 0.1)
}

// sfxLance: a bright, shimmering beam zap sliding down in pitch.
func sfxLance(r *rng) stereo {
	const dur = 0.46
	var o1, o2, o3 osc
	beam := gen(dur, func(t float64) float64 {
		f := glide(2600, 1150, math.Pow(t/dur, 0.7))
		vib := 1 + 0.035*sinT(t*43)
		v := o1.saw(f*vib) + 0.6*o2.saw(f*1.503*vib) + 0.5*o3.sine(f*0.5)
		trem := 0.8 + 0.2*sinT(t*71)
		return v * trem * math.Min(t/0.004, 1) * math.Exp(-math.Max(0, t-0.10)/0.11)
	}).tail(0.03).highpass(600, 0.7).lowpass(7500, 0.7).normalize()
	sizzle := noise(r, 0.4).highpass(5000, 0.7).shape(percEnv(0.002, 0.09)).normalize()
	m := beam.add(sizzle, 0, 0.14)
	return widen(m, 0.2).room(0.14, 0.5, 0.5, 0.3, 0.12)
}

// sfxShipShot: a short hollow "pew".
func sfxShipShot(r *rng) stereo {
	var o osc
	m := gen(0.18, func(t float64) float64 {
		return o.pulse(drop(1250, 300, t, 0.035), 0.3) * perc(t, 0.001, 0.05)
	}).tail(0.02).lowpass(4200, 0.8).normalize()
	click := noise(r, 0.03).highpass(2500, 0.7).shape(percEnv(0.0002, 0.008)).normalize()
	return centre(m.add(click, 0, 0.22)).room(0.08, 0.4, 0.4, 0.4, 0.06)
}

// sfxBomb: a brief falling whistle of air, then a heavy muffled whump.
func sfxBomb(r *rng) stereo {
	fall := noise(r, 0.1).sweepBand(3200, 700, 2).normalize().
		shape(func(t float64) float64 { return bump(t, 0.1, 0.8) })
	m := make(track, 0).add(fall, 0, 0.3)
	m = m.add(explosion(r, 0.6, 95, 40, 0.09, 0.24, 0.2, 1500, 260, 2), 0.09, 1)
	return widen(m, 0.25).room(0.24, 0.9, 0.9, 0.55, 0.25)
}

// sfxShieldHit: an energy field ringing — inharmonic partials with a fast flutter and a
// downward-swept fizz.
func sfxShieldHit(r *rng) stereo {
	const dur = 0.4
	freqs := [4]float64{432, 648, 1021, 1534}
	amps := [4]float64{1, 0.7, 0.5, 0.3}
	var ph [4]float64
	ring := gen(dur, func(t float64) float64 {
		bend := 1 - 0.06*t/dur
		v := 0.0
		for i := range freqs {
			v += amps[i] * sinT(ph[i])
			ph[i] += freqs[i] * bend * invSR
		}
		return v * (0.55 + 0.45*sinT(t*74)) * perc(t, 0.002, 0.10)
	}).tail(0.03).normalize()
	fizz := noise(r, 0.3).sweepBand(6500, 1400, 4).shape(percEnv(0.001, 0.05)).normalize()
	return centre(ring.add(fizz, 0, 0.3)).room(0.25, 0.6, 0.8, 0.25, 0.2)
}

// whoosh is a band of noise swept from f0 to f1 under a single swell that peaks at peak·sec,
// with a low engine tone following the same motion.
func whoosh(r *rng, sec, f0, f1, peak, eng0, eng1 float64) track {
	swell := func(t float64) float64 { return bump(t, sec, peak) }
	air := noise(r, sec).sweepBand(f0, f1, 1.3).normalize().shape(swell)
	var o1, o2 osc
	engine := gen(sec, func(t float64) float64 {
		f := glide(eng0, eng1, t/sec)
		return o1.saw(f) + 0.8*o2.sine(f*0.5)
	}).lowpass(520, 0.8).normalize().shape(swell)
	return air.add(engine, 0, 0.45)
}

// sfxLaunch: a rising whoosh as a fleet pulls away.
func sfxLaunch(r *rng) stereo {
	m := whoosh(r, 0.9, 260, 2800, 0.72, 52, 150)
	return widen(m, 0.4).room(0.15, 0.8, 0.8, 0.5, 0.2)
}

// sfxArrive: a falling whoosh that settles into two soft descending pings.
func sfxArrive(r *rng) stereo {
	m := whoosh(r, 0.55, 2600, 320, 0.18, 140, 55)
	m = m.add(fmTone(0.40, hzA5, 2, 1.0, 0.06, 0.003, 0.14), 0.33, 0.4)
	m = m.add(fmTone(0.30, hzD5, 2, 1.0, 0.06, 0.003, 0.12), 0.45, 0.32)
	return widen(m, 0.35).room(0.2, 0.7, 0.8, 0.45, 0.2)
}

// sfxAlert: an attention signal — a hollow two-tone call, twice.
func sfxAlert(*rng) stereo {
	beep := func(freq, dur float64) track {
		var o osc
		return gen(dur, func(t float64) float64 {
			ph := o.ph
			o.advance(freq)
			v := sinT(ph) + 0.3*sinT(3*ph) + 0.1*sinT(5*ph)
			return v * trap(t, dur, 0.006, 0.03)
		})
	}
	m := beep(hzA5, 0.13)
	m = m.add(beep(hzE5, 0.13), 0.16, 1)
	m = m.add(beep(hzA5, 0.13), 0.34, 1)
	m = m.add(beep(hzE5, 0.18), 0.50, 1)
	return centre(m.lowpass(5000, 0.7)).room(0.12, 0.5, 0.5, 0.4, 0.12)
}

// sfxNotice: one mellow mallet note with a faint echo.
func sfxNotice(*rng) stereo {
	note := fmTone(0.3, hzD6, 4, 0.9, 0.025, 0.001, 0.085)
	m := make(track, 0).add(note, 0, 1).add(note, 0.13, 0.26)
	return centre(m).room(0.15, 0.5, 0.5, 0.4, 0.1)
}

// sfxGood: a quick rising major arpeggio.
func sfxGood(*rng) stereo {
	m := fmTone(0.30, hzD5, 2, 1.0, 0.06, 0.002, 0.12)
	m = m.add(fmTone(0.30, hzFs5, 2, 1.0, 0.06, 0.002, 0.12), 0.07, 0.95)
	m = m.add(fmTone(0.40, hzA5, 2, 1.0, 0.06, 0.002, 0.18), 0.14, 0.95)
	return centre(m).room(0.18, 0.5, 0.6, 0.4, 0.15)
}

// sfxTechDone: a sparkling bell arpeggio climbing two octaves under a high shimmer.
func sfxTechDone(*rng) stereo {
	var s stereo
	notes := []float64{hzD5, hzA5, hzD6, hzFs6, hzA6}
	gains := []float64{1, 0.9, 0.8, 0.62, 0.48}
	for i, f := range notes {
		bell := fmTone(0.6, f, 3.5, 1.4, 0.10, 0.002, 0.28)
		s = s.addPan(bell, 0.065*float64(i), gains[i], -0.4+0.2*float64(i))
	}
	shimmer := gen(0.9, func(t float64) float64 {
		v := sinT(t*hzD7) + 0.7*sinT(t*hzA7)
		return v * (0.6 + 0.4*sinT(t*9)) * bump(t, 0.9, 0.35)
	})
	s = s.addPan(shimmer, 0.1, 0.035, 0)
	return s.room(0.3, 0.9, 1.1, 0.3, 0.35)
}

// sfxDirectiveDone: two warm brass-like chords, an open fifth answered by a full major chord.
func sfxDirectiveDone(*rng) stereo {
	var m track
	for _, f := range []float64{hzD3, hzA3, hzD4} {
		m = m.add(brassTone(0.22, f, 0.02, 0.08, 2400, 1200), 0, 0.8)
	}
	for _, f := range []float64{hzD3, hzFs4, hzA4, hzD5} {
		m = m.add(brassTone(0.62, f, 0.025, 0.3, 2800, 1400), 0.2, 0.8)
	}
	return centre(m).room(0.25, 0.9, 1.0, 0.4, 0.3)
}

// sfxCoin: a tiny two-note chime stepping up a fourth.
func sfxCoin(*rng) stereo {
	m := softTone(0.09, hzA5, 0.001, 0.035, 0.3, 0.12)
	m = m.add(softTone(0.28, hzD6, 0.001, 0.085, 0.3, 0.12), 0.07, 1)
	m = m.add(softTone(0.12, hzD6*2.76, 0.001, 0.03, 0, 0), 0.07, 0.07)
	return centre(m).room(0.1, 0.4, 0.4, 0.4, 0.08)
}

// sfxSlugFire: a Mass Driver shot — a rising charge whine released into a hard thump, a crack
// and a short metallic ring.
func sfxSlugFire(r *rng) stereo {
	const charge = 0.07
	const release = 0.012 // the whine is cut off by the shot, but smoothly
	var o1, o2 osc
	whine := gen(charge+release, func(t float64) float64 {
		f := glide(170, 2100, t/charge)
		x := math.Min(t/charge, 1)
		return (o1.sine(f) + 0.4*o2.saw(f)) * x * x
	}).tail(release).lowpass(5000, 0.7).normalize()
	m := make(track, 0).add(whine, 0, 0.4)
	shot := thump(0.35, 200, 40, 0.03, 0.0005, 0.10)
	crack := noise(r, 0.12).highpass(1400, 0.7).shape(percEnv(0.0002, 0.022)).normalize()
	shot = shot.add(crack, 0, 0.7)
	ring := gen(0.4, func(t float64) float64 {
		return (sinT(t*311) + 0.7*sinT(t*466.2) + 0.4*sinT(t*933)) * perc(t, 0.001, 0.13)
	})
	shot = shot.add(ring, 0, 0.14).normalize().drive(1.8)
	m = m.add(shot, charge, 1)
	return widen(m, 0.2).room(0.16, 0.7, 0.7, 0.45, 0.2)
}

// sfxSlugImpact: a slug striking a structure — boom, shattering debris and a clang.
func sfxSlugImpact(r *rng) stereo {
	m := explosion(r, 0.8, 125, 32, 0.08, 0.26, 0.2, 5200, 200, 2)
	debris := grains(r, 0.7, 18, 0.06, 0.6, 800, 2500, 0.008, 0.03).normalize()
	m = m.add(debris, 0, 0.3)
	freqs := [4]float64{523, 790, 1217, 1731}
	taus := [4]float64{0.25, 0.2, 0.15, 0.1}
	clang := gen(0.7, func(t float64) float64 {
		v := 0.0
		for i := range freqs {
			v += sinT(t*freqs[i]) * perc(t, 0.001, taus[i]) / float64(i+1)
		}
		return v
	}).normalize()
	m = m.add(clang, 0.005, 0.14)
	return widen(m, 0.3).room(0.26, 1.0, 1.0, 0.5, 0.3)
}

// sfxMeteor: a short incoming hiss, then a gritty strike and scattering fragments.
func sfxMeteor(r *rng) stereo {
	const lead = 0.16
	hiss := noise(r, lead+0.012).sweepBand(5200, 900, 1.6).normalize().
		shape(func(t float64) float64 { return math.Pow(math.Min(t/lead, 1), 2.5) }).tail(0.012)
	m := make(track, 0).add(hiss, 0, 0.5)
	hit := explosion(r, 0.4, 140, 44, 0.05, 0.16, 0.17, 2800, 300, 1.7)
	m = m.add(hit, lead, 1)
	debris := grains(r, 0.5, 12, 0.03, 0.4, 700, 2800, 0.008, 0.03).normalize()
	m = m.add(debris, lead, 0.28)
	return widen(m, 0.3).room(0.2, 0.8, 0.8, 0.5, 0.25)
}

// sfxFlareStart: a solar flare swelling — a rising drone, fluttering bright noise and static.
func sfxFlareStart(r *rng) stereo {
	const dur = 1.3
	swell := func(t float64) float64 { return bump(t, dur, 0.6) }
	hiss := noise(r, dur).sweepBand(900, 4800, 2.2).normalize().
		shape(func(t float64) float64 { return swell(t) * (0.7 + 0.3*sinT(t*13)) })
	var o1, o2, o3 osc
	drone := gen(dur, func(float64) float64 {
		return o1.saw(hzD3) + o2.saw(hzA3*1.002) + 0.7*o3.saw(hzD4*0.998)
	}).sweepLow(300, 2600, 0.9).normalize().shape(swell)
	static := grains(r, dur, 50, 0.2, dur-0.1, 3000, 6500, 0.002, 0.008).normalize().shape(swell)
	m := hiss.add(drone, 0, 0.55).add(static, 0, 0.22)
	return widen(m, 0.5).room(0.3, 1.0, 1.2, 0.3, 0.2)
}

// padPluck is a soft, quickly opening and slowly fading chord tone.
func padPluck(sec, freq, tau float64) track {
	var o1, o2 osc
	return gen(sec, func(t float64) float64 {
		return (o1.saw(freq*0.997) + o2.saw(freq*1.003)) * perc(t, 0.05, tau)
	}).tail(0.2).lowpass(1700, 0.7)
}

// sfxColonyFounded: a warm major chord unfolding upward, crowned by two bell notes.
func sfxColonyFounded(*rng) stereo {
	var s stereo
	for i, f := range []float64{hzD3, hzA3, hzD4, hzFs4, hzA4} {
		s = s.addPan(padPluck(1.1, f, 0.45), 0.07*float64(i), 0.8, -0.3+0.15*float64(i))
	}
	s = s.addPan(fmTone(0.7, hzD5, 2, 1.0, 0.1, 0.002, 0.25), 0.42, 0.9, -0.2)
	s = s.addPan(fmTone(0.7, hzA5, 2, 1.0, 0.1, 0.002, 0.28), 0.56, 0.8, 0.2)
	return s.room(0.3, 1.0, 1.3, 0.35, 0.35)
}

// sfxColonyFell: a dull boom under three falling, drooping tones in D minor.
func sfxColonyFell(r *rng) stereo {
	m := thump(1.0, 110, 48, 0.15, 0.003, 0.4)
	dust := noise(r, 1.0).lowpass(420, 0.7).shape(percEnv(0.01, 0.45)).normalize()
	m = m.add(dust, 0, 0.3)
	for i, f := range []float64{hzD4, hzC4, hzA3} {
		var o osc
		tone := gen(0.5, func(t float64) float64 {
			return o.tri(f*(1-0.03*t/0.5)) * trap(t, 0.5, 0.02, 0.35)
		}).lowpass(800, 0.7)
		m = m.add(tone, 0.24*float64(i), 0.55)
	}
	var o1, o2 osc
	pad := gen(1.2, func(t float64) float64 {
		return (o1.saw(hzD3) + o2.saw(hzF3)) * bump(t, 1.2, 0.3)
	}).lowpass(500, 0.7).normalize()
	m = m.add(pad, 0, 0.3)
	return widen(m, 0.2).room(0.32, 1.1, 1.5, 0.6, 0.3)
}

// sfxVictory: a rising arpeggio into a sustained, glowing D major chord with a short melody
// on top (about 3.5 s).
func sfxVictory(*rng) stereo {
	var s stereo
	for i, f := range []float64{hzD4, hzA4, hzD5, hzFs5} {
		s = s.addPan(brassTone(0.26, f, 0.012, 0.12, 3000, 1500), 0.09*float64(i), 0.7, -0.3+0.2*float64(i))
	}
	const hit = 0.36
	chord := []float64{hzD3, hzA3, hzD4, hzFs4, hzA4, hzE5}
	for i, f := range chord {
		g := 0.7
		if i == len(chord)-1 {
			g = 0.4 // the added ninth is colour, not weight
		}
		s = s.addPan(brassTone(2.2, f, 0.06, 1.3, 3000, 1500), hit, g, -0.5+0.2*float64(i))
	}
	s = s.addPan(thump(0.8, 110, hzD2, 0.05, 0.002, 0.3), hit, 0.9, 0)
	for _, n := range []struct{ f, at, dur float64 }{
		{hzA5, hit, 0.6}, {hzB5, hit + 0.54, 0.45}, {hzD6, hit + 0.94, 1.3},
	} {
		s = s.addPan(brassTone(n.dur, n.f, 0.03, n.dur*0.5, 3600, 2000), n.at, 0.5, 0.1)
	}
	for i, f := range []float64{hzD6, hzA6, hzFs6, hzD7} {
		bell := fmTone(0.9, f, 3.5, 1.2, 0.12, 0.002, 0.35)
		s = s.addPan(bell, hit+0.14*float64(i), 0.22, 0.5-0.3*float64(i))
	}
	return s.room(0.35, 1.2, 2.2, 0.35, 0.9)
}

// sfxDefeat: a low boom and a dark D minor pad under a slow descending line (about 3.5 s).
func sfxDefeat(*rng) stereo {
	var s stereo
	s = s.addPan(thump(1.5, 95, hzA2/2, 0.2, 0.004, 0.6), 0, 0.8, 0)
	for i, f := range []float64{hzD2, hzA2, hzD3, hzF3} {
		var o1, o2 osc
		pad := gen(2.6, func(t float64) float64 {
			return (o1.saw(f*0.997) + o2.saw(f*1.003)) * trap(t, 2.6, 0.3, 1.5)
		}).lowpass(520, 0.7)
		s = s.addPan(pad, 0, 0.5, -0.3+0.2*float64(i))
	}
	for _, n := range []struct{ f, at, tau float64 }{
		{hzA4, 0.10, 0.42}, {hzG4, 0.62, 0.42}, {hzF4, 1.14, 0.45}, {hzD4, 1.70, 0.6},
	} {
		tone := fmTone(n.tau*3.2, n.f, 1, 0.9, 0.3, 0.03, n.tau)
		s = s.addPan(tone, n.at, 0.55, 0.05)
	}
	return s.room(0.4, 1.3, 2.0, 0.6, 0.5)
}
