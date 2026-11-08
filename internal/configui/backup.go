package configui

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/assets"
	"github.com/beedywool-vibecoded/roundbar/internal/config"
	"github.com/beedywool-vibecoded/roundbar/internal/ipc"
)

var assetName = regexp.MustCompile(`^[0-9a-f]{12}\.(jpg|gif)$`)

// getBackup streams a zip of config.json plus every uploaded image.
func (s *server) getBackup(w http.ResponseWriter, r *http.Request) {
	_, raw, err := config.Load()
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="roundbar-backup-%s.zip"`, time.Now().Format("2006-01-02")))
	zw := zip.NewWriter(w)
	if f, err := zw.CreateHeader(&zip.FileHeader{Name: "config.json", Method: zip.Deflate, Modified: time.Now()}); err == nil {
		_, _ = f.Write(raw)
	}
	files, _ := filepath.Glob(filepath.Join(assets.Dir(), "*"))
	for _, p := range files {
		name := filepath.Base(p)
		if !assetName.MatchString(name) {
			continue // skips the device/ render cache, rebuilt on demand
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if f, err := zw.CreateHeader(&zip.FileHeader{Name: "assets/" + name, Method: zip.Store, Modified: time.Now()}); err == nil {
			_, _ = f.Write(b)
		}
	}
	_ = zw.Close()
}

// postRestore replaces the config and adds the images from a backup zip.
func (s *server) postRestore(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 256<<20))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error(), "")
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "that file isn't a Roundbar backup (zip)", "")
		return
	}
	var cfgRaw []byte
	images := map[string][]byte{}
	for _, f := range zr.File {
		name := path.Clean(f.Name)
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, err := io.ReadAll(io.LimitReader(rc, 32<<20))
		rc.Close()
		if err != nil {
			continue
		}
		switch {
		case name == "config.json":
			cfgRaw = b
		case path.Dir(name) == "assets" && assetName.MatchString(path.Base(name)):
			images[path.Base(name)] = b
		}
	}
	if cfgRaw == nil {
		jsonErr(w, http.StatusBadRequest, "the backup has no config.json", "")
		return
	}
	c, err := config.Parse(cfgRaw)
	var ce *config.Error
	if errors.As(err, &ce) {
		jsonErr(w, http.StatusBadRequest, "backup config is invalid: "+ce.Error(), ce.Path)
		return
	}
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error(), "")
		return
	}
	if err := os.MkdirAll(assets.Dir(), 0o755); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	for name, b := range images {
		if err := os.WriteFile(assets.Path(name), b, 0o644); err != nil {
			jsonErr(w, http.StatusInternalServerError, err.Error(), "")
			return
		}
	}
	if _, err := config.Save(c); err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	_ = ipc.Call(ipc.Request{Op: "reload"}, nil)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "images": len(images)})
}
