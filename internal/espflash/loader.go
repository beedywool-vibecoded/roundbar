package espflash

import (
	"errors"
	"fmt"
	"io"
	"time"
)

// Port is the serial connection to the chip. Read must honour the deadline
// (returning an error with Timeout() == true, like os.File does).
type Port interface {
	io.Reader
	io.Writer
	SetReadDeadline(t time.Time) error
}

// Timeouts, from esptool.
const (
	defaultTimeout         = 3 * time.Second
	syncTimeout            = 100 * time.Millisecond
	md5TimeoutPerMB        = 8 * time.Second
	eraseTimeoutPerMB      = 40 * time.Second
	eraseWriteTimeoutPerMB = 40 * time.Second
)

func timeoutPerMB(per time.Duration, size int) time.Duration {
	t := time.Duration(float64(per) * float64(size) / 1e6)
	if t < defaultTimeout {
		return defaultTimeout
	}
	return t
}

// Loader is a session with the ROM bootloader.
type Loader struct {
	p Port
	r *slipReader
	// Log receives a line per step, if set.
	Log func(string)
	// sleep is time.Sleep, swapped out in tests.
	sleep func(time.Duration)
	wrote bool // a data block went out: no more falling back to plain writes
}

func NewLoader(p Port) *Loader {
	return &Loader{p: p, r: newSlipReader(p), sleep: time.Sleep}
}

func (l *Loader) logf(format string, a ...any) {
	if l.Log != nil {
		l.Log(fmt.Sprintf(format, a...))
	}
}

// command sends one request and waits for the response to the same command.
// Responses to other commands (extra SYNC replies) are skipped.
func (l *Loader) command(op byte, data []byte, chk uint32, timeout time.Duration) (Response, error) {
	if _, err := l.p.Write(SlipEncode(Command(op, data, chk))); err != nil {
		return Response{}, err
	}
	deadline := time.Now().Add(timeout)
	for range 100 {
		f, err := l.r.next(deadline)
		if err != nil {
			return Response{}, fmt.Errorf("command 0x%02x: %w", op, err)
		}
		r, ok := ParseResponse(f)
		if !ok {
			continue
		}
		if r.Op == op {
			return r, nil
		}
		// "Invalid message" for an unsupported command comes back with another op.
		if len(r.Data) >= 2 && r.Data[0] != 0 && r.Data[1] == 0x05 {
			return r, fmt.Errorf("command 0x%02x: %w", op, errUnsupported)
		}
	}
	return Response{}, fmt.Errorf("command 0x%02x: no matching response", op)
}

// check runs a command and checks its status bytes.
func (l *Loader) check(op byte, data []byte, chk uint32, timeout time.Duration) (Response, error) {
	r, err := l.command(op, data, chk, timeout)
	if err != nil {
		return r, err
	}
	return r, r.Status(0)
}

// Sync gets the ROM's attention: SYNC until it answers (it auto-detects the
// link), then drains the extra replies it sends to one SYNC.
func (l *Loader) Sync(attempts int) error {
	var err error
	for i := range attempts {
		_, err = l.command(CmdSync, SyncData(), 0, syncTimeout)
		if err == nil {
			// The ROM answers one SYNC up to 8 times; read them away.
			for {
				if _, e := l.r.next(time.Now().Add(syncTimeout)); e != nil {
					break
				}
			}
			l.r.reset()
			return nil
		}
		l.r.reset()
		if i < attempts-1 {
			l.sleep(50 * time.Millisecond)
		}
	}
	return fmt.Errorf("no answer from the bootloader: %w", err)
}

func (l *Loader) ReadReg(addr uint32) (uint32, error) {
	r, err := l.check(CmdReadReg, words(addr), 0, defaultTimeout)
	return r.Value, err
}

// WriteReg writes value to addr under mask (the ROM reads, masks and writes).
func (l *Loader) WriteReg(addr, value, mask, delayUS uint32) error {
	_, err := l.check(CmdWriteReg, words(addr, value, mask, delayUS), 0, defaultTimeout)
	return err
}

// SPIAttach connects the SPI flash with the default pins. The ROM takes a
// second word ("is legacy" plus reserved bytes), zero here.
func (l *Loader) SPIAttach() error {
	_, err := l.check(CmdSPIAttach, words(0, 0), 0, defaultTimeout)
	return err
}

// SPISetParams tells the ROM the flash size (it assumes less otherwise and
// refuses writes beyond it). Block 64 KiB, sector 4 KiB, page 256 B.
func (l *Loader) SPISetParams(size uint32) error {
	_, err := l.check(CmdSPISetParams, words(0, size, 64*1024, 4*1024, 256, 0xFFFF), 0, defaultTimeout)
	return err
}

// FlashMD5 asks the chip for the MD5 of a flash region.
func (l *Loader) FlashMD5(addr, size uint32) ([16]byte, error) {
	r, err := l.command(CmdSPIFlashMD5, words(addr, size, 0, 0), 0, timeoutPerMB(md5TimeoutPerMB, int(size)))
	if err != nil {
		return [16]byte{}, err
	}
	return ParseMD5(r)
}

// ESP32-S3 registers (esptool targets/esp32s3.py).
const (
	rtcCntlBase          = 0x60008000
	rtcCntlWDTConfig0    = rtcCntlBase + 0x98
	rtcCntlWDTConfig1    = rtcCntlBase + 0x9C
	rtcCntlWDTWProtect   = rtcCntlBase + 0xB0
	rtcCntlWDTKey        = 0x50D83AA1
	rtcCntlSWDConf       = rtcCntlBase + 0xB4
	rtcCntlSWDWProtect   = rtcCntlBase + 0xB8
	rtcCntlSWDKey        = 0x8F1D312A
	rtcCntlSWDAutoFeed   = 1 << 31
	rtcCntlOption1       = 0x6000812C
	forceDownloadBootBit = 0x1
)

// DisableWatchdogs stops the RTC watchdog and auto-feeds the super watchdog.
// Needed on the USB-Serial-JTAG port, where they keep running in download mode
// and would reset the chip halfway through a write.
func (l *Loader) DisableWatchdogs() error {
	steps := []func() error{
		func() error { return l.WriteReg(rtcCntlWDTWProtect, rtcCntlWDTKey, 0xFFFFFFFF, 0) },
		func() error { return l.WriteReg(rtcCntlWDTConfig0, 0, 0xFFFFFFFF, 0) },
		func() error { return l.WriteReg(rtcCntlWDTWProtect, 0, 0xFFFFFFFF, 0) },
		func() error { return l.WriteReg(rtcCntlSWDWProtect, rtcCntlSWDKey, 0xFFFFFFFF, 0) },
		func() error {
			v, err := l.ReadReg(rtcCntlSWDConf)
			if err != nil {
				return err
			}
			return l.WriteReg(rtcCntlSWDConf, v|rtcCntlSWDAutoFeed, 0xFFFFFFFF, 0)
		},
		func() error { return l.WriteReg(rtcCntlSWDWProtect, 0, 0xFFFFFFFF, 0) },
	}
	for _, s := range steps {
		if err := s(); err != nil {
			return err
		}
	}
	return nil
}

// ClearForceDownload clears the "boot into download mode" flag our firmware
// sets before rebooting into the ROM, so the next reset starts the app.
func (l *Loader) ClearForceDownload() error {
	return l.WriteReg(rtcCntlOption1, 0, forceDownloadBootBit, 0)
}

// WatchdogReset resets the chip through the RTC watchdog: the only way out of
// the ROM's USB-OTG download port, which ignores DTR/RTS (esptool's
// --after watchdog-reset). The chip drops off USB right after.
func (l *Loader) WatchdogReset() error {
	if err := l.WriteReg(rtcCntlWDTWProtect, rtcCntlWDTKey, 0xFFFFFFFF, 0); err != nil {
		return err
	}
	if err := l.WriteReg(rtcCntlWDTConfig1, 2000, 0xFFFFFFFF, 0); err != nil {
		return err
	}
	if err := l.WriteReg(rtcCntlWDTConfig0, 1<<31|5<<28|1<<8|2, 0xFFFFFFFF, 0); err != nil {
		return err
	}
	// The watchdog may fire before this reply makes it out: ignore its fate.
	_ = l.WriteReg(rtcCntlWDTWProtect, 0, 0xFFFFFFFF, 0)
	l.sleep(500 * time.Millisecond)
	return nil
}

var errUnsupported = errors.New("not supported by this bootloader")

// Begin prepares a freshly synced loader for flash access: watchdogs off on
// the USB-Serial-JTAG port, SPI flash attached, flash size set.
func (l *Loader) Begin(flashSize uint32, usbSerialJTAG bool) error {
	if usbSerialJTAG {
		if err := l.DisableWatchdogs(); err != nil {
			return fmt.Errorf("disable watchdogs: %w", err)
		}
	}
	if err := l.SPIAttach(); err != nil {
		return fmt.Errorf("attach flash: %w", err)
	}
	if err := l.SPISetParams(flashSize); err != nil {
		return fmt.Errorf("set flash size: %w", err)
	}
	return nil
}
