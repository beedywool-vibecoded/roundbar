package steam

import "testing"

func TestAppID(t *testing.T) {
	if AppID("steam_app_1091500") != "1091500" || AppID("firefox") != "" || AppID("STEAM_APP_42") != "42" {
		t.Fatal("AppID")
	}
}

// Uses the local Steam install when there is one.
func TestLocal(t *testing.T) {
	for _, id := range []string{"228980", "1070560"} {
		if n := Name(id); n != "" {
			t.Logf("%s: %q art=%q", id, n, Art(id))
		}
	}
}
