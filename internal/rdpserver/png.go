package rdpserver

import (
	"bytes"
	"image/png"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func buildPNGSurfaceBitsCommand(src frame.Frame, codecID byte) ([]byte, bool) {
	if codecID == 0 {
		return nil, false
	}
	img, ok := frameToRGBAImage(src)
	if !ok {
		return nil, false
	}
	var buf bytes.Buffer
	_, _ = buf.Write(emptySurfaceBitsHeader[:])
	encoder := png.Encoder{CompressionLevel: pngCompressionLevelFromEnv()}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, false
	}
	out := buf.Bytes()
	encodedLen := len(out) - surfaceBitsHeaderLen
	if !validSurfaceBitsCommand(src.Width, src.Height, codecID, encodedLen) {
		return nil, false
	}
	writeSurfaceBitsHeader(out[:surfaceBitsHeaderLen], src.Width, src.Height, codecID, encodedLen)
	return out, true
}
