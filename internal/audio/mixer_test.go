package audio

import (
	"encoding/binary"
	"math"
	"slices"
	"sync"
	"testing"
	"time"
)

// testMixer returns a mixer loaded with the real effects.
func testMixer() *mixer { return newMixer(renderedSfx()) }

// renderFrames pulls the given number of frames from a mixer.
func renderFrames(m *mixer, frames int) []float32 {
	out := make([]float32, 2*frames)
	m.render(out)
	return out
}

// smoothTone is a stand-in effect for click tests: a loud, pure 100 Hz tone with faded ends.
// Its own waveform is so smooth that any discontinuity the mixer adds stands out.
func smoothTone(sec float64) []float32 {
	n := secondsToSamples(sec)
	buf := make([]float32, 2*n)
	for i := 0; i < n; i++ {
		v := float32(0.5 * math.Sin(2*math.Pi*100*float64(i)*invSR))
		buf[2*i], buf[2*i+1] = v, v
	}
	fadeEdges(buf, secondsToSamples(0.02), secondsToSamples(0.02))
	return buf
}

func TestPanGains(t *testing.T) {
	tests := []struct {
		vol, pan float32
		l, r     float64
	}{
		{1, 0, 1, 1},
		{1, -1, 1, 0},
		{1, 1, 0, 1},
		{0.5, 0, 0.5, 0.5},
		{1, -0.5, 1, math.Sqrt(0.5)},
		{1, 0.75, 0.5, 1},
		{2, 5, 0, 2},  // pan is clamped
		{1, -9, 1, 0}, // pan is clamped
	}
	for _, tc := range tests {
		l, r := panGains(tc.vol, tc.pan)
		if math.Abs(float64(l)-tc.l) > 1e-6 || math.Abs(float64(r)-tc.r) > 1e-6 {
			t.Errorf("panGains(%v, %v) = %v, %v; want %v, %v", tc.vol, tc.pan, l, r, tc.l, tc.r)
		}
		if l > tc.vol || r > tc.vol {
			t.Errorf("panGains(%v, %v) boosts a channel above the volume", tc.vol, tc.pan)
		}
	}
}

func TestMixerPlaysEffectUnaltered(t *testing.T) {
	m := testMixer()
	src := m.sfx[BuildDone]
	m.play(BuildDone, 1, 0)
	out := renderFrames(m, len(src)/2+1000)
	// At unity volume and below the limiter's threshold the effect comes out sample for sample,
	// delayed by the limiter's look-ahead.
	lead := 2 * m.lim.lookahead
	if peakOf(midOf(out[:lead])) != 0 {
		t.Error("output before the look-ahead delay is not silent")
	}
	if !slices.Equal(out[lead:lead+len(src)], src) {
		t.Error("the mixed effect differs from its source buffer")
	}
	if peakOf(midOf(out[lead+len(src):])) != 0 {
		t.Error("output after the effect ended is not silent")
	}
	if m.activeVoices() != 0 {
		t.Errorf("%d voices still active after the effect ended", m.activeVoices())
	}
}

func TestMixerPanAndVolume(t *testing.T) {
	levels := func(vol, pan float32) (l, r float64) {
		m := testMixer()
		m.play(Alert, vol, pan)
		a, b := splitStereo(renderFrames(m, SampleRate))
		return rmsOf(a), rmsOf(b)
	}
	cl, cr := levels(1, 0)
	if cl == 0 || math.Abs(cl-cr) > 0.05*cl {
		t.Fatalf("centre: left %v right %v, want equal and audible", cl, cr)
	}
	if l, r := levels(1, -1); r != 0 || math.Abs(l-cl) > 1e-6 {
		t.Errorf("hard left: left %v (centre %v) right %v", l, cl, r)
	}
	if l, r := levels(1, 1); l != 0 || math.Abs(r-cr) > 1e-6 {
		t.Errorf("hard right: left %v right %v (centre %v)", l, r, cr)
	}
	if l, r := levels(1, 0.5); l >= r || l < 0.6*cl {
		t.Errorf("half right: left %v right %v", l, r)
	}
	if l, _ := levels(0.5, 0); math.Abs(l-0.5*cl) > 1e-6 {
		t.Errorf("half volume: %v, want %v", l, 0.5*cl)
	}
}

func TestMixerVolumeBuses(t *testing.T) {
	level := func(master, sfx, music float64, withMusic bool) float64 {
		m := testMixer()
		m.setVolume(master, sfx, music)
		renderFrames(m, volumeGlide) // let the volumes finish gliding to their targets
		if withMusic {
			m.setMood(newMusicGen(Battle, 1))
			renderFrames(m, 4*SampleRate) // let the fade-in finish
		}
		m.play(Alert, 1, 0)
		return rmsOf(midOf(renderFrames(m, SampleRate)))
	}
	full := level(1, 1, 1, false)
	if got := level(1, 0.25, 1, false); math.Abs(got-0.25*full) > 1e-4 {
		t.Errorf("effects bus at 0.25: %v, want %v", got, 0.25*full)
	}
	if got := level(0.5, 1, 1, false); math.Abs(got-0.5*full) > 1e-4 {
		t.Errorf("master at 0.5: %v, want %v", got, 0.5*full)
	}
	if got := level(0, 1, 1, true); got != 0 {
		t.Errorf("master at 0: output %v, want silence", got)
	}
	if got := level(1, 0, 0, true); got != 0 {
		t.Errorf("effects and music at 0: output %v, want silence", got)
	}
	musicOnly := level(1, 0, 1, true)
	if musicOnly <= 0 || musicOnly > 0.2 {
		t.Errorf("music only: level %v", musicOnly)
	}
	if got := level(1, 0, 0.5, true); math.Abs(got-0.5*musicOnly) > 0.02*musicOnly {
		t.Errorf("music bus at 0.5: %v, want %v", got, 0.5*musicOnly)
	}
}

func TestMixerVolumeChangeIsSmooth(t *testing.T) {
	m := testMixer()
	m.sfx[Alert] = smoothTone(1)
	m.play(Alert, 1, 0)
	out := renderFrames(m, 8000)
	m.setVolume(0.2, 1, 1)
	for i := 0; i < 8000; i += 5 { // however small the chunks the device asks for
		out = append(out, renderFrames(m, 5)...)
	}
	m.setVolume(1, 1, 1)
	out = append(out, renderFrames(m, 8000)...)
	l, _ := splitStereo(out)
	if before, after := peakOf(l[4000:8000]), peakOf(l[12000:16000]); math.Abs(after-0.2*before) > 0.01 {
		t.Fatalf("volume change not applied: peak %v → %v", before, after)
	}
	// A 0.5 amplitude 100 Hz tone has second differences of 1e-4; an abrupt gain step would
	// produce one of about 0.4.
	if kink, at := maxKink(l); kink > 0.004 {
		t.Errorf("volume change clicks: kink %.4f at frame %d", kink, at)
	}
}

func TestMixerVoiceCap(t *testing.T) {
	m := testMixer()
	long := []Sfx{ExplosionLarge, Victory, Defeat, ColonyFell, ColonyFounded, FlareStart, TechDone,
		DirectiveDone, Launch, Arrive, SlugImpact, Bomb, Meteor, Demolish, BuildDone, Alert,
		ExplosionSmall, Lance, ShieldHit, Good, Notice, SlugFire}
	if len(long) <= maxVoices {
		t.Fatal("the test needs more distinct effects than there are voices")
	}
	for _, s := range long {
		m.play(s, 1, 0)
	}
	renderFrames(m, 64)
	if got := m.activeVoices(); got != maxVoices {
		t.Errorf("%d voices active after %d triggers, want the cap of %d", got, len(long), maxVoices)
	}
	// The newest sound always gets a voice; the ones displaced are fading, not cut.
	newest := false
	fading := 0
	for i := range m.voices {
		v := &m.voices[i]
		if v.live && v.fade == 0 && v.sfx == long[len(long)-1] {
			newest = true
		}
		if v.live && v.fade > 0 {
			fading++
		}
	}
	if !newest {
		t.Error("the most recent trigger did not get a voice")
	}
	if want := len(long) - maxVoices; fading != want {
		t.Errorf("%d voices fading out, want %d", fading, want)
	}
	renderFrames(m, stealFade+16)
	live := 0
	for i := range m.voices {
		if m.voices[i].live {
			live++
		}
	}
	if live != maxVoices {
		t.Errorf("%d voices alive after the fade-outs, want %d", live, maxVoices)
	}
	// A storm of triggers never overflows the slots or the output.
	for round := 0; round < 50; round++ {
		for _, s := range long {
			m.play(s, 2, 0)
		}
		out := renderFrames(m, 300)
		if m.activeVoices() > maxVoices {
			t.Fatalf("round %d: %d active voices", round, m.activeVoices())
		}
		for _, v := range out {
			if math.IsNaN(float64(v)) || math.Abs(float64(v)) > clipCeiling {
				t.Fatalf("round %d: sample %v outside ±%v", round, v, clipCeiling)
			}
		}
	}
}

func TestMixerInstanceCap(t *testing.T) {
	m := testMixer()
	count := func(s Sfx) int {
		n := 0
		for i := range m.voices {
			if v := &m.voices[i]; v.live && v.fade == 0 && v.sfx == s {
				n++
			}
		}
		return n
	}
	for _, s := range []Sfx{ExplosionLarge, Victory, UIClick} {
		for i := 0; i < 8; i++ {
			m.play(s, 1, 0)
			renderFrames(m, 300)
			if got, limit := count(s), sfxDefs[s].maxInst; got > limit {
				t.Fatalf("%s: %d instances sounding, limit %d", s, got, limit)
			}
		}
	}
	if got := count(ExplosionLarge); got != sfxDefs[ExplosionLarge].maxInst {
		t.Errorf("ExplosionLarge: %d instances after 8 triggers, want %d", got, sfxDefs[ExplosionLarge].maxInst)
	}
}

func TestMixerReplacingVoiceDoesNotClick(t *testing.T) {
	m := testMixer()
	m.sfx[Alert] = smoothTone(2) // Alert allows a single instance
	m.play(Alert, 1, 0)
	out := renderFrames(m, 20000)
	m.play(Alert, 1, 0) // replaces the first one mid-flight
	out = append(out, renderFrames(m, 20000)...)
	l, _ := splitStereo(out)
	if peakOf(l[25000:]) < 0.2 {
		t.Fatal("the replacement voice is not sounding")
	}
	if kink, at := maxKink(l); kink > 0.01 {
		t.Errorf("replacing a voice clicks: kink %.4f at frame %d (a hard cut would be about 0.4)", kink, at)
	}
}

func TestMixerNeverClips(t *testing.T) {
	m := testMixer()
	m.setMood(newMusicGen(Battle, 3))
	renderFrames(m, 4*SampleRate)
	peak := 0.0
	for round := 0; round < 40; round++ {
		for _, s := range []Sfx{ExplosionLarge, Bomb, SlugImpact, ColonyFell, Victory, Defeat, Meteor, SlugFire} {
			m.play(s, 2, float32(round%3-1))
		}
		for _, v := range renderFrames(m, 4000) {
			if math.IsNaN(float64(v)) {
				t.Fatal("NaN in the mix")
			}
			peak = math.Max(peak, math.Abs(float64(v)))
		}
	}
	t.Logf("peak of a heavily overloaded mix: %.3f", peak)
	if peak > clipCeiling {
		t.Errorf("peak %.4f exceeds the ceiling %.2f", peak, clipCeiling)
	}
	if peak < limiterThreshold-0.05 {
		t.Errorf("peak %.3f: the overload test did not reach the limiter", peak)
	}
}

func TestMixerMoodCrossFade(t *testing.T) {
	m := testMixer()
	if out := renderFrames(m, 2000); peakOf(midOf(out)) != 0 {
		t.Fatal("a mixer with no mood set must be silent")
	}
	m.setMood(newMusicGen(Calm, 1))
	first := renderFrames(m, 6*SampleRate)
	if len(m.layers) != 1 || m.layers[0].pos != 1 {
		t.Fatalf("after the fade-in: %d layers, position %v", len(m.layers), m.layers[0].pos)
	}
	// The first mood fades in from silence over the cross-fade time.
	fl := midOf(first)
	early, settled := rmsOf(fl[:SampleRate/4]), rmsOf(fl[4*SampleRate:])
	if early > 0.3*settled || settled == 0 {
		t.Errorf("fade-in: first 250 ms at %v, settled %v", early, settled)
	}

	// Asking for the same mood again changes nothing.
	m.setMood(newMusicGen(Calm, 2))
	first = append(first, renderFrames(m, 100)...)
	if len(m.layers) != 1 {
		t.Fatalf("re-requesting the current mood created a layer (%d layers)", len(m.layers))
	}

	m.setMood(newMusicGen(Battle, 1))
	mid := renderFrames(m, SampleRate) // one second into the three-second cross-fade
	if len(m.layers) != 2 {
		t.Fatalf("during the cross-fade: %d layers, want 2", len(m.layers))
	}
	rest := renderFrames(m, 5*SampleRate)
	if len(m.layers) != 1 || m.layers[0].gen.mood != Battle || m.layers[0].pos != 1 {
		t.Fatalf("after the cross-fade: %d layers, top mood %v", len(m.layers), m.layers[0].gen.mood)
	}
	// Equal-power fade: the level through the transition stays close to the levels around it,
	// with neither a hole nor a bump, and nothing clicks.
	xl := midOf(append(mid, rest...))
	during, after := linToDb(rmsOf(xl[:3*SampleRate])), linToDb(rmsOf(xl[4*SampleRate:]))
	before := linToDb(settled)
	t.Logf("level before %.1f dB, during %.1f dB, after %.1f dB", before, during, after)
	if during < math.Min(before, after)-3 || during > math.Max(before, after)+3 {
		t.Errorf("cross-fade level %.1f dB strays from %.1f / %.1f dB", during, before, after)
	}
	whole, _ := splitStereo(append(append(first, mid...), rest...))
	if ratio, at := kinkOutlier(whole, 0.004); ratio > 14 {
		t.Errorf("click at %.3f s: %.1f× the local roughness", float64(at)*invSR, ratio)
	}
}

func TestMixerMoodFlipFlop(t *testing.T) {
	m := testMixer()
	m.setMood(newMusicGen(Calm, 1))
	renderFrames(m, 4*SampleRate)
	calm := m.layers[0].gen
	m.setMood(newMusicGen(Battle, 1))
	renderFrames(m, SampleRate)
	// Going straight back re-uses the Calm generator that is still fading out.
	m.setMood(newMusicGen(Calm, 9))
	renderFrames(m, 100)
	if len(m.layers) != 2 {
		t.Fatalf("%d layers after flipping back, want 2", len(m.layers))
	}
	for _, l := range m.layers {
		if l.gen.mood == Calm && (l.gen != calm || l.target != 1) {
			t.Error("the fading Calm layer was not brought back")
		}
		if l.gen.mood == Battle && l.target != 0 {
			t.Error("the Battle layer is not fading out")
		}
	}
	renderFrames(m, 4*SampleRate)
	if len(m.layers) != 1 || m.layers[0].gen != calm {
		t.Fatalf("after settling: %d layers", len(m.layers))
	}
	// A burst of mood changes keeps the number of live generators bounded and stays clean.
	var out []float32
	for i, mood := range []Mood{Tense, Battle, Triumph, Lament, Menu, Tense, Battle, Calm} {
		m.setMood(newMusicGen(mood, uint64(i)))
		out = append(out, renderFrames(m, SampleRate/5)...)
		if len(m.layers) > maxLayers+1 {
			t.Fatalf("%d music layers alive during rapid mood changes", len(m.layers))
		}
	}
	out = append(out, renderFrames(m, 4*SampleRate)...)
	if len(m.layers) != 1 || m.layers[0].gen.mood != Calm {
		t.Fatalf("after the burst: %d layers", len(m.layers))
	}
	l, _ := splitStereo(out)
	if ratio, at := kinkOutlier(l, 0.004); ratio > 14 {
		t.Errorf("click at %.3f s during rapid mood changes: %.1f× the local roughness", float64(at)*invSR, ratio)
	}
}

func TestMixerPausesMusicWhenMuted(t *testing.T) {
	m := testMixer()
	m.setMood(newMusicGen(Calm, 1))
	renderFrames(m, SampleRate)
	gen := m.layers[0].gen
	m.setVolume(1, 1, 0)
	// Time for the volume to glide to zero and for the limiter's look-ahead to drain.
	renderFrames(m, volumeGlide+m.lim.lookahead)
	step := gen.step
	out := renderFrames(m, 10*SampleRate)
	if gen.step != step {
		t.Errorf("the generator advanced %d steps while the music was muted", gen.step-step)
	}
	if peakOf(midOf(out)) != 0 {
		t.Error("muted music is audible")
	}
	m.setVolume(1, 1, 1)
	out = renderFrames(m, SampleRate)
	if gen.step == step || peakOf(midOf(out)) == 0 {
		t.Error("the music did not resume when unmuted")
	}
}

func TestMixerQueueIsBounded(t *testing.T) {
	m := testMixer()
	for i := 0; i < 2*maxPending; i++ { // nobody is rendering
		m.play(UIClick, 1, 0)
		m.setVolume(1, 1, float64(i%10)/10)
		m.setMood(newMusicGen(Mood(i%NumMoods), 1))
	}
	m.setMood(newMusicGen(Lament, 1))
	if len(m.pending) != maxPending {
		t.Fatalf("%d triggers queued, want the limit %d", len(m.pending), maxPending)
	}
	renderFrames(m, 64)
	// Only the latest volume and the latest mood are applied.
	if want := float64((2*maxPending-1)%10) / 10; m.volTo[2] != want {
		t.Errorf("music volume after the backlog = %v, want the last value %v", m.volTo[2], want)
	}
	if len(m.layers) != 1 || m.layers[0].gen.mood != Lament {
		t.Errorf("after the backlog: %d layers, want only the last mood requested", len(m.layers))
	}
	if len(m.pending) != 0 || m.wantMood != nil || m.hasVol {
		t.Error("requests were not consumed by the render")
	}
	m.play(Sfx(-1), 1, 0) // ignored
	m.setMood(nil)        // ignored
	renderFrames(m, 64)
}

func decodeF32(raw []byte) []float32 {
	out := make([]float32, len(raw)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
	}
	return out
}

func TestStreamRead(t *testing.T) {
	setup := func() *mixer {
		m := testMixer()
		m.setMood(newMusicGen(Tense, 5))
		m.play(TechDone, 1, -0.3)
		return m
	}
	const frames = 30000
	want := renderFrames(setup(), frames)

	// Reading in awkward chunk sizes, including ones smaller than a frame and ones that are
	// not a multiple of a frame, yields exactly the same stream.
	s := &stream{m: setup()}
	sizes := []int{8, 3, 1, 13, 4096, 5, 800, 7, 16, 1 << 14, 2, 9}
	var raw []byte
	for i := 0; len(raw) < frames*frameBytes; i++ {
		p := make([]byte, sizes[i%len(sizes)])
		n, err := s.Read(p)
		if err != nil {
			t.Fatalf("Read returned error %v", err)
		}
		if n <= 0 || n > len(p) {
			t.Fatalf("Read(%d bytes) returned %d", len(p), n)
		}
		raw = append(raw, p[:n]...)
	}
	got := decodeF32(raw[:frames*frameBytes])
	if !slices.Equal(got, want) {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("stream differs from a direct render at sample %d: %v vs %v", i, got[i], want[i])
			}
		}
	}
	if n, err := s.Read(nil); n != 0 || err != nil {
		t.Errorf("Read(nil) = %d, %v", n, err)
	}
}

func TestRateLimiter(t *testing.T) {
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	var l rateLimiter
	steps := []struct {
		s    Sfx
		at   time.Duration
		want bool
	}{
		{UIClick, ms(0), true},     // first trigger always plays
		{UIClick, ms(10), false},   // 10 ms later: too soon (gap 40 ms)
		{UIClick, ms(39), false},   // still inside the gap of the trigger that played
		{UIClick, ms(40), true},    // gap elapsed
		{UIHover, ms(41), true},    // other effects have their own clocks
		{Flak, ms(41), true},       //
		{Flak, ms(100), false},     // 59 ms after the last Flak (gap 90 ms)
		{Flak, ms(131), true},      //
		{Victory, ms(200), true},   //
		{Victory, ms(2000), false}, // stingers do not stack
		{Victory, ms(3200), true},  //
		{Sfx(-1), ms(5000), false}, // invalid effects never play
		{Sfx(NumSfx), ms(5000), false},
	}
	for i, st := range steps {
		if got := l.allow(st.s, st.at); got != st.want {
			t.Errorf("step %d: allow(%v, %v) = %v, want %v", i, st.s, st.at, got, st.want)
		}
	}
	// Every effect has a sensible gap: long enough to stop machine-gunning, short next to
	// its own length for the brief ones.
	for _, s := range AllSfx() {
		gap := sfxDefs[s].minGap
		if gap < 0.03 || gap > 3 {
			t.Errorf("%s: retrigger gap %v s", s, gap)
		}
	}
}

func TestSilentEngine(t *testing.T) {
	var nilEngine *Engine
	for name, e := range map[string]*Engine{"NewSilent": NewSilent(), "nil": nilEngine} {
		if !e.Silent() {
			t.Errorf("%s: Silent() = false", name)
		}
		// Every method is a harmless no-op.
		e.Play(UIClick)
		e.PlayAt(ExplosionLarge, 1, 0.5)
		e.PlayAt(Sfx(-5), 1, 0)
		e.SetMood(Battle)
		e.SetMood(Mood(77))
		e.SetVolume(1, 0.5, 0.2)
		e.Update()
		e.Close()
		e.Close()
	}
	if e := NewSilent(); e.mx != nil || e.player != nil {
		t.Error("a silent engine must not own a mixer or a player")
	}
}

func TestEngineWithoutDevice(t *testing.T) {
	e := newEngine(1)
	if e.Silent() {
		t.Fatal("a live engine reports Silent")
	}
	active := func() int {
		renderFrames(e.mx, 32)
		return e.mx.activeVoices()
	}
	e.Play(ExplosionLarge)
	e.Play(ExplosionLarge) // same instant: rate-limited
	e.Play(ExplosionLarge)
	if got := active(); got != 1 {
		t.Errorf("%d voices after three simultaneous identical triggers, want 1", got)
	}
	e.Play(Bomb) // a different effect is not affected
	if got := active(); got != 2 {
		t.Errorf("%d voices, want 2", got)
	}
	// Ignored: silent, negative or NaN volumes and invalid effects.
	e.PlayAt(Coin, 0, 0)
	e.PlayAt(Coin, -1, 0)
	e.PlayAt(Coin, float32(math.NaN()), 0)
	e.PlayAt(Sfx(NumSfx), 1, 0)
	e.PlayAt(Sfx(-1), 1, 0)
	if got := active(); got != 2 {
		t.Errorf("%d voices after ignored triggers, want 2", got)
	}
	// Out-of-range volume and pan are clamped rather than rejected.
	e.PlayAt(Coin, 50, float32(math.NaN()))
	renderFrames(e.mx, 32)
	found := false
	for i := range e.mx.voices {
		if v := &e.mx.voices[i]; v.live && v.sfx == Coin {
			found = true
			if v.gl > 2 || v.gr > 2 || v.gl < 0 || v.gr < 0 || v.gl != v.gl || v.gr != v.gr {
				t.Errorf("clamped trigger has gains %v, %v", v.gl, v.gr)
			}
		}
	}
	if !found {
		t.Error("an over-range but valid trigger was dropped")
	}

	// Moods: invalid ignored, repeats ignored, changes cross-fade.
	e.SetMood(Mood(-1))
	renderFrames(e.mx, 32)
	if len(e.mx.layers) != 0 {
		t.Error("an invalid mood started music")
	}
	e.SetMood(Calm)
	e.SetMood(Calm)
	e.SetMood(Calm)
	renderFrames(e.mx, SampleRate)
	if len(e.mx.layers) != 1 {
		t.Errorf("%d layers after repeated SetMood(Calm), want 1", len(e.mx.layers))
	}
	e.SetMood(Battle)
	renderFrames(e.mx, 32)
	if len(e.mx.layers) != 2 {
		t.Errorf("%d layers after SetMood(Battle), want 2 (cross-fading)", len(e.mx.layers))
	}

	// Volumes are slider positions on a square law, clamped to 0..1.
	for _, tc := range []struct {
		in   [3]float32
		want [3]float64
	}{
		{[3]float32{1, 1, 1}, [3]float64{1, 1, 1}},
		{[3]float32{0.5, 1, 0}, [3]float64{0.25, 1, 0}},
		{[3]float32{2, -1, 0.1}, [3]float64{1, 0, 0.1 * 0.1}},
		{[3]float32{float32(math.NaN()), 0.3, 1}, [3]float64{0, 0.3 * 0.3, 1}},
	} {
		e.SetVolume(tc.in[0], tc.in[1], tc.in[2])
		renderFrames(e.mx, 32)
		for i := range tc.want {
			if math.Abs(e.mx.volTo[i]-tc.want[i]) > 1e-6 {
				t.Errorf("SetVolume(%v): bus %d gain %v, want %v", tc.in, i, e.mx.volTo[i], tc.want[i])
			}
		}
	}
	// Setting the same volumes again does not queue anything.
	e.SetVolume(0.4, 0.4, 0.4)
	renderFrames(e.mx, 32)
	e.SetVolume(0.4, 0.4, 0.4)
	if e.mx.hasVol {
		t.Error("an unchanged SetVolume left a request behind")
	}

	e.Update() // no player attached: nothing to do, must not panic
	e.Close()
	if !e.Silent() {
		t.Error("a closed engine should report Silent")
	}
	e.Play(UIClick)
	e.SetMood(Lament)
	e.SetVolume(1, 1, 1)
	if len(e.mx.pending) != 0 || e.mx.wantMood != nil || e.mx.hasVol {
		t.Error("a closed engine still accepts requests")
	}
}

func TestEngineIsSafeForConcurrentUse(t *testing.T) {
	e := newEngine(7)
	s := &stream{m: e.mx}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // the audio thread
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if n, err := s.Read(buf); n == 0 || err != nil {
				t.Errorf("Read = %d, %v", n, err)
				return
			}
		}
	}()
	for g := 0; g < 4; g++ { // game threads
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 400; i++ {
				e.PlayAt(Sfx((i+g)%NumSfx), 0.8, float32(g-2)/2)
				if i%40 == 0 {
					e.SetMood(Mood((i/40 + g) % NumMoods))
				}
				e.SetVolume(1, float32(i%10)/10, float32(g)/4)
				e.Update()
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(stop)
	wg.Wait()
}
