package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLayout(t *testing.T) {
	dir := t.TempDir()
	w := func(name string, b []byte) {
		p := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	boot := append([]byte{0xE9, 0x03, 0x02, 0x4F}, bytes.Repeat([]byte{1}, 100)...)
	w("build/bootloader/bootloader.bin", boot)
	w("build/partition_table/partition-table.bin", bytes.Repeat([]byte{2}, 3072))
	w("build/roundbar.bin", bytes.Repeat([]byte{3}, 5000))
	w("fonts.bin", bytes.Repeat([]byte{4}, 777))
	w("build/flasher_args.json", []byte(`{
		"flash_settings": {"flash_mode": "dio", "flash_size": "16MB", "flash_freq": "80m"},
		"flash_files": {"0x0": "bootloader/bootloader.bin", "0x8000": "partition_table/partition-table.bin", "0x10000": "roundbar.bin"},
		"extra_esptool_args": {"chip": "esp32s3"}}`))
	w("partitions.csv", []byte("# Name, Type, SubType, Offset, Size\nfactory, app, factory, 0x10000, 0x200000\n# fonts\nfonts,    data, 0x40,    0x250000, 0x900000\n"))

	m, files, err := layout(filepath.Join(dir, "build"), filepath.Join(dir, "fonts.bin"), filepath.Join(dir, "partitions.csv"), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Parts) != 4 || m.Parts[3].Name != "fonts" || m.Parts[3].Offset != 0x250000 || m.FlashBytes != 16<<20 {
		t.Fatalf("%+v", m)
	}
	merged := files["roundbar-full.bin"]
	if len(merged) != 0x250000+777 || m.Merged.Size != len(merged) {
		t.Fatalf("merged size %d", len(merged))
	}
	if !bytes.Equal(merged[:len(boot)], boot) || merged[len(boot)] != 0xFF || merged[0x8000] != 2 || merged[0x10000] != 3 || merged[0x250000] != 4 {
		t.Fatal("merged layout")
	}

	// A bootloader built for other flash settings is refused.
	boot[3] = 0x2F
	w("build/bootloader/bootloader.bin", boot)
	if _, _, err := layout(filepath.Join(dir, "build"), filepath.Join(dir, "fonts.bin"), filepath.Join(dir, "partitions.csv"), "v1.0.0"); err == nil {
		t.Fatal("header mismatch accepted")
	}
}
