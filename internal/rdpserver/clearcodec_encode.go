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
	bytesPerRow := src.Width * 2
	if bytesPerRow <= 0 {
		return nil, false
	}
	rowsPerRect := clearCodecMaxRawRectBytes / bytesPerRow
	if rowsPerRect <= 0 {
		rowsPerRect = 1
	}
	numRects := (src.Height + rowsPerRect - 1) / rowsPerRect
	if numRects <= 0 || numRects > 0xffff {
		return nil, false
	}
	headerLen := 4 + 2 + 2 + 2
	rectHeadersLen := numRects * (1 + 2 + 2 + 2 + 2 + 4)
	// Reserve raw data for every band plus three bytes for each band that turns
	// out to be solid. The returned payload is sliced to the actual offset.
	total := headerLen + rectHeadersLen + src.Width*src.Height*2 + numRects*3
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
	binary.LittleEndian.PutUint16(payload[off:off+2], uint16(numRects))
	off += 2

	for rect := 0; rect < numRects; rect++ {
		y0 := rect * rowsPerRect
		h := rowsPerRect
		if y0+h > src.Height {
			h = src.Height - y0
		}
		r, g, b, solid, ok := clearCodecSolidRectRGB(src, stride, 0, y0, src.Width, h)
		if !ok {
			return nil, false
		}
		if solid {
			payload[off] = clearCodecOpSolidRect
			off++
			binary.LittleEndian.PutUint16(payload[off:off+2], 0)
			off += 2
			binary.LittleEndian.PutUint16(payload[off:off+2], uint16(y0))
			off += 2
			binary.LittleEndian.PutUint16(payload[off:off+2], uint16(src.Width))
			off += 2
			binary.LittleEndian.PutUint16(payload[off:off+2], uint16(h))
			off += 2
			binary.LittleEndian.PutUint32(payload[off:off+4], 0)
			off += 4
			payload[off+0], payload[off+1], payload[off+2] = r, g, b
			off += 3
			continue
		}
		rectLen := src.Width * h * 2
		payload[off] = clearCodecOpRawRect
		off++
		binary.LittleEndian.PutUint16(payload[off:off+2], 0)
		off += 2
		binary.LittleEndian.PutUint16(payload[off:off+2], uint16(y0))
		off += 2
		binary.LittleEndian.PutUint16(payload[off:off+2], uint16(src.Width))
		off += 2
		binary.LittleEndian.PutUint16(payload[off:off+2], uint16(h))
		off += 2
		binary.LittleEndian.PutUint32(payload[off:off+4], uint32(rectLen))
		off += 4
		for y := 0; y < h; y++ {
			row := (y0 + y) * stride
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
	}
	return payload[:off], true
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
