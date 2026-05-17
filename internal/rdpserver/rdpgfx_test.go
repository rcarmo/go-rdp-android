package rdpserver

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestParseRDPGFXCapsAdvertise(t *testing.T) {
	payload := []byte{0x02, 0x00}
	payload = appendLE32Bytes(payload, rdpgfxCapsVersion8)
	payload = appendLE32Bytes(payload, 4)
	payload = appendLE32Bytes(payload, 0)
	payload = appendLE32Bytes(payload, rdpgfxCapsVersion106)
	payload = appendLE32Bytes(payload, 4)
	payload = appendLE32Bytes(payload, 0x00000003)
	pduBytes := buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, payload)

	pdu, err := parseRDPGFXPDU(pduBytes)
	if err != nil {
		t.Fatal(err)
	}
	if pdu.CmdID != rdpgfxCmdCapsAdvertise || len(pdu.Caps) != 2 {
		t.Fatalf("unexpected pdu: cmd=0x%x caps=%d", pdu.CmdID, len(pdu.Caps))
	}
	best, ok := negotiateRDPGFXCapability(pdu.Caps)
	if !ok {
		t.Fatal("expected negotiated capability")
	}
	if best.Version != rdpgfxCapsVersion106 || best.Flags != 0x00000003 {
		t.Fatalf("unexpected negotiated capability: version=0x%08x flags=0x%08x", best.Version, best.Flags)
	}
}

func TestParseRDPGFXRejectsMalformedPDUs(t *testing.T) {
	cases := map[string][]byte{
		"short header": {0x01, 0x00},
		"length mismatch": func() []byte {
			p := buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{1, 0, 0, 0, 0, 0, 0, 0})
			binary.LittleEndian.PutUint32(p[4:8], uint32(len(p)+1))
			return p
		}(),
		"empty caps": buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{0, 0}),
		"short caps": buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{2, 0, 0, 0, 0, 0, 0, 0}),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseRDPGFXPDU(data); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBuildRDPGFXCapsConfirmPDU(t *testing.T) {
	pdu := buildRDPGFXCapsConfirmPDU(rdpgfxCapabilitySet{Version: rdpgfxCapsVersion10, Flags: 7})
	parsed, err := parseRDPGFXPDU(pdu)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.CmdID != rdpgfxCmdCapsConfirm {
		t.Fatalf("unexpected cmd 0x%x", parsed.CmdID)
	}
	if binary.LittleEndian.Uint32(pdu[8:12]) != rdpgfxCapsVersion10 || binary.LittleEndian.Uint32(pdu[12:16]) != 4 || binary.LittleEndian.Uint32(pdu[16:20]) != 7 {
		t.Fatalf("unexpected caps confirm payload: %x", pdu[8:])
	}
}

func TestBuildRDPGFXSurfaceAndFramePDUs(t *testing.T) {
	create, ok := buildRDPGFXCreateSurfacePDU(1, 640, 480)
	if !ok {
		t.Fatal("expected create surface PDU")
	}
	if pdu, err := parseRDPGFXPDU(create); err != nil || pdu.CmdID != rdpgfxCmdCreateSurface {
		t.Fatalf("unexpected create pdu=%#v err=%v", pdu, err)
	}
	mapped, ok := buildRDPGFXMapSurfaceToOutputPDU(1, 0, 0)
	if !ok {
		t.Fatal("expected map surface PDU")
	}
	if pdu, err := parseRDPGFXPDU(mapped); err != nil || pdu.CmdID != rdpgfxCmdMapSurfaceToOutput {
		t.Fatalf("unexpected map pdu=%#v err=%v", pdu, err)
	}

	src := frame.Frame{
		Width:     16,
		Height:    1,
		Stride:    64,
		Format:    frame.PixelFormatRGBA8888,
		Timestamp: time.Now(),
		Data:      repeatRGBAForTest(16, 0x11, 0x22, 0x33, 0xff),
	}
	pdus, ok := buildRDPGFXPlanarFramePDUs(1, 42, src, 16, 1)
	if !ok || len(pdus) != 3 {
		t.Fatalf("expected three frame PDUs, ok=%t len=%d", ok, len(pdus))
	}
	for i, want := range []uint16{rdpgfxCmdStartFrame, rdpgfxCmdWireToSurface1, rdpgfxCmdEndFrame} {
		pdu, err := parseRDPGFXPDU(pdus[i])
		if err != nil {
			t.Fatalf("parse frame pdu %d: %v", i, err)
		}
		if pdu.CmdID != want {
			t.Fatalf("frame pdu %d cmd=0x%x want=0x%x", i, pdu.CmdID, want)
		}
	}
	wire := pdus[1]
	if codec := binary.LittleEndian.Uint16(wire[10:12]); codec != rdpgfxCodecPlanar {
		t.Fatalf("codec=0x%x", codec)
	}
	payload := wire[25:]
	if len(payload) == 0 || payload[0] != 0x30 {
		t.Fatalf("expected no-alpha RLE planar payload, got=%x", payload)
	}
	if len(payload) >= 64 {
		t.Fatalf("expected compact planar payload smaller than raw XRGB, got %d bytes", len(payload))
	}
}

func repeatRGBAForTest(count int, r, g, b, a byte) []byte {
	out := make([]byte, 0, count*4)
	for i := 0; i < count; i++ {
		out = append(out, r, g, b, a)
	}
	return out
}

func FuzzParseRDPGFXPDU(f *testing.F) {
	f.Add(buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{1, 0, 4, 0, 8, 0, 0, 0, 0, 0}))
	f.Add([]byte{0x01, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseRDPGFXPDU(data)
	})
}
