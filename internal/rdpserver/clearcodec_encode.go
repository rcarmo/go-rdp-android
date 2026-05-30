package rdpserver

import (
	"encoding/binary"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

const (
	clearCodecMagic            = "CLR0"
	clearCodecOpSolidRect byte = 1
	clearCodecOpRawRect   byte = 2

	clearCodecMaxRawRectBytes = 256 * 1024
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
		payload := make([]byte, 22)
		copy(payload[0:4], clearCodecMagic)
		binary.LittleEndian.PutUint16(payload[4:6], uint16(src.Width))
		binary.LittleEndian.PutUint16(payload[6:8], uint16(src.Height))
		binary.LittleEndian.PutUint16(payload[8:10], 1) // rect count
		payload[10] = clearCodecOpSolidRect
		binary.LittleEndian.PutUint16(payload[15:17], uint16(src.Width))
		binary.LittleEndian.PutUint16(payload[17:19], uint16(src.Height))
		payload[19], payload[20], payload[21] = r, g, b
		if len(payload) >= rawBytes || len(payload) > rdpgfxMaxPDUSize {
			return nil, false
		}
		return payload, true
	}

	payload, ok := buildClearCodecRects(src, stride, rawBytes)
	if !ok {
		return nil, false
	}
	return payload, true
}

func buildClearCodecRawRects(src frame.Frame, stride int, rawBytes int) ([]byte, bool) {
	return buildClearCodecRects(src, stride, rawBytes)
}

func buildClearCodecRects(src frame.Frame, stride int, rawBytes int) ([]byte, bool) {
	const tileSize = 64
	tilesX := (src.Width + tileSize - 1) / tileSize
	tilesY := (src.Height + tileSize - 1) / tileSize
	numRects := tilesX * tilesY
	if numRects <= 0 || numRects > 0xffff {
		return nil, false
	}
	headerLen := 4 + 2 + 2 + 2
	maxRectHeaderLen := numRects * (1 + 2 + 2 + 2 + 2 + 4)
	payload := make([]byte, headerLen, headerLen+src.Width*src.Height*2+maxRectHeaderLen)
	copy(payload[0:4], clearCodecMagic)
	binary.LittleEndian.PutUint16(payload[4:6], uint16(src.Width))
	binary.LittleEndian.PutUint16(payload[6:8], uint16(src.Height))
	// Rect count at payload[8:10] is patched below.
	rectCount := 0
	for y0 := 0; y0 < src.Height; y0 += tileSize {
		h := tileSize
		if y0+h > src.Height {
			h = src.Height - y0
		}
		for x0 := 0; x0 < src.Width; x0 += tileSize {
			w := tileSize
			if x0+w > src.Width {
				w = src.Width - x0
			}
			before := len(payload)
			var ok bool
			payload, ok = appendClearCodecRect(payload, src, stride, x0, y0, w, h)
			if !ok || len(payload) > rdpgfxMaxPDUSize || len(payload) >= rawBytes {
				return nil, false
			}
			if len(payload) > before {
				rectCount++
			}
		}
	}
	if rectCount == 0 || rectCount > 0xffff {
		return nil, false
	}
	binary.LittleEndian.PutUint16(payload[8:10], uint16(rectCount))
	return payload, true
}

func appendClearCodecRect(payload []byte, src frame.Frame, stride, x0, y0, w, h int) ([]byte, bool) {
	r, g, b, solid, ok := clearCodecSolidRectRGB(src, stride, x0, y0, w, h)
	if !ok {
		return nil, false
	}
	if solid {
		payload = append(payload, clearCodecOpSolidRect)
		payload = appendLE16Bytes(payload, uint16(x0))
		payload = appendLE16Bytes(payload, uint16(y0))
		payload = appendLE16Bytes(payload, uint16(w))
		payload = appendLE16Bytes(payload, uint16(h))
		payload = binary.LittleEndian.AppendUint32(payload, 0)
		return append(payload, r, g, b), true
	}
	rectLen := w * h * 2
	if rectLen > clearCodecMaxRawRectBytes {
		return nil, false
	}
	payload = append(payload, clearCodecOpRawRect)
	payload = appendLE16Bytes(payload, uint16(x0))
	payload = appendLE16Bytes(payload, uint16(y0))
	payload = appendLE16Bytes(payload, uint16(w))
	payload = appendLE16Bytes(payload, uint16(h))
	payload = binary.LittleEndian.AppendUint32(payload, uint32(rectLen))
	for y := 0; y < h; y++ {
		row := (y0 + y) * stride
		for x := 0; x < w; x++ {
			si := row + (x0+x)*4
			var r8, g8, b8 byte
			switch src.Format {
			case frame.PixelFormatRGBA8888:
				r8, g8, b8 = src.Data[si+0], src.Data[si+1], src.Data[si+2]
			case frame.PixelFormatBGRA8888:
				r8, g8, b8 = src.Data[si+2], src.Data[si+1], src.Data[si+0]
			default:
				return nil, false
			}
			rgb565 := uint16(r8>>3)<<11 | uint16(g8>>2)<<5 | uint16(b8>>3)
			payload = binary.LittleEndian.AppendUint16(payload, rgb565)
		}
	}
	return payload, true
}

func clearCodecSolidRGB(src frame.Frame, stride int) (r, g, b byte, solid bool) {
	r, g, b, solid, ok := clearCodecSolidRectRGB(src, stride, 0, 0, src.Width, src.Height)
	if !ok {
		return 0, 0, 0, false
	}
	return r, g, b, solid
}

func clearCodecSolidRectRGB(src frame.Frame, stride, x0, y0, width, height int) (r, g, b byte, solid bool, ok bool) {
	if width <= 0 || height <= 0 || x0 < 0 || y0 < 0 || x0+width > src.Width || y0+height > src.Height {
		return 0, 0, 0, false, false
	}
	firstR, firstG, firstB, ok := clearCodecRGBAt(src, stride, x0, y0)
	if !ok {
		return 0, 0, 0, false, false
	}
	for y := y0; y < y0+height; y++ {
		for x := x0; x < x0+width; x++ {
			r, g, b, ok := clearCodecRGBAt(src, stride, x, y)
			if !ok {
				return 0, 0, 0, false, false
			}
			if r != firstR || g != firstG || b != firstB {
				return firstR, firstG, firstB, false, true
			}
		}
	}
	return firstR, firstG, firstB, true, true
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
