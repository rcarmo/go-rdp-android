package rdpserver

import (
	"bytes"
	"image"
	"image/color"
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
	if src.Width <= 0 || src.Height <= 0 || src.Format != frame.PixelFormatBGRA8888 && src.Format != frame.PixelFormatRGBA8888 {
		return nil, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok || src.Width > int(^uint16(0)) || src.Height > int(^uint16(0)) {
		return nil, false
	}
	img := image.NewRGBA(image.Rect(0, 0, src.Width, src.Height))
	for y := 0; y < src.Height; y++ {
		row := src.Data[y*stride:]
		for x := 0; x < src.Width; x++ {
			px := row[x*4:]
			var r, g, b byte
			if src.Format == frame.PixelFormatBGRA8888 {
				b, g, r = px[0], px[1], px[2]
			} else {
				r, g, b = px[0], px[1], px[2]
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 0xff})
		}
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
