// Package config holds the roundbar config file format (docs/spec.md §1), its
// validation and where it lives on disk.
package config

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata" // world clocks validate the same on every machine
)

type Config struct {
	Version  int       `json:"version"`
	Theme    Theme     `json:"theme"`
	Media    Media     `json:"media"`
	Weather  *Weather  `json:"weather,omitempty"`
	Device   Device    `json:"device"`
	Profiles []Profile `json:"profiles"`
}

type Theme struct {
	Seed    string            `json:"seed"`
	Mode    string            `json:"mode"`
	Palette map[string]string `json:"palette"`
}

type Weather struct {
	City  string  `json:"city"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	Units string  `json:"units"`
}

type Zone struct {
	Label string `json:"label"`
	TZ    string `json:"tz"`
}

type Media struct {
	CoverBackground bool `json:"coverBackground"`
	CoverColors     bool `json:"coverColors"`
	Blur            *int `json:"blur,omitempty"`         // 0..100, nil = 60
	CoverOpacity    *int `json:"coverOpacity,omitempty"` // 0..100, nil = 50
}

// BlurOrDefault returns the configured blur strength.
func (m Media) BlurOrDefault() int {
	if m.Blur == nil {
		return 60
	}
	return *m.Blur
}

type Device struct {
	Brightness        int      `json:"brightness"`
	DimAfter          int      `json:"dimAfter"`
	SleepAfter        int      `json:"sleepAfter"`
	LongPressMs       int      `json:"longPressMs"`
	KeyboardLayout    string   `json:"keyboardLayout"`
	Rotation          int      `json:"rotation"` // degrees clockwise: 0, 90, 180, 270
	FollowPc          *bool    `json:"followPc,omitempty"`
	EdgeDoubleTap     string   `json:"edgeDoubleTap,omitempty"`
	MicIndicator      *bool    `json:"micIndicator,omitempty"`
	Screensaver       string   `json:"screensaver,omitempty"`
	ScreensaverAfter  int      `json:"screensaverAfter,omitempty"`
	ScreensaverImages []string `json:"screensaverImages,omitempty"`
}

type Profile struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Match   []string          `json:"match"`
	Pages   []Page            `json:"pages"`
	Seed    string            `json:"seed,omitempty"`
	Palette map[string]string `json:"palette,omitempty"`
}

type Page struct {
	ID      string   `json:"id"`
	Type    string   `json:"type"`
	Title   string   `json:"title,omitempty"`
	Center  *Button  `json:"center,omitempty"`
	Buttons []Button `json:"buttons,omitempty"`
	Items   []string `json:"items,omitempty"`
	Icon    string   `json:"icon,omitempty"`
	Label   string   `json:"label,omitempty"`
	Source  string   `json:"source,omitempty"`
	Step    int      `json:"step,omitempty"`
	OnUp    *Action  `json:"onUp,omitempty"`
	OnDown  *Action  `json:"onDown,omitempty"`
	// media pages: shapes of the transport buttons, keys prev/next/play/playing
	Shapes map[string]string `json:"shapes,omitempty"`
	// image pages
	Images   []string `json:"images,omitempty"`
	Interval int      `json:"interval,omitempty"`
	Fit      string   `json:"fit,omitempty"`
	// touchpad pages
	Speed int `json:"speed,omitempty"`
	// clock pages
	Style       string `json:"style,omitempty"`
	ShowWeather *bool  `json:"weather,omitempty"`
	Zones       []Zone `json:"zones,omitempty"`
	// timer pages
	Minutes  []int `json:"minutes,omitempty"`
	Pomodoro bool  `json:"pomodoro,omitempty"`
}

var assetRe = regexp.MustCompile(`^[0-9a-f]{12}\.(jpg|gif)$`)

type Button struct {
	ID    string  `json:"id"`
	Icon  string  `json:"icon"`
	Label string  `json:"label,omitempty"`
	Color string  `json:"color,omitempty"`
	Shape string  `json:"shape,omitempty"`
	Tap   *Action `json:"tap,omitempty"`
	Hold  *Action `json:"hold,omitempty"`
}

type Action struct {
	Type   string `json:"type"`
	Keys   string `json:"keys,omitempty"`
	Text   string `json:"text,omitempty"`
	Cmd    string `json:"cmd,omitempty"`
	Page   string `json:"page,omitempty"`
	Target string `json:"target,omitempty"`
	Steps  []Step `json:"steps,omitempty"`
}

type Step struct {
	Keys    string `json:"keys,omitempty"`
	Text    string `json:"text,omitempty"`
	Delay   int    `json:"delay,omitempty"`
	Command string `json:"command,omitempty"`
}

// Dir is ~/.config/roundbar (or $XDG_CONFIG_HOME/roundbar).
func Dir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "roundbar")
}

func Path() string { return filepath.Join(Dir(), "config.json") }

// Load reads the config file. A missing file yields the default config, written
// to disk so the daemon and the configurator agree on the same bytes.
func Load() (*Config, []byte, error) {
	raw, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		c := Default()
		raw, err = Save(c)
		return c, raw, err
	}
	if err != nil {
		return nil, nil, err
	}
	c, err := Parse(raw)
	return c, raw, err
}

func Parse(raw []byte) (*Config, error) {
	var c Config
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	if err := dec.Decode(&c); err != nil {
		return nil, &Error{Msg: "invalid JSON: " + err.Error()}
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save validates, then writes atomically and returns the exact bytes written.
func Save(c *Config) ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(Dir(), ".config-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	return raw, os.Rename(tmp.Name(), Path())
}

func Sha(raw []byte) string {
	s := sha1.Sum(raw)
	return hex.EncodeToString(s[:])
}

// Error carries the JSON path of the offending value so the UI can point at it.
type Error struct {
	Path string `json:"path,omitempty"`
	Msg  string `json:"error"`
}

func (e *Error) Error() string {
	if e.Path == "" {
		return e.Msg
	}
	return e.Path + ": " + e.Msg
}

var (
	idRe  = regexp.MustCompile(`^[a-z0-9-]{1,24}$`)
	hexRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
)

var Shapes = []string{"circle", "square", "cookie", "clover", "sunny", "flower", "burst"}

var PaletteRoles = []string{
	"surface", "surfaceContainer", "surfaceContainerHigh", "surfaceContainerHighest", "onSurface", "onSurfaceVariant",
	"primary", "onPrimary", "primaryContainer", "onPrimaryContainer", "secondaryContainer", "onSecondaryContainer",
	"tertiary", "tertiaryContainer", "onTertiaryContainer", "error", "errorContainer", "onErrorContainer",
	"outline", "outlineVariant",
}

func oneOf(v string, opts ...string) bool {
	for _, o := range opts {
		if v == o {
			return true
		}
	}
	return false
}

func (c *Config) Validate() error {
	fail := func(path, f string, a ...any) error { return &Error{Path: path, Msg: fmt.Sprintf(f, a...)} }
	if c.Version != 1 {
		return fail("version", "must be 1")
	}
	if !hexRe.MatchString(c.Theme.Seed) {
		return fail("theme.seed", "must be #RRGGBB")
	}
	if !oneOf(c.Theme.Mode, "dark", "light") {
		return fail("theme.mode", "must be dark or light")
	}
	for _, r := range PaletteRoles {
		if !hexRe.MatchString(c.Theme.Palette[r]) {
			return fail("theme.palette."+r, "must be #RRGGBB")
		}
	}
	if b := c.Media.Blur; b != nil && (*b < 0 || *b > 100) {
		return fail("media.blur", "must be 0..100")
	}
	if o := c.Media.CoverOpacity; o != nil && (*o < 0 || *o > 100) {
		return fail("media.coverOpacity", "must be 0..100")
	}
	d := c.Device
	switch {
	case d.Brightness < 5 || d.Brightness > 100:
		return fail("device.brightness", "must be 5..100")
	case d.DimAfter < 0 || d.DimAfter > 86400:
		return fail("device.dimAfter", "must be 0..86400")
	case d.SleepAfter < 0 || d.SleepAfter > 86400:
		return fail("device.sleepAfter", "must be 0..86400")
	case d.LongPressMs < 200 || d.LongPressMs > 2000:
		return fail("device.longPressMs", "must be 200..2000")
	case !oneOf(d.KeyboardLayout, "fr", "us"):
		return fail("device.keyboardLayout", "must be fr or us")
	case !oneOf(d.Screensaver, "", "off", "clock", "photos"):
		return fail("device.screensaver", "must be off, clock or photos")
	case d.ScreensaverAfter != 0 && (d.ScreensaverAfter < 10 || d.ScreensaverAfter > 3600):
		return fail("device.screensaverAfter", "must be 10..3600 seconds")
	case len(d.ScreensaverImages) > 20:
		return fail("device.screensaverImages", "at most 20 images")
	case !oneOf(d.EdgeDoubleTap, "", "page", "quickSettings", "screenOff", "off"):
		return fail("device.edgeDoubleTap", "must be page, quickSettings, screenOff or off")
	case d.Rotation != 0 && d.Rotation != 90 && d.Rotation != 180 && d.Rotation != 270:
		return fail("device.rotation", "must be 0, 90, 180 or 270")
	}
	for k, id := range d.ScreensaverImages {
		if !assetRe.MatchString(id) || strings.HasSuffix(id, ".gif") {
			return fail(fmt.Sprintf("device.screensaverImages[%d]", k), "must be a still image asset")
		}
	}
	if w := c.Weather; w != nil {
		switch {
		case w.City == "" || len(w.City) > 64:
			return fail("weather.city", "must be 1..64 chars")
		case w.Lat < -90 || w.Lat > 90 || w.Lon < -180 || w.Lon > 180:
			return fail("weather", "bad coordinates")
		case !oneOf(w.Units, "metric", "imperial"):
			return fail("weather.units", "must be metric or imperial")
		}
	}
	if len(c.Profiles) == 0 || len(c.Profiles) > 32 {
		return fail("profiles", "need 1..32 profiles")
	}
	seen := map[string]bool{}
	hasDefault := false
	for i, p := range c.Profiles {
		pp := fmt.Sprintf("profiles[%d]", i)
		if !idRe.MatchString(p.ID) {
			return fail(pp+".id", "must match [a-z0-9-]{1,24}")
		}
		if seen[p.ID] {
			return fail(pp+".id", "duplicate id %q", p.ID)
		}
		seen[p.ID] = true
		hasDefault = hasDefault || p.ID == "default"
		if len(p.Name) == 0 || len(p.Name) > 32 {
			return fail(pp+".name", "must be 1..32 chars")
		}
		for j, m := range p.Match {
			if m == "" || m != strings.ToLower(m) || len(m) > 128 {
				return fail(fmt.Sprintf("%s.match[%d]", pp, j), "must be a non-empty lowercase window class")
			}
		}
		if p.Seed != "" && !hexRe.MatchString(p.Seed) {
			return fail(pp+".seed", "must be #RRGGBB")
		}
		for role, v := range p.Palette {
			if !hexRe.MatchString(v) {
				return fail(pp+".palette."+role, "must be #RRGGBB")
			}
		}
		if len(p.Pages) == 0 || len(p.Pages) > 8 {
			return fail(pp+".pages", "need 1..8 pages")
		}
		pageIDs := map[string]bool{}
		for _, pg := range p.Pages {
			pageIDs[pg.ID] = true
		}
		if len(pageIDs) != len(p.Pages) {
			return fail(pp+".pages", "page ids must be unique")
		}
		for j, pg := range p.Pages {
			if err := pg.validate(fmt.Sprintf("%s.pages[%d]", pp, j), pageIDs); err != nil {
				return err
			}
		}
	}
	if !hasDefault {
		return fail("profiles", `a profile with id "default" is required`)
	}
	return nil
}

func (pg *Page) validate(pp string, pageIDs map[string]bool) error {
	fail := func(path, f string, a ...any) error { return &Error{Path: path, Msg: fmt.Sprintf(f, a...)} }
	if !idRe.MatchString(pg.ID) {
		return fail(pp+".id", "must match [a-z0-9-]{1,24}")
	}
	if len(pg.Title) > 24 {
		return fail(pp+".title", "at most 24 chars")
	}
	switch pg.Type {
	case "radial":
		if len(pg.Buttons) == 0 || len(pg.Buttons) > 8 {
			return fail(pp+".buttons", "need 1..8 buttons")
		}
		ids := map[string]bool{}
		for k, b := range pg.Buttons {
			bp := fmt.Sprintf("%s.buttons[%d]", pp, k)
			if ids[b.ID] {
				return fail(bp+".id", "duplicate id")
			}
			ids[b.ID] = true
			if err := b.validate(bp, pageIDs); err != nil {
				return err
			}
		}
		if pg.Center != nil {
			if ids[pg.Center.ID] {
				return fail(pp+".center.id", "duplicate id")
			}
			if err := pg.Center.validate(pp+".center", pageIDs); err != nil {
				return err
			}
		}
	case "media":
		for k, v := range pg.Shapes {
			if !oneOf(k, "prev", "next", "play", "playing") {
				return fail(pp+".shapes", "unknown key %q (prev, next, play, playing)", k)
			}
			if !oneOf(v, Shapes...) {
				return fail(pp+".shapes."+k, "unknown shape %q", v)
			}
		}
	case "clock":
		if !oneOf(pg.Style, "", "analog", "digital") {
			return fail(pp+".style", "must be analog or digital")
		}
		if len(pg.Zones) > 3 {
			return fail(pp+".zones", "at most 3 world clocks")
		}
		for k, z := range pg.Zones {
			if _, err := time.LoadLocation(z.TZ); err != nil || z.TZ == "" || z.TZ == "Local" {
				return fail(fmt.Sprintf("%s.zones[%d].tz", pp, k), "unknown time zone %q", z.TZ)
			}
			if len([]rune(z.Label)) > 16 {
				return fail(fmt.Sprintf("%s.zones[%d].label", pp, k), "at most 16 chars")
			}
		}
	case "timer":
		if len(pg.Minutes) > 4 {
			return fail(pp+".minutes", "at most 4 presets")
		}
		for k, m := range pg.Minutes {
			if m < 1 || m > 180 {
				return fail(fmt.Sprintf("%s.minutes[%d]", pp, k), "must be 1..180")
			}
		}
		if len([]rune(pg.Label)) > 24 {
			return fail(pp+".label", "at most 24 chars")
		}
	case "touchpad":
		if pg.Speed != 0 && (pg.Speed < 1 || pg.Speed > 10) {
			return fail(pp+".speed", "must be 1..10")
		}
	case "image":
		if len(pg.Images) == 0 || len(pg.Images) > 20 {
			return fail(pp+".images", "need 1..20 images")
		}
		for k, id := range pg.Images {
			if !assetRe.MatchString(id) {
				return fail(fmt.Sprintf("%s.images[%d]", pp, k), "not an asset id")
			}
			if strings.HasSuffix(id, ".gif") && len(pg.Images) > 1 {
				return fail(pp+".images", "a GIF must be the only image of its page")
			}
		}
		if pg.Interval != 0 && (pg.Interval < 2 || pg.Interval > 3600) {
			return fail(pp+".interval", "must be 2..3600 seconds")
		}
		if !oneOf(pg.Fit, "", "cover", "contain") {
			return fail(pp+".fit", "must be cover or contain")
		}
	case "stats":
		if len(pg.Items) == 0 || len(pg.Items) > 4 {
			return fail(pp+".items", "need 1..4 items")
		}
		for k, it := range pg.Items {
			if !oneOf(it, "cpu", "cpuTemp", "ram", "gpu", "gpuTemp", "vram") {
				return fail(fmt.Sprintf("%s.items[%d]", pp, k), "unknown stat %q", it)
			}
		}
	case "dial":
		if !oneOf(pg.Source, "volume", "mic", "brightness", "custom") {
			return fail(pp+".source", "must be volume, mic, brightness or custom")
		}
		if pg.Icon != "" && !IconExists(pg.Icon) {
			return fail(pp+".icon", "unknown icon %q", pg.Icon)
		}
		if len(pg.Label) > 16 {
			return fail(pp+".label", "at most 16 chars")
		}
		if pg.Source == "custom" {
			if pg.Step < 1 || pg.Step > 20 {
				return fail(pp+".step", "must be 1..20")
			}
			for name, a := range map[string]*Action{"onUp": pg.OnUp, "onDown": pg.OnDown} {
				if a != nil {
					if err := a.validate(pp+"."+name, pageIDs); err != nil {
						return err
					}
				}
			}
		}
	default:
		return fail(pp+".type", "unknown page type %q", pg.Type)
	}
	return nil
}

func (b *Button) validate(bp string, pageIDs map[string]bool) error {
	if !idRe.MatchString(b.ID) {
		return &Error{Path: bp + ".id", Msg: "must match [a-z0-9-]{1,24}"}
	}
	if !IconExists(b.Icon) {
		return &Error{Path: bp + ".icon", Msg: fmt.Sprintf("unknown icon %q", b.Icon)}
	}
	if len([]rune(b.Label)) > 16 {
		return &Error{Path: bp + ".label", Msg: "at most 16 chars"}
	}
	if b.Color != "" && !oneOf(b.Color, "primary", "secondary", "tertiary", "error", "surface") {
		return &Error{Path: bp + ".color", Msg: "unknown color role"}
	}
	if b.Shape != "" && !oneOf(b.Shape, Shapes...) {
		return &Error{Path: bp + ".shape", Msg: "unknown shape"}
	}
	if b.Tap == nil {
		return &Error{Path: bp + ".tap", Msg: "required"}
	}
	if err := b.Tap.validate(bp+".tap", pageIDs); err != nil {
		return err
	}
	if b.Hold != nil {
		return b.Hold.validate(bp+".hold", pageIDs)
	}
	return nil
}

func (a *Action) validate(ap string, pageIDs map[string]bool) error {
	fail := func(path, f string, args ...any) error { return &Error{Path: path, Msg: fmt.Sprintf(f, args...)} }
	switch a.Type {
	case "keys":
		if err := ValidKeys(a.Keys); err != nil {
			return fail(ap+".keys", "%v", err)
		}
	case "text":
		if err := ValidText(a.Text); err != nil {
			return fail(ap+".text", "%v", err)
		}
	case "media":
		if !oneOf(a.Cmd, "play_pause", "next", "prev", "vol_up", "vol_down", "mute") {
			return fail(ap+".cmd", "unknown media command")
		}
	case "page":
		if !pageIDs[a.Page] {
			return fail(ap+".page", "no page %q in this profile", a.Page)
		}
	case "command":
		if strings.TrimSpace(a.Cmd) == "" || len(a.Cmd) > 1024 {
			return fail(ap+".cmd", "must be 1..1024 chars")
		}
	case "open":
		if strings.TrimSpace(a.Target) == "" || len(a.Target) > 1024 {
			return fail(ap+".target", "must be 1..1024 chars")
		}
	case "mic":
		if !oneOf(a.Cmd, "toggle", "mute", "unmute") {
			return fail(ap+".cmd", "must be toggle, mute or unmute")
		}
	case "macro":
		if len(a.Steps) == 0 || len(a.Steps) > 32 {
			return fail(ap+".steps", "need 1..32 steps")
		}
		for i, s := range a.Steps {
			sp := fmt.Sprintf("%s.steps[%d]", ap, i)
			set := 0
			if s.Keys != "" {
				set++
				if err := ValidKeys(s.Keys); err != nil {
					return fail(sp+".keys", "%v", err)
				}
			}
			if s.Text != "" {
				set++
				if err := ValidText(s.Text); err != nil {
					return fail(sp+".text", "%v", err)
				}
			}
			if s.Command != "" {
				set++
			}
			if s.Delay != 0 {
				set++
				if s.Delay < 0 || s.Delay > 5000 {
					return fail(sp+".delay", "must be 1..5000 ms")
				}
			}
			if set != 1 {
				return fail(sp, "a step needs exactly one of keys, text, delay, command")
			}
		}
	default:
		return fail(ap+".type", "unknown action type %q", a.Type)
	}
	return nil
}

// RunsOnDaemon tells whether the device hands this action to the PC.
func (a *Action) RunsOnDaemon() bool {
	return oneOf(a.Type, "command", "open", "mic", "macro")
}
