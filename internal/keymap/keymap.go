// Package keymap turns characters into HID keystrokes for the host keyboard layouts
// described in shared/keymaps.json (see docs/spec.md, "Keyboard layouts"). The same
// resolution feeds config validation, the generated firmware table and the
// configurator, so all three agree on what each layout can type.
package keymap

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/shared"
	"golang.org/x/text/unicode/norm"
)

// HID keyboard modifier bits (left shift, right alt = AltGr).
const (
	ModShift = 0x02
	ModAltGr = 0x40
)

// Stroke is one key press: modifiers + HID usage (page 0x07).
type Stroke struct{ Mods, Key uint8 }

// Layout is one entry of shared/keymaps.json, resolved.
type Layout struct {
	ID, Name, XKB string
	chars         map[rune][]Stroke
	dead          map[string]Stroke // dead key name -> its stroke
}

// keyOrder lists the typing keys by XKB name with the HID usage of that position.
// Searching in this order (per level) makes the choice between duplicates stable.
var keyOrder = []struct {
	name string
	hid  uint8
}{
	{"TLDE", 0x35}, {"AE01", 0x1E}, {"AE02", 0x1F}, {"AE03", 0x20}, {"AE04", 0x21}, {"AE05", 0x22}, {"AE06", 0x23},
	{"AE07", 0x24}, {"AE08", 0x25}, {"AE09", 0x26}, {"AE10", 0x27}, {"AE11", 0x2D}, {"AE12", 0x2E},
	{"AD01", 0x14}, {"AD02", 0x1A}, {"AD03", 0x08}, {"AD04", 0x15}, {"AD05", 0x17}, {"AD06", 0x1C}, {"AD07", 0x18},
	{"AD08", 0x0C}, {"AD09", 0x12}, {"AD10", 0x13}, {"AD11", 0x2F}, {"AD12", 0x30},
	{"AC01", 0x04}, {"AC02", 0x16}, {"AC03", 0x07}, {"AC04", 0x09}, {"AC05", 0x0A}, {"AC06", 0x0B}, {"AC07", 0x0D},
	{"AC08", 0x0E}, {"AC09", 0x0F}, {"AC10", 0x33}, {"AC11", 0x34}, {"BKSL", 0x31}, {"LSGT", 0x64},
	{"AB01", 0x1D}, {"AB02", 0x1B}, {"AB03", 0x06}, {"AB04", 0x19}, {"AB05", 0x05}, {"AB06", 0x11}, {"AB07", 0x10},
	{"AB08", 0x36}, {"AB09", 0x37}, {"AB10", 0x38},
}

var levelMods = []uint8{0, ModShift, ModAltGr, ModShift | ModAltGr}

// Dead keys in the order they are tried, with the Linux Compose results:
// combining mark (dead key + letter), dead key + space, dead key twice.
type deadKey struct {
	name             string
	mark, space, dbl rune
}

var deadKeys = []deadKey{
	{"acute", 0x301, '\'', '´'},
	{"grave", 0x300, '`', '`'},
	{"circumflex", 0x302, '^', '^'},
	{"tilde", 0x303, '~', '~'},
	{"diaeresis", 0x308, '"', '¨'},
	{"abovering", 0x30A, '°', '°'},
	{"cedilla", 0x327, '¸', '¸'},
	{"caron", 0x30C, 'ˇ', 'ˇ'},
}

const (
	hidEnter = 0x28
	hidTab   = 0x2B
	hidSpace = 0x2C
)

type entry struct {
	ID   string              `json:"id"`
	Name string              `json:"name"`
	XKB  string              `json:"xkb"`
	Keys map[string][]string `json:"keys"`
}

var layouts = func() []*Layout {
	ls, err := parse(shared.KeymapsJSON)
	if err != nil {
		panic(err)
	}
	return ls
}()

func parse(data []byte) ([]*Layout, error) {
	var es []entry
	if err := json.Unmarshal(data, &es); err != nil {
		return nil, err
	}
	var out []*Layout
	seen := map[string]bool{}
	for _, e := range es {
		if e.ID == "" || e.ID == "auto" || seen[e.ID] {
			return nil, fmt.Errorf("keymaps: bad or repeated id %q", e.ID)
		}
		seen[e.ID] = true
		l, err := build(e)
		if err != nil {
			return nil, fmt.Errorf("keymaps %s: %w", e.ID, err)
		}
		out = append(out, l)
	}
	return out, nil
}

func build(e entry) (*Layout, error) {
	known := map[string]bool{}
	for _, k := range keyOrder {
		known[k.name] = true
	}
	for k, lv := range e.Keys {
		if !known[k] {
			return nil, fmt.Errorf("unknown key %s", k)
		}
		if len(lv) > len(levelMods) {
			return nil, fmt.Errorf("%s: more than 4 levels", k)
		}
	}
	l := &Layout{ID: e.ID, Name: e.Name, XKB: e.XKB, chars: map[rune][]Stroke{
		' ': {{0, hidSpace}}, '\n': {{0, hidEnter}}, '\t': {{0, hidTab}},
	}}
	dead := map[string]Stroke{}
	l.dead = dead
	// The ISO key next to left shift (LSGT) is tried after the other keys of the same
	// modifier group (none/shift, then AltGr): ANSI keyboards don't have it, so < on US
	// is typed as shift+comma, while \ on UK stays on its own key.
	type pass struct {
		lvl  int
		lsgt bool
	}
	for _, p := range []pass{{0, false}, {1, false}, {0, true}, {1, true}, {2, false}, {3, false}, {2, true}, {3, true}} {
		lvl, mods := p.lvl, levelMods[p.lvl]
		for _, k := range keyOrder {
			if (k.name == "LSGT") != p.lsgt {
				continue
			}
			lv := e.Keys[k.name]
			if lvl >= len(lv) || lv[lvl] == "" {
				continue
			}
			s := Stroke{mods, k.hid}
			if name, ok := strings.CutPrefix(lv[lvl], "dead_"); ok {
				if !slices.ContainsFunc(deadKeys, func(d deadKey) bool { return d.name == name }) {
					return nil, fmt.Errorf("%s: unknown dead key %s", k.name, lv[lvl])
				}
				if _, ok := dead[name]; !ok {
					dead[name] = s
				}
				continue
			}
			r := []rune(lv[lvl])
			if len(r) != 1 || r[0] < 0x20 || r[0] > 0xFFFF {
				return nil, fmt.Errorf("%s: level %d must be one BMP character, got %q", k.name, lvl+1, lv[lvl])
			}
			if _, ok := l.chars[r[0]]; !ok {
				l.chars[r[0]] = []Stroke{s}
			}
		}
	}
	// Spacing accents: dead key + space, or twice when space gives an ASCII stand-in.
	for _, d := range deadKeys {
		s, ok := dead[d.name]
		if !ok {
			continue
		}
		if _, ok := l.chars[d.space]; !ok {
			l.chars[d.space] = []Stroke{s, {0, hidSpace}}
		}
		if _, ok := l.chars[d.dbl]; !ok {
			l.chars[d.dbl] = []Stroke{s, s}
		}
	}
	// Accented letters: dead key + base letter.
	for _, d := range deadKeys {
		s, ok := dead[d.name]
		if !ok {
			continue
		}
		for _, b := range "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" {
			bs := l.chars[b]
			if len(bs) != 1 {
				continue
			}
			c := []rune(norm.NFC.String(string([]rune{b, d.mark})))
			if len(c) != 1 || c[0] > 0x17F { // Latin-1 and Latin Extended-A: all in the Compose table
				continue
			}
			if _, ok := l.chars[c[0]]; !ok {
				l.chars[c[0]] = []Stroke{s, bs[0]}
			}
		}
	}
	return l, nil
}

// All returns the layouts in file order.
func All() []*Layout { return layouts }

// Get returns the layout with this id, or nil.
func Get(id string) *Layout {
	for _, l := range layouts {
		if l.ID == id {
			return l
		}
	}
	return nil
}

// Strokes returns the keystrokes that type r, or nil.
func (l *Layout) Strokes(r rune) []Stroke { return l.chars[r] }

// Runes returns every character the layout can type, sorted.
func (l *Layout) Runes() []rune {
	rs := make([]rune, 0, len(l.chars))
	for r := range l.chars {
		rs = append(rs, r)
	}
	slices.Sort(rs)
	return rs
}

var anyLayout = func() map[rune]bool {
	m := map[rune]bool{}
	for _, l := range layouts {
		for r := range l.chars {
			m[r] = true
		}
	}
	return m
}()

// Typeable reports whether at least one layout can type r. Used to validate text
// actions, whose layout may only be known at run time ("auto").
func Typeable(r rune) bool { return anyLayout[r] }

// FromXKB maps an XKB layout and variant to a layout id: the exact variant if we
// have it, else the base layout (most variants keep the main characters in place).
func FromXKB(layout, variant string) string {
	layout, variant = strings.TrimSpace(layout), strings.TrimSpace(variant)
	if layout == "" {
		return ""
	}
	if variant != "" {
		for _, l := range layouts {
			if l.XKB == layout+"("+variant+")" {
				return l.ID
			}
		}
	}
	for _, l := range layouts {
		if l.XKB == layout {
			return l.ID
		}
	}
	return ""
}
