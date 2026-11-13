// Command preview writes the Drifthold emblem to PNG files for inspection.
//
// Usage:
//
//	go run ./internal/emblem/preview -out /some/dir
//
// It writes emblem-<size>.png for every size shipped in the Windows icon plus
// a 512 pixel rendering, and sheet.png: a contact sheet showing the small
// sizes at 1:1 and magnified, on a dark and a light backdrop, which is the
// quickest way to judge how the mark reads in a title bar or task bar.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"

	"drifthold/internal/emblem"
)

func main() {
	log.SetFlags(0)
	log.SetPrefix("preview: ")
	out := flag.String("out", "", "directory to write the PNG files into (required)")
	flag.Parse()
	if *out == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*out); err != nil {
		log.Fatal(err)
	}
}

func run(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, size := range append(emblem.Sizes(), 512) {
		name := filepath.Join(dir, fmt.Sprintf("emblem-%d.png", size))
		if err := writePNG(name, emblem.Render(size)); err != nil {
			return err
		}
	}
	return writePNG(filepath.Join(dir, "sheet.png"), sheet())
}

func writePNG(name string, img image.Image) error {
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// sheet lays out the small icon sizes on a dark and a light band: a row at
// 1:1, a row magnified with nearest-neighbour sampling (the actual pixels)
// and a row magnified smoothly (the overall impression).
func sheet() *image.RGBA {
	const (
		cell   = 208 // magnified cell size
		margin = 16
		strip  = 80 // height of the 1:1 row
	)
	sizes := []int{16, 20, 24, 32, 40, 48, 64}
	backdrops := []color.RGBA{
		{0x1B, 0x1B, 0x1F, 0xFF}, // dark task bar
		{0xF3, 0xF3, 0xF3, 0xFF}, // light title bar
	}
	bandH := strip + 2*cell + 4*margin
	w := margin + len(sizes)*(cell+margin)
	img := image.NewRGBA(image.Rect(0, 0, w, bandH*len(backdrops)))
	for b, bg := range backdrops {
		top := b * bandH
		draw.Draw(img, image.Rect(0, top, w, top+bandH), image.NewUniform(bg), image.Point{}, draw.Src)
		for i, size := range sizes {
			src := emblem.Render(size)
			x := margin + i*(cell+margin)

			// 1:1, centred in the strip.
			at := image.Pt(x+(cell-size)/2, top+margin+(strip-size)/2)
			draw.Draw(img, image.Rectangle{Min: at, Max: at.Add(image.Pt(size, size))}, src, image.Point{}, draw.Over)

			// Magnified by an integer factor.
			k := cell / size
			big := magnify(src, k)
			at = image.Pt(x+(cell-size*k)/2, top+strip+2*margin+(cell-size*k)/2)
			draw.Draw(img, image.Rectangle{Min: at, Max: at.Add(big.Bounds().Size())}, big, image.Point{}, draw.Over)

			// Magnified smoothly, which approximates how the eye blends
			// the pixels at the icon's real size.
			at = image.Pt(x, top+strip+cell+3*margin)
			xdraw.CatmullRom.Scale(img, image.Rectangle{Min: at, Max: at.Add(image.Pt(cell, cell))}, src, src.Bounds(), draw.Over, nil)
		}
	}
	return img
}

// magnify scales src up by the integer factor k without interpolation.
func magnify(src *image.RGBA, k int) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()*k, b.Dy()*k))
	for y := 0; y < b.Dy()*k; y++ {
		for x := 0; x < b.Dx()*k; x++ {
			dst.SetRGBA(x, y, src.RGBAAt(b.Min.X+x/k, b.Min.Y+y/k))
		}
	}
	return dst
}
