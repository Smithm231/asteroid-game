package audio

import (
	"encoding/binary"
	"math"
	"sync"
	"time"
)

// The mixer is the real-time heart of the engine and is still plain Go: it owns the playing
// sound-effect voices and the music layers, applies the volumes and the limiter, and exposes
// the result as a PCM byte stream. engine.go merely hands that stream to Ebitengine.
//
// Threading: the game thread only ever leaves small requests behind a brief lock — effect
// triggers in a queue, the latest volumes and the latest mood in single slots. The audio thread
// collects them at the start of each render and then works without holding that lock, so
// neither side can stall the other.

const (
	maxVoices    = 16   // sound effects audible at once
	voiceSlots   = 24   // maxVoices plus room for voices fading out after being replaced
	stealFade    = 220  // frames (5 ms) a replaced voice takes to fade out
	moodFade     = 3.0  // seconds of a mood cross-fade
	maxLayers    = 3    // music generators alive at once (current plus ones still fading out)
	maxPending   = 256  // queued effect triggers before new ones are dropped
	mixBlock     = 512  // frames mixed per inner block
	volumeGlide  = 512  // frames a volume takes to travel from 0 to 1
	volumeSilent = 1e-5 // below this a volume is treated as off
)

type sfxVoice struct {
	live   bool
	sfx    Sfx
	buf    []float32 // interleaved stereo source
	pos    int       // index of the next sample in buf
	gl, gr float32
	fade   int    // > 0: frames left of a fade-out; the voice no longer counts as active
	seq    uint64 // start order, to find the oldest instance
}

type musicLayer struct {
	gen    *musicGen
	pos    float64 // fade position, 0 (silent) .. 1 (full)
	target float64
	rate   float64 // fade speed in positions per frame
}

// trigger is a queued request to start an effect.
type trigger struct {
	sfx    Sfx
	gl, gr float32
}

type mixer struct {
	sfx [NumSfx][]float32

	mu       sync.Mutex // guards the requests from the game side
	pending  []trigger
	wantVol  [3]float64
	hasVol   bool
	wantMood *musicGen

	rmu    sync.Mutex // serialises render; everything below belongs to the rendering side
	spare  []trigger
	voices [voiceSlots]sfxVoice
	seq    uint64
	layers []*musicLayer
	lim    *limiter
	vol    [3]float64 // current master, effects and music gains
	volTo  [3]float64
	sfxBuf []float32
	musBuf []float32
	tmp    []float32
}

// newMixer creates a mixer playing the given rendered effects, with all volumes at full.
func newMixer(sfx [NumSfx][]float32) *mixer {
	return &mixer{
		sfx:    sfx,
		lim:    newLimiter(),
		vol:    [3]float64{1, 1, 1},
		volTo:  [3]float64{1, 1, 1},
		sfxBuf: make([]float32, 2*mixBlock),
		musBuf: make([]float32, 2*mixBlock),
		tmp:    make([]float32, 2*mixBlock),
	}
}

// panGains converts a volume and a pan position (-1 left .. +1 right) to channel gains. The
// centre is unity on both sides and panning only attenuates the far side, so a panned sound is
// never louder than a centred one.
func panGains(vol, pan float32) (gl, gr float32) {
	p := clamp(float64(pan), -1, 1)
	gl = vol * float32(math.Sqrt(math.Min(1, 1-p)))
	gr = vol * float32(math.Sqrt(math.Min(1, 1+p)))
	return gl, gr
}

// play queues a sound effect. If rendering has stalled and the queue is full the trigger is
// dropped, so the queue stays bounded.
func (m *mixer) play(s Sfx, vol, pan float32) {
	if !s.Valid() {
		return
	}
	gl, gr := panGains(vol, pan)
	m.mu.Lock()
	if len(m.pending) < maxPending {
		m.pending = append(m.pending, trigger{sfx: s, gl: gl, gr: gr})
	}
	m.mu.Unlock()
}

// setMood requests a cross-fade to the mood of the given (fresh) generator. Only the latest
// request made between two renders is honoured.
func (m *mixer) setMood(g *musicGen) {
	if g == nil {
		return
	}
	m.mu.Lock()
	m.wantMood = g
	m.mu.Unlock()
}

// setVolume requests new linear gains for the master, effects and music buses.
func (m *mixer) setVolume(master, sfx, music float64) {
	m.mu.Lock()
	m.wantVol, m.hasVol = [3]float64{master, sfx, music}, true
	m.mu.Unlock()
}

// drain applies the requests made since the last render; called on the rendering side.
func (m *mixer) drain() {
	m.mu.Lock()
	triggers := m.pending
	m.pending = m.spare[:0]
	mood := m.wantMood
	m.wantMood = nil
	if m.hasVol {
		m.volTo, m.hasVol = m.wantVol, false
	}
	m.mu.Unlock()
	if mood != nil {
		m.startMood(mood)
	}
	for _, t := range triggers {
		m.startVoice(t.sfx, t.gl, t.gr)
	}
	m.spare = triggers[:0]
}

// activeVoices counts the voices that are sounding and not being faded out.
func (m *mixer) activeVoices() int {
	n := 0
	for i := range m.voices {
		if m.voices[i].live && m.voices[i].fade == 0 {
			n++
		}
	}
	return n
}

func (m *mixer) startVoice(s Sfx, gl, gr float32) {
	buf := m.sfx[s]
	if len(buf) < 2 {
		return
	}
	// Keep each effect to its own small number of instances: the oldest makes way.
	same, oldest := 0, -1
	for i := range m.voices {
		v := &m.voices[i]
		if v.live && v.fade == 0 && v.sfx == s {
			same++
			if oldest < 0 || v.seq < m.voices[oldest].seq {
				oldest = i
			}
		}
	}
	if same >= sfxDefs[s].maxInst && oldest >= 0 {
		m.voices[oldest].fade = stealFade
	}
	// Keep the whole mix to maxVoices: the voice nearest its end makes way.
	if m.activeVoices() >= maxVoices {
		victim, most := -1, -1.0
		for i := range m.voices {
			v := &m.voices[i]
			if !v.live || v.fade != 0 {
				continue
			}
			if done := float64(v.pos) / float64(len(v.buf)); done > most {
				victim, most = i, done
			}
		}
		if victim >= 0 {
			m.voices[victim].fade = stealFade
		}
	}
	for i := range m.voices {
		if !m.voices[i].live {
			m.seq++
			m.voices[i] = sfxVoice{live: true, sfx: s, buf: buf, gl: gl, gr: gr, seq: m.seq}
			return
		}
	}
	// No free slot (a burst replaced more than eight voices within 5 ms): drop the sound.
}

func (m *mixer) startMood(g *musicGen) {
	rate := 1 / (moodFade * sr)
	var reuse *musicLayer
	for _, l := range m.layers {
		if l.gen.mood == g.mood {
			if l.target == 1 {
				return // already playing or fading in
			}
			reuse = l
		}
	}
	for _, l := range m.layers {
		l.target, l.rate = 0, rate
	}
	if reuse != nil {
		reuse.target = 1 // the mood was still fading out: bring it back rather than restart it
	} else {
		m.layers = append(m.layers, &musicLayer{gen: g, target: 1, rate: rate})
	}
	// Under rapid mood changes, hurry the oldest fading layers out to bound the work.
	for extra := len(m.layers) - maxLayers; extra > 0; extra-- {
		for _, l := range m.layers {
			if l.target == 0 && l.rate == rate {
				l.rate = 1 / (0.05 * sr)
				break
			}
		}
	}
}

// render fills dst (interleaved stereo float32) with the next len(dst)/2 frames of the mix.
func (m *mixer) render(dst []float32) {
	m.rmu.Lock()
	defer m.rmu.Unlock()
	m.drain()
	for len(dst) >= 2 {
		n := min(len(dst)/2, mixBlock)
		m.block(dst[:2*n])
		dst = dst[2*n:]
	}
}

func (m *mixer) block(dst []float32) {
	n := len(dst) / 2
	sfxBuf, musBuf, tmp := m.sfxBuf[:2*n], m.musBuf[:2*n], m.tmp[:2*n]
	clear(sfxBuf)
	clear(musBuf)

	for i := range m.voices {
		v := &m.voices[i]
		if !v.live {
			continue
		}
		for f := 0; f < n; f++ {
			if v.pos+1 >= len(v.buf) {
				v.live = false
				break
			}
			gl, gr := v.gl, v.gr
			if v.fade > 0 {
				k := float32(v.fade) / stealFade
				gl *= k
				gr *= k
				if v.fade--; v.fade == 0 {
					v.live = false
				}
			}
			sfxBuf[2*f] += v.buf[v.pos] * gl
			sfxBuf[2*f+1] += v.buf[v.pos+1] * gr
			v.pos += 2
			if !v.live {
				break
			}
		}
	}

	// With the music turned off the generators are paused rather than rendered and discarded.
	musicOn := m.vol[0]*m.vol[2] > volumeSilent || m.volTo[0]*m.volTo[2] > volumeSilent
	kept := m.layers[:0]
	for _, l := range m.layers {
		if musicOn {
			l.gen.render(tmp)
		}
		for f := 0; f < n; f++ {
			if l.pos < l.target {
				l.pos = math.Min(l.target, l.pos+l.rate)
			} else if l.pos > l.target {
				l.pos = math.Max(l.target, l.pos-l.rate)
			}
			if musicOn {
				g := float32(sinT(l.pos / 4)) // equal-power fade: sin(pos·π/2)
				musBuf[2*f] += tmp[2*f] * g
				musBuf[2*f+1] += tmp[2*f+1] * g
			}
		}
		if l.pos > 0 || l.target > 0 {
			kept = append(kept, l)
		}
	}
	for i := len(kept); i < len(m.layers); i++ {
		m.layers[i] = nil
	}
	m.layers = kept

	// Volumes glide to their targets at a bounded rate (a full-scale change takes volumeGlide
	// frames), so slider moves never zipper.
	for f := 0; f < n; f++ {
		if m.vol != m.volTo {
			for i := range m.vol {
				m.vol[i] += clamp(m.volTo[i]-m.vol[i], -1.0/volumeGlide, 1.0/volumeGlide)
			}
		}
		l := (float64(sfxBuf[2*f])*m.vol[1] + float64(musBuf[2*f])*m.vol[2]) * m.vol[0]
		r := (float64(sfxBuf[2*f+1])*m.vol[1] + float64(musBuf[2*f+1])*m.vol[2]) * m.vol[0]
		if math.IsNaN(l+r) || math.IsInf(l+r, 0) {
			l, r = 0, 0
		}
		l, r = m.lim.tick(l, r)
		dst[2*f] = float32(l)
		dst[2*f+1] = float32(r)
	}
}

// stream adapts a mixer to io.Reader: an endless stream of 32-bit float little-endian stereo
// PCM, the format Ebitengine's NewPlayerF32 expects.
type stream struct {
	m     *mixer
	f32   []float32
	carry []byte
}

const frameBytes = 8 // two float32 samples

// Read renders as many whole frames as fit in p. It never returns an error or io.EOF.
func (s *stream) Read(p []byte) (int, error) {
	n := 0
	if len(s.carry) > 0 {
		c := copy(p, s.carry)
		s.carry = s.carry[c:]
		n += c
		p = p[c:]
	}
	if frames := len(p) / frameBytes; frames > 0 {
		if cap(s.f32) < 2*frames {
			s.f32 = make([]float32, 2*frames)
		}
		buf := s.f32[:2*frames]
		s.m.render(buf)
		for i, v := range buf {
			binary.LittleEndian.PutUint32(p[4*i:], math.Float32bits(v))
		}
		n += frames * frameBytes
		p = p[frames*frameBytes:]
	}
	if n == 0 && len(p) > 0 {
		// The caller asked for less than one frame: render one and keep the rest for later.
		var one [2]float32
		s.m.render(one[:])
		var raw [frameBytes]byte
		binary.LittleEndian.PutUint32(raw[0:], math.Float32bits(one[0]))
		binary.LittleEndian.PutUint32(raw[4:], math.Float32bits(one[1]))
		n = copy(p, raw[:])
		s.carry = append(s.carry[:0], raw[n:]...)
	}
	return n, nil
}

// rateLimiter refuses triggers of an effect that follow the previous one too closely, so a
// burst of identical events becomes one sound instead of a machine-gun stack.
type rateLimiter struct {
	last [NumSfx]time.Duration
	used [NumSfx]bool
}

// allow reports whether effect s may start at time now (measured on any monotonic clock) and,
// if so, records the trigger.
func (l *rateLimiter) allow(s Sfx, now time.Duration) bool {
	if !s.Valid() {
		return false
	}
	gap := time.Duration(sfxDefs[s].minGap * float64(time.Second))
	if l.used[s] && now-l.last[s] < gap {
		return false
	}
	l.last[s], l.used[s] = now, true
	return true
}
