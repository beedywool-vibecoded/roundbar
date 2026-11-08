package icons

import "testing"

func TestCommonIcons(t *testing.T) {
	found := 0
	for _, n := range []string{"firefox", "org.mozilla.firefox", "org.gnome.Nautilus", "spotify", "code", "discord", "audio-speakers"} {
		if b := Bitmap(n); b != nil {
			found++
			t.Logf("%s: %d bytes (%s)", n, len(b), find(n))
		}
	}
	if found == 0 {
		t.Skip("no icon theme found")
	}
}
