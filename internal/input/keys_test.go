package input

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestParseKey(t *testing.T) {
	tests := []struct {
		name string
		want Key
	}{
		{"R", ebiten.KeyR},
		{"r", ebiten.KeyR},
		{"Escape", ebiten.KeyEscape},
		{"ShiftLeft", ebiten.KeyShiftLeft},
		{"shiftleft", ebiten.KeyShiftLeft},
		{"ControlRight", ebiten.KeyControlRight},
		{"F1", ebiten.KeyF1},
		{"F12", ebiten.KeyF12},
		{"Digit1", ebiten.KeyDigit1},
		{"ArrowLeft", ebiten.KeyArrowLeft},
		{"Enter", ebiten.KeyEnter},
		{"Space", ebiten.KeySpace},
		{"Tab", ebiten.KeyTab},
		{"Backspace", ebiten.KeyBackspace},
		{"NumpadAdd", ebiten.KeyNumpadAdd},
		{" PageDown ", ebiten.KeyPageDown},
		// Aliases.
		{"Esc", ebiten.KeyEscape},
		{"Return", ebiten.KeyEnter},
		{"Ctrl", ebiten.KeyControl},
		{"Up", ebiten.KeyArrowUp},
		{"7", ebiten.KeyDigit7},
		// Combined modifier keys keep their identity here.
		{"Shift", ebiten.KeyShift},
		{"Alt", ebiten.KeyAlt},
	}
	for _, tc := range tests {
		got, err := ParseKey(tc.name)
		if err != nil {
			t.Errorf("ParseKey(%q): unexpected error %v", tc.name, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseKey(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestParseKeyUnknown(t *testing.T) {
	for _, name := range []string{"", "Nope", "KeyR", "Shift Left", "F25"} {
		if k, err := ParseKey(name); err == nil {
			t.Errorf("ParseKey(%q) = %v, want an error", name, k)
		}
	}
}

// Every key Ebitengine names must survive a round trip through its name.
func TestKeyNameRoundTrip(t *testing.T) {
	named := 0
	for k := Key(0); int(k) < numKeys; k++ {
		name := KeyName(k)
		if name == "" {
			continue
		}
		named++
		got, err := ParseKey(name)
		if err != nil {
			t.Errorf("ParseKey(KeyName(%d) = %q): %v", int(k), name, err)
			continue
		}
		if got != k {
			t.Errorf("ParseKey(%q) = %d, want %d", name, int(got), int(k))
		}
	}
	if named < 100 {
		t.Errorf("only %d keys have names; the key table looks wrong", named)
	}
	if KeyName(Key(-1)) != "" || KeyName(Key(numKeys)) != "" {
		t.Errorf("KeyName of an out-of-range key should be empty")
	}
}

func TestParseButton(t *testing.T) {
	tests := []struct {
		name    string
		want    Button
		wantErr bool
	}{
		{"", Left, false},
		{"left", Left, false},
		{"Left", Left, false},
		{"right", Right, false},
		{"RIGHT", Right, false},
		{"middle", Middle, false},
		{"centre", Left, true},
		{"4", Left, true},
	}
	for _, tc := range tests {
		got, err := ParseButton(tc.name)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseButton(%q) error = %v, wantErr %v", tc.name, err, tc.wantErr)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseButton(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
	for _, b := range []Button{Left, Right, Middle} {
		got, err := ParseButton(b.String())
		if err != nil || got != b {
			t.Errorf("ParseButton(%q) = %v, %v; want %v", b.String(), got, err, b)
		}
	}
	if Button(7).String() != "unknown" {
		t.Errorf("Button(7).String() = %q", Button(7).String())
	}
}
