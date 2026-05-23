package rdpserver

import (
	"bytes"
	"image/jpeg"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

const bitmapCodecJPEGDefaultID byte = 2

func buildJPEGSurfaceBitsCommand(src frame.Frame, codecID byte, quality int) ([]byte, bool) {
	if codecID == 0 {
		codecID = bitmapCodecJPEGDefaultID
	}
	if quality <= 0 || quality > 100 {
		quality = 75
	}
	img, ok := frameToRGBAImage(src)
	if !ok {
		return nil, false
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, false
	}
	encoded := buf.Bytes()
	if len(encoded) == 0 || len(encoded) > rdpgfxMaxPDUSize {
		return nil, false
	}
	return buildSurfaceBitsCommand(src.Width, src.Height, codecID, encoded)
}
