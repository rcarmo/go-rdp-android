package rdpserver

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func buildPNGSurfaceBitsCommand(src frame.Frame, codecID byte) ([]byte, bool) {
	if codecID == 0 || src.Width <= 0 || src.Height <= 0 || src.Format != frame.PixelFormatBGRA8888 && src.Format != frame.PixelFormatRGBA8888 {
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
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, false
	}
	encoded := buf.Bytes()
	if len(encoded) == 0 || len(encoded) > rdpgfxMaxPDUSize {
		return nil, false
	}
	out := make([]byte, 0, 22+len(encoded))
	out = appendLE16Bytes(out, surfaceCmdSetSurfaceBits)
	out = appendLE16Bytes(out, 0)
	out = appendLE16Bytes(out, 0)
	out = appendLE16Bytes(out, uint16(src.Width-1))
	out = appendLE16Bytes(out, uint16(src.Height-1))
	out = append(out, byte(32), 0, 0, codecID)
	out = appendLE16Bytes(out, uint16(src.Width))
	out = appendLE16Bytes(out, uint16(src.Height))
	out = appendLE32Bytes(out, uint32(len(encoded))) // #nosec G115 bounded above
	out = append(out, encoded...)
	return out, true
}
