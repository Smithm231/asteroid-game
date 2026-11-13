package gfx

import (
	"fmt"
	"image/color"
	"math"
	"strconv"
	"strings"
)

// Color is the colour type of this package: 8-bit sRGB with straight
// (non-premultiplied) alpha, laid out like [color.NRGBA] and convertible to
// and from it. It is a plain value, so passing it around never allocates,
// and it implements [color.Color] for use with Ebitengine and the standard
// library.
//
// The zero Color (transparent black) means "not set" wherever a colour is
// optional, such as [TextStyle].Color or [ImageOpts].Tint.
type Color struct{ R, G, B, A uint8 }

// RGBA implements [color.Color]: it returns the alpha-premultiplied
// components scaled to 0..0xFFFF.
func (c Color) RGBA() (r, g, b, a uint32) { return color.NRGBA(c).RGBA() }

// Basic colours.
var (
	White       = Color{0xFF, 0xFF, 0xFF, 0xFF}
	Black       = Color{0x00, 0x00, 0x00, 0xFF}
	Transparent = Color{}
)

// RGB returns the opaque colour 0xRRGGBB.
func RGB(hex uint32) Color {
	return Color{uint8(hex >> 16), uint8(hex >> 8), uint8(hex), 0xFF}
}

// RGBA returns the colour 0xRRGGBBAA with straight alpha.
func RGBA(hex uint32) Color {
	return Color{uint8(hex >> 24), uint8(hex >> 16), uint8(hex >> 8), uint8(hex)}
}

// ParseHex parses "#RRGGBB", "#RRGGBBAA" or the same without the leading
// '#'.
func ParseHex(s string) (Color, error) {
	h := strings.TrimPrefix(s, "#")
	if len(h) != 6 && len(h) != 8 {
		return Color{}, fmt.Errorf("gfx: colour %q: want #RRGGBB or #RRGGBBAA", s)
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return Color{}, fmt.Errorf("gfx: colour %q: not hexadecimal", s)
	}
	if len(h) == 6 {
		return RGB(uint32(v)), nil
	}
	return RGBA(uint32(v)), nil
}

// Gray returns the opaque grey of the given level.
func Gray(v uint8) Color { return Color{v, v, v, 0xFF} }

// WithAlpha returns c with its opacity replaced by a (0 transparent .. 1
// opaque).
func WithAlpha(c Color, a float32) Color {
	c.A = unit8(a)
	return c
}

// Fade returns c with its opacity multiplied by f (0..1).
func Fade(c Color, f float32) Color {
	c.A = unit8(float32(c.A) / 255 * clampf(f, 0, 1))
	return c
}

// Opacity returns white at opacity a. It is the tint that draws an image
// unchanged but translucent.
func Opacity(a float32) Color { return Color{0xFF, 0xFF, 0xFF, unit8(a)} }

// Lerp blends from a (t = 0) to b (t = 1); t is clamped to 0..1. The blend
// is done on premultiplied values, so fading towards a transparent colour
// does not drag the hue towards that colour's hidden RGB.
func Lerp(a, b Color, t float32) Color {
	t = clampf(t, 0, 1)
	if t == 0 {
		return a
	}
	if t == 1 {
		return b
	}
	ar, ag, ab, aa := premul(a)
	br, bg, bb, ba := premul(b)
	r := ar + (br-ar)*t
	g := ag + (bg-ag)*t
	bl := ab + (bb-ab)*t
	al := aa + (ba-aa)*t
	if al <= 0 {
		return Color{}
	}
	return Color{unit8(r / al), unit8(g / al), unit8(bl / al), unit8(al)}
}

// Lighten moves the colour towards white by amt (0 unchanged .. 1 white),
// keeping its alpha.
func Lighten(c Color, amt float32) Color {
	amt = clampf(amt, 0, 1)
	return Color{
		lerp8(c.R, 0xFF, amt), lerp8(c.G, 0xFF, amt), lerp8(c.B, 0xFF, amt), c.A,
	}
}

// Darken moves the colour towards black by amt (0 unchanged .. 1 black),
// keeping its alpha.
func Darken(c Color, amt float32) Color {
	amt = clampf(amt, 0, 1)
	return Color{lerp8(c.R, 0, amt), lerp8(c.G, 0, amt), lerp8(c.B, 0, amt), c.A}
}

// Luma returns the perceived brightness of the colour's RGB, 0 (black) to 1
// (white), ignoring alpha. Text on a fill reads best in a dark ink when the
// fill's Luma is above about 0.5.
func Luma(c Color) float32 {
	return (0.2126*float32(c.R) + 0.7152*float32(c.G) + 0.0722*float32(c.B)) / 255
}

// premul returns the colour as premultiplied-alpha components in 0..1, the
// form Ebitengine's vertices and colour scales take.
func premul(c Color) (r, g, b, a float32) {
	a = float32(c.A) * (1.0 / 255)
	k := a * (1.0 / 255)
	return float32(c.R) * k, float32(c.G) * k, float32(c.B) * k, a
}

// unit8 converts 0..1 to 0..255 with rounding and clamping.
func unit8(v float32) uint8 {
	if !(v > 0) { // also catches NaN
		return 0
	}
	if v >= 1 {
		return 0xFF
	}
	return uint8(math.Round(float64(v) * 255))
}

func lerp8(a, b uint8, t float32) uint8 {
	return uint8(math.Round(float64(float32(a) + (float32(b)-float32(a))*t)))
}
