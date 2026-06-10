package rdpserver

import rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"

const surfaceBitsHeaderLen = 22

var emptySurfaceBitsHeader [surfaceBitsHeaderLen]byte

func buildSurfaceBitsCommand(width, height int, codecID byte, encoded []byte) ([]byte, bool) {
	if !validSurfaceBitsCommand(width, height, codecID, len(encoded)) {
		return nil, false
	}
	out, err := rdpcodec.BuildSetSurfaceBits(rdpcodec.Rect{Right: uint16(width - 1), Bottom: uint16(height - 1)}, 32, codecID, uint16(width), uint16(height), encoded) // #nosec G115 -- validated above.
	if err != nil {
		return nil, false
	}
	return out, true
}

func validSurfaceBitsCommand(width, height int, codecID byte, encodedLen int) bool {
	return width > 0 && height > 0 && width <= int(^uint16(0)) && height <= int(^uint16(0)) && codecID != 0 && encodedLen > 0 && encodedLen <= rdpgfxMaxPDUSize
}

func writeSurfaceBitsHeader(out []byte, width, height int, codecID byte, encodedLen int) {
	cmd, ok := buildSurfaceBitsCommand(width, height, codecID, make([]byte, encodedLen))
	if !ok {
		return
	}
	copy(out, cmd[:surfaceBitsHeaderLen])
}
