package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

type productionProgressiveEncoder struct{}

func (productionProgressiveEncoder) EncodeRDPGFX(src frame.Frame, width, height int) ([]byte, bool) {
	if src.Width <= 0 || src.Height <= 0 || src.Width > 8192 || src.Height > 8192 {
		return nil, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return nil, false
	}
	rawBytes := src.Width * src.Height * 4
	if rawBytes <= 0 {
		return nil, false
	}
	data := make([]byte, src.Width*src.Height*2)
	off := 0
	for y := 0; y < src.Height; y++ {
		row := y * stride
		for x := 0; x < src.Width; x++ {
			if row+x*4+3 >= len(src.Data) {
				return nil, false
			}
			r, g, b, ok := frameRGB(src.Data[row:], x*4, src.Format)
			if !ok {
				return nil, false
			}
			rgb565 := uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
			data[off+0] = byte(rgb565)
			data[off+1] = byte(rgb565 >> 8)
			off += 2
		}
	}
	payload, ok := buildProgressivePayload(progressivePayload{
		Width:      src.Width,
		Height:     src.Height,
		LayerCount: 1,
		Quant:      0,
		RegionRects: []progressiveRect{{
			Left:   0,
			Top:    0,
			Right:  uint16(src.Width),
			Bottom: uint16(src.Height),
		}},
		Data: data,
	})
	if !ok || len(payload) >= rawBytes {
		return nil, false
	}
	return payload, true
}
