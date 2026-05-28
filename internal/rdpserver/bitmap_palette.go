package rdpserver

const updateTypePalette = 0x0002

func buildGrayscalePaletteUpdate() []byte {
	out := appendLE16Bytes(nil, updateTypePalette)
	out = appendLE16Bytes(out, 256)
	for i := 0; i < 256; i++ {
		v := byte(i)
		out = append(out, v, v, v)
	}
	return out
}

func prependPaletteUpdateIfNeeded(updates [][]byte, bpp uint16) [][]byte {
	if bpp != bitmapBPP8 || len(updates) == 0 {
		return updates
	}
	out := make([][]byte, 0, len(updates)+1)
	out = append(out, buildGrayscalePaletteUpdate())
	out = append(out, updates...)
	return out
}
