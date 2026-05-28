package rdpserver

import "encoding/binary"

const (
	bitmapCompressionFlag     = 0x0001
	noBitmapCompressionHeader = 0x0400
	bitmapRLEMaxLiteralPixels = 0xffff
)

// encodeBitmapRLECopyOnly encodes an 8/15/16/24-bpp bitmap rectangle using only
// RDP bitmap compression COPY orders, plus COLOR orders for solid spans. It is
// intentionally conservative: it does not attempt fill/mix/bicolor optimization
// yet, but it produces standards-shaped compressed streams for every classic RLE
// bpp that go-rdp's client decoder supports and that a server may negotiate.
//
// The input rectangle data is padded, top-down bitmap data. RDP bitmap
// compression streams are emitted bottom-up and omit scanline padding.
func encodeBitmapRLECopyOnly(rect bitmapRect) ([]byte, bool) {
	bytesPerPixel, ok := bitmapRLEBytesPerPixel(rect.BPP)
	if !ok || rect.Width == 0 || rect.Height == 0 {
		return nil, false
	}
	rowBytes := alignedBitmapRowBytes(int(rect.Width), rect.BPP)
	visibleRowBytes := int(rect.Width) * bytesPerPixel
	required := rowBytes * int(rect.Height)
	if visibleRowBytes <= 0 || rowBytes < visibleRowBytes || len(rect.Data) < required {
		return nil, false
	}
	out := make([]byte, 0, visibleRowBytes*int(rect.Height)+int(rect.Height))
	for y := int(rect.Height) - 1; y >= 0; y-- {
		row := rect.Data[y*rowBytes : y*rowBytes+visibleRowBytes]
		for offset := 0; offset < len(row); {
			pixels := (len(row) - offset) / bytesPerPixel
			if pixels > bitmapRLEMaxLiteralPixels {
				pixels = bitmapRLEMaxLiteralPixels
			}
			span := row[offset : offset+pixels*bytesPerPixel]
			out = appendBitmapRLECopyOrder(out, pixels, bytesPerPixel, span)
			offset += pixels * bytesPerPixel
		}
	}
	return out, len(out) > 0
}

func encodeBitmapRLE24CopyOnly(rect bitmapRect) ([]byte, bool) {
	if rect.BPP != bitmapBPP24 {
		return nil, false
	}
	return encodeBitmapRLECopyOnly(rect)
}

func bitmapRLEBytesPerPixel(bpp uint16) (int, bool) {
	switch bpp {
	case 8:
		return 1, true
	case 15, 16:
		return 2, true
	case 24:
		return 3, true
	default:
		return 0, false
	}
}

func appendBitmapRLECopyOrder(out []byte, pixels, bytesPerPixel int, data []byte) []byte {
	if pixels <= 0 || bytesPerPixel <= 0 || len(data) < pixels*bytesPerPixel {
		return out
	}
	if bitmapRLEAllSamePixel(data, pixels, bytesPerPixel) {
		return appendBitmapRLEColorOrder(out, pixels, data[:bytesPerPixel])
	}
	if pixels < 32 {
		out = append(out, byte(0x80|pixels))
	} else if pixels < 32+256 {
		out = append(out, 0x80, byte(pixels-32))
	} else {
		out = append(out, 0xf4)
		out = binary.LittleEndian.AppendUint16(out, uint16(pixels))
	}
	return append(out, data[:pixels*bytesPerPixel]...)
}

func appendBitmapRLEColorOrder(out []byte, pixels int, pixel []byte) []byte {
	if pixels <= 0 || len(pixel) == 0 {
		return out
	}
	if pixels < 32 {
		out = append(out, byte(0x60|pixels))
	} else if pixels < 32+256 {
		out = append(out, 0x60, byte(pixels-32))
	} else {
		out = append(out, 0xf3)
		out = binary.LittleEndian.AppendUint16(out, uint16(pixels))
	}
	return append(out, pixel...)
}

func bitmapRLEAllSamePixel(data []byte, pixels, bytesPerPixel int) bool {
	if pixels <= 1 || bytesPerPixel <= 0 || len(data) < pixels*bytesPerPixel {
		return false
	}
	first := data[:bytesPerPixel]
	for i := 1; i < pixels; i++ {
		o := i * bytesPerPixel
		for b := 0; b < bytesPerPixel; b++ {
			if data[o+b] != first[b] {
				return false
			}
		}
	}
	return true
}
