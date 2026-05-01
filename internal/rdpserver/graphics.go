package rdpserver

import "encoding/binary"

const (
	pduType2Update = 0x02

	updateTypeBitmap = 0x0001
	bitmapBPP32      = 32
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
