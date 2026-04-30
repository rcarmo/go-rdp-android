package rdpserver

import "testing"

func TestParseMCSSendDataRequest(t *testing.T) {
	sec := []byte{secInfoPacket, 0, 0, 0, 1, 2, 3}
	body := buildMCSSendDataRequest(defaultMCSUserID, 1003, sec)
	req, err := parseMCSSendDataRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if req.Initiator != defaultMCSUserID || req.ChannelID != 1003 || len(req.Data) != len(sec) {
		t.Fatalf("unexpected request: %#v", req)
	}
}

func TestParseSecurityPDU(t *testing.T) {
	pdu, err := parseSecurityPDU([]byte{secInfoPacket, 0, 0, 0, 1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if pdu.Flags != secInfoPacket || pdu.KindLabel != "client-info" || len(pdu.Payload) != 3 {
		t.Fatalf("unexpected security pdu: %#v", pdu)
	}

	pdu, err = parseSecurityPDU([]byte{secExchangePacket, 0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if pdu.KindLabel != "security-exchange" {
		t.Fatalf("unexpected kind %q", pdu.KindLabel)
	}
}

func TestEncodePERLength(t *testing.T) {
	if got := encodePERLength(0x7f); len(got) != 1 || got[0] != 0x7f {
		t.Fatalf("bad short length: %x", got)
	}
	if got := encodePERLength(0x80); len(got) != 2 || got[0] != 0x80 || got[1] != 0x80 {
		t.Fatalf("bad long length: %x", got)
	}
}
