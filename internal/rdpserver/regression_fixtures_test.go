package rdpserver

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/input"
	rdpauth "github.com/rcarmo/go-rdp/pkg/auth"
)

func TestRegressionClientInfoTerminatorsFixture(t *testing.T) {
	fields := [][]byte{
		encodeClientInfoStringWithoutTerminator(""),
		encodeClientInfoStringWithoutTerminator("runner"),
		encodeClientInfoStringWithoutTerminator("secret"),
		encodeClientInfoStringWithoutTerminator(""),
		encodeClientInfoStringWithoutTerminator(""),
	}
	payload := make([]byte, 18)
	binary.LittleEndian.PutUint32(payload[4:8], 0x00000010)
	for i, field := range fields {
		binary.LittleEndian.PutUint16(payload[8+i*2:10+i*2], uint16(len(field)))
	}
	for _, field := range fields {
		payload = append(payload, field...)
		payload = append(payload, 0, 0)
	}
	info, err := parseClientInfo(payload)
	if err != nil {
		t.Fatal(err)
	}
	if info.UserName != "runner" || info.Password != "secret" {
		t.Fatalf("unexpected client info: %#v", info)
	}
}

func TestRegressionFastPathInputEquivalenceFixture(t *testing.T) {
	slowSink := &recordingInputSink{}
	fastSink := &recordingInputSink{}

	slowPayload := buildSlowPathInputPDU(
		buildSlowPathInputEvent(slowInputScanCode, 0, 0x1e, 0),
		buildSlowPathInputEvent(slowInputScanCode, slowKeyboardFlagRelease, 0x1e, 0),
		buildSlowPathInputEvent(slowInputMouse, slowPointerFlagMove, 20, 30),
	)
	fastPayload := []byte{
		byte(fastPathInputEventScanCode<<5) | 0x00, 0x1e,
		byte(fastPathInputEventScanCode<<5) | fastPathKeyboardFlagRelease, 0x1e,
		byte(fastPathInputEventMouse << 5),
	}
	fastPayload = appendLE16Bytes(fastPayload, slowPointerFlagMove)
	fastPayload = appendLE16Bytes(fastPayload, 20)
	fastPayload = appendLE16Bytes(fastPayload, 30)

	if err := dispatchSlowPathInput(slowPayload, slowSink); err != nil {
		t.Fatalf("slow path: %v", err)
	}
	if err := dispatchFastPathInput(byte(3<<2), fastPayload, fastSink); err != nil {
		t.Fatalf("fast path: %v", err)
	}
	assertInputSinksEqual(t, slowSink, fastSink)
}

func TestRegressionCredSSPServerNonceFixture(t *testing.T) {
	clientNonce := bytes.Repeat([]byte{0x33}, 32)
	serverNonce := bytes.Repeat([]byte{0x44}, 32)
	pubKey := []byte("subject-public-key")
	actual := rdpauth.ComputeClientPubKeyAuth(6, pubKey, serverNonce)
	matched, ok := matchClientPubKeyAuth(6, [][]byte{pubKey}, [][]byte{clientNonce, serverNonce}, actual)
	if !ok {
		t.Fatal("expected matched binding")
	}
	if !bytes.Equal(matched.Nonce, serverNonce) {
		t.Fatalf("expected server nonce binding, got %#v", matched)
	}
}

func TestRegressionDRDYNVCFragmentationFixture(t *testing.T) {
	sink := &recordingTouchSink{}
	m := newDRDYNVCManager([]clientChannel{{Name: "drdynvc", ID: 1004}}, sink)
	markDRDYNVCCapsForTest(m)
	markRDPEIChannelForTest(m, 9)

	rdpei := withRDPEIHeader(rdpeiEventTouch, rdpeiTouchEventPayloadForTest(1, 100, 200, rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact))
	first := buildDRDYNVCDataFirstPDUForTest(9, uint32(len(rdpei)), rdpei[:4])
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(first)); err != nil {
		t.Fatalf("data first: %v", err)
	}
	if len(m.fragments) != 1 {
		t.Fatalf("expected pending fragment, got %#v", m.fragments)
	}
	if err := m.handleStaticPDU(discardConn{}, buildStaticVirtualChannelPDU(buildDRDYNVCDataPDU(9, rdpei[4:]))); err != nil {
		t.Fatalf("data completion: %v", err)
	}
	if len(m.fragments) != 0 {
		t.Fatalf("fragments not cleared: %#v", m.fragments)
	}
	if len(sink.frames) != 1 || sink.frames[0][0].Flags&input.TouchDown == 0 {
		t.Fatalf("assembled RDPEI touch not dispatched: %#v", sink.frames)
	}
}
