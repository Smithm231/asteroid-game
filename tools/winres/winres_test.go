package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"

	"drifthold/internal/emblem"
)

// testOptions are the options most tests generate the object with.
var testOptions = options{Version: "1.2.3", Copyright: "(c) 2026 The Drifthold authors"}

// mustGenerate returns the object file for the given options.
func mustGenerate(t *testing.T, o options) []byte {
	t.Helper()
	obj, err := generate(o)
	if err != nil {
		t.Fatalf("generate(%+v): %v", o, err)
	}
	return obj
}

// wantLeaves is the resource tree the tool must produce, in directory order:
// (type, id) pairs, all in US English.
func wantLeaves() [][2]uint16 {
	var want [][2]uint16
	for i := range emblem.Sizes() {
		want = append(want, [2]uint16{rtIcon, uint16(i + 1)})
	}
	return append(want, [2]uint16{rtGroupIcon, 1}, [2]uint16{rtVersion, 1}, [2]uint16{rtManifest, 1})
}

// TestObjectFile parses the generated object with debug/pe, the same package
// the Go linker uses to read .syso files, and checks everything the linker
// depends on.
func TestObjectFile(t *testing.T) {
	obj := mustGenerate(t, testOptions)
	f, err := pe.NewFile(bytes.NewReader(obj))
	if err != nil {
		t.Fatalf("debug/pe cannot parse the object: %v", err)
	}
	numLeaves := len(wantLeaves())

	if f.Machine != pe.IMAGE_FILE_MACHINE_AMD64 {
		t.Errorf("machine = %#x, want AMD64", f.Machine)
	}
	if f.OptionalHeader != nil || f.SizeOfOptionalHeader != 0 {
		t.Errorf("object has an optional header")
	}
	if f.TimeDateStamp != 0 {
		t.Errorf("time stamp = %d, want 0 for reproducible output", f.TimeDateStamp)
	}
	if len(f.Sections) != 1 {
		t.Fatalf("%d sections, want 1", len(f.Sections))
	}
	s := f.Sections[0]
	if s.Name != ".rsrc" {
		t.Errorf("section name = %q, want .rsrc", s.Name)
	}
	if want := uint32(pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ); s.Characteristics != want {
		t.Errorf("section characteristics = %#x, want %#x", s.Characteristics, want)
	}
	if s.VirtualAddress != 0 || s.VirtualSize != 0 {
		t.Errorf("section has virtual address %#x / size %d, want 0 in an object", s.VirtualAddress, s.VirtualSize)
	}
	if s.Size%resDataAlign != 0 {
		t.Errorf("section size %d is not a multiple of %d", s.Size, resDataAlign)
	}
	data, err := s.Data()
	if err != nil || uint32(len(data)) != s.Size {
		t.Fatalf("section data: %d bytes, err %v; want %d bytes", len(data), err, s.Size)
	}

	// Relocations: one per resource data entry.
	if len(s.Relocs) != numLeaves || int(s.NumberOfRelocations) != numLeaves {
		t.Fatalf("%d relocations (header says %d), want %d", len(s.Relocs), s.NumberOfRelocations, numLeaves)
	}
	for i, r := range s.Relocs {
		if r.Type != 3 { // IMAGE_REL_AMD64_ADDR32NB
			t.Errorf("relocation %d: type %#x, want IMAGE_REL_AMD64_ADDR32NB (3)", i, r.Type)
		}
		if r.SymbolTableIndex != 0 {
			t.Errorf("relocation %d: symbol index %d, want 0", i, r.SymbolTableIndex)
		}
		if i > 0 && r.VirtualAddress <= s.Relocs[i-1].VirtualAddress {
			t.Errorf("relocation %d: offset %#x does not ascend", i, r.VirtualAddress)
		}
	}

	// Symbols: the section symbol with its auxiliary record.
	if len(f.Symbols) != 1 {
		t.Fatalf("%d symbols, want 1: %+v", len(f.Symbols), f.Symbols)
	}
	sym := f.Symbols[0]
	if sym.Name != ".rsrc" || sym.Value != 0 || sym.SectionNumber != 1 || sym.Type != 0 || sym.StorageClass != 3 {
		t.Errorf("symbol = %+v, want static section symbol .rsrc of section 1", sym)
	}
	if len(f.COFFSymbols) != 2 || f.COFFSymbols[0].NumberOfAuxSymbols != 1 {
		t.Fatalf("COFF symbol table has %d records, want the symbol plus 1 auxiliary record", len(f.COFFSymbols))
	}
	aux, err := f.COFFSymbolReadSectionDefAux(0)
	if err != nil {
		t.Fatalf("auxiliary section definition: %v", err)
	}
	if aux.Size != s.Size || int(aux.NumRelocs) != numLeaves {
		t.Errorf("auxiliary record: size %d, %d relocations; want %d, %d", aux.Size, aux.NumRelocs, s.Size, numLeaves)
	}
	if len(f.StringTable) != 0 {
		t.Errorf("string table has %d bytes, want it empty", len(f.StringTable))
	}

	// The file must end exactly after the string table's length field.
	end := int(f.PointerToSymbolTable) + 18*int(f.NumberOfSymbols) + 4
	if len(obj) != end {
		t.Errorf("object is %d bytes, symbol and string tables end at %d", len(obj), end)
	}
	if got := binary.LittleEndian.Uint32(obj[end-4:]); got != 4 {
		t.Errorf("string table length field = %d, want 4", got)
	}

	// The loader used by -check must agree.
	ld, err := loadResources(bytes.NewReader(obj))
	if err != nil {
		t.Fatalf("loadResources: %v", err)
	}
	if ld.Kind != "object" || len(ld.Leaves) != numLeaves {
		t.Errorf("loadResources: kind %q with %d leaves, want object with %d", ld.Kind, len(ld.Leaves), numLeaves)
	}
}

// TestResourceTree walks the directory tree of the generated section and
// checks every type, id and language, the data entries and their relocations.
func TestResourceTree(t *testing.T) {
	obj := mustGenerate(t, testOptions)
	f, err := pe.NewFile(bytes.NewReader(obj))
	if err != nil {
		t.Fatal(err)
	}
	s := f.Section(".rsrc")
	data, err := s.Data()
	if err != nil {
		t.Fatal(err)
	}
	leaves, err := walkResources(data, 0, 0)
	if err != nil {
		t.Fatalf("walkResources: %v", err)
	}
	want := wantLeaves()
	if len(leaves) != len(want) {
		t.Fatalf("%d resources, want %d", len(leaves), len(want))
	}

	le := binary.LittleEndian
	if n := le.Uint16(data[14:]); n != 4 || le.Uint16(data[12:]) != 0 {
		t.Errorf("root directory has %d id and %d named entries, want 4 types and 0", n, le.Uint16(data[12:]))
	}
	var gotTypes []uint32
	for i := 0; i < 4; i++ {
		e := data[16+8*i:]
		gotTypes = append(gotTypes, le.Uint32(e))
		if le.Uint32(e[4:])&0x80000000 == 0 {
			t.Errorf("root entry %d does not point at a subdirectory", i)
		}
	}
	if !slices.Equal(gotTypes, []uint32{3, 14, 16, 24}) {
		t.Errorf("types = %v, want [3 14 16 24] (icon, group icon, version, manifest) in ascending order", gotTypes)
	}

	prevEnd := uint32(0)
	for i, l := range leaves {
		if l.Type != want[i][0] || l.ID != want[i][1] || l.Lang != 0x0409 {
			t.Errorf("resource %d is type %d id %d lang %#04x, want type %d id %d lang 0x0409",
				i, l.Type, l.ID, l.Lang, want[i][0], want[i][1])
		}
		if l.CodePage != 0 {
			t.Errorf("resource %d: code page %d, want 0", i, l.CodePage)
		}
		if len(l.Data) == 0 {
			t.Errorf("resource %d is empty", i)
		}
		if l.DataRVA%8 != 0 {
			t.Errorf("resource %d: data offset %#x is not 8-byte aligned", i, l.DataRVA)
		}
		if l.DataRVA < prevEnd {
			t.Errorf("resource %d: data at %#x overlaps the previous resource ending at %#x", i, l.DataRVA, prevEnd)
		}
		prevEnd = l.DataRVA + uint32(len(l.Data))

		// The relocation patches this entry's OffsetToData, whose
		// addend is the section offset of the data.
		if got := s.Relocs[i].VirtualAddress; got != l.EntryOff {
			t.Errorf("resource %d: relocation at %#x, data entry at %#x", i, got, l.EntryOff)
		}
		if got := le.Uint32(data[l.EntryOff:]); got != l.DataRVA {
			t.Errorf("resource %d: addend %#x, want %#x", i, got, l.DataRVA)
		}
		if got := le.Uint32(data[l.EntryOff+4:]); int(got) != len(l.Data) {
			t.Errorf("resource %d: size field %d, want %d", i, got, len(l.Data))
		}
	}
	// Directories and data entries precede all data.
	if first := leaves[0].DataRVA; first < leaves[len(leaves)-1].EntryOff+16 {
		t.Errorf("first data at %#x lies before the end of the data entries", first)
	}
	if int(prevEnd) > len(data) || len(data)-int(prevEnd) >= resDataAlign {
		t.Errorf("section is %d bytes but the last resource ends at %d", len(data), prevEnd)
	}
}

// nrgba returns the non-premultiplied pixels of the emblem at a size, row by
// row, as the icon formats store them.
func nrgba(size int) []color.NRGBA {
	src := emblem.Render(size)
	img := image.NewNRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, image.Point{}, draw.Src)
	pix := make([]color.NRGBA, 0, size*size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			pix = append(pix, img.NRGBAAt(x, y))
		}
	}
	return pix
}

// TestIcons checks that the icon group refers to existing icon resources
// with matching sizes and byte lengths, and that every image decodes to
// exactly the emblem.
func TestIcons(t *testing.T) {
	obj := mustGenerate(t, testOptions)
	ld, err := loadResources(bytes.NewReader(obj))
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[uint16]leaf)
	var group []byte
	for _, l := range ld.Leaves {
		switch l.Type {
		case rtIcon:
			byID[l.ID] = l
		case rtGroupIcon:
			if l.ID != 1 {
				t.Errorf("icon group has id %d, want 1", l.ID)
			}
			group = l.Data
		}
	}
	sizes := emblem.Sizes()
	if len(byID) != len(sizes) {
		t.Fatalf("%d RT_ICON resources, want %d", len(byID), len(sizes))
	}

	// GRPICONDIR, decoded by hand.
	le := binary.LittleEndian
	if len(group) != 6+14*len(sizes) {
		t.Fatalf("icon group is %d bytes, want %d", len(group), 6+14*len(sizes))
	}
	if le.Uint16(group) != 0 || le.Uint16(group[2:]) != 1 || int(le.Uint16(group[4:])) != len(sizes) {
		t.Fatalf("icon group header = % x, want reserved 0, type 1, count %d", group[:6], len(sizes))
	}
	for i, size := range sizes {
		e := group[6+14*i:]
		wantDim := uint8(size) // 256 is stored as 0
		if e[0] != wantDim || e[1] != wantDim || e[2] != 0 || e[3] != 0 {
			t.Errorf("entry %d (%d px): width/height/colours/reserved = %v, want %d %d 0 0", i, size, e[:4], wantDim, wantDim)
		}
		if planes, bits := le.Uint16(e[4:]), le.Uint16(e[6:]); planes != 1 || bits != 32 {
			t.Errorf("entry %d (%d px): %d planes, %d bits; want 1, 32", i, size, planes, bits)
		}
		id := le.Uint16(e[12:])
		icon, ok := byID[id]
		if !ok {
			t.Errorf("entry %d (%d px) refers to RT_ICON %d, which does not exist", i, size, id)
			continue
		}
		if got := le.Uint32(e[8:]); int(got) != len(icon.Data) {
			t.Errorf("entry %d (%d px): dwBytesInRes = %d, RT_ICON %d has %d bytes", i, size, got, id, len(icon.Data))
		}

		pix, format, err := decodeIcon(icon.Data)
		if err != nil {
			t.Errorf("RT_ICON %d (%d px): %v", id, size, err)
			continue
		}
		wantFormat := "DIB"
		if size == 256 {
			wantFormat = "PNG"
		}
		if format != wantFormat {
			t.Errorf("RT_ICON %d (%d px) is a %s, want %s", id, size, format, wantFormat)
		}
		if !slices.Equal(pix, nrgba(size)) {
			t.Errorf("RT_ICON %d (%d px) does not decode to the emblem", id, size)
		}
		if format == "DIB" {
			maskStride := (size + 31) / 32 * 4
			if want := 40 + 4*size*size + maskStride*size; len(icon.Data) != want {
				t.Errorf("RT_ICON %d (%d px): %d bytes, want %d", id, size, len(icon.Data), want)
			}
		} else {
			cfg, err := png.DecodeConfig(bytes.NewReader(icon.Data))
			if err != nil || cfg.Width != size || cfg.Height != size {
				t.Errorf("RT_ICON %d: PNG config %+v, err %v; want %dx%d", id, cfg, err, size, size)
			}
			// IHDR: bit depth 8, colour type 6 (RGBA).
			if icon.Data[24] != 8 || icon.Data[25] != 6 {
				t.Errorf("RT_ICON %d: PNG bit depth %d, colour type %d; want 8, 6", id, icon.Data[24], icon.Data[25])
			}
		}
	}

	rep, err := inspect(ld.Leaves)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if len(rep.Icons) != len(sizes) {
		t.Fatalf("inspect found %d icons, want %d", len(rep.Icons), len(sizes))
	}
	for i, ic := range rep.Icons {
		if ic.Size != sizes[i] || ic.ID != uint16(i+1) || ic.Bytes != len(byID[ic.ID].Data) {
			t.Errorf("inspect icon %d = %+v, want size %d, id %d, %d bytes", i, ic, sizes[i], i+1, len(byID[ic.ID].Data))
		}
	}
}

// TestEncodeIconDIB checks the classic icon encoding byte by byte on a tiny
// image.
func TestEncodeIconDIB(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 1, G: 2, B: 3, A: 255})  // top left
	img.SetNRGBA(1, 0, color.NRGBA{R: 9, G: 9, B: 9, A: 0})    // top right, transparent
	img.SetNRGBA(0, 1, color.NRGBA{R: 40, G: 50, B: 60, A: 7}) // bottom left, faint
	img.SetNRGBA(1, 1, color.NRGBA{R: 4, G: 5, B: 6, A: 128})  // bottom right
	got, err := encodeIconDIB(img)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		40, 0, 0, 0, // biSize
		2, 0, 0, 0, // biWidth
		4, 0, 0, 0, // biHeight = 2 * height
		1, 0, // biPlanes
		32, 0, // biBitCount
		0, 0, 0, 0, // biCompression
		24, 0, 0, 0, // biSizeImage = 16 colour + 8 mask
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // resolution, colours
		// Colour bitmap, bottom row first, BGRA.
		60, 50, 40, 7, 6, 5, 4, 128,
		3, 2, 1, 255, 0, 0, 0, 0,
		// Mask, bottom row first, one bit per pixel, rows padded to 32 bits.
		0x00, 0, 0, 0,
		0x40, 0, 0, 0,
	}
	if !bytes.Equal(got, want) {
		t.Errorf("encodeIconDIB =\n% x\nwant\n% x", got, want)
	}

	if _, err := encodeIconDIB(image.NewNRGBA(image.Rect(0, 0, 3, 2))); err == nil {
		t.Error("encodeIconDIB accepted a non-square image")
	}
	if _, err := encodeIconPNG(image.NewNRGBA(image.Rect(0, 0, 257, 257))); err == nil {
		t.Error("encodeIconPNG accepted an image larger than 256 px")
	}
	opaque := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	draw.Draw(opaque, opaque.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	if _, err := encodeIconPNG(opaque); err == nil {
		t.Error("encodeIconPNG accepted an opaque image, which would be written as 24-bit")
	}
}

// utf16At decodes the NUL-terminated UTF-16 string at off.
func utf16At(b []byte, off int) string {
	var u []uint16
	for ; off+1 < len(b); off += 2 {
		c := binary.LittleEndian.Uint16(b[off:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

// TestVersionInfo decodes the VS_VERSIONINFO blob, both through the decoder
// and at the fixed offsets the format implies.
func TestVersionInfo(t *testing.T) {
	for _, tc := range []struct {
		name    string
		opts    options
		numeric [4]uint16
		flags   uint32
		strings map[string]string
	}{
		{
			name:    "plain",
			opts:    options{Version: "1.0.0"},
			numeric: [4]uint16{1, 0, 0, 0},
			strings: map[string]string{
				"FileDescription": "Drifthold", "FileVersion": "1.0.0", "InternalName": "drifthold",
				"OriginalFilename": "Drifthold.exe", "ProductName": "Drifthold", "ProductVersion": "1.0.0",
			},
		},
		{
			name:    "copyright and four parts",
			opts:    options{Version: "v2.10.3.4077", Copyright: "  \u00a9 2026 Drifthold \u2014 \u661f  "},
			numeric: [4]uint16{2, 10, 3, 4077},
			strings: map[string]string{
				"FileDescription": "Drifthold", "FileVersion": "2.10.3.4077", "InternalName": "drifthold",
				"LegalCopyright":   "\u00a9 2026 Drifthold \u2014 \u661f",
				"OriginalFilename": "Drifthold.exe", "ProductName": "Drifthold", "ProductVersion": "2.10.3.4077",
			},
		},
		{
			name:    "pre-release",
			opts:    options{Version: "0.9.0-beta.2"},
			numeric: [4]uint16{0, 9, 0, 0},
			flags:   0x2, // VS_FF_PRERELEASE
			strings: map[string]string{
				"FileDescription": "Drifthold", "FileVersion": "0.9.0-beta.2", "InternalName": "drifthold",
				"OriginalFilename": "Drifthold.exe", "ProductName": "Drifthold", "ProductVersion": "0.9.0-beta.2",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ld, err := loadResources(bytes.NewReader(mustGenerate(t, tc.opts)))
			if err != nil {
				t.Fatal(err)
			}
			var blob []byte
			for _, l := range ld.Leaves {
				if l.Type == rtVersion && l.ID == 1 {
					blob = l.Data
				}
			}
			if blob == nil {
				t.Fatal("no RT_VERSION resource with id 1")
			}

			// Fixed layout: header, key, padding, VS_FIXEDFILEINFO at
			// offset 40, StringFileInfo at offset 92.
			le := binary.LittleEndian
			if int(le.Uint16(blob)) != len(blob) || le.Uint16(blob[2:]) != 52 || le.Uint16(blob[4:]) != 0 {
				t.Errorf("header: length %d, value length %d, type %d; want %d, 52, 0",
					le.Uint16(blob), le.Uint16(blob[2:]), le.Uint16(blob[4:]), len(blob))
			}
			if key := utf16At(blob, 6); key != "VS_VERSION_INFO" {
				t.Errorf("root key = %q", key)
			}
			if sig := le.Uint32(blob[40:]); sig != 0xFEEF04BD {
				t.Errorf("signature at offset 40 = %#x, want 0xFEEF04BD", sig)
			}
			if key := utf16At(blob, 92+6); key != "StringFileInfo" {
				t.Errorf("block at offset 92 has key %q, want StringFileInfo", key)
			}
			if le.Uint16(blob[92+2:]) != 0 || le.Uint16(blob[92+4:]) != 1 {
				t.Errorf("StringFileInfo: value length %d, type %d; want 0, 1", le.Uint16(blob[92+2:]), le.Uint16(blob[92+4:]))
			}
			// The string table follows: 92 + header 6 + key 30 = 128.
			if key := utf16At(blob, 128+6); key != "040904B0" {
				t.Errorf("string table key = %q, want 040904B0", key)
			}

			v, err := decodeVersion(blob)
			if err != nil {
				t.Fatalf("decodeVersion: %v", err)
			}
			fx := v.Fixed
			if fx.FileVersion() != tc.numeric || fx.ProductVersion() != tc.numeric {
				t.Errorf("numeric versions = %v / %v, want %v", fx.FileVersion(), fx.ProductVersion(), tc.numeric)
			}
			if fx.StrucVersion != 0x10000 || fx.FileFlagsMask != 0x3F || fx.FileFlags != tc.flags ||
				fx.FileOS != 0x40004 || fx.FileType != 1 || fx.FileSubtype != 0 || fx.FileDateMS != 0 || fx.FileDateLS != 0 {
				t.Errorf("fixed file info = %+v", fx)
			}
			if v.Table != "040904B0" {
				t.Errorf("string table = %q, want 040904B0", v.Table)
			}
			if !slices.Equal(v.Translations, [][2]uint16{{0x0409, 0x04B0}}) {
				t.Errorf("translations = %v, want [[0x409 0x4b0]]", v.Translations)
			}
			if !maps.Equal(v.Strings, tc.strings) {
				t.Errorf("strings = %q\nwant      %q", v.Strings, tc.strings)
			}
			if _, ok := v.Strings["CompanyName"]; ok {
				t.Error("CompanyName must be omitted")
			}
		})
	}
}

// TestVersionStringLayout checks the encoding of a single String block, which
// has the fussiest rules: the value length counts UTF-16 units including the
// terminator, and the block length excludes the padding after it.
func TestVersionStringLayout(t *testing.T) {
	vi := versionInfo{Strings: []versionString{{"Ab", "xyz"}, {"C", "d"}}}
	blob, err := vi.encode()
	if err != nil {
		t.Fatal(err)
	}
	// Root 40 + fixed 52 = 92; StringFileInfo header and key 36 -> 128;
	// string table header 6 + key 18 -> 152.
	want := []byte{
		// "Ab" = "xyz": header 6 + key 6 + value 8, no padding needed.
		20, 0, // wLength
		4, 0, // wValueLength: "xyz" and the terminator
		1, 0, // wType: text
		'A', 0, 'b', 0, 0, 0,
		'x', 0, 'y', 0, 'z', 0, 0, 0,
		// "C" = "d": header 6 + key 4, padded to 12, + value 4.
		16, 0, // wLength
		2, 0, // wValueLength
		1, 0, // wType
		'C', 0, 0, 0, 0, 0,
		'd', 0, 0, 0,
	}
	if got := blob[152 : 152+len(want)]; !bytes.Equal(got, want) {
		t.Errorf("string blocks =\n% x\nwant\n% x", got, want)
	}
	// The string table ends with its last string; VarFileInfo follows.
	if key := utf16At(blob, 152+len(want)+6); key != "VarFileInfo" {
		t.Errorf("block after the string table has key %q, want VarFileInfo", key)
	}

	if _, err := (versionInfo{Strings: []versionString{{"A", "1"}, {"A", "2"}}}).encode(); err == nil {
		t.Error("duplicate version string accepted")
	}
	if _, err := (versionInfo{Strings: []versionString{{"A", strings.Repeat("x", 40000)}}}).encode(); err == nil {
		t.Error("oversized version information accepted")
	}
}

func TestParseVersion(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want [4]uint16
		pre  bool
		ok   bool
	}{
		{"1.0.0", [4]uint16{1, 0, 0, 0}, false, true},
		{"1", [4]uint16{1, 0, 0, 0}, false, true},
		{"v3.2", [4]uint16{3, 2, 0, 0}, false, true},
		{" 1.2.3.4 ", [4]uint16{1, 2, 3, 4}, false, true},
		{"0.0.0", [4]uint16{}, false, true},
		{"65535.65535.65535.65535", [4]uint16{65535, 65535, 65535, 65535}, false, true},
		{"1.2.3-rc.1", [4]uint16{1, 2, 3, 0}, true, true},
		{"1.2.3+build.7", [4]uint16{1, 2, 3, 0}, false, true},
		{"", [4]uint16{}, false, false},
		{"1.2.3.4.5", [4]uint16{}, false, false},
		{"1..2", [4]uint16{}, false, false},
		{"1.x", [4]uint16{}, false, false},
		{"65536", [4]uint16{}, false, false},
		{"-1", [4]uint16{}, false, false},
		{"1.2 beta", [4]uint16{}, false, false},
	} {
		got, pre, err := parseVersion(tc.in)
		if (err == nil) != tc.ok {
			t.Errorf("parseVersion(%q) error = %v, want ok=%v", tc.in, err, tc.ok)
			continue
		}
		if tc.ok && (got != tc.want || pre != tc.pre) {
			t.Errorf("parseVersion(%q) = %v, pre %v; want %v, pre %v", tc.in, got, pre, tc.want, tc.pre)
		}
	}
	if _, err := generate(options{Version: "one"}); err == nil {
		t.Error("generate accepted an invalid version")
	}
}

func TestManifest(t *testing.T) {
	ld, err := loadResources(bytes.NewReader(mustGenerate(t, testOptions)))
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for _, l := range ld.Leaves {
		if l.Type == rtManifest {
			if l.ID != 1 {
				t.Errorf("manifest has id %d, want 1 (CREATEPROCESS_MANIFEST_RESOURCE_ID)", l.ID)
			}
			text = string(l.Data)
		}
	}
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`,
		`<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">`,
		`<assemblyIdentity type="win32" name="Drifthold" version="1.2.3.0" processorArchitecture="amd64"/>`,
		`<requestedExecutionLevel level="asInvoker" uiAccess="false"/>`,
		`<supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>`,
		`<dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>`,
		`<dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">PerMonitorV2,PerMonitor</dpiAwareness>`,
		`<longPathAware xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">true</longPathAware>`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("manifest lacks %s", want)
		}
	}
	for _, unwanted := range []string{"Common-Controls", "<dependency", "\x00", "\ufeff"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("manifest contains %q", unwanted)
		}
	}
	if !strings.HasPrefix(text, "<?xml") || !strings.HasSuffix(text, "</assembly>\n") {
		t.Errorf("manifest does not start with the XML declaration or end with </assembly>:\n%s", text)
	}

	m, err := decodeManifest([]byte(text))
	if err != nil {
		t.Fatalf("decodeManifest: %v", err)
	}
	want := manifestReport{
		Text: text, Name: "Drifthold", Version: "1.2.3.0", ExecutionLevel: "asInvoker",
		SupportedOS: []string{"{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"},
		DPIAware:    "true/pm", DPIAwareness: "PerMonitorV2,PerMonitor", LongPathAware: "true",
	}
	if m.Name != want.Name || m.Version != want.Version || m.ExecutionLevel != want.ExecutionLevel ||
		!slices.Equal(m.SupportedOS, want.SupportedOS) || m.DPIAware != want.DPIAware ||
		m.DPIAwareness != want.DPIAwareness || m.LongPathAware != want.LongPathAware {
		t.Errorf("decodeManifest = %+v\nwant %+v", m, want)
	}

	if _, err := decodeManifest([]byte("<assembly><unclosed></assembly>")); err == nil {
		t.Error("decodeManifest accepted malformed XML")
	}
}

// TestBuildSection exercises the directory builder with resources that need
// sorting and with several languages under one id.
func TestBuildSection(t *testing.T) {
	in := []resource{
		{Type: 24, ID: 1, Lang: 0x0409, Data: []byte("manifest")},
		{Type: 3, ID: 2, Lang: 0x0409, Data: []byte("icon two")},
		{Type: 16, ID: 1, Lang: 0x0809, Data: []byte("version en-GB!")},
		{Type: 3, ID: 1, Lang: 0x0409, Data: []byte("i")},
		{Type: 16, ID: 1, Lang: 0x0407, Data: []byte("version de-DE")},
	}
	sec, relocs, err := buildSection(in)
	if err != nil {
		t.Fatal(err)
	}
	leaves, err := walkResources(sec, 0, 0)
	if err != nil {
		t.Fatalf("walkResources: %v", err)
	}
	type key struct{ typ, id, lang uint16 }
	var got []key
	for i, l := range leaves {
		got = append(got, key{l.Type, l.ID, l.Lang})
		if relocs[i] != l.EntryOff {
			t.Errorf("relocation %d at %#x, data entry at %#x", i, relocs[i], l.EntryOff)
		}
		if l.DataRVA%8 != 0 {
			t.Errorf("data of %v at %#x is not aligned", got[i], l.DataRVA)
		}
	}
	want := []key{{3, 1, 0x0409}, {3, 2, 0x0409}, {16, 1, 0x0407}, {16, 1, 0x0809}, {24, 1, 0x0409}}
	if !slices.Equal(got, want) {
		t.Errorf("tree order = %v, want %v", got, want)
	}
	for _, l := range leaves {
		for _, r := range in {
			if r.Type == l.Type && r.ID == l.ID && r.Lang == l.Lang && !bytes.Equal(r.Data, l.Data) {
				t.Errorf("type %d id %d lang %#x: data %q, want %q", l.Type, l.ID, l.Lang, l.Data, r.Data)
			}
		}
	}
	// 3 types: root 16+24; type dirs 3*16 + 4 ids*8; id dirs 4*16 + 5*8;
	// 5 data entries. Data starts on the next 8-byte boundary.
	if want := 16 + 24 + 48 + 32 + 64 + 40 + 5*16; int(leaves[0].DataRVA) != want {
		t.Errorf("first data at %d, want %d", leaves[0].DataRVA, want)
	}

	if _, _, err := buildSection(append(in, in[0])); err == nil {
		t.Error("duplicate resource accepted")
	}
	if _, _, err := buildSection(nil); err == nil {
		t.Error("empty resource list accepted")
	}
	if _, err := buildObject(sec, []uint32{uint32(len(sec))}); err == nil {
		t.Error("relocation outside the section accepted")
	}
}

// TestWalkRejectsCorruptTrees makes sure the checker is not a rubber stamp.
func TestWalkRejectsCorruptTrees(t *testing.T) {
	sec, _, err := buildSection([]resource{
		{Type: 3, ID: 1, Lang: 0x0409, Data: []byte("one")},
		{Type: 3, ID: 2, Lang: 0x0409, Data: []byte("two")},
		{Type: 14, ID: 1, Lang: 0x0409, Data: []byte("group")},
	})
	if err != nil {
		t.Fatal(err)
	}
	leaves, err := walkResources(sec, 0, 0)
	if err != nil || len(leaves) != 3 {
		t.Fatalf("walkResources on the pristine section: %d leaves, err %v", len(leaves), err)
	}
	le := binary.LittleEndian
	for _, tc := range []struct {
		name    string
		corrupt func(b []byte)
	}{
		{"unsorted types", func(b []byte) { le.PutUint32(b[16:], 20) }},
		{"named type", func(b []byte) { le.PutUint32(b[16:], 0x80000040) }},
		{"named entries", func(b []byte) { le.PutUint16(b[12:], 1) }},
		{"type entry is a leaf", func(b []byte) { le.PutUint32(b[20:], le.Uint32(b[20:])&^0x80000000) }},
		{"directory outside the section", func(b []byte) { le.PutUint32(b[20:], 0x80000000|uint32(len(b))) }},
		{"too many entries", func(b []byte) { le.PutUint16(b[14:], 0xFFFF) }},
		{"data outside the section", func(b []byte) { le.PutUint32(b[leaves[0].EntryOff:], uint32(len(b))-1) }},
		{"oversized data", func(b []byte) { le.PutUint32(b[leaves[2].EntryOff+4:], 1<<20) }},
		{"language entry is a directory", func(b []byte) {
			// The first id directory follows the root (16+2*8) and
			// the two type directories (16+2*8 and 16+8).
			off := 32 + 32 + 24 + 16 + 4
			le.PutUint32(b[off:], le.Uint32(b[off:])|0x80000000)
		}},
	} {
		b := bytes.Clone(sec)
		tc.corrupt(b)
		if _, err := walkResources(b, 0, 0); err == nil {
			t.Errorf("%s: walkResources reported no error", tc.name)
		}
	}
	if _, err := walkResources(sec[:10], 0, 0); err == nil {
		t.Error("truncated section accepted")
	}
}

// TestInspectRejectsInconsistency corrupts individual resources and expects
// inspect to notice.
func TestInspectRejectsInconsistency(t *testing.T) {
	res, err := driftholdResources(testOptions)
	if err != nil {
		t.Fatal(err)
	}
	load := func(res []resource) []leaf {
		sec, _, err := buildSection(res)
		if err != nil {
			t.Fatal(err)
		}
		leaves, err := walkResources(sec, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		return leaves
	}
	if _, err := inspect(load(res)); err != nil {
		t.Fatalf("inspect on pristine resources: %v", err)
	}
	index := func(typ, id uint16) int {
		return slices.IndexFunc(res, func(r resource) bool { return r.Type == typ && r.ID == id })
	}
	for _, tc := range []struct {
		name   string
		mutate func(res []resource) []resource
	}{
		{"missing icon image", func(res []resource) []resource { return slices.Delete(res, index(rtIcon, 4), index(rtIcon, 4)+1) }},
		{"icon length mismatch", func(res []resource) []resource {
			i := index(rtIcon, 2)
			res[i].Data = append(bytes.Clone(res[i].Data), 0, 0, 0, 0)
			return res
		}},
		{"wrong icon size in group", func(res []resource) []resource {
			i := index(rtGroupIcon, 1)
			res[i].Data = bytes.Clone(res[i].Data)
			res[i].Data[6], res[i].Data[7] = 17, 17
			return res
		}},
		{"orphan icon", func(res []resource) []resource {
			return append(res, resource{Type: rtIcon, ID: 99, Lang: langEnUS, Data: res[index(rtIcon, 1)].Data})
		}},
		{"mask disagrees with alpha", func(res []resource) []resource {
			i := index(rtIcon, 1)
			res[i].Data = bytes.Clone(res[i].Data)
			res[i].Data[len(res[i].Data)-4] ^= 0x01
			return res
		}},
		{"no manifest", func(res []resource) []resource {
			return slices.Delete(res, index(rtManifest, 1), index(rtManifest, 1)+1)
		}},
		{"truncated manifest", func(res []resource) []resource {
			i := index(rtManifest, 1)
			res[i].Data = res[i].Data[:len(res[i].Data)/2]
			return res
		}},
		{"no version", func(res []resource) []resource { return slices.Delete(res, index(rtVersion, 1), index(rtVersion, 1)+1) }},
		{"truncated version", func(res []resource) []resource {
			i := index(rtVersion, 1)
			res[i].Data = res[i].Data[:len(res[i].Data)-8]
			return res
		}},
		{"bad version signature", func(res []resource) []resource {
			i := index(rtVersion, 1)
			res[i].Data = bytes.Clone(res[i].Data)
			res[i].Data[40] ^= 0xFF
			return res
		}},
	} {
		if _, err := inspect(load(tc.mutate(slices.Clone(res)))); err == nil {
			t.Errorf("%s: inspect reported no error", tc.name)
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, b := mustGenerate(t, testOptions), mustGenerate(t, testOptions)
	if !bytes.Equal(a, b) {
		t.Error("two runs with the same options produced different objects")
	}
	if c := mustGenerate(t, options{Version: "9.9.9"}); bytes.Equal(a, c) {
		t.Error("the version option has no effect on the object")
	}
}

func TestRunCheck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rsrc_windows_amd64.syso")
	if err := os.WriteFile(path, mustGenerate(t, testOptions), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runCheck(&out, path, "1.2.3"); err != nil {
		t.Fatalf("runCheck: %v\n%s", err, out.String())
	}
	for _, want := range []string{
		"object, section .rsrc", "12 resources", "256x256 32-bit PNG, RT_ICON 9", " 16x16  32-bit DIB, RT_ICON 1",
		"level asInvoker", `ProductName      "Drifthold"`, `LegalCopyright   "(c) 2026 The Drifthold authors"`, "\nok\n",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("runCheck output lacks %q:\n%s", want, out.String())
		}
	}
	if err := runCheck(&out, path, "1.2.4"); err == nil {
		t.Error("runCheck accepted an object with a different version than requested")
	}
	if err := runCheck(&out, filepath.Join(t.TempDir(), "missing.syso"), ""); err == nil {
		t.Error("runCheck accepted a missing file")
	}
	garbage := filepath.Join(t.TempDir(), "garbage.syso")
	if err := os.WriteFile(garbage, []byte("this is not a COFF file at all, not even close"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runCheck(&out, garbage, ""); err == nil {
		t.Error("runCheck accepted garbage")
	}
}

// TestShippedObject verifies the object checked in next to the game's main
// package, when it is present: it must be well-formed and its icon must be
// the current emblem. If this fails after a change to internal/emblem,
// regenerate the file:
//
//	go run ./tools/winres -out cmd/drifthold/rsrc_windows_amd64.syso -version <version>
func TestShippedObject(t *testing.T) {
	path := filepath.Join("..", "..", "cmd", "drifthold", "rsrc_windows_amd64.syso")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skipf("%s has not been generated", path)
	}
	if err != nil {
		t.Fatal(err)
	}
	ld, err := loadResources(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	rep, err := inspect(ld.Leaves)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if err := checkDrifthold(rep, ""); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	for _, l := range ld.Leaves {
		if l.Type != rtIcon {
			continue
		}
		pix, _, err := decodeIcon(l.Data)
		if err != nil {
			t.Fatal(err)
		}
		i := slices.IndexFunc(rep.Icons, func(ic iconReport) bool { return ic.ID == l.ID })
		size := rep.Icons[i].Size
		// Allow for rounding differences between machines; a real
		// change to the emblem moves many pixels by far more.
		const tolerance = 6
		worst := 0
		for i, want := range nrgba(size) {
			got := pix[i]
			for _, d := range []int{int(got.R) - int(want.R), int(got.G) - int(want.G), int(got.B) - int(want.B), int(got.A) - int(want.A)} {
				// Colour is meaningless where the pixel is (almost) invisible.
				if got.A < 8 && want.A < 8 {
					continue
				}
				worst = max(worst, max(d, -d))
			}
		}
		if worst > tolerance {
			t.Errorf("%s: the %d px icon differs from the current emblem (by up to %d); regenerate the file with tools/winres", path, size, worst)
		}
	}
}
