// Package espflash talks to the ESP32-S3 ROM serial bootloader: SLIP framing,
// the command/response packets, and the few commands needed to write and verify
// flash (SYNC, SPI_ATTACH, SPI_SET_PARAMS, FLASH_DEFL_*, SPI_FLASH_MD5, READ_REG,
// WRITE_REG). It follows the protocol described in Espressif's esptool
// documentation ("Serial Protocol") and esptool's loader.
//
// Only the ROM loader is used (no flasher stub), so there is nothing to upload
// to RAM and nothing chip-specific beyond a few register addresses.
package espflash

import (
	"errors"
	"time"
)

const (
	slipEnd    = 0xC0
	slipEsc    = 0xDB
	slipEscEnd = 0xDC
	slipEscEsc = 0xDD
)

// SlipEncode frames p: 0xC0, payload with 0xC0 → DB DC and 0xDB → DB DD, 0xC0.
func SlipEncode(p []byte) []byte {
	out := make([]byte, 0, len(p)+len(p)/16+2)
	out = append(out, slipEnd)
	for _, b := range p {
		switch b {
		case slipEnd:
			out = append(out, slipEsc, slipEscEnd)
		case slipEsc:
			out = append(out, slipEsc, slipEscEsc)
		default:
			out = append(out, b)
		}
	}
	return append(out, slipEnd)
}

// ErrTimeout is returned when no complete frame arrived in time.
var ErrTimeout = errors.New("timed out waiting for the bootloader")

// slipReader splits the incoming byte stream into SLIP frames. Bytes outside
// a frame (boot messages printed by the ROM) are skipped.
type slipReader struct {
	p      Port
	buf    []byte // unread bytes from the port
	frame  []byte
	in     bool
	escape bool
}

func newSlipReader(p Port) *slipReader { return &slipReader{p: p} }

// reset drops everything buffered (after a timeout or a resync).
func (r *slipReader) reset() {
	r.buf, r.frame, r.in, r.escape = nil, nil, false, false
}

// next returns the next complete frame, waiting until deadline.
func (r *slipReader) next(deadline time.Time) ([]byte, error) {
	tmp := make([]byte, 4096)
	for {
		for len(r.buf) > 0 {
			b := r.buf[0]
			r.buf = r.buf[1:]
			if !r.in {
				if b == slipEnd {
					r.in, r.frame = true, r.frame[:0]
				}
				continue
			}
			if r.escape {
				r.escape = false
				switch b {
				case slipEscEnd:
					r.frame = append(r.frame, slipEnd)
				case slipEscEsc:
					r.frame = append(r.frame, slipEsc)
				default: // invalid escape: drop the frame
					r.in = false
				}
				continue
			}
			switch b {
			case slipEsc:
				r.escape = true
			case slipEnd:
				if len(r.frame) == 0 {
					continue // back-to-back delimiters: the C0 opens the next frame
				}
				f := append([]byte(nil), r.frame...)
				r.in, r.frame = false, r.frame[:0]
				return f, nil
			default:
				r.frame = append(r.frame, b)
			}
		}
		if !time.Now().Before(deadline) {
			return nil, ErrTimeout
		}
		if err := r.p.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
		n, err := r.p.Read(tmp)
		if n > 0 {
			r.buf = append(r.buf, tmp[:n]...)
			continue
		}
		if err != nil {
			if isTimeout(err) {
				return nil, ErrTimeout
			}
			return nil, err
		}
	}
}

func isTimeout(err error) bool {
	var t interface{ Timeout() bool }
	return errors.As(err, &t) && t.Timeout()
}
