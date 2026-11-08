package espflash

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"errors"
	"io"
	"testing"
	"time"
)

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSlipEncode(t *testing.T) {
	// Both special bytes get escaped, delimiters at both ends.
	got := SlipEncode([]byte{0x01, 0xC0, 0x02, 0xDB, 0x03})
	want := []byte{0xC0, 0x01, 0xDB, 0xDC, 0x02, 0xDB, 0xDD, 0x03, 0xC0}
	if !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}

// chunkPort hands out a fixed byte stream a few bytes at a time.
type chunkPort struct {
	data []byte
	n    int
}

func (c *chunkPort) Read(p []byte) (int, error) {
	if len(c.data) == 0 {
		return 0, timeoutErr{}
	}
	k := min(c.n, len(p), len(c.data))
	copy(p, c.data[:k])
	c.data = c.data[k:]
	return k, nil
}
func (c *chunkPort) Write(p []byte) (int, error)     { return len(p), nil }
func (c *chunkPort) SetReadDeadline(time.Time) error { return nil }

func TestSlipReader(t *testing.T) {
	a := []byte{0x01, 0x08, 0xC0, 0xDB, 0x00}
	b := []byte{0x01, 0x0A, 0x02}
	// ROM boot text before the first frame, back-to-back frames, split reads.
	stream := append([]byte("ESP-ROM:esp32s3\r\n"), SlipEncode(a)...)
	stream = append(stream, SlipEncode(b)...)
	r := newSlipReader(&chunkPort{data: stream, n: 3})
	dl := time.Now().Add(time.Second)
	for _, want := range [][]byte{a, b} {
		got, err := r.next(dl)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("got % x, want % x", got, want)
		}
	}
	if _, err := r.next(dl); !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout, got %v", err)
	}
}

func TestChecksum(t *testing.T) {
	if Checksum(nil) != 0xEF {
		t.Fatal("seed must be 0xEF")
	}
	if got := Checksum([]byte{0xC0, 0xDB, 0x01, 0xFF}); got != 0x0A {
		t.Fatalf("got %#x", got)
	}
}

// Packets as esptool builds them (struct.pack("<BBHI", 0, op, len, chk) + data),
// computed independently with Python's struct module.
func TestCommandFixtures(t *testing.T) {
	cases := []struct {
		name string
		got  []byte
		want string
	}{
		{"sync", Command(CmdSync, SyncData(), 0),
			"0008240000000000070712205555555555555555555555555555555555555555555555555555555555555555"},
		{"defl begin", Command(CmdFlashDeflBegin, words(0x2000, 3, 0x400, 0x10000, 0), 0),
			"00101400000000000020000003000000000400000000010000000000"},
		{"defl data", Command(CmdFlashDeflData, DataPacket(5, []byte{0xC0, 0xDB, 0x01, 0xFF}), Checksum([]byte{0xC0, 0xDB, 0x01, 0xFF})),
			"001114000a00000004000000050000000000000000000000c0db01ff"},
		{"spi attach", Command(CmdSPIAttach, words(0, 0), 0), "000d0800000000000000000000000000"},
		{"spi set params", Command(CmdSPISetParams, words(0, 16<<20, 64<<10, 4<<10, 256, 0xFFFF), 0),
			"000b1800000000000000000000000001000001000010000000010000ffff0000"},
		{"write reg", Command(CmdWriteReg, words(rtcCntlWDTWProtect, rtcCntlWDTKey, 0xFFFFFFFF, 0), 0),
			"0009100000000000b0800060a13ad850ffffffff00000000"},
		{"md5", Command(CmdSPIFlashMD5, words(0x10000, 4096, 0, 0), 0),
			"001310000000000000000100001000000000000000000000"},
	}
	for _, c := range cases {
		if h := hex.EncodeToString(c.got); h != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, h, c.want)
		}
	}
	// The SYNC frame on the wire: nothing to escape in it.
	f := SlipEncode(Command(CmdSync, SyncData(), 0))
	if f[0] != 0xC0 || f[len(f)-1] != 0xC0 || len(f) != 46 {
		t.Fatalf("sync frame % x", f)
	}
}

func TestParseResponse(t *testing.T) {
	// READ_REG reply from a ROM: value 0x09, 4 status bytes.
	r, ok := ParseResponse(unhex(t, "010a0400090000000000"+"0000"))
	if !ok || r.Op != CmdReadReg || r.Value != 9 || r.Status(0) != nil {
		t.Fatalf("%+v %v %v", r, ok, r.Status(0))
	}
	// Not a response (direction 0).
	if _, ok := ParseResponse(unhex(t, "0008000000000000")); ok {
		t.Fatal("request parsed as response")
	}
	// FLASH_DATA failed with "invalid CRC".
	r, _ = ParseResponse(unhex(t, "01030400000000000107"+"0000"))
	var se *StatusError
	if err := r.Status(0); !errors.As(err, &se) || se.Code != 0x07 {
		t.Fatalf("got %v", err)
	}
	if se.Error() == "" {
		t.Fatal("empty message")
	}
}

func TestParseMD5(t *testing.T) {
	// ROM: 32 hex chars then status. MD5 of "" is d41d8cd98f00b204e9800998ecf8427e.
	raw := unhex(t, "011324000000000000"[:16])
	raw = append(raw, []byte("d41d8cd98f00b204e9800998ecf8427e")...)
	raw = append(raw, 0, 0, 0, 0)
	r, ok := ParseResponse(raw)
	if !ok {
		t.Fatal("not parsed")
	}
	sum, err := ParseMD5(r)
	if err != nil || hex.EncodeToString(sum[:]) != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Fatalf("%x %v", sum, err)
	}
	// Failure status after the digest slot.
	raw[8+32] = 1
	raw[8+33] = 0x09
	r, _ = ParseResponse(raw)
	if _, err := ParseMD5(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestUncompressedPerBlock(t *testing.T) {
	img := make([]byte, 300_000)
	for i := range img {
		img[i] = byte(i * 7 % 251)
	}
	for i := 100_000; i < 200_000; i++ {
		img[i] = 0xFF // compresses to almost nothing
	}
	comp := Compress(img)
	blocks := (len(comp) + FlashWriteSize - 1) / FlashWriteSize
	per := uncompressedPerBlock(comp, blocks)
	total := 0
	for _, n := range per {
		total += n
	}
	if total != len(img) {
		t.Fatalf("blocks add up to %d, want %d", total, len(img))
	}
	// And it is a plain zlib stream.
	zr, err := zlib.NewReader(bytes.NewReader(comp))
	if err != nil {
		t.Fatal(err)
	}
	back, _ := io.ReadAll(zr)
	if !bytes.Equal(back, img) {
		t.Fatal("round trip")
	}
}

func TestPad4(t *testing.T) {
	if got := pad4([]byte{1, 2, 3, 4, 5}); !bytes.Equal(got, []byte{1, 2, 3, 4, 5, 0xFF, 0xFF, 0xFF}) {
		t.Fatalf("% x", got)
	}
}
