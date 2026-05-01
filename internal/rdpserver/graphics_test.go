package rdpserver

import "testing"

func TestBuildSolidBitmapUpdate(t *testing.T) {
	payload := buildSolidBitmapUpdate(8, 4, 0xff336699)
	rects, err := parseBitmapUpdateHeader(payload)
	if err != nil {
		t.Fatal(err)
	}
	if rects != 1 {
		t.Fatalf("expected one rectangle, got %d", rects)
	}
	// update header (4) + rect header (18) + 8*4*4 bytes.
	if want := 4 + 18 + 128; len(payload) != want {
		t.Fatalf("unexpected payload length: got %d want %d", len(payload), want)
	}
}

func TestBuildSolidBitmapUpdateClamp(t *testing.T) {
	payload := buildSolidBitmapUpdate(1000, 1000, 0)
	if want := 4 + 18 + 64*64*4; len(payload) != want {
		t.Fatalf("unexpected clamped payload length: got %d want %d", len(payload), want)
	}
}
