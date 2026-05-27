package rdpserver

import (
	"encoding/binary"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

const (
	clearCodecMagic            = "CLR0"
	clearCodecOpSolidRect byte = 1
	clearCodecOpRawRect   byte = 2
)

type clearCodecEncoder struct{}

func (clearCodecEncoder) EncodeRDPGFX(src frame.Frame, width, height int) ([]byte, bool) {
	if src.Width <= 0 || src.Height <= 0 || src.Width > 8192 || src.Height > 8192 {
		return nil, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return nil, false
	}
	rawBytes := src.Width * src.Height * 4
	if rawBytes <= 0 || rawBytes > rdpgfxMaxPDUSize {
		return nil, false
	}
	raw565Bytes := src.Width * src.Height * 2
	if raw565Bytes <= 0 {
		return nil, false
	}

	if r, g, b, solid := clearCodecSolidRGB(src, stride); solid {
		payload := make([]byte, 0, 4+2+2+2+1+8+3)
		payload = append(payload, clearCodecMagic...)
		payload = appendLE16Bytes(payload, uint16(src.Width))
		payload = appendLE16Bytes(payload, uint16(src.Height))
		payload = appendLE16Bytes(payload, 1) // rect count
		payload = append(payload, clearCodecOpSolidRect)
		payload = appendLE16Bytes(payload, 0)
		payload = appendLE16Bytes(payload, 0)
		payload = appendLE16Bytes(payload, uint16(src.Width))
		payload = appendLE16Bytes(payload, uint16(src.Height))
		payload = append(payload, r, g, b)
		if len(payload) >= rawBytes || len(payload) > rdpgfxMaxPDUSize {
			return nil, false
		}
		return payload, true
	}

	rectHeaderLen := 1 + 2 + 2 + 2 + 2 + 4
	headerLen := 4 + 2 + 2 + 2
	total := headerLen + rectHeaderLen + raw565Bytes
	if total > rdpgfxMaxPDUSize || total >= rawBytes {
		return nil, false
	}
	payload := make([]byte, total)
	off := 0
	copy(payload[off:], []byte(clearCodecMagic))
	off += 4
	binary.LittleEndian.PutUint16(payload[off:off+2], uint16(src.Width))
	off += 2
	binary.LittleEndian.PutUint16(payload[off:off+2], uint16(src.Height))
	off += 2
	binary.LittleEndian.PutUint16(payload[off:off+2], 1)
	off += 2
	payload[off] = clearCodecOpRawRect
	off++
	binary.LittleEndian.PutUint16(payload[off:off+2], 0)
	off += 2
	binary.LittleEndian.PutUint16(payload[off:off+2], 0)
	off += 2
	binary.LittleEndian.PutUint16(payload[off:off+2], uint16(src.Width))
	off += 2
	binary.LittleEndian.PutUint16(payload[off:off+2], uint16(src.Height))
	off += 2
	binary.LittleEndian.PutUint32(payload[off:off+4], uint32(raw565Bytes))
	off += 4

	for y := 0; y < src.Height; y++ {
		row := y * stride
		for x := 0; x < src.Width; x++ {
			si := row + x*4
			var r8, g8, b8 byte
			switch src.Format {
			case frame.PixelFormatRGBA8888:
				r8, g8, b8 = src.Data[si+0], src.Data[si+1], src.Data[si+2]
			case frame.PixelFormatBGRA8888:
				r8, g8, b8 = src.Data[si+2], src.Data[si+1], src.Data[si+0]
			default:
				return nil, false
			}
			r5 := uint16(r8 >> 3)
			g6 := uint16(g8 >> 2)
			b5 := uint16(b8 >> 3)
			rgb565 := (r5 << 11) | (g6 << 5) | b5
			binary.LittleEndian.PutUint16(payload[off:off+2], rgb565)
			off += 2
		}
	}
	return payload, true
}

func clearCodecSolidRGB(src frame.Frame, stride int) (r, g, b byte, solid bool) {
	if src.Width <= 0 || src.Height <= 0 {
		return 0, 0, 0, false
	}
	firstR, firstG, firstB, ok := clearCodecRGBAt(src, stride, 0, 0)
	if !ok {
		return 0, 0, 0, false
	}
	for y := 0; y < src.Height; y++ {
		for x := 0; x < src.Width; x++ {
			r, g, b, ok := clearCodecRGBAt(src, stride, x, y)
			if !ok || r != firstR || g != firstG || b != firstB {
				return 0, 0, 0, false
			}
		}
	}
	return firstR, firstG, firstB, true
}

func clearCodecRGBAt(src frame.Frame, stride, x, y int) (r, g, b byte, ok bool) {
	si := y*stride + x*4
	switch src.Format {
	case frame.PixelFormatRGBA8888:
		return src.Data[si+0], src.Data[si+1], src.Data[si+2], true
	case frame.PixelFormatBGRA8888:
		return src.Data[si+2], src.Data[si+1], src.Data[si+0], true
	default:
		return 0, 0, 0, false
	}
}
