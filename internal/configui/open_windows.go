package configui

import (
	"log"

	"github.com/beedywool-vibecoded/roundbar/internal/win"
)

// openBrowser hands the URL to the default browser.
func openBrowser(url string) {
	if err := win.ShellOpen(url, ""); err != nil {
		log.Printf("open browser: %v", err)
	}
}
