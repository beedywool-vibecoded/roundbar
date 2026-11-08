package desktop

import (
	"log"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/internal/keymap"
	"golang.org/x/sys/windows/registry"
)

// platform: Windows' media sessions.
type platform struct {
	media *smtc
}

// Detect names the backend Connect picks: there is one on Windows.
func Detect() string { return BackendWindows }

func Connect() (*Desktop, error) {
	d := newDesktop()
	d.backend = BackendWindows
	s, err := newWinShell(d)
	if err != nil {
		log.Printf("desktop: windows: %v", err)
		d.shell = none{}
	} else {
		d.shell = s
	}
	if m, err := newSMTC(d.notifyMedia); err != nil {
		log.Printf("media: %v (no now-playing info)", err)
	} else {
		d.media = m
	}
	return d, nil
}

func (d *Desktop) Close() {
	d.shell.Close()
	if d.media != nil {
		d.media.close()
	}
}

// KDELayoutIndex is KDE's; there is no KDE here.
func (d *Desktop) KDELayoutIndex() (int, bool) { return 0, false }

// KLID is the keyboard layout of the focused window's thread (GetKeyboardLayout).
func (d *Desktop) KLID() (string, bool) {
	ws, ok := d.shell.(*winShell)
	if !ok {
		return "", false
	}
	hkl := ws.foregroundHKL()
	if hkl == 0 {
		return "", false
	}
	k := keymap.KLIDFromHKL(hkl, layoutIDToKLID)
	return k, k != ""
}

// layoutIDToKLID finds the layout whose registry entry has this "Layout Id".
func layoutIDToKLID(id uint16) string {
	root, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Keyboard Layouts`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return ""
	}
	defer root.Close()
	names, _ := root.ReadSubKeyNames(0)
	for _, n := range names {
		k, err := registry.OpenKey(root, n, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		v, _, err := k.GetStringValue("Layout Id")
		k.Close()
		if err == nil && strings.EqualFold(strings.TrimLeft(v, "0"), strings.TrimLeft(hex4(id), "0")) {
			return n
		}
	}
	return ""
}

func hex4(v uint16) string {
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>12], digits[v>>8&15], digits[v>>4&15], digits[v&15]})
}

// Media returns the session Windows considers current (the one media keys
// control). ok=false when there is none.
func (d *Desktop) Media() (Media, bool) {
	if d.media == nil {
		return Media{}, false
	}
	return d.media.media()
}
