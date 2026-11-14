# DRIFTHOLD — Presentation Specification (graphics, input, UI toolkit, audio) v1

Read `SPEC_GAME.md` (same directory) first; its section 0 ground rules apply here too.
Engine: Ebitengine `github.com/hajimehoshi/ebiten/v2 v2.10.6`. Read its source under
`$(go env GOMODCACHE)/github.com/hajimehoshi/ebiten/v2@v2.10.6` whenever unsure of an API
(`vector`, `text/v2`, `audio`, `inpututil`). Do not use deprecated functions.
Fonts: the Go font family shipped in `golang.org/x/image/font/gofont/*` (already in the module
cache; add the dependency with `go get golang.org/x/image@v0.45.0` if `go.mod` lacks it — this is
the one permitted go.mod edit and it is idempotent).

## 1. Visual identity — "lantern-lit drafting table"

Original look; do not imitate any existing game. Top-down, flat, vector-drawn; no pixel art, no
isometric projection, no bitmaps loaded from disk — **every visual is generated in code**.

* Space: near-black indigo `#0B0D14` with a sparse procedural starfield (three parallax layers,
  a few warm stars) and a faint red-orange glow toward Emberfall.
* Rocks: desaturated slate / umber / frost-blue / glint-violet by class, hex tiles with subtle
  per-tile value noise and a soft rim light from the upper left; blocked terrain drawn as darker
  ridges/cracks; the blob edge has a darker outline and a soft outer shadow.
* Structures: crisp geometric top-down shapes, 2–3 flat tones per material, one light direction
  (upper-left), a faction accent stripe/roof, small animated details (rotating dish, blinking
  beacon, pulsing core) driven by a time parameter. Each building kind must be recognisable by
  silhouette alone at 28 px.
* UI: ink-dark panels `#12151F` / `#1A1E2B` with **chamfered (cut) corners**, hairline rules in
  amber `#E8A33D`, body text cream `#E9E2D0`, dim text `#8C8A80`. Status colours: good `#7BC96F`,
  warn `#E6C84F`, alert `#E5533D`, info `#6FB7E6`. Faction accents: neutral `#8A8F98`, amber
  `#E8A33D`, teal `#3DC9B3`, magenta `#D45CA8`, lime `#A6D94A`.
* Typography: Go Regular / Go Medium / Go Bold for UI, Go Mono for numbers in tables.
  Sizes (points): Small 11, Body 13, Head 16, Title 22, Display 40. Headings are upper-case with
  slight tracking.
* Motion: restrained — 120–200 ms eases on hover/open, no bouncing.

## 2. Coordinates & scaling (high-DPI correct)

* The game renders at **native pixel resolution**: `Layout`/`LayoutF` returns outside size ×
  device scale factor.
* All UI code works in **points**. `Scale` = pixels per point = device scale × user UI scale
  (0.75–2.0). Screen size in points = pixels / Scale. Minimum supported window: 1280×720 points.
* `gfx.Canvas` converts points → pixels at draw time; text faces are instantiated at
  `size × Scale` so text is always crisp. Nothing outside `gfx` should multiply by Scale manually.

## 3. Package `internal/input`

```go
type Button int                       // Left, Right, Middle
type Key = ebiten.Key
type State struct {
    X, Y        float32               // cursor in points
    Wheel       float32               // vertical wheel this frame (+ = away from user)
    Down, Pressed, Released [3]bool   // per Button; Pressed/Released are edges
    DoubleClick bool                  // left double click this frame
    Shift, Ctrl, Alt bool
    Chars       []rune                // text typed this frame
    W, H        float32               // screen size in points
    Scale       float32               // pixels per point
    Frame       int64
    Dt          float32               // seconds since previous frame (1/60 nominal)
    // + key state queried through methods:
}
func (s *State) KeyDown(k Key) bool
func (s *State) KeyPressed(k Key) bool   // edge, with OS-style repeat available via KeyRepeated
func (s *State) KeyRepeated(k Key) bool
type Source interface{ Poll(pixelW, pixelH int, scale float32) State }
func NewDevice() Source                  // real mouse/keyboard via ebiten + inpututil
```

Scripted playback for automated UI tests: `Script` is a JSON list of steps executed in order, e.g.
`{"wait":30}`, `{"move":[640,360]}`, `{"click":[100,200]}`, `{"click":[100,200],"button":"right"}`,
`{"dblclick":[x,y]}`, `{"drag":[x0,y0,x1,y1],"frames":20}`, `{"wheel":-3}`, `{"key":"R"}`,
`{"key":"Escape"}`, `{"hold":"ShiftLeft","frames":10}`, `{"text":"hello"}`,
`{"shot":"name.png"}`, `{"quit":true}`. Coordinates are in points. `NewScript(steps) *ScriptSource`
implements `Source` and exposes `TakeShot() (name string, ok bool)` and `Done() bool` for the host.
A click is press on one frame, release on the next, with the cursor moved there one frame earlier.

## 4. Package `internal/uitest` (owned by the input task)

Tiny host for visual tests, used by every UI package's demo programs:

```go
type Options struct {
    W, H   int        // window size in points (default 1600×900)
    Scale  float32    // pixels per point (default 1)
    Script []input.Step
    OutDir string     // where {"shot": ...} PNGs go
    Frames int        // optional hard stop
}
// Run opens a window, feeds scripted input, calls update then draw each frame, writes
// screenshots, and returns when the script quits. Must be called from main().
func Run(o Options, update func(in input.State), draw func(dst *ebiten.Image)) error
```
Demos are run as `xvfb-run -a -s "-screen 0 1920x1200x24" go run ./internal/.../demo -out /tmp/x`
and the PNGs inspected with the image viewer (Read tool). A no-audio environment is assumed.

## 5. Package `internal/gfx`

* `Rect{X,Y,W,H float32}` in points with rect-cut layout helpers: `CutTop/CutBottom/CutLeft/
  CutRight(n)` (mutating, returning the slice cut off), `Inset`, `Pad`, `Center(w,h)`, `Contains`,
  `SplitH(n)`/`SplitV(n)`, `Grid(cols, rows, gap)`.
* `Theme` with the palette, metrics (Pad 8, Gap 6, RowH 26, BarH 44, Chamfer 6) and font sizes
  from section 1; `FactionColor(i)`, `GoodColor(good index)`, `SeverityColor`.
  `Default()` returns it. Colour helpers: `Lerp`, `WithAlpha`, `Lighten/Darken`.
* `Canvas{Dst *ebiten.Image; Scale float32}` — all arguments in points:
  `FillRect, StrokeRect, FillChamfer, StrokeChamfer (cut-corner panel), Line, Polyline, FillPoly,
  StrokePoly, FillCircle, StrokeCircle, Arc, FillHex/StrokeHex (pointy-top), Glow (soft radial
  additive blob), DrawImage(img, dstRect, opts{Alpha, Tint, Rotation, Additive}), Text(str, x, y,
  TextStyle{Size, Weight, Mono, Color, Align, Tracking, Upper}), MeasureText, WrapText(width),
  PushClip(rect)/PopClip, Sub(rect)`. Anti-aliased. Efficient enough for ~3,000 primitives/frame.
* Text via `text/v2` + Go fonts, face cache keyed by (weight, mono, pixel size).
* `Backdrop`: starfield + Emberfall glow generator (seeded), drawn with parallax offset.
* (Later tasks add procedural art for rocks, buildings, ships and icons on top of this.)
* A demo `internal/gfx/demo` renders a contact sheet of every primitive and text style.

## 6. Package `internal/ui/kit` — immediate-mode toolkit

Widgets are declared during the game's Update (so input edges are exact) and recorded as draw
commands; `Draw` replays them. All geometry in points.

```go
ctx := kit.New(theme)
ctx.Begin(in input.State)        // each Update
... declare widgets ...
ctx.End()
ctx.Draw(screen)                 // each Draw
```
Required features:
* Layers: Base < Panels < Windows (z-ordered, click to raise) < Popups/Dropdowns < Tooltips.
  Input goes only to the top-most layer under the cursor. `ctx.WantMouse()` / `ctx.WantKeys()`
  tell world views to ignore input that the UI owns (computed from the previous frame's hit
  regions plus current captures).
* IDs: string ids hashed with an id stack (`PushID/PopID`).
* Widgets (explicit-rect API): `Panel`, `Label` (styles, alignment, ellipsis), `Button`,
  `IconButton` (icon drawn by a callback), `Toggle`, `Checkbox`, `Radio`, `Slider`, `Stepper`,
  `Progress` (with overlay label), `TextInput` (caret, selection optional, max length, filters),
  `Dropdown`, `Tabs`, `Separator`, `Badge`, `KeyHint`, `List`/table rows with selection and
  alternating shading, `BeginScroll/EndScroll` (wheel + draggable thumb, clipped),
  `BeginWindow/EndWindow` (title bar, drag to move, close button, optional modal dim; keeps
  position; clamps to screen), `Tooltip` for the last widget (plain text or rich: title, body
  lines, key/value rows) with a short hover delay, `Toast` stack helper, `Canvas(rect, func(cv
  *gfx.Canvas, r gfx.Rect))` for deferred custom drawing within the current layer, `Hover(rect)`,
  `Pressed/Clicked(id, rect)` low-level hit helpers, drag helpers reporting deltas.
* Keyboard: focus for text inputs, Tab traversal optional, Esc closes the top window (reported to
  caller), per-button hotkey support (`Button(..., kit.Hotkey(key))`) shown as a key hint.
* Sound hooks: `ctx.OnSound func(kit.Sound)` invoked for Hover/Click/Open/Close/Error so the app
  can route them to the audio engine (the kit never imports audio).
* Disabled state for every interactive widget, with a tooltip explaining why when provided.
* A demo `internal/ui/kit/demo` showing every widget; verified by scripted screenshots.

## 7. Package `internal/audio` — everything synthesised, nothing loaded

```go
func New() *Engine           // creates the Ebitengine audio context (44100 Hz); on failure returns a silent engine
func NewSilent() *Engine     // never touches the audio device (used by tests / --mute)
func (e *Engine) Play(s Sfx)
func (e *Engine) PlayAt(s Sfx, vol, pan float32)
func (e *Engine) SetMood(m Mood)              // Menu, Calm, Tense, Battle, Triumph, Lament — cross-fades
func (e *Engine) SetVolume(master, sfx, music float32)
func (e *Engine) Update()                     // call once per frame
```
* SFX are rendered once at start-up from small synthesis recipes (oscillators, FM, filtered noise,
  envelopes, simple delay/reverb) into PCM buffers. Required set: UIHover, UIClick, UIOpen,
  UIClose, UIError, Place, BuildDone, Demolish, ExplosionSmall, ExplosionLarge, Flak, Lance,
  ShipShot, Bomb, ShieldHit, Launch, Arrive, Alert, Notice, Good, TechDone, DirectiveDone, Coin,
  SlugFire, SlugImpact, Meteor, FlareStart, ColonyFounded, ColonyFell, Victory, Defeat.
  Rate-limit identical sounds (no machine-gun stacking), cap simultaneous voices.
* Music is a real-time generative stream: slow evolving pads, sparse plucked motifs in a modal
  scale, soft sub-bass, gentle noise "wind"; per-mood parameters (tempo, density, mode, brightness).
  It must be original, calm, and never clip; overall loudness consistent between moods.
* Offline entry points for tests: `RenderSfx(s) []float32` and
  `RenderMusic(m Mood, seconds float64, seed uint64) []float32` (mono or interleaved stereo,
  documented). Tests assert: no NaN/Inf, peak ≤ 1.0, RMS within a sane band, non-silent, SFX
  durations in range, deterministic output for a given seed, no DC offset, no clicks at loop/voice
  boundaries (discontinuity check). A tiny tool `internal/audio/wavdump` writes WAVs for manual
  inspection.
