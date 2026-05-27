package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestBuildRDPGFXAVC444BitmapStreamLayout(t *testing.T) {
	in := avc444EncoderInput{
		Width:       64,
		Height:      64,
		BaseLayer:   h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x65, 0x88}, KeyFrame: true},
		AuxLayer:    h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x61, 0x11}},
		RegionRects: []avc444RegionRect{{Left: 0, Top: 0, Right: 64, Bottom: 64}},
	}
	payload, ok := buildRDPGFXAVC444BitmapStream(in)
	if !ok || len(payload) == 0 {
		t.Fatalf("buildRDPGFXAVC444BitmapStream len=%d ok=%t", len(payload), ok)
	}
	if got := binary.LittleEndian.Uint32(payload[0:4]); got != 1 {
		t.Fatalf("numRegionRects=%d want=1", got)
	}
	if got := binary.LittleEndian.Uint16(payload[4:6]); got != 0 {
		t.Fatalf("rect.left=%d want=0", got)
	}
	if got := binary.LittleEndian.Uint16(payload[8:10]); got != 64 {
		t.Fatalf("rect.right=%d want=64", got)
	}
	off := 12
	baseLen := int(binary.LittleEndian.Uint32(payload[off : off+4]))
	off += 4
	auxLen := int(binary.LittleEndian.Uint32(payload[off : off+4]))
	off += 4
	flags := binary.LittleEndian.Uint16(payload[off : off+2])
	off += 4 // flags + reserved
	if baseLen != len(in.BaseLayer.Data) || auxLen != len(in.AuxLayer.Data) {
		t.Fatalf("layer lengths base=%d aux=%d want=%d/%d", baseLen, auxLen, len(in.BaseLayer.Data), len(in.AuxLayer.Data))
	}
	if flags != 0 {
		t.Fatalf("flags=%d want=0", flags)
	}
	if got := payload[off : off+baseLen]; string(got) != string(in.BaseLayer.Data) {
		t.Fatalf("base payload mismatch")
	}
}

func TestBuildRDPGFXAVC444v2BitmapStreamSetsFlag(t *testing.T) {
	in := avc444EncoderInput{
		Width:       64,
		Height:      64,
		BaseLayer:   h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x65, 0x88}, KeyFrame: true},
		AuxLayer:    h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x61, 0x11}},
		RegionRects: []avc444RegionRect{{Left: 0, Top: 0, Right: 64, Bottom: 64}},
	}
	payload, ok := buildRDPGFXAVC444v2BitmapStream(in)
	if !ok || len(payload) < 20 {
		t.Fatalf("buildRDPGFXAVC444v2BitmapStream len=%d ok=%t", len(payload), ok)
	}
	off := 4 + 8 + 4 + 4
	flags := binary.LittleEndian.Uint16(payload[off : off+2])
	if flags != 1 {
		t.Fatalf("v2 flags=%d want=1", flags)
	}
}

func TestBuildRDPGFXAVC444BitmapStreamRejectsInvalidInput(t *testing.T) {
	bad := avc444EncoderInput{}
	if _, ok := buildRDPGFXAVC444BitmapStream(bad); ok {
		t.Fatal("buildRDPGFXAVC444BitmapStream accepted invalid input")
	}

	tooBig := avc444EncoderInput{
		Width:       64,
		Height:      64,
		BaseLayer:   h264AccessUnit{PresentationTimeUS: 1, Data: make([]byte, rdpgfxMaxPDUSize), KeyFrame: true},
		AuxLayer:    h264AccessUnit{PresentationTimeUS: 1, Data: []byte{1}},
		RegionRects: []avc444RegionRect{{Left: 0, Top: 0, Right: 64, Bottom: 64}},
	}
	if _, ok := buildRDPGFXAVC444BitmapStream(tooBig); ok {
		t.Fatal("buildRDPGFXAVC444BitmapStream accepted oversized payload")
	}
}
