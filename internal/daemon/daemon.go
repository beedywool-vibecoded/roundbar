// Package daemon is the long-running bridge between the device and the desktop.
// It is built to idle: every goroutine blocks on a channel, a socket or the
// kernel, and the only timer (stats) runs while a stats page is on screen.
package daemon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/assets"
	"github.com/beedywool-vibecoded/roundbar/internal/audio"
	"github.com/beedywool-vibecoded/roundbar/internal/config"
	"github.com/beedywool-vibecoded/roundbar/internal/cover"
	"github.com/beedywool-vibecoded/roundbar/internal/desktop"
	"github.com/beedywool-vibecoded/roundbar/internal/device"
	"github.com/beedywool-vibecoded/roundbar/internal/discord"
	"github.com/beedywool-vibecoded/roundbar/internal/keymap"
	"github.com/beedywool-vibecoded/roundbar/internal/obs"
	"github.com/beedywool-vibecoded/roundbar/internal/stats"
	"github.com/beedywool-vibecoded/roundbar/internal/steam"
)

const statsEvery = 1500 * time.Millisecond

type View struct {
	Profile string `json:"profile"`
	Page    string `json:"page"`
	Kind    string `json:"kind"`
	Awake   bool   `json:"awake"`
}

type hello struct {
	FW    string `json:"fw"`
	Proto int    `json:"proto"`
	Cfg   string `json:"cfg"`
	Touch string `json:"touch"`
}

type Daemon struct {
	mu               sync.Mutex
	cfg              *config.Config
	raw              []byte
	sha              string
	port             *device.Port
	openErr          string // why the last open failed ("permission"), shown in status
	hello            hello
	view             View
	focus            desktop.Focus
	recent           []desktop.Focus
	sentPro          string
	lastVol, lastMic string
	acks             map[string]chan bool
	ackSeq           int
	replies          map[string]chan json.RawMessage // device messages awaited by type (e.g. "assets")
	syncing, resync  bool                            // asset sync running / config changed meanwhile

	desk  *desktop.Desktop
	audio *audio.Audio
	stats *stats.Reader

	artSent   string             // artwork URL (+ settings) the device currently shows
	artCancel context.CancelFunc // in-flight artwork job
	coverSeq  int

	clockKick     chan struct{}
	discord       *discord.Client
	obs           *obs.Client
	game          gameInfo
	discordLive   discord.State // from Vesktop via DevTools
	discordLiveAt time.Time
	discordKick   chan struct{}
	cdp           *discord.CDP
	iconsSent     map[string]bool

	layout   keymap.Detected // the PC's active keyboard layout
	layoutMu sync.Mutex      // one detection at a time

	statsStop chan struct{}
	syncNote  string // last config sync with the board, for the configurator
	mangoStop chan struct{}
	watchers  map[chan struct{}]bool
	done      chan struct{}
}

func New() *Daemon {
	return &Daemon{obs: obs.New(), clockKick: make(chan struct{}, 1), discord: discord.New(), discordKick: make(chan struct{}, 1), iconsSent: map[string]bool{}, acks: map[string]chan bool{}, replies: map[string]chan json.RawMessage{}, watchers: map[chan struct{}]bool{}, done: make(chan struct{})}
}

func (d *Daemon) Run() error {
	cfg, raw, err := config.Load()
	if err != nil {
		log.Printf("config: %v (using defaults until it is fixed)", err)
		cfg = config.Default()
		raw, _ = json.MarshalIndent(cfg, "", "  ")
	}
	d.setConfig(cfg, raw)
	d.stats = stats.New()

	if d.desk, err = desktop.Connect(); err != nil {
		log.Printf("session bus: %v", err)
	} else {
		info := d.desk.Info()
		log.Printf("desktop: %s backend (focus %v, windows %v, notifications %v, clipboard %v)",
			info.Backend, info.Focus, info.Windows, info.Notifications, info.Clipboard)
		if f, ok := d.desk.CurrentFocus(); ok {
			d.onFocus(f)
		} else if info.Backend == desktop.BackendGnome {
			log.Printf("focus: GNOME extension not answering, per-app profiles disabled until it starts")
		}
		go d.desktopLoop()
	}
	go d.detectLayout(false) // for the configurator; sent to the device on hello
	go d.audioLoop()
	go d.clockLoop()
	go d.discord.Run(d.done, d.discordKick)
	go d.obs.Run(d.done)
	go d.obsLoop()
	cdp := &discord.CDP{OnState: d.setDiscordLive}
	d.cdp = cdp
	go cdp.Run(d.done)
	go d.discordLoop()
	go d.watchConfig()
	go d.serveSocket()
	d.deviceLoop() // blocks
	return nil
}

func (d *Daemon) setConfig(c *config.Config, raw []byte) {
	d.mu.Lock()
	d.cfg, d.raw, d.sha = c, raw, config.Sha(raw)
	d.mu.Unlock()
}

// ---- device -------------------------------------------------------------

func (d *Daemon) deviceLoop() {
	for {
		path := device.Find()
		if path == "" {
			d.mu.Lock()
			stale := d.openErr != ""
			d.openErr = ""
			d.mu.Unlock()
			if stale {
				d.notify()
			}
			device.WaitHotplug(d.done)
			continue
		}
		p, err := device.Open(path)
		if err != nil {
			why := ""
			if errors.Is(err, os.ErrPermission) { // EACCES/EPERM, or access denied on Windows
				why = "permission"
				log.Printf("open %s: %v (install the udev rule: roundbar udev)", path, err)
			} else {
				log.Printf("open %s: %v", path, err)
			}
			d.mu.Lock()
			d.openErr = why
			d.mu.Unlock()
			d.notify()
			device.WaitHotplug(d.done)
			continue
		}
		log.Printf("device on %s", path)
		d.mu.Lock()
		d.openErr = ""
		d.port = p
		d.sentPro, d.lastVol, d.lastMic, d.artSent = "", "", "", ""
		d.iconsSent = map[string]bool{}
		d.game.artSent = ""
		d.mu.Unlock()
		d.notify()
		_ = p.Send(map[string]any{"t": "hello"})
		err = p.Read(d.onDeviceMsg)
		log.Printf("device gone: %v", err)
		p.Close()
		d.mu.Lock()
		d.port = nil
		d.view = View{}
		d.hello = hello{}
		d.mu.Unlock()
		d.stopStats()
		d.notify()
		time.Sleep(300 * time.Millisecond)
	}
}

func (d *Daemon) send(v any) {
	d.mu.Lock()
	p := d.port
	d.mu.Unlock()
	if p == nil {
		return
	}
	if err := p.Send(v); err != nil {
		log.Printf("send: %v", err)
	}
}

// sendWait sends a message with an id and waits for the device's ack.
func (d *Daemon) sendWait(m map[string]any, timeout time.Duration) bool {
	d.mu.Lock()
	d.ackSeq++
	id := fmt.Sprintf("m%d", d.ackSeq)
	ch := make(chan bool, 1)
	d.acks[id] = ch
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.acks, id); d.mu.Unlock() }()
	m["id"] = id
	d.send(m)
	select {
	case ok := <-ch:
		return ok
	case <-time.After(timeout):
		return false
	}
}

func (d *Daemon) pushConfig() {
	d.mu.Lock()
	var buf bytes.Buffer
	err := json.Compact(&buf, d.raw)
	sha := d.sha
	d.mu.Unlock()
	if err != nil {
		return
	}
	go func() {
		if !d.sendWait(map[string]any{"t": "config", "cfg": json.RawMessage(buf.Bytes()), "sha": sha}, 5*time.Second) {
			log.Printf("device did not ack the config")
			return
		}
		d.mu.Lock()
		d.hello.Cfg = sha
		d.mu.Unlock()
		saveSync(sha)
		d.notify()
		d.syncAssets()
	}()
}

func (d *Daemon) onDeviceMsg(m device.Msg) {
	switch m.T {
	case "hello":
		var h hello
		_ = json.Unmarshal(m.Raw, &h)
		d.mu.Lock()
		d.hello = h
		d.sentPro = ""
		d.mu.Unlock()
		log.Printf("device fw %s, touch %s", h.FW, h.Touch)
		d.syncOnHello(h.Cfg) // push, pull or nothing; copies assets after
		d.syncProfile(true)
		d.notify()
		d.kickClock()
		go d.sendNotifs("")
		go d.detectLayout(true)
	case "view":
		var v View
		_ = json.Unmarshal(m.Raw, &v)
		d.mu.Lock()
		d.view = v
		d.lastVol, d.lastMic = "", ""
		d.mu.Unlock()
		d.onView(v)
		d.notify()
	case "action":
		var a struct {
			Action config.Action `json:"action"`
		}
		if json.Unmarshal(m.Raw, &a) == nil {
			go d.run(a.Action)
		}
	case "obsCmd":
		var v struct {
			Request, Scene, Input string
		}
		_ = json.Unmarshal(m.Raw, &v)
		go func() {
			if err := d.obs.Do(v.Request, v.Scene, v.Input); err != nil {
				log.Printf("obs: %v", err)
			}
		}()
	case "gameClose": // confirmed on the device's game page
		d.mu.Lock()
		id := d.game.AppID
		d.mu.Unlock()
		go func() {
			n, err := steam.Close(id)
			if err != nil {
				log.Printf("game: %v", err)
				return
			}
			log.Printf("game: closing app %s (%d processes)", id, n)
			d.mu.Lock()
			d.game = gameInfo{}
			d.mu.Unlock()
			if d.viewKind() == "game" {
				d.sendGame()
			}
		}()
	case "discordToggle":
		var v struct {
			What string `json:"what"`
		}
		_ = json.Unmarshal(m.Raw, &v)
		go func() {
			toggle := d.discord.Toggle // official client (RPC)
			if d.cdp != nil && d.cdp.Connected() {
				toggle = d.cdp.Toggle // Vesktop
			}
			if err := toggle(v.What); err != nil {
				log.Printf("discord: %v", err)
			}
		}()
	case "activate", "mixerSet", "mixerMute", "output", "paste", "notifOpen", "notifDismiss", "notifClear":
		d.onShellMsg(m.Raw, m.T)
	case "timerDone":
		var v struct {
			Label string `json:"label"`
			Phase string `json:"phase"`
		}
		_ = json.Unmarshal(m.Raw, &v)
		d.timerDone(v.Label, v.Phase)
	case "dial":
		var v struct {
			Source string `json:"source"`
			Value  *int   `json:"value"`
		}
		if json.Unmarshal(m.Raw, &v) != nil || v.Value == nil || d.audio == nil {
			return
		}
		switch v.Source {
		case "volume":
			_ = d.audio.SetVolume(*v.Value)
		case "mic":
			_ = d.audio.SetMicVolume(*v.Value)
		}
	case "ack":
		var a struct {
			ID string `json:"id"`
			OK bool   `json:"ok"`
		}
		_ = json.Unmarshal(m.Raw, &a)
		d.mu.Lock()
		ch := d.acks[a.ID]
		d.mu.Unlock()
		if ch != nil {
			ch <- a.OK
		}
	case "log":
		log.Printf("device: %s", m.Raw)
	default:
		d.mu.Lock()
		ch := d.replies[m.T]
		d.mu.Unlock()
		if ch != nil {
			select {
			case ch <- m.Raw:
			default:
			}
		}
	}
}

// page returns the config page the device reports as visible.
func (d *Daemon) page(v View) *config.Page {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, p := range d.cfg.Profiles {
		if p.ID != v.Profile {
			continue
		}
		for i := range p.Pages {
			if p.Pages[i].ID == v.Page {
				return &p.Pages[i]
			}
		}
	}
	return nil
}

func (d *Daemon) onView(v View) {
	d.stopStats()
	d.stopMango()
	if !v.Awake {
		return
	}
	pg := d.page(v)
	switch v.Kind {
	case "stats":
		if pg != nil {
			d.startStats(pg.Items)
		}
	case "media":
		d.sendMedia()
		d.sendAudio()
	case "dial":
		d.sendAudio()
	case "obs":
		d.obs.Kick() // connects if OBS is open; state follows on Changed
		go d.sendObs()
	case "mangohud":
		d.startMango()
	case "game":
		go d.sendGame()
	default:
		d.onShellView(v.Kind)
	}
}

// ---- stats --------------------------------------------------------------

func (d *Daemon) startStats(items []string) {
	want := map[string]bool{}
	for _, it := range items {
		want[it] = true
	}
	stop := make(chan struct{})
	d.mu.Lock()
	d.statsStop = stop
	d.mu.Unlock()
	go func() {
		// Stops helpers such as the nvidia-smi child once the page is gone. If another
		// stats page already started, its next Sample starts them again.
		defer d.stats.Stop()
		d.stats.Sample(want) // primes the CPU delta (and starts nvidia-smi)
		t := time.NewTicker(statsEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				msg := map[string]any{"t": "stats"}
				for k, v := range d.stats.Sample(want) {
					msg[k] = v
				}
				d.send(msg)
			}
		}
	}()
}

func (d *Daemon) stopStats() {
	d.mu.Lock()
	if d.statsStop != nil {
		close(d.statsStop)
		d.statsStop = nil
	}
	d.mu.Unlock()
}

// ---- desktop ------------------------------------------------------------

func (d *Daemon) desktopLoop() {
	for {
		select {
		case f := <-d.desk.FocusChanged:
			d.onFocus(f)
		case <-d.desk.LayoutChanged:
			go d.detectLayout(false)
		case <-d.desk.WindowsChanged:
			if d.viewKind() == "windows" {
				d.sendWindows()
			}
		case <-d.desk.ClipsChanged:
			if d.viewKind() == "clipboard" {
				d.sendClips()
			}
		case <-d.desk.NotifsChanged:
			d.sendNotifs("")
		case id := <-d.desk.NotifNew:
			d.sendNotifs(id)
		case <-d.desk.MediaChanged:
			d.mu.Lock()
			on := d.view.Awake && d.view.Kind == "media"
			d.mu.Unlock()
			if on {
				d.sendMedia()
			}
		}
	}
}

func (d *Daemon) onFocus(f desktop.Focus) {
	f.ID, f.Class = strings.ToLower(f.ID), strings.ToLower(f.Class)
	if f.ID == "" && f.Class == "" {
		return // desktop / shell overview: keep the current profile
	}
	if f.ID == "" {
		f.ID = f.Class
	}
	d.mu.Lock()
	d.focus = f
	if f.ID != "org.gnome.shell" && f.ID != "gnome-shell" {
		rec := []desktop.Focus{f}
		for _, r := range d.recent {
			if r.ID != f.ID && len(rec) < 16 {
				rec = append(rec, r)
			}
		}
		d.recent = rec
	}
	d.mu.Unlock()
	d.syncProfile(false)
	d.trackGame(f)
	d.notify()
}

// matches: exact app id / window class, or a "*" pattern like "steam_app_*".
func matches(pattern string, f desktop.Focus) bool {
	if pattern == f.ID || pattern == f.Class || pattern+".desktop" == f.ID {
		return true
	}
	if strings.Contains(pattern, "*") {
		for _, v := range []string{f.ID, f.Class} {
			if ok, _ := path.Match(pattern, v); ok && v != "" {
				return true
			}
		}
	}
	return false
}

func (d *Daemon) profileFor(f desktop.Focus) (string, string) {
	if d.cfg == nil { // asked (by the tray app) before Run loaded the config
		return "default", "Default"
	}
	for _, p := range d.cfg.Profiles {
		for _, m := range p.Match {
			if matches(m, f) {
				return p.ID, p.Name
			}
		}
	}
	return "default", "Default"
}

func (d *Daemon) syncProfile(force bool) {
	d.mu.Lock()
	id, _ := d.profileFor(d.focus)
	f := d.focus
	changed := force || id != d.sentPro
	d.sentPro = id
	d.mu.Unlock()
	if !changed {
		return
	}
	d.send(map[string]any{"t": "profile", "id": id})
	if f.Name != "" && !force {
		d.send(map[string]any{"t": "app", "id": f.ID, "name": f.Name})
	}
}

func (d *Daemon) sendMedia() {
	if d.desk == nil {
		return
	}
	m, ok := d.desk.Media()
	d.syncCover(m.ArtURL)
	if !ok {
		d.send(map[string]any{"t": "media", "title": "", "artist": "", "album": "", "playing": false, "player": ""})
		return
	}
	d.send(map[string]any{"t": "media", "title": m.Title, "artist": m.Artist, "album": m.Album,
		"playing": m.Playing, "canNext": m.CanNext, "canPrev": m.CanPrev, "player": m.Player})
}

// ---- image page assets -----------------------------------------------------

// request sends m and waits for the next device message of type `reply`.
func (d *Daemon) request(m map[string]any, reply string, timeout time.Duration) (json.RawMessage, bool) {
	ch := make(chan json.RawMessage, 1)
	d.mu.Lock()
	d.replies[reply] = ch
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.replies, reply); d.mu.Unlock() }()
	d.send(m)
	select {
	case r := <-ch:
		return r, true
	case <-time.After(timeout):
		return nil, false
	}
}

// syncAssets copies the images the config references to the device and
// prunes the rest. One run at a time; a config change mid-run triggers another.
func (d *Daemon) syncAssets() {
	d.mu.Lock()
	if d.syncing {
		d.resync = true
		d.mu.Unlock()
		return
	}
	d.syncing = true
	want := assets.Wanted(d.cfg)
	connected := d.port != nil
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.syncing = false
		again := d.resync
		d.resync = false
		d.mu.Unlock()
		if again {
			d.syncAssets()
		}
	}()
	if !connected {
		return
	}
	var raw json.RawMessage
	ok := false
	for try := 0; try < 3 && !ok; try++ { // right after connecting the device may still be building its screen
		raw, ok = d.request(map[string]any{"t": "assetsWant"}, "assets", 5*time.Second)
	}
	if !ok {
		log.Printf("assets: device did not answer")
		return
	}
	var have struct {
		Have []string `json:"have"`
		Free int64    `json:"free"`
	}
	_ = json.Unmarshal(raw, &have)
	present := map[string]bool{}
	for _, n := range have.Have {
		present[n] = true
	}
	keep := make([]string, 0, len(want))
	sent := 0
	for name, src := range want {
		keep = append(keep, name)
		if present[name] {
			continue
		}
		data, err := assets.DeviceFile(src[0], src[1])
		if err != nil {
			log.Printf("assets: %s: %v", src[0], err)
			continue
		}
		chunks := cover.Chunks(data, 32*1024)
		for i, c := range chunks {
			if !d.sendWait(map[string]any{"t": "asset", "name": name, "n": len(chunks), "i": i,
				"data": base64.StdEncoding.EncodeToString(c)}, 10*time.Second) {
				log.Printf("assets: device rejected %s (full?)", name)
				return
			}
		}
		sent++
	}
	d.send(map[string]any{"t": "assetsPrune", "keep": keep})
	d.send(map[string]any{"t": "assetsDone"})
	if sent > 0 {
		log.Printf("assets: sent %d image(s) to the device", sent)
	}
}

// ---- album art ------------------------------------------------------------

// syncCover makes the device show the artwork for artURL (blurred background
// and/or palette, per config). Work runs off the caller's goroutine and is
// cancelled when the track changes again.
func (d *Daemon) syncCover(artURL string) {
	d.mu.Lock()
	bg, colors, blur := d.cfg.Media.CoverBackground, d.cfg.Media.CoverColors, d.cfg.Media.BlurOrDefault()
	dark := d.cfg.Theme.Mode != "light"
	errs := map[string]string{}
	for _, k := range []string{"error", "errorContainer", "onErrorContainer"} {
		errs[k] = d.cfg.Theme.Palette[k]
	}
	key := fmt.Sprintf("%s|%v|%v|%v|%d", artURL, bg, colors, dark, blur)
	if key == d.artSent {
		d.mu.Unlock()
		return
	}
	d.artSent = key
	if d.artCancel != nil {
		d.artCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.artCancel = cancel
	d.coverSeq++
	seq := d.coverSeq
	d.mu.Unlock()

	go func() {
		var img image.Image
		if artURL != "" && (bg || colors) {
			var err error
			if img, err = cover.Load(ctx, artURL); err != nil {
				log.Printf("artwork: %v", err)
			}
		}
		if ctx.Err() != nil {
			return
		}
		if img != nil && colors {
			d.send(map[string]any{"t": "mediaTheme", "palette": cover.Palette(img, dark, errs)})
		} else {
			d.send(map[string]any{"t": "mediaTheme", "palette": nil})
		}
		if img == nil || !bg {
			d.send(map[string]any{"t": "cover", "seq": seq, "n": 0})
			return
		}
		chunks := cover.Chunks(cover.Blurred(img, blur), 32*1024)
		for i, c := range chunks {
			if ctx.Err() != nil {
				return
			}
			d.send(map[string]any{"t": "cover", "seq": seq, "n": len(chunks), "i": i,
				"w": cover.Size, "h": cover.Size, "data": base64.StdEncoding.EncodeToString(c)})
		}
	}()
}

// ---- audio --------------------------------------------------------------

func (d *Daemon) audioLoop() {
	for {
		a, err := audio.Connect()
		if err != nil {
			log.Printf("audio: %v", err)
			time.Sleep(10 * time.Second)
			continue
		}
		d.mu.Lock()
		d.audio = a
		d.mu.Unlock()
		for alive := true; alive; {
			select {
			case <-a.Changed:
				d.sendAudio()
			case <-a.Streams:
				if d.viewKind() == "mixer" {
					d.sendMixer()
				}
			case <-a.Closed():
				alive = false
			}
		}
		d.mu.Lock()
		d.audio = nil
		d.mu.Unlock()
		time.Sleep(2 * time.Second)
	}
}

// sendAudio pushes volume and mic state if a page that shows them is visible
// and the value differs from what the device already has.
func (d *Daemon) sendAudio() {
	d.mu.Lock()
	a, v := d.audio, d.view
	d.mu.Unlock()
	if a == nil || !v.Awake || (v.Kind != "media" && v.Kind != "dial") {
		return
	}
	s, err := a.State()
	if err != nil {
		return
	}
	vol := fmt.Sprintf("%d/%v", s.Volume, s.Muted)
	mic := fmt.Sprintf("%d/%v", s.Mic, s.MicMuted)
	d.mu.Lock()
	sv, sm := vol != d.lastVol, mic != d.lastMic
	d.lastVol, d.lastMic = vol, mic
	d.mu.Unlock()
	if sv {
		d.send(map[string]any{"t": "volume", "value": min(s.Volume, 100), "muted": s.Muted})
	}
	if sm {
		d.send(map[string]any{"t": "mic", "value": min(s.Mic, 100), "muted": s.MicMuted})
	}
}

// ---- actions ------------------------------------------------------------

func (d *Daemon) run(a config.Action) {
	switch a.Type {
	case "command":
		runCommand(a.Cmd)
	case "open":
		openTarget(a.Target)
	case "mic":
		d.mu.Lock()
		au := d.audio
		d.mu.Unlock()
		if au != nil {
			if err := au.Mic(a.Cmd); err != nil {
				log.Printf("mic: %v", err)
			}
		}
	case "obs":
		if err := d.obs.Do(a.Request, a.Scene, a.Input); err != nil {
			log.Printf("obs: %v", err)
		}
	case "macro":
		for _, s := range a.Steps {
			switch {
			case s.Keys != "":
				d.sendWait(map[string]any{"t": "hid", "keys": s.Keys}, 2*time.Second)
			case s.Text != "":
				d.sendWait(map[string]any{"t": "hid", "text": s.Text}, 5*time.Second)
			case s.Delay > 0:
				time.Sleep(time.Duration(s.Delay) * time.Millisecond)
			case s.Command != "":
				runCommand(s.Command)
			}
		}
	// Device-side actions, only reached through "test" in the configurator.
	case "keys":
		d.send(map[string]any{"t": "hid", "keys": a.Keys})
	case "text":
		d.send(map[string]any{"t": "hid", "text": a.Text})
	case "media":
		d.send(map[string]any{"t": "hid", "media": a.Cmd})
	case "page":
		d.mu.Lock()
		prof := d.view.Profile
		d.mu.Unlock()
		d.send(map[string]any{"t": "show", "profile": prof, "page": a.Page})
	}
}

// ---- status for the socket ----------------------------------------------

type Status struct {
	OS         string           `json:"os"` // runtime.GOOS: "linux" | "windows"
	Device     *DeviceStatus    `json:"device"`
	Keyboard   *keymap.Detected `json:"keyboard,omitempty"`
	Focus      desktop.Focus    `json:"focus"`
	Profile    string           `json:"profile"`
	RecentApps []desktop.Focus  `json:"recentApps"`
	View       View             `json:"view"`
	Extension  bool             `json:"extension"` // the focus source answers
	Desktop    desktop.Info     `json:"desktop"`
	Discord    discord.State    `json:"discord"`
	OBS        obs.State        `json:"obs"`
}

type DeviceStatus struct {
	Connected bool   `json:"connected"`
	Port      string `json:"port,omitempty"`
	FW        string `json:"fw,omitempty"`
	Cfg       string `json:"cfg,omitempty"`
	InSync    bool   `json:"inSync"`
	SyncNote  string `json:"syncNote,omitempty"` // e.g. "Loaded the setup stored on your Roundbar"
	// Error is "permission" when the port exists but can't be opened (no udev rule).
	Error string `json:"error,omitempty"`
}

func (d *Daemon) status() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	ds := &DeviceStatus{Connected: d.port != nil, Error: d.openErr}
	if d.port != nil {
		ds.Port, ds.FW, ds.Cfg, ds.InSync = d.port.Path, d.hello.FW, d.hello.Cfg, d.hello.Cfg == d.sha
		ds.SyncNote = d.syncNote
	}
	pid, _ := d.profileFor(d.focus)
	rec := append([]desktop.Focus{}, d.recent...)
	disc := d.discordStateLocked()
	var kbd *keymap.Detected
	if d.layout.ID != "" {
		k := d.layout
		kbd = &k
	}
	var info desktop.Info
	if d.desk != nil {
		info = d.desk.Info()
	}
	return Status{OS: runtime.GOOS, Keyboard: kbd, Desktop: info, OBS: d.obs.State(), Discord: disc, Device: ds, Focus: d.focus, Profile: pid, RecentApps: rec, View: d.view, Extension: d.focus.ID != ""}
}

// Status is the daemon's state as `roundbar status` shows it (for the tray app).
func (d *Daemon) Status() Status { return d.status() }

// Watch returns a channel that wakes (coalesced) whenever the status changes,
// and a function that stops it.
func (d *Daemon) Watch() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	d.mu.Lock()
	d.watchers[ch] = true
	d.mu.Unlock()
	return ch, func() { d.mu.Lock(); delete(d.watchers, ch); d.mu.Unlock() }
}

// notify wakes every socket watcher (coalesced per watcher).
func (d *Daemon) notify() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for ch := range d.watchers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (d *Daemon) reload() error {
	c, raw, err := config.Load()
	if err != nil {
		return err
	}
	d.mu.Lock()
	same := config.Sha(raw) == d.sha
	d.mu.Unlock()
	if same {
		return nil
	}
	d.setConfig(c, raw)
	log.Printf("config reloaded")
	d.kickClock() // zones or weather city may have changed
	d.mu.Lock()
	d.artSent = "" // the cover settings may have changed
	onMedia := d.view.Awake && d.view.Kind == "media"
	d.mu.Unlock()
	if onMedia {
		go d.sendMedia()
	}
	d.pushConfig()
	d.syncProfile(true)
	d.notify()
	return nil
}
