package rdpserver

import (
	"encoding/binary"
	"hash/fnv"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

const (
	pduType2Update = 0x02

	updateTypeBitmap       = 0x0001
	bitmapBPP24            = 24
	maxInitialBitmapUpdate = 80
)

type bitmapRect struct {
	Left   uint16
	Top    uint16
	Right  uint16
	Bottom uint16
	Width  uint16
	Height uint16
	BPP    uint16
	Data   []byte
}

type bitmapTileCache struct {
	hashes      map[bitmapTileKey]uint64
	frameWidth  int
	frameHeight int
}

type bitmapTileKey struct {
	x      int
	y      int
	width  int
	height int
}

func newBitmapTileCache() *bitmapTileCache {
	return &bitmapTileCache{hashes: make(map[bitmapTileKey]uint64)}
}

func buildFrameBitmapUpdate(src frame.Frame) ([]byte, bool) {
	updates, ok := buildFrameBitmapUpdates(src)
	if !ok || len(updates) == 0 {
		return nil, false
	}
	return updates[0], true
}

func buildFrameBitmapUpdates(src frame.Frame) ([][]byte, bool) {
	return buildFrameBitmapUpdatesWithCache(src, nil, false)
}

func buildFrameBitmapUpdatesForDesktop(src frame.Frame, cache *bitmapTileCache, dirtyOnly bool, width, height int) ([][]byte, bool) {
	normalized := normalizeFrameForDesktop(src, width, height)
	return buildFrameBitmapUpdatesWithCache(normalized, cache, dirtyOnly)
}

func buildFrameBitmapUpdatesWithCache(src frame.Frame, cache *bitmapTileCache, dirtyOnly bool) ([][]byte, bool) {
	if cache != nil {
		if cache.frameWidth != 0 && (cache.frameWidth != src.Width || cache.frameHeight != src.Height) {
			clear(cache.hashes)
		}
		cache.frameWidth = src.Width
		cache.frameHeight = src.Height
	}
	if src.Width <= 0 || src.Height <= 0 || len(src.Data) == 0 {
		return nil, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return nil, false
	}
	if src.Format != frame.PixelFormatRGBA8888 && src.Format != frame.PixelFormatBGRA8888 {
		return nil, false
	}

	updates := make([][]byte, 0, ((src.Width+maxInitialBitmapUpdate-1)/maxInitialBitmapUpdate)*((src.Height+maxInitialBitmapUpdate-1)/maxInitialBitmapUpdate))
	for y := 0; y < src.Height; y += maxInitialBitmapUpdate {
		tileHeight := minInt(maxInitialBitmapUpdate, src.Height-y)
		for x := 0; x < src.Width; x += maxInitialBitmapUpdate {
			tileWidth := minInt(maxInitialBitmapUpdate, src.Width-x)
			tile, hash, ok := buildFrameBitmapTile(src, stride, x, y, tileWidth, tileHeight)
			if !ok {
				return nil, false
			}
			key := bitmapTileKey{x: x, y: y, width: tileWidth, height: tileHeight}
			if cache != nil {
				if dirtyOnly && cache.hashes[key] == hash {
					tracef("bitmap_tile_skip", "x=%d y=%d width=%d height=%d", x, y, tileWidth, tileHeight)
					continue
				}
				cache.hashes[key] = hash
			}
			update := buildBitmapUpdate([]bitmapRect{tile})
			tracef("bitmap_tile", "x=%d y=%d width=%d height=%d bytes=%d", x, y, tileWidth, tileHeight, len(update))
			updates = append(updates, update)
		}
	}
	return updates, true
}

func buildFrameBitmapTile(src frame.Frame, stride, x0, y0, width, height int) (bitmapRect, uint64, bool) {
	rowBytes := alignedBitmapRowBytes(width, bitmapBPP24)
	data := make([]byte, rowBytes*height)
	for y := 0; y < height; y++ {
		rowOffset := (y0 + y) * stride
		row := src.Data[rowOffset:]
		for x := 0; x < width; x++ {
			si := (x0 + x) * 4
			di := y*rowBytes + x*3
			if si+3 >= len(row) {
				return bitmapRect{}, 0, false
			}
			switch src.Format {
			case frame.PixelFormatBGRA8888:
				data[di+0] = row[si+0]
				data[di+1] = row[si+1]
				data[di+2] = row[si+2]
			case frame.PixelFormatRGBA8888:
				data[di+0] = row[si+2]
				data[di+1] = row[si+1]
				data[di+2] = row[si+0]
			}
		}
	}
	return bitmapRect{
		Left:   uint16(x0),
		Top:    uint16(y0),
		Right:  uint16(x0 + width - 1),
		Bottom: uint16(y0 + height - 1),
		Width:  uint16(width),
		Height: uint16(height),
		BPP:    bitmapBPP24,
		Data:   data,
	}, hashBytes(data), true
}

func alignedBitmapRowBytes(width int, bpp uint16) int {
	bits := width * int(bpp)
	return ((bits + 31) / 32) * 4
}

func hashBytes(data []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(data)
	return h.Sum64()
}

func normalizedFrameStride(src frame.Frame) (int, bool) {
	if src.Width <= 0 || src.Height <= 0 {
		return 0, false
	}
	maxInt := int(^uint(0) >> 1)
	if src.Width > maxInt/4 {
		return 0, false
	}
	minStride := src.Width * 4
	stride := src.Stride
	if stride <= 0 {
		stride = minStride
	}
	if stride < minStride || stride > maxInt/src.Height {
		return 0, false
	}
	if len(src.Data) < stride*src.Height {
		return 0, false
	}
	return stride, true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func normalizeFrameForDesktop(src frame.Frame, width, height int) frame.Frame {
	if width <= 0 || height <= 0 || src.Width <= 0 || src.Height <= 0 {
		return src
	}
	if src.Width == width && src.Height == height {
		return src
	}
	if src.Format != frame.PixelFormatRGBA8888 && src.Format != frame.PixelFormatBGRA8888 {
		return src
	}
	scaled, ok := scaleFrameNearest(src, width, height)
	if !ok {
		return src
	}
	tracef("frame_resize", "source=%dx%d target=%dx%d", src.Width, src.Height, width, height)
	return scaled
}

func scaleFrameNearest(src frame.Frame, width, height int) (frame.Frame, bool) {
	if width <= 0 || height <= 0 {
		return frame.Frame{}, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return frame.Frame{}, false
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/4 {
		return frame.Frame{}, false
	}
	dstStride := width * 4
	if dstStride > maxInt/height {
		return frame.Frame{}, false
	}
	dst := make([]byte, dstStride*height)
	for y := 0; y < height; y++ {
		sy := (y * src.Height) / height
		srcRow := sy * stride
		dstRow := y * dstStride
		for x := 0; x < width; x++ {
			sx := (x * src.Width) / width
			si := srcRow + sx*4
			di := dstRow + x*4
			copy(dst[di:di+4], src.Data[si:si+4])
		}
	}
	return frame.Frame{
		Width:     width,
		Height:    height,
		Stride:    dstStride,
		Format:    src.Format,
		Timestamp: src.Timestamp,
		Data:      dst,
	}, true
}

func buildSolidBitmapUpdate(width, height int, argb uint32) []byte {
	if width <= 0 || width > 64 {
		width = 64
	}
	if height <= 0 || height > 64 {
		height = 64
	}
	rowBytes := alignedBitmapRowBytes(width, bitmapBPP24)
	data := make([]byte, rowBytes*height)
	b := byte(argb)
	g := byte(argb >> 8)
	r := byte(argb >> 16)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			di := y*rowBytes + x*3
			data[di+0] = b
			data[di+1] = g
			data[di+2] = r
		}
	}
	return buildBitmapUpdate([]bitmapRect{{
		Left:   0,
		Top:    0,
		Right:  uint16(width - 1),
		Bottom: uint16(height - 1),
		Width:  uint16(width),
		Height: uint16(height),
		BPP:    bitmapBPP24,
		Data:   data,
	}})
}

func buildBitmapUpdate(rects []bitmapRect) []byte {
	out := appendLE16Bytes(nil, updateTypeBitmap)
	out = appendLE16Bytes(out, uint16(len(rects)))
	for _, rect := range rects {
		out = appendLE16Bytes(out, rect.Left)
		out = appendLE16Bytes(out, rect.Top)
		out = appendLE16Bytes(out, rect.Right)
		out = appendLE16Bytes(out, rect.Bottom)
		out = appendLE16Bytes(out, rect.Width)
		out = appendLE16Bytes(out, rect.Height)
		out = appendLE16Bytes(out, rect.BPP)
		out = appendLE16Bytes(out, 0) // flags: uncompressed bitmap data
		out = appendLE16Bytes(out, uint16(len(rect.Data)))
		out = append(out, rect.Data...)
	}
	return out
}

func parseBitmapUpdateHeader(payload []byte) (rectangles uint16, err error) {
	if len(payload) < 4 {
		return 0, errShortBitmapUpdate
	}
	if binary.LittleEndian.Uint16(payload[0:2]) != updateTypeBitmap {
		return 0, errNotBitmapUpdate
	}
	return binary.LittleEndian.Uint16(payload[2:4]), nil
}

type bitmapUpdateError string

func (e bitmapUpdateError) Error() string { return string(e) }

const (
	errShortBitmapUpdate bitmapUpdateError = "short bitmap update"
	errNotBitmapUpdate   bitmapUpdateError = "not a bitmap update"
)
