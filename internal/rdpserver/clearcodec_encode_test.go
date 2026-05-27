package rdpserver

import (
	"encoding/binary"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

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
}

func TestClearCodecEncoderRejectsInvalid(t *testing.T) {
	enc := clearCodecEncoder{}
	if _, ok := enc.EncodeRDPGFX(frame.Frame{}, 0, 0); ok {
		t.Fatal("EncodeRDPGFX accepted invalid frame")
	}
}
