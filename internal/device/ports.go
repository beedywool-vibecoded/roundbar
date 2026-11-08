package device

import (
	"fmt"
	"strconv"
	"strings"
)

func hexID(b []byte) uint16 {
	n, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 16, 16)
	return uint16(n)
}

// usbKeyMatches: a Windows Enum\USB key name ("VID_303A&PID_4011&MI_00")
// belongs to the device with these ids.
func usbKeyMatches(key string, vid, pid uint16) bool {
	k := strings.ToUpper(key)
	want := fmt.Sprintf("VID_%04X&PID_%04X", vid, pid)
	return k == want || strings.HasPrefix(k, want+"&")
}
