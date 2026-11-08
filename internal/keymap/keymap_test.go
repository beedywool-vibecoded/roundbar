package keymap

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestEveryLayoutTypesASCII(t *testing.T) {
	for _, l := range All() {
		for r := rune(0x20); r <= 0x7E; r++ {
			if l.Strokes(r) == nil {
				t.Errorf("%s: can't type %q", l.ID, r)
			}
		}
		for _, r := range "\n\t" {
			if l.Strokes(r) == nil {
				t.Errorf("%s: can't type %q", l.ID, r)
			}
		}
		for _, r := range "abcdefghijklmnopqrstuvwxyz" {
			if s := l.Strokes(r); len(s) != 1 || s[0].Mods != 0 {
				t.Errorf("%s: letter %c must be one unmodified key (shortcuts use it), got %v", l.ID, r, s)
			}
		}
	}
}

func strokes(ss ...uint8) []Stroke {
	var out []Stroke
	for i := 0; i+1 < len(ss); i += 2 {
		out = append(out, Stroke{ss[i], ss[i+1]})
	}
	return out
}

const (
	S  = ModShift
	G  = ModAltGr
	SG = ModShift | ModAltGr
)

func TestKnownCharacters(t *testing.T) {
	cases := []struct {
		layout string
		r      rune
		want   []Stroke
	}{
		{"us", 'a', strokes(0, 0x04)},
		{"us", '@', strokes(S, 0x1F)},
		{"us", '|', strokes(S, 0x31)},
		{"us", '<', strokes(S, 0x36)}, // not the ISO key: ANSI keyboards lack it
		{"de", '<', strokes(0, 0x64)},
		{"fr", 'a', strokes(0, 0x14)},
		{"fr", 'm', strokes(0, 0x33)},
		{"fr", '1', strokes(S, 0x1E)},
		{"fr", 'é', strokes(0, 0x1F)},
		{"fr", '@', strokes(G, 0x27)},
		{"fr", '€', strokes(G, 0x08)},
		{"fr", 'ê', strokes(0, 0x2F, 0, 0x08)}, // dead circumflex, e
		{"fr", 'Ë', strokes(S, 0x2F, S, 0x08)}, // dead diaeresis, E
		{"de", 'z', strokes(0, 0x1C)},
		{"de", 'y', strokes(0, 0x1D)},
		{"de", 'ü', strokes(0, 0x2F)},
		{"de", 'ß', strokes(0, 0x2D)},
		{"de", '@', strokes(G, 0x14)},
		{"de", '{', strokes(G, 0x24)},
		{"de", '|', strokes(G, 0x64)},
		{"de", 'é', strokes(0, 0x2E, 0, 0x08)}, // dead acute, e
		{"de", '^', strokes(0, 0x35, 0, 0x2C)}, // dead circumflex, space
		{"de", '´', strokes(0, 0x2E, 0, 0x2E)}, // dead acute twice
		{"gb", '"', strokes(S, 0x1F)},
		{"gb", '@', strokes(S, 0x34)},
		{"gb", '#', strokes(0, 0x31)},
		{"gb", '\\', strokes(0, 0x64)},
		{"gb", '£', strokes(S, 0x20)},
		{"es", 'ñ', strokes(0, 0x33)},
		{"es", 'á', strokes(0, 0x34, 0, 0x04)},
		{"it", 'è', strokes(0, 0x2F)},
		{"it", '@', strokes(G, 0x14)},
		{"be", '@', strokes(G, 0x1F)},
		{"ch", 'ü', strokes(0, 0x2F)},
		{"ch", 'è', strokes(S, 0x2F)},
		{"ch-fr", 'è', strokes(0, 0x2F)},
		{"se", 'å', strokes(0, 0x2F)},
		{"no", 'ø', strokes(0, 0x33)},
		{"dk", 'æ', strokes(0, 0x33)},
		{"us-intl", '\'', strokes(G, 0x34)},
		{"us-intl", '"', strokes(SG, 0x34)},
		{"us-intl", 'é', strokes(G, 0x08)},
		{"us-intl", 'ê', strokes(S, 0x23, 0, 0x08)},
	}
	for _, c := range cases {
		l := Get(c.layout)
		if l == nil {
			t.Fatalf("no layout %s", c.layout)
		}
		if got := l.Strokes(c.r); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s %q: got %v, want %v", c.layout, c.r, got, c.want)
		}
	}
}

func TestFromXKB(t *testing.T) {
	for _, c := range [][3]string{
		{"fr", "", "fr"}, {"fr", "oss", "fr"}, {"ch", "fr", "ch-fr"}, {"ch", "", "ch"}, {"us", "intl", "us-intl"},
		{"gb", "extd", "gb"}, {"xx", "", ""}, {"", "", ""},
	} {
		if got := FromXKB(c[0], c[1]); got != c[2] {
			t.Errorf("FromXKB(%q, %q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// The generated tables must match shared/keymaps.json: run `go run ./tools/gen-keymaps`.
func TestGeneratedUpToDate(t *testing.T) {
	for path, want := range map[string][]byte{CHeaderPath: CHeader(), TSPath: TS()} {
		got, err := os.ReadFile(filepath.Join("..", "..", path))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s is stale: run go run ./tools/gen-keymaps", path)
		}
	}
}

// Every two-keystroke sequence must be one the Linux Compose table knows, or the PC
// would type something else. Skipped where libX11's table isn't installed.
func TestDeadSequencesInCompose(t *testing.T) {
	f, err := os.Open("/usr/share/X11/locale/en_US.UTF-8/Compose")
	if err != nil {
		t.Skip("no Compose table")
	}
	defer f.Close()
	compose := map[string]string{}
	re := regexp.MustCompile(`^(<dead_\w+>\s+<\w+>)\s*:\s*"((?:[^"\\]|\\.)*)"`)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if m := re.FindStringSubmatch(sc.Text()); m != nil {
			res := m[2]
			if res == `\"` {
				res = `"`
			}
			compose[regexp.MustCompile(`\s+`).ReplaceAllString(m[1], " ")] = res
		}
	}
	for _, l := range All() {
		name := map[Stroke]string{}
		for n, s := range l.dead {
			name[s] = n
		}
		for _, r := range l.Runes() {
			s := l.Strokes(r)
			if len(s) != 2 {
				continue
			}
			d, ok := name[s[0]]
			if !ok {
				t.Errorf("%s %q: first stroke isn't a dead key", l.ID, r)
				continue
			}
			var second string
			switch {
			case s[1] == Stroke{0, hidSpace}:
				second = "space"
			case s[1] == s[0]:
				second = "dead_" + d
			default:
				for b := 'A'; b <= 'z'; b++ {
					if bs := l.Strokes(b); len(bs) == 1 && bs[0] == s[1] && (b <= 'Z' || b >= 'a') {
						second = string(b)
					}
				}
			}
			seq := fmt.Sprintf("<dead_%s> <%s>", d, second)
			if got := compose[seq]; got != string(r) {
				t.Errorf("%s %q: %s gives %q in Compose", l.ID, r, seq, got)
			}
		}
	}
}
