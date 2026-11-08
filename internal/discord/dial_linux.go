package discord

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

func socketPaths() []string {
	run := os.Getenv("XDG_RUNTIME_DIR")
	var out []string
	for _, dir := range []string{run, filepath.Join(run, "app/com.discordapp.Discord"), filepath.Join(run, "snap.discord"),
		filepath.Join(run, ".flatpak/com.discordapp.Discord/xdg-run"), filepath.Join(run, "app/dev.vencord.Vesktop")} {
		for i := 0; i < 10; i++ {
			out = append(out, filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", i)))
		}
	}
	return out
}

func dial() (net.Conn, error) {
	for _, p := range socketPaths() {
		if c, err := net.DialTimeout("unix", p, time.Second); err == nil {
			return c, nil
		}
	}
	return nil, errors.New("Discord isn't running")
}
