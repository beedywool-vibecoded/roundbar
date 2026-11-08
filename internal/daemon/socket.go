package daemon

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
	"github.com/beedywool-vibecoded/roundbar/internal/discord"
	"github.com/beedywool-vibecoded/roundbar/internal/ipc"
)

func (d *Daemon) serveSocket() {
	path := ipc.SocketPath()
	_ = os.Remove(path)
	l, err := net.Listen("unix", path)
	if err != nil {
		log.Printf("socket: %v", err)
		return
	}
	_ = os.Chmod(path, 0o600)
	for {
		c, err := l.Accept()
		if err != nil {
			log.Printf("socket accept: %v", err)
			return
		}
		go d.handle(c)
	}
}

func (d *Daemon) handle(c net.Conn) {
	defer c.Close()
	enc := json.NewEncoder(c)
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 4096), 1<<20)
	for sc.Scan() {
		var req ipc.Request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			_ = enc.Encode(map[string]string{"error": "bad request"})
			return
		}
		ok := map[string]bool{"ok": true}
		switch req.Op {
		case "status":
			_ = enc.Encode(d.status())
		case "watch":
			ch := make(chan struct{}, 1)
			d.mu.Lock()
			d.watchers[ch] = true
			d.mu.Unlock()
			defer func() { d.mu.Lock(); delete(d.watchers, ch); d.mu.Unlock() }()
			ch <- struct{}{}
			for range ch {
				if enc.Encode(d.status()) != nil {
					return
				}
			}
		case "show":
			d.send(map[string]any{"t": "show", "profile": req.Profile, "page": req.Page})
			_ = enc.Encode(ok)
		case "run":
			var a config.Action
			if err := json.Unmarshal(req.Action, &a); err != nil {
				_ = enc.Encode(map[string]string{"error": "bad action"})
				continue
			}
			go d.run(a)
			_ = enc.Encode(ok)
		case "reload":
			if err := d.reload(); err != nil {
				_ = enc.Encode(map[string]string{"error": err.Error()})
				continue
			}
			_ = enc.Encode(ok)
		case "screenshot":
			png, err := d.screenshot()
			if err != nil {
				_ = enc.Encode(map[string]string{"error": err.Error()})
				continue
			}
			_ = enc.Encode(map[string]any{"png": png}) // base64 in JSON
		case "discord": // save Discord app credentials from the configurator
			var cr struct {
				ClientID     string `json:"clientId"`
				ClientSecret string `json:"clientSecret"`
			}
			if err := json.Unmarshal(req.Action, &cr); err != nil || cr.ClientID == "" || cr.ClientSecret == "" {
				_ = enc.Encode(map[string]string{"error": "client id and secret are required"})
				continue
			}
			old, _ := discord.LoadCreds()
			c := &discord.Creds{ClientID: cr.ClientID, ClientSecret: cr.ClientSecret}
			if old != nil && old.ClientID == cr.ClientID && old.ClientSecret == cr.ClientSecret {
				c = old // keep tokens
			}
			if err := discord.SaveCreds(c); err != nil {
				_ = enc.Encode(map[string]string{"error": err.Error()})
				continue
			}
			select {
			case d.discordKick <- struct{}{}:
			default:
			}
			_ = enc.Encode(ok)
		case "bootloader":
			d.send(map[string]any{"t": "bootloader"})
			_ = enc.Encode(ok)
		default:
			_ = enc.Encode(map[string]string{"error": "unknown op"})
		}
	}
}
