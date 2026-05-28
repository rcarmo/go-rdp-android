package rdpserver

import (
	"encoding/binary"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestProductionAVC444Encoder(t *testing.T) {
	enc := productionAVC444Encoder{}
	src := frame.Frame{Width: 16, Height: 16, Stride: 64, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(16, 16, 0x11, 0x22, 0x33, 0xff)}
	payload, ok := enc.EncodeRDPGFX(src, 16, 16)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX len=%d ok=%t", len(payload), ok)
	}
	if rects := binary.LittleEndian.Uint32(payload[0:4]); rects != 1 {
		t.Fatalf("rects=%d want 1", rects)
	}
	baseLenOffset := 4 + 8
	baseLen := int(binary.LittleEndian.Uint32(payload[baseLenOffset : baseLenOffset+4]))
	auxLen := int(binary.LittleEndian.Uint32(payload[baseLenOffset+4 : baseLenOffset+8]))
	if baseLen <= 0 || auxLen <= 0 {
		t.Fatalf("baseLen=%d auxLen=%d", baseLen, auxLen)
	}
	if flags := binary.LittleEndian.Uint16(payload[baseLenOffset+8 : baseLenOffset+10]); flags != 0 {
		t.Fatalf("flags=0x%04x want v1", flags)
	}
}

func TestProductionAVC444v2Encoder(t *testing.T) {
	enc := productionAVC444Encoder{v2: true}
	src := frame.Frame{Width: 16, Height: 16, Stride: 64, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(16, 16, 0x11, 0x22, 0x33, 0xff)}
	payload, ok := enc.EncodeRDPGFX(src, 16, 16)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX len=%d ok=%t", len(payload), ok)
	}
	flagsOffset := 4 + 8 + 4 + 4
	if flags := binary.LittleEndian.Uint16(payload[flagsOffset : flagsOffset+2]); flags != 1 {
		t.Fatalf("flags=0x%04x want v2", flags)
	}
}

func TestProductionAVC444EncoderRejectsInvalid(t *testing.T) {
	enc := productionAVC444Encoder{}
	if payload, ok := enc.EncodeRDPGFX(frame.Frame{}, 0, 0); ok || payload != nil {
		t.Fatal("expected invalid frame rejection")
	}
	if payload, ok := enc.EncodeRDPGFX(frame.Frame{Width: 1, Height: 1, Stride: 3, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3}}, 1, 1); ok || payload != nil {
		t.Fatal("expected short data rejection")
	}
}
