package daemon

import (
	"context"
	"encoding/base64"
	"log"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/cover"
	"github.com/beedywool-vibecoded/roundbar/internal/desktop"
	"github.com/beedywool-vibecoded/roundbar/internal/steam"
)

// ---- OBS --------------------------------------------------------------------

func (d *Daemon) sendObs() {
	st := d.obs.State()
	if len(st.Scenes) > 8 {
		st.Scenes = st.Scenes[:8]
	}
	d.send(map[string]any{"t": "obs", "connected": st.Connected, "recording": st.Recording, "paused": st.Paused,
		"recTime": st.RecTime, "streaming": st.Streaming, "scenes": st.Scenes, "current": st.Current})
}

// obsLoop forwards OBS changes, plus a once-a-second tick for the recording
// timer, only while an OBS page is on screen.
func (d *Daemon) obsLoop() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	for {
		select {
		case <-d.obs.Changed:
			if d.viewKind() == "obs" {
				d.sendObs()
			}
			d.notify()
		case <-tick.C:
			if d.viewKind() == "obs" && d.obs.State().Recording {
				d.sendObs()
			}
		}
	}
}

// ---- Steam games ------------------------------------------------------------

type gameInfo struct {
	AppID, Name string
	Since       time.Time
	artSent     string
}

// trackGame remembers the last Steam game that had focus (and since when).
func (d *Daemon) trackGame(f desktop.Focus) {
	id := steam.AppID(f.Class)
	if id == "" {
		id = steam.AppID(f.ID)
	}
	if id == "" {
		return
	}
	d.mu.Lock()
	if d.game.AppID != id {
		d.game = gameInfo{AppID: id, Name: steam.Name(id), Since: time.Now()}
		if d.game.Name == "" {
			d.game.Name = f.Name
		}
	}
	d.mu.Unlock()
	if d.viewKind() == "game" {
		go d.sendGame()
	}
}

func (d *Daemon) sendGame() {
	d.mu.Lock()
	g := d.game
	bg, blur := d.cfg.Media.CoverBackground, d.cfg.Media.BlurOrDefault()
	d.mu.Unlock()
	if g.AppID == "" {
		d.send(map[string]any{"t": "game", "name": ""})
		return
	}
	d.send(map[string]any{"t": "game", "name": g.Name, "appid": g.AppID, "since": g.Since.Unix()})
	if !bg || g.artSent == g.AppID {
		return
	}
	path := steam.Art(g.AppID)
	if path == "" {
		return
	}
	img, err := cover.Load(context.Background(), "file://"+path)
	if err != nil {
		log.Printf("game art: %v", err)
		return
	}
	d.mu.Lock()
	d.coverSeq++
	seq := d.coverSeq
	d.game.artSent = g.AppID
	d.mu.Unlock()
	chunks := cover.Chunks(cover.Blurred(img, blur), 32*1024)
	for i, c := range chunks {
		d.send(map[string]any{"t": "gameArt", "seq": seq, "n": len(chunks), "i": i, "w": cover.Size, "h": cover.Size,
			"data": base64.StdEncoding.EncodeToString(c)})
	}
}
