package gfx

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
)

// glowShaderSrc shades a quad as a soft radial blob. The quad's corners
// carry (±1, ±1) in the first two custom vertex attributes, so the length of
// the interpolated pair is the distance from the centre in radii; the vertex
// colour is the premultiplied colour at the centre. The third attribute
// tightens the falloff and the fourth scales a half-LSB noise that keeps
// large faint glows from showing 8-bit bands. Everything arrives through
// vertex attributes — no uniforms — so consecutive glows share a draw call.
const glowShaderSrc = `//kage:unit pixels

package main

func Fragment(dstPos vec4, srcPos vec2, color vec4, custom vec4) vec4 {
	t := clamp(1-length(custom.xy), 0, 1)
	f := t * t * (3 - 2*t)
	f = mix(f, f*f, custom.z)
	n := fract(sin(dot(floor(dstPos.xy), vec2(12.9898, 78.233))) * 43758.5453)
	d := (n - 0.5) / 255 * custom.w * step(0.001, f)
	return max(color*f+vec4(d), vec4(0))
}
`

var (
	glowShader     *ebiten.Shader
	glowOptsAdd    = ebiten.DrawTrianglesShaderOptions{Blend: ebiten.BlendLighter}
	glowOptsNormal ebiten.DrawTrianglesShaderOptions
	glowVerts      [4]ebiten.Vertex
	glowIndices    = [6]uint32{0, 1, 2, 0, 2, 3}
)

// ensureGlowShader compiles the glow shader on first use.
func ensureGlowShader() *ebiten.Shader {
	if glowShader == nil {
		s, err := ebiten.NewShader([]byte(glowShaderSrc))
		if err != nil {
			// The source is a constant; failing to compile it is a
			// programming error, like a bad regular expression.
			panic(fmt.Sprintf("gfx: compiling the glow shader: %v", err))
		}
		glowShader = s
	}
	return glowShader
}

// Glow adds a soft round blob of light centred on (cx, cy): col at the
// centre, fading smoothly to nothing at radius r. The light is added to
// what is already drawn, so overlapping glows build up and black adds
// nothing; col's alpha scales the intensity.
func (c *Canvas) Glow(cx, cy, r float32, col Color) {
	c.blob(cx, cy, r, r, col, 0, true)
}

// GlowCore is Glow with a tighter falloff: a small hot centre and a wide
// faint skirt, as for a star or a beacon.
func (c *Canvas) GlowCore(cx, cy, r float32, col Color) {
	c.blob(cx, cy, r, r, col, 1, true)
}

// Blob draws a soft-edged ellipse of col centred on (cx, cy), fading to
// nothing at the radii rx and ry. Unlike Glow it covers what is beneath it
// (normal blending), which makes it suitable for soft shadows and smoke.
func (c *Canvas) Blob(cx, cy, rx, ry float32, col Color) {
	c.blob(cx, cy, rx, ry, col, 0, false)
}

func (c *Canvas) blob(cx, cy, rx, ry float32, col Color, tight float32, additive bool) {
	if col.A == 0 || !(rx > 0 && ry > 0) || c.Dst == nil {
		return
	}
	x, y := c.px(cx, cy)
	s := c.scale()
	rx, ry = rx*s, ry*s
	if !c.visible(x-rx, y-ry, x+rx, y+ry) {
		return
	}
	p := toPcol(col)
	corners := [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}}
	for i, k := range corners {
		glowVerts[i] = ebiten.Vertex{
			DstX: x + k[0]*rx, DstY: y + k[1]*ry,
			ColorR: p.r, ColorG: p.g, ColorB: p.b, ColorA: p.a,
			Custom0: k[0], Custom1: k[1], Custom2: tight, Custom3: 1,
		}
	}
	op := &glowOptsNormal
	if additive {
		op = &glowOptsAdd
	}
	c.Dst.DrawTrianglesShader32(glowVerts[:], glowIndices[:], ensureGlowShader(), op)
}
