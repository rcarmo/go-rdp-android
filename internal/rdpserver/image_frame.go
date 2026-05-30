package rdpserver

import (
	"image"

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
	if src.Format == frame.PixelFormatRGBA8888 {
		return &image.RGBA{
			Pix:    src.Data[:(src.Height-1)*stride+src.Width*4],
			Stride: stride,
			Rect:   image.Rect(0, 0, src.Width, src.Height),
		}, true
	}

	img := image.NewRGBA(image.Rect(0, 0, src.Width, src.Height))
	for y := 0; y < src.Height; y++ {
		srcRow := src.Data[y*stride:]
		dstRow := img.Pix[y*img.Stride:]
		for x := 0; x < src.Width; x++ {
			si := x * 4
			di := x * 4
			dstRow[di+0] = srcRow[si+2]
			dstRow[di+1] = srcRow[si+1]
			dstRow[di+2] = srcRow[si+0]
			dstRow[di+3] = srcRow[si+3]
		}
	}
	return img, true
}
