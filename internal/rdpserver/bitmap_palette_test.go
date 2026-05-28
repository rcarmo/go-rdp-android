package rdpserver

import "testing"

func TestBuildGrayscalePaletteUpdate(t *testing.T) {
	update := buildGrayscalePaletteUpdate()
	if got := le16ForTest(update[0:2]); got != updateTypePalette {
		t.Fatalf("updateType = 0x%04x, want palette", got)
	}
	if got := le16ForTest(update[2:4]); got != 256 {
		t.Fatalf("colors = %d, want 256", got)
	}
	if len(update) != 4+256*3 {
		t.Fatalf("len = %d, want %d", len(update), 4+256*3)
	}
	for _, idx := range []int{0, 127, 255} {
		o := 4 + idx*3
		if update[o] != byte(idx) || update[o+1] != byte(idx) || update[o+2] != byte(idx) {
			t.Fatalf("palette[%d] = %x", idx, update[o:o+3])
		}
	}
}

func TestPrependPaletteUpdateIfNeeded(t *testing.T) {
	updates := [][]byte{{1, 2, 3}}
	got := prependPaletteUpdateIfNeeded(updates, bitmapBPP24)
	if len(got) != 1 || string(got[0]) != string(updates[0]) {
		t.Fatalf("24bpp updates = %#v", got)
	}
	got = prependPaletteUpdateIfNeeded(updates, bitmapBPP8)
	if len(got) != 2 {
		t.Fatalf("8bpp updates len = %d, want 2", len(got))
	}
	if le16ForTest(got[0][0:2]) != updateTypePalette {
		t.Fatalf("first update is not palette: %x", got[0][:2])
	}
}
