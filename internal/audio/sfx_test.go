package audio

import (
	"math"
	"slices"
	"sync"
	"testing"
)

// specSfx is the required effect set, in the order the presentation specification lists it.
var specSfx = []string{
	"UIHover", "UIClick", "UIOpen", "UIClose", "UIError", "Place", "BuildDone", "Demolish",
	"ExplosionSmall", "ExplosionLarge", "Flak", "Lance", "ShipShot", "Bomb", "ShieldHit",
	"Launch", "Arrive", "Alert", "Notice", "Good", "TechDone", "DirectiveDone", "Coin",
	"SlugFire", "SlugImpact", "Meteor", "FlareStart", "ColonyFounded", "ColonyFell", "Victory",
	"Defeat",
}

// renderedSfx renders the whole set once for all tests that only read it.
var renderedSfx = sync.OnceValue(renderAllSfx)

func TestSfxSetMatchesSpec(t *testing.T) {
	if NumSfx != len(specSfx) {
		t.Fatalf("NumSfx = %d, want %d", NumSfx, len(specSfx))
	}
	for i, s := range AllSfx() {
		if int(s) != i || !s.Valid() {
			t.Errorf("AllSfx()[%d] = %d (valid %v)", i, int(s), s.Valid())
		}
		if s.String() != specSfx[i] {
			t.Errorf("effect %d is named %q, want %q", i, s.String(), specSfx[i])
		}
		def := sfxDefs[s]
		if def.build == nil || def.maxInst < 1 || def.minGap <= 0 || def.minDur >= def.maxDur {
			t.Errorf("%s: incomplete definition %+v", s, def)
		}
	}
	for _, bad := range []Sfx{-1, Sfx(NumSfx), 1000} {
		if bad.Valid() || bad.String() != "Sfx(invalid)" || RenderSfx(bad) != nil {
			t.Errorf("Sfx(%d) must be invalid, unnamed and render to nil", int(bad))
		}
	}
}

func TestSfxProperties(t *testing.T) {
	all := renderedSfx()
	stingers := []Sfx{Victory, Defeat}
	for _, s := range AllSfx() {
		t.Run(s.String(), func(t *testing.T) {
			buf := all[s]
			if len(buf) == 0 || len(buf)%2 != 0 {
				t.Fatalf("buffer length %d, want a positive even number (interleaved stereo)", len(buf))
			}
			for i, v := range buf {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
					t.Fatalf("sample %d is %v", i, v)
				}
			}
			l, r := splitStereo(buf)
			mid := midOf(buf)
			def := sfxDefs[s]
			dur := float64(len(l)) * invSR
			peak := math.Max(peakOf(l), peakOf(r))
			rms := linToDb(rmsOf(mid))
			t.Logf("%.3f s, peak %.3f, rms %.1f dBFS, loudness %.1f dB", dur, peak, rms, loudnessDB(l, r))

			if peak > 0.9 {
				t.Errorf("peak %.3f exceeds 0.9 (must stay clear of 1.0)", peak)
			}
			if peak < 0.05 {
				t.Errorf("peak %.3f: effectively silent", peak)
			}
			if rms < -36 || rms > -12 {
				t.Errorf("rms %.1f dBFS outside the sane band -36..-12", rms)
			}
			if peakOf(l) < peak/4 || peakOf(r) < peak/4 {
				t.Errorf("channels are lopsided: left peak %.3f, right peak %.3f", peakOf(l), peakOf(r))
			}

			if dur < def.minDur || dur > def.maxDur {
				t.Errorf("duration %.3f s outside its design range %.2f..%.2f", dur, def.minDur, def.maxDur)
			}
			if slices.Contains(stingers, s) {
				if dur < 2 || dur > 4 {
					t.Errorf("stinger lasts %.2f s, want 2..4", dur)
				}
			} else if dur < 0.04 || dur > 1.9 {
				t.Errorf("duration %.3f s outside 40 ms..1.9 s", dur)
			}

			// Clean boundaries: starts and ends at silence, so starting or finishing a voice
			// can never click.
			for _, ch := range [][]float64{l, r} {
				n := len(ch)
				if math.Abs(ch[0]) > 1e-3 || math.Abs(ch[n-1]) > 1e-3 {
					t.Errorf("does not start/end at silence: first %.5f, last %.5f", ch[0], ch[n-1])
				}
				if edge := peakOf(ch[n-8:]); edge > 0.01*peak {
					t.Errorf("last samples still at %.4f of the peak", edge/peak)
				}
				mean := 0.0
				for _, v := range ch {
					mean += v / float64(n)
				}
				if math.Abs(mean) > 0.004 {
					t.Errorf("DC offset %.5f", mean)
				}
				// No isolated discontinuity inside the sound either (a layer cut off dead).
				if ratio, at := kinkOutlier(ch, 0.004); ratio > 14 {
					t.Errorf("click at %.4f s: %.1f× the local roughness", float64(at)*invSR, ratio)
				}
			}

			if again := RenderSfx(s); !slices.Equal(again, buf) {
				t.Error("rendering twice gave different samples")
			}
		})
	}
}

func TestSfxLoudnessIsLevelled(t *testing.T) {
	all := renderedSfx()
	loud := make(map[Sfx]float64)
	for _, s := range AllSfx() {
		l, r := splitStereo(all[s])
		loud[s] = loudnessDB(l, r)
		target := sfxDefs[s].loudness
		peak := math.Max(peakOf(l), peakOf(r))
		// A sound either sits on its target or was held back by the peak ceiling.
		if d := loud[s] - target; math.Abs(d) > 0.3 && !(d < 0 && peak > 0.88) {
			t.Errorf("%s: loudness %.1f dB, target %.1f dB (peak %.3f)", s, loud[s], target, peak)
		}
		if loud[s] < -36 || loud[s] > -12 {
			t.Errorf("%s: loudness %.1f dB outside the set's range -36..-12", s, loud[s])
		}
	}
	// Interface sounds are heard hundreds of times: they must sit well under the big events.
	for _, ui := range []Sfx{UIHover, UIClick, UIOpen, UIClose, UIError} {
		for _, big := range []Sfx{ExplosionSmall, ExplosionLarge, Bomb, SlugImpact, Victory, Defeat} {
			if loud[ui] > loud[big]-5 {
				t.Errorf("%s (%.1f dB) is not clearly quieter than %s (%.1f dB)", ui, loud[ui], big, loud[big])
			}
		}
	}
	if loud[UIHover] >= loud[UIClick] {
		t.Errorf("hover (%.1f dB) should be quieter than click (%.1f dB)", loud[UIHover], loud[UIClick])
	}
	if loud[ExplosionSmall] >= loud[ExplosionLarge] {
		t.Errorf("small explosion (%.1f dB) should be quieter than large (%.1f dB)",
			loud[ExplosionSmall], loud[ExplosionLarge])
	}
}

// bursts counts the separate events in a sound: the number of times its 5 ms level envelope
// rises through half of its maximum.
func bursts(x []float64) int {
	win := secondsToSamples(0.005)
	var env []float64
	for i := 0; i+win <= len(x); i += win {
		env = append(env, rmsOf(x[i:i+win]))
	}
	limit := peakOf(env) / 2
	count, above := 0, false
	for _, e := range env {
		if e > limit && !above {
			count++
		}
		above = e > limit*0.6 && (above || e > limit)
	}
	return count
}

func TestSfxCharacter(t *testing.T) {
	all := renderedSfx()
	mono := func(s Sfx) []float64 { return midOf(all[s]) }
	centroid := func(x []float64) float64 { return spectralCentroid(powerSpectrum(x, 2048)) }
	band := func(s Sfx, lo, hi float64) float64 { return bandEnergy(powerSpectrum(mono(s), 2048), lo, hi) }
	halves := func(s Sfx) (first, second []float64) {
		x := mono(s)
		// Compare the opening and closing thirds of the body, ignoring the reverb tail.
		n := len(x) * 3 / 4
		return x[:n/3], x[2*n/3 : n]
	}

	t.Run("Lance is a bright beam, explosions are dark", func(t *testing.T) {
		lance, small, large := centroid(mono(Lance)), centroid(mono(ExplosionSmall)), centroid(mono(ExplosionLarge))
		t.Logf("centroids: Lance %.0f Hz, ExplosionSmall %.0f Hz, ExplosionLarge %.0f Hz", lance, small, large)
		if lance < 1800 {
			t.Errorf("Lance centroid %.0f Hz, want above 1800", lance)
		}
		if lance < 3*large || lance < 2*small {
			t.Errorf("Lance (%.0f Hz) is not clearly brighter than the explosions (%.0f, %.0f Hz)", lance, small, large)
		}
		if e := band(Lance, 0, 300); e > 0.05 {
			t.Errorf("Lance has %.2f of its energy below 300 Hz, want a thin, bright sound", e)
		}
	})
	t.Run("explosions have body and a tail", func(t *testing.T) {
		for _, s := range []Sfx{ExplosionSmall, ExplosionLarge, Bomb, SlugImpact} {
			if e := band(s, 0, 250); e < 0.5 {
				t.Errorf("%s: only %.2f of the energy below 250 Hz", s, e)
			}
			// ... but not only sub-bass: small speakers must still reproduce something.
			if e := band(s, 250, 4000); e < 0.04 {
				t.Errorf("%s: only %.3f of the energy in the mid-range", s, e)
			}
			x := mono(s)
			if tail := rmsOf(x[len(x)*2/3:]); tail < 0.01*peakOf(x) {
				t.Errorf("%s: no audible tail (last third at %.4f of the peak)", s, tail/peakOf(x))
			}
		}
		if len(all[ExplosionLarge]) < 2*len(all[ExplosionSmall]) {
			t.Error("the large explosion should last at least twice as long as the small one")
		}
	})
	t.Run("interface sounds are light", func(t *testing.T) {
		for _, s := range []Sfx{UIHover, UIClick, UIOpen, UIClose} {
			if c := centroid(mono(s)); c < 500 || c > 3000 {
				t.Errorf("%s centroid %.0f Hz, want 500..3000", s, c)
			}
			if e := band(s, 0, 150); e > 0.02 {
				t.Errorf("%s has %.3f of its energy below 150 Hz", s, e)
			}
		}
		if c := centroid(mono(UIError)); c > 500 {
			t.Errorf("UIError centroid %.0f Hz, want a low, dull sound", c)
		}
	})
	t.Run("Coin is a two-note chime", func(t *testing.T) {
		x := mono(Coin)
		split := secondsToSamples(0.07)
		first, second := x[:split], x[split:]
		// The first note is A5, the second D6 a fourth higher.
		if a, d := goertzel(first, hzA5), goertzel(first, hzD6); a < 3*d {
			t.Errorf("first note: A5 %.4f vs D6 %.4f, want A5 dominant", a, d)
		}
		if a, d := goertzel(second, hzA5), goertzel(second, hzD6); d < 3*a {
			t.Errorf("second note: D6 %.4f vs A5 %.4f, want D6 dominant", d, a)
		}
	})
	t.Run("rhythm", func(t *testing.T) {
		for _, tc := range []struct {
			s    Sfx
			want int
		}{{Flak, 3}, {Alert, 4}, {UIError, 2}, {UIClick, 1}, {ExplosionSmall, 1}} {
			if got := bursts(mono(tc.s)); got != tc.want {
				t.Errorf("%s: %d separate bursts, want %d", tc.s, got, tc.want)
			}
		}
	})
	t.Run("motion", func(t *testing.T) {
		a, b := halves(Launch)
		if centroid(b) < 1.5*centroid(a) {
			t.Errorf("Launch should rise: centroid %.0f → %.0f Hz", centroid(a), centroid(b))
		}
		a, b = halves(Lance)
		if centroid(b) > 0.9*centroid(a) {
			t.Errorf("Lance should fall in pitch: centroid %.0f → %.0f Hz", centroid(a), centroid(b))
		}
		a, b = halves(UIOpen)
		c, d := halves(UIClose)
		if centroid(b) <= centroid(a) || centroid(d) >= centroid(c) {
			t.Errorf("UIOpen should rise (%.0f → %.0f Hz) and UIClose fall (%.0f → %.0f Hz)",
				centroid(a), centroid(b), centroid(c), centroid(d))
		}
		x := mono(FlareStart)
		if early, mid := rmsOf(x[:len(x)/5]), rmsOf(x[len(x)*2/5:len(x)*3/5]); mid < 2*early {
			t.Errorf("FlareStart should swell: %.4f → %.4f", early, mid)
		}
	})
	t.Run("stingers", func(t *testing.T) {
		v, d := centroid(mono(Victory)), centroid(mono(Defeat))
		if v < 1.5*d {
			t.Errorf("Victory (%.0f Hz) should be clearly brighter than Defeat (%.0f Hz)", v, d)
		}
		// Victory settles on a D major chord, Defeat on D minor: compare the thirds.
		major, minor := goertzel(mono(Victory), hzFs4), goertzel(mono(Victory), hzF4)
		if major < 3*minor {
			t.Errorf("Victory: F#4 %.4f vs F4 %.4f, want a major third", major, minor)
		}
		major, minor = goertzel(mono(Defeat), hzFs4/2), goertzel(mono(Defeat), hzF3)
		if minor < 3*major {
			t.Errorf("Defeat: F3 %.4f vs F#3 %.4f, want a minor third", minor, major)
		}
	})
}

func TestRenderAllSfxMatchesSingleRenders(t *testing.T) {
	all := renderedSfx()
	for _, s := range []Sfx{UIClick, Demolish, ExplosionLarge, Flak, FlareStart} {
		if !slices.Equal(all[s], RenderSfx(s)) {
			t.Errorf("%s: the concurrent batch render differs from a single render", s)
		}
	}
}

func TestOfflineHelpers(t *testing.T) {
	t.Run("envelopes", func(t *testing.T) {
		tests := []struct {
			name      string
			got, want float64
		}{
			{"perc before start", perc(-1, 0.01, 0.1), 0},
			{"perc mid attack", perc(0.005, 0.01, 0.1), 0.5},
			{"perc one tau later", perc(0.11, 0.01, 0.1), 1 / math.E},
			{"trap start", trap(0, 1, 0.1, 0.1), 0},
			{"trap hold", trap(0.5, 1, 0.1, 0.1), 1},
			{"trap mid release", trap(0.95, 1, 0.1, 0.1), 0.5},
			{"trap end", trap(1, 1, 0.1, 0.1), 0},
			{"bump start", bump(0, 2, 0.25), 0},
			{"bump peak", bump(0.5, 2, 0.25), 1},
			{"bump end", bump(2, 2, 0.25), 0},
			{"glide start", glide(100, 400, 0), 100},
			{"glide middle is the geometric mean", glide(100, 400, 0.5), 200},
			{"glide clamps", glide(100, 400, 3), 400},
			{"drop start", drop(200, 50, 0, 0.1), 200},
			{"drop settles", drop(200, 50, 10, 0.1), 50},
		}
		for _, tc := range tests {
			if math.Abs(tc.got-tc.want) > 1e-9 {
				t.Errorf("%s: got %v, want %v", tc.name, tc.got, tc.want)
			}
		}
	})
	t.Run("add grows and tails", func(t *testing.T) {
		a := make(track, 10)
		b := track{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
		a = a.add(b, 0.0005, 0.5) // 22 samples in
		if len(a) != 32 || a[21] != 0 || a[22] != 0.5 {
			t.Fatalf("add placed the layer wrongly: len %d, a[21..22] = %v %v", len(a), a[21], a[22])
		}
		if a[31] != 0 {
			t.Errorf("a layer ending at full level was not tailed: last sample %v", a[31])
		}
	})
	t.Run("normalize and drive", func(t *testing.T) {
		x := track{0.1, -0.4, 0.2}.normalize()
		if peakOf(x) != 1 || x[1] != -1 {
			t.Errorf("normalize: %v", x)
		}
		if y := (track{0, 0}).normalize(); y[0] != 0 {
			t.Errorf("normalize of silence: %v", y)
		}
		d := track{1, -1, 0.5}.drive(2)
		if math.Abs(d[0]-1) > 1e-12 || math.Abs(d[1]+1) > 1e-12 || d[2] <= 0.5 {
			t.Errorf("drive: %v", d)
		}
	})
	t.Run("stereo placement", func(t *testing.T) {
		m := track{1, 1, 1, 1}
		var s stereo
		s = s.addPan(m, 0, 1, -1)
		if s.l[0] < 1.4 || math.Abs(s.r[0]) > 1e-9 {
			t.Errorf("hard left: l %v r %v", s.l[0], s.r[0])
		}
		c := stereo{}.addPan(track{1, 1, 1, 1}, 0, 1, 0)
		if math.Abs(c.l[0]-1) > 1e-9 || math.Abs(c.r[0]-1) > 1e-9 {
			t.Errorf("centre: l %v r %v", c.l[0], c.r[0])
		}
		w := widen(noise(&rng{s: 3}, 0.2), 0.3)
		sum := make(track, len(w.l))
		for i := range sum {
			sum[i] = (w.l[i] + w.r[i]) / 2
		}
		ref := noise(&rng{s: 3}, 0.2)
		for i := range ref {
			if math.Abs(sum[i]-ref[i]) > 1e-12 {
				t.Fatalf("widen changed the mono sum at %d", i)
			}
		}
	})
	t.Run("finalize", func(t *testing.T) {
		out := finalize(centre(make(track, 1000)), -20, 0.01)
		if len(out) != 2000 || peakOf(midOf(out)) != 0 {
			t.Errorf("silence must stay silent: len %d", len(out))
		}
		// A full-scale tone asked to be far louder than possible is held at the ceiling.
		var o osc
		tone := gen(0.5, func(float64) float64 { return o.sine(1000) })
		out = finalize(centre(tone), 0, 0.01)
		if p := peakOf(midOf(out)); p > 0.9 || p < 0.85 {
			t.Errorf("peak after finalize = %v, want held just under 0.9", p)
		}
	})
}
