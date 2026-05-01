package rdpserver

import (
	"encoding/binary"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

const (
	pduType2Update = 0x02

	updateTypeBitmap       = 0x0001
	bitmapBPP32            = 32
	maxInitialBitmapUpdate = 96
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

func buildFrameBitmapUpdate(src frame.Frame) ([]byte, bool) {
	updates, ok := buildFrameBitmapUpdates(src)
	if !ok || len(updates) == 0 {
		return nil, false
	}
	return updates[0], true
}

func buildFrameBitmapUpdates(src frame.Frame) ([][]byte, bool) {
	if src.Width <= 0 || src.Height <= 0 || len(src.Data) == 0 {
		return nil, false
	}
	stride := src.Stride
	if stride <= 0 {
		stride = src.Width * 4
	}
	if len(src.Data) < stride*src.Height {
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
			update, ok := buildFrameBitmapTile(src, stride, x, y, tileWidth, tileHeight)
			if !ok {
				return nil, false
			}
			updates = append(updates, update)
		}
	}
	return updates, len(updates) > 0
}

func buildFrameBitmapTile(src frame.Frame, stride, x0, y0, width, height int) ([]byte, bool) {
	data := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		rowOffset := (y0 + y) * stride
		row := src.Data[rowOffset:]
		for x := 0; x < width; x++ {
			si := (x0 + x) * 4
			di := (y*width + x) * 4
			if si+3 >= len(row) {
				return nil, false
			}
			switch src.Format {
			case frame.PixelFormatBGRA8888:
				copy(data[di:di+4], row[si:si+4])
			case frame.PixelFormatRGBA8888:
				data[di+0] = row[si+2]
				data[di+1] = row[si+1]
				data[di+2] = row[si+0]
				data[di+3] = row[si+3]
			}
		}
	}
	return buildBitmapUpdate([]bitmapRect{{
		Left:   uint16(x0),
		Top:    uint16(y0),
		Right:  uint16(x0 + width - 1),
		Bottom: uint16(y0 + height - 1),
		Width:  uint16(width),
		Height: uint16(height),
		BPP:    bitmapBPP32,
		Data:   data,
	}}), true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func buildSolidBitmapUpdate(width, height int, argb uint32) []byte {
	if width <= 0 || width > 64 {
		width = 64
	}
	if height <= 0 || height > 64 {
		height = 64
	}
	data := make([]byte, width*height*4)
	b := byte(argb)
	g := byte(argb >> 8)
	r := byte(argb >> 16)
	a := byte(argb >> 24)
	if a == 0 {
		a = 0xff
	}
	for i := 0; i < len(data); i += 4 {
		data[i+0] = b
		data[i+1] = g
		data[i+2] = r
		data[i+3] = a
	}
	return buildBitmapUpdate([]bitmapRect{{
		Left:   0,
		Top:    0,
		Right:  uint16(width - 1),
		Bottom: uint16(height - 1),
		Width:  uint16(width),
		Height: uint16(height),
		BPP:    bitmapBPP32,
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
