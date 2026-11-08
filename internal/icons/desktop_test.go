package icons

import "testing"

func TestForApp(t *testing.T) {
	for _, c := range []string{"vesktop", "firefox", "spotify"} {
		t.Logf("%s -> %q", c, ForApp(c))
	}
}
