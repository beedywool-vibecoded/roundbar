package espflash_test

import (
	"bytes"
	"testing"

	"github.com/beedywool-vibecoded/roundbar/internal/espflash"
	"github.com/beedywool-vibecoded/roundbar/internal/espflash/fakerom"
)

// ESP32-S3 RTC registers the resets touch.
const (
	wdtConfig0 = 0x60008098
	wdtConfig1 = 0x6000809C
	swdConf    = 0x600080B4
	option1    = 0x6000812C
)

func newROM(t *testing.T, size int) *fakerom.ROM {
	r := fakerom.New(size)
	r.Errorf = t.Errorf
	return r
}

func testImage(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i/13) ^ seed
	}
	return b
}

func TestFakeFlashSequence(t *testing.T) {
	rom := newROM(t, 1<<20)
	rom.IgnoreSyncs = 3
	rom.Regs[option1] = 1 // set by our firmware's reboot-to-bootloader
	l := espflash.NewLoader(rom)
	if err := l.Sync(10); err != nil {
		t.Fatal(err)
	}
	if err := l.Begin(uint32(len(rom.Flash)), false); err != nil {
		t.Fatal(err)
	}
	app := testImage(70_001, 0x5A) // not a multiple of 4 or of a block
	same, err := l.Matches(0x10000, app)
	if err != nil || same {
		t.Fatalf("matches before write: %v %v", same, err)
	}
	var last int
	if err := l.WriteFlash(0x10000, app, func(done, total int) { last = done }); err != nil {
		t.Fatal(err)
	}
	if last != len(espflash.Pad4(app)) {
		t.Fatalf("progress ended at %d", last)
	}
	if !bytes.Equal(rom.Flash[0x10000:0x10000+len(app)], app) {
		t.Fatal("flash content differs")
	}
	if same, _ := l.Matches(0x10000, app); !same {
		t.Fatal("matches after write")
	}
	if err := l.ClearForceDownload(); err != nil {
		t.Fatal(err)
	}
	if rom.Regs[option1] != 0 {
		t.Fatal("force download flag still set")
	}
	if err := l.WatchdogReset(); err != nil {
		t.Fatal(err)
	}
	if rom.Regs[wdtConfig0] != 0xD0000102 || rom.Regs[wdtConfig1] != 2000 {
		t.Fatalf("watchdog regs %#x", rom.Regs)
	}
	if !rom.Sent(espflash.CmdFlashDeflData) || rom.Sent(espflash.CmdFlashData) {
		t.Fatal("expected compressed writes only")
	}
}

func TestFakeRetryAndFallback(t *testing.T) {
	rom := newROM(t, 1<<20)
	rom.NoDefl = true
	rom.FailOnce[1] = true
	l := espflash.NewLoader(rom)
	if err := l.Sync(3); err != nil {
		t.Fatal(err)
	}
	if err := l.Begin(uint32(len(rom.Flash)), true); err != nil {
		t.Fatal(err)
	}
	if rom.Regs[swdConf]&(1<<31) == 0 {
		t.Fatal("super watchdog not set to auto feed")
	}
	img := testImage(5000, 1)
	if err := l.WriteFlash(0x8000, img, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rom.Flash[0x8000:0x8000+len(img)], img) {
		t.Fatal("flash content differs")
	}
}

func TestFakeErrors(t *testing.T) {
	rom := newROM(t, 1<<20)
	rom.IgnoreSyncs = 100
	if err := espflash.NewLoader(rom).Sync(3); err == nil {
		t.Fatal("sync should fail")
	}
	rom = newROM(t, 1<<20)
	l := espflash.NewLoader(rom)
	if err := l.Sync(1); err != nil {
		t.Fatal(err)
	}
	if err := l.Begin(64<<10, false); err != nil { // the ROM now thinks the flash is 64 KiB
		t.Fatal(err)
	}
	if err := l.WriteFlash(0x10000, testImage(100, 0), nil); err == nil {
		t.Fatal("write beyond flash size should fail")
	}
	if err := l.WriteFlash(0x10, testImage(100, 0), nil); err == nil {
		t.Fatal("unaligned write should fail")
	}
}
