package config

import (
	"encoding/json"
	"testing"
)

func TestDefaultIsValid(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRoundTrip(t *testing.T) {
	raw, _ := json.Marshal(Default())
	if _, err := Parse(raw); err != nil {
		t.Fatal(err)
	}
}

func TestKeys(t *testing.T) {
	for _, ok := range []string{"a", "ctrl+shift+t", "super", "super+l", "f12", "alt+left", "ctrl+pagedown"} {
		if err := ValidKeys(ok); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "ctrl+ctrl+a", "a+b", "hyper+a", "ctrl+"} {
		if ValidKeys(bad) == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestErrorPath(t *testing.T) {
	c := Default()
	c.Profiles[1].Pages[0].Buttons[2].Icon = "nope"
	err := c.Validate()
	e, ok := err.(*Error)
	if !ok || e.Path != "profiles[1].pages[0].buttons[2].icon" {
		t.Fatalf("got %v", err)
	}
}
