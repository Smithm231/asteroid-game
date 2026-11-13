package audio

import (
	"sync"
	"sync/atomic"
	"time"

	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
)

// streamBuffer is how much audio the player keeps queued. Triggered effects are heard after
// roughly this long plus the device's own latency, so it is kept short; rendering a buffer
// takes a small fraction of its duration, so there is ample margin against drop-outs.
const streamBuffer = 30 * time.Millisecond

// Engine plays the game's sound effects and music. All methods are safe to call from any
// goroutine and on a nil *Engine (which behaves like a silent one).
//
// An Engine is either live — bound to the Ebitengine audio context, mixing everything into one
// stream — or silent, in which case every method returns immediately and no audio device, PCM
// buffer or goroutine is ever touched.
type Engine struct {
	silent bool
	closed atomic.Bool
	mx     *mixer
	player *ebaudio.Player

	mu      sync.Mutex
	limiter rateLimiter
	start   time.Time
	mood    Mood
	hasMood bool
	seed    uint64
	vol     [3]float32
	hasVol  bool
}

// New renders the sound effects, creates (or reuses) the Ebitengine audio context at
// SampleRate and starts the mix stream. If the context or player cannot be set up — for
// example because another context with a different sample rate already exists — it returns a
// silent engine instead, so callers never need to check for an error.
//
// New must not be used where no audio device can exist (tests, headless runs): Ebitengine
// opens the device once the game loop starts and reports a missing device as a fatal game-loop
// error. Use NewSilent there. No music plays until SetMood is called.
func New() (e *Engine) {
	defer func() {
		if recover() != nil {
			e = NewSilent()
		}
	}()
	ctx := ebaudio.CurrentContext()
	if ctx == nil {
		ctx = ebaudio.NewContext(SampleRate)
	}
	if ctx.SampleRate() != SampleRate {
		return NewSilent()
	}
	e = newEngine(uint64(time.Now().UnixNano()))
	player, err := ctx.NewPlayerF32(&stream{m: e.mx})
	if err != nil {
		return NewSilent()
	}
	player.SetBufferSize(streamBuffer)
	player.Play()
	e.player = player
	return e
}

// NewSilent returns an engine that does nothing at all: no audio context, no synthesis. Use
// it for --mute, automated tests and machines without a sound device.
func NewSilent() *Engine { return &Engine{silent: true} }

// newEngine builds a live engine around a fresh mixer but without an output device; New adds
// the player, and the tests drive the mixer directly.
func newEngine(seed uint64) *Engine {
	return &Engine{mx: newMixer(renderAllSfx()), start: time.Now(), seed: seed}
}

// Silent reports whether the engine is the do-nothing variant: requested through NewSilent,
// returned by New because audio could not be set up, or closed.
func (e *Engine) Silent() bool { return e == nil || e.silent || e.closed.Load() }

// Play triggers a sound effect at full volume in the centre of the stereo field.
func (e *Engine) Play(s Sfx) { e.PlayAt(s, 1, 0) }

// PlayAt triggers a sound effect with a volume (0..1; values up to 2 boost) and a pan position
// (-1 = left, 0 = centre, +1 = right).
//
// Identical effects are rate-limited: a trigger that follows the previous trigger of the same
// effect too closely (40 ms for interface ticks up to seconds for the stingers) is dropped, and
// each effect is limited to a few overlapping instances. At most 16 effects sound at once; when
// that is exceeded the one closest to its end is faded out to make room.
func (e *Engine) PlayAt(s Sfx, vol, pan float32) {
	if e.Silent() || !s.Valid() || !(vol > 0) {
		return
	}
	if vol > 2 {
		vol = 2
	}
	if !(pan >= -1) { // also catches NaN
		pan = -1
	} else if pan > 1 {
		pan = 1
	}
	e.mu.Lock()
	ok := e.limiter.allow(s, time.Since(e.start))
	e.mu.Unlock()
	if ok {
		e.mx.play(s, vol, pan)
	}
}

// SetMood selects the music. The first call fades the music in; later calls cross-fade from
// the current mood to the new one over about three seconds. Calling it repeatedly with the
// mood that is already selected does nothing, so it may be called every frame.
func (e *Engine) SetMood(m Mood) {
	if e.Silent() || !m.Valid() {
		return
	}
	e.mu.Lock()
	if e.hasMood && e.mood == m {
		e.mu.Unlock()
		return
	}
	e.mood, e.hasMood = m, true
	e.seed++
	seed := e.seed
	e.mu.Unlock()
	e.mx.setMood(newMusicGen(m, seed))
}

// SetVolume sets the master, effects and music volumes as slider positions from 0 (off) to 1
// (full); out-of-range values are clamped. Positions are mapped through a square law, so half
// a slider is about a quarter of the amplitude (-12 dB), which is what sounds like "half".
// Changes are smoothed and may be made every frame. With the music (or master) volume at 0
// the music generators pause and cost nothing.
func (e *Engine) SetVolume(master, sfx, music float32) {
	if e.Silent() {
		return
	}
	v := [3]float32{master, sfx, music}
	var gain [3]float64
	for i, x := range v {
		if !(x > 0) {
			x = 0
		} else if x > 1 {
			x = 1
		}
		v[i] = x
		gain[i] = float64(x) * float64(x)
	}
	e.mu.Lock()
	changed := !e.hasVol || e.vol != v
	e.vol, e.hasVol = v, true
	e.mu.Unlock()
	if changed {
		e.mx.setVolume(gain[0], gain[1], gain[2])
	}
}

// Update must be called once per frame from the game's Update. It keeps the output stream
// running (restarting it if the platform paused it); the mixing itself happens on the audio
// thread and needs no per-frame work.
func (e *Engine) Update() {
	if e.Silent() || e.player == nil {
		return
	}
	if !e.player.IsPlaying() {
		e.player.Play()
	}
}

// Close stops the output stream; afterwards the engine behaves like a silent one. Closing is
// optional (exiting the process is enough) and may be done more than once.
func (e *Engine) Close() {
	if e == nil || e.silent || e.closed.Swap(true) {
		return
	}
	if e.player != nil {
		e.player.Pause()
	}
}
