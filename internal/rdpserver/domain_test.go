package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestParseMCSDomainPDU(t *testing.T) {
	erect, err := parseMCSDomainPDU([]byte{byte(mcsErectDomainRequestApp << 2), 1, 0, 1, 0})
	if err != nil {
		t.Fatal(err)
	}
	if erect.Application != mcsErectDomainRequestApp {
		t.Fatalf("unexpected app %d", erect.Application)
	}

	attach, err := parseMCSDomainPDU([]byte{byte(mcsAttachUserRequestApp << 2)})
	if err != nil {
		t.Fatal(err)
	}
	if attach.Application != mcsAttachUserRequestApp {
		t.Fatalf("unexpected app %d", attach.Application)
	}

	joinWire := []byte{byte(mcsChannelJoinRequestApp << 2), 0, 0, 0x03, 0xeb}
	join, err := parseMCSDomainPDU(joinWire)
	if err != nil {
		t.Fatal(err)
	}
	if join.Application != mcsChannelJoinRequestApp || join.Initiator != 1001 || join.ChannelID != 1003 {
		t.Fatalf("unexpected join: %#v", join)
	}
}

func TestEncodePERInteger16(t *testing.T) {
	encoded := encodePERInteger16(1003, 1001)
	if got := binary.BigEndian.Uint16(encoded); got != 2 {
		t.Fatalf("got %d", got)
	}
}
