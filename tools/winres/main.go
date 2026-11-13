// Command winres generates the Windows resource object for Drifthold: the
// application icon, the application manifest and the version information.
//
// Usage:
//
//	go run ./tools/winres -out cmd/drifthold/rsrc_windows_amd64.syso -version 1.0.0
//	go run ./tools/winres -check Drifthold.exe [-version 1.0.0]
//
// The output is a COFF object file for AMD64 containing a single .rsrc
// section. The Go linker automatically links every *_windows_amd64.syso file
// found in a main package's directory into Windows builds of that package,
// so placing the file next to cmd/drifthold's sources is all that is needed;
// it has no effect on other platforms.
//
// The object holds:
//
//   - RT_ICON resources with the emblem from internal/emblem at every size in
//     emblem.Sizes (classic bitmaps below 256 pixels, PNG at 256) and the
//     RT_GROUP_ICON directory with id 1 that ties them together,
//   - RT_MANIFEST id 1: asInvoker execution level, Windows 10/11
//     compatibility, per-monitor (V2) DPI awareness and long path awareness,
//   - RT_VERSION id 1: product name, file description, internal name,
//     original file name, the versions given by -version and, when
//     -copyright is given, a copyright notice.
//
// The tool has no dependencies beyond the module itself: it writes the COFF
// file, the resource directory tree and every resource format directly. The
// output is reproducible; it contains no time stamps.
//
// With -check the tool instead reads the resources back from an object file
// or from a linked executable, verifies their structure (see inspect.go) and
// prints a summary. If -version is given as well, the embedded version must
// match it.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"drifthold/internal/emblem"
)

// Product identity recorded in the version information and the manifest.
const (
	productName      = "Drifthold"
	fileDescription  = "Drifthold"
	internalName     = "drifthold"
	originalFilename = "Drifthold.exe"
)

// options are the user-selectable parts of the generated resources.
type options struct {
	Version   string // e.g. "1.0.0"
	Copyright string // LegalCopyright; omitted from the resources when empty
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("winres: ")

	out := flag.String("out", "", "path of the .syso object file to write")
	check := flag.String("check", "", "verify and summarise the resources of this .syso or .exe instead of generating")
	version := flag.String("version", "0.0.0", "file and product version, one to four dot-separated numbers")
	copyright := flag.String("copyright", "", "copyright notice (LegalCopyright); omitted when empty")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: winres -out FILE.syso [-version V] [-copyright TEXT]\n       winres -check FILE [-version V]\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 0 || (*out == "") == (*check == "") {
		flag.Usage()
		os.Exit(2)
	}

	if *check != "" {
		wantVersion := ""
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "version" {
				wantVersion = *version
			}
		})
		if err := runCheck(os.Stdout, *check, wantVersion); err != nil {
			log.Fatalf("%s: %v", *check, err)
		}
		return
	}

	obj, err := generate(options{Version: *version, Copyright: *copyright})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, obj, 0o644); err != nil {
		log.Fatal(err)
	}
}

// generate builds the complete resource object file.
func generate(o options) ([]byte, error) {
	res, err := driftholdResources(o)
	if err != nil {
		return nil, err
	}
	section, relocs, err := buildSection(res)
	if err != nil {
		return nil, err
	}
	return buildObject(section, relocs)
}

// driftholdResources assembles every resource of the game's executable.
func driftholdResources(o options) ([]resource, error) {
	version, pre, err := parseVersion(o.Version)
	if err != nil {
		return nil, err
	}

	icons, err := emblemIcons()
	if err != nil {
		return nil, err
	}
	res, err := iconResources(icons, 1)
	if err != nil {
		return nil, err
	}

	// The strings show the version as given ("1.0.0"); the numeric fields
	// and the manifest always carry four parts ("1.0.0.0").
	shown := strings.TrimPrefix(strings.TrimSpace(o.Version), "v")
	vi := versionInfo{
		Version:    version,
		PreRelease: pre,
		Strings: []versionString{
			{"FileDescription", fileDescription},
			{"FileVersion", shown},
			{"InternalName", internalName},
			{"OriginalFilename", originalFilename},
			{"ProductName", productName},
			{"ProductVersion", shown},
		},
	}
	if c := strings.TrimSpace(o.Copyright); c != "" {
		// Kept in alphabetical order, as resource compilers emit them.
		vi.Strings = slices.Insert(vi.Strings, 3, versionString{"LegalCopyright", c})
	}
	versionData, err := vi.encode()
	if err != nil {
		return nil, err
	}

	res = append(res,
		resource{Type: rtVersion, ID: 1, Lang: langEnUS, Data: versionData},
		resource{Type: rtManifest, ID: 1, Lang: langEnUS, Data: manifestXML(version)},
	)
	return res, nil
}

// runCheck verifies the resources in the object or executable at path and
// writes a summary to w. A non-empty wantVersion must match the embedded
// version.
func runCheck(w io.Writer, path, wantVersion string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	ld, err := loadResources(bytes.NewReader(data))
	if err != nil {
		return err
	}
	rep, err := inspect(ld.Leaves)
	if err != nil {
		return err
	}
	if err := checkDrifthold(rep, wantVersion); err != nil {
		return err
	}

	s := ld.Section
	size := s.VirtualSize // zero in an object file
	if size == 0 {
		size = s.Size
	}
	fmt.Fprintf(w, "%s: %s, section %s at RVA %#x, %d bytes, %d resources\n",
		filepath.Base(path), ld.Kind, s.Name, s.VirtualAddress, size, len(ld.Leaves))
	for _, l := range ld.Leaves {
		fmt.Fprintf(w, "  type %2d id %d lang %#04x: %6d bytes at RVA %#x\n", l.Type, l.ID, l.Lang, len(l.Data), l.DataRVA)
	}
	fmt.Fprintf(w, "icon group %d:\n", iconGroupID)
	for _, ic := range rep.Icons {
		fmt.Fprintf(w, "  %3dx%-3d 32-bit %s, RT_ICON %d, %d bytes\n", ic.Size, ic.Size, ic.Format, ic.ID, ic.Bytes)
	}
	m := rep.Manifest
	fmt.Fprintf(w, "manifest: %s %s, level %s, supportedOS %s, dpiAware %s, dpiAwareness %s, longPathAware %s\n",
		m.Name, m.Version, m.ExecutionLevel, strings.Join(m.SupportedOS, " "), m.DPIAware, m.DPIAwareness, m.LongPathAware)
	v := rep.Version
	fmt.Fprintf(w, "version: file %s, product %s, table %s\n", dotted(v.Fixed.FileVersion()), dotted(v.Fixed.ProductVersion()), v.Table)
	for _, k := range sortedKeys(v.Strings) {
		fmt.Fprintf(w, "  %-16s %q\n", k, v.Strings[k])
	}
	fmt.Fprintln(w, "ok")
	return nil
}

// checkDrifthold verifies that decoded resources are the ones this tool
// generates for Drifthold.
func checkDrifthold(rep *report, wantVersion string) error {
	var sizes []int
	for _, ic := range rep.Icons {
		sizes = append(sizes, ic.Size)
		wantFormat := "DIB"
		if ic.Size >= pngIconSize {
			wantFormat = "PNG"
		}
		if ic.Format != wantFormat {
			return fmt.Errorf("%d px icon is stored as %s, want %s", ic.Size, ic.Format, wantFormat)
		}
	}
	if !slices.Equal(sizes, emblem.Sizes()) {
		return fmt.Errorf("icon sizes are %v, want %v", sizes, emblem.Sizes())
	}

	m := rep.Manifest
	switch {
	case m.Name != productName:
		return fmt.Errorf("manifest is for %q, want %q", m.Name, productName)
	case m.ExecutionLevel != "asInvoker":
		return fmt.Errorf("manifest execution level is %q, want asInvoker", m.ExecutionLevel)
	case !slices.Contains(m.SupportedOS, supportedOSWindows10):
		return fmt.Errorf("manifest does not declare Windows 10/11 support: %v", m.SupportedOS)
	case m.DPIAware != "true/pm":
		return fmt.Errorf("manifest dpiAware is %q, want true/pm", m.DPIAware)
	case m.DPIAwareness != "PerMonitorV2,PerMonitor":
		return fmt.Errorf("manifest dpiAwareness is %q, want PerMonitorV2,PerMonitor", m.DPIAwareness)
	case m.LongPathAware != "true":
		return fmt.Errorf("manifest longPathAware is %q, want true", m.LongPathAware)
	}

	v := rep.Version
	want := map[string]string{
		"FileDescription":  fileDescription,
		"InternalName":     internalName,
		"OriginalFilename": originalFilename,
		"ProductName":      productName,
	}
	for k, w := range want {
		if got := v.Strings[k]; got != w {
			return fmt.Errorf("version string %s is %q, want %q", k, got, w)
		}
	}
	if _, ok := v.Strings["CompanyName"]; ok {
		return fmt.Errorf("version information has a CompanyName; Drifthold ships without one")
	}
	if v.Fixed.FileVersion() != v.Fixed.ProductVersion() || v.Strings["FileVersion"] != v.Strings["ProductVersion"] {
		return fmt.Errorf("file and product versions differ")
	}
	if dotted(v.Fixed.FileVersion()) != m.Version {
		return fmt.Errorf("manifest version %s differs from file version %s", m.Version, dotted(v.Fixed.FileVersion()))
	}
	if v.Fixed.FileOS != vsOSNTWindows32 || v.Fixed.FileType != vsTypeApp {
		return fmt.Errorf("fixed file info has OS %#x and type %#x, want a Win32 application", v.Fixed.FileOS, v.Fixed.FileType)
	}
	if len(v.Translations) != 1 || v.Translations[0] != [2]uint16{langEnUS, codePageUnicode} {
		return fmt.Errorf("translations are %v, want US English in Unicode", v.Translations)
	}

	if wantVersion != "" {
		num, _, err := parseVersion(wantVersion)
		if err != nil {
			return err
		}
		shown := strings.TrimPrefix(strings.TrimSpace(wantVersion), "v")
		if v.Fixed.FileVersion() != num || v.Strings["FileVersion"] != shown {
			return fmt.Errorf("embedded version is %s (%q), want %s (%q)",
				dotted(v.Fixed.FileVersion()), v.Strings["FileVersion"], dotted(num), shown)
		}
	}
	return nil
}
