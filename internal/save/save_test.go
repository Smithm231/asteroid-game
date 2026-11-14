package save

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"drifthold/internal/model"
	"drifthold/internal/sim"
)

func TestSaveRoundTripAndList(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")
	w, err := sim.NewGame(model.DefaultSettings(5))
	if err != nil {
		t.Fatal(err)
	}
	sim.New(w).Run(900)
	path, err := Write("  My first/hold!  ", w)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "My firsthold"+Ext {
		t.Errorf("file name %q", filepath.Base(path))
	}
	got, meta, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(w)
	b, _ := json.Marshal(got)
	if string(a) != string(b) {
		t.Error("the loaded world differs from the saved one")
	}
	if meta.Cycle != w.Cycle() || meta.Hold != w.Human().Name || meta.Rivals != 2 || meta.Pop == 0 {
		t.Errorf("meta %+v", meta)
	}
	// A loaded game continues exactly like the original.
	sim.New(w).Run(600)
	sim.New(got).Run(600)
	a, _ = json.Marshal(w)
	b, _ = json.Marshal(got)
	if string(a) != string(b) {
		t.Error("the loaded game diverged from the original")
	}
	if _, err := Write("second", w); err != nil {
		t.Fatal(err)
	}
	dir, _ := Dir()
	os.WriteFile(filepath.Join(dir, "broken"+Ext), []byte("not a save"), 0o644)
	list, err := List()
	if err != nil || len(list) != 2 || list[0].Name != "second" {
		t.Fatalf("List = %v, %v", list, err)
	}
	if _, _, err := Read(filepath.Join(dir, "broken"+Ext)); err == nil {
		t.Error("a broken file must not load")
	}
	if err := Delete(list[0].Path); err != nil {
		t.Fatal(err)
	}
	if list, _ = List(); len(list) != 1 {
		t.Errorf("after delete: %d saves", len(list))
	}
	if _, err := Write("  ///  ", w); err == nil {
		t.Error("an empty name must be refused")
	}
}

func TestSettings(t *testing.T) {
	SetRoot(t.TempDir())
	defer SetRoot("")
	if s := LoadSettings(); s.MasterVolume != 0.8 || s.AutosaveMinutes != 5 {
		t.Errorf("defaults: %+v", s)
	}
	s := DefaultSettings()
	s.MusicVolume, s.UIScale, s.Fullscreen = 0.25, 1.5, true
	s.LastSetup.Rivals = 3
	if err := SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings()
	if got.MusicVolume != 0.25 || got.UIScale != 1.5 || !got.Fullscreen || got.LastSetup.Rivals != 3 {
		t.Errorf("round trip: %+v", got)
	}
	r, _ := root()
	os.WriteFile(filepath.Join(r, "settings.json"), []byte(`{"MasterVolume": 9, "UIScale": 0.1}`), 0o644)
	if got = LoadSettings(); got.MasterVolume != 1 || got.UIScale != 0.75 {
		t.Errorf("clamping: %+v", got)
	}
}
