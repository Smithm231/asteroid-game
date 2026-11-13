package main

import (
	"bytes"
	"debug/pe"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildHello builds testdata/hello for windows/amd64 together with a freshly
// generated resource object, in a scratch module of its own, and returns the
// path of the executable and the object's bytes.
//
// The same can be done by hand, inside the repository:
//
//	go run ./tools/winres -out tools/winres/testdata/hello/rsrc_windows_amd64.syso -version 1.2.3
//	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o hello.exe ./tools/winres/testdata/hello
//	go run ./tools/winres -check hello.exe -version 1.2.3
func buildHello(t *testing.T, o options) (exe string, obj []byte) {
	t.Helper()
	if testing.Short() {
		t.Skip("builds a Windows executable; skipped in short mode")
	}
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the go tool is not available")
	}

	dir := t.TempDir()
	sources, err := filepath.Glob(filepath.Join("testdata", "hello", "*.go"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no sources in testdata/hello (err %v)", err)
	}
	for _, src := range sources {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(src)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module hello\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	obj = mustGenerate(t, o)
	if err := os.WriteFile(filepath.Join(dir, "rsrc_windows_amd64.syso"), obj, 0o644); err != nil {
		t.Fatal(err)
	}

	exe = filepath.Join(dir, "hello.exe")
	cmd := exec.Command(goTool, "build", "-o", exe, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0", "GOWORK=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build with the generated .syso failed: %v\n%s", err, out)
	}
	return exe, obj
}

// TestLinkedExecutable links a real Windows executable with the generated
// object and verifies, through the PE resource data directory, that every
// resource arrived intact and that every RVA lies inside the .rsrc section.
func TestLinkedExecutable(t *testing.T) {
	exe, obj := buildHello(t, testOptions)

	f, err := pe.Open(exe)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	oh, ok := f.OptionalHeader.(*pe.OptionalHeader64)
	if !ok {
		t.Fatalf("optional header is %T, want PE32+", f.OptionalHeader)
	}
	dir := oh.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_RESOURCE] // index 2
	sec := f.Section(".rsrc")
	if sec == nil {
		t.Fatal("the executable has no .rsrc section: the linker ignored the .syso")
	}
	if dir.VirtualAddress != sec.VirtualAddress || dir.Size == 0 || dir.Size > sec.VirtualSize {
		t.Errorf("resource data directory = RVA %#x, %d bytes; .rsrc section = RVA %#x, %d bytes",
			dir.VirtualAddress, dir.Size, sec.VirtualAddress, sec.VirtualSize)
	}
	if want := uint32(pe.IMAGE_SCN_CNT_INITIALIZED_DATA | pe.IMAGE_SCN_MEM_READ); sec.Characteristics != want {
		t.Errorf(".rsrc characteristics = %#x, want %#x (read-only initialised data)", sec.Characteristics, want)
	}

	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := loadResources(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("resources of the executable: %v", err)
	}
	original, err := loadResources(bytes.NewReader(obj))
	if err != nil {
		t.Fatal(err)
	}
	if linked.Kind != "executable" || len(linked.Leaves) != len(original.Leaves) {
		t.Fatalf("executable has %d resources (%s), the object %d", len(linked.Leaves), linked.Kind, len(original.Leaves))
	}
	lo, hi := sec.VirtualAddress, sec.VirtualAddress+sec.VirtualSize
	for i, l := range linked.Leaves {
		o := original.Leaves[i]
		if l.Type != o.Type || l.ID != o.ID || l.Lang != o.Lang {
			t.Errorf("resource %d is type %d id %d lang %#x, in the object type %d id %d lang %#x",
				i, l.Type, l.ID, l.Lang, o.Type, o.ID, o.Lang)
		}
		if !bytes.Equal(l.Data, o.Data) {
			t.Errorf("resource type %d id %d: data differs from the object", l.Type, l.ID)
		}
		// The linker must have turned the section offset into an RVA.
		if want := sec.VirtualAddress + o.DataRVA; l.DataRVA != want {
			t.Errorf("resource type %d id %d: RVA %#x, want section RVA + offset = %#x", l.Type, l.ID, l.DataRVA, want)
		}
		if l.DataRVA < lo || l.DataRVA+uint32(len(l.Data)) > hi {
			t.Errorf("resource type %d id %d: RVA range %#x+%d is outside .rsrc (%#x..%#x)", l.Type, l.ID, l.DataRVA, len(l.Data), lo, hi)
		}
	}

	rep, err := inspect(linked.Leaves)
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if err := checkDrifthold(rep, testOptions.Version); err != nil {
		t.Errorf("checkDrifthold: %v", err)
	}
	if got := rep.Version.Strings["LegalCopyright"]; got != testOptions.Copyright {
		t.Errorf("LegalCopyright = %q, want %q", got, testOptions.Copyright)
	}
	if !strings.Contains(rep.Manifest.Text, `level="asInvoker"`) {
		t.Errorf("manifest text is damaged:\n%s", rep.Manifest.Text)
	}

	var out bytes.Buffer
	if err := runCheck(&out, exe, testOptions.Version); err != nil {
		t.Errorf("runCheck on the executable: %v", err)
	}
	if !strings.Contains(out.String(), "executable, section .rsrc") {
		t.Errorf("runCheck output:\n%s", out.String())
	}
}

// TestHelloUnderWine runs the linked executable under Wine, where the test
// program asks the Win32 API for its own icon, manifest settings and version
// strings. Starting Wine is slow and touches the Wine prefix, so the test
// only runs on request:
//
//	WINRES_WINE=1 WINEPREFIX=/some/prefix go test -run Wine ./tools/winres
func TestHelloUnderWine(t *testing.T) {
	if os.Getenv("WINRES_WINE") == "" {
		t.Skip("set WINRES_WINE=1 to run the test program under Wine")
	}
	wine, err := exec.LookPath("wine")
	if err != nil {
		t.Skip("wine is not installed")
	}
	exe, _ := buildHello(t, testOptions)
	cmd := exec.Command(wine, exe)
	cmd.Env = append(os.Environ(), "WINEDEBUG=-all")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("wine %s: %v\n%s", exe, err, out)
	}
	for _, want := range []string{
		"hello from the Drifthold resource test program",
		"icon  16 px: RT_ICON 1",
		"icon 256 px: RT_ICON 9",
		`manifest setting dpiAwareness = "PerMonitorV2,PerMonitor"`,
		"version: file 1.2.3.0, product 1.2.3.0",
		`version: ProductName = "Drifthold"`,
		`version: FileVersion = "1.2.3"`,
		"selfcheck: ok",
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("output lacks %q", want)
		}
	}
	if t.Failed() {
		t.Logf("output:\n%s", out)
	}
}
