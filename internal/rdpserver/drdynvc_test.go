package rdpserver

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/input"
)

func TestNewDRDYNVCManagerFindsStaticChannel(t *testing.T) {
	m := newDRDYNVCManager([]clientChannel{{Name: "cliprdr", ID: 1004}, {Name: "drdynvc", ID: 1005}}, nil)
	if !m.enabled() || m.staticChannelID != 1005 {
		t.Fatalf("unexpected manager: %#v", m)
	}
}

func TestParseAndBuildStaticVirtualChannelPDU(t *testing.T) {
	payload := []byte{1, 2, 3}
	wire := buildStaticVirtualChannelPDU(payload)
	pdu, err := parseStaticVirtualChannelPDU(wire)
	if err != nil {
		t.Fatalf("parseStaticVirtualChannelPDU: %v", err)
	}
	if pdu.Length != uint32(len(payload)) || pdu.Flags != channelFlagFirst|channelFlagLast || !bytes.Equal(pdu.Data, payload) {
		t.Fatalf("unexpected static PDU: %#v", pdu)
	}
}

func TestParseDRDYNVCCapsPDU(t *testing.T) {
	pdu, err := parseDRDYNVCPDU(buildDRDYNVCCapsPDU(drdynvcCapsVersion1))
	if err != nil {
		t.Fatalf("parseDRDYNVCPDU: %v", err)
	}
	if pdu.Header.Cmd != drdynvcCmdCapability || pdu.Version != drdynvcCapsVersion1 {
		t.Fatalf("unexpected caps PDU: %#v", pdu)
	}
}

func TestParseDRDYNVCCreatePDU(t *testing.T) {
	wire := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize(), 3}
	wire = append(wire, []byte(rdpeiDynamicChannelName)...)
	wire = append(wire, 0)
	pdu, err := parseDRDYNVCPDU(wire)
	if err != nil {
		t.Fatalf("parseDRDYNVCPDU: %v", err)
	}
	if pdu.Header.Cmd != drdynvcCmdCreate || pdu.ChannelID != 3 || pdu.Name != rdpeiDynamicChannelName {
		t.Fatalf("unexpected create PDU: %#v", pdu)
	}
}

func TestBuildDRDYNVCCreateResponsePDU(t *testing.T) {
	wire := buildDRDYNVCCreateResponsePDU(3, drdynvcCreateOK)
	if wire[0] != (drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize() || wire[1] != 3 {
		t.Fatalf("unexpected response prefix: %x", wire)
	}
	if code := binary.LittleEndian.Uint32(wire[2:6]); code != drdynvcCreateOK {
		t.Fatalf("creation code = 0x%x", code)
	}
}

func TestBuildDRDYNVCDataPDU(t *testing.T) {
	data := buildRDPEISCReadyPDU(rdpeiProtocolV300, nil)
	wire := buildDRDYNVCDataPDU(3, data)
	pdu, err := parseDRDYNVCPDU(wire)
	if err != nil {
		t.Fatalf("parseDRDYNVCPDU: %v", err)
	}
	if pdu.Header.Cmd != drdynvcCmdData || pdu.ChannelID != 3 || !bytes.Equal(pdu.Data, data) {
		t.Fatalf("unexpected data PDU: %#v", pdu)
	}
}

func TestDRDYNVCManagerCreateRDPEIWritesResponseAndSCReady(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil)
	create := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize(), 7}
	create = append(create, []byte(rdpeiDynamicChannelName)...)
	create = append(create, 0)

	done := make(chan error, 1)
	go func() {
		done <- m.handleStaticPDU(server, buildStaticVirtualChannelPDU(create))
	}()

	first := readTestDomainPDUFromPipe(t, client)
	firstStatic, err := parseStaticVirtualChannelPDU(first.Data)
	if err != nil {
		t.Fatalf("parse first static PDU: %v", err)
	}
	if len(firstStatic.Data) != 6 || firstStatic.Data[0] != (drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize() || firstStatic.Data[1] != 7 {
		t.Fatalf("unexpected create response prefix: %x", firstStatic.Data)
	}
	if code := binary.LittleEndian.Uint32(firstStatic.Data[2:6]); code != drdynvcCreateOK {
		t.Fatalf("create response code = 0x%x", code)
	}

	second := readTestDomainPDUFromPipe(t, client)
	secondStatic, err := parseStaticVirtualChannelPDU(second.Data)
	if err != nil {
		t.Fatalf("parse second static PDU: %v", err)
	}
	secondDVC, err := parseDRDYNVCPDU(secondStatic.Data)
	if err != nil {
		t.Fatalf("parse second DVC PDU: %v", err)
	}
	if secondDVC.Header.Cmd != drdynvcCmdData || secondDVC.ChannelID != 7 {
		t.Fatalf("unexpected SC_READY wrapper: %#v", secondDVC)
	}
	if rdpei, err := parseRDPEIPDU(secondDVC.Data); err != nil || rdpei.SCReady == nil {
		t.Fatalf("expected RDPEI SC_READY, got %#v err=%v", rdpei, err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handleStaticPDU: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handleStaticPDU did not return")
	}
	if !m.hasRDPEIChannel || m.rdpeiChannelID != 7 {
		t.Fatalf("RDPEI channel not tracked: %#v", m)
	}
}

func TestDRDYNVCManagerHandlesRDPEIData(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink)
	m.hasRDPEIChannel = true
	m.rdpeiChannelID = 9
	touchPayload := rdpeiTouchEventPayloadForTest(4, 11, 22, rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact)
	data := buildDRDYNVCDataPDU(9, withRDPEIHeader(rdpeiEventTouch, touchPayload))
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(data)); err != nil {
		t.Fatalf("handleStaticPDU: %v", err)
	}
	if len(sink.frames) != 1 || len(sink.frames[0]) != 1 {
		t.Fatalf("expected one touch frame, got %#v", sink.frames)
	}
	contact := sink.frames[0][0]
	if contact.ID != 4 || contact.X != 11 || contact.Y != 22 || contact.Flags&input.TouchDown == 0 {
		t.Fatalf("unexpected dispatched contact: %#v", contact)
	}
}

func TestDRDYNVCDataFirstAssembly(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink)
	m.hasRDPEIChannel = true
	m.rdpeiChannelID = 9
	rdpei := withRDPEIHeader(rdpeiEventTouch, rdpeiTouchEventPayloadForTest(1, 100, 200, rdpeiContactFlagUp))
	firstPayload := buildDRDYNVCDataFirstPDUForTest(9, uint32(len(rdpei)), rdpei[:4])
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(firstPayload)); err != nil {
		t.Fatalf("data first: %v", err)
	}
	if len(m.fragments) != 1 {
		t.Fatalf("expected pending fragment, got %#v", m.fragments)
	}
	secondPayload := buildDRDYNVCDataPDU(9, rdpei[4:])
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(secondPayload)); err != nil {
		t.Fatalf("data completion: %v", err)
	}
	if len(m.fragments) != 0 {
		t.Fatalf("fragments not cleared: %#v", m.fragments)
	}
	if len(sink.frames) != 1 || sink.frames[0][0].Flags&input.TouchUp == 0 {
		t.Fatalf("assembled RDPEI touch was not dispatched: %#v", sink.frames)
	}
}

func TestParseDRDYNVCErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty", data: nil},
		{name: "short caps", data: []byte{(drdynvcHeader{Cmd: drdynvcCmdCapability}).serialize()}},
		{name: "create missing nul", data: []byte{(drdynvcHeader{Cmd: drdynvcCmdCreate}).serialize(), 1, 'x'}},
		{name: "unsupported", data: []byte{(drdynvcHeader{Cmd: 0x0f}).serialize()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseDRDYNVCPDU(tt.data); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func FuzzParseDRDYNVCPDU(f *testing.F) {
	f.Add(buildDRDYNVCCapsPDU(drdynvcCapsVersion1))
	f.Add(buildDRDYNVCDataPDU(1, withRDPEIHeader(rdpeiEventDismissHoveringTouchContact, []byte{1})))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseDRDYNVCPDU(data)
	})
}

type testDomainPDU struct {
	Application int
	ChannelID   uint16
	Data        []byte
}

func readTestDomainPDUFromPipe(t *testing.T, conn net.Conn) testDomainPDU {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	transport, err := readTransportPDU(conn)
	if err != nil {
		t.Fatalf("read transport: %v", err)
	}
	pdu, err := parseMCSDomainTransportPayload(transport.Payload)
	if err != nil {
		t.Fatalf("parse domain: %v", err)
	}
	return testDomainPDU{Application: pdu.Application, ChannelID: pdu.ChannelID, Data: pdu.Data}
}

func buildDRDYNVCDataFirstPDUForTest(channelID uint32, totalLength uint32, data []byte) []byte {
	cb := drdynvcCbChID(channelID)
	sp := uint8(2)
	out := []byte{(drdynvcHeader{CbChID: cb, Sp: sp, Cmd: drdynvcCmdDataFirst}).serialize()}
	out = appendDVCChannelID(out, cb, channelID)
	out = appendLE32Bytes(out, totalLength)
	out = append(out, data...)
	return out
}

func rdpeiTouchEventPayloadForTest(contactID uint8, x, y int32, flags uint32) []byte {
	payload := []byte{}
	payload = append(payload, rdpeiTestVarUint32(0)...)
	payload = append(payload, rdpeiTestVarUint16(1)...)
	payload = append(payload, rdpeiTestVarUint16(1)...)
	payload = append(payload, rdpeiTestVarUint64(0)...)
	payload = append(payload, contactID)
	payload = append(payload, rdpeiTestVarUint16(0)...)
	payload = append(payload, rdpeiTestVarInt32(x)...)
	payload = append(payload, rdpeiTestVarInt32(y)...)
	payload = append(payload, rdpeiTestVarUint32(flags)...)
	return payload
}

type recordingTouchSink struct {
	frames [][]input.TouchContact
}

func (s *recordingTouchSink) PointerMove(_, _ int) error { return nil }
func (s *recordingTouchSink) PointerButton(_, _ int, _ input.ButtonState, _ bool) error {
	return nil
}
func (s *recordingTouchSink) Key(_ uint16, _ bool) error { return nil }
func (s *recordingTouchSink) Unicode(_ rune) error       { return nil }
func (s *recordingTouchSink) TouchFrame(contacts []input.TouchContact) error {
	s.frames = append(s.frames, append([]input.TouchContact(nil), contacts...))
	return nil
}

type discardConn struct{}

func (discardConn) Read([]byte) (int, error)         { return 0, nil }
func (discardConn) Write(p []byte) (int, error)      { return len(p), nil }
func (discardConn) Close() error                     { return nil }
func (discardConn) LocalAddr() net.Addr              { return nil }
func (discardConn) RemoteAddr() net.Addr             { return nil }
func (discardConn) SetDeadline(time.Time) error      { return nil }
func (discardConn) SetReadDeadline(time.Time) error  { return nil }
func (discardConn) SetWriteDeadline(time.Time) error { return nil }
