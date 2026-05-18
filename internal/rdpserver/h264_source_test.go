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
	bitmapLen := uint32(payload[13]) | uint32(payload[14])<<8 | uint32(payload[15])<<16 | uint32(payload[16])<<24
	if bitmapLen != uint32(4+8+2+len(unit.Data)) {
		t.Fatalf("bitmapLen = %d, want AVC420 metadata + access unit", bitmapLen)
	}
	bitmap := payload[17:]
	if got := uint32(bitmap[0]) | uint32(bitmap[1])<<8 | uint32(bitmap[2])<<16 | uint32(bitmap[3])<<24; got != 1 {
		t.Fatalf("numRegionRects = %d, want 1", got)
	}
	if left, top := le16ForTest(bitmap[4:6]), le16ForTest(bitmap[6:8]); left != 0 || top != 0 {
		t.Fatalf("region origin = %d,%d, want 0,0", left, top)
	}
	if right, bottom := le16ForTest(bitmap[8:10]), le16ForTest(bitmap[10:12]); right != 64 || bottom != 48 {
		t.Fatalf("region bounds = %d,%d, want 64,48", right, bottom)
	}
	if qp, quality := bitmap[12], bitmap[13]; qp != 0 || quality != 0 {
		t.Fatalf("quant/quality = %d/%d, want 0/0", qp, quality)
	}
	if got := bitmap[14:]; string(got) != string(unit.Data) {
		t.Fatalf("access unit = %x, want %x", got, unit.Data)
	}
}

func le16ForTest(data []byte) uint16 {
	return uint16(data[0]) | uint16(data[1])<<8
}

func TestH264StreamStateQueuesConfigOnly(t *testing.T) {
	var state h264StreamState
	_, ok := state.prepareForWire(h264AccessUnit{CodecConfig: true, Data: []byte{0, 0, 0, 1, 1}})
	if ok {
		t.Fatal("prepareForWire() ok = true for config-only unit, want false")
	}
}
