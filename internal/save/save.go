// Package save stores and loads games and the player's settings.
//
// A save file is gzip-compressed text: the first line is a small JSON header
// (Meta) that can be listed without decoding the game, the second line is the
// world itself. Files live in the user's configuration directory, which on
// Windows is %AppData%\Drifthold.
package save

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"drifthold/internal/model"
)

// Ext is the file extension of save files.
const Ext = ".drift"

// AutosaveName is the name of the rolling autosave.
const AutosaveName = "autosave"

// Meta is the header of a save file: what the load screen shows.
type Meta struct {
	// Name is the save's display name (the file name without extension).
	Name string
	// SavedAt is the wall-clock time the game was saved.
	SavedAt time.Time
	// Version is the save-format version of the world that follows.
	Version int
	// Hold is the name of the human player's Hold.
	Hold string
	// Cycle is the in-game date.
	Cycle int
	// Rocks, Rivals and Difficulty describe the game's setup.
	Rocks      int
	Rivals     int
	Difficulty model.Difficulty
	// Colonies and Pop summarise the player's position.
	Colonies int
	Pop      int
	// Over is true for a finished game.
	Over bool
}

// Entry is one save file found on disk.
type Entry struct {
	Meta
	// Path is the full path of the file.
	Path string
}

// root is the directory under which saves and settings live; tests point it
// somewhere else.
var root = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot find the settings folder: %w", err)
	}
	return filepath.Join(dir, "Drifthold"), nil
}

// SetRoot overrides the directory used for saves and settings. It is meant
// for tests and for a portable install; an empty dir restores the default.
func SetRoot(dir string) {
	if dir == "" {
		root = func() (string, error) {
			d, err := os.UserConfigDir()
			if err != nil {
				return "", fmt.Errorf("cannot find the settings folder: %w", err)
			}
			return filepath.Join(d, "Drifthold"), nil
		}
		return
	}
	root = func() (string, error) { return dir, nil }
}

// Dir returns the directory holding save files, creating it if needed.
func Dir() (string, error) {
	r, err := root()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(r, "saves")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create the saves folder: %w", err)
	}
	return dir, nil
}

// CleanName turns what a player typed into a safe file name: letters, digits,
// spaces, dashes and underscores, at most 40 characters. It returns "" when
// nothing usable is left.
func CleanName(name string) string {
	var sb strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == ' ', r == '-', r == '_':
			sb.WriteRune(r)
		}
		if sb.Len() >= 40 {
			break
		}
	}
	return strings.TrimSpace(sb.String())
}

// PathFor returns the path of the save with the given display name.
func PathFor(name string) (string, error) {
	clean := CleanName(name)
	if clean == "" {
		return "", errors.New("Please give the save a name")
	}
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, clean+Ext), nil
}

// MetaOf builds the header for a world about to be saved under name.
func MetaOf(w *model.World, name string) Meta {
	m := Meta{
		Name: name, SavedAt: time.Now(), Version: w.Version, Cycle: w.Cycle(),
		Rocks: len(w.Rocks), Rivals: len(w.Factions) - 1, Difficulty: w.Settings.Difficulty, Over: w.GameOver,
	}
	if h := w.Human(); h != nil {
		m.Hold = h.Name
		m.Colonies = w.ColonyCount(h.ID)
		m.Pop = int(w.TotalPop(h.ID))
	}
	return m
}

// Write saves the world under the given display name and returns the path.
// The file is written to a temporary name first, so an interrupted save never
// destroys an existing one.
func Write(name string, w *model.World) (string, error) {
	path, err := PathFor(name)
	if err != nil {
		return "", err
	}
	return path, WriteFile(path, w, MetaOf(w, CleanName(name)))
}

// WriteFile saves the world with the given header to an explicit path.
func WriteFile(path string, w *model.World, meta Meta) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("cannot write the save: %w", err)
	}
	zw := gzip.NewWriter(f)
	enc := json.NewEncoder(zw)
	err = enc.Encode(meta)
	if err == nil {
		err = enc.Encode(w)
	}
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("cannot write the save: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("cannot write the save: %w", err)
	}
	return nil
}

func open(path string) (*os.File, *gzip.Reader, *bufio.Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, nil, err
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		f.Close()
		return nil, nil, nil, err
	}
	return f, zr, bufio.NewReaderSize(zr, 1<<16), nil
}

// ReadMeta reads only the header of a save file.
func ReadMeta(path string) (Meta, error) {
	var m Meta
	f, zr, br, err := open(path)
	if err != nil {
		return m, err
	}
	defer f.Close()
	defer zr.Close()
	line, err := br.ReadBytes('\n')
	if err != nil {
		return m, err
	}
	err = json.Unmarshal(line, &m)
	return m, err
}

// Read loads a save file. The world is checked for consistency before it is
// returned, so a damaged or foreign file yields an error rather than a game
// that misbehaves later.
func Read(path string) (*model.World, Meta, error) {
	var m Meta
	bad := func(err error) (*model.World, Meta, error) {
		return nil, m, fmt.Errorf("This save cannot be loaded: %w", err)
	}
	f, zr, br, err := open(path)
	if err != nil {
		return bad(err)
	}
	defer f.Close()
	defer zr.Close()
	dec := json.NewDecoder(br)
	if err := dec.Decode(&m); err != nil {
		return bad(err)
	}
	if m.Version != model.SaveVersion {
		return bad(fmt.Errorf("it was made by a different version of the game (format %d, expected %d)", m.Version, model.SaveVersion))
	}
	w := &model.World{}
	if err := dec.Decode(w); err != nil {
		return bad(err)
	}
	if err := w.Validate(); err != nil {
		return bad(err)
	}
	for _, fac := range w.Factions {
		fac.RecomputeMods(w.Settings.Difficulty)
	}
	return w, m, nil
}

// List returns the save files on disk, newest first. Files that cannot be
// read are skipped.
func List() ([]Entry, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []Entry
	for _, de := range files {
		if de.IsDir() || !strings.HasSuffix(de.Name(), Ext) {
			continue
		}
		path := filepath.Join(dir, de.Name())
		m, err := ReadMeta(path)
		if err != nil {
			continue
		}
		m.Name = strings.TrimSuffix(de.Name(), Ext)
		out = append(out, Entry{Meta: m, Path: path})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SavedAt.After(out[j].SavedAt) })
	return out, nil
}

// Delete removes a save file.
func Delete(path string) error {
	if !strings.HasSuffix(path, Ext) {
		return errors.New("Not a save file")
	}
	return os.Remove(path)
}

// Settings are the player's preferences, kept between sessions.
type Settings struct {
	MasterVolume float64
	SfxVolume    float64
	MusicVolume  float64
	// UIScale multiplies the interface size (0.75 to 2); 0 means automatic.
	UIScale    float64
	Fullscreen bool
	// AutosaveMinutes is the autosave interval in real minutes; 0 turns it off.
	AutosaveMinutes int
	// EdgeScroll pans the view when the cursor touches the window edge.
	EdgeScroll bool
	// LastSetup remembers the options of the most recent new game.
	LastSetup model.GameSettings
}

// DefaultSettings returns the settings of a fresh install.
func DefaultSettings() Settings {
	return Settings{
		MasterVolume: 0.8, SfxVolume: 0.8, MusicVolume: 0.6,
		AutosaveMinutes: 5, LastSetup: model.DefaultSettings(1),
	}
}

func settingsPath() (string, error) {
	r, err := root()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(r, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(r, "settings.json"), nil
}

// LoadSettings reads the player's settings; anything missing or unreadable
// falls back to the defaults.
func LoadSettings() Settings {
	s := DefaultSettings()
	path, err := settingsPath()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if json.Unmarshal(data, &s) != nil {
		return DefaultSettings()
	}
	clamp := func(v *float64, lo, hi float64) {
		if !(*v >= lo) {
			*v = lo
		}
		if *v > hi {
			*v = hi
		}
	}
	clamp(&s.MasterVolume, 0, 1)
	clamp(&s.SfxVolume, 0, 1)
	clamp(&s.MusicVolume, 0, 1)
	if s.UIScale != 0 {
		clamp(&s.UIScale, 0.75, 2)
	}
	if s.AutosaveMinutes < 0 || s.AutosaveMinutes > 60 {
		s.AutosaveMinutes = 5
	}
	s.LastSetup = s.LastSetup.Normalized()
	return s
}

// SaveSettings writes the player's settings.
func SaveSettings(s Settings) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
