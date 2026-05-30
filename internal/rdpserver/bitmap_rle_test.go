package rdpserver

import (
	"fmt"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func BenchmarkEncodeBitmapRLESolid64x64(b *testing.B) {
	rect := buildSolidBitmapRectForBPP(64, 64, 0xff336699, bitmapBPP24)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		encoded, ok := encodeBitmapRLECopyOnly(rect)
		if !ok || len(encoded) == 0 {
			b.Fatal("bad RLE encoding")
		}
	}
}

func TestEncodeBitmapRLECopyOnlyRoundTripAllClassicDepths(t *testing.T) {
	for _, tc := range []struct {
		name          string
		bpp           uint16
		bytesPerPixel int
	}{
		{name: "8bpp", bpp: 8, bytesPerPixel: 1},
		{name: "15bpp", bpp: 15, bytesPerPixel: 2},
		{name: "16bpp", bpp: 16, bytesPerPixel: 2},
		{name: "24bpp", bpp: bitmapBPP24, bytesPerPixel: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rect := bitmapRect{Width: 3, Height: 2, BPP: tc.bpp}
			rowBytes := alignedBitmapRowBytes(int(rect.Width), rect.BPP)
			rect.Data = make([]byte, rowBytes*int(rect.Height))
			visible := int(rect.Width) * tc.bytesPerPixel
			for i := 0; i < visible; i++ {
				rect.Data[i] = byte(i + 1)
				rect.Data[rowBytes+i] = byte(i + 1 + visible)
			}
			encoded, ok := encodeBitmapRLECopyOnly(rect)
			if !ok {
				t.Fatal("encodeBitmapRLECopyOnly() ok = false")
			}
			decoded, ok := decodeBitmapRLECopyOnlyForTest(encoded, int(rect.Width), int(rect.Height), tc.bpp)
			if !ok {
				t.Fatalf("decode failed for %x", encoded)
			}
			if string(decoded) != string(rect.Data) {
				t.Fatalf("decoded = %x, want %x", decoded, rect.Data)
			}
		})
	}
}

func TestBuildCompressedBitmapRLEUpdate(t *testing.T) {
	rect := bitmapRect{Width: 80, Height: 1, BPP: bitmapBPP24, Data: make([]byte, alignedBitmapRowBytes(80, bitmapBPP24))}
	update, ok := buildCompressedBitmapRLEUpdate([]bitmapRect{rect})
	if !ok {
		t.Fatal("buildCompressedBitmapRLEUpdate() ok = false")
	}
	if rects, err := parseBitmapUpdateHeader(update); err != nil || rects != 1 {
		t.Fatalf("parseBitmapUpdateHeader() rects=%d err=%v", rects, err)
	}
	flags := le16ForTest(update[4+14 : 4+16])
	if flags != bitmapCompressionFlag|noBitmapCompressionHeader {
		t.Fatalf("flags = 0x%04x", flags)
	}
	compressedLen := int(le16ForTest(update[4+16 : 4+18]))
	if compressedLen >= len(rect.Data) {
		t.Fatalf("compressed length = %d, uncompressed = %d", compressedLen, len(rect.Data))
	}
	if compressedLen != len(update)-(4+18) {
		t.Fatalf("compressed length = %d, payload has %d", compressedLen, len(update)-(4+18))
	}
}

func TestBuildFrameBitmapUpdatesCanUseRLEWhenEnabled(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_BITMAP_RLE", "1")
	fr := frameOfSolidBGRAForRLETest(80, 1)
	updates, ok := buildFrameBitmapUpdates(fr)
	if !ok || len(updates) != 1 {
		t.Fatalf("buildFrameBitmapUpdates() ok=%t len=%d", ok, len(updates))
	}
	flags := le16ForTest(updates[0][4+14 : 4+16])
	if flags != bitmapCompressionFlag|noBitmapCompressionHeader {
		t.Fatalf("flags = 0x%04x", flags)
	}
}

func TestBuildFrameBitmapUpdatesLeavesRLEDisabledByDefault(t *testing.T) {
	fr := frameOfSolidBGRAForRLETest(80, 1)
	updates, ok := buildFrameBitmapUpdates(fr)
	if !ok || len(updates) != 1 {
		t.Fatalf("buildFrameBitmapUpdates() ok=%t len=%d", ok, len(updates))
	}
	flags := le16ForTest(updates[0][4+14 : 4+16])
	if flags != 0 {
		t.Fatalf("flags = 0x%04x, want uncompressed", flags)
	}
}

func frameOfSolidBGRAForRLETest(width, height int) frame.Frame {
	data := make([]byte, width*height*4)
	for i := 0; i < len(data); i += 4 {
		data[i+0] = 0x44
		data[i+1] = 0x33
		data[i+2] = 0x22
		data[i+3] = 0xff
	}
	return frame.Frame{Width: width, Height: height, Stride: width * 4, Format: frame.PixelFormatBGRA8888, Data: data}
}

func TestBuildSolidBitmapUpdateUsesRLEWhenEnabled(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_BITMAP_RLE", "1")
	update := buildSolidBitmapUpdate(64, 64, 0xff336699)
	flags := le16ForTest(update[4+14 : 4+16])
	if flags != bitmapCompressionFlag|noBitmapCompressionHeader {
		t.Fatalf("flags = 0x%04x", flags)
	}
}

func TestBitmapRLEStatsFromUpdates(t *testing.T) {
	for _, bpp := range []uint16{bitmapBPP8, bitmapBPP15, bitmapBPP16, bitmapBPP24} {
		t.Run(fmt.Sprintf("%dbpp", bpp), func(t *testing.T) {
			rect := buildSolidBitmapRectForBPP(64, 64, 0xff336699, bpp)
			update, ok := buildCompressedBitmapRLEUpdate([]bitmapRect{rect})
			if !ok {
				t.Fatal("buildCompressedBitmapRLEUpdate() ok = false")
			}
			rects, bytes, saved := bitmapRLEStatsFromUpdates([][]byte{update})
			if rects != 1 {
				t.Fatalf("rects = %d, want 1", rects)
			}
			if bytes <= 0 {
				t.Fatalf("bytes = %d, want positive", bytes)
			}
			if saved <= 0 {
				t.Fatalf("saved = %d, want positive", saved)
			}
		})
	}
}

func TestBitmapRLEStatsFromUpdatesIgnoresMalformed(t *testing.T) {
	for _, update := range [][][]byte{
		{nil},
		{{0x00, 0x00}},
		{{0x01, 0x00, 0x01, 0x00, 0x00}},
		{{0x01, 0x00, 0x01, 0x00, 0, 0, 0, 0, 64, 0, 64, 0, 24, 0, byte(bitmapCompressionFlag), byte((bitmapCompressionFlag | noBitmapCompressionHeader) >> 8), 0xff, 0xff}},
	} {
		rects, bytes, saved := bitmapRLEStatsFromUpdates(update)
		if rects != 0 || bytes != 0 || saved != 0 {
			t.Fatalf("bitmapRLEStatsFromUpdates(%x) = %d,%d,%d", update, rects, bytes, saved)
		}
	}
}

func TestBuildCompressedBitmapRLEUpdateRejectsExpansion(t *testing.T) {
	rect := bitmapRect{Width: 80, Height: 1, BPP: bitmapBPP24, Data: make([]byte, alignedBitmapRowBytes(80, bitmapBPP24))}
	for i := range rect.Data {
		rect.Data[i] = byte(i)
	}
	if _, ok := buildCompressedBitmapRLEUpdate([]bitmapRect{rect}); ok {
		t.Fatal("expected compressed update that expands data to be rejected")
	}
}

func TestEncodeBitmapRLECopyOnlyRejectsInvalid(t *testing.T) {
	if _, ok := encodeBitmapRLECopyOnly(bitmapRect{Width: 1, Height: 1, BPP: 32, Data: []byte{0, 0, 0, 0}}); ok {
		t.Fatal("expected unsupported 32bpp input to be rejected")
	}
	if _, ok := encodeBitmapRLECopyOnly(bitmapRect{Width: 2, Height: 2, BPP: bitmapBPP24, Data: []byte{0}}); ok {
		t.Fatal("expected short data to be rejected")
	}
}

func decodeBitmapRLECopyOnlyForTest(data []byte, width, height int, bpp uint16) ([]byte, bool) {
	if width <= 0 || height <= 0 {
		return nil, false
	}
	bytesPerPixel, ok := bitmapRLEBytesPerPixel(bpp)
	if !ok {
		return nil, false
	}
	rowBytes := alignedBitmapRowBytes(width, bpp)
	visibleRowBytes := width * bytesPerPixel
	out := make([]byte, rowBytes*height)
	offset := 0
	for y := height - 1; y >= 0; y-- {
		rowOffset := y * rowBytes
		rowPixels := 0
		for rowPixels < width {
			if offset >= len(data) {
				return nil, false
			}
			code := data[offset]
			offset++
			pixels := 0
			isColor := false
			switch {
			case code&0xe0 == 0x60 && code != 0x60:
				pixels = int(code & 0x1f)
				isColor = true
			case code == 0x60:
				if offset >= len(data) {
					return nil, false
				}
				pixels = 32 + int(data[offset])
				offset++
				isColor = true
			case code == 0xf3:
				if offset+2 > len(data) {
					return nil, false
				}
				pixels = int(data[offset]) | int(data[offset+1])<<8
				offset += 2
				isColor = true
			case code&0xe0 == 0x80 && code != 0x80:
				pixels = int(code & 0x1f)
			case code == 0x80:
				if offset >= len(data) {
					return nil, false
				}
				pixels = 32 + int(data[offset])
				offset++
			case code == 0xf4:
				if offset+2 > len(data) {
					return nil, false
				}
				pixels = int(data[offset]) | int(data[offset+1])<<8
				offset += 2
			default:
				return nil, false
			}
			if pixels <= 0 || rowPixels+pixels > width {
				return nil, false
			}
			if isColor {
				if offset+bytesPerPixel > len(data) {
					return nil, false
				}
				for i := 0; i < pixels; i++ {
					copy(out[rowOffset+(rowPixels+i)*bytesPerPixel:], data[offset:offset+bytesPerPixel])
				}
				offset += bytesPerPixel
			} else {
				if offset+pixels*bytesPerPixel > len(data) {
					return nil, false
				}
				copy(out[rowOffset+rowPixels*bytesPerPixel:], data[offset:offset+pixels*bytesPerPixel])
				offset += pixels * bytesPerPixel
			}
			rowPixels += pixels
		}
		if rowPixels*bytesPerPixel != visibleRowBytes {
			return nil, false
		}
	}
	return out, offset == len(data)
}
