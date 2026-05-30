package rdpserver

import "encoding/binary"

const progressiveMaxRects = 256

type progressiveRect struct {
	Left   uint16
	Top    uint16
	Right  uint16
	Bottom uint16
}

type progressivePayload struct {
	Width       int
	Height      int
	LayerCount  uint8
	Quant       uint8
	RegionRects []progressiveRect
	Data        []byte
}

func validateProgressivePayload(in progressivePayload) bool {
	if in.Width <= 0 || in.Height <= 0 || in.Width > 8192 || in.Height > 8192 {
		return false
	}
	if in.LayerCount == 0 || in.LayerCount > 8 {
		return false
	}
	if len(in.RegionRects) == 0 || len(in.RegionRects) > progressiveMaxRects {
		return false
	}
	for _, r := range in.RegionRects {
		if r.Right <= r.Left || r.Bottom <= r.Top {
			return false
		}
		if int(r.Right) > in.Width || int(r.Bottom) > in.Height {
			return false
		}
	}
	if len(in.Data) == 0 || len(in.Data) > rdpgfxMaxPDUSize {
		return false
	}
	return true
}

func buildProgressivePayload(in progressivePayload) ([]byte, bool) {
	if !validateProgressivePayload(in) {
		return nil, false
	}
	headerLen := 2 + 2 + 1 + 1 + 2
	rectsLen := len(in.RegionRects) * 8
	total := headerLen + rectsLen + 4 + len(in.Data)
	if total <= 0 || total > rdpgfxMaxPDUSize {
		return nil, false
	}
	out := make([]byte, total)
	off := 0
	binary.LittleEndian.PutUint16(out[off:off+2], uint16(in.Width))
	off += 2
	binary.LittleEndian.PutUint16(out[off:off+2], uint16(in.Height))
	off += 2
	out[off] = in.LayerCount
	off++
	out[off] = in.Quant
	off++
	binary.LittleEndian.PutUint16(out[off:off+2], uint16(len(in.RegionRects)))
	off += 2
	for _, r := range in.RegionRects {
		binary.LittleEndian.PutUint16(out[off:off+2], r.Left)
		binary.LittleEndian.PutUint16(out[off+2:off+4], r.Top)
		binary.LittleEndian.PutUint16(out[off+4:off+6], r.Right)
		binary.LittleEndian.PutUint16(out[off+6:off+8], r.Bottom)
		off += 8
	}
	binary.LittleEndian.PutUint32(out[off:off+4], uint32(len(in.Data)))
	off += 4
	copy(out[off:], in.Data)
	return out, true
}

func parseProgressivePayload(data []byte) (progressivePayload, bool) {
	var out progressivePayload
	if len(data) < 12 {
		return out, false
	}
	off := 0
	out.Width = int(binary.LittleEndian.Uint16(data[off : off+2]))
	off += 2
	out.Height = int(binary.LittleEndian.Uint16(data[off : off+2]))
	off += 2
	out.LayerCount = data[off]
	off++
	out.Quant = data[off]
	off++
	rectCount := int(binary.LittleEndian.Uint16(data[off : off+2]))
	off += 2
	if rectCount <= 0 || rectCount > progressiveMaxRects {
		return out, false
	}
	if off+rectCount*8+4 > len(data) {
		return out, false
	}
	out.RegionRects = make([]progressiveRect, rectCount)
	for i := 0; i < rectCount; i++ {
		out.RegionRects[i] = progressiveRect{
			Left:   binary.LittleEndian.Uint16(data[off : off+2]),
			Top:    binary.LittleEndian.Uint16(data[off+2 : off+4]),
			Right:  binary.LittleEndian.Uint16(data[off+4 : off+6]),
			Bottom: binary.LittleEndian.Uint16(data[off+6 : off+8]),
		}
		off += 8
	}
	dataLen := int(binary.LittleEndian.Uint32(data[off : off+4]))
	off += 4
	if dataLen <= 0 || dataLen > len(data)-off {
		return out, false
	}
	out.Data = data[off : off+dataLen]
	if !validateProgressivePayload(out) {
		return progressivePayload{}, false
	}
	return out, true
}
