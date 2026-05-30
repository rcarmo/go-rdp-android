package rdpserver

import "encoding/binary"

func buildRDPGFXAVC444FramePDUs(surfaceID uint16, frameID uint32, in avc444EncoderInput) ([][]byte, bool) {
	return buildRDPGFXAVC444FramePDUsForCodec(surfaceID, frameID, in, rdpgfxCodecAVC444, false)
}

func buildRDPGFXAVC444v2FramePDUs(surfaceID uint16, frameID uint32, in avc444EncoderInput) ([][]byte, bool) {
	return buildRDPGFXAVC444FramePDUsForCodec(surfaceID, frameID, in, rdpgfxCodecAVC444v2, true)
}

func buildRDPGFXAVC444FramePDUsForCodec(surfaceID uint16, frameID uint32, in avc444EncoderInput, codecID uint16, useV2 bool) ([][]byte, bool) {
	in.UseV2 = useV2
	if err := validateAVC444EncoderInput(in); err != nil {
		return nil, false
	}
	bitmapLen := avc444BitmapStreamLen(in)
	if bitmapLen <= 0 || bitmapLen > avc444MaxBitmapStreamLen {
		return nil, false
	}
	wireLen := rdpgfxHeaderLen + rdpgfxWireToSurface1PayloadHeaderLen + bitmapLen
	backing := make([]byte, 16+wireLen+12)
	start := backing[:16]
	writeRDPGFXPDUHeader(start, rdpgfxCmdStartFrame, 0)
	binary.LittleEndian.PutUint32(start[8:12], uint32(0))
	binary.LittleEndian.PutUint32(start[12:16], frameID)
	wire := backing[16 : 16+wireLen]
	writeRDPGFXWireToSurface1Header(wire, surfaceID, codecID, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(in.Width), uint16(in.Height), bitmapLen) // #nosec G115 -- validateAVC444EncoderInput bounds width/height.
	if !writeRDPGFXAVC444BitmapStream(wire[rdpgfxHeaderLen+rdpgfxWireToSurface1PayloadHeaderLen:], in) {
		return nil, false
	}
	end := backing[16+wireLen:]
	writeRDPGFXPDUHeader(end, rdpgfxCmdEndFrame, 0)
	binary.LittleEndian.PutUint32(end[8:12], frameID)
	return [][]byte{start, wire, end}, true
}
