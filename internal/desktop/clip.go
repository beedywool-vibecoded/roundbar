package desktop

import (
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxClips   = 10
	maxClipLen = 2000
)

// pushHistory puts text first in a clipboard history (≤ 10, each ≤ 2000 chars).
func pushHistory(list []string, text string) ([]string, bool) {
	text = strings.ToValidUTF8(text, "")
	if utf8.RuneCountInString(text) > maxClipLen {
		text = string([]rune(text)[:maxClipLen])
	}
	if strings.TrimSpace(text) == "" {
		return list, false
	}
	if len(list) > 0 && list[0] == text {
		return list, false
	}
	out := append([]string{text}, slices.DeleteFunc(slices.Clone(list), func(s string) bool { return s == text })...)
	if len(out) > maxClips {
		out = out[:maxClips]
	}
	return out, true
}
