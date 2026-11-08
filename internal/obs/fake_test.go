package obs

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

// fakeOBS speaks just enough obs-websocket v5 for the client: hello with an
// auth challenge, identify, a scene list, and records the requests it gets.
func fakeOBS(t *testing.T, password string, got chan<- string) *httptest.Server {
	return httptest.NewServer(websocket.Handler(func(ws *websocket.Conn) {
		_ = websocket.JSON.Send(ws, map[string]any{"op": 0, "d": map[string]any{
			"rpcVersion": 1, "authentication": map[string]string{"challenge": "ch", "salt": "salt"}}})
		var id struct {
			D struct {
				Auth string `json:"authentication"`
			} `json:"d"`
		}
		if websocket.JSON.Receive(ws, &id) != nil || id.D.Auth != Auth(password, "salt", "ch") {
			return // wrong password: OBS closes the socket
		}
		_ = websocket.JSON.Send(ws, map[string]any{"op": 2, "d": map[string]any{}})
		for {
			var r struct {
				D struct {
					Type string          `json:"requestType"`
					ID   string          `json:"requestId"`
					Data json.RawMessage `json:"requestData"`
				} `json:"d"`
			}
			if websocket.JSON.Receive(ws, &r) != nil {
				return
			}
			data := map[string]any{}
			switch r.D.Type {
			case "GetSceneList":
				data = map[string]any{"currentProgramSceneName": "Game",
					"scenes": []map[string]any{{"sceneName": "BRB"}, {"sceneName": "Game"}}}
			case "GetRecordStatus":
				data = map[string]any{"outputActive": true, "outputDuration": 61000}
			case "GetStreamStatus":
			default:
				got <- r.D.Type + string(r.D.Data)
			}
			raw, _ := json.Marshal(data)
			_ = websocket.JSON.Send(ws, map[string]any{"op": 7, "d": map[string]any{
				"requestId": r.D.ID, "requestStatus": map[string]any{"result": true}, "responseData": json.RawMessage(raw)}})
		}
	}))
}

func TestClientAgainstFake(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got := make(chan string, 4)
	srv := fakeOBS(t, "hunter2", got)
	defer srv.Close()
	port := srv.Listener.Addr().String()
	host, p, _ := strings.Cut(port, ":")
	var portN int
	_, _ = fmt.Sscan(p, &portN)
	if err := SaveCreds(&Creds{Host: host, Port: portN, Password: "hunter2"}); err != nil {
		t.Fatal(err)
	}
	c := New()
	stop := make(chan struct{})
	defer close(stop)
	go c.Run(stop, nil)

	deadline := time.After(5 * time.Second)
	for {
		s := c.State()
		if s.Connected && s.Current == "Game" && s.Recording {
			if len(s.Scenes) != 2 || s.Scenes[0] != "Game" { // dock order: reversed
				t.Fatalf("scenes %v", s.Scenes)
			}
			if s.RecTime < 61 {
				t.Fatalf("recTime %d", s.RecTime)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("never got the state: %+v", s)
		case <-c.Changed:
		case <-time.After(50 * time.Millisecond):
		}
	}
	if err := c.Do("SetScene", "BRB", ""); err != nil {
		t.Fatal(err)
	}
	if r := <-got; r != `SetCurrentProgramScene{"sceneName":"BRB"}` {
		t.Fatalf("got request %s", r)
	}
	if err := c.Do("ToggleMute", "", ""); err != nil {
		t.Fatal(err)
	}
	if r := <-got; r != `ToggleInputMute{"inputName":"Mic/Aux"}` {
		t.Fatalf("got request %s", r)
	}
	if c.Do("Explode", "", "") == nil {
		t.Fatal("unknown request should fail")
	}
}
