// Package fwupdate installs firmware releases without a toolchain: it reads
// the release manifest (firmware.json, written by tools/fwrelease), downloads
// and verifies the parts, puts the device in download mode and writes the
// parts whose flash contents differ, with internal/espflash.
package fwupdate

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/internal/espflash"
)

// Manifest describes one firmware release (release asset firmware.json).
type Manifest struct {
	Version    string `json:"version"` // the release tag, e.g. "v0.2.0"
	Chip       string `json:"chip"`    // "esp32s3"
	FlashMode  string `json:"flashMode"`
	FlashFreq  string `json:"flashFreq"`
	FlashSize  string `json:"flashSize"` // "16MB"
	FlashBytes uint32 `json:"flashBytes"`
	Parts      []Part `json:"parts"`
	// Merged is the single image at offset 0 for first installs (web installer).
	Merged *Image `json:"merged,omitempty"`
}

// Part is one region written at Offset.
type Part struct {
	Name   string `json:"name"` // "bootloader" | "partition-table" | "app" | "fonts"
	File   string `json:"file"` // release asset name
	Offset uint32 `json:"offset"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"` // of the file
	// FlashMD5 is the MD5 of the file as it sits in flash (padded with 0xFF to
	// a multiple of 4 bytes), compared with the chip's SPI_FLASH_MD5 so that an
	// unchanged part is neither downloaded nor written.
	FlashMD5 string `json:"flashMd5"`
}

type Image struct {
	File   string `json:"file"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
}

// PartFor fills a Part's size and digests from its contents.
func PartFor(name, file string, offset uint32, data []byte) Part {
	s := sha256.Sum256(data)
	m := md5.Sum(Padded(data))
	return Part{Name: name, File: file, Offset: offset, Size: len(data),
		SHA256: hex.EncodeToString(s[:]), FlashMD5: hex.EncodeToString(m[:])}
}

// Padded is data as the flasher writes it: padded to 4 bytes with 0xFF.
func Padded(data []byte) []byte { return espflash.Pad4(data) }

// flashSize is the size of the part in flash (padded to 4 bytes).
func (p Part) flashSize() uint32 { return uint32(p.Size+3) &^ 3 }

// ParseManifest decodes and sanity-checks a manifest.
func ParseManifest(raw []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("firmware manifest: %w", err)
	}
	if m.Version == "" || len(m.Parts) == 0 {
		return nil, errors.New("firmware manifest: no version or parts")
	}
	if m.Chip != "esp32s3" {
		return nil, fmt.Errorf("firmware manifest is for %q, not esp32s3", m.Chip)
	}
	if m.FlashBytes == 0 {
		return nil, errors.New("firmware manifest: no flash size")
	}
	for i, p := range m.Parts {
		if p.File == "" || strings.ContainsAny(p.File, "/\\") || p.Size <= 0 ||
			len(p.SHA256) != 64 || len(p.FlashMD5) != 32 || p.Offset%0x1000 != 0 ||
			uint64(p.Offset)+uint64(p.Size) > uint64(m.FlashBytes) {
			return nil, fmt.Errorf("firmware manifest: bad part %d (%s)", i, p.Name)
		}
		for _, q := range m.Parts[:i] {
			if p.Offset < q.Offset+uint32(q.Size) && q.Offset < p.Offset+uint32(p.Size) {
				return nil, fmt.Errorf("firmware manifest: %s overlaps %s", p.Name, q.Name)
			}
		}
	}
	return &m, nil
}

// Part returns the named part, if the release has it.
func (m *Manifest) Part(name string) (Part, bool) {
	for _, p := range m.Parts {
		if p.Name == name {
			return p, true
		}
	}
	return Part{}, false
}

// Verify checks data against the part's size and SHA-256.
func (p Part) Verify(data []byte) error {
	if len(data) != p.Size {
		return fmt.Errorf("%s: got %d bytes, expected %d", p.File, len(data), p.Size)
	}
	s := sha256.Sum256(data)
	if hex.EncodeToString(s[:]) != p.SHA256 {
		return fmt.Errorf("%s: SHA-256 mismatch (corrupted download?)", p.File)
	}
	return nil
}
