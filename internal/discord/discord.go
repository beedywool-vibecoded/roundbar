// Package discord speaks Discord's local RPC protocol (the IPC socket the
// desktop client opens) to show and control the current voice channel.
//
// The user creates a Discord application once and gives Roundbar its client id
// and secret. The first connection asks for approval inside Discord; the
// OAuth token is then kept in ~/.config/roundbar/discord.json (0600) and
// refreshed automatically. Nothing of this goes to the device.
package discord

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/beedywool-vibecoded/roundbar/internal/config"
)

type Creds struct {
	ClientID     string    `json:"clientId"`
	ClientSecret string    `json:"clientSecret"`
	AccessToken  string    `json:"accessToken,omitempty"`
	RefreshToken string    `json:"refreshToken,omitempty"`
	Expires      time.Time `json:"expires,omitempty"`
}

func credsPath() string { return filepath.Join(config.Dir(), "discord.json") }

func LoadCreds() (*Creds, error) {
	raw, err := os.ReadFile(credsPath())
	if err != nil {
		return nil, err
	}
	var c Creds
	return &c, json.Unmarshal(raw, &c)
}

func SaveCreds(c *Creds) error {
	raw, _ := json.MarshalIndent(c, "", "  ")
	if err := os.MkdirAll(config.Dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(credsPath(), raw, 0o600)
}

// ---- voice state ---------------------------------------------------------

type Member struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Avatar   string `json:"avatar"` // CDN URL, "" for the default avatar
	Muted    bool   `json:"muted"`
	Speaking bool   `json:"speaking"`
}

type State struct {
	Connected bool     `json:"connected"` // RPC session authenticated
	Controls  bool     `json:"controls"`  // mute/deafen can be toggled from the device (RPC only)
	User      string   `json:"user,omitempty"`
	Channel   string   `json:"channel,omitempty"` // "" = not in voice
	Guild     string   `json:"guild,omitempty"`   // server name ("" for DMs / group calls)
	Members   []Member `json:"members,omitempty"`
	Mute      bool     `json:"mute"`
	Deaf      bool     `json:"deaf"`
	Error     string   `json:"error,omitempty"`
}

type Client struct {
	mu        sync.Mutex
	conn      net.Conn
	pending   map[string]chan json.RawMessage
	state     State
	channelID string
	Changed   chan struct{} // coalesced
}

func New() *Client {
	return &Client{pending: map[string]chan json.RawMessage{}, Changed: make(chan struct{}, 1)}
}

func (c *Client) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.state
	s.Members = append([]Member(nil), c.state.Members...)
	return s
}

func (c *Client) changed() {
	select {
	case c.Changed <- struct{}{}:
	default:
	}
}

func (c *Client) setErr(err error) {
	c.mu.Lock()
	c.state = State{Error: err.Error()}
	c.mu.Unlock()
	c.changed()
}

// ---- transport -------------------------------------------------------------

func socketPaths() []string {
	run := os.Getenv("XDG_RUNTIME_DIR")
	var out []string
	for _, dir := range []string{run, filepath.Join(run, "app/com.discordapp.Discord"), filepath.Join(run, "snap.discord"),
		filepath.Join(run, ".flatpak/com.discordapp.Discord/xdg-run"), filepath.Join(run, "app/dev.vencord.Vesktop")} {
		for i := 0; i < 10; i++ {
			out = append(out, filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", i)))
		}
	}
	return out
}

func dial() (net.Conn, error) {
	for _, p := range socketPaths() {
		if c, err := net.DialTimeout("unix", p, time.Second); err == nil {
			return c, nil
		}
	}
	return nil, errors.New("Discord isn't running")
}

func writeFrame(w io.Writer, op uint32, v any) error {
	body, err := json.Marshal(v)
	if err != nil {
		return err
	}
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr, op)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(len(body)))
	_, err = w.Write(append(hdr, body...))
	return err
}

func readFrame(r io.Reader) (uint32, []byte, error) {
	hdr := make([]byte, 8)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return 0, nil, err
	}
	n := binary.LittleEndian.Uint32(hdr[4:])
	if n > 1<<20 {
		return 0, nil, errors.New("frame too large")
	}
	body := make([]byte, n)
	_, err := io.ReadFull(r, body)
	return binary.LittleEndian.Uint32(hdr), body, err
}

func nonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type frame struct {
	Cmd   string          `json:"cmd"`
	Evt   string          `json:"evt"`
	Nonce string          `json:"nonce"`
	Data  json.RawMessage `json:"data"`
}

// call sends a command and waits for its reply (by nonce).
func (c *Client) call(cmd string, args any, evt string, timeout time.Duration) (json.RawMessage, error) {
	n := nonce()
	ch := make(chan json.RawMessage, 1)
	c.mu.Lock()
	conn := c.conn
	c.pending[n] = ch
	c.mu.Unlock()
	defer func() { c.mu.Lock(); delete(c.pending, n); c.mu.Unlock() }()
	if conn == nil {
		return nil, errors.New("not connected")
	}
	msg := map[string]any{"cmd": cmd, "args": args, "nonce": n}
	if evt != "" {
		msg["evt"] = evt
	}
	if err := writeFrame(conn, 1, msg); err != nil {
		return nil, err
	}
	select {
	case d := <-ch:
		var e struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(d, &e) == nil && e.Code != 0 && e.Message != "" {
			return nil, errors.New(e.Message)
		}
		return d, nil
	case <-time.After(timeout):
		return nil, errors.New(cmd + ": no answer from Discord")
	}
}

// ---- OAuth -------------------------------------------------------------------

func token(form url.Values) (*Creds, error) {
	resp, err := http.PostForm("https://discord.com/api/oauth2/token", form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Error        string `json:"error_description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	if resp.StatusCode != http.StatusOK || r.AccessToken == "" {
		if r.Error == "" {
			r.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, errors.New("Discord token: " + r.Error)
	}
	return &Creds{AccessToken: r.AccessToken, RefreshToken: r.RefreshToken,
		Expires: time.Now().Add(time.Duration(r.ExpiresIn) * time.Second)}, nil
}

func (c *Client) ensureToken(cr *Creds) error {
	if cr.AccessToken != "" && time.Until(cr.Expires) > time.Hour {
		return nil
	}
	var t *Creds
	var err error
	if cr.RefreshToken != "" {
		t, err = token(url.Values{"client_id": {cr.ClientID}, "client_secret": {cr.ClientSecret},
			"grant_type": {"refresh_token"}, "refresh_token": {cr.RefreshToken}})
	}
	if t == nil { // first time (or refresh rejected): ask the user in Discord
		raw, err2 := c.call("AUTHORIZE", map[string]any{"client_id": cr.ClientID,
			"scopes": []string{"rpc", "rpc.voice.read", "rpc.voice.write", "identify"}}, "", 2*time.Minute)
		if err2 != nil {
			if err != nil {
				return fmt.Errorf("%v; %v", err, err2)
			}
			return err2
		}
		var a struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(raw, &a)
		t, err = token(url.Values{"client_id": {cr.ClientID}, "client_secret": {cr.ClientSecret},
			"grant_type": {"authorization_code"}, "code": {a.Code}, "redirect_uri": {"http://127.0.0.1"}})
		if err != nil {
			return err
		}
	}
	cr.AccessToken, cr.RefreshToken, cr.Expires = t.AccessToken, t.RefreshToken, t.Expires
	return SaveCreds(cr)
}

// ---- session -------------------------------------------------------------------

// Run keeps a session alive while credentials exist, reconnecting when Discord
// starts or restarts. It returns when stop is closed.
// Run sleeps until kicked (Discord page shown, credentials saved), then holds
// one RPC session while Discord stays open; nothing is retried in the background.
func (c *Client) Run(stop <-chan struct{}, kick <-chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case <-kick:
		}
		cr, err := LoadCreds()
		if err == nil && cr.ClientID != "" && cr.ClientSecret != "" {
			if err := c.session(cr); err != nil {
				c.setErr(err)
			}
		}
	}
}

func (c *Client) session(cr *Creds) error {
	conn, err := dial()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := writeFrame(conn, 0, map[string]any{"v": 1, "client_id": cr.ClientID}); err != nil {
		return err
	}
	if _, _, err := readFrame(conn); err != nil { // READY
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.conn = nil; c.state.Connected = false; c.mu.Unlock(); c.changed() }()

	readErr := make(chan error, 1)
	go func() { readErr <- c.readLoop(conn) }()

	if err := c.ensureToken(cr); err != nil {
		return err
	}
	raw, err := c.call("AUTHENTICATE", map[string]any{"access_token": cr.AccessToken}, "", 10*time.Second)
	if err != nil {
		cr.AccessToken = "" // stale: next round re-authorizes or refreshes
		_ = SaveCreds(cr)
		return err
	}
	var auth struct {
		User struct {
			Username   string `json:"username"`
			GlobalName string `json:"global_name"`
		} `json:"user"`
	}
	_ = json.Unmarshal(raw, &auth)
	name := auth.User.GlobalName
	if name == "" {
		name = auth.User.Username
	}
	c.mu.Lock()
	c.state = State{Connected: true, Controls: true, User: name}
	c.mu.Unlock()
	c.changed()

	for _, evt := range []string{"VOICE_SETTINGS_UPDATE", "VOICE_CHANNEL_SELECT"} {
		_, _ = c.call("SUBSCRIBE", map[string]any{}, evt, 5*time.Second)
	}
	c.refreshSettings()
	c.refreshChannel()
	return <-readErr
}

func (c *Client) readLoop(conn net.Conn) error {
	for {
		op, body, err := readFrame(conn)
		if err != nil {
			return err
		}
		switch op {
		case 3: // ping
			_ = writeFrame(conn, 4, json.RawMessage(body))
			continue
		case 2:
			return errors.New("Discord closed the connection")
		}
		var f frame
		if json.Unmarshal(body, &f) != nil {
			continue
		}
		if f.Cmd == "DISPATCH" {
			go c.onEvent(f.Evt, f.Data)
			continue
		}
		c.mu.Lock()
		ch := c.pending[f.Nonce]
		c.mu.Unlock()
		if ch != nil {
			ch <- f.Data
		}
	}
}

func (c *Client) refreshSettings() {
	raw, err := c.call("GET_VOICE_SETTINGS", map[string]any{}, "", 5*time.Second)
	if err != nil {
		return
	}
	c.applySettings(raw)
}

func (c *Client) applySettings(raw json.RawMessage) {
	var s struct {
		Mute *bool `json:"mute"`
		Deaf *bool `json:"deaf"`
	}
	if json.Unmarshal(raw, &s) != nil {
		return
	}
	c.mu.Lock()
	if s.Mute != nil {
		c.state.Mute = *s.Mute
	}
	if s.Deaf != nil {
		c.state.Deaf = *s.Deaf
	}
	c.mu.Unlock()
	c.changed()
}

type voiceState struct {
	Nick string `json:"nick"`
	User struct {
		ID         string `json:"id"`
		Username   string `json:"username"`
		GlobalName string `json:"global_name"`
		Avatar     string `json:"avatar"`
	} `json:"user"`
	VoiceState struct {
		Mute     bool `json:"mute"`
		Deaf     bool `json:"deaf"`
		SelfMute bool `json:"self_mute"`
		SelfDeaf bool `json:"self_deaf"`
	} `json:"voice_state"`
}

func member(v voiceState) Member {
	name := v.Nick
	if name == "" {
		name = v.User.GlobalName
	}
	if name == "" {
		name = v.User.Username
	}
	m := Member{ID: v.User.ID, Name: name,
		Muted: v.VoiceState.Mute || v.VoiceState.SelfMute || v.VoiceState.Deaf || v.VoiceState.SelfDeaf}
	if v.User.Avatar != "" {
		m.Avatar = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png?size=64", v.User.ID, v.User.Avatar)
	}
	return m
}

// refreshChannel reloads the selected voice channel and (re)subscribes to its events.
func (c *Client) refreshChannel() {
	raw, err := c.call("GET_SELECTED_VOICE_CHANNEL", map[string]any{}, "", 5*time.Second)
	if err != nil {
		return
	}
	var ch struct {
		ID          string       `json:"id"`
		Name        string       `json:"name"`
		GuildID     string       `json:"guild_id"`
		VoiceStates []voiceState `json:"voice_states"`
	}
	if string(raw) == "null" || json.Unmarshal(raw, &ch) != nil || ch.ID == "" {
		c.mu.Lock()
		old := c.channelID
		c.channelID, c.state.Channel, c.state.Guild, c.state.Members = "", "", "", nil
		c.mu.Unlock()
		if old != "" {
			c.subscribeChannel(old, "UNSUBSCRIBE")
		}
		c.changed()
		return
	}
	members := make([]Member, 0, len(ch.VoiceStates))
	for _, v := range ch.VoiceStates {
		members = append(members, member(v))
	}
	c.mu.Lock()
	old := c.channelID
	c.channelID = ch.ID
	if ch.Name == "" {
		ch.Name = "Voice"
	}
	c.state.Channel, c.state.Members = ch.Name, members
	c.mu.Unlock()
	guild := ""
	if ch.GuildID != "" {
		if raw, err := c.call("GET_GUILD", map[string]any{"guild_id": ch.GuildID}, "", 5*time.Second); err == nil {
			var g struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(raw, &g)
			guild = g.Name
		}
	}
	c.mu.Lock()
	c.state.Guild = guild
	c.mu.Unlock()
	if old != ch.ID {
		if old != "" {
			c.subscribeChannel(old, "UNSUBSCRIBE")
		}
		c.subscribeChannel(ch.ID, "SUBSCRIBE")
	}
	c.changed()
}

func (c *Client) subscribeChannel(id, cmd string) {
	for _, evt := range []string{"VOICE_STATE_CREATE", "VOICE_STATE_UPDATE", "VOICE_STATE_DELETE", "SPEAKING_START", "SPEAKING_STOP"} {
		_, _ = c.call(cmd, map[string]any{"channel_id": id}, evt, 5*time.Second)
	}
}

func (c *Client) onEvent(evt string, data json.RawMessage) {
	switch evt {
	case "VOICE_SETTINGS_UPDATE":
		c.applySettings(data)
	case "VOICE_CHANNEL_SELECT", "VOICE_STATE_CREATE", "VOICE_STATE_DELETE":
		c.refreshChannel()
	case "VOICE_STATE_UPDATE":
		var v voiceState
		if json.Unmarshal(data, &v) != nil {
			return
		}
		m := member(v)
		c.mu.Lock()
		for i := range c.state.Members {
			if c.state.Members[i].ID == m.ID {
				m.Speaking = c.state.Members[i].Speaking
				c.state.Members[i] = m
			}
		}
		c.mu.Unlock()
		c.changed()
	case "SPEAKING_START", "SPEAKING_STOP":
		var s struct {
			UserID string `json:"user_id"`
		}
		if json.Unmarshal(data, &s) != nil {
			return
		}
		c.mu.Lock()
		for i := range c.state.Members {
			if c.state.Members[i].ID == s.UserID {
				c.state.Members[i].Speaking = evt == "SPEAKING_START"
			}
		}
		c.mu.Unlock()
		c.changed()
	}
}

// Toggle flips "mute" or "deaf" for the local user.
func (c *Client) Toggle(what string) error {
	st := c.State()
	if !st.Connected {
		return errors.New("Discord isn't connected")
	}
	args := map[string]any{}
	switch what {
	case "mute":
		args["mute"] = !st.Mute
	case "deaf":
		args["deaf"] = !st.Deaf
	case "leave":
		_, err := c.call("SELECT_VOICE_CHANNEL", map[string]any{"channel_id": nil}, "", 5*time.Second)
		return err
	default:
		return errors.New("unknown toggle " + what)
	}
	raw, err := c.call("SET_VOICE_SETTINGS", args, "", 5*time.Second)
	if err == nil {
		c.applySettings(raw)
	}
	return err
}
