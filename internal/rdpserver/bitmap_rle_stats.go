package rdpserver

import "encoding/binary"

func bitmapRLEStatsFromUpdates(updates [][]byte) (rects int64, bytes int64, savedBytes int64) {
	for _, update := range updates {
		if len(update) < 4 || binary.LittleEndian.Uint16(update[0:2]) != updateTypeBitmap {
			continue
		}
		rectCount := int(binary.LittleEndian.Uint16(update[2:4]))
		off := 4
		for i := 0; i < rectCount; i++ {
			if off+18 > len(update) {
				break
			}
			width := int(binary.LittleEndian.Uint16(update[off+8 : off+10]))
			height := int(binary.LittleEndian.Uint16(update[off+10 : off+12]))
			bpp := binary.LittleEndian.Uint16(update[off+12 : off+14])
			flags := binary.LittleEndian.Uint16(update[off+14 : off+16])
			dataLen := int(binary.LittleEndian.Uint16(update[off+16 : off+18]))
			off += 18
			if dataLen < 0 || off+dataLen > len(update) {
				break
			}
			if flags&(bitmapCompressionFlag|noBitmapCompressionHeader) == bitmapCompressionFlag|noBitmapCompressionHeader {
				rects++
				bytes += int64(dataLen)
				if _, ok := bitmapRLEBytesPerPixel(bpp); ok && width > 0 && height > 0 {
					uncompressed := int64(alignedBitmapRowBytes(width, bpp) * height)
					if uncompressed > int64(dataLen) {
						savedBytes += uncompressed - int64(dataLen)
					}
				}
			}
			off += dataLen
		}
	}
	return rects, bytes, savedBytes
}
