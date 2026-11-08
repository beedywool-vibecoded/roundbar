package daemon

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
	"golang.org/x/sys/unix"
)

// watchConfig reloads when config.json is replaced or rewritten. The
// configurator writes a temp file and renames it, which shows up as IN_MOVED_TO.
func (d *Daemon) watchConfig() {
	dir := config.Dir()
	_ = os.MkdirAll(dir, 0o755)
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC)
	if err != nil {
		log.Printf("inotify: %v", err)
		return
	}
	if _, err := unix.InotifyAddWatch(fd, dir, unix.IN_CLOSE_WRITE|unix.IN_MOVED_TO); err != nil {
		log.Printf("inotify watch %s: %v", dir, err)
		return
	}
	buf := make([]byte, 4096)
	name := filepath.Base(config.Path())
	for {
		n, err := unix.Read(fd, buf) // blocks in the kernel
		if err != nil {
			log.Printf("inotify read: %v", err)
			return
		}
		hit := false
		for off := 0; off+unix.SizeofInotifyEvent <= n; {
			ev := (*unix.InotifyEvent)(unsafe.Pointer(&buf[off]))
			nb := buf[off+unix.SizeofInotifyEvent : off+unix.SizeofInotifyEvent+int(ev.Len)]
			off += unix.SizeofInotifyEvent + int(ev.Len)
			if strings.TrimRight(string(nb), "\x00") == name {
				hit = true
			}
		}
		if hit {
			time.Sleep(50 * time.Millisecond)
			if err := d.reload(); err != nil {
				log.Printf("config: %v (keeping the previous one)", err)
			}
		}
	}
}
