package rdpserver

import (
	"image"
	"image/color"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func frameToRGBAImage(src frame.Frame) (*image.RGBA, bool) {
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
			var r, g, b, a byte
			if src.Format == frame.PixelFormatBGRA8888 {
				b, g, r, a = px[0], px[1], px[2], px[3]
			} else {
				r, g, b, a = px[0], px[1], px[2], px[3]
			}
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img, true
}
