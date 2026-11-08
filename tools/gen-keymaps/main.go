// Command gen-keymaps regenerates the firmware and configurator keyboard tables from
// shared/keymaps.json. Run it from the repository root after editing that file:
//
//	go run ./tools/gen-keymaps
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/beedywool-vibecoded/roundbar/internal/keymap"
)

func main() {
	for path, data := range map[string][]byte{keymap.CHeaderPath: keymap.CHeader(), keymap.TSPath: keymap.TS()} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println(len(keymap.All()), "layouts")
}
