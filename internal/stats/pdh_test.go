package stats

import "testing"

func TestPDHParsing(t *testing.T) {
	luid := luidString(0, 0xC3B5)
	if luid != "0x00000000_0x0000C3B5" {
		t.Fatalf("luid %s", luid)
	}
	l, e, ok := engineInstance("pid_1234_luid_0x00000000_0x0000C3B5_phys_0_eng_3_engtype_VideoDecode")
	if !ok || l != "0x00000000_0x0000c3b5" || e != "phys_0_eng_3" {
		t.Fatalf("engine %q %q %v", l, e, ok)
	}
	if _, _, ok := engineInstance("_Total"); ok {
		t.Fatal("_Total is not an engine")
	}
	items := []pdhItem{
		{"pid_1_luid_0x00000000_0x0000C3B5_phys_0_eng_0_engtype_3D", 30},
		{"pid_2_luid_0x00000000_0x0000C3B5_phys_0_eng_0_engtype_3D", 25.4}, // same engine: adds up
		{"pid_2_luid_0x00000000_0x0000C3B5_phys_0_eng_3_engtype_VideoDecode", 40},
		{"pid_3_luid_0x00000000_0x0000AAAA_phys_0_eng_0_engtype_3D", 99}, // another adapter
	}
	if v, ok := gpuLoad(items, luid); !ok || v != 55 {
		t.Fatalf("load %d %v", v, ok)
	}
	if _, ok := gpuLoad(items, luidString(0, 1)); ok {
		t.Fatal("unknown adapter")
	}
	if v, _ := gpuLoad([]pdhItem{{items[0].Name, 80}, {items[1].Name, 70}}, luid); v != 100 {
		t.Fatalf("capped load %d", v)
	}
	mem := []pdhItem{{"luid_0x00000000_0x0000C3B5_phys_0", 2 << 30}, {"luid_0x00000000_0x0000AAAA_phys_0", 1 << 30}}
	if v, ok := adapterMemory(mem, luid); !ok || v != 2<<30 {
		t.Fatalf("memory %v %v", v, ok)
	}
}
