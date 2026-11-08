package espflash

import (
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/beedywool-vibecoded/roundbar/internal/device"
	"golang.org/x/sys/windows"
)

// Espressif USB ids of the ROM's download ports.
const (
	PIDUSBOTG        = "0009" // ROM USB-OTG CDC, reached via our firmware's reboot-to-bootloader
	PIDUSBSerialJTAG = "1001" // USB-Serial-JTAG, e.g. after holding BOOT while plugging in
	PIDRoundbar      = "4011" // our own firmware: not a download port
)

// DownloadPort is a COM port of a chip waiting in the ROM bootloader.
type DownloadPort struct {
	Path string // COM7
	PID  string // PIDUSBOTG or PIDUSBSerialJTAG
}

// FindDownloadPort looks for an Espressif ROM download port (303a:0009 or
// 303a:1001) among the present COM ports. It does not open anything.
func FindDownloadPort() (DownloadPort, bool) {
	for _, pid := range []string{PIDUSBOTG, PIDUSBSerialJTAG} {
		n, _ := strconv.ParseUint(pid, 16, 16)
		if p := device.Ports(0x303a, uint16(n)); len(p) > 0 {
			return DownloadPort{Path: p[0], PID: pid}, true
		}
	}
	return DownloadPort{}, false
}

// Serial is a COM port opened for the bootloader (overlapped, so the Go
// runtime serves reads, deadlines and Close).
type Serial struct {
	*os.File
	h windows.Handle
}

// OpenSerial opens a download port raw 8N1. It leaves DTR/RTS low and
// unmanaged: the ROM's USB-OTG port rejects line changes.
func OpenSerial(path string) (*Serial, error) {
	name := path
	if !strings.HasPrefix(name, `\\.\`) {
		name = `\\.\` + path
	}
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	var dcb windows.DCB
	dcb.DCBlength = uint32(unsafe.Sizeof(dcb))
	if windows.GetCommState(h, &dcb) == nil {
		dcb.BaudRate = 115200 // never 1200: that is "reboot to bootloader" for our firmware
		dcb.ByteSize = 8
		dcb.Parity = windows.NOPARITY
		dcb.StopBits = windows.ONESTOPBIT
		const (
			fBinary         = 1 << 0
			fOutxCtsFlow    = 1 << 2
			fOutxDsrFlow    = 1 << 3
			fDtrControlMask = 3 << 4
			fDsrSensitivity = 1 << 6
			fOutX           = 1 << 8
			fInX            = 1 << 9
			fRtsControlMask = 3 << 12
			fAbortOnError   = 1 << 14
		)
		dcb.Flags &^= fOutxCtsFlow | fOutxDsrFlow | fDtrControlMask | fDsrSensitivity | fOutX | fInX | fRtsControlMask | fAbortOnError
		dcb.Flags |= fBinary
		_ = windows.SetCommState(h, &dcb) // a USB CDC port may refuse line settings; they don't matter
	}
	to := windows.CommTimeouts{ReadIntervalTimeout: 0xFFFFFFFF, ReadTotalTimeoutMultiplier: 0xFFFFFFFF, ReadTotalTimeoutConstant: 0xFFFFFFFE}
	_ = windows.SetCommTimeouts(h, &to)
	_ = windows.PurgeComm(h, windows.PURGE_RXCLEAR|windows.PURGE_TXCLEAR)
	return &Serial{File: os.NewFile(uintptr(h), path), h: h}, nil
}

// HardReset pulses EN through RTS with DTR released (so GPIO0 is high and the
// app boots), like esptool's hard reset on the USB-Serial-JTAG port.
func (s *Serial) HardReset() error {
	if err := windows.EscapeCommFunction(s.h, windows.CLRDTR); err != nil {
		return err
	}
	if err := windows.EscapeCommFunction(s.h, windows.SETRTS); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	err := windows.EscapeCommFunction(s.h, windows.CLRRTS)
	time.Sleep(200 * time.Millisecond)
	return err
}
