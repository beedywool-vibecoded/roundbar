package daemon

import (
	"encoding/json"
	"testing"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name              string
		pc, board, synced string
		pcDefault         bool
		want              syncAction
		conflict          bool
	}{
		{"blank board", "A", "", "", false, syncPush, false},
		{"in sync", "A", "A", "A", false, syncNone, false},
		{"in sync, never recorded (upgrade)", "A", "A", "", false, syncNone, false},
		{"fresh PC adopts the board", "D", "B", "", true, syncPull, false},
		{"edited on another PC", "A", "B", "A", false, syncPull, false},
		{"edited here while unplugged", "A2", "A", "A", false, syncPush, false},
		{"both changed: board wins, backup", "A2", "B", "A", false, syncPull, true},
		{"own setup meets a friend's board", "A", "F", "", false, syncPull, true},
	}
	for _, c := range cases {
		a, conflict := decide(c.pc, c.board, c.synced, c.pcDefault)
		if a != c.want || conflict != c.conflict {
			t.Errorf("%s: got %v/%v, want %v/%v", c.name, a, conflict, c.want, c.conflict)
		}
	}
}

func TestIsDefault(t *testing.T) {
	raw, _ := json.MarshalIndent(config.Default(), "", "  ")
	if !isDefault(append(raw, '\n')) {
		t.Fatal("default config not recognized")
	}
	c := config.Default()
	c.Device.Brightness = 42
	raw, _ = json.MarshalIndent(c, "", "  ")
	if isDefault(raw) {
		t.Fatal("edited config taken for the default")
	}
}
