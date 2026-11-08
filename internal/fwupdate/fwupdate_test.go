package fwupdate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/espflash"
	"github.com/beedywool-vibecoded/roundbar/internal/espflash/fakerom"
)

func TestCompare(t *testing.T) {
	cases := []struct{ dev, rel, want string }{
		{"v0.2.0", "v0.2.0", UpToDate},
		{"v0.2.0-dirty", "v0.2.0", UpToDate},
		{"v0.1.9", "v0.2.0", UpdateAvailable},
		{"v0.1.0-12-gabc1234", "v0.2.0", UpdateAvailable},
		{"v0.2.0-3-gabc1234-dirty", "v0.2.0", DeviceNewer},
		{"v1.0.0", "v0.9.9", DeviceNewer},
		{"c2f6c71-dirty", "v0.2.0", Unknown},
		{"", "v0.2.0", Unknown},
		{"0.10.0", "v0.9.0", DeviceNewer},
	}
	for _, c := range cases {
		if got := Compare(c.dev, c.rel); got != c.want {
			t.Errorf("Compare(%q, %q) = %s, want %s", c.dev, c.rel, got, c.want)
		}
	}
}

type release struct {
	m     *Manifest
	files map[string][]byte
}

func makeRelease(t *testing.T, version string, app, fonts []byte) release {
	t.Helper()
	boot := bytes.Repeat([]byte{0xE9, 0x03, 0x02, 0x4F}, 1000)
	pt := bytes.Repeat([]byte{0xAA, 0x50}, 1536)
	r := release{files: map[string][]byte{"bootloader.bin": boot, "partition-table.bin": pt, "roundbar.bin": app, "fonts.bin": fonts}}
	r.m = &Manifest{Version: version, Chip: "esp32s3", FlashMode: "dio", FlashFreq: "80m", FlashSize: "1MB", FlashBytes: 1 << 20,
		Parts: []Part{
			PartFor("bootloader", "bootloader.bin", 0x0, boot),
			PartFor("partition-table", "partition-table.bin", 0x8000, pt),
			PartFor("app", "roundbar.bin", 0x10000, app),
			PartFor("fonts", "fonts.bin", 0x50000, fonts),
		}}
	raw, _ := json.Marshal(r.m)
	if _, err := ParseManifest(raw); err != nil {
		t.Fatal(err)
	}
	return r
}

func img(n int, seed byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*31/7) ^ seed
	}
	return b
}

func TestParseManifestRejects(t *testing.T) {
	r := makeRelease(t, "v1.0.0", img(1000, 1), img(1000, 2))
	bad := []func(m *Manifest){
		func(m *Manifest) { m.Chip = "esp32" },
		func(m *Manifest) { m.Parts[2].File = "../x" },
		func(m *Manifest) { m.Parts[2].Offset = 0x8100 },
		func(m *Manifest) { m.Parts[3].Offset = 0x10000 }, // overlaps the app
		func(m *Manifest) { m.Parts[3].Offset = 0xFF000; m.Parts[3].Size = 5000 }, // past the end of flash
		func(m *Manifest) { m.Parts[1].SHA256 = "x" },
	}
	for i, f := range bad {
		m := *r.m
		m.Parts = append([]Part(nil), r.m.Parts...)
		f(&m)
		raw, _ := json.Marshal(m)
		if _, err := ParseManifest(raw); err == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}

func TestFetcher(t *testing.T) {
	r := makeRelease(t, "v1.2.3", img(3000, 1), img(5000, 2))
	hits := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		hits[req.URL.Path]++
		switch {
		case req.URL.Path == "/rel/latest/download/firmware.json":
			_ = json.NewEncoder(w).Encode(r.m)
		case strings.HasPrefix(req.URL.Path, "/rel/download/v1.2.3/"):
			name := strings.TrimPrefix(req.URL.Path, "/rel/download/v1.2.3/")
			if name == "fonts.bin" {
				_, _ = w.Write([]byte("tampered"))
				return
			}
			_, _ = w.Write(r.files[name])
		default:
			http.NotFound(w, req)
		}
	}))
	defer srv.Close()
	f := &Fetcher{Releases: srv.URL + "/rel", CacheDir: t.TempDir(), Client: srv.Client()}
	m, err := f.Latest(context.Background())
	if err != nil || m.Version != "v1.2.3" {
		t.Fatal(m, err)
	}
	app, _ := m.Part("app")
	for range 2 {
		b, err := f.Part(context.Background(), m, app)
		if err != nil || !bytes.Equal(b, r.files["roundbar.bin"]) {
			t.Fatal(err)
		}
	}
	if hits["/rel/download/v1.2.3/roundbar.bin"] != 1 {
		t.Fatalf("app downloaded %d times, expected once (cache)", hits["/rel/download/v1.2.3/roundbar.bin"])
	}
	fonts, _ := m.Part("fonts")
	if _, err := f.Part(context.Background(), m, fonts); err == nil {
		t.Fatal("tampered download accepted")
	}
	f.Prune(m)
	if _, err := f.Part(context.Background(), m, app); err != nil {
		t.Fatal("pruned a part still in use")
	}
}

// harness wires an Updater to a fake chip that appears once the firmware is
// asked to reboot.
type harness struct {
	u         *Updater
	rom       *fakerom.ROM
	rebooted  bool
	fetched   map[string]int
	logs      []string
	lastTotal int
}

func newHarness(t *testing.T, r release, rom *fakerom.ROM) *harness {
	h := &harness{rom: rom, fetched: map[string]int{}}
	rom.Errorf = t.Errorf
	h.u = &Updater{
		Manifest: r.m,
		Part: func(_ context.Context, p Part) ([]byte, error) {
			h.fetched[p.Name]++
			return r.files[p.File], nil
		},
		RequestBootloader: func() error { h.rebooted = true; return nil },
		FindPort: func() (espflash.DownloadPort, bool) {
			return espflash.DownloadPort{Path: "/dev/ttyACM9", PID: espflash.PIDUSBOTG}, h.rebooted
		},
		OpenPort:   func(string) (SerialPort, error) { return rom, nil },
		Log:        func(s string) { h.logs = append(h.logs, s) },
		Progress:   func(done, total int) { h.lastTotal = total },
		ManualWait: 50 * time.Millisecond,
		sleep:      func(time.Duration) {},
	}
	return h
}

// put writes a part into the fake flash, as if an earlier release was installed.
func put(rom *fakerom.ROM, off uint32, b []byte) { copy(rom.Flash[off:], Padded(b)) }

func TestUpdateWritesOnlyWhatChanged(t *testing.T) {
	old := makeRelease(t, "v1.0.0", img(40_000, 1), img(100_000, 9))
	r := makeRelease(t, "v1.1.0", img(41_001, 2), img(100_000, 9)) // new app, same fonts
	rom := fakerom.New(1 << 20)
	for _, p := range old.m.Parts {
		put(rom, p.Offset, old.files[p.File])
	}
	rom.Regs[0x6000812C] = 1
	h := newHarness(t, r, rom)
	if err := h.u.Run(context.Background()); err != nil {
		t.Fatal(err, h.logs)
	}
	for _, p := range r.m.Parts {
		if !bytes.Equal(rom.Flash[p.Offset:int(p.Offset)+p.Size], r.files[p.File]) {
			t.Errorf("%s not in flash", p.Name)
		}
	}
	if h.fetched["fonts"] != 0 {
		t.Error("unchanged font pack was downloaded")
	}
	if h.lastTotal != len(Padded(r.files["roundbar.bin"])) {
		t.Errorf("wrote %d bytes, expected only the app", h.lastTotal)
	}
	if rom.Regs[0x6000812C] != 0 || rom.Regs[0x60008098] == 0 {
		t.Error("device not reset into the app")
	}
	if !rom.Closed {
		t.Error("port left open")
	}
}

func TestUpdateFetchesFontsWhenDifferent(t *testing.T) {
	r := makeRelease(t, "v1.1.0", img(20_000, 2), img(200_000, 3))
	rom := fakerom.New(1 << 20) // blank chip
	h := newHarness(t, r, rom)
	if err := h.u.Run(context.Background()); err != nil {
		t.Fatal(err, h.logs)
	}
	if h.fetched["fonts"] != 1 {
		t.Fatal("fonts not fetched")
	}
	for _, p := range r.m.Parts {
		if !bytes.Equal(rom.Flash[p.Offset:int(p.Offset)+p.Size], r.files[p.File]) {
			t.Errorf("%s not in flash", p.Name)
		}
	}
	// Running again changes nothing.
	h.fetched = map[string]int{}
	before := len(rom.Ops)
	if err := h.u.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, op := range rom.Ops[before:] {
		if op == espflash.CmdFlashDeflBegin {
			t.Fatal("rewrote an identical part")
		}
	}
}

func TestUpdateFailureKeepsDownloadMode(t *testing.T) {
	r := makeRelease(t, "v1.1.0", img(20_000, 2), img(1000, 3))
	rom := fakerom.New(1 << 20)
	rom.Errorf = t.Errorf
	h := newHarness(t, r, rom)
	h.u.Manifest.FlashBytes = 0x10000 // the ROM will refuse writes past this
	err := h.u.Run(context.Background())
	if err == nil {
		t.Fatal("expected failure")
	}
	if rom.Regs[0x60008098] != 0 {
		t.Fatal("reset after a failed write")
	}
}

func TestUpdateNoDevice(t *testing.T) {
	r := makeRelease(t, "v1.1.0", img(20_000, 2), img(1000, 3))
	h := newHarness(t, r, fakerom.New(1<<20))
	h.u.RequestBootloader = nil
	if err := h.u.Run(context.Background()); err != ErrNoDevice {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(fmt.Sprint(h.logs), "BOOT") {
		t.Fatal("no hint about the BOOT button")
	}
}
