package rdpserver

import "encoding/binary"

const surfaceBitsHeaderLen = 22

var emptySurfaceBitsHeader [surfaceBitsHeaderLen]byte

func buildSurfaceBitsCommand(width, height int, codecID byte, encoded []byte) ([]byte, bool) {
	if !validSurfaceBitsCommand(width, height, codecID, len(encoded)) {
		return nil, false
	}
	out := make([]byte, surfaceBitsHeaderLen+len(encoded))
	writeSurfaceBitsHeader(out[:surfaceBitsHeaderLen], width, height, codecID, len(encoded))
	copy(out[surfaceBitsHeaderLen:], encoded)
	return out, true
}

func validSurfaceBitsCommand(width, height int, codecID byte, encodedLen int) bool {
	return width > 0 && height > 0 && width <= int(^uint16(0)) && height <= int(^uint16(0)) && codecID != 0 && encodedLen > 0 && encodedLen <= rdpgfxMaxPDUSize
}

func writeSurfaceBitsHeader(out []byte, width, height int, codecID byte, encodedLen int) {
	binary.LittleEndian.PutUint16(out[0:2], surfaceCmdSetSurfaceBits)
	binary.LittleEndian.PutUint16(out[2:4], 0) // destLeft
	binary.LittleEndian.PutUint16(out[4:6], 0) // destTop
	binary.LittleEndian.PutUint16(out[6:8], uint16(width-1))
	binary.LittleEndian.PutUint16(out[8:10], uint16(height-1))
	out[10] = 32 // bpp
	out[11] = 0  // flags
	out[12] = 0  // reserved
	out[13] = codecID
	binary.LittleEndian.PutUint16(out[14:16], uint16(width))
	binary.LittleEndian.PutUint16(out[16:18], uint16(height))
	binary.LittleEndian.PutUint32(out[18:22], uint32(encodedLen)) // #nosec G115 bounded by validSurfaceBitsCommand
}
