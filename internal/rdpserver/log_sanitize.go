package rdpserver

import "strings"

func sanitizeForLog(value string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 64
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var b strings.Builder
	count := 0
	for _, r := range trimmed {
		if count >= maxRunes {
			b.WriteRune('…')
			break
		}
		if r < 0x20 || r == 0x7f {
			b.WriteRune('?')
		} else {
			b.WriteRune(r)
		}
		count++
	}
	return b.String()
}
