package espflash

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"io"
	"testing"
	"time"
)

type timeoutErr struct{}

func (timeoutErr) Error() string { return "i/o timeout" }
func (timeoutErr) Timeout() bool { return true }

// fakeROM plays the ESP32-S3 ROM loader on the other end of a Port: it
// decodes SLIP frames, checks every field the real ROM would, keeps a flash
// image and answers like the ROM (4 status bytes, MD5 as hex text).
type fakeROM struct {
	t     *testing.T
	flash []byte
	regs  map[uint32]uint32
	out   []byte // pending bytes to the host
	in    []byte // partial frame bytes from the host
	log   []byte // command ops in order

	ignoreSyncs int  // drop this many SYNCs (chip not ready yet)
	noDefl      bool // answer FLASH_DEFL_BEGIN with "invalid message"
	failOnce    map[uint32]bool
	flashSize   uint32
	attached    bool

	// current write
	op       byte
	offset   uint32
	blocks   uint32
	seq      uint32
	comp     []byte
	writeLen uint32
}

func newFakeROM(t *testing.T, size int) *fakeROM {
	f := &fakeROM{t: t, flash: bytes.Repeat([]byte{0xFF}, size), regs: map[uint32]uint32{}, failOnce: map[uint32]bool{}}
	f.out = append(f.out, []byte("ESP-ROM:esp32s3-20210327\r\nwaiting for download\r\n")...)
	return f
}

func (f *fakeROM) Read(p []byte) (int, error) {
	if len(f.out) == 0 {
		return 0, timeoutErr{}
	}
	n := copy(p, f.out)
	f.out = f.out[n:]
	return n, nil
}

func (f *fakeROM) SetReadDeadline(time.Time) error { return nil }

func (f *fakeROM) Write(p []byte) (int, error) {
	r := newSlipReader(&chunkPort{data: append(f.in, p...), n: 1 << 20})
	for {
		fr, err := r.next(time.Now().Add(time.Second))
		if err != nil {
			break
		}
		f.handle(fr)
	}
	f.in = nil
	return len(p), nil
}

func (f *fakeROM) reply(op byte, val uint32, data []byte, status, code byte) {
	data = append(append([]byte(nil), data...), status, code, 0, 0)
	h := make([]byte, 8)
	h[0], h[1] = 1, op
	binary.LittleEndian.PutUint16(h[2:], uint16(len(data)))
	binary.LittleEndian.PutUint32(h[4:], val)
	f.out = append(f.out, SlipEncode(append(h, data...))...)
}

func (f *fakeROM) word(d []byte, i int) uint32 { return binary.LittleEndian.Uint32(d[4*i:]) }

func (f *fakeROM) handle(fr []byte) {
	if len(fr) < 8 || fr[0] != 0 {
		f.t.Errorf("bad request frame % x", fr)
		return
	}
	op := fr[1]
	size := int(binary.LittleEndian.Uint16(fr[2:]))
	chk := binary.LittleEndian.Uint32(fr[4:])
	d := fr[8:]
	if size != len(d) {
		f.t.Errorf("op %#x: length field %d, data %d", op, size, len(d))
	}
	f.log = append(f.log, op)
	switch op {
	case CmdSync:
		if !bytes.Equal(d, SyncData()) {
			f.t.Errorf("bad sync payload")
		}
		if f.ignoreSyncs > 0 {
			f.ignoreSyncs--
			return
		}
		for range 8 {
			f.reply(op, 0x20120707, nil, 0, 0)
		}
	case CmdReadReg:
		f.reply(op, f.regs[f.word(d, 0)], nil, 0, 0)
	case CmdWriteReg:
		if len(d) != 16 {
			f.t.Errorf("write reg: %d bytes", len(d))
		}
		a, v, m := f.word(d, 0), f.word(d, 1), f.word(d, 2)
		f.regs[a] = f.regs[a]&^m | v&m
		f.reply(op, 0, nil, 0, 0)
	case CmdSPIAttach:
		if len(d) != 8 {
			f.t.Errorf("spi attach: ROM wants 8 bytes, got %d", len(d))
		}
		f.attached = true
		f.reply(op, 0, nil, 0, 0)
	case CmdSPISetParams:
		if len(d) != 24 {
			f.t.Errorf("set params: %d bytes", len(d))
		}
		f.flashSize = f.word(d, 1)
		f.reply(op, 0, nil, 0, 0)
	case CmdFlashDeflBegin, CmdFlashBegin:
		if op == CmdFlashDeflBegin && f.noDefl {
			f.reply(op, 0, nil, 1, 0x05)
			return
		}
		if len(d) != 20 || !f.attached {
			f.t.Errorf("begin: %d bytes, attached %v", len(d), f.attached)
		}
		eraseSize, blocks, bs, off := f.word(d, 0), f.word(d, 1), f.word(d, 2), f.word(d, 3)
		if bs != FlashWriteSize || off+eraseSize > f.flashSize || off%0x1000 != 0 {
			f.reply(op, 0, nil, 1, 0x06)
			return
		}
		if op == CmdFlashDeflBegin && eraseSize%FlashWriteSize != 0 {
			f.t.Errorf("defl begin: erase size %#x not block aligned", eraseSize)
		}
		for i := off; i < off+eraseSize; i++ {
			f.flash[i] = 0xFF
		}
		f.op, f.offset, f.blocks, f.seq, f.comp, f.writeLen = op, off, blocks, 0, nil, eraseSize
		f.reply(op, 0, nil, 0, 0)
	case CmdFlashDeflData, CmdFlashData:
		n, seq := f.word(d, 0), f.word(d, 1)
		payload := d[16:]
		if int(n) != len(payload) || Checksum(payload) != chk {
			f.reply(op, 0, nil, 1, 0x07)
			return
		}
		if f.failOnce[seq] {
			delete(f.failOnce, seq)
			f.reply(op, 0, nil, 1, 0x07) // as if a byte got lost
			return
		}
		if seq != f.seq || seq >= f.blocks || (op == CmdFlashData) != (f.op == CmdFlashBegin) {
			f.reply(op, 0, nil, 1, 0x06)
			return
		}
		f.seq++
		if op == CmdFlashData {
			if len(payload) != FlashWriteSize {
				f.t.Errorf("plain block of %d bytes", len(payload))
			}
			copy(f.flash[f.offset+seq*FlashWriteSize:], payload)
		} else {
			f.comp = append(f.comp, payload...)
			if f.seq == f.blocks {
				zr, err := zlib.NewReader(bytes.NewReader(f.comp))
				if err != nil {
					f.reply(op, 0, nil, 1, 0x0B)
					return
				}
				img, err := io.ReadAll(zr)
				if err != nil || uint32(len(img)) > f.writeLen {
					f.reply(op, 0, nil, 1, 0x0B)
					return
				}
				copy(f.flash[f.offset:], img)
			}
		}
		f.reply(op, 0, nil, 0, 0)
	case CmdSPIFlashMD5:
		a, n := f.word(d, 0), f.word(d, 1)
		sum := md5.Sum(f.flash[a : a+n])
		f.reply(op, 0, []byte(hex.EncodeToString(sum[:])), 0, 0)
	default:
		f.reply(op, 0, nil, 1, 0x05)
	}
}

func newTestLoader(f *fakeROM) *Loader {
	l := NewLoader(f)
	l.sleep = func(time.Duration) {}
	return l
}

func testImage(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i/13) ^ seed
	}
	return b
}

func TestFakeFlashSequence(t *testing.T) {
	f := newFakeROM(t, 1<<20)
	f.ignoreSyncs = 3
	l := newTestLoader(f)
	if err := l.Sync(10); err != nil {
		t.Fatal(err)
	}
	if err := l.Begin(uint32(len(f.flash)), false); err != nil {
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
	if last != len(pad4(app)) {
		t.Fatalf("progress ended at %d", last)
	}
	if !bytes.Equal(f.flash[0x10000:0x10000+len(app)], app) {
		t.Fatal("flash content differs")
	}
	if same, _ := l.Matches(0x10000, app); !same {
		t.Fatal("matches after write")
	}
	if err := l.ClearForceDownload(); err != nil {
		t.Fatal(err)
	}
	if err := l.WatchdogReset(); err != nil {
		t.Fatal(err)
	}
	if f.regs[rtcCntlWDTConfig0] != 0xD0000102 || f.regs[rtcCntlWDTConfig1] != 2000 {
		t.Fatalf("watchdog regs %#x", f.regs)
	}
	// Never FLASH_END / FLASH_DEFL_END: that would leave the ROM loader.
	for _, op := range f.log {
		if op == CmdFlashEnd || op == CmdFlashDeflEnd {
			t.Fatalf("sent end command %#x", op)
		}
	}
}

func TestFakeRetryAndFallback(t *testing.T) {
	f := newFakeROM(t, 1<<20)
	f.noDefl = true
	f.failOnce[1] = true
	l := newTestLoader(f)
	if err := l.Sync(3); err != nil {
		t.Fatal(err)
	}
	if err := l.Begin(uint32(len(f.flash)), true); err != nil {
		t.Fatal(err)
	}
	if f.regs[rtcCntlSWDConf]&rtcCntlSWDAutoFeed == 0 {
		t.Fatal("super watchdog not set to auto feed")
	}
	img := testImage(5000, 1)
	if err := l.WriteFlash(0x8000, img, nil); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.flash[0x8000:0x8000+len(img)], img) {
		t.Fatal("flash content differs")
	}
}

func TestFakeErrors(t *testing.T) {
	f := newFakeROM(t, 1<<20)
	f.ignoreSyncs = 100
	l := newTestLoader(f)
	if err := l.Sync(3); err == nil {
		t.Fatal("sync should fail")
	}
	f = newFakeROM(t, 1<<20)
	l = newTestLoader(f)
	_ = l.Sync(1)
	_ = l.Begin(64<<10, false) // the ROM now thinks the flash is 64 KiB
	err := l.WriteFlash(0x10000, testImage(100, 0), nil)
	if err == nil {
		t.Fatal("write beyond flash size should fail")
	}
	if err := l.WriteFlash(0x10, testImage(100, 0), nil); err == nil {
		t.Fatal("unaligned write should fail")
	}
}
