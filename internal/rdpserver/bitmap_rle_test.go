package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestEncodeBitmapRLE24CopyOnlyRoundTrip(t *testing.T) {
	rect := bitmapRect{Width: 3, Height: 2, BPP: bitmapBPP24}
	rowBytes := alignedBitmapRowBytes(int(rect.Width), rect.BPP)
	rect.Data = make([]byte, rowBytes*int(rect.Height))
	copy(rect.Data[0:9], []byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	copy(rect.Data[rowBytes:rowBytes+9], []byte{10, 11, 12, 13, 14, 15, 16, 17, 18})
	encoded, ok := encodeBitmapRLE24CopyOnly(rect)
	if !ok {
		t.Fatal("encodeBitmapRLE24CopyOnly() ok = false")
	}
	decoded, ok := decodeBitmapRLE24CopyOnlyForTest(encoded, int(rect.Width), int(rect.Height))
	if !ok {
		t.Fatalf("decode failed for %x", encoded)
	}
	if string(decoded) != string(rect.Data) {
		t.Fatalf("decoded = %x, want %x", decoded, rect.Data)
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

func TestBuildCompressedBitmapRLEUpdateRejectsExpansion(t *testing.T) {
	rect := bitmapRect{Width: 80, Height: 1, BPP: bitmapBPP24, Data: make([]byte, alignedBitmapRowBytes(80, bitmapBPP24))}
	for i := range rect.Data {
		rect.Data[i] = byte(i)
	}
	if _, ok := buildCompressedBitmapRLEUpdate([]bitmapRect{rect}); ok {
		t.Fatal("expected compressed update that expands data to be rejected")
	}
}

func TestEncodeBitmapRLE24CopyOnlyRejectsInvalid(t *testing.T) {
	if _, ok := encodeBitmapRLE24CopyOnly(bitmapRect{Width: 1, Height: 1, BPP: 16, Data: []byte{0}}); ok {
		t.Fatal("expected non-24bpp input to be rejected")
	}
	if _, ok := encodeBitmapRLE24CopyOnly(bitmapRect{Width: 2, Height: 2, BPP: bitmapBPP24, Data: []byte{0}}); ok {
		t.Fatal("expected short data to be rejected")
	}
}

func decodeBitmapRLE24CopyOnlyForTest(data []byte, width, height int) ([]byte, bool) {
	if width <= 0 || height <= 0 {
		return nil, false
	}
	rowBytes := alignedBitmapRowBytes(width, bitmapBPP24)
	visibleRowBytes := width * 3
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
				if offset+3 > len(data) {
					return nil, false
				}
				for i := 0; i < pixels; i++ {
					copy(out[rowOffset+(rowPixels+i)*3:], data[offset:offset+3])
				}
				offset += 3
			} else {
				if offset+pixels*3 > len(data) {
					return nil, false
				}
				copy(out[rowOffset+rowPixels*3:], data[offset:offset+pixels*3])
				offset += pixels * 3
			}
			rowPixels += pixels
		}
		if rowPixels*3 != visibleRowBytes {
			return nil, false
		}
	}
	return out, offset == len(data)
}
