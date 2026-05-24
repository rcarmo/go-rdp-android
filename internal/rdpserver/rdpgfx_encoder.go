package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

type RDPGFXFrameEncoder interface {
	EncodeRDPGFX(frame.Frame, int, int) ([]byte, bool)
}

type rdpgfxFrameEncoderFunc func(frame.Frame, int, int) ([]byte, bool)

func (f rdpgfxFrameEncoderFunc) EncodeRDPGFX(src frame.Frame, width, height int) ([]byte, bool) {
	return f(src, width, height)
}

func buildRDPGFXEncodedFramePDUs(surfaceID uint16, frameID uint32, src frame.Frame, width, height int, codecID uint16, path string, encoder RDPGFXFrameEncoder) ([][]byte, string, bool) {
	if encoder == nil || codecID == 0 || path == "" {
		return nil, "", false
	}
	normalized := normalizeFrameForDesktop(src, width, height)
	if _, ok := normalizedFrameStride(normalized); !ok {
		return nil, "", false
	}
	encoded, ok := encoder.EncodeRDPGFX(normalized, width, height)
	if !ok || len(encoded) == 0 || len(encoded) > rdpgfxMaxPDUSize {
		return nil, "", false
	}
	return [][]byte{
		buildRDPGFXStartFramePDU(frameID),
		buildRDPGFXWireToSurface1PDU(surfaceID, codecID, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(normalized.Width), uint16(normalized.Height), encoded), // #nosec G115 -- dimensions bounded by normalizeFrameForDesktop/normalizedFrameStride.
		buildRDPGFXEndFramePDU(frameID),
	}, path, true
}
