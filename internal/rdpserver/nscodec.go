package rdpserver

import (
	"encoding/binary"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
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
	var encoded []byte
	if src.Format == frame.PixelFormatBGRA8888 {
		encoded, ok = rdpcodec.EncodeNSCodecRawBGRA(src.Data, src.Width, src.Height, stride)
	} else {
		encoded, ok = rdpcodec.EncodeNSCodecRawRGBA(src.Data, src.Width, src.Height, stride)
	}
	if !ok || len(encoded) == 0 || len(encoded) > rdpgfxMaxPDUSize || src.Width > int(^uint16(0)) || src.Height > int(^uint16(0)) {
		return nil, false
	}
	out := make([]byte, 0, 22+len(encoded))
	out = appendLE16Bytes(out, surfaceCmdSetSurfaceBits)
	out = appendLE16Bytes(out, 0) // destLeft
	out = appendLE16Bytes(out, 0) // destTop
	out = appendLE16Bytes(out, uint16(src.Width-1))
	out = appendLE16Bytes(out, uint16(src.Height-1))
	out = append(out, byte(32)) // bpp
	out = append(out, 0)        // flags
	out = append(out, 0)        // reserved
	out = append(out, codecID)
	out = appendLE16Bytes(out, uint16(src.Width))
	out = appendLE16Bytes(out, uint16(src.Height))
	out = appendLE32Bytes(out, uint32(len(encoded))) // #nosec G115 bounded above
	out = append(out, encoded...)
	return out, true
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
