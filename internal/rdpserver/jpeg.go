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
	_, _ = buf.Write(emptySurfaceBitsHeader[:])
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
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
