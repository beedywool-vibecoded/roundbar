package fwupdate

import (
	"regexp"
	"strconv"
)

// Version states for the configurator.
const (
	UpToDate        = "upToDate"        // the device runs this release
	UpdateAvailable = "updateAvailable" // the release is newer
	DeviceNewer     = "deviceNewer"     // a development build past the release
	Unknown         = "unknown"         // the device version isn't a release version (or no device)
)

// The firmware reports `git describe` (v0.2.0, v0.2.0-3-gabc1234, v0.2.0-dirty)
// or, for builds outside a tagged checkout, a bare hash.
var verRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-(\d+)-g[0-9a-f]+)?(?:-dirty)?$`)

type version struct {
	n     [3]int
	ahead int // commits past the tag
}

func parseVersion(s string) (version, bool) {
	m := verRe.FindStringSubmatch(s)
	if m == nil {
		return version{}, false
	}
	var v version
	for i := range 3 {
		v.n[i], _ = strconv.Atoi(m[i+1])
	}
	if m[4] != "" {
		v.ahead, _ = strconv.Atoi(m[4])
	}
	return v, true
}

// Compare says how the device's firmware relates to a release.
func Compare(device, release string) string {
	d, ok1 := parseVersion(device)
	r, ok2 := parseVersion(release)
	if !ok1 || !ok2 {
		return Unknown
	}
	for i := range 3 {
		if d.n[i] != r.n[i] {
			if d.n[i] < r.n[i] {
				return UpdateAvailable
			}
			return DeviceNewer
		}
	}
	if d.ahead > 0 {
		return DeviceNewer
	}
	return UpToDate
}
