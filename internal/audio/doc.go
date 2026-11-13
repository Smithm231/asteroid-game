// Package audio is Drifthold's procedural sound system. Nothing is loaded from disk: every
// sound effect is rendered at start-up from a small synthesis recipe, and the music is a
// real-time generative stream.
//
// # Layout
//
// The synthesis core is plain Go with no Ebitengine dependency, so it can be rendered offline:
//
//   - dsp.go, reverb.go, analysis.go — oscillators, envelopes, filters, delay lines, a feedback
//     delay network reverb, a look-ahead limiter and measurement helpers.
//   - sfx.go, sfx_recipes.go — the [Sfx] set and its recipes ([RenderSfx]).
//   - music.go, music_harmony.go, music_moods.go — the generative music engine ([RenderMusic]).
//   - mixer.go — voice management, volumes, mood cross-fades and the PCM stream.
//   - engine.go — the only file that touches Ebitengine: [New] binds the mixer to an
//     audio.Context and a single streaming player.
//
// # Formats
//
// All audio is 44100 Hz ([SampleRate]). Every buffer returned by [RenderSfx] and [RenderMusic]
// is interleaved stereo float32 (L, R, L, R, ...) in the range -1..1, so a buffer of n frames
// has length 2n.
//
// # Use
//
//	eng := audio.New()        // or audio.NewSilent() for --mute, tests and headless runs
//	eng.SetMood(audio.Menu)
//	...
//	eng.Play(audio.UIClick)   // from anywhere, any goroutine
//	eng.Update()              // once per frame
package audio
