package rdpserver

import "encoding/binary"

const avc444MaxBitmapStreamLen = rdpgfxMaxPDUSize

func buildRDPGFXAVC444BitmapStream(in avc444EncoderInput) ([]byte, bool) {
	if err := validateAVC444EncoderInput(in); err != nil {
		return nil, false
	}
	total := avc444BitmapStreamLen(in)
	if total <= 0 || total > avc444MaxBitmapStreamLen {
		return nil, false
	}
	out := make([]byte, total)
	if !writeRDPGFXAVC444BitmapStream(out, in) {
		return nil, false
	}
	return out, true
}

func avc444BitmapStreamLen(in avc444EncoderInput) int {
	rectsLen := len(in.RegionRects) * 8
	headLen := 4 + rectsLen + 4 + 4 + 2 + 2
	return headLen + len(in.BaseLayer.Data) + len(in.AuxLayer.Data)
}

func writeRDPGFXAVC444BitmapStream(out []byte, in avc444EncoderInput) bool {
	total := avc444BitmapStreamLen(in)
	if total <= 0 || total > len(out) {
		return false
	}
	off := 0
	binary.LittleEndian.PutUint32(out[off:off+4], uint32(len(in.RegionRects)))
	off += 4
	for _, r := range in.RegionRects {
		binary.LittleEndian.PutUint16(out[off:off+2], r.Left)
		binary.LittleEndian.PutUint16(out[off+2:off+4], r.Top)
		binary.LittleEndian.PutUint16(out[off+4:off+6], r.Right)
		binary.LittleEndian.PutUint16(out[off+6:off+8], r.Bottom)
		off += 8
	}
	binary.LittleEndian.PutUint32(out[off:off+4], uint32(len(in.BaseLayer.Data)))
	off += 4
	binary.LittleEndian.PutUint32(out[off:off+4], uint32(len(in.AuxLayer.Data)))
	off += 4
	var flags uint16
	if in.UseV2 {
		flags = 1
	}
	binary.LittleEndian.PutUint16(out[off:off+2], flags)
	off += 2
	binary.LittleEndian.PutUint16(out[off:off+2], 0)
	off += 2
	off += copy(out[off:], in.BaseLayer.Data)
	copy(out[off:], in.AuxLayer.Data)
	return true
}

func buildRDPGFXAVC444v2BitmapStream(in avc444EncoderInput) ([]byte, bool) {
	in.UseV2 = true
	return buildRDPGFXAVC444BitmapStream(in)
}
