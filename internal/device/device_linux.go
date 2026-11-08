package device

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Ports lists the serial devices of every attached USB device with these ids
// (for tools such as the flasher; the daemon uses Find).
func Ports(vid, pid uint16) []string {
	ttys, _ := filepath.Glob("/sys/class/tty/tty*/device")
	var out []string
	for _, t := range ttys {
		dev, err := filepath.EvalSymlinks(t)
		if err != nil {
			continue
		}
		// .../X-Y/X-Y:1.0 is the interface; the USB device a level up holds the ids
		for d := dev; d != "/" && d != "."; d = filepath.Dir(d) {
			v, err1 := os.ReadFile(filepath.Join(d, "idVendor"))
			p, err2 := os.ReadFile(filepath.Join(d, "idProduct"))
			if err1 != nil || err2 != nil {
				continue
			}
			if hexID(v) == vid && hexID(p) == pid {
				out = append(out, "/dev/"+filepath.Base(filepath.Dir(t)))
			}
			break
		}
	}
	return out
}

// Find returns the serial path of an attached roundbar, or "".
// The firmware reports manufacturer and product "roundbar", which udev turns into
// /dev/serial/by-id/usb-roundbar_roundbar_<serial>-if00.
func Find() string {
	m, _ := filepath.Glob("/dev/serial/by-id/usb-roundbar_roundbar_*")
	for _, p := range m {
		if strings.HasSuffix(p, "-if00") || len(m) == 1 {
			return p
		}
	}
	if len(m) > 0 {
		return m[0]
	}
	return ""
}

func Open(path string) (*Port, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0) // nonblocking: the Go poller owns it, so deadlines and Close work
	if err != nil {
		return nil, err
	}
	t, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		unix.Close(fd)
		return nil, err
	}
	// cfmakeraw; the baud rate is meaningless on USB CDC.
	t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	t.Oflag &^= unix.OPOST
	t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	t.Cflag &^= unix.CSIZE | unix.PARENB
	t.Cflag |= unix.CS8 | unix.CREAD | unix.CLOCAL | unix.HUPCL
	t.Cc[unix.VMIN], t.Cc[unix.VTIME] = 1, 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, t); err != nil {
		unix.Close(fd)
		return nil, err
	}
	// Raise DTR+RTS: the firmware treats DTR as "a host is listening".
	bits := unix.TIOCM_DTR | unix.TIOCM_RTS
	_ = unix.IoctlSetPointerInt(fd, unix.TIOCMBIS, bits)
	return &Port{Path: path, f: os.NewFile(uintptr(fd), path)}, nil
}

// WaitHotplug blocks until something changes under /dev/serial/by-id (or /dev
// when that directory doesn't exist yet), or until done is closed.
func WaitHotplug(done <-chan struct{}) {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		sleep(done, 5*time.Second)
		return
	}
	defer unix.Close(fd)
	dir := "/dev/serial/by-id"
	if _, err := os.Stat(dir); err != nil {
		dir = "/dev"
	}
	if _, err := unix.InotifyAddWatch(fd, dir, unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVED_TO); err != nil {
		sleep(done, 5*time.Second)
		return
	}
	buf := make([]byte, 4096)
	pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		select {
		case <-done:
			return
		default:
		}
		// Poll in 1 s slices only to notice done; the thread sleeps in the kernel.
		n, _ := unix.Poll(pfd, 1000)
		if n <= 0 {
			continue
		}
		k, _ := unix.Read(fd, buf)
		for off := 0; off+unix.SizeofInotifyEvent <= k; {
			ev := (*unix.InotifyEvent)(unsafePointer(&buf[off]))
			nameb := buf[off+unix.SizeofInotifyEvent : off+unix.SizeofInotifyEvent+int(ev.Len)]
			name := strings.TrimRight(string(nameb), "\x00")
			off += unix.SizeofInotifyEvent + int(ev.Len)
			if dir != "/dev" || strings.HasPrefix(name, "ttyACM") || name == "serial" {
				// udev creates the by-id link a moment after the node.
				sleep(done, 400*time.Millisecond)
				return
			}
		}
	}
}
