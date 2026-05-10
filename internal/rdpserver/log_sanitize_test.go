package rdpserver

import "testing"

func TestSanitizeForLog(t *testing.T) {
	if got := sanitizeForLog("", 16); got != "" {
		t.Fatalf("empty sanitize = %q", got)
	}
	if got := sanitizeForLog("ru\x01i", 16); got != "ru?i" {
		t.Fatalf("control char sanitize = %q", got)
	}
	if got := sanitizeForLog("abcdefghijkl", 5); got != "abcde…" {
		t.Fatalf("truncate sanitize = %q", got)
	}
}
