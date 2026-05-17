package rdpserver

import "testing"

type testH264Source struct{ ch chan H264Frame }

func (s *testH264Source) H264Frames() <-chan H264Frame { return s.ch }

func TestBuildRDPGFXH264FramePDUs(t *testing.T) {
	unit := h264AccessUnit{PresentationTimeUS: 42, KeyFrame: true, Data: []byte{0, 0, 0, 1, 0x65}}
	pdus, ok := buildRDPGFXH264FramePDUs(0, 7, unit, 64, 48)
	if !ok {
		t.Fatal("buildRDPGFXH264FramePDUs() ok = false, want true")
	}
	if len(pdus) != 3 {
		t.Fatalf("len(pdus) = %d, want 3", len(pdus))
	}
	wire, err := parseRDPGFXPDU(pdus[1])
	if err != nil {
		t.Fatalf("parse wire pdu: %v", err)
	}
	if wire.CmdID != rdpgfxCmdWireToSurface1 {
		t.Fatalf("wire CmdID = 0x%04x, want WireToSurface1", wire.CmdID)
	}
	payload := pdus[1][8:]
	codecID := uint16(payload[2]) | uint16(payload[3])<<8
	if codecID != rdpgfxCodecAVC420 {
		t.Fatalf("codecID = 0x%04x, want AVC420", codecID)
	}
}

func TestBuildRDPGFXH264FramePDUsRejectsConfigOnly(t *testing.T) {
	_, ok := buildRDPGFXH264FramePDUs(0, 1, h264AccessUnit{CodecConfig: true, Data: []byte{1}}, 64, 48)
	if ok {
		t.Fatal("buildRDPGFXH264FramePDUs() ok = true for config-only unit, want false")
	}
}
