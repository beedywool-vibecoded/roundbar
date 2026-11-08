package espflash

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Espressif USB ids of the ROM's download ports.
const (
	PIDUSBOTG        = "0009" // ROM USB-OTG CDC, reached via our firmware's reboot-to-bootloader
	PIDUSBSerialJTAG = "1001" // USB-Serial-JTAG, e.g. after holding BOOT while plugging in
	PIDRoundbar      = "4011" // our own firmware: not a download port
)

// DownloadPort is a tty of a chip waiting in the ROM bootloader.
type DownloadPort struct {
	Path string // /dev/ttyACM1
	PID  string // PIDUSBOTG or PIDUSBSerialJTAG
}

// FindDownloadPort looks for an Espressif ROM download port (303a:0009 or
// 303a:1001) in sysfs. It does not open anything.
func FindDownloadPort() (DownloadPort, bool) {
	return findDownloadPort("/sys/class/tty")
}

func findDownloadPort(sysTTY string) (DownloadPort, bool) {
	m, _ := filepath.Glob(filepath.Join(sysTTY, "ttyACM*"))
	for _, t := range m {
		vid := readID(filepath.Join(t, "device", "..", "idVendor"))
		pid := readID(filepath.Join(t, "device", "..", "idProduct"))
		if vid == "303a" && (pid == PIDUSBOTG || pid == PIDUSBSerialJTAG) {
			return DownloadPort{Path: "/dev/" + filepath.Base(t), PID: pid}, true
		}
	}
	return DownloadPort{}, false
}

func readID(p string) string {
	b, _ := os.ReadFile(p)
	return strings.TrimSpace(string(b))
}

// Serial is a raw tty opened for the bootloader.
type Serial struct {
	*os.File
	fd int
}

// OpenSerial opens a download port in raw mode. It leaves the modem lines
// alone: the ROM's USB-OTG port rejects DTR/RTS requests.
func OpenSerial(path string) (*Serial, error) {
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_NOCTTY|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	if t, err := unix.IoctlGetTermios(fd, unix.TCGETS); err == nil {
		t.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON | unix.IXOFF
		t.Oflag &^= unix.OPOST
		t.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
		t.Cflag &^= unix.CSIZE | unix.PARENB | unix.CRTSCTS | unix.CBAUD
		t.Cflag |= unix.CS8 | unix.CREAD | unix.CLOCAL | unix.B115200 // never 1200: that is "reboot to bootloader" for our firmware
		t.Ispeed, t.Ospeed = unix.B115200, unix.B115200
		t.Cc[unix.VMIN], t.Cc[unix.VTIME] = 1, 0
		_ = unix.IoctlSetTermios(fd, unix.TCSETS, t) // a USB CDC port may refuse line settings; they don't matter
	}
	_ = unix.IoctlSetInt(fd, unix.TCFLSH, unix.TCIOFLUSH)
	return &Serial{File: os.NewFile(uintptr(fd), path), fd: fd}, nil
}

func (s *Serial) setLine(bit int, on bool) error {
	req := uint(unix.TIOCMBIC)
	if on {
		req = unix.TIOCMBIS
	}
	return unix.IoctlSetPointerInt(s.fd, req, bit)
}

// HardReset pulses EN through RTS with DTR released (so GPIO0 is high and the
// app boots), like esptool's hard reset on the USB-Serial-JTAG port.
func (s *Serial) HardReset() error {
	if err := s.setLine(unix.TIOCM_DTR, false); err != nil {
		return err
	}
	if err := s.setLine(unix.TIOCM_RTS, true); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	err := s.setLine(unix.TIOCM_RTS, false)
	time.Sleep(200 * time.Millisecond)
	return err
}
