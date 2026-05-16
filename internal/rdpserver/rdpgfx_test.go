package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestParseRDPGFXCapsAdvertise(t *testing.T) {
	payload := []byte{0x02, 0x00}
	payload = appendLE32Bytes(payload, rdpgfxCapsVersion8)
	payload = appendLE32Bytes(payload, 0)
	payload = appendLE32Bytes(payload, rdpgfxCapsVersion106)
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
	if binary.LittleEndian.Uint32(pdu[8:12]) != rdpgfxCapsVersion10 || binary.LittleEndian.Uint32(pdu[12:16]) != 7 {
		t.Fatalf("unexpected caps confirm payload: %x", pdu[8:])
	}
}

func FuzzParseRDPGFXPDU(f *testing.F) {
	f.Add(buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{1, 0, 4, 0, 8, 0, 0, 0, 0, 0}))
	f.Add([]byte{0x01, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseRDPGFXPDU(data)
	})
}
