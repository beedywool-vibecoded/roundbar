package discord

// Live voice state from Vesktop (stock Vesktop + stock Vencord) through the
// Chrome DevTools protocol. Vesktop is started with a localhost-only debug
// port (~/.config/vesktop-flags.conf); we inject a few lines into Discord's
// page that subscribe to its own voice events through Vencord's public webpack
// API and call back into us with a compact state on every change.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

const CDPPort = 9337

// The injected script: idempotent (stops a previous copy), coalesces bursts,
// heartbeat every 30 s so the daemon knows it's still alive.
const cdpScript = `(() => {
  if (!window.Vencord || !Vencord.Webpack || !Vencord.Webpack.Common) return "wait";
  const W = Vencord.Webpack, C = W.Common;
  const VS = W.findStore("VoiceStateStore"), SP = W.findStore("SpeakingStore"), ME = W.findStore("MediaEngineStore");
  if (!VS || !C.FluxDispatcher || !C.UserStore) return "wait";
  if (window.__roundbarStop) window.__roundbarStop();
  const speaking = new Set();
  let t = null;
  const state = () => {
    const me = C.UserStore.getCurrentUser();
    const id = C.SelectedChannelStore.getVoiceChannelId();
    const ch = id ? C.ChannelStore.getChannel(id) : null;
    const members = ch ? Object.values(VS.getVoiceStatesForChannel(id) || {}).map(vs => {
      const u = C.UserStore.getUser(vs.userId);
      const nick = ch.guild_id && C.GuildMemberStore ? C.GuildMemberStore.getNick(ch.guild_id, vs.userId) : null;
      return {
        id: vs.userId,
        name: nick || (u && (u.globalName || u.username)) || "",
        avatar: (u && u.getAvatarURL && u.getAvatarURL(undefined, 64, false)) || "",
        muted: !!(vs.selfMute || vs.mute || vs.selfDeaf || vs.deaf),
        deaf: !!(vs.selfDeaf || vs.deaf),
        speaking: speaking.has(vs.userId) || !!(SP && SP.isSpeaking && SP.isSpeaking(vs.userId)),
      };
    }) : [];
    return {
      connected: true,
      user: me ? (me.globalName || me.username) : "",
      channel: ch ? (ch.name || "Voice") : "",
      guild: ch && ch.guild_id && C.GuildStore ? ((C.GuildStore.getGuild(ch.guild_id) || {}).name || "") : "",
      members,
      mute: !!(ME && ME.isSelfMute && ME.isSelfMute()),
      deaf: !!(ME && ME.isSelfDeaf && ME.isSelfDeaf()),
    };
  };
  const push = () => {
    if (t) return;
    t = setTimeout(() => { t = null; try { __roundbar(JSON.stringify(state())); } catch (e) {} }, 120);
  };
  const onSpeak = e => { if (e.speakingFlags) speaking.add(e.userId); else speaking.delete(e.userId); push(); };
  const evs = ["VOICE_STATE_UPDATES", "VOICE_CHANNEL_SELECT", "RTC_CONNECTION_STATE", "AUDIO_TOGGLE_SELF_MUTE", "AUDIO_TOGGLE_SELF_DEAF"];
  evs.forEach(e => C.FluxDispatcher.subscribe(e, push));
  C.FluxDispatcher.subscribe("SPEAKING", onSpeak);
  const hb = setInterval(push, 30000);
  window.__roundbarStop = () => {
    evs.forEach(e => C.FluxDispatcher.unsubscribe(e, push));
    C.FluxDispatcher.unsubscribe("SPEAKING", onSpeak);
    clearInterval(hb);
  };
  push();
  return "ok";
})()`

type cdpMsg struct {
	ID     int             `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

// CDP watches Vesktop and reports voice state through OnState.
type CDP struct {
	OnState func(State)

	mu     sync.Mutex
	nextID int
	live   bool // a session delivered state since the last failure
	kick   chan struct{}
	ws     *websocket.Conn // current session, for Toggle
}

// Discord's own voice actions, found through Vencord's public webpack API.
var cdpActions = map[string]string{
	"mute":  `Vencord.Webpack.findByProps("toggleSelfMute", "toggleSelfDeaf").toggleSelfMute()`,
	"deaf":  `Vencord.Webpack.findByProps("toggleSelfMute", "toggleSelfDeaf").toggleSelfDeaf()`,
	"leave": `Vencord.Webpack.findByProps("selectVoiceChannel").selectVoiceChannel(null)`,
}

// Toggle mutes, deafens or leaves the call in Vesktop.
func (c *CDP) Toggle(what string) error {
	js, ok := cdpActions[what]
	if !ok {
		return fmt.Errorf("unknown Discord control %q", what)
	}
	c.mu.Lock()
	ws := c.ws
	c.mu.Unlock()
	if ws == nil {
		return errors.New("Vesktop isn't connected")
	}
	return c.send(ws, "Runtime.evaluate", map[string]any{"expression": "(() => { " + js + "; })()"})
}

// Connected reports whether a Vesktop session is open.
func (c *CDP) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws != nil
}

// Kick asks for a session now (a Discord page is on screen).
func (c *CDP) Kick() {
	c.mu.Lock()
	if c.kick == nil {
		c.kick = make(chan struct{}, 1)
	}
	k := c.kick
	c.mu.Unlock()
	select {
	case k <- struct{}{}:
	default:
	}
}

func discordTarget() (string, error) {
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", CDPPort))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var targets []struct {
		Type, URL, WebSocketDebuggerURL string
	}
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return "", err
	}
	for _, t := range targets {
		if t.Type == "page" && strings.Contains(t.URL, "discord.com") && t.WebSocketDebuggerURL != "" {
			return t.WebSocketDebuggerURL, nil
		}
	}
	return "", errors.New("no Discord page in Vesktop yet")
}

// Run sleeps until Kick, then holds a session while Vesktop is open (Discord
// pushes changes; nothing is polled) and sleeps again once Vesktop closes.
func (c *CDP) Run(stop <-chan struct{}) {
	c.Kick()
	c.mu.Lock()
	k := c.kick
	c.mu.Unlock()
	<-k // drop the priming kick
	for {
		select {
		case <-stop:
			return
		case <-k:
		}
		if err := c.session(stop); err != nil && c.live && c.OnState != nil {
			c.live = false
			c.OnState(State{}) // Vesktop closed: report it once
		}
	}
}

func (c *CDP) send(ws *websocket.Conn, method string, params any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()
	p, _ := json.Marshal(params)
	return websocket.JSON.Send(ws, cdpMsg{ID: id, Method: method, Params: p})
}

func (c *CDP) session(stop <-chan struct{}) error {
	url, err := discordTarget()
	if err != nil {
		return err
	}
	ws, err := websocket.Dial(url, "", fmt.Sprintf("http://127.0.0.1:%d", CDPPort))
	if err != nil {
		return err
	}
	defer ws.Close()
	c.mu.Lock()
	c.ws = ws
	c.mu.Unlock()
	defer func() { c.mu.Lock(); c.ws = nil; c.mu.Unlock() }()

	inject := func() {
		_ = c.send(ws, "Runtime.evaluate", map[string]any{"expression": cdpScript, "returnByValue": true})
	}
	_ = c.send(ws, "Runtime.enable", map[string]any{})
	_ = c.send(ws, "Runtime.addBinding", map[string]any{"name": "__roundbar"}) // survives page reloads
	inject()

	msgs := make(chan cdpMsg, 16)
	errc := make(chan error, 1)
	go func() {
		for {
			var m cdpMsg
			if err := websocket.JSON.Receive(ws, &m); err != nil {
				errc <- err
				return
			}
			msgs <- m
		}
	}()
	retry := time.NewTicker(3 * time.Second) // until Vencord is ready inside the page
	defer retry.Stop()
	installed := false
	for {
		select {
		case <-stop:
			return nil
		case err := <-errc:
			return err
		case <-retry.C:
			if !installed {
				inject()
			}
		case m := <-msgs:
			switch m.Method {
			case "Runtime.bindingCalled":
				var p struct {
					Name, Payload string
				}
				if json.Unmarshal(m.Params, &p) == nil && p.Name == "__roundbar" {
					installed = true
					var st State
					if json.Unmarshal([]byte(p.Payload), &st) == nil && c.OnState != nil {
						c.live = true
						c.OnState(st)
					}
				}
			case "Runtime.executionContextCreated": // the page reloaded: install again
				installed = false
			}
			if m.ID != 0 && strings.Contains(string(m.Result), `"value":"ok"`) {
				installed = true
			}
		}
	}
}
