package rdpserver

import (
	"encoding/binary"
	"os"
	"strings"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

const (
	pduType2Update = 0x02

	updateTypeBitmap       = rdpcodec.UpdateTypeBitmap
	bitmapBPP8             = rdpcodec.BitmapBPP8
	bitmapBPP15            = rdpcodec.BitmapBPP15
	bitmapBPP16            = rdpcodec.BitmapBPP16
	bitmapBPP24            = rdpcodec.BitmapBPP24
	maxInitialBitmapUpdate = 80

	fnv64Offset = 14695981039346656037
	fnv64Prime  = 1099511628211
)

type bitmapRect struct {
	Left   uint16
	Top    uint16
	Right  uint16
	Bottom uint16
	Width  uint16
	Height uint16
	BPP    uint16
	Flags  uint16
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
	return buildFrameBitmapUpdatesForDesktopBPP(src, cache, dirtyOnly, width, height, bitmapBPP24)
}

func buildFrameBitmapUpdatesForDesktopBPP(src frame.Frame, cache *bitmapTileCache, dirtyOnly bool, width, height int, bpp uint16) ([][]byte, bool) {
	normalized := normalizeFrameForDesktop(src, width, height)
	return buildFrameBitmapUpdatesWithCacheBPP(normalized, cache, dirtyOnly, bpp)
}

func buildFrameBitmapUpdatesWithCache(src frame.Frame, cache *bitmapTileCache, dirtyOnly bool) ([][]byte, bool) {
	return buildFrameBitmapUpdatesWithCacheBPP(src, cache, dirtyOnly, bitmapBPP24)
}

func buildFrameBitmapUpdatesWithCacheBPP(src frame.Frame, cache *bitmapTileCache, dirtyOnly bool, bpp uint16) ([][]byte, bool) {
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

	rleEnabled := bitmapRLEEnabledFromEnv()
	if !rleEnabled {
		if cache == nil {
			return buildFrameBitmapUpdatesNoCacheBPP(src, stride, bpp)
		}
		if !dirtyOnly {
			return buildFrameBitmapUpdatesInitialCacheBPP(src, stride, cache, bpp)
		}
	}
	updates := make([][]byte, 0, ((src.Width+maxInitialBitmapUpdate-1)/maxInitialBitmapUpdate)*((src.Height+maxInitialBitmapUpdate-1)/maxInitialBitmapUpdate))
	for y := 0; y < src.Height; y += maxInitialBitmapUpdate {
		tileHeight := minInt(maxInitialBitmapUpdate, src.Height-y)
		for x := 0; x < src.Width; x += maxInitialBitmapUpdate {
			tileWidth := minInt(maxInitialBitmapUpdate, src.Width-x)
			var update []byte
			var hash uint64
			if rleEnabled {
				tile, tileHash, ok := buildFrameBitmapTileForBPP(src, stride, x, y, tileWidth, tileHeight, bpp)
				if !ok {
					return nil, false
				}
				hash = tileHash
				update = buildBitmapUpdateSingle(tile)
				if compressed, ok := buildCompressedBitmapRLEUpdateSingle(tile); ok && len(compressed) < len(update) {
					if traceEnabled {
						tracef("bitmap_rle_tile", "x=%d y=%d width=%d height=%d bytes=%d uncompressed_bytes=%d", x, y, tileWidth, tileHeight, len(compressed), len(update))
					}
					update = compressed
				}
			} else {
				var ok bool
				update, hash, ok = buildFrameBitmapTileUpdateForBPP(src, stride, x, y, tileWidth, tileHeight, bpp)
				if !ok {
					return nil, false
				}
			}
			key := bitmapTileKey{x: x, y: y, width: tileWidth, height: tileHeight}
			if cache != nil {
				if dirtyOnly && cache.hashes[key] == hash {
					if traceEnabled {
						tracef("bitmap_tile_skip", "x=%d y=%d width=%d height=%d", x, y, tileWidth, tileHeight)
					}
					continue
				}
				cache.hashes[key] = hash
			}
			if traceEnabled {
				tracef("bitmap_tile", "x=%d y=%d width=%d height=%d bpp=%d bytes=%d", x, y, tileWidth, tileHeight, bpp, len(update))
			}
			updates = append(updates, update)
		}
	}
	return updates, true
}

func buildFrameBitmapUpdatesInitialCacheBPP(src frame.Frame, stride int, cache *bitmapTileCache, bpp uint16) ([][]byte, bool) {
	bytesPerPixel, ok := rawBitmapBytesPerPixel(bpp)
	if !ok {
		return nil, false
	}
	tilesX := (src.Width + maxInitialBitmapUpdate - 1) / maxInitialBitmapUpdate
	tilesY := (src.Height + maxInitialBitmapUpdate - 1) / maxInitialBitmapUpdate
	updates := make([][]byte, 0, tilesX*tilesY)
	totalBytes := 0
	for y := 0; y < src.Height; y += maxInitialBitmapUpdate {
		tileHeight := minInt(maxInitialBitmapUpdate, src.Height-y)
		for x := 0; x < src.Width; x += maxInitialBitmapUpdate {
			tileWidth := minInt(maxInitialBitmapUpdate, src.Width-x)
			totalBytes += 4 + 18 + alignedBitmapRowBytes(tileWidth, bpp)*tileHeight
		}
	}
	buf := make([]byte, 0, totalBytes)
	for y := 0; y < src.Height; y += maxInitialBitmapUpdate {
		tileHeight := minInt(maxInitialBitmapUpdate, src.Height-y)
		for x := 0; x < src.Width; x += maxInitialBitmapUpdate {
			tileWidth := minInt(maxInitialBitmapUpdate, src.Width-x)
			start := len(buf)
			var ok bool
			buf, ok = appendFrameBitmapTileUpdateForBPP(buf, src, stride, x, y, tileWidth, tileHeight, bpp, bytesPerPixel)
			if !ok {
				return nil, false
			}
			update := buf[start:len(buf):len(buf)]
			dataOffset := start + 4 + 18
			cache.hashes[bitmapTileKey{x: x, y: y, width: tileWidth, height: tileHeight}] = hashBytes(buf[dataOffset:len(buf)])
			if traceEnabled {
				tracef("bitmap_tile", "x=%d y=%d width=%d height=%d bpp=%d bytes=%d", x, y, tileWidth, tileHeight, bpp, len(update))
			}
			updates = append(updates, update)
		}
	}
	return updates, true
}

func buildFrameBitmapUpdatesNoCacheBPP(src frame.Frame, stride int, bpp uint16) ([][]byte, bool) {
	bytesPerPixel, ok := rawBitmapBytesPerPixel(bpp)
	if !ok {
		return nil, false
	}
	tilesX := (src.Width + maxInitialBitmapUpdate - 1) / maxInitialBitmapUpdate
	tilesY := (src.Height + maxInitialBitmapUpdate - 1) / maxInitialBitmapUpdate
	updates := make([][]byte, 0, tilesX*tilesY)
	totalBytes := 0
	for y := 0; y < src.Height; y += maxInitialBitmapUpdate {
		tileHeight := minInt(maxInitialBitmapUpdate, src.Height-y)
		for x := 0; x < src.Width; x += maxInitialBitmapUpdate {
			tileWidth := minInt(maxInitialBitmapUpdate, src.Width-x)
			totalBytes += 4 + 18 + alignedBitmapRowBytes(tileWidth, bpp)*tileHeight
		}
	}
	buf := make([]byte, 0, totalBytes)
	for y := 0; y < src.Height; y += maxInitialBitmapUpdate {
		tileHeight := minInt(maxInitialBitmapUpdate, src.Height-y)
		for x := 0; x < src.Width; x += maxInitialBitmapUpdate {
			tileWidth := minInt(maxInitialBitmapUpdate, src.Width-x)
			start := len(buf)
			var ok bool
			buf, ok = appendFrameBitmapTileUpdateForBPP(buf, src, stride, x, y, tileWidth, tileHeight, bpp, bytesPerPixel)
			if !ok {
				return nil, false
			}
			update := buf[start:len(buf):len(buf)]
			if traceEnabled {
				tracef("bitmap_tile", "x=%d y=%d width=%d height=%d bpp=%d bytes=%d", x, y, tileWidth, tileHeight, bpp, len(update))
			}
			updates = append(updates, update)
		}
	}
	return updates, true
}

func appendFrameBitmapTileUpdateForBPP(out []byte, src frame.Frame, stride, x0, y0, width, height int, bpp uint16, bytesPerPixel int) ([]byte, bool) {
	rowBytes := alignedBitmapRowBytes(width, bpp)
	rect := bitmapRect{Left: uint16(x0), Top: uint16(y0), Right: uint16(x0 + width - 1), Bottom: uint16(y0 + height - 1), Width: uint16(width), Height: uint16(height), BPP: bpp}
	out = appendLE16Bytes(out, updateTypeBitmap)
	out = appendLE16Bytes(out, 1)
	out = appendBitmapRectHeader(out, rect, 0, rowBytes*height)
	for y := 0; y < height; y++ {
		rowOffset := (y0 + y) * stride
		row := src.Data[rowOffset:]
		dst := out[len(out) : len(out)+rowBytes]
		out = out[:len(out)+rowBytes]
		for x := 0; x < width; x++ {
			si := (x0 + x) * 4
			di := x * bytesPerPixel
			if si+3 >= len(row) {
				return nil, false
			}
			r, g, b, ok := frameRGB(row, si, src.Format)
			if !ok {
				return nil, false
			}
			writeRawBitmapPixel(dst, di, bpp, r, g, b)
		}
	}
	return out, true
}

func buildFrameBitmapTile(src frame.Frame, stride, x0, y0, width, height int) (bitmapRect, uint64, bool) {
	return buildFrameBitmapTileForBPP(src, stride, x0, y0, width, height, bitmapBPP24)
}

func buildFrameBitmapTileForBPP(src frame.Frame, stride, x0, y0, width, height int, bpp uint16) (bitmapRect, uint64, bool) {
	if width <= 0 || height <= 0 {
		return bitmapRect{}, 0, false
	}
	bytesPerPixel, ok := rawBitmapBytesPerPixel(bpp)
	if !ok {
		return bitmapRect{}, 0, false
	}
	rowBytes := alignedBitmapRowBytes(width, bpp)
	data := make([]byte, rowBytes*height)
	for y := 0; y < height; y++ {
		rowOffset := (y0 + y) * stride
		row := src.Data[rowOffset:]
		for x := 0; x < width; x++ {
			si := (x0 + x) * 4
			di := y*rowBytes + x*bytesPerPixel
			if si+3 >= len(row) {
				return bitmapRect{}, 0, false
			}
			r, g, b, ok := frameRGB(row, si, src.Format)
			if !ok {
				return bitmapRect{}, 0, false
			}
			writeRawBitmapPixel(data, di, bpp, r, g, b)
		}
	}
	return bitmapRect{
		Left:   uint16(x0),
		Top:    uint16(y0),
		Right:  uint16(x0 + width - 1),
		Bottom: uint16(y0 + height - 1),
		Width:  uint16(width),
		Height: uint16(height),
		BPP:    bpp,
		Data:   data,
	}, hashBytes(data), true
}

func buildFrameBitmapTileUpdateForBPP(src frame.Frame, stride, x0, y0, width, height int, bpp uint16) ([]byte, uint64, bool) {
	if width <= 0 || height <= 0 {
		return nil, 0, false
	}
	bytesPerPixel, ok := rawBitmapBytesPerPixel(bpp)
	if !ok {
		return nil, 0, false
	}
	rowBytes := alignedBitmapRowBytes(width, bpp)
	rect := bitmapRect{
		Left:   uint16(x0),
		Top:    uint16(y0),
		Right:  uint16(x0 + width - 1),
		Bottom: uint16(y0 + height - 1),
		Width:  uint16(width),
		Height: uint16(height),
		BPP:    bpp,
	}
	data := make([]byte, rowBytes*height)
	hash := uint64(fnv64Offset)
	for y := 0; y < height; y++ {
		rowOffset := (y0 + y) * stride
		row := src.Data[rowOffset:]
		dst := data[y*rowBytes : y*rowBytes+rowBytes]
		for x := 0; x < width; x++ {
			si := (x0 + x) * 4
			di := x * bytesPerPixel
			if si+3 >= len(row) {
				return nil, 0, false
			}
			r, g, b, ok := frameRGB(row, si, src.Format)
			if !ok {
				return nil, 0, false
			}
			writeRawBitmapPixel(dst, di, bpp, r, g, b)
		}
		for _, b := range dst {
			hash ^= uint64(b)
			hash *= fnv64Prime
		}
	}
	rect.Data = data
	out, err := rdpcodec.BuildBitmapUpdate(upstreamBitmapRects([]bitmapRect{rect}))
	if err != nil {
		return nil, 0, false
	}
	return out, hash, true
}

func rawBitmapBytesPerPixel(bpp uint16) (int, bool) {
	return rdpcodec.BitmapBytesPerPixel(bpp)
}

func frameRGB(row []byte, offset int, format frame.PixelFormat) (r, g, b byte, ok bool) {
	switch format {
	case frame.PixelFormatBGRA8888:
		return row[offset+2], row[offset+1], row[offset+0], true
	case frame.PixelFormatRGBA8888:
		return row[offset+0], row[offset+1], row[offset+2], true
	default:
		return 0, 0, 0, false
	}
}

func writeRawBitmapPixel(data []byte, offset int, bpp uint16, r, g, b byte) {
	switch bpp {
	case bitmapBPP8:
		data[offset] = rgbToGray8(r, g, b)
	case bitmapBPP15:
		v := uint16(r>>3)<<10 | uint16(g>>3)<<5 | uint16(b>>3)
		binary.LittleEndian.PutUint16(data[offset:offset+2], v)
	case bitmapBPP16:
		v := uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
		binary.LittleEndian.PutUint16(data[offset:offset+2], v)
	case bitmapBPP24:
		data[offset+0] = b
		data[offset+1] = g
		data[offset+2] = r
	}
}

func rgbToGray8(r, g, b byte) byte {
	return byte((uint16(r)*30 + uint16(g)*59 + uint16(b)*11) / 100)
}

func bitmapRLEEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_BITMAP_RLE"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func alignedBitmapRowBytes(width int, bpp uint16) int {
	if bpp == 32 {
		return width * 4
	}
	rowBytes, ok := rdpcodec.AlignedBitmapRowBytes(width, bpp)
	if !ok {
		return 0
	}
	return rowBytes
}

func hashBytes(data []byte) uint64 {
	h := uint64(fnv64Offset)
	for _, b := range data {
		h ^= uint64(b)
		h *= fnv64Prime
	}
	return h
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
	if stride < minStride {
		return 0, false
	}
	required := minStride
	if src.Height > 1 {
		if stride > (maxInt-minStride)/(src.Height-1) {
			return 0, false
		}
		required += stride * (src.Height - 1)
	}
	if len(src.Data) < required {
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
	if traceEnabled {
		tracef("frame_resize", "source=%dx%d target=%dx%d", src.Width, src.Height, width, height)
	}
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
	return buildSolidBitmapUpdateBPP(width, height, argb, bitmapBPP24)
}

func buildSolidBitmapUpdateBPP(width, height int, argb uint32, bpp uint16) []byte {
	rect := buildSolidBitmapRectForBPP(width, height, argb, bpp)
	if bitmapRLEEnabledFromEnv() {
		if compressed, ok := buildCompressedBitmapRLEUpdate([]bitmapRect{rect}); ok {
			if traceEnabled {
				tracef("bitmap_rle_solid", "width=%d height=%d bytes=%d uncompressed_bytes=%d", rect.Width, rect.Height, len(compressed), len(buildBitmapUpdate([]bitmapRect{rect})))
			}
			return compressed
		}
	}
	if traceEnabled {
		tracef("bitmap_tile", "x=0 y=0 width=%d height=%d bpp=%d bytes=%d", rect.Width, rect.Height, rect.BPP, len(rect.Data)+22)
	}
	return buildBitmapUpdate([]bitmapRect{rect})
}

func buildSolidBitmapRect(width, height int, argb uint32) bitmapRect {
	return buildSolidBitmapRectForBPP(width, height, argb, bitmapBPP24)
}

func buildSolidBitmapRectForBPP(width, height int, argb uint32, bpp uint16) bitmapRect {
	if width <= 0 || width > 64 {
		width = 64
	}
	if height <= 0 || height > 64 {
		height = 64
	}
	if _, ok := rawBitmapBytesPerPixel(bpp); !ok {
		bpp = bitmapBPP24
	}
	rowBytes := alignedBitmapRowBytes(width, bpp)
	data := make([]byte, rowBytes*height)
	b := byte(argb)
	g := byte(argb >> 8)
	r := byte(argb >> 16)
	bytesPerPixel, _ := rawBitmapBytesPerPixel(bpp)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			di := y*rowBytes + x*bytesPerPixel
			writeRawBitmapPixel(data, di, bpp, r, g, b)
		}
	}
	return bitmapRect{Left: 0, Top: 0, Right: uint16(width - 1), Bottom: uint16(height - 1), Width: uint16(width), Height: uint16(height), BPP: bpp, Data: data}
}

func buildBitmapUpdate(rects []bitmapRect) []byte {
	out, err := rdpcodec.BuildBitmapUpdate(upstreamBitmapRects(rects))
	if err != nil {
		return nil
	}
	return out
}

func buildBitmapUpdateSingle(rect bitmapRect) []byte {
	return buildBitmapUpdate([]bitmapRect{rect})
}

func buildCompressedBitmapRLEUpdate(rects []bitmapRect) ([]byte, bool) {
	out, err := rdpcodec.BuildCompressedBitmapRLEUpdate(upstreamBitmapRects(rects))
	if err != nil {
		return nil, false
	}
	return out, true
}

func buildCompressedBitmapRLEUpdateSingle(rect bitmapRect) ([]byte, bool) {
	return buildCompressedBitmapRLEUpdate([]bitmapRect{rect})
}

func upstreamBitmapRects(rects []bitmapRect) []rdpcodec.BitmapUpdateRect {
	out := make([]rdpcodec.BitmapUpdateRect, 0, len(rects))
	for _, rect := range rects {
		if rect.Right < rect.Left && rect.Width > 0 {
			rect.Right = rect.Left + rect.Width - 1
		}
		if rect.Bottom < rect.Top && rect.Height > 0 {
			rect.Bottom = rect.Top + rect.Height - 1
		}
		// Preserve older Android-local shorthand tests that set Width/Height only.
		if rect.Left == 0 && rect.Top == 0 && rect.Right == 0 && rect.Bottom == 0 {
			if rect.Width > 0 {
				rect.Right = rect.Width - 1
			}
			if rect.Height > 0 {
				rect.Bottom = rect.Height - 1
			}
		}
		out = append(out, rdpcodec.BitmapUpdateRect{
			Left:         rect.Left,
			Top:          rect.Top,
			Right:        rect.Right,
			Bottom:       rect.Bottom,
			Width:        rect.Width,
			Height:       rect.Height,
			BitsPerPixel: rect.BPP,
			Flags:        rect.Flags,
			Data:         rect.Data,
		})
	}
	return out
}

func appendBitmapRect(out []byte, rect bitmapRect, flags uint16, data []byte) []byte {
	out = appendBitmapRectHeader(out, rect, flags, len(data))
	out = append(out, data...)
	return out
}

func appendBitmapRectHeader(out []byte, rect bitmapRect, flags uint16, dataLen int) []byte {
	out = appendLE16Bytes(out, rect.Left)
	out = appendLE16Bytes(out, rect.Top)
	out = appendLE16Bytes(out, rect.Right)
	out = appendLE16Bytes(out, rect.Bottom)
	out = appendLE16Bytes(out, rect.Width)
	out = appendLE16Bytes(out, rect.Height)
	out = appendLE16Bytes(out, rect.BPP)
	out = appendLE16Bytes(out, flags)
	out = appendLE16Bytes(out, uint16(dataLen))
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
