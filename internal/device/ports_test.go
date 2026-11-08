package device

import "testing"

func TestUSBKeyMatches(t *testing.T) {
	for key, want := range map[string]bool{
		"VID_303A&PID_4011":       true,
		"VID_303A&PID_4011&MI_00": true,
		"vid_303a&pid_4011&mi_02": true,
		"VID_303A&PID_40110":      false,
		"VID_303A&PID_1001":       false,
		"VID_1234&PID_4011&MI_00": false,
	} {
		if got := usbKeyMatches(key, VID, PID); got != want {
			t.Errorf("%s: got %v", key, got)
		}
	}
	if hexID([]byte("303a\n")) != 0x303a {
		t.Error("hexID")
	}
}
