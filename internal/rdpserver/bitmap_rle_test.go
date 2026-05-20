package rdpserver

import "testing"

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
	rect := bitmapRect{Width: 2, Height: 1, BPP: bitmapBPP24, Data: []byte{1, 2, 3, 4, 5, 6, 0, 0}}
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
	if compressedLen != len(update)-(4+18) {
		t.Fatalf("compressed length = %d, payload has %d", compressedLen, len(update)-(4+18))
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
			switch {
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
			if pixels <= 0 || rowPixels+pixels > width || offset+pixels*3 > len(data) {
				return nil, false
			}
			copy(out[rowOffset+rowPixels*3:], data[offset:offset+pixels*3])
			offset += pixels * 3
			rowPixels += pixels
		}
		if rowPixels*3 != visibleRowBytes {
			return nil, false
		}
	}
	return out, offset == len(data)
}
