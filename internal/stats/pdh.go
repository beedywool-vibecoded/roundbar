package stats

import (
	"fmt"
	"strings"
)

// Windows GPU figures come from the performance counters Task Manager uses
// (WDDM 2.x, every vendor): "GPU Engine(*)\Utilization Percentage" has one
// instance per process and engine, "GPU Adapter Memory(*)\Dedicated Usage"
// one per adapter. This file parses their instance names; it is plain logic
// so it is tested everywhere.

// pdhItem is one instance of a wildcard counter.
type pdhItem struct {
	Name  string
	Value float64
}

// luidString formats an adapter LUID the way counter instance names do.
func luidString(high int32, low uint32) string {
	return fmt.Sprintf("0x%08X_0x%08X", uint32(high), low)
}

// engineInstance splits "pid_1234_luid_0x00000000_0x0000C3B5_phys_0_eng_3_engtype_3D"
// into the adapter LUID and the engine ("phys_0_eng_3").
func engineInstance(name string) (luid, engine string, ok bool) {
	_, rest, found := strings.Cut(strings.ToLower(name), "luid_")
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "_")
	// 0x…, 0x…, phys, N, eng, N, engtype, T
	if len(parts) < 6 || parts[2] != "phys" || parts[4] != "eng" {
		return "", "", false
	}
	return parts[0] + "_" + parts[1], strings.Join(parts[2:6], "_"), true
}

// gpuLoad is the adapter's load as Task Manager shows it: per engine, the
// sum over processes; the busiest engine wins.
func gpuLoad(items []pdhItem, luid string) (int, bool) {
	luid = strings.ToLower(luid)
	engines := map[string]float64{}
	for _, it := range items {
		l, e, ok := engineInstance(it.Name)
		if ok && l == luid {
			engines[e] += it.Value
		}
	}
	if len(engines) == 0 {
		return 0, false
	}
	best := 0.0
	for _, v := range engines {
		best = max(best, v)
	}
	return int(min(100, best) + 0.5), true
}

// adapterMemory sums "Dedicated Usage" (bytes) over the instances of one
// adapter ("luid_0x00000000_0x0000C3B5_phys_0").
func adapterMemory(items []pdhItem, luid string) (float64, bool) {
	luid = "luid_" + strings.ToLower(luid) + "_"
	sum, found := 0.0, false
	for _, it := range items {
		if strings.HasPrefix(strings.ToLower(it.Name), luid) {
			sum += it.Value
			found = true
		}
	}
	return sum, found
}
