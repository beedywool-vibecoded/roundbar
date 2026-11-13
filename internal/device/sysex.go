package device

// MIDI transport: when the CDC serial port can't be opened (no udev rule, no
// sudo), the board is also a USB MIDI device, which the logged-in user may
// open on any systemd distribution (70-uaccess.rules tags sound devices).
// Each JSON line travels as one SysEx message:
//
//	F0 7D <payload> F7
//
// 7D is the "non-commercial" manufacturer id. The payload packs the line's
// bytes 7 at a time: a byte holding their top bits (bit i = top bit of byte
// i), then the 7 bytes' low 7 bits. docs/spec.md §2.

const (
	sysexStart = 0xF0
	sysexEnd   = 0xF7
	sysexID    = 0x7D
)

// encodeSysEx wraps one line (without its newline) in a SysEx message.
func encodeSysEx(line []byte) []byte {
	out := make([]byte, 0, 3+len(line)+len(line)/7+1)
	out = append(out, sysexStart, sysexID)
	for i := 0; i < len(line); i += 7 {
		group := line[i:min(i+7, len(line))]
		var msb byte
		for j, b := range group {
			msb |= (b >> 7) << j
		}
		out = append(out, msb)
		for _, b := range group {
			out = append(out, b&0x7F)
		}
	}
	return append(out, sysexEnd)
}

// sysexDecoder turns a MIDI byte stream back into lines. Bytes outside our
// SysEx messages (other MIDI traffic, real-time bytes) are ignored.
type sysexDecoder struct {
	in   bool   // inside F0 7D … F7
	head bool   // the next byte is the manufacturer id
	buf  []byte // packed payload so far
}

// feed consumes stream bytes and calls line for each complete message.
func (d *sysexDecoder) feed(p []byte, line func([]byte)) {
	for _, b := range p {
		switch {
		case b >= 0xF8: // real-time bytes may appear anywhere
		case b == sysexStart:
			d.in, d.head, d.buf = true, true, d.buf[:0]
		case !d.in:
		case b == sysexEnd:
			d.in = false
			if !d.head {
				line(unpack7(d.buf))
			}
		case b&0x80 != 0: // another status byte ends the message unfinished
			d.in = false
		case d.head:
			d.head = false
			if b != sysexID {
				d.in = false // someone else's SysEx
			}
		default:
			d.buf = append(d.buf, b)
		}
	}
}

func unpack7(p []byte) []byte {
	out := make([]byte, 0, len(p)*7/8+1)
	for i := 0; i < len(p); i += 8 {
		msb := p[i]
		for j, b := range p[i+1 : min(i+8, len(p))] {
			out = append(out, b|((msb>>j)&1)<<7)
		}
	}
	return out
}
