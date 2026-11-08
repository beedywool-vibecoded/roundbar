package keymap

import (
	"fmt"
	"strings"
)

// Windows names keyboard layouts by KLID, an 8-digit hex string: the low
// word is the layout's language, the high word picks a variant
// (00020409 = United States-International).
var klids = map[string]string{
	"00000409": "us",
	"00020409": "us-intl",
	"00000809": "gb",
	"0000040C": "fr",
	"0000080C": "be",
	"0001080C": "be", // Belgian (Comma)
	"00000813": "be", // Belgian (Period)
	"00000407": "de",
	"00000807": "ch", // Swiss German
	"0000100C": "ch-fr",
	"0000040A": "es",
	"00000C0A": "es",
	"00000410": "it",
	"0000041D": "se",
	"0000040B": "fi",
	"00000414": "no",
	"00000406": "dk",
}

// FromKLID maps a KLID to a layout id of shared/keymaps.json, or "".
func FromKLID(klid string) string {
	return klids[strings.ToUpper(klid)]
}

// KLIDFromHKL turns the keyboard layout handle of a thread (GetKeyboardLayout)
// into its KLID. The HKL's high word is either the layout's language (the
// plain layout of that language) or 0xFnnn, a "layout id" that the registry
// (Keyboard Layouts\<KLID>\Layout Id) maps back to the KLID: layoutID does
// that lookup.
func KLIDFromHKL(hkl uint32, layoutID func(id uint16) string) string {
	hi := uint16(hkl >> 16)
	if hi&0xF000 == 0xF000 {
		if layoutID == nil {
			return ""
		}
		return strings.ToUpper(layoutID(hi & 0x0FFF))
	}
	return fmt.Sprintf("%08X", uint32(hi))
}
