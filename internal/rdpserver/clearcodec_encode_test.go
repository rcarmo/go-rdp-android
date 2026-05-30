package rdpserver

import (
	"encoding/binary"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func BenchmarkClearCodecEncoderSolidRect(b *testing.B) {
	enc := clearCodecEncoder{}
	src := frame.Frame{Width: 64, Height: 64, Stride: 64 * 4, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(64, 64, 0x11, 0x22, 0x33, 0xff)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		payload, ok := enc.EncodeRDPGFX(src, src.Width, src.Height)
		if !ok || len(payload) != 22 {
			b.Fatalf("EncodeRDPGFX solid len=%d ok=%t", len(payload), ok)
		}
	}
}

func TestClearCodecEncoderSolidRect(t *testing.T) {
	enc := clearCodecEncoder{}
	src := frame.Frame{Width: 8, Height: 8, Stride: 32, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(8, 8, 0x11, 0x22, 0x33, 0xff)}
	payload, ok := enc.EncodeRDPGFX(src, 8, 8)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX solid len=%d ok=%t", len(payload), ok)
	}
	if string(payload[0:4]) != clearCodecMagic {
		t.Fatalf("magic=%q want=%q", string(payload[0:4]), clearCodecMagic)
	}
	if payload[10] != clearCodecOpSolidRect {
		t.Fatalf("opcode=%d want=%d", payload[10], clearCodecOpSolidRect)
	}
}

func TestClearCodecEncoderRawRectRGB565(t *testing.T) {
	enc := clearCodecEncoder{}
	data := make([]byte, 8*8*4)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			i := (y*8 + x) * 4
			data[i+0] = byte(x * 31)
			data[i+1] = byte(y * 17)
			data[i+2] = byte((x + y) * 9)
			data[i+3] = 0xff
		}
	}
	src := frame.Frame{Width: 8, Height: 8, Stride: 32, Format: frame.PixelFormatRGBA8888, Data: data}
	payload, ok := enc.EncodeRDPGFX(src, 8, 8)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX raw len=%d ok=%t", len(payload), ok)
	}
	if payload[10] != clearCodecOpRawRect {
		t.Fatalf("opcode=%d want=%d", payload[10], clearCodecOpRawRect)
	}
	rawLen := int(binary.LittleEndian.Uint32(payload[19:23]))
	if rawLen != 8*8*2 {
		t.Fatalf("rawLen=%d want=%d", rawLen, 8*8*2)
	}
	if len(payload) >= 8*8*4 {
		t.Fatalf("payload len=%d should be smaller than raw=%d", len(payload), 8*8*4)
	}
	rects, ok := parseClearCodecRectHeaders(payload)
	if !ok || len(rects) != 1 {
		t.Fatalf("parseClearCodecRectHeaders len=%d ok=%t", len(rects), ok)
	}
}

func TestClearCodecEncoderUsesSolidBands(t *testing.T) {
	enc := clearCodecEncoder{}
	w, h := 1024, 130
	data := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if y < 128 {
				data[i+0], data[i+1], data[i+2], data[i+3] = 0x11, 0x22, 0x33, 0xff
			} else {
				data[i+0], data[i+1], data[i+2], data[i+3] = byte(x), byte(y), byte(x+y), 0xff
			}
		}
	}
	src := frame.Frame{Width: w, Height: h, Stride: w * 4, Format: frame.PixelFormatRGBA8888, Data: data}
	payload, ok := enc.EncodeRDPGFX(src, w, h)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX banded len=%d ok=%t", len(payload), ok)
	}
	rects, ok := parseClearCodecRectHeaders(payload)
	if !ok || len(rects) < 2 {
		t.Fatalf("rects len=%d ok=%t", len(rects), ok)
	}
	if payload[10] != clearCodecOpSolidRect {
		t.Fatalf("first opcode=%d want solid", payload[10])
	}
}

func TestClearCodecEncoderTiledMixedRects(t *testing.T) {
	enc := clearCodecEncoder{}
	w, h := 128, 64
	data := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			if x < 64 {
				data[i+0], data[i+1], data[i+2], data[i+3] = 0x22, 0x44, 0x66, 0xff
			} else {
				data[i+0], data[i+1], data[i+2], data[i+3] = byte(x), byte(y), byte(x+y), 0xff
			}
		}
	}
	src := frame.Frame{Width: w, Height: h, Stride: w * 4, Format: frame.PixelFormatRGBA8888, Data: data}
	payload, ok := enc.EncodeRDPGFX(src, w, h)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX tiled len=%d ok=%t", len(payload), ok)
	}
	rects, ok := parseClearCodecRectHeaders(payload)
	if !ok || len(rects) != 2 {
		t.Fatalf("rects len=%d ok=%t", len(rects), ok)
	}
	if payload[10] != clearCodecOpSolidRect {
		t.Fatalf("first opcode=%d want solid", payload[10])
	}
	secondOffset := 10 + 13 + 3
	if payload[secondOffset] != clearCodecOpRawRect {
		t.Fatalf("second opcode=%d want raw", payload[secondOffset])
	}
}

func TestClearCodecEncoderLargeFrameSplitsRects(t *testing.T) {
	enc := clearCodecEncoder{}
	w, h := 1024, 256
	data := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 4
			data[i+0] = byte((x + y) & 0xff)
			data[i+1] = byte((x * 3) & 0xff)
			data[i+2] = byte((y * 5) & 0xff)
			data[i+3] = 0xff
		}
	}
	src := frame.Frame{Width: w, Height: h, Stride: w * 4, Format: frame.PixelFormatRGBA8888, Data: data}
	payload, ok := enc.EncodeRDPGFX(src, w, h)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX large len=%d ok=%t", len(payload), ok)
	}
	rects, ok := parseClearCodecRectHeaders(payload)
	if !ok || len(rects) <= 1 {
		t.Fatalf("expected split rectangles, got len=%d ok=%t", len(rects), ok)
	}
	for i, r := range rects {
		if int(r[0])+int(r[2]) > w || int(r[1])+int(r[3]) > h {
			t.Fatalf("rect %d out of bounds: %+v", i, r)
		}
	}
}

func parseClearCodecRectHeaders(payload []byte) ([][4]uint16, bool) {
	if len(payload) < 10 || string(payload[:4]) != clearCodecMagic {
		return nil, false
	}
	rectCount := int(binary.LittleEndian.Uint16(payload[8:10]))
	off := 10
	rects := make([][4]uint16, 0, rectCount)
	for i := 0; i < rectCount; i++ {
		if off+13 > len(payload) {
			return nil, false
		}
		op := payload[off]
		x := binary.LittleEndian.Uint16(payload[off+1 : off+3])
		y := binary.LittleEndian.Uint16(payload[off+3 : off+5])
		w := binary.LittleEndian.Uint16(payload[off+5 : off+7])
		h := binary.LittleEndian.Uint16(payload[off+7 : off+9])
		l := int(binary.LittleEndian.Uint32(payload[off+9 : off+13]))
		off += 13
		if op != clearCodecOpRawRect && op != clearCodecOpSolidRect {
			return nil, false
		}
		rects = append(rects, [4]uint16{x, y, w, h})
		if op == clearCodecOpSolidRect {
			off += 3
		} else {
			off += l
		}
		if off > len(payload) {
			return nil, false
		}
	}
	return rects, true
}

func TestClearCodecEncoderRejectsInvalid(t *testing.T) {
	enc := clearCodecEncoder{}
	if _, ok := enc.EncodeRDPGFX(frame.Frame{}, 0, 0); ok {
		t.Fatal("EncodeRDPGFX accepted invalid frame")
	}
}
