package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// COFF constants (winnt.h), named as in the PE/COFF specification.
const (
	imageFileMachineAMD64 = 0x8664

	imageScnCntInitializedData = 0x00000040
	imageScnMemRead            = 0x40000000

	// imageRelAMD64Addr32NB is a 32-bit address without image base: the
	// linker stores the target's RVA.
	imageRelAMD64Addr32NB = 0x0003

	imageSymClassStatic = 3

	coffFileHeaderSize    = 20
	coffSectionHeaderSize = 40
	coffRelocationSize    = 10
	coffSymbolSize        = 18
)

// rsrcSectionName is the name of the resource section and of its section
// symbol. It fits the 8-byte short-name field, so the string table stays
// empty.
const rsrcSectionName = ".rsrc"

// rsrcSectionFlags are the characteristics of the resource section.
const rsrcSectionFlags = imageScnCntInitializedData | imageScnMemRead

// buildObject wraps a .rsrc section in a COFF object file for AMD64.
//
// The file consists of the file header, a single section header, the section
// data, one IMAGE_REL_AMD64_ADDR32NB relocation per entry of relocs (each a
// section offset whose 32-bit value is a section-relative offset to be turned
// into an RVA), the symbol table holding the ".rsrc" section symbol, and an
// empty string table. No time stamp is written, so the result depends only on
// the arguments.
func buildObject(section []byte, relocs []uint32) ([]byte, error) {
	if len(relocs) > 0xFFFF {
		// More would need IMAGE_SCN_LNK_NRELOC_OVFL, which is never
		// required for a handful of icons.
		return nil, fmt.Errorf("too many relocations: %d", len(relocs))
	}
	for _, off := range relocs {
		if int(off)+4 > len(section) {
			return nil, fmt.Errorf("relocation at %#x is outside the %d byte section", off, len(section))
		}
	}

	const numSymbols = 2 // the section symbol and its auxiliary record
	rawOff := coffFileHeaderSize + coffSectionHeaderSize
	relocOff := rawOff + len(section)
	symOff := relocOff + coffRelocationSize*len(relocs)

	var b bytes.Buffer
	var writeErr error
	put := func(v any) {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil && writeErr == nil {
			writeErr = err
		}
	}
	var name [8]byte
	copy(name[:], rsrcSectionName)

	// IMAGE_FILE_HEADER.
	put(struct {
		Machine              uint16
		NumberOfSections     uint16
		TimeDateStamp        uint32
		PointerToSymbolTable uint32
		NumberOfSymbols      uint32
		SizeOfOptionalHeader uint16
		Characteristics      uint16
	}{
		Machine:              imageFileMachineAMD64,
		NumberOfSections:     1,
		PointerToSymbolTable: uint32(symOff),
		NumberOfSymbols:      numSymbols,
	})

	// IMAGE_SECTION_HEADER.
	put(struct {
		Name                 [8]byte
		VirtualSize          uint32
		VirtualAddress       uint32
		SizeOfRawData        uint32
		PointerToRawData     uint32
		PointerToRelocations uint32
		PointerToLinenumbers uint32
		NumberOfRelocations  uint16
		NumberOfLinenumbers  uint16
		Characteristics      uint32
	}{
		Name:                 name,
		SizeOfRawData:        uint32(len(section)),
		PointerToRawData:     uint32(rawOff),
		PointerToRelocations: uint32(relocOff),
		NumberOfRelocations:  uint16(len(relocs)),
		Characteristics:      rsrcSectionFlags,
	})

	b.Write(section)

	// IMAGE_RELOCATION, all against symbol 0 (the section symbol).
	for _, off := range relocs {
		put(struct {
			VirtualAddress   uint32
			SymbolTableIndex uint32
			Type             uint16
		}{VirtualAddress: off, Type: imageRelAMD64Addr32NB})
	}

	// IMAGE_SYMBOL: the section symbol ...
	put(struct {
		Name               [8]byte
		Value              uint32
		SectionNumber      int16
		Type               uint16
		StorageClass       uint8
		NumberOfAuxSymbols uint8
	}{
		Name:               name,
		SectionNumber:      1,
		StorageClass:       imageSymClassStatic,
		NumberOfAuxSymbols: 1,
	})
	// ... followed by its auxiliary section definition record.
	put(struct {
		Length              uint32
		NumberOfRelocations uint16
		NumberOfLinenumbers uint16
		CheckSum            uint32
		Number              uint16
		Selection           uint8
		_                   [3]byte
	}{
		Length:              uint32(len(section)),
		NumberOfRelocations: uint16(len(relocs)),
	})

	// String table: just its own 4-byte length field.
	put(uint32(4))

	if writeErr != nil {
		return nil, writeErr
	}
	if want := symOff + coffSymbolSize*numSymbols + 4; b.Len() != want {
		return nil, fmt.Errorf("internal error: object is %d bytes, expected %d", b.Len(), want)
	}
	return b.Bytes(), nil
}
