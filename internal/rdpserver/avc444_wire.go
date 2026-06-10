package rdpserver

import rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"

func buildRDPGFXAVC444FramePDUs(surfaceID uint16, frameID uint32, in avc444EncoderInput) ([][]byte, bool) {
	return buildRDPGFXAVC444FramePDUsForCodec(surfaceID, frameID, in, false)
}

func buildRDPGFXAVC444v2FramePDUs(surfaceID uint16, frameID uint32, in avc444EncoderInput) ([][]byte, bool) {
	return buildRDPGFXAVC444FramePDUsForCodec(surfaceID, frameID, in, true)
}

func buildRDPGFXAVC444FramePDUsForCodec(surfaceID uint16, frameID uint32, in avc444EncoderInput, useV2 bool) ([][]byte, bool) {
	in.UseV2 = useV2
	if err := validateAVC444EncoderInput(in); err != nil {
		return nil, false
	}
	start, err := rdpcodec.BuildRDPGFXStartFrame(frameID)
	if err != nil {
		return nil, false
	}
	wire, err := rdpcodec.BuildAVC444WireToSurface(surfaceID, rdpgfxPixelFormatXRGB8888, rdpcodec.Rect{Right: uint16(in.Width - 1), Bottom: uint16(in.Height - 1)}, upstreamAVC444Input(in)) // #nosec G115 -- validateAVC444EncoderInput bounds width/height.
	if err != nil {
		return nil, false
	}
	end, err := rdpcodec.BuildRDPGFXEndFrame(frameID)
	if err != nil {
		return nil, false
	}
	return [][]byte{start, wire, end}, true
}
