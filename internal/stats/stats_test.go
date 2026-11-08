package stats

import (
	"testing"
	"time"
)

func TestSample(t *testing.T) {
	r := New()
	all := map[string]bool{"cpu": true, "ram": true, "gpu": true, "vram": true, "cpuTemp": true, "gpuTemp": true}
	r.Sample(all)
	time.Sleep(300 * time.Millisecond)
	s := r.Sample(all)
	t.Logf("%v", s)
	if _, ok := s["ram"]; !ok {
		t.Fatal("no ram")
	}
}
