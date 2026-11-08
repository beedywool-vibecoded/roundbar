//go:build !windows

package icons

import "image"

// exeIcon reads the icon embedded in a Windows executable; elsewhere there is none.
func exeIcon(string, int) image.Image { return nil }
