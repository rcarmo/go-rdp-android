package rdpserver

func buildRDPGFXAVC444FramePDUs(surfaceID uint16, frameID uint32, in avc444EncoderInput) ([][]byte, bool) {
	payload, ok := buildRDPGFXAVC444BitmapStream(in)
	if !ok {
		return nil, false
	}
	start, end := buildRDPGFXFrameBoundaryPDUs(frameID)
	return [][]byte{
		start,
		buildRDPGFXWireToSurface1PDU(surfaceID, rdpgfxCodecAVC444, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(in.Width), uint16(in.Height), payload), // #nosec G115 -- validateAVC444EncoderInput bounds width/height.
		end,
	}, true
}

func buildRDPGFXAVC444v2FramePDUs(surfaceID uint16, frameID uint32, in avc444EncoderInput) ([][]byte, bool) {
	payload, ok := buildRDPGFXAVC444v2BitmapStream(in)
	if !ok {
		return nil, false
	}
	start, end := buildRDPGFXFrameBoundaryPDUs(frameID)
	return [][]byte{
		start,
		buildRDPGFXWireToSurface1PDU(surfaceID, rdpgfxCodecAVC444v2, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(in.Width), uint16(in.Height), payload), // #nosec G115 -- validateAVC444EncoderInput bounds width/height.
		end,
	}, true
}
