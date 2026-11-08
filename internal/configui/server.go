// Package configui serves the configurator: the embedded Vue app plus the API in
// docs/spec.md §4. It lives only while a browser tab keeps /api/events open.
package configui

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/assets"
	"github.com/beedywool-vibecoded/roundbar/internal/config"
	"github.com/beedywool-vibecoded/roundbar/internal/ipc"
	"github.com/beedywool-vibecoded/roundbar/shared"
)

//go:embed all:dist
var dist embed.FS

const (
	DevPort    = 7360
	idleExit   = 10 * time.Second
	firstVisit = 2 * time.Minute
)

type server struct {
	token string

	mu      sync.Mutex
	clients map[chan []byte]bool // payloads are complete SSE frames
	last    []byte
	idle    *time.Timer
}

// Run starts the server. open=false skips launching the browser.
func Run(open bool) error {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	s := &server{token: hex.EncodeToString(b), clients: map[chan []byte]bool{}}
	if t := os.Getenv("ROUNDBAR_TOKEN"); t != "" { // fixed token for `npm run dev`
		s.token = t
	}

	l, err := net.Listen("tcp", "127.0.0.1:7360")
	if err != nil {
		l, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return err
		}
	}
	url := "http://" + l.Addr().String() + "/#t=" + s.token
	log.Printf("configurator on %s", url)

	s.idle = time.AfterFunc(firstVisit, func() { log.Printf("no browser connected, exiting"); os.Exit(0) })
	go s.watchDaemon()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/config", s.auth(s.getConfig))
	mux.HandleFunc("PUT /api/config", s.auth(s.putConfig))
	mux.HandleFunc("GET /api/status", s.auth(s.getStatus))
	mux.HandleFunc("GET /api/icons", s.auth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(shared.IconsJSON)
	}))
	mux.HandleFunc("POST /api/show", s.auth(s.forward("show")))
	mux.HandleFunc("POST /api/run", s.auth(s.forward("run")))
	mux.HandleFunc("GET /api/events", s.auth(s.events))
	mux.HandleFunc("POST /api/assets", s.auth(s.postAsset))
	mux.HandleFunc("GET /api/firmware", s.auth(s.getFirmware))
	mux.HandleFunc("POST /api/discord", s.auth(s.postDiscord))
	mux.HandleFunc("POST /api/firmware/update", s.auth(s.postFirmware))
	mux.HandleFunc("GET /api/assets/{id}", s.auth(s.getAsset))
	mux.Handle("/", spa())

	if open {
		go func() {
			c := exec.Command("xdg-open", url)
			if err := c.Start(); err == nil {
				go c.Wait()
			}
		}()
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.Serve(l)
}

func (s *server) auth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only same-origin pages on 127.0.0.1 with the token get through.
		tok := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tok == "" {
			tok = r.URL.Query().Get("t") // EventSource can't set headers
		}
		if tok != s.token {
			jsonErr(w, http.StatusUnauthorized, "bad token", "")
			return
		}
		h(w, r)
	}
}

func jsonErr(w http.ResponseWriter, code int, msg, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg, "path": path})
}

func (s *server) getConfig(w http.ResponseWriter, r *http.Request) {
	_, raw, err := config.Load()
	if err != nil {
		// Hand back the defaults so a broken file can be repaired from the UI.
		raw, _ = json.Marshal(config.Default())
		w.Header().Set("X-Config-Error", err.Error())
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func (s *server) putConfig(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error(), "")
		return
	}
	c, err := config.Parse(body)
	if err == nil {
		_, err = config.Save(c)
	}
	var ce *config.Error
	if errors.As(err, &ce) {
		jsonErr(w, http.StatusBadRequest, ce.Msg, ce.Path)
		return
	}
	if err != nil {
		jsonErr(w, http.StatusInternalServerError, err.Error(), "")
		return
	}
	_ = ipc.Call(ipc.Request{Op: "reload"}, nil) // the daemon also sees the file change
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(c)
}

func (s *server) postAsset(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 21<<20))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error(), "")
		return
	}
	info, err := assets.Store(raw)
	if errors.Is(err, assets.ErrNotImage) {
		jsonErr(w, http.StatusUnsupportedMediaType, "that file isn't an image (JPEG, PNG, WebP or GIF)", "")
		return
	}
	if err != nil {
		jsonErr(w, http.StatusRequestEntityTooLarge, err.Error(), "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

// postDiscord saves the Discord app credentials (kept by the daemon in
// discord.json, never in the config that goes to the device).
func (s *server) postDiscord(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 4096))
	if err != nil {
		jsonErr(w, http.StatusBadRequest, "bad body", "")
		return
	}
	if err := ipc.Call(ipc.Request{Op: "discord", Action: body}, nil); err != nil {
		jsonErr(w, http.StatusBadRequest, err.Error(), "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func (s *server) getAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if strings.ContainsAny(id, "/\\") || strings.HasPrefix(id, ".") {
		jsonErr(w, http.StatusBadRequest, "bad id", "")
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable") // ids are content hashes
	http.ServeFile(w, r, assets.Path(id))
}

func (s *server) getStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var st map[string]any
	if err := ipc.Call(ipc.Request{Op: "status"}, &st); err != nil {
		_, _ = w.Write([]byte(`{"daemon":false}`))
		return
	}
	st["daemon"] = true
	_ = json.NewEncoder(w).Encode(st)
}

func (s *server) forward(op string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ipc.Request
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&req); err != nil {
			jsonErr(w, http.StatusBadRequest, "bad body", "")
			return
		}
		req.Op = op
		if err := ipc.Call(req, nil); err != nil {
			jsonErr(w, http.StatusServiceUnavailable, "daemon: "+err.Error(), "")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}
}

// watchDaemon relays daemon status changes to every SSE client.
func (s *server) watchDaemon() {
	for {
		err := ipc.Watch(func(line json.RawMessage) bool {
			var st map[string]any
			if json.Unmarshal(line, &st) != nil {
				return true
			}
			st["daemon"] = true
			b, _ := json.Marshal(st)
			s.broadcast(b)
			return true
		})
		_ = err
		s.broadcast([]byte(`{"daemon":false}`))
		time.Sleep(2 * time.Second)
	}
}

func (s *server) broadcast(b []byte) {
	s.mu.Lock()
	if string(b) == string(s.last) {
		s.mu.Unlock()
		return
	}
	s.last = b
	s.mu.Unlock()
	s.broadcastEvent("status", b)
}

// broadcastEvent sends one SSE event to every client.
func (s *server) broadcastEvent(event string, data []byte) {
	frame := []byte("event: " + event + "\ndata: " + string(data) + "\n\n")
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- frame:
		default:
		}
	}
}

func (s *server) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		jsonErr(w, http.StatusInternalServerError, "no streaming", "")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	ch := make(chan []byte, 64)
	s.mu.Lock()
	s.clients[ch] = true
	s.idle.Stop()
	last := s.last
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		if len(s.clients) == 0 {
			s.idle = time.AfterFunc(idleExit, func() { log.Printf("last tab closed, exiting"); os.Exit(0) })
		}
		s.mu.Unlock()
	}()
	write := func(frame []byte) bool {
		_, err := w.Write(frame)
		fl.Flush()
		return err == nil
	}
	if last != nil && !write([]byte("event: status\ndata: "+string(last)+"\n\n")) {
		return
	}
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case b := <-ch:
			if !write(b) {
				return
			}
		case <-ping.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		}
	}
}

func spa() http.Handler {
	sub, _ := fs.Sub(dist, "dist")
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p != "" {
			if _, err := fs.Stat(sub, p); err == nil {
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "configurator not built: run `npm run build` in web/", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}
