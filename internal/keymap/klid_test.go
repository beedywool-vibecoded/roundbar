package keymap

import "testing"

func TestKLID(t *testing.T) {
	ids := func(id uint16) string {
		if id == 1 {
			return "00020409" // Layout Id 0001 in the registry
		}
		return ""
	}
	for hkl, want := range map[uint32]string{
		0x04090409: "us",
		0x040C040C: "fr",
		0x08090409: "gb",      // English (US) language with the UK layout
		0xF0010409: "us-intl", // a variant, through its layout id
		0x08070807: "ch",
		0x100C100C: "ch-fr",
		0x0C0A0C0A: "es",
		0x04110411: "", // Japanese: none of ours
	} {
		if got := FromKLID(KLIDFromHKL(hkl, ids)); got != want {
			t.Errorf("%08x: got %q, want %q", hkl, got, want)
		}
	}
	if KLIDFromHKL(0xF0020409, nil) != "" {
		t.Error("variant without a lookup")
	}
	// Every id we map to must exist in keymaps.json.
	for k, id := range klids {
		if Get(id) == nil {
			t.Errorf("%s maps to unknown layout %q", k, id)
		}
	}
}
