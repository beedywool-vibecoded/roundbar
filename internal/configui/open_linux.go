package configui

import "os/exec"

func openBrowser(url string) {
	c := exec.Command("xdg-open", url)
	if err := c.Start(); err == nil {
		go c.Wait()
	}
}
