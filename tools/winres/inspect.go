package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"sort"
	"strings"
	"unicode/utf16"
)

// This file reads resources back: from the object file this tool writes and
// from a linked executable. It is what the -check flag and the tests use to
// prove that the resources survive the trip through the linker intact.

// leaf is a resource found by walking a resource directory tree.
type leaf struct {
	Type, ID, Lang uint16
	EntryOff       uint32 // offset of its IMAGE_RESOURCE_DATA_ENTRY from the start of the section
	DataRVA        uint32 // OffsetToData as stored: an RVA once linked
	CodePage       uint32
	Data           []byte
}

// walkResources parses the three-level resource directory tree rooted at
// offset root of image, where image holds the bytes of the section containing
// the resources and base is the RVA of image[0]. In an object file, before
// relocation, base is zero and "RVAs" are plain section offsets.
//
// It is deliberately strict and rejects anything Windows could trip over:
// unsorted directories, named entries (Drifthold uses numeric ids only), a
// tree that is not exactly type/id/language deep, and offsets or data that
// fall outside the section.
func walkResources(image []byte, root, base uint32) ([]leaf, error) {
	le := binary.LittleEndian
	// entries returns the (id, offset) pairs of the directory at off.
	entries := func(off uint32, what string) ([][2]uint32, error) {
		if uint64(off)+resDirSize > uint64(len(image)) {
			return nil, fmt.Errorf("%s directory at %#x is outside the section", what, off)
		}
		named, ids := int(le.Uint16(image[off+12:])), int(le.Uint16(image[off+14:]))
		if named != 0 {
			return nil, fmt.Errorf("%s directory at %#x has %d named entries; only numeric ids are supported", what, off, named)
		}
		first := uint64(off) + resDirSize
		if first+uint64(ids)*resDirEntrySize > uint64(len(image)) {
			return nil, fmt.Errorf("%s directory at %#x: %d entries do not fit in the section", what, off, ids)
		}
		out := make([][2]uint32, ids)
		for i := range out {
			e := image[first+uint64(i)*resDirEntrySize:]
			out[i] = [2]uint32{le.Uint32(e), le.Uint32(e[4:])}
			if out[i][0] > 0xFFFF {
				return nil, fmt.Errorf("%s directory at %#x: entry %d is named or has an id above 65535", what, off, i)
			}
			if i > 0 && out[i][0] <= out[i-1][0] {
				return nil, fmt.Errorf("%s directory at %#x is not sorted: id %d follows %d", what, off, out[i][0], out[i-1][0])
			}
		}
		return out, nil
	}
	// subdir checks that an entry points at a directory and returns the
	// directory's offset within image. Directory offsets are relative to
	// the root of the tree.
	subdir := func(target uint32, what string) (uint32, error) {
		if target&resSubdirectory == 0 {
			return 0, fmt.Errorf("%s entry points at data, expected a directory", what)
		}
		return root + target&^resSubdirectory, nil
	}

	var leaves []leaf
	types, err := entries(root, "type")
	if err != nil {
		return nil, err
	}
	for _, t := range types {
		off, err := subdir(t[1], "type")
		if err != nil {
			return nil, err
		}
		ids, err := entries(off, "id")
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			off, err := subdir(id[1], "id")
			if err != nil {
				return nil, err
			}
			langs, err := entries(off, "language")
			if err != nil {
				return nil, err
			}
			for _, lang := range langs {
				what := fmt.Sprintf("resource type %d id %d lang %#04x", t[0], id[0], lang[0])
				if lang[1]&resSubdirectory != 0 {
					return nil, fmt.Errorf("%s: language entry points at a directory", what)
				}
				entry := root + lang[1]
				if uint64(entry)+resDataSize > uint64(len(image)) {
					return nil, fmt.Errorf("%s: data entry at %#x is outside the section", what, entry)
				}
				rva, size := le.Uint32(image[entry:]), le.Uint32(image[entry+4:])
				if rva < base || uint64(rva-base)+uint64(size) > uint64(len(image)) {
					return nil, fmt.Errorf("%s: data at RVA %#x (%d bytes) is outside the section at RVA %#x (%d bytes)",
						what, rva, size, base, len(image))
				}
				leaves = append(leaves, leaf{
					Type: uint16(t[0]), ID: uint16(id[0]), Lang: uint16(lang[0]),
					EntryOff: entry,
					DataRVA:  rva,
					CodePage: le.Uint32(image[entry+8:]),
					Data:     image[rva-base : rva-base+size],
				})
			}
		}
	}
	return leaves, nil
}

// loaded is the outcome of reading the resources of a PE/COFF file.
type loaded struct {
	Kind    string // "object" or "executable"
	Section *pe.Section
	Leaves  []leaf
}

// loadResources reads the resources of a COFF object (such as the .syso
// written by this tool) or of a linked PE executable.
//
// For an object it also verifies what the linker relies on: the .rsrc
// section's flags, and that every data entry - and nothing else - carries an
// IMAGE_REL_AMD64_ADDR32NB relocation against the section symbol. For an
// executable it locates the tree through the resource data directory and
// verifies that the directory describes the .rsrc section.
func loadResources(r io.ReaderAt) (*loaded, error) {
	f, err := pe.NewFile(r)
	if err != nil {
		return nil, err
	}
	var dirs []pe.DataDirectory
	switch oh := f.OptionalHeader.(type) {
	case nil:
		return loadObject(f)
	case *pe.OptionalHeader64:
		dirs = oh.DataDirectory[:oh.NumberOfRvaAndSizes]
	case *pe.OptionalHeader32:
		dirs = oh.DataDirectory[:oh.NumberOfRvaAndSizes]
	}
	if len(dirs) <= pe.IMAGE_DIRECTORY_ENTRY_RESOURCE || dirs[pe.IMAGE_DIRECTORY_ENTRY_RESOURCE].VirtualAddress == 0 {
		return nil, errors.New("executable has no resource data directory")
	}
	dir := dirs[pe.IMAGE_DIRECTORY_ENTRY_RESOURCE]
	for _, s := range f.Sections {
		if dir.VirtualAddress < s.VirtualAddress || dir.VirtualAddress >= s.VirtualAddress+s.VirtualSize {
			continue
		}
		if s.Name != rsrcSectionName {
			return nil, fmt.Errorf("resource directory lies in section %q, expected %q", s.Name, rsrcSectionName)
		}
		if dir.VirtualAddress != s.VirtualAddress || dir.Size > s.VirtualSize {
			return nil, fmt.Errorf("resource data directory (RVA %#x, %d bytes) does not match section %s (RVA %#x, %d bytes)",
				dir.VirtualAddress, dir.Size, s.Name, s.VirtualAddress, s.VirtualSize)
		}
		if s.Characteristics&rsrcSectionFlags != rsrcSectionFlags {
			return nil, fmt.Errorf("section %s has characteristics %#x, want initialised readable data", s.Name, s.Characteristics)
		}
		data, err := s.Data()
		if err != nil {
			return nil, err
		}
		if uint32(len(data)) > dir.Size {
			data = data[:dir.Size] // ignore file alignment padding
		}
		leaves, err := walkResources(data, dir.VirtualAddress-s.VirtualAddress, s.VirtualAddress)
		if err != nil {
			return nil, err
		}
		return &loaded{Kind: "executable", Section: s, Leaves: leaves}, nil
	}
	return nil, fmt.Errorf("resource directory RVA %#x is not inside any section", dir.VirtualAddress)
}

// loadObject reads and verifies the .rsrc section of a COFF object.
func loadObject(f *pe.File) (*loaded, error) {
	if f.Machine != imageFileMachineAMD64 {
		return nil, fmt.Errorf("object is for machine %#x, want AMD64 (%#x)", f.Machine, imageFileMachineAMD64)
	}
	s := f.Section(rsrcSectionName)
	if s == nil {
		return nil, fmt.Errorf("object has no %s section", rsrcSectionName)
	}
	if s.Characteristics != rsrcSectionFlags {
		return nil, fmt.Errorf("section %s has characteristics %#x, want %#x", s.Name, s.Characteristics, uint32(rsrcSectionFlags))
	}
	data, err := s.Data()
	if err != nil {
		return nil, err
	}
	leaves, err := walkResources(data, 0, 0)
	if err != nil {
		return nil, err
	}

	// The relocations must cover exactly the OffsetToData fields.
	if len(s.Relocs) != len(leaves) {
		return nil, fmt.Errorf("%d relocations for %d resource data entries", len(s.Relocs), len(leaves))
	}
	want := make(map[uint32]bool, len(leaves))
	for _, l := range leaves {
		want[l.EntryOff] = true
		if l.DataRVA%4 != 0 {
			return nil, fmt.Errorf("data of resource type %d id %d is not 4-byte aligned (offset %#x)", l.Type, l.ID, l.DataRVA)
		}
	}
	for i, rel := range s.Relocs {
		if rel.Type != imageRelAMD64Addr32NB {
			return nil, fmt.Errorf("relocation %d has type %#x, want IMAGE_REL_AMD64_ADDR32NB", i, rel.Type)
		}
		if !want[rel.VirtualAddress] {
			return nil, fmt.Errorf("relocation %d at %#x does not patch a resource data entry", i, rel.VirtualAddress)
		}
		delete(want, rel.VirtualAddress)
		if int(rel.SymbolTableIndex) >= len(f.COFFSymbols) {
			return nil, fmt.Errorf("relocation %d refers to symbol %d of %d", i, rel.SymbolTableIndex, len(f.COFFSymbols))
		}
		sym := f.COFFSymbols[rel.SymbolTableIndex]
		name, err := sym.FullName(f.StringTable)
		if err != nil {
			return nil, err
		}
		if name != rsrcSectionName || sym.SectionNumber != 1 || sym.Value != 0 || sym.StorageClass != imageSymClassStatic {
			return nil, fmt.Errorf("relocation %d is against symbol %q (section %d, value %d, class %d), want the %s section symbol",
				i, name, sym.SectionNumber, sym.Value, sym.StorageClass, rsrcSectionName)
		}
	}
	return &loaded{Kind: "object", Section: s, Leaves: leaves}, nil
}

// iconReport describes one image of the application icon.
type iconReport struct {
	ID     uint16
	Size   int    // width and height in pixels
	Format string // "DIB" or "PNG"
	Bytes  int
	Solid  int // number of pixels that are not fully transparent
}

// manifestReport holds the settings found in the application manifest.
type manifestReport struct {
	Text           string
	Name           string // assemblyIdentity name
	Version        string // assemblyIdentity version
	ExecutionLevel string // requestedExecutionLevel level
	SupportedOS    []string
	DPIAware       string
	DPIAwareness   string
	LongPathAware  string
}

// versionReport holds the decoded version information.
type versionReport struct {
	Fixed        fixedFileInfo
	Table        string            // key of the string table, e.g. "040904B0"
	Strings      map[string]string // StringFileInfo entries
	Translations [][2]uint16       // (language, code page) pairs
}

// report is everything inspect extracts from a set of resources.
type report struct {
	Icons    []iconReport // in icon group order
	Manifest manifestReport
	Version  versionReport
}

// inspect decodes and cross-checks the resources Drifthold ships: the icon
// group with id 1 and the images it refers to, the manifest and the version
// information. Any inconsistency is an error.
func inspect(leaves []leaf) (*report, error) {
	find := func(typ, id uint16) *leaf {
		for i := range leaves {
			if leaves[i].Type == typ && leaves[i].ID == id {
				return &leaves[i]
			}
		}
		return nil
	}
	var rep report

	// Icon group and its images.
	group := find(rtGroupIcon, iconGroupID)
	if group == nil {
		return nil, fmt.Errorf("no RT_GROUP_ICON resource with id %d", iconGroupID)
	}
	le := binary.LittleEndian
	g := group.Data
	if len(g) < 6 || le.Uint16(g) != 0 || le.Uint16(g[2:]) != 1 {
		return nil, errors.New("RT_GROUP_ICON: bad header")
	}
	n := int(le.Uint16(g[4:]))
	const entrySize = 14
	if len(g) != 6+entrySize*n {
		return nil, fmt.Errorf("RT_GROUP_ICON: %d bytes for %d entries, want %d", len(g), n, 6+entrySize*n)
	}
	used := make(map[uint16]bool)
	for i := 0; i < n; i++ {
		e := g[6+entrySize*i:]
		w, h := int(e[0]), int(e[1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		planes, bits, size, id := le.Uint16(e[4:]), le.Uint16(e[6:]), le.Uint32(e[8:]), le.Uint16(e[12:])
		what := fmt.Sprintf("icon group entry %d (%dx%d, id %d)", i, w, h, id)
		if w != h || planes != 1 || bits != iconBitCount {
			return nil, fmt.Errorf("%s: want a square 32-bit image, got %d planes, %d bits", what, planes, bits)
		}
		icon := find(rtIcon, id)
		if icon == nil {
			return nil, fmt.Errorf("%s: no RT_ICON resource with that id", what)
		}
		if used[id] {
			return nil, fmt.Errorf("%s: RT_ICON referenced twice", what)
		}
		used[id] = true
		if icon.Lang != group.Lang {
			return nil, fmt.Errorf("%s: icon language %#04x differs from the group's %#04x", what, icon.Lang, group.Lang)
		}
		if int(size) != len(icon.Data) {
			return nil, fmt.Errorf("%s: group says %d bytes, RT_ICON has %d", what, size, len(icon.Data))
		}
		pix, format, err := decodeIcon(icon.Data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", what, err)
		}
		if len(pix) != w*h {
			return nil, fmt.Errorf("%s: image data is for %d pixels", what, len(pix))
		}
		solid := 0
		for _, p := range pix {
			if p.A != 0 {
				solid++
			}
		}
		if solid == 0 {
			return nil, fmt.Errorf("%s: image is entirely transparent", what)
		}
		rep.Icons = append(rep.Icons, iconReport{ID: id, Size: w, Format: format, Bytes: len(icon.Data), Solid: solid})
	}
	for _, l := range leaves {
		if l.Type == rtIcon && !used[l.ID] {
			return nil, fmt.Errorf("RT_ICON %d is not referenced by the icon group", l.ID)
		}
	}

	// Manifest.
	m := find(rtManifest, 1)
	if m == nil {
		return nil, errors.New("no RT_MANIFEST resource with id 1")
	}
	var err error
	if rep.Manifest, err = decodeManifest(m.Data); err != nil {
		return nil, fmt.Errorf("RT_MANIFEST: %w", err)
	}

	// Version information.
	v := find(rtVersion, 1)
	if v == nil {
		return nil, errors.New("no RT_VERSION resource with id 1")
	}
	if rep.Version, err = decodeVersion(v.Data); err != nil {
		return nil, fmt.Errorf("RT_VERSION: %w", err)
	}
	return &rep, nil
}

// decodeIcon decodes the payload of an RT_ICON resource, in either of the two
// formats Windows accepts, into rows of non-premultiplied pixels (top row
// first). It returns the pixels and the format name, "DIB" or "PNG".
func decodeIcon(data []byte) ([]color.NRGBA, string, error) {
	if bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, "", err
		}
		if img.ColorModel() != color.NRGBAModel {
			return nil, "", errors.New("PNG icon is not 8-bit RGBA")
		}
		b := img.Bounds()
		pix := make([]color.NRGBA, 0, b.Dx()*b.Dy())
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				pix = append(pix, color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA))
			}
		}
		return pix, "PNG", nil
	}

	le := binary.LittleEndian
	if len(data) < bitmapInfoHeaderSize || le.Uint32(data) != bitmapInfoHeaderSize {
		return nil, "", errors.New("neither a PNG nor a BITMAPINFOHEADER")
	}
	w, h2 := int(int32(le.Uint32(data[4:]))), int(int32(le.Uint32(data[8:])))
	planes, bits, compression := le.Uint16(data[12:]), le.Uint16(data[14:]), le.Uint32(data[16:])
	if w <= 0 || w > 256 || h2 != 2*w || planes != 1 || bits != iconBitCount || compression != 0 {
		return nil, "", fmt.Errorf("unsupported DIB: %dx%d, %d planes, %d bits, compression %d", w, h2, planes, bits, compression)
	}
	maskStride := alignUp(w, 32) / 8
	colourSize, maskSize := 4*w*w, maskStride*w
	if len(data) != bitmapInfoHeaderSize+colourSize+maskSize {
		return nil, "", fmt.Errorf("DIB is %d bytes, want %d", len(data), bitmapInfoHeaderSize+colourSize+maskSize)
	}
	if sz := le.Uint32(data[20:]); sz != 0 && int(sz) != colourSize+maskSize {
		return nil, "", fmt.Errorf("DIB biSizeImage is %d, want %d", sz, colourSize+maskSize)
	}
	colour, mask := data[bitmapInfoHeaderSize:], data[bitmapInfoHeaderSize+colourSize:]
	pix := make([]color.NRGBA, w*w)
	for y := 0; y < w; y++ {
		row := w - 1 - y // stored bottom-up
		for x := 0; x < w; x++ {
			c := colour[4*(row*w+x):]
			p := color.NRGBA{R: c[2], G: c[1], B: c[0], A: c[3]}
			masked := mask[row*maskStride+x/8]&(0x80>>(x%8)) != 0
			if masked != (p.A == 0) {
				return nil, "", fmt.Errorf("DIB mask and alpha disagree at (%d,%d)", x, y)
			}
			pix[y*w+x] = p
		}
	}
	return pix, "DIB", nil
}

// decodeManifest checks that the manifest is well-formed XML and extracts the
// settings Drifthold relies on.
func decodeManifest(data []byte) (manifestReport, error) {
	rep := manifestReport{Text: string(data)}
	if bytes.IndexByte(data, 0) >= 0 {
		return rep, errors.New("manifest contains NUL bytes")
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var path []xml.Name
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return rep, fmt.Errorf("not well-formed XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			path = append(path, t.Name)
			attr := func(name string) string {
				for _, a := range t.Attr {
					if a.Name.Local == name {
						return a.Value
					}
				}
				return ""
			}
			switch t.Name.Local {
			case "assembly":
				if len(path) != 1 || t.Name.Space != "urn:schemas-microsoft-com:asm.v1" || attr("manifestVersion") != "1.0" {
					return rep, errors.New("root element is not an asm.v1 assembly with manifestVersion 1.0")
				}
			case "assemblyIdentity":
				if len(path) == 2 {
					rep.Name, rep.Version = attr("name"), attr("version")
				}
			case "requestedExecutionLevel":
				rep.ExecutionLevel = attr("level")
			case "supportedOS":
				rep.SupportedOS = append(rep.SupportedOS, attr("Id"))
			}
		case xml.EndElement:
			path = path[:len(path)-1]
		case xml.CharData:
			if len(path) == 0 {
				continue
			}
			text := strings.TrimSpace(string(t))
			switch name := path[len(path)-1]; {
			case name.Local == "dpiAware" && name.Space == nsSettings2005:
				rep.DPIAware = text
			case name.Local == "dpiAwareness" && name.Space == nsSettings2016:
				rep.DPIAwareness = text
			case name.Local == "longPathAware" && name.Space == nsSettings2016:
				rep.LongPathAware = text
			}
		}
	}
	if rep.Name == "" || rep.Version == "" {
		return rep, errors.New("manifest has no assemblyIdentity")
	}
	return rep, nil
}

// verNode is a decoded block of a version resource.
type verNode struct {
	Key      string
	Text     bool
	Value    []byte
	Children []verNode
}

// decodeVerNode decodes the block at the start of b and returns it with its
// encoded length. depth guards against runaway nesting in corrupt data.
func decodeVerNode(b []byte, depth int) (verNode, int, error) {
	var n verNode
	le := binary.LittleEndian
	if depth > 8 {
		return n, 0, errors.New("blocks nested too deeply")
	}
	if len(b) < 6 {
		return n, 0, errors.New("truncated block header")
	}
	length, valueLen, typ := int(le.Uint16(b)), int(le.Uint16(b[2:])), le.Uint16(b[4:])
	if length < 6 || length > len(b) || typ > 1 {
		return n, 0, fmt.Errorf("bad block header: length %d of %d available, type %d", length, len(b), typ)
	}
	b = b[:length]
	n.Text = typ == 1

	// Key: UTF-16 up to a NUL.
	off := 6
	var key []uint16
	for {
		if off+2 > len(b) {
			return n, 0, errors.New("unterminated block key")
		}
		u := le.Uint16(b[off:])
		off += 2
		if u == 0 {
			break
		}
		key = append(key, u)
	}
	n.Key = string(utf16.Decode(key))
	off = alignUp(off, 4)

	// Value: its length counts UTF-16 units for text and bytes otherwise.
	size := valueLen
	if n.Text {
		size *= 2
	}
	if size > 0 {
		if off+size > len(b) {
			return n, 0, fmt.Errorf("block %q: value of %d bytes exceeds the block", n.Key, size)
		}
		n.Value = b[off : off+size]
		off += size
	}

	// Children fill the rest, each on a 32-bit boundary.
	for off = alignUp(off, 4); off < len(b); off = alignUp(off, 4) {
		child, used, err := decodeVerNode(b[off:], depth+1)
		if err != nil {
			return n, 0, fmt.Errorf("block %q: %w", n.Key, err)
		}
		n.Children = append(n.Children, child)
		off += used
	}
	return n, length, nil
}

// text returns a text block's value without its terminating NUL.
func (n verNode) text() (string, error) {
	if !n.Text || len(n.Value) < 2 || len(n.Value)%2 != 0 {
		return "", fmt.Errorf("block %q does not hold a string", n.Key)
	}
	u := make([]uint16, len(n.Value)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(n.Value[2*i:])
	}
	if u[len(u)-1] != 0 {
		return "", fmt.Errorf("string %q is not NUL-terminated", n.Key)
	}
	return string(utf16.Decode(u[:len(u)-1])), nil
}

// decodeVersion decodes a VS_VERSIONINFO structure.
func decodeVersion(data []byte) (versionReport, error) {
	rep := versionReport{Strings: make(map[string]string)}
	root, used, err := decodeVerNode(data, 0)
	if err != nil {
		return rep, err
	}
	if used != len(data) {
		return rep, fmt.Errorf("structure is %d bytes but the resource has %d", used, len(data))
	}
	if root.Key != "VS_VERSION_INFO" || root.Text || len(root.Value) != fixedFileInfoSize {
		return rep, fmt.Errorf("bad root block %q with a %d byte value", root.Key, len(root.Value))
	}
	if _, err := binary.Decode(root.Value, binary.LittleEndian, &rep.Fixed); err != nil {
		return rep, err
	}
	if rep.Fixed.Signature != vsSignature || rep.Fixed.StrucVersion != vsStrucVersion {
		return rep, fmt.Errorf("bad fixed file info signature %#x / version %#x", rep.Fixed.Signature, rep.Fixed.StrucVersion)
	}

	for _, c := range root.Children {
		switch c.Key {
		case "StringFileInfo":
			if len(c.Children) != 1 {
				return rep, fmt.Errorf("StringFileInfo has %d string tables, want 1", len(c.Children))
			}
			table := c.Children[0]
			rep.Table = table.Key
			for _, s := range table.Children {
				if _, dup := rep.Strings[s.Key]; dup {
					return rep, fmt.Errorf("duplicate string %q", s.Key)
				}
				if rep.Strings[s.Key], err = s.text(); err != nil {
					return rep, err
				}
			}
		case "VarFileInfo":
			for _, v := range c.Children {
				if v.Key != "Translation" || v.Text || len(v.Value) == 0 || len(v.Value)%4 != 0 {
					return rep, fmt.Errorf("bad VarFileInfo block %q", v.Key)
				}
				for i := 0; i < len(v.Value); i += 4 {
					rep.Translations = append(rep.Translations, [2]uint16{
						binary.LittleEndian.Uint16(v.Value[i:]), binary.LittleEndian.Uint16(v.Value[i+2:]),
					})
				}
			}
		default:
			return rep, fmt.Errorf("unexpected block %q", c.Key)
		}
	}
	if rep.Table == "" || len(rep.Translations) == 0 {
		return rep, errors.New("missing StringFileInfo or VarFileInfo")
	}
	// The string table must be the one the translation announces, or
	// Windows will not find the strings.
	t := rep.Translations[0]
	if want := fmt.Sprintf("%04X%04X", t[0], t[1]); !strings.EqualFold(rep.Table, want) {
		return rep, fmt.Errorf("string table %q does not match translation %s", rep.Table, want)
	}
	return rep, nil
}

// sortedKeys returns the keys of m in sorted order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
