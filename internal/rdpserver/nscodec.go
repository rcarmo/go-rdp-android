package rdpserver

import (
	"encoding/binary"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

const (
	surfaceCmdSetSurfaceBits    uint16 = 0x0001
	bitmapCodecNSCodecDefaultID byte   = 1
)

func buildNSCodecSurfaceBitsCommand(src frame.Frame, codecID byte) ([]byte, bool) {
	if codecID == 0 {
		codecID = bitmapCodecNSCodecDefaultID
	}
	if src.Width <= 0 || src.Height <= 0 || src.Format != frame.PixelFormatBGRA8888 && src.Format != frame.PixelFormatRGBA8888 {
		return nil, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return nil, false
	}
	return buildNSCodecSurfaceBitsCommandRaw(src, stride, codecID)
}

func buildNSCodecSurfaceBitsCommandRaw(src frame.Frame, stride int, codecID byte) ([]byte, bool) {
	if src.Width > int(^uint16(0)) || src.Height > int(^uint16(0)) {
		return nil, false
	}
	planeSize := src.Width * src.Height
	encodedLen := 20 + 3*planeSize
	if !validSurfaceBitsCommand(src.Width, src.Height, codecID, encodedLen) {
		return nil, false
	}
	out := make([]byte, surfaceBitsHeaderLen+encodedLen)
	writeSurfaceBitsHeader(out[:surfaceBitsHeaderLen], src.Width, src.Height, codecID, encodedLen)
	encoded := out[surfaceBitsHeaderLen:]
	binary.LittleEndian.PutUint32(encoded[0:4], uint32(planeSize))
	binary.LittleEndian.PutUint32(encoded[4:8], uint32(planeSize))
	binary.LittleEndian.PutUint32(encoded[8:12], uint32(planeSize))
	binary.LittleEndian.PutUint32(encoded[12:16], 0)
	encoded[16] = 1 // colorLossLevel: lossless plane values.
	encoded[17] = 0 // no chroma subsampling.
	luma := encoded[20 : 20+planeSize]
	orange := encoded[20+planeSize : 20+2*planeSize]
	green := encoded[20+2*planeSize : 20+3*planeSize]
	idx := 0
	for y := 0; y < src.Height; y++ {
		row := src.Data[y*stride:]
		for x := 0; x < src.Width; x++ {
			si := x * 4
			var r, g, b int
			if src.Format == frame.PixelFormatBGRA8888 {
				b = int(row[si+0])
				g = int(row[si+1])
				r = int(row[si+2])
			} else {
				r = int(row[si+0])
				g = int(row[si+1])
				b = int(row[si+2])
			}
			co := (r - b) / 2
			t := b + co
			cg := g - t
			yv := t + cg/2
			luma[idx] = clampByteNS(yv)
			orange[idx] = clampByteNS(co + 128)
			green[idx] = clampByteNS(cg + 128)
			idx++
		}
	}
	return out, true
}

func clampByteNS(v int) byte {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

func parseSurfaceBitsCommandHeaderForTest(data []byte) (cmd uint16, codecID byte, width uint16, height uint16, bitmapLen uint32, ok bool) {
	if len(data) < 22 {
		return 0, 0, 0, 0, 0, false
	}
	cmd = binary.LittleEndian.Uint16(data[0:2])
	codecID = data[13]
	width = binary.LittleEndian.Uint16(data[14:16])
	height = binary.LittleEndian.Uint16(data[16:18])
	bitmapLen = binary.LittleEndian.Uint32(data[18:22])
	return cmd, codecID, width, height, bitmapLen, int(bitmapLen) <= len(data)-22
}
