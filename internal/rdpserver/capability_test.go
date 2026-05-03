package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestBuildDemandActivePDU(t *testing.T) {
	pdu := buildDemandActivePDU(800, 600)
	if len(pdu) < 32 {
		t.Fatalf("Demand Active too short: %d", len(pdu))
	}
	share, err := parseShareControlPDU(pdu)
	if err != nil {
		t.Fatal(err)
	}
	if share.PDUType != pduTypeDemandActive || share.PDUSource != serverChannelID {
		t.Fatalf("unexpected share header: %#v", share)
	}
}

func TestBuildGeneralCapabilityLength(t *testing.T) {
	cap := capabilitySet(capTypeGeneral, buildGeneralCapability())
	if len(cap) != 24 {
		t.Fatalf("general capability length = %d, want 24", len(cap))
	}
	if got := binary.LittleEndian.Uint16(cap[2:4]); got != 24 {
		t.Fatalf("general capability length field = %d, want 24", got)
	}
}

func TestParseConfirmActive(t *testing.T) {
	pdu := buildTestConfirmActivePDU(defaultShareID, defaultMCSUserID)
	info, err := parseConfirmActive(pdu)
	if err != nil {
		t.Fatal(err)
	}
	if info.ShareID != defaultShareID || info.OriginatorID != 1002 || info.CapabilitySetCount != 1 {
		t.Fatalf("unexpected confirm active: %#v", info)
	}
}

func TestBuildMCSSendDataIndication(t *testing.T) {
	body := buildMCSSendDataIndication(serverChannelID, globalChannelID, []byte{1, 2, 3})
	if len(body) < 9 || body[0] != 0 || body[1] != 1 || body[4] != 0x70 {
		t.Fatalf("unexpected send data indication body: %x", body)
	}
}
