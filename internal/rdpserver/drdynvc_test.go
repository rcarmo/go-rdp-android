package rdpserver

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/input"
)

func TestNewDRDYNVCManagerFindsStaticChannel(t *testing.T) {
	m := newDRDYNVCManager([]clientChannel{{Name: "cliprdr", ID: 1004}, {Name: "drdynvc", ID: 1005}}, nil, serverMetrics{})
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

func TestDRDYNVCVariableLengthChannelIDs(t *testing.T) {
	for _, channelID := range []uint32{0xff, 0x0100, 0xffff, 0x00010000, 0x01020304} {
		data := []byte{1, 2, 3}
		pdu, err := parseDRDYNVCPDU(buildDRDYNVCDataPDU(channelID, data))
		if err != nil {
			t.Fatalf("parse channel %d: %v", channelID, err)
		}
		if pdu.ChannelID != channelID || !bytes.Equal(pdu.Data, data) {
			t.Fatalf("unexpected PDU for channel %d: %#v", channelID, pdu)
		}
		response := buildDRDYNVCCreateResponsePDU(channelID, drdynvcCreateOK)
		if got := (drdynvcHeader{CbChID: response[0] & 0x03, Cmd: (response[0] >> 4) & 0x0f}); got.CbChID != drdynvcCbChID(channelID) || got.Cmd != drdynvcCmdCreate {
			t.Fatalf("unexpected create response header for channel %d: %#v", channelID, got)
		}
	}
}

func TestDRDYNVCRDPEITouchClientSequenceIntegration(t *testing.T) {
	var dvcFragments atomic.Int64
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink, serverMetrics{dvcFragments: &dvcFragments})

	capsServer, capsClient := net.Pipe()
	defer capsServer.Close()
	defer capsClient.Close()
	capsDone := make(chan error, 1)
	go func() {
		capsDone <- m.handleStaticPDU(capsServer, buildStaticVirtualChannelPDU(buildDRDYNVCCapsPDU(drdynvcCapsVersion1)))
	}()
	capsResponse := readTestDomainPDUFromPipe(t, capsClient)
	capsStatic, err := parseStaticVirtualChannelPDU(capsResponse.Data)
	if err != nil {
		t.Fatalf("parse caps static response: %v", err)
	}
	capsDVC, err := parseDRDYNVCPDU(capsStatic.Data)
	if err != nil {
		t.Fatalf("parse caps response: %v", err)
	}
	if capsDVC.Header.Cmd != drdynvcCmdCapability || capsDVC.Version != drdynvcCapsVersion1 || !m.capsReceived {
		t.Fatalf("unexpected caps state response=%#v manager=%#v", capsDVC, m)
	}
	waitForDRDYNVCTestHandler(t, capsDone, "caps")

	createServer, createClient := net.Pipe()
	defer createServer.Close()
	defer createClient.Close()
	create := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize(), 7}
	create = append(create, []byte(rdpeiDynamicChannelName)...)
	create = append(create, 0)
	createDone := make(chan error, 1)
	go func() { createDone <- m.handleStaticPDU(createServer, buildStaticVirtualChannelPDU(create)) }()
	createResponse := readTestDomainPDUFromPipe(t, createClient)
	createStatic, err := parseStaticVirtualChannelPDU(createResponse.Data)
	if err != nil {
		t.Fatalf("parse create static response: %v", err)
	}
	if code := binary.LittleEndian.Uint32(createStatic.Data[2:6]); code != drdynvcCreateOK {
		t.Fatalf("create response code = 0x%x", code)
	}
	scReadyResponse := readTestDomainPDUFromPipe(t, createClient)
	scReadyStatic, err := parseStaticVirtualChannelPDU(scReadyResponse.Data)
	if err != nil {
		t.Fatalf("parse SC_READY static response: %v", err)
	}
	scReadyDVC, err := parseDRDYNVCPDU(scReadyStatic.Data)
	if err != nil {
		t.Fatalf("parse SC_READY DVC: %v", err)
	}
	if scReadyDVC.ChannelID != 7 {
		t.Fatalf("SC_READY channel = %d", scReadyDVC.ChannelID)
	}
	if rdpei, err := parseRDPEIPDU(scReadyDVC.Data); err != nil || rdpei.SCReady == nil {
		t.Fatalf("expected SC_READY, got %#v err=%v", rdpei, err)
	}
	waitForDRDYNVCTestHandler(t, createDone, "create")

	csReady := make([]byte, 10)
	binary.LittleEndian.PutUint32(csReady[0:4], rdpeiCSReadyShowTouchVisuals)
	binary.LittleEndian.PutUint32(csReady[4:8], rdpeiProtocolV300)
	binary.LittleEndian.PutUint16(csReady[8:10], 5)
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCDataPDU(7, withRDPEIHeader(rdpeiEventCSReady, csReady)))); err != nil {
		t.Fatalf("CS_READY: %v", err)
	}

	down := withRDPEIHeader(rdpeiEventTouch, rdpeiTouchEventPayloadForTest(2, 100, 200, rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact))
	first := buildDRDYNVCDataFirstPDUForTest(7, uint32(len(down)), down[:5])
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(first)); err != nil {
		t.Fatalf("touch down data-first: %v", err)
	}
	if len(m.fragments) != 1 {
		t.Fatalf("expected one pending fragment: %#v", m.fragments)
	}
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCDataPDU(7, down[5:]))); err != nil {
		t.Fatalf("touch down completion: %v", err)
	}
	up := withRDPEIHeader(rdpeiEventTouch, rdpeiTouchEventPayloadForTest(2, 105, 205, rdpeiContactFlagUp|rdpeiContactFlagInRange))
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCDataPDU(7, up))); err != nil {
		t.Fatalf("touch up: %v", err)
	}
	if len(m.fragments) != 0 {
		t.Fatalf("fragments not cleared after integration sequence: %#v", m.fragments)
	}
	if dvcFragments.Load() != 2 {
		t.Fatalf("expected two DVC fragments, got %d", dvcFragments.Load())
	}
	if len(sink.frames) != 2 || sink.frames[0][0].Flags&input.TouchDown == 0 || sink.frames[1][0].Flags&input.TouchUp == 0 {
		t.Fatalf("expected down/up touch frames, got %#v", sink.frames)
	}
}

func TestDRDYNVCManagerCreateRDPEIWritesResponseAndSCReady(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	markDRDYNVCCapsForTest(m)
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
	if !m.hasRDPEIChannel || m.rdpeiChannelID != 7 || m.channels[7] != rdpeiDynamicChannelName {
		t.Fatalf("RDPEI channel not tracked: %#v", m)
	}
}

func TestDRDYNVCManagerRejectsDuplicateCreate(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 7)
	create := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize(), 7}
	create = append(create, []byte(rdpeiDynamicChannelName)...)
	create = append(create, 0)

	done := make(chan error, 1)
	go func() { done <- m.handleStaticPDU(server, buildStaticVirtualChannelPDU(create)) }()
	pdu := readTestDomainPDUFromPipe(t, client)
	staticPDU, err := parseStaticVirtualChannelPDU(pdu.Data)
	if err != nil {
		t.Fatalf("parse static duplicate response: %v", err)
	}
	if len(staticPDU.Data) != 6 {
		t.Fatalf("unexpected duplicate response length: %x", staticPDU.Data)
	}
	if code := binary.LittleEndian.Uint32(staticPDU.Data[2:6]); code != drdynvcCreateAlreadyExists {
		t.Fatalf("duplicate create code = 0x%x", code)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("duplicate create: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("duplicate create did not return")
	}
	if !m.hasRDPEIChannel || m.rdpeiChannelID != 7 || len(m.channels) != 1 {
		t.Fatalf("duplicate create disturbed existing state: %#v", m)
	}
}

func TestDRDYNVCManagerRequiresCapsBeforeLifecycleCommands(t *testing.T) {
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	create := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize(), 7}
	create = append(create, []byte(rdpeiDynamicChannelName)...)
	create = append(create, 0)
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(create)); err == nil {
		t.Fatal("expected create before caps to fail")
	}
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCDataPDU(7, []byte{1}))); err == nil {
		t.Fatal("expected data before caps to fail")
	}
	closePDU := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdClose}).serialize(), 7}
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(closePDU)); err == nil {
		t.Fatal("expected close before caps to fail")
	}
}

func TestDRDYNVCManagerRejectsUnsupportedCapsVersion(t *testing.T) {
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCCapsPDU(0))); err == nil {
		t.Fatal("expected unsupported caps version error")
	}
	if m.capsReceived || m.negotiatedCapsVersion != 0 {
		t.Fatalf("unsupported caps should not update state: %#v", m)
	}
}

func TestDRDYNVCManagerRejectsSecondRDPEIChannel(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 7)
	create := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize(), 8}
	create = append(create, []byte(rdpeiDynamicChannelName)...)
	create = append(create, 0)

	done := make(chan error, 1)
	go func() { done <- m.handleStaticPDU(server, buildStaticVirtualChannelPDU(create)) }()
	pdu := readTestDomainPDUFromPipe(t, client)
	staticPDU, err := parseStaticVirtualChannelPDU(pdu.Data)
	if err != nil {
		t.Fatalf("parse second RDPEI response: %v", err)
	}
	if code := binary.LittleEndian.Uint32(staticPDU.Data[2:6]); code != drdynvcCreateAlreadyExists {
		t.Fatalf("second RDPEI create code = 0x%x", code)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second RDPEI create: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("second RDPEI create did not return")
	}
	if !m.hasRDPEIChannel || m.rdpeiChannelID != 7 || len(m.channels) != 1 {
		t.Fatalf("second RDPEI create disturbed existing state: %#v", m)
	}
}

func TestDRDYNVCManagerRejectsUnsupportedChannel(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	create := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize(), 12}
	create = append(create, []byte("unsupported")...)
	create = append(create, 0)

	done := make(chan error, 1)
	go func() { done <- m.handleStaticPDU(server, buildStaticVirtualChannelPDU(create)) }()
	pdu := readTestDomainPDUFromPipe(t, client)
	staticPDU, err := parseStaticVirtualChannelPDU(pdu.Data)
	if err != nil {
		t.Fatalf("parse unsupported response: %v", err)
	}
	if code := binary.LittleEndian.Uint32(staticPDU.Data[2:6]); code != drdynvcCreateNoListener {
		t.Fatalf("unsupported create code = 0x%x", code)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unsupported create: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("unsupported create did not return")
	}
	if m.hasRDPEIChannel || len(m.channels) != 0 {
		t.Fatalf("unsupported create changed state: %#v", m)
	}
}

func TestDRDYNVCManagerCloseAndReopenRDPEI(t *testing.T) {
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 9)
	m.touchLifecycle.ApplyFrame([]input.TouchContact{{ID: 1, X: 1, Y: 2, Flags: input.TouchDown}})
	if err := m.handleDynamicDataFirst(9, 2, []byte{1}); err != nil {
		t.Fatalf("fragment: %v", err)
	}
	closePDU := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdClose}).serialize(), 9}
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(closePDU)); err != nil {
		t.Fatalf("close: %v", err)
	}
	if m.hasRDPEIChannel || m.rdpeiChannelID != 0 || len(m.channels) != 0 || len(m.fragments) != 0 || m.touchLifecycle.ActiveCount() != 0 {
		t.Fatalf("close did not clear RDPEI state: %#v", m)
	}

	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	create := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdCreate}).serialize(), 9}
	create = append(create, []byte(rdpeiDynamicChannelName)...)
	create = append(create, 0)
	done := make(chan error, 1)
	go func() { done <- m.handleStaticPDU(server, buildStaticVirtualChannelPDU(create)) }()
	_ = readTestDomainPDUFromPipe(t, client)
	_ = readTestDomainPDUFromPipe(t, client)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("reopen: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("reopen did not return")
	}
	if !m.hasRDPEIChannel || m.rdpeiChannelID != 9 || m.channels[9] != rdpeiDynamicChannelName {
		t.Fatalf("RDPEI channel not reopened: %#v", m)
	}
}

func TestDRDYNVCManagerHandlesRDPEIData(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 9)
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

func TestDRDYNVCManagerPreservesRDPEIOptionalContactMetadata(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 9)
	payload := rdpeiTouchEventPayloadWithOptionalFieldsForTest(8, 123, 456, rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact)
	data := buildDRDYNVCDataPDU(9, withRDPEIHeader(rdpeiEventTouch, payload))
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(data)); err != nil {
		t.Fatalf("handle optional metadata: %v", err)
	}
	if len(sink.frames) != 1 || len(sink.frames[0]) != 1 {
		t.Fatalf("expected one touch frame, got %#v", sink.frames)
	}
	contact := sink.frames[0][0]
	if contact.Rect == nil || *contact.Rect != (input.TouchRect{Left: -4, Top: -5, Right: 6, Bottom: 7}) {
		t.Fatalf("optional rect was not preserved: %#v", contact.Rect)
	}
	if contact.Orientation == nil || *contact.Orientation != 45 {
		t.Fatalf("optional orientation was not preserved: %#v", contact.Orientation)
	}
	if contact.Pressure == nil || *contact.Pressure != 512 {
		t.Fatalf("optional pressure was not preserved: %#v", contact.Pressure)
	}
}

func TestDRDYNVCManagerDropsStrayTouchLifecycleEvents(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 9)
	strayUpdate := rdpeiTouchEventPayloadForTest(4, 11, 22, rdpeiContactFlagUpdate|rdpeiContactFlagInRange|rdpeiContactFlagInContact)
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCDataPDU(9, withRDPEIHeader(rdpeiEventTouch, strayUpdate)))); err != nil {
		t.Fatalf("handle stray update: %v", err)
	}
	if len(sink.frames) != 0 {
		t.Fatalf("stray update should not dispatch: %#v", sink.frames)
	}
	down := rdpeiTouchEventPayloadForTest(4, 11, 22, rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact)
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCDataPDU(9, withRDPEIHeader(rdpeiEventTouch, down)))); err != nil {
		t.Fatalf("handle down: %v", err)
	}
	up := rdpeiTouchEventPayloadForTest(4, 12, 23, rdpeiContactFlagUp|rdpeiContactFlagInRange)
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCDataPDU(9, withRDPEIHeader(rdpeiEventTouch, up)))); err != nil {
		t.Fatalf("handle up: %v", err)
	}
	if len(sink.frames) != 2 || sink.frames[0][0].Flags&input.TouchDown == 0 || sink.frames[1][0].Flags&input.TouchUp == 0 {
		t.Fatalf("expected down/up after stray update, got %#v", sink.frames)
	}
}

func TestDRDYNVCManagerRejectsUnexpectedDataChannel(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 9)
	data := buildDRDYNVCDataPDU(10, withRDPEIHeader(rdpeiEventTouch, rdpeiTouchEventPayloadForTest(1, 10, 20, rdpeiContactFlagDown)))
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(data)); err == nil {
		t.Fatal("expected unopened data channel error")
	}
	if len(sink.frames) != 0 {
		t.Fatalf("unexpected channel should not dispatch touch frames: %#v", sink.frames)
	}
}

func TestDRDYNVCManagerHandlesMultipleSimultaneousFragments(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 9)
	m.channels[10] = "other"
	rdpei := withRDPEIHeader(rdpeiEventTouch, rdpeiTouchEventPayloadForTest(1, 10, 20, rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact))
	other := []byte{1, 2, 3, 4}
	if err := m.handleDynamicDataFirst(9, uint32(len(rdpei)), rdpei[:3]); err != nil {
		t.Fatalf("rdpei data-first: %v", err)
	}
	if err := m.handleDynamicDataFirst(10, uint32(len(other)), other[:2]); err != nil {
		t.Fatalf("other data-first: %v", err)
	}
	if len(m.fragments) != 2 {
		t.Fatalf("expected two pending fragments: %#v", m.fragments)
	}
	if err := m.handleDynamicData(10, other[2:]); err != nil {
		t.Fatalf("other completion: %v", err)
	}
	if len(sink.frames) != 0 || len(m.fragments) != 1 {
		t.Fatalf("other channel should complete without RDPEI dispatch frames=%#v fragments=%#v", sink.frames, m.fragments)
	}
	if err := m.handleDynamicData(9, rdpei[3:]); err != nil {
		t.Fatalf("rdpei completion: %v", err)
	}
	if len(m.fragments) != 0 || len(sink.frames) != 1 {
		t.Fatalf("RDPEI channel should dispatch and clear fragments frames=%#v fragments=%#v", sink.frames, m.fragments)
	}
}

func TestDRDYNVCDataFirstAssembly(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 9)
	rdpei := withRDPEIHeader(rdpeiEventTouch, rdpeiTouchEventPayloadForTest(1, 100, 200, rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact))
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
	if len(sink.frames) != 1 || sink.frames[0][0].Flags&input.TouchDown == 0 {
		t.Fatalf("assembled RDPEI touch was not dispatched: %#v", sink.frames)
	}
}

func TestDRDYNVCSizeBounds(t *testing.T) {
	if _, err := parseStaticVirtualChannelPDU(staticVirtualChannelHeaderForTest(drdynvcMaxStaticPayload+1, channelFlagFirst|channelFlagLast)); err == nil {
		t.Fatal("expected oversized static virtual channel length error")
	}
	if _, err := parseDRDYNVCPDU(make([]byte, drdynvcMaxPDUSize+1)); err == nil {
		t.Fatal("expected oversized drdynvc PDU error")
	}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	if err := m.handleDynamicDataFirst(1, drdynvcMaxFragmentSize+1, []byte{1}); err == nil {
		t.Fatal("expected oversized data-first length error")
	}
}

func TestDRDYNVCFragmentLimitAndCleanup(t *testing.T) {
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	for i := uint32(1); i <= drdynvcMaxFragments; i++ {
		if err := m.handleDynamicDataFirst(i, 2, []byte{1}); err != nil {
			t.Fatalf("fragment %d: %v", i, err)
		}
	}
	if err := m.handleDynamicDataFirst(99, 2, []byte{1}); err == nil {
		t.Fatal("expected pending fragment limit error")
	}
	for _, frag := range m.fragments {
		frag.updatedAt = time.Now().Add(-drdynvcFragmentTTL - time.Second)
	}
	m.cleanupFragments(time.Now())
	if len(m.fragments) != 0 {
		t.Fatalf("expected stale fragments to be cleaned up: %#v", m.fragments)
	}
	if err := m.handleDynamicDataFirst(99, 2, []byte{1}); err != nil {
		t.Fatalf("fragment after cleanup: %v", err)
	}
}

func TestDRDYNVCCloseClearsNonRDPEIFragment(t *testing.T) {
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, nil, serverMetrics{})
	markDRDYNVCCapsForTest(m)
	m.channels[11] = "other"
	if err := m.handleDynamicDataFirst(11, 2, []byte{1}); err != nil {
		t.Fatalf("fragment: %v", err)
	}
	closePDU := []byte{(drdynvcHeader{CbChID: 0, Cmd: drdynvcCmdClose}).serialize(), 11}
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(closePDU)); err != nil {
		t.Fatalf("close: %v", err)
	}
	if len(m.channels) != 0 || len(m.fragments) != 0 {
		t.Fatalf("close did not clear non-RDPEI state: channels=%#v fragments=%#v", m.channels, m.fragments)
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

func waitForDRDYNVCTestHandler(t *testing.T, done <-chan error, name string) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s handler: %v", name, err)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s handler did not return", name)
	}
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

func staticVirtualChannelHeaderForTest(length uint32, flags uint32) []byte {
	out := make([]byte, 8)
	binary.LittleEndian.PutUint32(out[0:4], length)
	binary.LittleEndian.PutUint32(out[4:8], flags)
	return out
}

func markDRDYNVCCapsForTest(m *drdynvcManager) {
	m.capsReceived = true
	m.negotiatedCapsVersion = drdynvcCapsVersion1
}

func markRDPEIChannelForTest(m *drdynvcManager, channelID uint32) {
	m.channels[channelID] = rdpeiDynamicChannelName
	m.hasRDPEIChannel = true
	m.rdpeiChannelID = channelID
}

func rdpeiTouchEventPayloadForTest(contactID uint8, x, y int32, flags uint32) []byte {
	return rdpeiTouchEventPayloadWithFieldsForTest(contactID, x, y, flags, 0, nil)
}

func rdpeiTouchEventPayloadWithOptionalFieldsForTest(contactID uint8, x, y int32, flags uint32) []byte {
	optional := []byte{}
	optional = append(optional, rdpeiTestVarInt16(-4)...)
	optional = append(optional, rdpeiTestVarInt16(-5)...)
	optional = append(optional, rdpeiTestVarInt16(6)...)
	optional = append(optional, rdpeiTestVarInt16(7)...)
	optional = append(optional, rdpeiTestVarUint32(45)...)
	optional = append(optional, rdpeiTestVarUint32(512)...)
	return rdpeiTouchEventPayloadWithFieldsForTest(contactID, x, y, flags, rdpeiTouchContactRectPresent|rdpeiTouchContactOrientationPresent|rdpeiTouchContactPressurePresent, optional)
}

func rdpeiTouchEventPayloadWithFieldsForTest(contactID uint8, x, y int32, flags uint32, fieldsPresent uint16, optional []byte) []byte {
	payload := []byte{}
	payload = append(payload, rdpeiTestVarUint32(0)...)
	payload = append(payload, rdpeiTestVarUint16(1)...)
	payload = append(payload, rdpeiTestVarUint16(1)...)
	payload = append(payload, rdpeiTestVarUint64(0)...)
	payload = append(payload, contactID)
	payload = append(payload, rdpeiTestVarUint16(fieldsPresent)...)
	payload = append(payload, rdpeiTestVarInt32(x)...)
	payload = append(payload, rdpeiTestVarInt32(y)...)
	payload = append(payload, rdpeiTestVarUint32(flags)...)
	payload = append(payload, optional...)
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
