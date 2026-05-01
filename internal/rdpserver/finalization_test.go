package rdpserver

import "testing"

func TestBuildAndParseShareDataPDU(t *testing.T) {
	wire := buildShareDataPDU(pduType2Synchronize, buildSynchronizePayload())
	share, err := parseShareControlPDU(wire)
	if err != nil {
		t.Fatal(err)
	}
	data, err := parseShareDataPDU(share)
	if err != nil {
		t.Fatal(err)
	}
	if data.PDUType2 != pduType2Synchronize || data.ShareID != defaultShareID || len(data.Payload) != 4 {
		t.Fatalf("unexpected share data: %#v", data)
	}
}

func TestControlAndFontPayloads(t *testing.T) {
	ctrl := buildControlPayload(controlActionGrantedControl)
	action, err := parseControlAction(ctrl)
	if err != nil {
		t.Fatal(err)
	}
	if action != controlActionGrantedControl {
		t.Fatalf("unexpected action %d", action)
	}
	font := buildFontMapPayload()
	if len(font) != 8 || font[4] != 3 {
		t.Fatalf("unexpected font map payload: %x", font)
	}
}
