package daemon

import (
	"context"
	"encoding/base64"
	"log"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/cover"
	"github.com/beedywool-vibecoded/roundbar/internal/desktop"
	"github.com/beedywool-vibecoded/roundbar/internal/mangohud"
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
	var tick *time.Ticker
	var tickC <-chan time.Time
	for {
		select {
		case <-d.obs.Changed:
			if d.viewKind() == "obs" {
				d.sendObs()
			}
			d.notify()
		case <-tickC:
			if d.viewKind() == "obs" && d.obs.State().Recording {
				d.sendObs()
			}
		}
		// the clock only runs while a recording is on screen
		need := d.viewKind() == "obs" && d.obs.State().Recording
		if need && tick == nil {
			tick = time.NewTicker(time.Second)
			tickC = tick.C
		} else if !need && tick != nil {
			tick.Stop()
			tick, tickC = nil, nil
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

// ---- MangoHud ---------------------------------------------------------------

// startMango turns MangoHud logging on while the page is visible and streams
// a snapshot twice a second; stopMango turns it off and cleans up its files.
func (d *Daemon) startMango() {
	if changed, err := mangohud.EnsureConf(); err != nil {
		log.Printf("mangohud: %v", err)
	} else if changed {
		log.Printf("mangohud: added the control socket to %s (restart running games)", mangohud.ConfPath())
	}
	stop := make(chan struct{})
	d.mu.Lock()
	d.mangoStop = stop
	d.mu.Unlock()
	go func() {
		s := mangohud.Start()
		defer s.Stop()
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		began := time.Now()
		primed := false
		want := map[string]bool{"cpu": true, "gpu": true, "cpuTemp": true, "gpuTemp": true, "vram": true, "ram": true}
		for {
			sn := s.Poll()
			// Some games never let MangoHud log (DirectX 12 under Proton): after a
			// few seconds, show what Roundbar measures itself, without FPS.
			if !sn.Active && sn.Hint == "waiting" && time.Since(began) > 4*time.Second { // a game runs, but no data
				if !primed {
					d.stats.Sample(want) // CPU load is relative to the previous call
					primed = true
				}
				msg := map[string]any{"t": "mangohud", "active": false, "fallback": true}
				for k, v := range d.stats.Sample(want) {
					msg[k] = v
				}
				d.send(msg)
				select {
				case <-stop:
					return
				case <-t.C:
				}
				continue
			}
			d.send(map[string]any{"t": "mangohud", "active": sn.Active, "hint": sn.Hint, "fps": sn.FPS, "ft": sn.Frametime,
				"avg": sn.Avg, "low": sn.Low1, "cpu": sn.CPU, "gpu": sn.GPU, "cpuTemp": sn.CPUTemp, "gpuTemp": sn.GPUTemp,
				"power": sn.Power, "vram": sn.VRAM, "graph": sn.Graph})
			select {
			case <-stop:
				return
			case <-t.C:
			}
		}
	}()
}

func (d *Daemon) stopMango() {
	d.mu.Lock()
	if d.mangoStop != nil {
		close(d.mangoStop)
		d.mangoStop = nil
	}
	d.mu.Unlock()
}
