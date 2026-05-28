package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

// buildClassicBitmapPlanarUpdate builds a slow-path Bitmap Update rectangle using
// RDP6 Planar bitmap compression (BITMAP_COMPRESSION plus
// NO_BITMAP_COMPRESSION_HDR on a 32-bpp rectangle). This is the classic bitmap
// update codec decoded by go-rdp's ProcessBitmap(..., bpp=32, compressed=true,
// noHdr=true); it is intentionally separate from RDPGFX Planar WireToSurface.
func buildClassicBitmapPlanarUpdate(src frame.Frame) ([]byte, bool) {
	if src.Width <= 0 || src.Height <= 0 || src.Width > 0xffff || src.Height > 0xffff {
		return nil, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return nil, false
	}
	bottomUp, ok := bottomUpFrameForClassicPlanar(src, stride)
	if !ok {
		return nil, false
	}
	payload, ok := buildPlanarRLEPayload(bottomUp, bottomUp.Stride)
	if !ok || len(payload) == 0 || len(payload) > int(^uint16(0)) {
		return nil, false
	}
	rawBytes := alignedBitmapRowBytes(src.Width, 32) * src.Height
	if len(payload) >= rawBytes {
		return nil, false
	}
	rect := bitmapRect{Left: 0, Top: 0, Right: uint16(src.Width - 1), Bottom: uint16(src.Height - 1), Width: uint16(src.Width), Height: uint16(src.Height), BPP: 32}
	out := appendLE16Bytes(nil, updateTypeBitmap)
	out = appendLE16Bytes(out, 1)
	out = appendBitmapRect(out, rect, bitmapCompressionFlag|noBitmapCompressionHeader, payload)
	return out, true
}

func bottomUpFrameForClassicPlanar(src frame.Frame, stride int) (frame.Frame, bool) {
	if src.Format != frame.PixelFormatRGBA8888 && src.Format != frame.PixelFormatBGRA8888 {
		return frame.Frame{}, false
	}
	outStride := src.Width * 4
	data := make([]byte, outStride*src.Height)
	for y := 0; y < src.Height; y++ {
		srcRow := (src.Height - 1 - y) * stride
		dstRow := y * outStride
		copy(data[dstRow:dstRow+outStride], src.Data[srcRow:srcRow+outStride])
	}
	return frame.Frame{Width: src.Width, Height: src.Height, Stride: outStride, Format: src.Format, Timestamp: src.Timestamp, Data: data}, true
}
