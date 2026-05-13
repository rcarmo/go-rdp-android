package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBuildSolidBitmapUpdate(t *testing.T) {
	payload := buildSolidBitmapUpdate(8, 4, 0xff336699)
	rects, err := parseBitmapUpdateHeader(payload)
	if err != nil {
		t.Fatal(err)
	}
	if rects != 1 {
		t.Fatalf("expected one rectangle, got %d", rects)
	}
	// update header (4) + rect header (18) + 8*4 rows aligned to 4 bytes.
	if want := 4 + 18 + 96; len(payload) != want {
		t.Fatalf("unexpected payload length: got %d want %d", len(payload), want)
	}
}

func TestBuildSolidBitmapUpdateClamp(t *testing.T) {
	payload := buildSolidBitmapUpdate(1000, 1000, 0)
	if want := 4 + 18 + 64*64*3; len(payload) != want {
		t.Fatalf("unexpected clamped payload length: got %d want %d", len(payload), want)
	}
}

func TestBuildFrameBitmapUpdateRGBA(t *testing.T) {
	payload, ok := buildFrameBitmapUpdate(frame.Frame{
		Width:  1,
		Height: 1,
		Stride: 4,
		Format: frame.PixelFormatRGBA8888,
		Data:   []byte{0x11, 0x22, 0x33, 0x44},
	})
	if !ok {
		t.Fatal("expected frame conversion")
	}
	if got := payload[len(payload)-4:]; got[0] != 0x33 || got[1] != 0x22 || got[2] != 0x11 || got[3] != 0x00 {
		t.Fatalf("unexpected BGRA bytes: %x", got)
	}
}

func TestBuildFrameBitmapUpdateHandlesStrideConversionAlphaAndAlignment(t *testing.T) {
	payload, ok := buildFrameBitmapUpdate(frame.Frame{
		Width:  2,
		Height: 2,
		Stride: 12,
		Format: frame.PixelFormatRGBA8888,
		Data: []byte{
			0x10, 0x20, 0x30, 0xaa, 0x40, 0x50, 0x60, 0xbb, 0xee, 0xee, 0xee, 0xee,
			0x70, 0x80, 0x90, 0xcc, 0xa0, 0xb0, 0xc0, 0xdd, 0xff, 0xff, 0xff, 0xff,
		},
	})
	if !ok {
		t.Fatal("expected padded RGBA frame conversion")
	}
	const bitmapDataOffset = 4 + 18
	data := payload[bitmapDataOffset:]
	if len(data) != alignedBitmapRowBytes(2, bitmapBPP24)*2 {
		t.Fatalf("unexpected bitmap data length %d", len(data))
	}
	want := []byte{
		0x30, 0x20, 0x10, 0x60, 0x50, 0x40, 0x00, 0x00,
		0x90, 0x80, 0x70, 0xc0, 0xb0, 0xa0, 0x00, 0x00,
	}
	if string(data) != string(want) {
		t.Fatalf("unexpected converted bitmap bytes: got %x want %x", data, want)
	}
}

func TestBuildFrameBitmapUpdateHandlesBGRAConversion(t *testing.T) {
	payload, ok := buildFrameBitmapUpdate(frame.Frame{
		Width:  1,
		Height: 1,
		Stride: 4,
		Format: frame.PixelFormatBGRA8888,
		Data:   []byte{0x11, 0x22, 0x33, 0xff},
	})
	if !ok {
		t.Fatal("expected BGRA frame conversion")
	}
	if got := payload[len(payload)-4:]; got[0] != 0x11 || got[1] != 0x22 || got[2] != 0x33 || got[3] != 0x00 {
		t.Fatalf("unexpected BGR/padding bytes: %x", got)
	}
}

func TestBuildFrameBitmapUpdatesRejectsInvalidGeometry(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	cases := []struct {
		name string
		fr   frame.Frame
	}{
		{
			name: "short row stride",
			fr:   frame.Frame{Width: 2, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: make([]byte, 8)},
		},
		{
			name: "short data",
			fr:   frame.Frame{Width: 2, Height: 2, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: make([]byte, 15)},
		},
		{
			name: "width overflow",
			fr:   frame.Frame{Width: maxInt/4 + 1, Height: 1, Format: frame.PixelFormatRGBA8888, Data: make([]byte, 16)},
		},
		{
			name: "byte size overflow",
			fr:   frame.Frame{Width: 1, Height: maxInt/4 + 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: make([]byte, 16)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if updates, ok := buildFrameBitmapUpdates(tc.fr); ok || updates != nil {
				t.Fatalf("expected invalid frame to be rejected, ok=%v updates=%d", ok, len(updates))
			}
		})
	}
}

func TestAlignedBitmapRowBytes(t *testing.T) {
	if got := alignedBitmapRowBytes(1, bitmapBPP24); got != 4 {
		t.Fatalf("expected one 24bpp pixel to align to 4 bytes, got %d", got)
	}
	if got := alignedBitmapRowBytes(80, bitmapBPP24); got != 240 {
		t.Fatalf("expected 80 24bpp pixels to use 240 bytes, got %d", got)
	}
}

func TestBuildFrameBitmapUpdatesDirtyTileCache(t *testing.T) {
	width, height := 160, 80
	data := make([]byte, width*height*4)
	for i := range data {
		data[i] = byte(i)
	}
	fr := frame.Frame{Width: width, Height: height, Stride: width * 4, Format: frame.PixelFormatBGRA8888, Data: data}
	cache := newBitmapTileCache()
	updates, ok := buildFrameBitmapUpdatesWithCache(fr, cache, false)
	if !ok || len(updates) != 2 {
		t.Fatalf("expected initial two-tile frame, got ok=%v updates=%d", ok, len(updates))
	}
	updates, ok = buildFrameBitmapUpdatesWithCache(fr, cache, true)
	if !ok || len(updates) != 0 {
		t.Fatalf("expected unchanged frame to produce no dirty updates, got ok=%v updates=%d", ok, len(updates))
	}

	changed := append([]byte(nil), data...)
	// Pixel x=100 lands in the second 80x80 tile.
	changed[(100*4)+0] ^= 0xff
	fr.Data = changed
	updates, ok = buildFrameBitmapUpdatesWithCache(fr, cache, true)
	if !ok || len(updates) != 1 {
		t.Fatalf("expected one dirty tile after one-pixel change, got ok=%v updates=%d", ok, len(updates))
	}
}

func TestBuildFrameBitmapUpdatesTilesLargeFrame(t *testing.T) {
	width, height := 200, 90
	data := make([]byte, width*height*4)
	for i := range data {
		data[i] = byte(i)
	}
	updates, ok := buildFrameBitmapUpdates(frame.Frame{
		Width:  width,
		Height: height,
		Stride: width * 4,
		Format: frame.PixelFormatBGRA8888,
		Data:   data,
	})
	if !ok {
		t.Fatal("expected tiled frame conversion")
	}
	if len(updates) != 6 {
		t.Fatalf("expected 6 PER-safe tiles, got %d", len(updates))
	}
	for _, update := range updates {
		rects, err := parseBitmapUpdateHeader(update)
		if err != nil {
			t.Fatal(err)
		}
		if rects != 1 {
			t.Fatalf("expected one rect per update, got %d", rects)
		}
		if len(update) > 32767 {
			t.Fatalf("update too large for PER length envelope: %d", len(update))
		}
	}
}

func TestScaleFrameNearestRejectsInvalidGeometry(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	cases := []struct {
		name          string
		fr            frame.Frame
		width, height int
	}{
		{
			name:   "short source stride",
			fr:     frame.Frame{Width: 2, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: make([]byte, 8)},
			width:  1,
			height: 1,
		},
		{
			name:   "short source data",
			fr:     frame.Frame{Width: 2, Height: 2, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: make([]byte, 15)},
			width:  1,
			height: 1,
		},
		{
			name:   "destination stride overflow",
			fr:     frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: make([]byte, 4)},
			width:  maxInt/4 + 1,
			height: 1,
		},
		{
			name:   "destination byte overflow",
			fr:     frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: make([]byte, 4)},
			width:  1,
			height: maxInt/4 + 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if scaled, ok := scaleFrameNearest(tc.fr, tc.width, tc.height); ok || len(scaled.Data) != 0 {
				t.Fatalf("expected invalid scale to fail, ok=%v scaled=%#v", ok, scaled)
			}
		})
	}
}

func TestBuildFrameBitmapUpdatesForDesktopScalesToClientSize(t *testing.T) {
	src := frame.Frame{
		Width:  2,
		Height: 2,
		Stride: 8,
		Format: frame.PixelFormatRGBA8888,
		Data: []byte{
			0xff, 0x00, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff,
			0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		},
	}
	updates, ok := buildFrameBitmapUpdatesForDesktop(src, newBitmapTileCache(), false, 1, 1)
	if !ok || len(updates) != 1 {
		t.Fatalf("expected single scaled update, got ok=%v len=%d", ok, len(updates))
	}
	update := updates[0]
	if got := uint16(update[12]) | uint16(update[13])<<8; got != 1 {
		t.Fatalf("rect width=%d, want 1", got)
	}
	if got := uint16(update[14]) | uint16(update[15])<<8; got != 1 {
		t.Fatalf("rect height=%d, want 1", got)
	}
	pixel := update[len(update)-4:]
	if pixel[0] != 0x00 || pixel[1] != 0x00 || pixel[2] != 0xff {
		t.Fatalf("unexpected scaled top-left pixel (BGR): %x", pixel)
	}
}

func TestBuildFrameBitmapUpdatesCacheResetsOnResize(t *testing.T) {
	cache := newBitmapTileCache()
	first := frame.Frame{Width: 4, Height: 4, Stride: 16, Format: frame.PixelFormatBGRA8888, Data: make([]byte, 4*4*4)}
	if updates, ok := buildFrameBitmapUpdatesForDesktop(first, cache, false, 4, 4); !ok || len(updates) == 0 {
		t.Fatalf("expected initial updates, got ok=%v len=%d", ok, len(updates))
	}
	if updates, ok := buildFrameBitmapUpdatesForDesktop(first, cache, true, 4, 4); !ok || len(updates) != 0 {
		t.Fatalf("expected no dirty updates on unchanged frame, got ok=%v len=%d", ok, len(updates))
	}
	if updates, ok := buildFrameBitmapUpdatesForDesktop(first, cache, true, 2, 2); !ok || len(updates) == 0 {
		t.Fatalf("expected updates after desktop resize/cache reset, got ok=%v len=%d", ok, len(updates))
	}
}
