package input

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// specScript is the list of examples from the presentation specification,
// verbatim, as one script.
const specScript = `[{"wait":30},{"move":[640,360]},{"click":[100,200]},{"click":[100,200],"button":"right"},` +
	`{"dblclick":[10,20]},{"drag":[1,2,3,4],"frames":20},{"wheel":-3},{"key":"R"},{"key":"Escape"},` +
	`{"hold":"ShiftLeft","frames":10},{"text":"hello"},{"shot":"name.png"},{"quit":true}]`

func TestParseScriptSpecExamples(t *testing.T) {
	got, err := ParseScript([]byte(specScript))
	if err != nil {
		t.Fatalf("ParseScript: %v", err)
	}
	want := []Step{
		{Wait: 30},
		{Move: []float32{640, 360}},
		{Click: []float32{100, 200}},
		{Click: []float32{100, 200}, Button: "right"},
		{DblClick: []float32{10, 20}},
		{Drag: []float32{1, 2, 3, 4}, Frames: 20},
		{Wheel: -3},
		{Key: "R"},
		{Key: "Escape"},
		{Hold: "ShiftLeft", Frames: 10},
		{Text: "hello"},
		{Shot: "name.png"},
		{Quit: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed steps differ:\n got %+v\nwant %+v", got, want)
	}

	// The constructors build the very same steps.
	built := []Step{
		Wait(30),
		MoveTo(640, 360),
		ClickAt(100, 200),
		ClickWith(Right, 100, 200),
		DblClickAt(10, 20),
		DragTo(1, 2, 3, 4, 20),
		Scroll(-3),
		Tap(ebiten.KeyR),
		Tap(ebiten.KeyEscape),
		Hold(ebiten.KeyShiftLeft, 10),
		Type("hello"),
		Shot("name.png"),
		Quit(),
	}
	if !reflect.DeepEqual(built, want) {
		t.Fatalf("constructed steps differ:\n got %+v\nwant %+v", built, want)
	}

	// And they encode back to exactly the specified JSON: the field tags
	// are the contract.
	enc, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(enc) != specScript {
		t.Errorf("re-encoded script differs:\n got %s\nwant %s", enc, specScript)
	}
}

func TestParseScriptExtensions(t *testing.T) {
	src := `[
		{"note": "glide, then a held press"},
		{"move": [10, 20], "frames": 12},
		{"press": [30, 40], "button": "middle"},
		{"release": [50, 60], "button": "middle"},
		{"key": "Backspace", "frames": 40, "note": "auto-repeat"},
		{"hold": "Ctrl"},
		{"click": [1.5, 2.25], "button": "LEFT"},
		{}
	]`
	got, err := ParseScript([]byte(src))
	if err != nil {
		t.Fatalf("ParseScript: %v", err)
	}
	want := []Step{
		{Note: "glide, then a held press"},
		{Move: []float32{10, 20}, Frames: 12},
		{Press: []float32{30, 40}, Button: "middle"},
		{Release: []float32{50, 60}, Button: "middle"},
		{Key: "Backspace", Frames: 40, Note: "auto-repeat"},
		{Hold: "Ctrl"},
		{Click: []float32{1.5, 2.25}, Button: "LEFT"},
		{},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parsed steps differ:\n got %+v\nwant %+v", got, want)
	}

	built := []Step{
		GlideTo(10, 20, 12),
		PressAt(Middle, 30, 40),
		ReleaseAt(Middle, 50, 60),
		TapFor(ebiten.KeyBackspace, 40),
		DragWith(Right, 1, 2, 3, 4, 5),
		Hold(ebiten.KeyControlLeft, -3),
	}
	wantBuilt := []Step{
		{Move: []float32{10, 20}, Frames: 12},
		{Press: []float32{30, 40}, Button: "middle"},
		{Release: []float32{50, 60}, Button: "middle"},
		{Key: "Backspace", Frames: 40},
		{Drag: []float32{1, 2, 3, 4}, Frames: 5, Button: "right"},
		{Hold: "ControlLeft"},
	}
	if !reflect.DeepEqual(built, wantBuilt) {
		t.Fatalf("constructed steps differ:\n got %+v\nwant %+v", built, wantBuilt)
	}
	if err := Validate(built); err != nil {
		t.Errorf("constructed steps do not validate: %v", err)
	}
}

func TestParseScriptEmpty(t *testing.T) {
	for _, src := range []string{`[]`, `null`, " [ ] \n"} {
		steps, err := ParseScript([]byte(src))
		if err != nil || len(steps) != 0 {
			t.Errorf("ParseScript(%q) = %v, %v; want no steps and no error", src, steps, err)
		}
	}
}

func TestParseScriptErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string // substring of the error
	}{
		{"not json", `[{"wait":`, "parsing script"},
		{"not a list", `{"wait":30}`, "parsing script"},
		{"empty input", ``, "parsing script"},
		{"trailing data", `[{"wait":1}] [{"wait":2}]`, "unexpected data"},
		{"unknown field", `[{"clik":[1,2]}]`, "unknown field"},
		{"wrong type", `[{"wait":"long"}]`, "parsing script"},
		{"two actions", `[{"click":[1,2],"key":"R"}]`, "more than one action in a step (click, key)"},
		{"two actions, second step", `[{"wait":1},{"wait":3,"quit":true}]`, "step 2"},
		{"move with one number", `[{"move":[640]}]`, `"move" needs 2 numbers, got 1`},
		{"move with none", `[{"move":[]}]`, `"move" needs 2 numbers, got 0`},
		{"click with three numbers", `[{"click":[1,2,3]}]`, `"click" needs 2 numbers`},
		{"dblclick with one number", `[{"dblclick":[1]}]`, `"dblclick" needs 2 numbers`},
		{"drag with two numbers", `[{"drag":[1,2]}]`, `"drag" needs 4 numbers, got 2`},
		{"press with four numbers", `[{"press":[1,2,3,4]}]`, `"press" needs 2 numbers`},
		{"release with one number", `[{"release":[1]}]`, `"release" needs 2 numbers`},
		{"negative wait", `[{"wait":-1}]`, "must not be negative"},
		{"negative frames", `[{"drag":[1,2,3,4],"frames":-5}]`, "must not be negative"},
		{"unknown key", `[{"key":"Hyper"}]`, `unknown key name "Hyper"`},
		{"unknown hold key", `[{"hold":"LeftShift"}]`, `unknown key name "LeftShift"`},
		{"unknown button", `[{"click":[1,2],"button":"fourth"}]`, `unknown mouse button "fourth"`},
		{"button on a key", `[{"key":"R","button":"right"}]`, `"button" only applies`},
		{"button alone", `[{"button":"right"}]`, `"button" only applies`},
		{"frames on a click", `[{"click":[1,2],"frames":3}]`, `"frames" only applies`},
		{"frames alone", `[{"frames":10}]`, `"frames" only applies`},
		{"frames on a wait", `[{"wait":5,"frames":10}]`, `"frames" only applies`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			steps, err := ParseScript([]byte(tc.src))
			if err == nil {
				t.Fatalf("ParseScript accepted %s: %+v", tc.src, steps)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention %q", err, tc.want)
			}
			if steps != nil {
				t.Errorf("steps = %+v alongside an error", steps)
			}
		})
	}
}

func TestValidateNamesTheStep(t *testing.T) {
	err := Validate([]Step{Wait(1), ClickAt(1, 2), {Key: "Bogus"}})
	if err == nil || !strings.Contains(err.Error(), "step 3") {
		t.Errorf("Validate error = %v, want it to name step 3", err)
	}
	if err := Validate(nil); err != nil {
		t.Errorf("Validate(nil) = %v", err)
	}
	if err := (Step{Hold: "Shift", Frames: 4}).Validate(); err != nil {
		t.Errorf("a combined modifier key should be accepted: %v", err)
	}
}

func TestLoadScript(t *testing.T) {
	dir := t.TempDir()

	good := filepath.Join(dir, "good.json")
	if err := os.WriteFile(good, []byte(specScript), 0o644); err != nil {
		t.Fatal(err)
	}
	steps, err := LoadScript(good)
	if err != nil {
		t.Fatalf("LoadScript: %v", err)
	}
	if len(steps) != 13 || steps[0].Wait != 30 || !steps[12].Quit {
		t.Errorf("unexpected steps: %+v", steps)
	}

	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte(`[{"key":"Nope"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadScript(bad); err == nil || !strings.Contains(err.Error(), "bad.json") || !strings.Contains(err.Error(), "Nope") {
		t.Errorf("LoadScript(bad) error = %v, want file name and cause", err)
	}

	if _, err := LoadScript(filepath.Join(dir, "missing.json")); err == nil || !strings.Contains(err.Error(), "loading script") {
		t.Errorf("LoadScript(missing) error = %v", err)
	}
}
