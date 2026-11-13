package main

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// VS_FIXEDFILEINFO constants (verrsrc.h).
const (
	vsSignature     = 0xFEEF04BD // dwSignature
	vsStrucVersion  = 0x00010000 // dwStrucVersion: 1.0
	vsFlagsMask     = 0x0000003F // VS_FFI_FILEFLAGSMASK
	vsFlagPreRel    = 0x00000002 // VS_FF_PRERELEASE
	vsOSNTWindows32 = 0x00040004 // VOS_NT_WINDOWS32
	vsTypeApp       = 0x00000001 // VFT_APP

	fixedFileInfoSize = 52 // 13 DWORDs
)

// fixedFileInfo mirrors VS_FIXEDFILEINFO, the binary part of the version
// resource.
type fixedFileInfo struct {
	Signature        uint32
	StrucVersion     uint32
	FileVersionMS    uint32
	FileVersionLS    uint32
	ProductVersionMS uint32
	ProductVersionLS uint32
	FileFlagsMask    uint32
	FileFlags        uint32
	FileOS           uint32
	FileType         uint32
	FileSubtype      uint32
	FileDateMS       uint32
	FileDateLS       uint32
}

// FileVersion returns the four parts of the numeric file version.
func (f fixedFileInfo) FileVersion() [4]uint16 {
	return [4]uint16{uint16(f.FileVersionMS >> 16), uint16(f.FileVersionMS), uint16(f.FileVersionLS >> 16), uint16(f.FileVersionLS)}
}

// ProductVersion returns the four parts of the numeric product version.
func (f fixedFileInfo) ProductVersion() [4]uint16 {
	return [4]uint16{uint16(f.ProductVersionMS >> 16), uint16(f.ProductVersionMS), uint16(f.ProductVersionLS >> 16), uint16(f.ProductVersionLS)}
}

// versionString is one name/value pair of the version resource's string
// table, such as ProductName = "Drifthold".
type versionString struct{ Key, Value string }

// versionInfo is the content of an RT_VERSION resource.
type versionInfo struct {
	Version    [4]uint16 // numeric file and product version
	PreRelease bool      // sets VS_FF_PRERELEASE
	Strings    []versionString
}

// parseVersion parses a dotted version of one to four numeric parts, such as
// "1.0.0" or "v2.3.1.7"; missing parts are zero. A suffix introduced by "-"
// or "+" ("1.1.0-beta.2", "1.0.0+build5") is accepted and ignored for the
// numeric value; pre reports whether it marks a pre-release ("-...").
func parseVersion(s string) (v [4]uint16, pre bool, err error) {
	num := strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(num, "-+"); i >= 0 {
		pre = num[i] == '-'
		num = num[:i]
	}
	parts := strings.Split(num, ".")
	if len(parts) > 4 {
		return v, false, fmt.Errorf("invalid version %q: more than four numeric parts", s)
	}
	for i, p := range parts {
		n, convErr := strconv.ParseUint(p, 10, 16)
		if convErr != nil {
			return v, false, fmt.Errorf("invalid version %q: part %q is not a number in 0..65535", s, p)
		}
		v[i] = uint16(n)
	}
	return v, pre, nil
}

// dotted formats a numeric version as "a.b.c.d".
func dotted(v [4]uint16) string {
	return fmt.Sprintf("%d.%d.%d.%d", v[0], v[1], v[2], v[3])
}

// verBlock is a node of the version resource. VS_VERSIONINFO and all the
// structures nested in it share one layout: a header of three WORDs (total
// length, value length, type), a NUL-terminated UTF-16 key, padding to a
// 32-bit boundary, an optional value, and child blocks, each starting on a
// 32-bit boundary.
type verBlock struct {
	Key      string
	Text     bool   // wType: true for text (1), false for binary (0)
	Value    []byte // raw value
	ValueLen uint16 // wValueLength: UTF-16 units for text, bytes for binary
	Children []verBlock
}

// encode serialises the block. Its length field excludes any padding that
// follows it, as resource compilers write it.
func (b verBlock) encode() []byte {
	le := binary.LittleEndian
	out := make([]byte, 6)
	out = appendUTF16z(out, b.Key)
	out = pad32(out)
	out = append(out, b.Value...)
	for _, c := range b.Children {
		out = pad32(out)
		out = append(out, c.encode()...)
	}
	le.PutUint16(out[0:], uint16(len(out)))
	le.PutUint16(out[2:], b.ValueLen)
	if b.Text {
		le.PutUint16(out[4:], 1)
	}
	return out
}

// appendUTF16z appends s as NUL-terminated UTF-16LE.
func appendUTF16z(out []byte, s string) []byte {
	for _, u := range utf16.Encode([]rune(s)) {
		out = binary.LittleEndian.AppendUint16(out, u)
	}
	return append(out, 0, 0)
}

// pad32 appends zero bytes up to the next multiple of four.
func pad32(out []byte) []byte {
	for len(out)%4 != 0 {
		out = append(out, 0)
	}
	return out
}

// encode builds the VS_VERSIONINFO structure: the fixed file information,
// a StringFileInfo block with one string table for US English in Unicode,
// and the VarFileInfo block announcing that translation.
func (vi versionInfo) encode() ([]byte, error) {
	ms := uint32(vi.Version[0])<<16 | uint32(vi.Version[1])
	ls := uint32(vi.Version[2])<<16 | uint32(vi.Version[3])
	fixed := fixedFileInfo{
		Signature:        vsSignature,
		StrucVersion:     vsStrucVersion,
		FileVersionMS:    ms,
		FileVersionLS:    ls,
		ProductVersionMS: ms,
		ProductVersionLS: ls,
		FileFlagsMask:    vsFlagsMask,
		FileOS:           vsOSNTWindows32,
		FileType:         vsTypeApp,
	}
	if vi.PreRelease {
		fixed.FileFlags |= vsFlagPreRel
	}
	value, err := binary.Append(nil, binary.LittleEndian, fixed)
	if err != nil {
		return nil, err
	}

	var strs []verBlock
	seen := make(map[string]bool)
	for _, s := range vi.Strings {
		if s.Key == "" || strings.ContainsRune(s.Key, 0) || strings.ContainsRune(s.Value, 0) {
			return nil, fmt.Errorf("invalid version string %q", s.Key)
		}
		if seen[s.Key] {
			return nil, fmt.Errorf("duplicate version string %q", s.Key)
		}
		seen[s.Key] = true
		text := appendUTF16z(nil, s.Value)
		strs = append(strs, verBlock{Key: s.Key, Text: true, Value: text, ValueLen: uint16(len(text) / 2)})
	}

	le := binary.LittleEndian
	translation := le.AppendUint16(le.AppendUint16(nil, langEnUS), codePageUnicode)
	root := verBlock{
		Key:      "VS_VERSION_INFO",
		Value:    value,
		ValueLen: fixedFileInfoSize,
		Children: []verBlock{
			{Key: "StringFileInfo", Text: true, Children: []verBlock{
				{Key: fmt.Sprintf("%04X%04X", langEnUS, codePageUnicode), Text: true, Children: strs},
			}},
			{Key: "VarFileInfo", Text: true, Children: []verBlock{
				{Key: "Translation", Value: translation, ValueLen: uint16(len(translation))},
			}},
		},
	}
	out := root.encode()
	if len(out) > 0xFFFF {
		return nil, fmt.Errorf("version information is %d bytes, the format allows 65535", len(out))
	}
	return out, nil
}
