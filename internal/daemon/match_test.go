package daemon

import (
	"testing"

	"github.com/beedywool-vibecoded/roundbar/internal/desktop"
)

func TestMatches(t *testing.T) {
	ff := desktop.Focus{ID: "org.mozilla.firefox", Class: "firefox"}
	game := desktop.Focus{ID: "", Class: "steam_app_1091500"}
	cases := []struct {
		pattern string
		f       desktop.Focus
		want    bool
	}{
		{"firefox", ff, true},
		{"org.mozilla.firefox", ff, true},
		{"org.mozilla", ff, false},
		{"org.mozilla.*", ff, true},
		{"*fox", ff, true},
		{"steam_app_*", game, true},
		{"steam_app_*", ff, false},
		{"*", desktop.Focus{}, false}, // an empty focus never matches a wildcard
		{"code", desktop.Focus{ID: "code"}, true},
	}
	for _, c := range cases {
		if got := matches(c.pattern, c.f); got != c.want {
			t.Errorf("matches(%q, %+v) = %v, want %v", c.pattern, c.f, got, c.want)
		}
	}
}
