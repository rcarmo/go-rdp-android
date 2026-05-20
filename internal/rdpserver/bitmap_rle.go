package rdpserver

import "encoding/binary"

const (
	bitmapCompressionFlag       = 0x0001
	noBitmapCompressionHeader   = 0x0400
	bitmapRLE24MaxLiteralPixels = 0xffff
)

// encodeBitmapRLE24CopyOnly encodes a 24-bpp bitmap rectangle using only RDP bitmap
// compression COPY orders. It is intentionally conservative: it does not attempt
// fill/mix/bicolor optimization yet, but it produces a standards-shaped compressed
// stream that is useful as the first legacy bitmap-compression building block.
//
// The input rectangle data is the server's normal padded, top-down BGR tile data.
// RDP bitmap compression streams are emitted bottom-up and omit scanline padding.
func encodeBitmapRLE24CopyOnly(rect bitmapRect) ([]byte, bool) {
	if rect.BPP != bitmapBPP24 || rect.Width == 0 || rect.Height == 0 {
		return nil, false
	}
	rowBytes := alignedBitmapRowBytes(int(rect.Width), rect.BPP)
	visibleRowBytes := int(rect.Width) * 3
	required := rowBytes * int(rect.Height)
	if visibleRowBytes <= 0 || rowBytes < visibleRowBytes || len(rect.Data) < required {
		return nil, false
	}
	out := make([]byte, 0, visibleRowBytes*int(rect.Height)+int(rect.Height))
	for y := int(rect.Height) - 1; y >= 0; y-- {
		row := rect.Data[y*rowBytes : y*rowBytes+visibleRowBytes]
		for offset := 0; offset < len(row); {
			pixels := (len(row) - offset) / 3
			if pixels > bitmapRLE24MaxLiteralPixels {
				pixels = bitmapRLE24MaxLiteralPixels
			}
			out = appendBitmapRLECopyOrder24(out, pixels, row[offset:offset+pixels*3])
			offset += pixels * 3
		}
	}
	return out, len(out) > 0
}

func appendBitmapRLECopyOrder24(out []byte, pixels int, data []byte) []byte {
	if pixels <= 0 {
		return out
	}
	if bitmapRLE24AllSamePixel(data, pixels) {
		return appendBitmapRLEColorOrder24(out, pixels, data[:3])
	}
	if pixels < 32 {
		out = append(out, byte(0x80|pixels))
	} else if pixels < 32+256 {
		out = append(out, 0x80, byte(pixels-32))
	} else {
		out = append(out, 0xf4)
		out = binary.LittleEndian.AppendUint16(out, uint16(pixels))
	}
	return append(out, data...)
}

func appendBitmapRLEColorOrder24(out []byte, pixels int, pixel []byte) []byte {
	if pixels <= 0 || len(pixel) < 3 {
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
	return append(out, pixel[0], pixel[1], pixel[2])
}

func bitmapRLE24AllSamePixel(data []byte, pixels int) bool {
	if pixels <= 1 || len(data) < pixels*3 {
		return false
	}
	b, g, r := data[0], data[1], data[2]
	for i := 1; i < pixels; i++ {
		o := i * 3
		if data[o] != b || data[o+1] != g || data[o+2] != r {
			return false
		}
	}
	return true
}
