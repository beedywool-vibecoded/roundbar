package config

import (
	"strings"
	"testing"
)

// radial returns the first radial page of the default config, for mutation.
func radial(c *Config) *Page {
	for i := range c.Profiles {
		for j := range c.Profiles[i].Pages {
			if c.Profiles[i].Pages[j].Type == "radial" {
				return &c.Profiles[i].Pages[j]
			}
		}
	}
	panic("no radial page in the default config")
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name, path string
		mut        func(c *Config)
	}{
		{"version", "version", func(c *Config) { c.Version = 2 }},
		{"seed", "theme.seed", func(c *Config) { c.Theme.Seed = "red" }},
		{"brightness", "device.brightness", func(c *Config) { c.Device.Brightness = 1 }},
		{"rotation", "device.rotation", func(c *Config) { c.Device.Rotation = 45 }},
		{"edge tap", "device.edgeDoubleTap", func(c *Config) { c.Device.EdgeDoubleTap = "dance" }},
		{"no default", "profiles", func(c *Config) { c.Profiles = c.Profiles[1:] }},
		{"dup profile", "profiles[1].id", func(c *Config) { c.Profiles[1].ID = c.Profiles[0].ID }},
		{"uppercase match", "profiles[1].match[0]", func(c *Config) { c.Profiles[1].Match = []string{"Firefox"} }},
		{"bad wildcard", "profiles[1].match[0]", func(c *Config) { c.Profiles[1].Match = []string{"steam_app_[0-9"} }},
		{"too many pages", "profiles[0].pages", func(c *Config) {
			for len(c.Profiles[0].Pages) <= 8 {
				pg := c.Profiles[0].Pages[0]
				pg.ID += "x"
				c.Profiles[0].Pages = append(c.Profiles[0].Pages, pg)
			}
		}},
		{"dup page", "profiles[0].pages", func(c *Config) {
			c.Profiles[0].Pages = append(c.Profiles[0].Pages, c.Profiles[0].Pages[0])
		}},
		{"obs request", ".request", func(c *Config) {
			radial(c).Buttons[0].Tap = &Action{Type: "obs", Request: "DeleteEverything"}
		}},
		{"obs scene", ".scene", func(c *Config) {
			radial(c).Buttons[0].Tap = &Action{Type: "obs", Request: "SetScene"}
		}},
		{"timer presets", ".minutes", func(c *Config) {
			c.Profiles[0].Pages = append(c.Profiles[0].Pages[:1], Page{ID: "t", Type: "timer", Minutes: []int{1, 2, 3, 4}})
		}},
		{"timezone", ".zones[0].tz", func(c *Config) {
			c.Profiles[0].Pages = append(c.Profiles[0].Pages[:1], Page{ID: "c", Type: "clock", Zones: []Zone{{TZ: "Mars/Olympus"}}})
		}},
		{"page type", ".type", func(c *Config) {
			c.Profiles[0].Pages = append(c.Profiles[0].Pages[:1], Page{ID: "x", Type: "hologram"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			tc.mut(c)
			err := c.Validate()
			e, ok := err.(*Error)
			if !ok {
				t.Fatalf("want *Error, got %v", err)
			}
			if !strings.HasSuffix(e.Path, tc.path) {
				t.Fatalf("path %q, want suffix %q (%s)", e.Path, tc.path, e.Msg)
			}
		})
	}
}

func TestValidateAccepts(t *testing.T) {
	c := Default()
	c.Profiles[1].Match = []string{"steam_app_*", "org.mozilla.firefox"}
	radial(c).Buttons[0].Tap = &Action{Type: "obs", Request: "SetScene", Scene: "Gameplay"}
	c.Profiles[0].Pages = append(c.Profiles[0].Pages[:1],
		Page{ID: "o", Type: "obs"}, Page{ID: "g", Type: "game"},
		Page{ID: "t", Type: "timer", Minutes: []int{5, 25, 180}})
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}
