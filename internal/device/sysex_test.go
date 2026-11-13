package device

import (
	"bytes"
	"math/rand"
	"testing"
)

func TestSysExRoundTrip(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	lines := [][]byte{{}, []byte(`{"t":"hello"}`), []byte("é 日本 🎮"), make([]byte, 7), make([]byte, 8), make([]byte, 50000)}
	for i := range lines[4] {
		lines[4][i] = 0xFF
	}
	r.Read(lines[5])
	var stream []byte
	for _, l := range lines {
		enc := encodeSysEx(l)
		for _, b := range enc[1 : len(enc)-1] {
			if b&0x80 != 0 {
				t.Fatalf("payload byte %#x has its top bit set", b)
			}
		}
		stream = append(stream, enc...)
		stream = append(stream, 0xF8, 0x90, 0x40, 0x7F) // clock + a note-on between messages
	}
	var got [][]byte
	var d sysexDecoder
	for len(stream) > 0 { // arbitrary read sizes
		n := min(len(stream), 1+r.Intn(300))
		d.feed(stream[:n], func(l []byte) { got = append(got, append([]byte(nil), l...)) })
		stream = stream[n:]
	}
	if len(got) != len(lines) {
		t.Fatalf("got %d lines, want %d", len(got), len(lines))
	}
	for i := range lines {
		if !bytes.Equal(got[i], lines[i]) {
			t.Fatalf("line %d differs", i)
		}
	}
}

func TestSysExIgnoresOthers(t *testing.T) {
	var n int
	var d sysexDecoder
	d.feed([]byte{0xF0, 0x41, 0x10, 0x20, 0xF7, 0xF0, 0x7D, 0x00, 0x7B, 0xF7}, func(l []byte) {
		n++
		if string(l) != "{" {
			t.Fatalf("got %q", l)
		}
	})
	if n != 1 {
		t.Fatalf("decoded %d messages", n)
	}
}
