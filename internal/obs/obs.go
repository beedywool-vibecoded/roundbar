// Package obs controls OBS Studio through its built-in WebSocket server
// (obs-websocket v5): record/stream toggles, scenes, mutes, and live state for
// the device's OBS page. Works whether or not OBS is focused.
//
// Connection details live in ~/.config/roundbar/obs.json (0600); the password
// never goes into the config that's sent to the device.
package obs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
	"golang.org/x/net/websocket"
)

type Creds struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
}

func credsPath() string { return filepath.Join(config.Dir(), "obs.json") }

func LoadCreds() (*Creds, error) {
	raw, err := os.ReadFile(credsPath())
	if err != nil {
		return nil, err
	}
	var c Creds
	return &c, json.Unmarshal(raw, &c)
}

func SaveCreds(c *Creds) error {
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == 0 {
		c.Port = 4455
	}
	raw, _ := json.MarshalIndent(c, "", "  ")
	if err := os.MkdirAll(config.Dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(credsPath(), raw, 0o600)
}

type State struct {
	Connected bool     `json:"connected"`
	Error     string   `json:"error,omitempty"`
	Recording bool     `json:"recording"`
	Paused    bool     `json:"paused"`
	RecSince  int64    `json:"-"` // unix ms when recording started (for the timer)
	RecTime   int      `json:"recTime"`
	Streaming bool     `json:"streaming"`
	Scenes    []string `json:"scenes"`
	Current   string   `json:"current"`
}

type msg struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
}

type Client struct {
	mu      sync.Mutex
	ws      *websocket.Conn
	state   State
	pending map[string]chan json.RawMessage
	Changed chan struct{} // coalesced
	kick    chan struct{}
	up      chan struct{} // closed-and-replaced each time a session is identified
}

func New() *Client {
	return &Client{pending: map[string]chan json.RawMessage{}, Changed: make(chan struct{}, 1),
		kick: make(chan struct{}, 1), up: make(chan struct{})}
}

// Kick asks for a connection now (OBS page shown, OBS button pressed,
// settings saved). Without kicks the client stays idle: no timers, no sockets.
func (c *Client) Kick() {
	select {
	case c.kick <- struct{}{}:
	default:
	}
}

func (c *Client) connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws != nil
}

func (c *Client) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state
	s.Scenes = append([]string(nil), c.state.Scenes...)
	if s.Recording && s.RecSince > 0 {
		s.RecTime = int((time.Now().UnixMilli() - s.RecSince) / 1000)
	}
	return s
}

func (c *Client) changed() {
	select {
	case c.Changed <- struct{}{}:
	default:
	}
}

// Auth answers obs-websocket's challenge: base64(sha256(base64(sha256(pass+salt)) + challenge)).
func Auth(password, salt, challenge string) string {
	h := sha256.Sum256([]byte(password + salt))
	secret := base64.StdEncoding.EncodeToString(h[:])
	h2 := sha256.Sum256([]byte(secret + challenge))
	return base64.StdEncoding.EncodeToString(h2[:])
}

// Run connects on demand: it sleeps until Kick, then holds one session while
// OBS stays open (events only, no polling) and goes back to sleep when OBS closes.
func (c *Client) Run(stop <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case <-c.kick:
		}
		cr, err := LoadCreds()
		if err != nil {
			continue // not set up
		}
		if err := c.session(cr, stop); err != nil {
			c.mu.Lock()
			c.state = State{Error: err.Error()}
			c.mu.Unlock()
			c.changed()
		}
	}
}

func (c *Client) session(cr *Creds, stop <-chan struct{}) error {
	url := fmt.Sprintf("ws://%s:%d", cr.Host, cr.Port)
	cfg, err := websocket.NewConfig(url, "http://127.0.0.1")
	if err != nil {
		return err
	}
	cfg.Protocol = []string{"obswebsocket.json"}
	ws, err := websocket.DialConfig(cfg)
	if err != nil {
		return errors.New("OBS isn't running (or its WebSocket server is off)")
	}
	defer ws.Close()

	var hello msg
	if err := websocket.JSON.Receive(ws, &hello); err != nil || hello.Op != 0 {
		return errors.New("unexpected OBS handshake")
	}
	var h struct {
		Auth *struct{ Challenge, Salt string } `json:"authentication"`
	}
	_ = json.Unmarshal(hello.D, &h)
	ident := map[string]any{"rpcVersion": 1, "eventSubscriptions": 1 | 4 | 64} // General, Scenes, Outputs
	if h.Auth != nil {
		ident["authentication"] = Auth(cr.Password, h.Auth.Salt, h.Auth.Challenge)
	}
	if err := websocket.JSON.Send(ws, map[string]any{"op": 1, "d": ident}); err != nil {
		return err
	}
	var ok msg
	if err := websocket.JSON.Receive(ws, &ok); err != nil || ok.Op != 2 {
		return errors.New("OBS rejected the password")
	}
	c.mu.Lock()
	c.ws = ws
	c.state = State{Connected: true}
	close(c.up)
	c.up = make(chan struct{})
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.ws = nil; c.mu.Unlock() }()

	errc := make(chan error, 1)
	go func() { errc <- c.readLoop(ws) }()
	go c.refresh()
	select {
	case <-stop:
		return nil
	case err := <-errc:
		return err
	}
}

func (c *Client) readLoop(ws *websocket.Conn) error {
	for {
		var m msg
		if err := websocket.JSON.Receive(ws, &m); err != nil {
			return errors.New("OBS closed the connection")
		}
		switch m.Op {
		case 7: // RequestResponse
			var r struct {
				RequestID string `json:"requestId"`
			}
			_ = json.Unmarshal(m.D, &r)
			c.mu.Lock()
			ch := c.pending[r.RequestID]
			c.mu.Unlock()
			if ch != nil {
				ch <- m.D
			}
		case 5: // Event
			var e struct {
				Type string          `json:"eventType"`
				Data json.RawMessage `json:"eventData"`
			}
			_ = json.Unmarshal(m.D, &e)
			c.onEvent(e.Type, e.Data)
		}
	}
}

func reqID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Request sends one obs-websocket request and returns its responseData.
func (c *Client) Request(kind string, data map[string]any) (json.RawMessage, error) {
	id := reqID()
	ch := make(chan json.RawMessage, 1)
	c.mu.Lock()
	ws := c.ws
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
	if ws == nil {
		return nil, errors.New("OBS isn't connected")
	}
	d := map[string]any{"requestType": kind, "requestId": id}
	if data != nil {
		d["requestData"] = data
	}
	if err := websocket.JSON.Send(ws, map[string]any{"op": 6, "d": d}); err != nil {
		return nil, err
	}
	select {
	case raw := <-ch:
		var r struct {
			Status struct {
				Result  bool   `json:"result"`
				Comment string `json:"comment"`
			} `json:"requestStatus"`
			Data json.RawMessage `json:"responseData"`
		}
		_ = json.Unmarshal(raw, &r)
		if !r.Status.Result {
			return nil, fmt.Errorf("OBS %s: %s", kind, r.Status.Comment)
		}
		return r.Data, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("OBS didn't answer")
	}
}

func (c *Client) refresh() {
	if raw, err := c.Request("GetSceneList", nil); err == nil {
		var r struct {
			Current string `json:"currentProgramSceneName"`
			Scenes  []struct {
				Name  string `json:"sceneName"`
				Index int    `json:"sceneIndex"`
			} `json:"scenes"`
		}
		_ = json.Unmarshal(raw, &r)
		// OBS lists scenes bottom-up; show them in the order of its Scenes dock.
		names := make([]string, len(r.Scenes))
		for i, s := range r.Scenes {
			names[len(r.Scenes)-1-i] = s.Name
		}
		c.mu.Lock()
		c.state.Scenes, c.state.Current = names, r.Current
		c.mu.Unlock()
	}
	if raw, err := c.Request("GetRecordStatus", nil); err == nil {
		var r struct {
			Active   bool  `json:"outputActive"`
			Paused   bool  `json:"outputPaused"`
			Duration int64 `json:"outputDuration"` // ms
		}
		_ = json.Unmarshal(raw, &r)
		c.mu.Lock()
		c.state.Recording, c.state.Paused = r.Active, r.Paused
		if r.Active {
			c.state.RecSince = time.Now().UnixMilli() - r.Duration
		}
		c.mu.Unlock()
	}
	if raw, err := c.Request("GetStreamStatus", nil); err == nil {
		var r struct {
			Active bool `json:"outputActive"`
		}
		_ = json.Unmarshal(raw, &r)
		c.mu.Lock()
		c.state.Streaming = r.Active
		c.mu.Unlock()
	}
	c.changed()
}

func (c *Client) onEvent(kind string, data json.RawMessage) {
	switch kind {
	case "CurrentProgramSceneChanged":
		var e struct {
			Name string `json:"sceneName"`
		}
		_ = json.Unmarshal(data, &e)
		c.mu.Lock()
		c.state.Current = e.Name
		c.mu.Unlock()
		c.changed()
	case "SceneListChanged", "SceneNameChanged", "SceneCreated", "SceneRemoved":
		go c.refresh()
	case "RecordStateChanged":
		var e struct {
			Active bool   `json:"outputActive"`
			State  string `json:"outputState"`
		}
		_ = json.Unmarshal(data, &e)
		c.mu.Lock()
		was := c.state.Recording
		c.state.Recording = e.Active
		c.state.Paused = e.State == "OBS_WEBSOCKET_OUTPUT_PAUSED"
		if e.Active && !was {
			c.state.RecSince = time.Now().UnixMilli()
		}
		c.mu.Unlock()
		c.changed()
	case "StreamStateChanged":
		var e struct {
			Active bool `json:"outputActive"`
		}
		_ = json.Unmarshal(data, &e)
		c.mu.Lock()
		c.state.Streaming = e.Active
		c.mu.Unlock()
		c.changed()
	}
}

// Do runs a Roundbar `obs` action (docs/spec.md §1), connecting first if needed.
func (c *Client) Do(request, scene, input string) error {
	if !c.connected() {
		c.mu.Lock()
		up := c.up
		c.mu.Unlock()
		c.Kick()
		select {
		case <-up:
		case <-time.After(3 * time.Second):
		}
	}
	switch request {
	case "ToggleRecord", "ToggleRecordPause", "ToggleStream", "SaveReplayBuffer", "ToggleVirtualCam":
		_, err := c.Request(request, nil)
		return err
	case "StudioTransition":
		_, err := c.Request("TriggerStudioModeTransition", nil)
		return err
	case "SetScene":
		_, err := c.Request("SetCurrentProgramScene", map[string]any{"sceneName": scene})
		return err
	case "ToggleMute":
		if input == "" {
			input = "Mic/Aux"
		}
		_, err := c.Request("ToggleInputMute", map[string]any{"inputName": input})
		return err
	}
	return errors.New("unknown OBS request " + request)
}
