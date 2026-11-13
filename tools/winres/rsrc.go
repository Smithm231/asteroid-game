package main

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// Predefined Windows resource types (RT_* in winuser.h) used by Drifthold.
const (
	rtIcon      = 3  // RT_ICON: one icon image
	rtGroupIcon = 14 // RT_GROUP_ICON: directory of the images of one icon
	rtVersion   = 16 // RT_VERSION: VS_VERSIONINFO
	rtManifest  = 24 // RT_MANIFEST: side-by-side application manifest
)

const (
	// langEnUS is the language id all resources are stored under
	// (MAKELANGID(LANG_ENGLISH, SUBLANG_ENGLISH_US)).
	langEnUS = 0x0409
	// codePageUnicode is the code page named in the version information
	// (1200, UTF-16LE).
	codePageUnicode = 0x04B0
)

// Sizes of the on-disk resource structures (winnt.h).
const (
	resDirSize      = 16 // IMAGE_RESOURCE_DIRECTORY
	resDirEntrySize = 8  // IMAGE_RESOURCE_DIRECTORY_ENTRY
	resDataSize     = 16 // IMAGE_RESOURCE_DATA_ENTRY

	// resSubdirectory is the high bit of a directory entry's offset; when
	// set the entry points at another directory instead of a data entry.
	resSubdirectory = 0x80000000

	// resDataAlign is the alignment of every resource's raw data within
	// the section.
	resDataAlign = 8
)

// resource is one leaf of the resource tree: a blob addressed by a numeric
// type, a numeric id and a language.
type resource struct {
	Type uint16
	ID   uint16
	Lang uint16
	Data []byte
}

// buildSection lays out the contents of a .rsrc section for the given
// resources, in the conventional order: the three levels of directories
// (type, then id, then language), the data entries, and finally the raw data.
//
// The returned relocs are the section offsets of the OffsetToData field of
// every data entry, in ascending order. Each of those fields holds the
// section-relative offset of the resource's data and must be turned into an
// RVA by the linker (IMAGE_REL_AMD64_ADDR32NB against the section symbol).
func buildSection(resources []resource) (section []byte, relocs []uint32, err error) {
	if len(resources) == 0 {
		return nil, nil, fmt.Errorf("no resources")
	}

	// Windows looks entries up by binary search, so every directory must
	// be sorted by id in ascending order.
	res := append([]resource(nil), resources...)
	sort.SliceStable(res, func(i, j int) bool {
		a, b := res[i], res[j]
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		return a.Lang < b.Lang
	})
	for i := 1; i < len(res); i++ {
		a, b := res[i-1], res[i]
		if a.Type == b.Type && a.ID == b.ID && a.Lang == b.Lang {
			return nil, nil, fmt.Errorf("duplicate resource type %d id %d lang %#04x", a.Type, a.ID, a.Lang)
		}
	}

	// Group the sorted leaves: types[i] holds its ids, each of which holds
	// a run of leaves (one per language).
	type idGroup struct {
		id     uint16
		leaves []int // indices into res
	}
	type typeGroup struct {
		typ uint16
		ids []idGroup
	}
	var types []typeGroup
	numIDs := 0
	for i, r := range res {
		if len(types) == 0 || types[len(types)-1].typ != r.Type {
			types = append(types, typeGroup{typ: r.Type})
		}
		t := &types[len(types)-1]
		if len(t.ids) == 0 || t.ids[len(t.ids)-1].id != r.ID {
			t.ids = append(t.ids, idGroup{id: r.ID})
			numIDs++
		}
		g := &t.ids[len(t.ids)-1]
		g.leaves = append(g.leaves, i)
	}

	// Offsets of the four regions.
	typeDirsOff := resDirSize + resDirEntrySize*len(types)
	idDirsOff := typeDirsOff + resDirSize*len(types) + resDirEntrySize*numIDs
	dataEntriesOff := idDirsOff + resDirSize*numIDs + resDirEntrySize*len(res)
	rawOff := alignUp(dataEntriesOff+resDataSize*len(res), resDataAlign)

	rawOffsets := make([]int, len(res))
	total := rawOff
	for i, r := range res {
		rawOffsets[i] = total
		total = alignUp(total+len(r.Data), resDataAlign)
	}
	if uint64(total) >= resSubdirectory {
		return nil, nil, fmt.Errorf("resources too large: %d bytes", total)
	}

	sec := make([]byte, total)
	le := binary.LittleEndian
	// putDir writes an IMAGE_RESOURCE_DIRECTORY header with n id entries
	// (and no named entries) and returns the offset of its first entry.
	putDir := func(off, n int) int {
		// Characteristics, TimeDateStamp, MajorVersion and MinorVersion
		// stay zero, which also keeps the output reproducible.
		le.PutUint16(sec[off+12:], 0)         // NumberOfNamedEntries
		le.PutUint16(sec[off+14:], uint16(n)) // NumberOfIdEntries
		return off + resDirSize
	}
	putEntry := func(off int, id uint16, target uint32) {
		le.PutUint32(sec[off:], uint32(id))
		le.PutUint32(sec[off+4:], target)
	}

	rootEntry := putDir(0, len(types))
	typeDir := typeDirsOff
	idDir := idDirsOff
	dataEntry := dataEntriesOff
	for ti, t := range types {
		putEntry(rootEntry+resDirEntrySize*ti, t.typ, resSubdirectory|uint32(typeDir))
		typeEntry := putDir(typeDir, len(t.ids))
		typeDir = typeEntry + resDirEntrySize*len(t.ids)
		for ii, g := range t.ids {
			putEntry(typeEntry+resDirEntrySize*ii, g.id, resSubdirectory|uint32(idDir))
			idEntry := putDir(idDir, len(g.leaves))
			idDir = idEntry + resDirEntrySize*len(g.leaves)
			for li, leaf := range g.leaves {
				r := res[leaf]
				putEntry(idEntry+resDirEntrySize*li, r.Lang, uint32(dataEntry))

				// IMAGE_RESOURCE_DATA_ENTRY.
				le.PutUint32(sec[dataEntry:], uint32(rawOffsets[leaf])) // OffsetToData (relocated)
				le.PutUint32(sec[dataEntry+4:], uint32(len(r.Data)))    // Size
				le.PutUint32(sec[dataEntry+8:], 0)                      // CodePage
				le.PutUint32(sec[dataEntry+12:], 0)                     // Reserved
				relocs = append(relocs, uint32(dataEntry))
				copy(sec[rawOffsets[leaf]:], r.Data)
				dataEntry += resDataSize
			}
		}
	}
	return sec, relocs, nil
}

// alignUp rounds n up to the next multiple of align (a power of two).
func alignUp(n, align int) int {
	return (n + align - 1) &^ (align - 1)
}
