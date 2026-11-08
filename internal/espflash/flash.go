package espflash

import (
	"bytes"
	"compress/zlib"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"time"
)

// FlashWriteSize is the ROM loader's block size for flash writes.
const FlashWriteSize = 0x400

// writeAttempts per block, like esptool's WRITE_BLOCK_ATTEMPTS.
const writeAttempts = 3

// Compress is what goes over the wire for FLASH_DEFL_DATA: a zlib stream
// (header, deflate data, Adler-32), as esptool sends it.
func Compress(img []byte) []byte {
	var buf bytes.Buffer
	w, _ := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	_, _ = w.Write(img)
	_ = w.Close()
	return buf.Bytes()
}

// Pad4 pads to a multiple of 4 bytes with 0xFF (erased flash), as esptool does.
func Pad4(b []byte) []byte {
	if len(b)%4 == 0 {
		return b
	}
	out := append([]byte(nil), b...)
	for len(out)%4 != 0 {
		out = append(out, 0xFF)
	}
	return out
}

// countingReader counts how many compressed bytes the inflater consumed. It is
// an io.ByteReader, so flate reads exactly what it needs and no more.
type countingReader struct {
	b []byte
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.n >= len(c.b) {
		return 0, io.EOF
	}
	k := copy(p, c.b[c.n:])
	c.n += k
	return k, nil
}

func (c *countingReader) ReadByte() (byte, error) {
	if c.n >= len(c.b) {
		return 0, io.EOF
	}
	c.n++
	return c.b[c.n-1], nil
}

// uncompressedPerBlock estimates how many bytes the chip will write for each
// compressed block, to size the per-block timeout as esptool does.
func uncompressedPerBlock(comp []byte, blocks int) []int {
	out := make([]int, blocks)
	cr := &countingReader{b: comp}
	zr, err := zlib.NewReader(cr)
	if err != nil {
		return out
	}
	buf := make([]byte, 256)
	for {
		n, err := zr.Read(buf)
		if n > 0 {
			i := min(max((cr.n-1)/FlashWriteSize, 0), blocks-1)
			out[i] += n
		}
		if err != nil {
			return out
		}
	}
}

// WriteFlash erases and writes img at addr, then checks the region's MD5 on
// the chip. It sends the image compressed (FLASH_DEFL_*), falling back to
// plain FLASH_DATA if the loader refuses that. progress gets (bytes, total).
func (l *Loader) WriteFlash(addr uint32, img []byte, progress func(done, total int)) error {
	if len(img) == 0 {
		return errors.New("empty image")
	}
	if addr%0x1000 != 0 {
		return fmt.Errorf("address 0x%x is not sector aligned", addr)
	}
	img = Pad4(img)
	l.wrote = false
	err := l.writeDeflated(addr, img, progress)
	var se *StatusError
	if err != nil && !l.wrote && (errors.Is(err, errUnsupported) || errors.As(err, &se) && se.Code == 0x05) {
		l.logf("compressed writes not supported, sending uncompressed")
		err = l.writePlain(addr, img, progress)
	}
	if err != nil {
		return err
	}
	// No FLASH_END / FLASH_DEFL_END: on the ROM it leaves the loader (esptool skips it too).
	got, err := l.FlashMD5(addr, uint32(len(img)))
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	if want := md5.Sum(img); got != want {
		return fmt.Errorf("verify at 0x%x: flash MD5 %x, expected %x", addr, got, want)
	}
	return nil
}

// sendBlock sends one data packet, retrying like esptool.
func (l *Loader) sendBlock(op byte, seq int, block []byte, timeout time.Duration) error {
	var err error
	for range writeAttempts {
		if _, err = l.check(op, DataPacket(uint32(seq), block), Checksum(block), timeout); err == nil {
			l.wrote = true
			return nil
		}
		l.r.reset()
	}
	return err
}

func (l *Loader) writeDeflated(addr uint32, img []byte, progress func(done, total int)) error {
	comp := Compress(img)
	blocks := (len(comp) + FlashWriteSize - 1) / FlashWriteSize
	eraseBlocks := (len(img) + FlashWriteSize - 1) / FlashWriteSize
	writeSize := uint32(eraseBlocks * FlashWriteSize) // the ROM wants whole blocks here
	l.logf("erasing %d KiB at 0x%x", writeSize/1024, addr)
	// Fifth word: not an encrypted write (S2 and later ROMs take it).
	if _, err := l.check(CmdFlashDeflBegin, words(writeSize, uint32(blocks), FlashWriteSize, addr, 0), 0,
		timeoutPerMB(eraseTimeoutPerMB, int(writeSize))); err != nil {
		return fmt.Errorf("erase: %w", err)
	}
	l.logf("writing %d KiB (%d KiB compressed)", len(img)/1024, len(comp)/1024)
	unc := uncompressedPerBlock(comp, blocks)
	done := 0
	for seq := range blocks {
		block := comp[seq*FlashWriteSize : min((seq+1)*FlashWriteSize, len(comp))]
		if err := l.sendBlock(CmdFlashDeflData, seq, block, timeoutPerMB(eraseWriteTimeoutPerMB, unc[seq])); err != nil {
			return fmt.Errorf("write block %d/%d at 0x%x: %w", seq+1, blocks, addr, err)
		}
		done += unc[seq]
		if progress != nil {
			progress(min(done, len(img)), len(img))
		}
	}
	return nil
}

func (l *Loader) writePlain(addr uint32, img []byte, progress func(done, total int)) error {
	blocks := (len(img) + FlashWriteSize - 1) / FlashWriteSize
	l.logf("erasing %d KiB at 0x%x", len(img)/1024, addr)
	if _, err := l.check(CmdFlashBegin, words(uint32(len(img)), uint32(blocks), FlashWriteSize, addr, 0), 0,
		timeoutPerMB(eraseTimeoutPerMB, len(img))); err != nil {
		return fmt.Errorf("erase: %w", err)
	}
	block := make([]byte, FlashWriteSize)
	for seq := range blocks {
		for i := range block {
			block[i] = 0xFF // the last block is padded with erased bytes
		}
		n := copy(block, img[seq*FlashWriteSize:])
		if err := l.sendBlock(CmdFlashData, seq, block, defaultTimeout); err != nil {
			return fmt.Errorf("write block %d/%d at 0x%x: %w", seq+1, blocks, addr, err)
		}
		if progress != nil {
			progress(seq*FlashWriteSize+n, len(img))
		}
	}
	return nil
}

// Matches reports whether the flash at addr already holds img (same MD5),
// so unchanged parts (the 9 MB font pack) are not rewritten.
func (l *Loader) Matches(addr uint32, img []byte) (bool, error) {
	img = Pad4(img)
	got, err := l.FlashMD5(addr, uint32(len(img)))
	if err != nil {
		return false, err
	}
	return got == md5.Sum(img), nil
}
