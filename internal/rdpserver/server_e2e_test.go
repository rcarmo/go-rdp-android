package rdpserver

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestServerLoopbackInitialHandshakeAndMCSProbe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := New(Config{Width: 800, Height: 600}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln) }()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := sendTestX224ConnectionRequest(conn); err != nil {
		t.Fatal(err)
	}
	resp, err := readTPKT(conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp) < 15 || resp[1] != x224TypeConnectionConfirm || resp[7] != rdpNegResp {
		t.Fatalf("unexpected X.224 confirm: %x", resp)
	}

	if err := sendTestMCSConnectInitial(conn); err != nil {
		t.Fatal(err)
	}
	mcsRespTPKT, err := readTPKT(conn)
	if err != nil {
		t.Fatal(err)
	}
	mcsPayload, err := parseX224Data(mcsRespTPKT)
	if err != nil {
		t.Fatal(err)
	}
	mcsInfo, err := parseMCSConnectResponse(mcsPayload)
	if err != nil {
		t.Fatal(err)
	}
	if mcsInfo.ApplicationTag != mcsConnectResponseAppTag {
		t.Fatalf("expected MCS connect response, got %#v", mcsInfo)
	}

	if err := sendTestMCSDomainPDU(conn, mcsErectDomainRequestApp, []byte{1, 0, 1, 0}); err != nil {
		t.Fatal(err)
	}
	if err := sendTestMCSDomainPDU(conn, mcsAttachUserRequestApp, nil); err != nil {
		t.Fatal(err)
	}
	attachResp, err := readTestMCSDomainPDU(conn)
	if err != nil {
		t.Fatal(err)
	}
	if attachResp.Application != mcsAttachUserConfirmApp {
		t.Fatalf("expected attach confirm, got %#v", attachResp)
	}

	joinBody := append(encodePERInteger16(defaultMCSUserID, defaultMCSUserID), encodePERInteger16(1003, 0)...)
	if err := sendTestMCSDomainPDU(conn, mcsChannelJoinRequestApp, joinBody); err != nil {
		t.Fatal(err)
	}
	joinResp, err := readTestMCSDomainPDU(conn)
	if err != nil {
		t.Fatal(err)
	}
	if joinResp.Application != mcsChannelJoinConfirmApp {
		t.Fatalf("expected channel join confirm, got %#v", joinResp)
	}

	clientInfo := []byte{secInfoPacket, 0, 0, 0, 1, 2, 3, 4}
	if err := sendTestMCSDomainPDU(conn, mcsSendDataRequestApp, buildMCSSendDataRequest(defaultMCSUserID, globalChannelID, clientInfo)); err != nil {
		t.Fatal(err)
	}
	demandResp, err := readTestMCSDomainPDU(conn)
	if err != nil {
		t.Fatal(err)
	}
	if demandResp.Application != mcsSendDataIndicationApp {
		t.Fatalf("expected SendDataIndication with Demand Active, got %#v", demandResp)
	}
	share, err := parseShareControlPDU(demandResp.Data)
	if err != nil {
		t.Fatal(err)
	}
	if share.PDUType != pduTypeDemandActive {
		t.Fatalf("expected Demand Active, got 0x%04x", share.PDUType)
	}

	confirm := buildTestConfirmActivePDU(defaultShareID, defaultMCSUserID)
	if err := sendTestMCSDomainPDU(conn, mcsSendDataRequestApp, buildMCSSendDataRequest(defaultMCSUserID, globalChannelID, confirm)); err != nil {
		t.Fatal(err)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func sendTestX224ConnectionRequest(conn net.Conn) error {
	neg := make([]byte, 8)
	neg[0] = rdpNegReq
	binary.LittleEndian.PutUint16(neg[2:4], 8)
	binary.LittleEndian.PutUint32(neg[4:8], protocolSSL)
	userData := append([]byte("Cookie: mstshash=test\r\n"), neg...)
	li := byte(6 + len(userData))
	x224 := []byte{li, x224TypeConnectionRequest, 0, 0, 0, 1, 0}
	x224 = append(x224, userData...)
	return writeTPKT(conn, x224)
}

func sendTestMCSConnectInitial(conn net.Conn) error {
	// X.224 Data TPDU + minimal BER [APPLICATION 101] length 0.
	return writeTPKT(conn, []byte{0x02, x224TypeData, 0x80, 0x7f, 0x65, 0x00})
}

func sendTestMCSDomainPDU(conn net.Conn, application int, body []byte) error {
	mcs := append([]byte{byte(application << 2)}, body...)
	return writeTPKT(conn, append([]byte{0x02, x224TypeData, 0x80}, mcs...))
}

func readTestMCSDomainPDU(conn net.Conn) (*domainPDU, error) {
	payload, err := readTPKT(conn)
	if err != nil {
		return nil, err
	}
	mcs, err := parseX224Data(payload)
	if err != nil {
		return nil, err
	}
	return parseMCSDomainPDU(mcs)
}

func buildTestConfirmActivePDU(shareID uint32, userID uint16) []byte {
	source := []byte("TEST")
	cap := capabilitySet(capTypeGeneral, buildGeneralCapability())
	combinedCapsLen := 4 + len(cap)
	totalLength := 6 + 4 + 2 + 2 + 2 + len(source) + combinedCapsLen
	buf := make([]byte, 0, totalLength)
	pdu := appendShareControlHeader(buf, totalLength, pduTypeConfirmActive, userID)
	pdu = appendLE32(pdu, shareID)
	pdu = appendLE16(pdu, serverChannelID)
	pdu = appendLE16(pdu, uint16(len(source)))
	pdu = appendLE16(pdu, uint16(combinedCapsLen))
	pdu = append(pdu, source...)
	pdu = appendLE16(pdu, 1)
	pdu = appendLE16(pdu, 0)
	pdu = append(pdu, cap...)
	return pdu
}

func appendShareControlHeader(dst []byte, totalLength int, pduType uint16, source uint16) []byte {
	dst = appendLE16(dst, uint16(totalLength))
	dst = appendLE16(dst, pduType)
	dst = appendLE16(dst, source)
	return dst
}

func appendLE16(dst []byte, v uint16) []byte {
	return append(dst, byte(v), byte(v>>8))
}

func appendLE32(dst []byte, v uint32) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
