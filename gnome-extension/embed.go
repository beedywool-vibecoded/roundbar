// Package gnomeext embeds the GNOME Shell extension so `roundbar install` can drop it in place.
package gnomeext

import "embed"

const UUID = "roundbar@beedy"

//go:embed roundbar@beedy
var Files embed.FS
