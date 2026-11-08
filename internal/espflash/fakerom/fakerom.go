// Package fakerom plays the ESP32-S3 ROM serial loader on the other end of an
// espflash.Port, for tests: it decodes SLIP frames (with its own decoder, not
// the one under test), checks every field the real ROM would, keeps a flash
// image and answers like the ROM (4 status bytes, MD5 as 32 hex characters).
package fakerom

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

// Commands, as in the esptool documentation.
const (
	flashBegin     = 0x02
	flashData      = 0x03
	flashEnd       = 0x04
	sync_          = 0x08
	writeReg       = 0x09
	readReg        = 0x0A
	spiSetParams   = 0x0B
	spiAttach      = 0x0D
	flashDeflBegin = 0x10
	flashDeflData  = 0x11
	flashDeflEnd   = 0x12
	spiFlashMD5    = 0x13
	blockSize      = 0x400
)

// ROM is a fake chip in download mode.
type ROM struct {
	mu    sync.Mutex
	Flash []byte
	Regs  map[uint32]uint32
	Ops   []byte // every command received, in order

	IgnoreSyncs int             // drop this many SYNCs (chip not ready yet)
	NoDefl      bool            // refuse FLASH_DEFL_BEGIN like a ROM without it
	FailOnce    map[uint32]bool // answer this data block seq with a CRC error once
	Errorf      func(format string, a ...any)
	Closed      bool

	out       []byte
	in        []byte
	inFrame   bool
	esc       bool
	frame     []byte
	flashSize uint32
	attached  bool

	op       byte
	offset   uint32
	blocks   uint32
	seq      uint32
	comp     []byte
	writeLen uint32
}

// New returns a chip with size bytes of erased flash. It starts by printing
// the ROM's boot banner, which the host must skip.
func New(size int) *ROM {
	r := &ROM{Flash: bytes.Repeat([]byte{0xFF}, size), Regs: map[uint32]uint32{}, FailOnce: map[uint32]bool{}}
	r.out = []byte("ESP-ROM:esp32s3-20210327\r\nboot:0x0 (DOWNLOAD(USB/UART0))\r\nwaiting for download\r\n")
	return r
}

type timeout struct{}

func (timeout) Error() string { return "i/o timeout" }
func (timeout) Timeout() bool { return true }

func (r *ROM) errorf(format string, a ...any) {
	if r.Errorf != nil {
		r.Errorf(format, a...)
	}
}

func (r *ROM) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.out) == 0 {
		return 0, timeout{}
	}
	n := copy(p, r.out)
	r.out = r.out[n:]
	return n, nil
}

func (r *ROM) SetReadDeadline(time.Time) error { return nil }

func (r *ROM) Close() error {
	r.mu.Lock()
	r.Closed = true
	r.mu.Unlock()
	return nil
}

func (r *ROM) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, b := range p {
		switch {
		case !r.inFrame:
			if b == 0xC0 {
				r.inFrame, r.frame = true, nil
			}
		case r.esc:
			r.esc = false
			switch b {
			case 0xDC:
				r.frame = append(r.frame, 0xC0)
			case 0xDD:
				r.frame = append(r.frame, 0xDB)
			default:
				r.errorf("bad SLIP escape 0xDB 0x%02x", b)
			}
		case b == 0xDB:
			r.esc = true
		case b == 0xC0:
			if len(r.frame) > 0 {
				r.handle(r.frame)
			}
			r.inFrame = false
		default:
			r.frame = append(r.frame, b)
		}
	}
	return len(p), nil
}

func (r *ROM) reply(op byte, val uint32, data []byte, status, code byte) {
	data = append(append([]byte(nil), data...), status, code, 0, 0)
	pkt := make([]byte, 8, 8+len(data))
	pkt[0], pkt[1] = 1, op
	binary.LittleEndian.PutUint16(pkt[2:], uint16(len(data)))
	binary.LittleEndian.PutUint32(pkt[4:], val)
	pkt = append(pkt, data...)
	r.out = append(r.out, 0xC0)
	for _, b := range pkt {
		switch b {
		case 0xC0:
			r.out = append(r.out, 0xDB, 0xDC)
		case 0xDB:
			r.out = append(r.out, 0xDB, 0xDD)
		default:
			r.out = append(r.out, b)
		}
	}
	r.out = append(r.out, 0xC0)
}

func word(d []byte, i int) uint32 {
	if len(d) < 4*i+4 {
		return 0
	}
	return binary.LittleEndian.Uint32(d[4*i:])
}

func (r *ROM) handle(fr []byte) {
	if len(fr) < 8 || fr[0] != 0 {
		r.errorf("bad request frame % x", fr)
		return
	}
	op := fr[1]
	size := int(binary.LittleEndian.Uint16(fr[2:]))
	chk := binary.LittleEndian.Uint32(fr[4:])
	d := fr[8:]
	if size != len(d) {
		r.errorf("op 0x%02x: length field %d, data %d", op, size, len(d))
	}
	r.Ops = append(r.Ops, op)
	switch op {
	case sync_:
		want := append([]byte{0x07, 0x07, 0x12, 0x20}, bytes.Repeat([]byte{0x55}, 32)...)
		if !bytes.Equal(d, want) {
			r.errorf("bad SYNC payload % x", d)
		}
		if r.IgnoreSyncs > 0 {
			r.IgnoreSyncs--
			return
		}
		for range 8 {
			r.reply(op, 0x20120707, nil, 0, 0)
		}
	case readReg:
		r.reply(op, r.Regs[word(d, 0)], nil, 0, 0)
	case writeReg:
		if len(d) != 16 {
			r.errorf("WRITE_REG: %d bytes", len(d))
		}
		a, v, m := word(d, 0), word(d, 1), word(d, 2)
		r.Regs[a] = r.Regs[a]&^m | v&m
		r.reply(op, 0, nil, 0, 0)
	case spiAttach:
		if len(d) != 8 {
			r.errorf("SPI_ATTACH: the ROM wants 8 bytes, got %d", len(d))
		}
		r.attached = true
		r.reply(op, 0, nil, 0, 0)
	case spiSetParams:
		if len(d) != 24 {
			r.errorf("SPI_SET_PARAMS: %d bytes", len(d))
		}
		r.flashSize = word(d, 1)
		r.reply(op, 0, nil, 0, 0)
	case flashDeflBegin, flashBegin:
		if op == flashDeflBegin && r.NoDefl {
			r.reply(op, 0, nil, 1, 0x05)
			return
		}
		if len(d) != 20 || !r.attached {
			r.errorf("begin: %d bytes (want 20), flash attached %v", len(d), r.attached)
		}
		erase, blocks, bs, off := word(d, 0), word(d, 1), word(d, 2), word(d, 3)
		if bs != blockSize || off%0x1000 != 0 || uint64(off)+uint64(erase) > uint64(min(r.flashSize, uint32(len(r.Flash)))) {
			r.reply(op, 0, nil, 1, 0x06)
			return
		}
		if op == flashDeflBegin && erase%blockSize != 0 {
			r.errorf("FLASH_DEFL_BEGIN: erase size 0x%x not block aligned", erase)
		}
		for i := off; i < off+erase; i++ {
			r.Flash[i] = 0xFF
		}
		r.op, r.offset, r.blocks, r.seq, r.comp, r.writeLen = op, off, blocks, 0, nil, erase
		r.reply(op, 0, nil, 0, 0)
	case flashDeflData, flashData:
		if len(d) < 16 {
			r.reply(op, 0, nil, 1, 0x05)
			return
		}
		n, seq := word(d, 0), word(d, 1)
		payload := d[16:]
		x := byte(0xEF)
		for _, b := range payload {
			x ^= b
		}
		if int(n) != len(payload) || uint32(x) != chk {
			r.reply(op, 0, nil, 1, 0x07)
			return
		}
		if r.FailOnce[seq] {
			delete(r.FailOnce, seq)
			r.reply(op, 0, nil, 1, 0x07)
			return
		}
		if seq != r.seq || seq >= r.blocks || (op == flashData) != (r.op == flashBegin) {
			r.reply(op, 0, nil, 1, 0x06)
			return
		}
		r.seq++
		if op == flashData {
			if len(payload) != blockSize {
				r.errorf("FLASH_DATA block of %d bytes", len(payload))
			}
			copy(r.Flash[r.offset+seq*blockSize:r.offset+r.writeLen], payload)
		} else {
			r.comp = append(r.comp, payload...)
			if r.seq == r.blocks {
				zr, err := zlib.NewReader(bytes.NewReader(r.comp))
				if err != nil {
					r.reply(op, 0, nil, 1, 0x0B)
					return
				}
				img, err := io.ReadAll(zr)
				if err != nil || uint32(len(img)) > r.writeLen {
					r.reply(op, 0, nil, 1, 0x0B)
					return
				}
				copy(r.Flash[r.offset:], img)
			}
		}
		r.reply(op, 0, nil, 0, 0)
	case spiFlashMD5:
		a, n := word(d, 0), word(d, 1)
		if uint64(a)+uint64(n) > uint64(len(r.Flash)) {
			r.reply(op, 0, nil, 1, 0x0A)
			return
		}
		sum := md5.Sum(r.Flash[a : a+n])
		r.reply(op, 0, []byte(hex.EncodeToString(sum[:])), 0, 0)
	case flashEnd, flashDeflEnd:
		r.errorf("0x%02x would leave the ROM loader", op)
		r.reply(op, 0, nil, 0, 0)
	default:
		r.reply(op, 0, nil, 1, 0x05)
	}
}

// Sent reports whether op was received.
func (r *ROM) Sent(op byte) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return bytes.IndexByte(r.Ops, op) >= 0
}

func (r *ROM) String() string { return fmt.Sprintf("fakerom(%d bytes)", len(r.Flash)) }
