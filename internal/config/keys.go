package config

import (
	"fmt"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/shared"
)

func IconExists(name string) bool {
	_, ok := shared.Icons[name]
	return ok
}

var namedKeys = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`enter esc tab space backspace delete insert home end pageup pagedown up down left right
		minus equal leftbracket rightbracket backslash semicolon quote grave comma period slash capslock printscreen`) {
		namedKeys[k] = true
	}
	for i := 1; i <= 24; i++ {
		namedKeys[fmt.Sprintf("f%d", i)] = true
	}
	for c := 'a'; c <= 'z'; c++ {
		namedKeys[string(c)] = true
	}
	for c := '0'; c <= '9'; c++ {
		namedKeys[string(c)] = true
	}
}

// ValidKeys checks a combo like "ctrl+shift+t": modifiers then exactly one key.
func ValidKeys(s string) error {
	if s == "" {
		return fmt.Errorf("empty key combo")
	}
	parts := strings.Split(s, "+")
	seen := map[string]bool{}
	for i, p := range parts {
		last := i == len(parts)-1
		switch {
		case p == "ctrl" || p == "shift" || p == "alt" || p == "super":
			if seen[p] {
				return fmt.Errorf("modifier %q repeated", p)
			}
			seen[p] = true
			if last && len(parts) > 1 {
				return fmt.Errorf("combo ends with a modifier")
			}
		case namedKeys[p]:
			if !last {
				return fmt.Errorf("only one non-modifier key allowed")
			}
		default:
			return fmt.Errorf("unknown key %q", p)
		}
	}
	return nil
}

func ValidText(s string) error {
	if s == "" || len([]rune(s)) > 64 {
		return fmt.Errorf("must be 1..64 chars")
	}
	for _, r := range s {
		if (r < 0x20 || r > 0x7e) && !strings.ContainsRune("éèàçù\n\t", r) {
			return fmt.Errorf("character %q can't be typed", r)
		}
	}
	return nil
}
