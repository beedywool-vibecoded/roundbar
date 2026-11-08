package daemon

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
	"golang.org/x/sys/windows"
)

// watchConfig reloads when config.json is replaced or rewritten. The
// configurator writes a temp file and renames it over the old one.
// ReadDirectoryChangesW blocks in the kernel until something changes.
func (d *Daemon) watchConfig() {
	dir := config.Dir()
	_ = os.MkdirAll(dir, 0o755)
	p, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return
	}
	h, err := windows.CreateFile(p, windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		log.Printf("watch %s: %v", dir, err)
		return
	}
	defer windows.CloseHandle(h)
	name := strings.ToLower(filepath.Base(config.Path()))
	buf := make([]byte, 16*1024)
	for {
		var n uint32
		err := windows.ReadDirectoryChanges(h, &buf[0], uint32(len(buf)), false,
			windows.FILE_NOTIFY_CHANGE_FILE_NAME|windows.FILE_NOTIFY_CHANGE_LAST_WRITE, &n, nil, 0)
		if err != nil {
			log.Printf("watch %s: %v", dir, err)
			return
		}
		hit := false
		for off := uint32(0); off+12 <= n; {
			next := *(*uint32)(unsafe.Pointer(&buf[off]))
			action := *(*uint32)(unsafe.Pointer(&buf[off+4]))
			ln := *(*uint32)(unsafe.Pointer(&buf[off+8]))
			if off+12+ln > n {
				break
			}
			fn := windows.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(&buf[off+12])), ln/2))
			if strings.ToLower(fn) == name && (action == windows.FILE_ACTION_ADDED || action == windows.FILE_ACTION_MODIFIED ||
				action == windows.FILE_ACTION_RENAMED_NEW_NAME) {
				hit = true
			}
			if next == 0 {
				break
			}
			off += next
		}
		if hit {
			time.Sleep(50 * time.Millisecond)
			if err := d.reload(); err != nil {
				log.Printf("config: %v (keeping the previous one)", err)
			}
		}
	}
}
