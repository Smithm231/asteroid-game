package input

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// keyAliases maps convenient lower-case spellings to Ebitengine key names.
var keyAliases = map[string]Key{
	"esc":    ebiten.KeyEscape,
	"return": ebiten.KeyEnter,
	"ctrl":   ebiten.KeyControl,
	"del":    ebiten.KeyDelete,
	"ins":    ebiten.KeyInsert,
	"up":     ebiten.KeyArrowUp,
	"down":   ebiten.KeyArrowDown,
	"left":   ebiten.KeyArrowLeft,
	"right":  ebiten.KeyArrowRight,
	"pgup":   ebiten.KeyPageUp,
	"pgdn":   ebiten.KeyPageDown,
	"0":      ebiten.KeyDigit0,
	"1":      ebiten.KeyDigit1,
	"2":      ebiten.KeyDigit2,
	"3":      ebiten.KeyDigit3,
	"4":      ebiten.KeyDigit4,
	"5":      ebiten.KeyDigit5,
	"6":      ebiten.KeyDigit6,
	"7":      ebiten.KeyDigit7,
	"8":      ebiten.KeyDigit8,
	"9":      ebiten.KeyDigit9,
}

// keysByName maps the lower-cased Ebitengine name of every key, plus the
// aliases, to its key code.
var keysByName = buildKeyNames()

func buildKeyNames() map[string]Key {
	m := make(map[string]Key, numKeys+len(keyAliases))
	for k := Key(0); int(k) < numKeys; k++ {
		if name := k.String(); name != "" {
			m[strings.ToLower(name)] = k
		}
	}
	for name, k := range keyAliases {
		m[name] = k
	}
	return m
}

// ParseKey resolves a key name to its key code. Names are Ebitengine's own
// (the constant name without the "Key" prefix: "R", "Escape", "ShiftLeft",
// "F1", "Digit1", "ArrowLeft", "Enter", "Space", "Tab", "Backspace",
// "NumpadAdd", ...) and are matched case-insensitively.
//
// The aliases "Esc", "Return", "Ctrl", "Del", "Ins", "Up", "Down", "Left",
// "Right", "PgUp", "PgDn" and the bare digits "0".."9" are accepted too.
// "Shift", "Control", "Alt" and "Meta" resolve to the combined modifier keys.
func ParseKey(name string) (Key, error) {
	k, ok := keysByName[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return 0, fmt.Errorf("input: unknown key name %q", name)
	}
	return k, nil
}

// KeyName returns the canonical name of k, the one ParseKey resolves back to
// k, or "" for a value that is not a key.
func KeyName(k Key) string {
	return k.String()
}

// ParseButton resolves a script button name ("left", "right" or "middle",
// case-insensitive). The empty string means the left button.
func ParseButton(name string) (Button, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "left":
		return Left, nil
	case "right":
		return Right, nil
	case "middle":
		return Middle, nil
	}
	return Left, fmt.Errorf("input: unknown mouse button %q", name)
}
