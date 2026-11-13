package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/png"

	"drifthold/internal/emblem"
)

const (
	// iconGroupID is the resource id of the application icon group. Windows
	// uses the group with the lowest id as the executable's icon.
	iconGroupID = 1

	// pngIconSize is the size from which icon images are stored as PNG
	// rather than as a device independent bitmap. Windows accepts PNG
	// for any size since Vista, but the classic format is the most
	// widely compatible for small images and costs little there.
	pngIconSize = 256

	bitmapInfoHeaderSize = 40 // BITMAPINFOHEADER
	iconBitCount         = 32
)

// iconImage is one image of an icon, in the form stored in an RT_ICON
// resource.
type iconImage struct {
	Size int    // width and height in pixels
	Data []byte // DIB (header, colour bitmap, mask) or PNG file
}

// emblemIcons renders the Drifthold emblem at every shipped icon size, in
// ascending order of size.
func emblemIcons() ([]iconImage, error) {
	var icons []iconImage
	for _, size := range emblem.Sizes() {
		img := emblem.Render(size)
		var data []byte
		var err error
		if size >= pngIconSize {
			data, err = encodeIconPNG(img)
		} else {
			data, err = encodeIconDIB(img)
		}
		if err != nil {
			return nil, fmt.Errorf("%d px icon: %w", size, err)
		}
		icons = append(icons, iconImage{Size: size, Data: data})
	}
	return icons, nil
}

// iconResources turns icon images into resources: one RT_ICON per image with
// ids counting up from firstIconID, and the RT_GROUP_ICON directory that
// lists them.
func iconResources(icons []iconImage, firstIconID uint16) ([]resource, error) {
	if len(icons) == 0 {
		return nil, fmt.Errorf("icon has no images")
	}
	var res []resource

	// GRPICONDIR header: reserved, type (1 = icon), image count.
	le := binary.LittleEndian
	dir := le.AppendUint16(nil, 0)
	dir = le.AppendUint16(dir, 1)
	dir = le.AppendUint16(dir, uint16(len(icons)))
	for i, ic := range icons {
		if ic.Size < 1 || ic.Size > 256 {
			return nil, fmt.Errorf("icon size %d is outside 1..256", ic.Size)
		}
		id := firstIconID + uint16(i)
		res = append(res, resource{Type: rtIcon, ID: id, Lang: langEnUS, Data: ic.Data})

		// GRPICONDIRENTRY. A width or height of 256 is stored as 0.
		dir = append(dir,
			uint8(ic.Size), // bWidth
			uint8(ic.Size), // bHeight
			0,              // bColorCount: not palettised
			0,              // bReserved
		)
		dir = le.AppendUint16(dir, 1)                    // wPlanes
		dir = le.AppendUint16(dir, iconBitCount)         // wBitCount
		dir = le.AppendUint32(dir, uint32(len(ic.Data))) // dwBytesInRes
		dir = le.AppendUint16(dir, id)                   // nId
	}
	res = append(res, resource{Type: rtGroupIcon, ID: iconGroupID, Lang: langEnUS, Data: dir})
	return res, nil
}

// encodeIconDIB encodes a square image in the classic icon format: a
// BITMAPINFOHEADER whose height covers both bitmaps, the 32-bit BGRA colour
// ("XOR") bitmap and the 1-bit transparency ("AND") mask, both stored bottom
// row first.
func encodeIconDIB(src image.Image) ([]byte, error) {
	img, n, err := straightAlpha(src)
	if err != nil {
		return nil, err
	}
	maskStride := alignUp(n, 32) / 8 // mask rows are padded to 32 bits
	colourSize := 4 * n * n
	maskSize := maskStride * n

	out := make([]byte, bitmapInfoHeaderSize+colourSize+maskSize)
	le := binary.LittleEndian
	le.PutUint32(out[0:], bitmapInfoHeaderSize)         // biSize
	le.PutUint32(out[4:], uint32(n))                    // biWidth
	le.PutUint32(out[8:], uint32(2*n))                  // biHeight: colour + mask
	le.PutUint16(out[12:], 1)                           // biPlanes
	le.PutUint16(out[14:], iconBitCount)                // biBitCount
	le.PutUint32(out[16:], 0)                           // biCompression = BI_RGB
	le.PutUint32(out[20:], uint32(colourSize+maskSize)) // biSizeImage
	// biXPelsPerMeter, biYPelsPerMeter, biClrUsed, biClrImportant: zero.

	colour := out[bitmapInfoHeaderSize:]
	mask := out[bitmapInfoHeaderSize+colourSize:]
	for y := 0; y < n; y++ {
		row := n - 1 - y // bottom-up
		for x := 0; x < n; x++ {
			p := img.Pix[img.PixOffset(x, y):]
			r, g, b, a := p[0], p[1], p[2], p[3]
			if a == 0 {
				// Fully transparent: leave the colour black and set
				// the mask bit, so that the icon is also correct
				// wherever only the mask is honoured.
				mask[row*maskStride+x/8] |= 0x80 >> (x % 8)
				continue
			}
			o := 4 * (row*n + x)
			colour[o+0], colour[o+1], colour[o+2], colour[o+3] = b, g, r, a
		}
	}
	return out, nil
}

// encodeIconPNG encodes a square image as a 32-bit RGBA PNG file, the form
// Windows expects for PNG-compressed icon images.
func encodeIconPNG(src image.Image) ([]byte, error) {
	img, _, err := straightAlpha(src)
	if err != nil {
		return nil, err
	}
	if img.Opaque() {
		// The encoder would drop the alpha channel and write a 24-bit
		// PNG, which the Windows icon loader does not accept.
		return nil, fmt.Errorf("image is fully opaque; icon PNGs must be 32-bit RGBA")
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// straightAlpha converts a square image of at most 256 pixels a side to
// non-premultiplied RGBA, which is how both icon formats store colour, and
// returns it with its side length.
func straightAlpha(src image.Image) (*image.NRGBA, int, error) {
	b := src.Bounds()
	if b.Dx() != b.Dy() || b.Dx() < 1 || b.Dx() > 256 {
		return nil, 0, fmt.Errorf("icon image must be square and 1..256 pixels a side, got %dx%d", b.Dx(), b.Dy())
	}
	img := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	if n, ok := src.(*image.NRGBA); ok {
		// Already in the right form; copy the rows unchanged.
		for y := 0; y < b.Dy(); y++ {
			copy(img.Pix[y*img.Stride:(y+1)*img.Stride], n.Pix[n.PixOffset(b.Min.X, b.Min.Y+y):])
		}
	} else {
		draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	}
	return img, b.Dx(), nil
}
