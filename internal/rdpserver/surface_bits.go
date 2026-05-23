package rdpserver

const surfaceBitsHeaderLen = 22

func buildSurfaceBitsCommand(width, height int, codecID byte, encoded []byte) ([]byte, bool) {
	if width <= 0 || height <= 0 || width > int(^uint16(0)) || height > int(^uint16(0)) || codecID == 0 {
		return nil, false
	}
	if len(encoded) == 0 || len(encoded) > rdpgfxMaxPDUSize {
		return nil, false
	}
	out := make([]byte, 0, surfaceBitsHeaderLen+len(encoded))
	out = appendLE16Bytes(out, surfaceCmdSetSurfaceBits)
	out = appendLE16Bytes(out, 0) // destLeft
	out = appendLE16Bytes(out, 0) // destTop
	out = appendLE16Bytes(out, uint16(width-1))
	out = appendLE16Bytes(out, uint16(height-1))
	out = append(out, byte(32)) // bpp
	out = append(out, 0)        // flags
	out = append(out, 0)        // reserved
	out = append(out, codecID)
	out = appendLE16Bytes(out, uint16(width))
	out = appendLE16Bytes(out, uint16(height))
	out = appendLE32Bytes(out, uint32(len(encoded))) // #nosec G115 bounded above
	out = append(out, encoded...)
	return out, true
}
