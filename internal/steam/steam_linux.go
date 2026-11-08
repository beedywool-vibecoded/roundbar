package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func root() string {
	home, _ := os.UserHomeDir()
	for _, p := range []string{".local/share/Steam", ".steam/steam", ".var/app/com.valvesoftware.Steam/.local/share/Steam"} {
		if _, err := os.Stat(filepath.Join(home, p, "steamapps")); err == nil {
			return filepath.Join(home, p)
		}
	}
	return filepath.Join(home, ".local/share/Steam")
}

// GamePIDs lists our processes Steam launched for appid (see gamePIDs).
func GamePIDs(appid string) []int {
	return gamePIDs("/proc", appid)
}

// Close ends a game: SIGTERM to its processes, SIGKILL to what's left after 6 s.
func Close(appid string) (int, error) {
	pids := GamePIDs(appid)
	if len(pids) == 0 {
		return 0, fmt.Errorf("no running processes for app %s", appid)
	}
	for _, p := range pids {
		_ = syscall.Kill(p, syscall.SIGTERM)
	}
	go func() {
		for i := 0; i < 20; i++ {
			time.Sleep(300 * time.Millisecond)
			if len(GamePIDs(appid)) == 0 {
				return
			}
		}
		for _, p := range GamePIDs(appid) {
			_ = syscall.Kill(p, syscall.SIGKILL)
		}
	}()
	return len(pids), nil
}
