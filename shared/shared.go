// Package shared embeds the data files used by both the firmware build and the Go side.
package shared

import (
	_ "embed"
	"encoding/json"
)

// IconsJSON maps Material Symbols Rounded names to codepoints. The firmware font
// is generated from the same file, so only these names exist on the device.
//
//go:embed icons.json
var IconsJSON []byte

var Icons = func() map[string]int {
	m := map[string]int{}
	if err := json.Unmarshal(IconsJSON, &m); err != nil {
		panic(err)
	}
	return m
}()
