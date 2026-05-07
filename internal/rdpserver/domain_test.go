package rdpserver

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
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

func TestHandleMCSDomainSequenceGracefulDisconnectProviderUltimatum(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan error, 1)
	go func() {
		done <- handleMCSDomainSequence(serverConn, nil, nil, 320, 240, nil, protocolSSL, nil)
	}()

	if err := sendTestMCSDomainPDU(clientConn, mcsDisconnectProviderUltimatumApp, nil); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handleMCSDomainSequence: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handleMCSDomainSequence did not return")
	}
}

func TestHandleMCSDomainSequenceGracefulDeactivateAll(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	done := make(chan error, 1)
	go func() {
		done <- handleMCSDomainSequence(serverConn, nil, nil, 320, 240, nil, protocolSSL, nil)
	}()

	deactivate := make([]byte, 6)
	binary.LittleEndian.PutUint16(deactivate[0:2], 6)
	binary.LittleEndian.PutUint16(deactivate[2:4], pduTypeDeactivateAll)
	binary.LittleEndian.PutUint16(deactivate[4:6], defaultMCSUserID)
	body := buildMCSSendDataRequest(defaultMCSUserID, globalChannelID, deactivate)
	if err := sendTestMCSDomainPDU(clientConn, mcsSendDataRequestApp, body); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handleMCSDomainSequence: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handleMCSDomainSequence did not return")
	}
}
