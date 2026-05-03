package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestBuildLicenseValidClientPDU(t *testing.T) {
	pdu := buildLicenseValidClientPDU()
	if len(pdu) != 20 {
		t.Fatalf("license PDU length = %d, want 20", len(pdu))
	}
	if flags := binary.LittleEndian.Uint16(pdu[0:2]); flags != secLicensePacket {
		t.Fatalf("security flags = 0x%04x, want license", flags)
	}
	if pdu[4] != licenseErrorAlert || pdu[5] != licensePreambleVersion3 {
		t.Fatalf("unexpected license preamble: %x", pdu[4:8])
	}
	if size := binary.LittleEndian.Uint16(pdu[6:8]); size != 16 {
		t.Fatalf("license size = %d, want 16", size)
	}
	if code := binary.LittleEndian.Uint32(pdu[8:12]); code != licenseStatusValidClient {
		t.Fatalf("license error code = 0x%08x, want valid client", code)
	}
	if state := binary.LittleEndian.Uint32(pdu[12:16]); state != licenseStateNoTransition {
		t.Fatalf("license state = 0x%08x, want no transition", state)
	}
}
