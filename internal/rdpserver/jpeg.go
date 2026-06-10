package rdpserver

import (
	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

const bitmapCodecJPEGDefaultID byte = 2

func buildJPEGSurfaceBitsCommand(src frame.Frame, codecID byte, quality int) ([]byte, bool) {
	if codecID == 0 {
		codecID = bitmapCodecJPEGDefaultID
	}
	if quality <= 0 || quality > 100 {
		quality = 75
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return nil, false
	}
	format, ok := planarPixelFormat(src.Format)
	if !ok {
		return nil, false
	}
	cmd, err := rdpcodec.BuildJPEGSetSurfaceBits(rdpcodec.Rect{Right: uint16(src.Width - 1), Bottom: uint16(src.Height - 1)}, codecID, rdpcodec.BitmapInput{Pixels: src.Data, Width: src.Width, Height: src.Height, Stride: stride, Format: format}, quality) // #nosec G115 -- BuildJPEGSetSurfaceBits validates bounds.
	if err != nil {
		return nil, false
	}
	return cmd, true
}

func jpegSurfaceBitsCapacityHint(width, height int) int {
	if width <= 0 || height <= 0 {
		return 4096
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/height {
		return 4096
	}
	hint := width * height / 4
	if hint < 4096 {
		return 4096
	}
	return hint
}
