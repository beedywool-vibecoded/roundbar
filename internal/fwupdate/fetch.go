package fwupdate

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// DefaultReleases is where releases are published.
const DefaultReleases = "https://github.com/beedywool-vibecoded/roundbar/releases"

// ManifestFile is the release asset name of the manifest.
const ManifestFile = "firmware.json"

// Fetcher gets manifests and parts from GitHub releases, caching parts by
// SHA-256 so an unchanged font pack is downloaded once.
type Fetcher struct {
	Releases string // DefaultReleases unless ROUNDBAR_RELEASES says otherwise
	CacheDir string
	Client   *http.Client
	// Progress, if set, is called while a part downloads.
	Progress func(file string, done, total int64)
}

func NewFetcher() *Fetcher {
	base := os.Getenv("ROUNDBAR_RELEASES")
	if base == "" {
		base = DefaultReleases
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	return &Fetcher{
		Releases: base,
		CacheDir: filepath.Join(cache, "roundbar", "firmware"),
		Client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

func (f *Fetcher) get(ctx context.Context, url string, limit int64, onRead func(n int64)) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "roundbar")
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: not found (no release published yet?)", url)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	var r io.Reader = io.LimitReader(resp.Body, limit+1)
	if onRead != nil {
		r = &progressReader{r: r, fn: onRead}
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s: larger than expected", url)
	}
	return b, nil
}

type progressReader struct {
	r  io.Reader
	n  int64
	fn func(int64)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.n += int64(n)
	p.fn(p.n)
	return n, err
}

// Latest fetches the manifest of the latest release.
func (f *Fetcher) Latest(ctx context.Context) (*Manifest, error) {
	raw, err := f.get(ctx, f.Releases+"/latest/download/"+ManifestFile, 1<<20, nil)
	if err != nil {
		return nil, err
	}
	return ParseManifest(raw)
}

// Part returns the verified contents of a part, from the cache or downloaded.
func (f *Fetcher) Part(ctx context.Context, m *Manifest, p Part) ([]byte, error) {
	cached := filepath.Join(f.CacheDir, p.SHA256+".bin")
	if b, err := os.ReadFile(cached); err == nil && p.Verify(b) == nil {
		return b, nil
	}
	url := f.Releases + "/download/" + m.Version + "/" + p.File
	var onRead func(int64)
	if f.Progress != nil {
		onRead = func(n int64) { f.Progress(p.File, n, int64(p.Size)) }
	}
	b, err := f.get(ctx, url, int64(p.Size), onRead)
	if err != nil {
		return nil, err
	}
	if err := p.Verify(b); err != nil {
		return nil, err
	}
	if os.MkdirAll(f.CacheDir, 0o755) == nil {
		tmp := cached + ".tmp"
		if os.WriteFile(tmp, b, 0o644) == nil {
			_ = os.Rename(tmp, cached)
		}
	}
	return b, nil
}

// Prune deletes cached parts the manifest doesn't use.
func (f *Fetcher) Prune(m *Manifest) {
	keep := map[string]bool{}
	for _, p := range m.Parts {
		keep[p.SHA256+".bin"] = true
	}
	ents, _ := os.ReadDir(f.CacheDir)
	for _, e := range ents {
		if !keep[e.Name()] {
			_ = os.Remove(filepath.Join(f.CacheDir, e.Name()))
		}
	}
}
