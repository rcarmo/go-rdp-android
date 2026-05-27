package rdpserver

import "testing"

func TestBuildRDPGFXAVC444FramePDUs(t *testing.T) {
	in := avc444EncoderInput{
		Width:       64,
		Height:      64,
		BaseLayer:   h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x65, 0x88}, KeyFrame: true},
		AuxLayer:    h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x61, 0x11}},
		RegionRects: []avc444RegionRect{{Left: 0, Top: 0, Right: 64, Bottom: 64}},
	}
	pdus, ok := buildRDPGFXAVC444FramePDUs(0, 5, in)
	if !ok || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXAVC444FramePDUs len=%d ok=%t", len(pdus), ok)
	}
	codecID, ok := wireToSurfaceCodecIDForTest(pdus[1])
	if !ok || codecID != rdpgfxCodecAVC444 {
		t.Fatalf("wire codec=0x%04x ok=%t", codecID, ok)
	}
}

func TestBuildRDPGFXAVC444v2FramePDUs(t *testing.T) {
	in := avc444EncoderInput{
		Width:       64,
		Height:      64,
		BaseLayer:   h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x65, 0x88}, KeyFrame: true},
		AuxLayer:    h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x61, 0x11}},
		RegionRects: []avc444RegionRect{{Left: 0, Top: 0, Right: 64, Bottom: 64}},
	}
	pdus, ok := buildRDPGFXAVC444v2FramePDUs(0, 6, in)
	if !ok || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXAVC444v2FramePDUs len=%d ok=%t", len(pdus), ok)
	}
	codecID, ok := wireToSurfaceCodecIDForTest(pdus[1])
	if !ok || codecID != rdpgfxCodecAVC444v2 {
		t.Fatalf("wire codec=0x%04x ok=%t", codecID, ok)
	}
}

func TestBuildRDPGFXAVC444FramePDUsRejectsInvalidInput(t *testing.T) {
	if _, ok := buildRDPGFXAVC444FramePDUs(0, 1, avc444EncoderInput{}); ok {
		t.Fatal("buildRDPGFXAVC444FramePDUs accepted invalid input")
	}
	if _, ok := buildRDPGFXAVC444v2FramePDUs(0, 1, avc444EncoderInput{}); ok {
		t.Fatal("buildRDPGFXAVC444v2FramePDUs accepted invalid input")
	}
}
