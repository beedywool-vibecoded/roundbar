package espflash

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

// ROM loader commands (esptool ESP_CMDS).
const (
	CmdFlashBegin     = 0x02
	CmdFlashData      = 0x03
	CmdFlashEnd       = 0x04
	CmdSync           = 0x08
	CmdWriteReg       = 0x09
	CmdReadReg        = 0x0A
	CmdSPISetParams   = 0x0B
	CmdSPIAttach      = 0x0D
	CmdFlashDeflBegin = 0x10
	CmdFlashDeflData  = 0x11
	CmdFlashDeflEnd   = 0x12
	CmdSPIFlashMD5    = 0x13
)

// checksumSeed is the initial value of the data checksum (ESP_CHECKSUM_MAGIC).
const checksumSeed = 0xEF

// Checksum is the XOR of every byte of data, seeded with 0xEF. Only the data
// commands (FLASH_DATA, FLASH_DEFL_DATA, MEM_DATA) carry it, computed over
// the payload after their 16-byte header.
func Checksum(data []byte) uint32 {
	s := byte(checksumSeed)
	for _, b := range data {
		s ^= b
	}
	return uint32(s)
}

// Command builds a request packet (before SLIP framing):
// direction 0x00, command, u16 data length, u32 checksum, data. Little endian.
func Command(op byte, data []byte, chk uint32) []byte {
	p := make([]byte, 8+len(data))
	p[0] = 0x00
	p[1] = op
	binary.LittleEndian.PutUint16(p[2:], uint16(len(data)))
	binary.LittleEndian.PutUint32(p[4:], chk)
	copy(p[8:], data)
	return p
}

// words packs little-endian u32s, the parameter format of every command.
func words(v ...uint32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], x)
	}
	return b
}

// SyncData is the SYNC payload: 07 07 12 20 followed by 32 × 0x55.
func SyncData() []byte {
	d := []byte{0x07, 0x07, 0x12, 0x20}
	for range 32 {
		d = append(d, 0x55)
	}
	return d
}

// DataPacket is the payload of FLASH_DATA / FLASH_DEFL_DATA: data length,
// sequence number, two zero words, then the data. Its checksum is Checksum(data).
func DataPacket(seq uint32, data []byte) []byte {
	return append(words(uint32(len(data)), seq, 0, 0), data...)
}

// Response is a decoded reply: direction 0x01, command, u16 size, u32 value, data.
// For ROM loaders the data ends with status bytes (status, error, 2 reserved).
type Response struct {
	Op    byte
	Value uint32
	Data  []byte
}

// ParseResponse decodes a de-SLIPped frame. ok is false for frames that are
// not responses (direction byte other than 1, or too short).
func ParseResponse(f []byte) (r Response, ok bool) {
	if len(f) < 8 || f[0] != 0x01 {
		return r, false
	}
	size := int(binary.LittleEndian.Uint16(f[2:]))
	data := f[8:]
	if size < len(data) {
		data = data[:size]
	}
	return Response{Op: f[1], Value: binary.LittleEndian.Uint32(f[4:]), Data: data}, true
}

// Status checks the status bytes, which follow respLen bytes of command
// specific data (32 for the ROM's SPI_FLASH_MD5, 0 for everything else).
func (r Response) Status(respLen int) error {
	if len(r.Data) < respLen+2 {
		if len(r.Data) >= 2 && r.Data[0] != 0 {
			return &StatusError{Op: r.Op, Code: r.Data[1]}
		}
		return fmt.Errorf("command 0x%02x: short response (%d bytes)", r.Op, len(r.Data))
	}
	if r.Data[respLen] != 0 {
		return &StatusError{Op: r.Op, Code: r.Data[respLen+1]}
	}
	return nil
}

// StatusError is a failure reported by the ROM loader.
type StatusError struct {
	Op   byte
	Code byte
}

var romErrors = map[byte]string{
	0x05: "received message is invalid",
	0x06: "failed to act on received message",
	0x07: "invalid CRC in message",
	0x08: "flash write error",
	0x09: "flash read error",
	0x0A: "flash read length error",
	0x0B: "deflate error",
}

func (e *StatusError) Error() string {
	msg := romErrors[e.Code]
	if msg == "" {
		msg = "unknown error"
	}
	return fmt.Sprintf("bootloader command 0x%02x failed: %s (0x%02x)", e.Op, msg, e.Code)
}

// ParseMD5 extracts the digest from a SPI_FLASH_MD5 reply. The ROM sends it
// as 32 ASCII hex characters (the flasher stub would send 16 raw bytes).
func ParseMD5(r Response) ([16]byte, error) {
	var sum [16]byte
	if err := r.Status(32); err != nil {
		// The stub's raw form, in case someone talks to a stub-running chip.
		if r.Status(16) == nil {
			copy(sum[:], r.Data[:16])
			return sum, nil
		}
		return sum, err
	}
	if _, err := hex.Decode(sum[:], r.Data[:32]); err != nil {
		return sum, errors.New("bad MD5 in bootloader reply")
	}
	return sum, nil
}
