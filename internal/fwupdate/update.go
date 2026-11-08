package fwupdate

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/espflash"
)

// SerialPort is an open download port.
type SerialPort interface {
	espflash.Port
	io.Closer
}

// Updater writes a release to the device. Every hook has a real default
// (see New); tests replace them to drive the whole sequence without hardware.
type Updater struct {
	Manifest *Manifest
	// Part returns the verified contents of a part (Fetcher.Part).
	Part func(ctx context.Context, p Part) ([]byte, error)
	// RequestBootloader asks the running firmware (through the daemon) to
	// reboot into the ROM download mode.
	RequestBootloader func() error
	FindPort          func() (espflash.DownloadPort, bool)
	OpenPort          func(path string) (SerialPort, error)
	// Log gets human readable progress lines.
	Log func(string)
	// Progress gets the overall write progress in bytes.
	Progress func(done, total int)
	// ManualWait is how long to wait for someone to plug the board in with
	// BOOT held when the firmware can't be asked to reboot.
	ManualWait time.Duration

	sleep func(time.Duration)
}

// New returns an Updater for m that downloads with f and asks the daemon
// through requestBootloader.
func New(m *Manifest, f *Fetcher, requestBootloader func() error) *Updater {
	return &Updater{
		Manifest:          m,
		Part:              func(ctx context.Context, p Part) ([]byte, error) { return f.Part(ctx, m, p) },
		RequestBootloader: requestBootloader,
		FindPort:          espflash.FindDownloadPort,
		OpenPort:          func(path string) (SerialPort, error) { return espflash.OpenSerial(path) },
		ManualWait:        2 * time.Minute,
	}
}

func (u *Updater) logf(format string, a ...any) {
	if u.Log != nil {
		u.Log(fmt.Sprintf(format, a...))
	}
}

func (u *Updater) wait(d time.Duration) {
	if u.sleep != nil {
		u.sleep(d)
		return
	}
	time.Sleep(d)
}

// ErrNoDevice means no download port showed up.
var ErrNoDevice = errors.New("the board did not show up in download mode")

// smallParts are fetched before the device reboots; the font pack (9 MB) is
// only fetched if the device's copy differs.
var bigParts = map[string]bool{"fonts": true}

// Run installs the release: fetch, reboot into download mode, compare, write,
// reset. The chip is only reset back into the app if nothing was written or
// everything was written and verified; after a failed write it stays in
// download mode so the update can simply be run again.
func (u *Updater) Run(ctx context.Context) error {
	m := u.Manifest
	parts := append([]Part(nil), m.Parts...)
	sort.Slice(parts, func(i, j int) bool { return parts[i].Offset < parts[j].Offset })

	data := map[string][]byte{}
	for _, p := range parts {
		if bigParts[p.Name] {
			continue
		}
		u.logf("downloading %s (%d KiB)", p.File, p.Size/1024)
		b, err := u.Part(ctx, p)
		if err != nil {
			return err
		}
		data[p.Name] = b
	}

	dp, err := u.enterDownloadMode(ctx)
	if err != nil {
		return err
	}
	u.logf("bootloader on %s (%s)", dp.Path, portKind(dp.PID))
	port, err := u.OpenPort(dp.Path)
	if errors.Is(err, os.ErrPermission) {
		return fmt.Errorf("no permission to open %s: install the udev rule with `roundbar udev`", dp.Path)
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", dp.Path, err)
	}
	defer port.Close()
	l := espflash.NewLoader(port)
	l.Log = u.Log
	if err := l.Sync(30); err != nil {
		return err
	}
	jtag := dp.PID == espflash.PIDUSBSerialJTAG
	if err := l.Begin(m.FlashBytes, jtag); err != nil {
		return err
	}

	// What differs on the chip?
	var todo []Part
	for _, p := range parts {
		if err := ctx.Err(); err != nil {
			u.reset(l, port, jtag)
			return err
		}
		sum, err := l.FlashMD5(p.Offset, p.flashSize())
		if err != nil {
			u.reset(l, port, jtag)
			return fmt.Errorf("read %s checksum: %w", p.Name, err)
		}
		if hex.EncodeToString(sum[:]) == p.FlashMD5 {
			u.logf("%s: already up to date", p.Name)
			continue
		}
		todo = append(todo, p)
	}
	total := 0
	for _, p := range todo {
		if data[p.Name] == nil {
			u.logf("downloading %s (%d KiB)", p.File, p.Size/1024)
			b, err := u.Part(ctx, p)
			if err != nil {
				u.reset(l, port, jtag) // nothing written yet: back to the old firmware
				return err
			}
			data[p.Name] = b
		}
		total += len(Padded(data[p.Name]))
	}

	done := 0
	for _, p := range todo {
		if err := ctx.Err(); err != nil {
			return err // stays in download mode
		}
		u.logf("writing %s at 0x%x (%d KiB)", p.Name, p.Offset, p.Size/1024)
		base := done
		err := l.WriteFlash(p.Offset, data[p.Name], func(d, _ int) {
			if u.Progress != nil {
				u.Progress(base+d, total)
			}
		})
		if err != nil {
			u.logf("the board stays in download mode: run the update again")
			return fmt.Errorf("%s: %w", p.Name, err)
		}
		done += len(Padded(data[p.Name]))
		u.logf("%s written and verified", p.Name)
	}
	if len(todo) == 0 {
		u.logf("everything already matches %s", m.Version)
	}
	u.reset(l, port, jtag)
	return nil
}

func portKind(pid string) string {
	if pid == espflash.PIDUSBSerialJTAG {
		return "USB-Serial-JTAG"
	}
	return "USB download port"
}

// enterDownloadMode returns the download port, asking the firmware to reboot
// into it when needed and falling back to waiting for a manual BOOT + plug.
func (u *Updater) enterDownloadMode(ctx context.Context) (espflash.DownloadPort, error) {
	if dp, ok := u.FindPort(); ok {
		return dp, nil
	}
	if u.RequestBootloader != nil {
		u.logf("asking the device to restart in download mode")
		// Repeat for a few seconds: the daemon may still be connecting.
		for range 8 {
			_ = u.RequestBootloader()
			for range 30 {
				if dp, ok := u.FindPort(); ok {
					return dp, nil
				}
				u.wait(50 * time.Millisecond)
			}
			u.wait(time.Second)
		}
	}
	u.logf("the device did not restart by itself: unplug it, hold the BOOT button while plugging it back in, then release it")
	deadline := time.Now().Add(u.ManualWait)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return espflash.DownloadPort{}, err
		}
		if dp, ok := u.FindPort(); ok {
			u.wait(300 * time.Millisecond) // let udev set permissions
			return dp, nil
		}
		u.wait(100 * time.Millisecond)
	}
	return espflash.DownloadPort{}, ErrNoDevice
}

// reset starts the app: clear the forced download flag our firmware set, then
// RTS on USB-Serial-JTAG or the watchdog on the ROM's USB-OTG port.
func (u *Updater) reset(l *espflash.Loader, port SerialPort, jtag bool) {
	_ = l.ClearForceDownload()
	if hr, ok := port.(interface{ HardReset() error }); ok && jtag {
		if hr.HardReset() == nil {
			u.logf("restarting the device")
			return
		}
	}
	u.logf("restarting the device")
	_ = l.WatchdogReset()
}
