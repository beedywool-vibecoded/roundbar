package daemon

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"log"
	"time"
	"unicode/utf8"

	"github.com/beedywool-vibecoded/roundbar/internal/icons"
)

// Pages backed by GNOME Shell and PipeWire: windows, mixer, clipboard, and the
// notification panel. Lists are pushed only while the page that shows them is
// on screen (notifications: always, if enabled), so idle cost stays zero.

func (d *Daemon) viewKind() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.view.Awake {
		return ""
	}
	return d.view.Kind
}

// sendIcons ships icon bitmaps the device hasn't seen since it connected.
func (d *Daemon) sendIcons(names ...string) {
	for _, n := range names {
		if n == "" {
			continue
		}
		d.mu.Lock()
		seen := d.iconsSent[n]
		d.iconsSent[n] = true
		d.mu.Unlock()
		if seen {
			continue
		}
		if b := icons.Bitmap(n); b != nil {
			d.send(map[string]any{"t": "icon", "name": n, "w": icons.Size, "h": icons.Size,
				"data": base64.StdEncoding.EncodeToString(b)})
		}
	}
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func (d *Daemon) sendWindows() {
	if d.desk == nil {
		return
	}
	ws, err := d.desk.Windows()
	if err != nil {
		log.Printf("windows: %v", err)
		return
	}
	if len(ws) > 10 { // focused + center + up to 8 on the ring
		ws = ws[:10]
	}
	for i := range ws {
		ws[i].Title = clip(ws[i].Title, 48)
		d.sendIcons(ws[i].Icon)
	}
	d.send(map[string]any{"t": "windows", "list": ws})
}

func (d *Daemon) sendMixer() {
	d.mu.Lock()
	a := d.audio
	d.mu.Unlock()
	if a == nil {
		return
	}
	apps, _, err := a.Mixer()
	if err != nil {
		return
	}
	for i := range apps {
		d.sendIcons(apps[i].Icon)
	}
	d.send(map[string]any{"t": "mixer", "apps": apps})
}

func (d *Daemon) sendClips() {
	if d.desk == nil {
		return
	}
	list, err := d.desk.Clipboard()
	if err != nil {
		return
	}
	out := make([]string, 0, len(list))
	for _, t := range list {
		out = append(out, clip(t, 200))
	}
	d.send(map[string]any{"t": "clips", "list": out})
}

func (d *Daemon) notifsEnabled() (panel, toast bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	dv := d.cfg.Device
	return dv.Notifications == nil || *dv.Notifications, dv.NotifToast == nil || *dv.NotifToast
}

func (d *Daemon) sendNotifs(newID string) {
	panel, toast := d.notifsEnabled()
	if d.desk == nil || (!panel && !toast) {
		return
	}
	list, err := d.desk.Notifications()
	if err != nil {
		log.Printf("notifications: %v", err)
		return
	}
	for i := range list {
		list[i].Title = clip(list[i].Title, 80)
		list[i].Body = clip(list[i].Body, 160)
		d.sendIcons(list[i].Icon)
		if list[i].ID == newID && toast {
			d.send(map[string]any{"t": "notif", "id": list[i].ID, "app": list[i].App, "title": list[i].Title,
				"body": list[i].Body, "icon": list[i].Icon})
		}
	}
	if panel {
		d.send(map[string]any{"t": "notifs", "list": list})
	}
}

// onShellView pushes the list a newly visible page needs.
func (d *Daemon) onShellView(kind string) {
	switch kind {
	case "windows":
		go d.sendWindows()
	case "mixer":
		go d.sendMixer()
	case "clipboard":
		go d.sendClips()
	case "discord":
		go d.sendDiscord()
	}
}

func (d *Daemon) onShellMsg(m json.RawMessage, t string) bool {
	var v struct {
		ID  string `json:"id"`
		Vol int    `json:"vol"`
		I   int    `json:"i"`
	}
	_ = json.Unmarshal(m, &v)
	d.mu.Lock()
	a := d.audio
	d.mu.Unlock()
	switch t {
	case "activate":
		if d.desk != nil {
			_ = d.desk.Activate(v.ID)
		}
	case "mixerSet":
		if a != nil {
			_ = a.SetAppVolume(v.ID, v.Vol)
		}
	case "mixerMute":
		if a != nil {
			_ = a.ToggleAppMute(v.ID)
		}
	case "output":
		if a != nil {
			_ = a.SetOutput(v.ID)
		}
	case "paste":
		if d.desk == nil {
			return true
		}
		list, err := d.desk.Clipboard()
		if err != nil || v.I < 0 || v.I >= len(list) {
			return true
		}
		if err := d.desk.SetClipboard(list[v.I]); err != nil {
			log.Printf("clipboard: %v", err)
			return true
		}
		go func() {
			time.Sleep(60 * time.Millisecond) // let the shell own the new selection first
			d.send(map[string]any{"t": "hid", "keys": "ctrl+v"})
		}()
	case "notifOpen", "notifDismiss":
		if d.desk != nil {
			action := "dismiss"
			if t == "notifOpen" {
				action = "open"
			}
			_ = d.desk.NotificationAction(v.ID, action)
		}
	case "notifClear":
		if d.desk != nil {
			if list, err := d.desk.Notifications(); err == nil {
				for _, n := range list {
					_ = d.desk.NotificationAction(n.ID, "dismiss")
				}
			}
		}
	default:
		return false
	}
	return true
}

func (d *Daemon) sendDiscord() {
	st := d.discord.State()
	for _, m := range st.Members {
		d.sendIcons(m.Avatar)
	}
	d.send(map[string]any{"t": "discord", "state": st})
}

func (d *Daemon) discordLoop() {
	for range d.discord.Changed {
		if d.viewKind() == "discord" {
			d.sendDiscord()
		}
		d.notify() // the configurator shows the connection state
	}
}

// ---- screenshots ----------------------------------------------------------

type shot struct {
	W, H, Stride, N, I int
	Data               string
}

// screenshot asks the device for its screen and returns it as PNG bytes.
func (d *Daemon) screenshot() ([]byte, error) {
	ch := make(chan json.RawMessage, 64)
	d.mu.Lock()
	d.replies["shot"] = ch
	d.mu.Unlock()
	defer func() { d.mu.Lock(); delete(d.replies, "shot"); d.mu.Unlock() }()
	d.send(map[string]any{"t": "screenshot"})
	var buf []byte
	var w, h, stride int
	for got, n := 0, 1; got < n; got++ {
		select {
		case raw := <-ch:
			var s shot
			if err := json.Unmarshal(raw, &s); err != nil {
				return nil, err
			}
			b, err := base64.StdEncoding.DecodeString(s.Data)
			if err != nil {
				return nil, err
			}
			w, h, stride, n = s.W, s.H, s.Stride, s.N
			buf = append(buf, b...)
		case <-time.After(10 * time.Second):
			return nil, errors.New("the device did not send a screenshot")
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := y*stride + 2*x
			if o+1 >= len(buf) {
				break
			}
			v := uint16(buf[o]) | uint16(buf[o+1])<<8
			img.Pix[img.PixOffset(x, y)+0] = uint8(v>>11) << 3
			img.Pix[img.PixOffset(x, y)+1] = uint8(v>>5&63) << 2
			img.Pix[img.PixOffset(x, y)+2] = uint8(v&31) << 3
			img.Pix[img.PixOffset(x, y)+3] = 255
		}
	}
	var out bytes.Buffer
	err := png.Encode(&out, img)
	return out.Bytes(), err
}
