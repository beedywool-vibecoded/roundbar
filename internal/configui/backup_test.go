package configui

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/beedywool-vibecoded/roundbar/internal/assets"
	"github.com/beedywool-vibecoded/roundbar/internal/config"
)

func TestBackupRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir()) // no daemon: reload is skipped
	if _, err := config.Save(config.Default()); err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(assets.Dir(), 0o755)
	_ = os.WriteFile(filepath.Join(assets.Dir(), "0123456789ab.jpg"), []byte("img"), 0o644)
	s := &server{}
	rec := httptest.NewRecorder()
	s.getBackup(rec, httptest.NewRequest("GET", "/api/backup", nil))
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(zr.File) != 2 {
		t.Fatalf("zip: %v %d files", err, len(zr.File))
	}
	_ = os.RemoveAll(assets.Dir())
	rec2 := httptest.NewRecorder()
	s.postRestore(rec2, httptest.NewRequest("POST", "/api/restore", bytes.NewReader(rec.Body.Bytes())))
	var r struct {
		OK     bool
		Images int
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &r)
	if !r.OK || r.Images != 1 {
		t.Fatalf("restore: %s", rec2.Body.String())
	}
	rec3 := httptest.NewRecorder()
	s.postRestore(rec3, httptest.NewRequest("POST", "/api/restore", bytes.NewReader([]byte("nope"))))
	if rec3.Code != 400 {
		t.Fatal("expected 400 for a non-zip")
	}
}
