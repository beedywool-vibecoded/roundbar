// fwrelease lays out the firmware release assets from an ESP-IDF build:
//
//	go run ./tools/fwrelease -build firmware/build -fonts fonts.bin \
//	    -partitions firmware/partitions.csv -version v0.2.0 -out dist
//
// It writes the parts (bootloader.bin, partition-table.bin, roundbar.bin,
// fonts.bin), roundbar-full.bin (everything merged at offset 0, gaps filled
// with 0xFF, for first installs) and firmware.json (internal/fwupdate.Manifest).
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/beedywool-vibecoded/roundbar/internal/fwupdate"
)

// flasherArgs is the part of build/flasher_args.json we use.
type flasherArgs struct {
	FlashSettings struct {
		Mode string `json:"flash_mode"`
		Size string `json:"flash_size"`
		Freq string `json:"flash_freq"`
	} `json:"flash_settings"`
	FlashFiles map[string]string `json:"flash_files"`
	Extra      struct {
		Chip string `json:"chip"`
	} `json:"extra_esptool_args"`
}

// Release asset names per build file.
var assetNames = map[string][2]string{
	"bootloader/bootloader.bin":           {"bootloader", "bootloader.bin"},
	"partition_table/partition-table.bin": {"partition-table", "partition-table.bin"},
	"roundbar.bin":                        {"app", "roundbar.bin"},
}

func main() {
	build := flag.String("build", "firmware/build", "ESP-IDF build directory (with flasher_args.json)")
	fonts := flag.String("fonts", "", "font pack from tools/fontpack")
	parts := flag.String("partitions", "firmware/partitions.csv", "partition table CSV (for the fonts offset)")
	version := flag.String("version", "", "release tag, e.g. v0.2.0")
	out := flag.String("out", "dist", "output directory")
	flag.Parse()
	if *version == "" || *fonts == "" {
		log.Fatal("-version and -fonts are required")
	}
	m, files, err := layout(*build, *fonts, *parts, *version)
	if err != nil {
		log.Fatal(err)
	}
	if err := write(*out, m, files); err != nil {
		log.Fatal(err)
	}
}

func layout(build, fontsPath, partsCSV, version string) (*fwupdate.Manifest, map[string][]byte, error) {
	raw, err := os.ReadFile(filepath.Join(build, "flasher_args.json"))
	if err != nil {
		return nil, nil, err
	}
	var fa flasherArgs
	if err := json.Unmarshal(raw, &fa); err != nil {
		return nil, nil, fmt.Errorf("flasher_args.json: %w", err)
	}
	if fa.Extra.Chip != "esp32s3" {
		return nil, nil, fmt.Errorf("build is for %q, expected esp32s3", fa.Extra.Chip)
	}
	size, err := flashBytes(fa.FlashSettings.Size)
	if err != nil {
		return nil, nil, err
	}
	m := &fwupdate.Manifest{Version: version, Chip: "esp32s3", FlashMode: fa.FlashSettings.Mode,
		FlashFreq: fa.FlashSettings.Freq, FlashSize: fa.FlashSettings.Size, FlashBytes: size}
	files := map[string][]byte{}
	for off, rel := range fa.FlashFiles {
		name, ok := assetNames[rel]
		if !ok {
			return nil, nil, fmt.Errorf("unexpected flash file %s", rel)
		}
		o, err := strconv.ParseUint(off, 0, 32)
		if err != nil {
			return nil, nil, fmt.Errorf("offset %q: %w", off, err)
		}
		b, err := os.ReadFile(filepath.Join(build, rel))
		if err != nil {
			return nil, nil, err
		}
		if name[0] == "bootloader" {
			if err := checkHeader(b, fa.FlashSettings.Mode, fa.FlashSettings.Size, fa.FlashSettings.Freq); err != nil {
				return nil, nil, err
			}
		}
		m.Parts = append(m.Parts, fwupdate.PartFor(name[0], name[1], uint32(o), b))
		files[name[1]] = b
	}
	fontsOff, err := partitionOffset(partsCSV, "fonts")
	if err != nil {
		return nil, nil, err
	}
	fb, err := os.ReadFile(fontsPath)
	if err != nil {
		return nil, nil, err
	}
	m.Parts = append(m.Parts, fwupdate.PartFor("fonts", "fonts.bin", fontsOff, fb))
	files["fonts.bin"] = fb
	sort.Slice(m.Parts, func(i, j int) bool { return m.Parts[i].Offset < m.Parts[j].Offset })

	merged := merge(m.Parts, files)
	s := sha256.Sum256(merged)
	m.Merged = &fwupdate.Image{File: "roundbar-full.bin", Size: len(merged), SHA256: hex.EncodeToString(s[:])}
	files["roundbar-full.bin"] = merged

	// The same checks the updater applies.
	mj, _ := json.Marshal(m)
	if _, err := fwupdate.ParseManifest(mj); err != nil {
		return nil, nil, err
	}
	return m, files, nil
}

// merge places every part at its offset, 0xFF in between (esptool merge_bin).
func merge(parts []fwupdate.Part, files map[string][]byte) []byte {
	end := 0
	for _, p := range parts {
		end = max(end, int(p.Offset)+p.Size)
	}
	out := bytes.Repeat([]byte{0xFF}, end)
	for _, p := range parts {
		copy(out[p.Offset:], files[p.File])
	}
	return out
}

func flashBytes(s string) (uint32, error) {
	if n, ok := strings.CutSuffix(s, "MB"); ok {
		v, err := strconv.Atoi(n)
		if err == nil && v > 0 && v <= 128 {
			return uint32(v) << 20, nil
		}
	}
	return 0, fmt.Errorf("flash size %q", s)
}

// checkHeader makes sure the bootloader image header already carries the
// flash settings (esptool would patch them in; we write files unchanged).
// Byte 2 is the mode, byte 3 is size (high nibble) and frequency (low nibble).
func checkHeader(b []byte, mode, size, freq string) error {
	modes := map[string]byte{"qio": 0, "qout": 1, "dio": 2, "dout": 3}
	sizes := map[string]byte{"1MB": 0, "2MB": 1, "4MB": 2, "8MB": 3, "16MB": 4, "32MB": 5, "64MB": 6, "128MB": 7}
	freqs := map[string]byte{"80m": 0xF, "40m": 0x0, "26m": 0x1, "20m": 0x2}
	if len(b) < 4 || b[0] != 0xE9 {
		return errors.New("bootloader.bin: not an ESP image")
	}
	m, ok1 := modes[mode]
	sz, ok2 := sizes[size]
	f, ok3 := freqs[freq]
	if !ok1 || !ok2 || !ok3 {
		return fmt.Errorf("unknown flash settings %s/%s/%s", mode, size, freq)
	}
	if b[2] != m || b[3] != sz<<4|f {
		return fmt.Errorf("bootloader.bin header %02x %02x doesn't match %s/%s/%s", b[2], b[3], mode, size, freq)
	}
	return nil
}

// partitionOffset reads a partition's offset from the CSV table.
func partitionOffset(path, name string) (uint32, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comment = '#'
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	recs, err := r.ReadAll()
	if err != nil {
		return 0, err
	}
	for _, rec := range recs {
		if len(rec) >= 4 && strings.TrimSpace(rec[0]) == name {
			v, err := strconv.ParseUint(strings.TrimSpace(rec[3]), 0, 32)
			return uint32(v), err
		}
	}
	return 0, fmt.Errorf("%s: no %q partition", path, name)
}

func write(dir string, m *fwupdate.Manifest, files map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return err
		}
	}
	mj, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(filepath.Join(dir, fwupdate.ManifestFile), append(mj, '\n'), 0o644)
}
