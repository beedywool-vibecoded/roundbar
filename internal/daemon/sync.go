package daemon

// The setup travels with the board: the board keeps the config it was last
// given, and a PC that connects decides, from what it last agreed on with a
// board, whether to push its own config or adopt the board's (docs/spec.md).

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/assets"
	"github.com/beedywool-vibecoded/roundbar/internal/config"
)

type syncState struct {
	Synced string `json:"synced"` // sha both sides had after the last push or pull
}

func syncPath() string { return filepath.Join(config.Dir(), "sync.json") }

func loadSync() syncState {
	var s syncState
	if raw, err := os.ReadFile(syncPath()); err == nil {
		_ = json.Unmarshal(raw, &s)
	}
	return s
}

func saveSync(sha string) {
	raw, _ := json.Marshal(syncState{Synced: sha})
	_ = os.WriteFile(syncPath(), raw, 0o644)
}

type syncAction int

const (
	syncNone syncAction = iota
	syncPush            // the PC's config goes to the board
	syncPull            // the board's config becomes this PC's
)

// decide picks the direction. pcDefault: this PC still has the untouched
// default config (a fresh install). conflict: both sides changed since they
// last agreed; the board wins and the PC's version is kept as a backup.
func decide(pcSha, boardSha, synced string, pcDefault bool) (a syncAction, conflict bool) {
	switch {
	case boardSha == "": // the board has never been set up
		return syncPush, false
	case boardSha == pcSha:
		return syncNone, false
	case pcDefault:
		return syncPull, false
	case synced == "": // this PC has its own setup but never met a board
		return syncPull, true
	case pcSha == synced: // edited elsewhere since
		return syncPull, false
	case boardSha == synced: // edited here while unplugged
		return syncPush, false
	default:
		return syncPull, true
	}
}

// isDefault reports whether raw is the config written for a fresh install.
func isDefault(raw []byte) bool {
	def, err := json.MarshalIndent(config.Default(), "", "  ")
	return err == nil && bytes.Equal(bytes.TrimSpace(raw), bytes.TrimSpace(def))
}

// blob collects a chunked {"t":kind,"n","i","data"} dump from the device.
func (d *Daemon) blob(req map[string]any, kind string, timeout time.Duration) ([]byte, string, error) {
	ch := make(chan json.RawMessage, 64)
	d.mu.Lock()
	d.replies[kind] = ch
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.replies, kind); d.mu.Unlock() }()
	d.send(req)
	var out []byte
	for {
		select {
		case raw := <-ch:
			var c struct {
				N, I int
				Sha  string
				Data string
			}
			if err := json.Unmarshal(raw, &c); err != nil {
				return nil, "", err
			}
			if c.N == 0 {
				return nil, c.Sha, nil
			}
			b, err := base64.StdEncoding.DecodeString(c.Data)
			if err != nil {
				return nil, "", err
			}
			out = append(out, b...)
			if c.I == c.N-1 {
				return out, c.Sha, nil
			}
		case <-time.After(timeout):
			return nil, "", errors.New("the board stopped answering")
		}
	}
}

// syncOnHello runs when the board says hello with the sha of its stored config.
func (d *Daemon) syncOnHello(boardSha string) {
	d.mu.Lock()
	pcSha, raw := d.sha, d.raw
	d.mu.Unlock()
	action, conflict := decide(pcSha, boardSha, loadSync().Synced, isDefault(raw))
	switch action {
	case syncNone:
		saveSync(pcSha)
		go d.syncAssets()
	case syncPush:
		d.pushConfig() // records the sync once acked, then copies assets
	case syncPull:
		go func() {
			if err := d.pullConfig(conflict); err != nil {
				log.Printf("sync: %v; sending this PC's setup instead", err)
				d.pushConfig()
			}
		}()
	}
}

// pullConfig adopts the board's config and the images it references.
func (d *Daemon) pullConfig(conflict bool) error {
	raw, _, err := d.blob(map[string]any{"t": "configGet"}, "configDump", 10*time.Second)
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return errors.New("the board has no stored setup")
	}
	c, err := config.Parse(raw)
	if err != nil {
		return fmt.Errorf("the board's setup isn't valid: %w", err)
	}
	for name, ref := range assets.Wanted(c) {
		id := ref[0]
		if _, err := os.Stat(assets.Path(id)); err == nil {
			continue
		}
		data, _, err := d.blob(map[string]any{"t": "assetGet", "name": name}, "assetDump", 10*time.Second)
		if err != nil || len(data) == 0 {
			log.Printf("sync: image %s not on the board", name)
			continue
		}
		// The board keeps screen-sized copies: they serve as this PC's originals
		// and as the ready-made device versions.
		_ = os.MkdirAll(filepath.Join(assets.Dir(), "device"), 0o755)
		_ = os.WriteFile(assets.Path(id), data, 0o644)
		_ = os.WriteFile(filepath.Join(assets.Dir(), "device", name), data, 0o644)
	}
	note := "Loaded the setup stored on your Roundbar"
	if conflict {
		if old, err := os.ReadFile(config.Path()); err == nil {
			bak := config.Path() + ".before-sync-" + time.Now().Format("20060102-150405")
			if os.WriteFile(bak, old, 0o644) == nil {
				note += "; this PC's previous setup was saved as " + filepath.Base(bak)
			}
		}
	}
	saved, err := config.Save(c)
	if err != nil {
		return err
	}
	d.setConfig(c, saved)
	d.mu.Lock()
	d.syncNote = note
	d.mu.Unlock()
	log.Printf("sync: %s", note)
	d.pushConfig() // same content; gives the board this file's sha
	d.syncProfile(true)
	d.notify()
	return nil
}
