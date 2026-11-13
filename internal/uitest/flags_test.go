package uitest

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"drifthold/internal/input"
)

func parse(t *testing.T, args ...string) (Options, error) {
	t.Helper()
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return ParseFlags(fs, args)
}

func TestParseFlagsDefaults(t *testing.T) {
	o, err := parse(t)
	if err != nil {
		t.Fatal(err)
	}
	want := Options{W: 1600, H: 900, Scale: 1, OutDir: "shots"}
	if !reflect.DeepEqual(o, want) {
		t.Errorf("defaults = %+v, want %+v", o, want)
	}
	if o.Script != nil {
		t.Error("Script must be nil without -script so that demos use their built-in script")
	}
}

func TestParseFlagsValues(t *testing.T) {
	o, err := parse(t, "-out", "some/dir", "-scale", "1.5", "-w", "1280", "-h", "720", "-frames", "90")
	if err != nil {
		t.Fatal(err)
	}
	want := Options{W: 1280, H: 720, Scale: 1.5, OutDir: "some/dir", Frames: 90}
	if !reflect.DeepEqual(o, want) {
		t.Errorf("options = %+v, want %+v", o, want)
	}
	if pw, ph := o.PixelSize(); pw != 1920 || ph != 1080 {
		t.Errorf("pixel size = %d×%d, want 1920×1080", pw, ph)
	}
}

func TestParseFlagsScript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.json")
	if err := os.WriteFile(path, []byte(`[{"click":[100,200]},{"shot":"a.png"},{"quit":true}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := parse(t, "-script", path, "-scale=2")
	if err != nil {
		t.Fatal(err)
	}
	want := []input.Step{input.ClickAt(100, 200), input.Shot("a.png"), input.Quit()}
	if !reflect.DeepEqual(o.Script, want) {
		t.Errorf("script = %+v, want %+v", o.Script, want)
	}

	// An empty script file is still "a script was given".
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err = parse(t, "-script", empty)
	if err != nil {
		t.Fatal(err)
	}
	if o.Script == nil || len(o.Script) != 0 {
		t.Errorf("script = %#v, want an empty non-nil script", o.Script)
	}
}

func TestParseFlagsErrors(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`[{"key":"Nope"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"zero scale", []string{"-scale", "0"}, "-scale"},
		{"negative scale", []string{"-scale", "-2"}, "-scale"},
		{"huge scale", []string{"-scale", "64"}, "-scale"},
		{"not a number", []string{"-scale", "big"}, "invalid value"},
		{"zero width", []string{"-w", "0"}, "-w and -h"},
		{"negative height", []string{"-h", "-5"}, "-w and -h"},
		{"negative frames", []string{"-frames", "-1"}, "-frames"},
		{"unknown flag", []string{"-colour", "red"}, "not defined"},
		{"stray argument", []string{"-scale", "2", "extra"}, "unexpected argument"},
		{"missing script", []string{"-script", filepath.Join(dir, "none.json")}, "-script"},
		{"invalid script", []string{"-script", bad}, "Nope"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o, err := parse(t, tc.args...)
			if err == nil {
				t.Fatalf("ParseFlags(%v) = %+v, want an error", tc.args, o)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
		})
	}
}

// Demo programs may define their own flags on the same set first.
func TestParseFlagsAlongsideOtherFlags(t *testing.T) {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	page := fs.String("page", "all", "which page to show")
	o, err := ParseFlags(fs, []string{"-page", "buttons", "-out", "o"})
	if err != nil {
		t.Fatal(err)
	}
	if *page != "buttons" || o.OutDir != "o" {
		t.Errorf("page %q, out %q", *page, o.OutDir)
	}
}
