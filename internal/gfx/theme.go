package gfx

// Theme holds the visual constants of the game: the palette, layout metrics
// and font sizes of the "lantern-lit drafting table" look — ink-dark
// chamfered panels, amber hairlines, cream text.
//
// All lengths are in points. A Theme is plain data; copy and adjust it
// freely.
type Theme struct {
	// Surfaces.
	Space   Color // backdrop behind everything, near-black indigo
	Well    Color // recessed areas: input fields, track grooves, list wells
	Panel   Color // panel body
	PanelHi Color // raised surfaces: title bars, buttons, alternate rows

	// Lines.
	Rule    Color // amber hairline, the signature accent
	RuleDim Color // subdued amber for secondary frames
	Border  Color // neutral hairline for quiet separators

	// Text.
	Text      Color // body text, cream
	TextDim   Color // secondary text
	TextFaint Color // disabled text
	Ink       Color // dark text on bright (amber, status) fills

	// Status.
	Good  Color
	Warn  Color
	Alert Color
	Info  Color

	// Translucent overlays, meant to be drawn over a surface.
	Hover  Color // pointer is over an interactive surface
	Press  Color // surface is held down
	Select Color // selected row or item
	Shadow Color // drop shadow under floating windows
	Scrim  Color // dims everything behind a modal window

	// Factions holds the accent colour per faction index: 0 neutral grey,
	// 1 amber (player), 2 teal, 3 magenta, 4 lime.
	Factions [5]Color
	// Goods holds a colour per good in enum order: Ore, Ice, Silica, Lumen,
	// Alloy, Fuel, Parts, Food.
	Goods [8]Color
	// Rocks holds the base surface colour per rock class in enum order:
	// Ferric (umber), Stony (slate), Frost (frost blue), Glint (violet).
	Rocks [4]Color

	// Metrics.
	Pad      float32 // inner padding of panels
	Gap      float32 // spacing between sibling widgets
	RowH     float32 // height of a list row, button or input
	BarH     float32 // height of the top bar and other chrome bars
	Chamfer  float32 // length cut off each panel corner
	Hairline float32 // thickness of rules and frames

	// Font sizes.
	SizeSmall   float32
	SizeBody    float32
	SizeHead    float32
	SizeTitle   float32
	SizeDisplay float32
}

// Default returns the standard Drifthold theme. Each call returns a fresh
// copy that the caller may modify.
func Default() *Theme {
	panel := RGB(0x12151F)
	rule := RGB(0xE8A33D)
	return &Theme{
		Space:   RGB(0x0B0D14),
		Well:    RGB(0x0D1018),
		Panel:   panel,
		PanelHi: RGB(0x1A1E2B),

		Rule:    rule,
		RuleDim: Lerp(panel, rule, 0.38),
		Border:  RGB(0x2A3042),

		Text:      RGB(0xE9E2D0),
		TextDim:   RGB(0x8C8A80),
		TextFaint: RGB(0x5B5B5A),
		Ink:       panel,

		Good:  RGB(0x7BC96F),
		Warn:  RGB(0xE6C84F),
		Alert: RGB(0xE5533D),
		Info:  RGB(0x6FB7E6),

		Hover:  Color{0xFF, 0xF4, 0xE0, 0x14},
		Press:  Color{0x00, 0x00, 0x00, 0x38},
		Select: WithAlpha(rule, 0.20),
		Shadow: Color{0x02, 0x03, 0x06, 0xA0},
		Scrim:  Color{0x05, 0x06, 0x0A, 0xB4},

		Factions: [5]Color{
			RGB(0x8A8F98), RGB(0xE8A33D), RGB(0x3DC9B3), RGB(0xD45CA8), RGB(0xA6D94A),
		},
		Goods: [8]Color{
			RGB(0xC0714F), // Ore: rust
			RGB(0x9FD8EE), // Ice: pale cyan
			RGB(0xD9C89A), // Silica: sand
			RGB(0xB690FF), // Lumen: violet
			RGB(0xAEB8C6), // Alloy: steel
			RGB(0xEE7B45), // Fuel: flame orange
			RGB(0x7D95C9), // Parts: gunmetal blue
			RGB(0x9BC56A), // Food: leaf green
		},
		Rocks: [4]Color{
			RGB(0x6E5A4B), // Ferric: umber
			RGB(0x5C6470), // Stony: slate
			RGB(0x7F97AD), // Frost: frost blue
			RGB(0x75699A), // Glint: violet
		},

		Pad:      8,
		Gap:      6,
		RowH:     26,
		BarH:     44,
		Chamfer:  6,
		Hairline: 1,

		SizeSmall:   11,
		SizeBody:    13,
		SizeHead:    16,
		SizeTitle:   22,
		SizeDisplay: 40,
	}
}

// FactionColor returns the accent colour of faction index i. Indices outside
// the table return the neutral grey of index 0.
func (t *Theme) FactionColor(i int) Color {
	if i < 0 || i >= len(t.Factions) {
		return t.Factions[0]
	}
	return t.Factions[i]
}

// GoodColor returns the colour of the good with enum index i (Ore, Ice,
// Silica, Lumen, Alloy, Fuel, Parts, Food). Other indices return TextDim.
func (t *Theme) GoodColor(i int) Color {
	if i < 0 || i >= len(t.Goods) {
		return t.TextDim
	}
	return t.Goods[i]
}

// SeverityColor returns the status colour of a log severity in enum order:
// 0 Info, 1 Good, 2 Warn, 3 Alert. Other values return Text.
func (t *Theme) SeverityColor(sev int) Color {
	switch sev {
	case 0:
		return t.Info
	case 1:
		return t.Good
	case 2:
		return t.Warn
	case 3:
		return t.Alert
	}
	return t.Text
}

// RockColor returns the base surface colour of a rock class in enum order:
// 0 Ferric, 1 Stony, 2 Frost, 3 Glint. Other values return the Stony slate.
func (t *Theme) RockColor(class int) Color {
	if class < 0 || class >= len(t.Rocks) {
		return t.Rocks[1]
	}
	return t.Rocks[class]
}

// Small returns the style of small print: SizeSmall, regular, cream.
func (t *Theme) Small() TextStyle {
	return TextStyle{Size: t.SizeSmall, Color: t.Text}
}

// Body returns the style of running text: SizeBody, regular, cream.
func (t *Theme) Body() TextStyle {
	return TextStyle{Size: t.SizeBody, Color: t.Text}
}

// Label returns the style of field labels and captions: SizeSmall, medium,
// upper-case, tracked, dim.
func (t *Theme) Label() TextStyle {
	return TextStyle{
		Size: t.SizeSmall, Weight: Medium, Color: t.TextDim, Upper: true, Tracking: 0.8,
	}
}

// Head returns the style of section headings: SizeHead, medium, upper-case
// with slight tracking.
func (t *Theme) Head() TextStyle {
	return TextStyle{
		Size: t.SizeHead, Weight: Medium, Color: t.Text, Upper: true, Tracking: 1,
	}
}

// Title returns the style of window and screen titles: SizeTitle, bold,
// upper-case, tracked.
func (t *Theme) Title() TextStyle {
	return TextStyle{
		Size: t.SizeTitle, Weight: Bold, Color: t.Text, Upper: true, Tracking: 1.5,
	}
}

// Display returns the style of the largest lettering (the game title,
// victory banners): SizeDisplay, bold, upper-case, widely tracked.
func (t *Theme) Display() TextStyle {
	return TextStyle{
		Size: t.SizeDisplay, Weight: Bold, Color: t.Text, Upper: true, Tracking: 4,
	}
}

// Num returns the style of numbers in tables: SizeBody, monospaced, cream.
// Right-align it with [TextStyle.Aligned] so that digits line up.
func (t *Theme) Num() TextStyle {
	return TextStyle{Size: t.SizeBody, Mono: true, Color: t.Text}
}

// NumSmall is Num at SizeSmall.
func (t *Theme) NumSmall() TextStyle {
	return TextStyle{Size: t.SizeSmall, Mono: true, Color: t.Text}
}
