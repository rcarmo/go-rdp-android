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
	if info.Capabilities.Bitmap.Present || info.Capabilities.Input.Present || info.Capabilities.Order.Present || info.Capabilities.VirtualChannel.Present || info.Capabilities.LargePointer.Present {
		t.Fatalf("unexpected parsed capabilities: %#v", info.Capabilities)
	}
}

func TestParseConfirmActiveCapabilitySummary(t *testing.T) {
	bitmap := capabilitySet(capTypeBitmap, buildBitmapCapability(1280, 720))
	input := capabilitySet(capTypeInput, buildInputCapability())
	order := capabilitySet(capTypeOrder, buildTestOrderCapabilityPayload(0x000a, 0x0004, 0x00020000))
	virtualChannel := capabilitySet(capTypeVirtualChannel, buildTestVirtualChannelCapabilityPayload(0x00000001, 1600))
	largePointer := capabilitySet(capTypeLargePointer, []byte{0x01, 0x00})
	pdu := buildTestConfirmActiveWithCapabilities(defaultShareID, defaultMCSUserID, bitmap, input, order, virtualChannel, largePointer)

	info, err := parseConfirmActive(pdu)
	if err != nil {
		t.Fatal(err)
	}
	if info.CapabilitySetCount != 5 {
		t.Fatalf("capability count = %d, want 5", info.CapabilitySetCount)
	}
	if !info.Capabilities.Bitmap.Present || info.Capabilities.Bitmap.DesktopWidth != 1280 || info.Capabilities.Bitmap.DesktopHeight != 720 || !info.Capabilities.Bitmap.DesktopResize {
		t.Fatalf("unexpected bitmap capability: %#v", info.Capabilities.Bitmap)
	}
	if !info.Capabilities.Input.Present || info.Capabilities.Input.Flags == 0 {
		t.Fatalf("unexpected input capability: %#v", info.Capabilities.Input)
	}
	if !info.Capabilities.Order.Present || info.Capabilities.Order.Flags != 0x000a || info.Capabilities.Order.SupportExFlags != 0x0004 || info.Capabilities.Order.DesktopSaveSize != 0x00020000 {
		t.Fatalf("unexpected order capability: %#v", info.Capabilities.Order)
	}
	if !info.Capabilities.VirtualChannel.Present || info.Capabilities.VirtualChannel.Flags != 0x00000001 || info.Capabilities.VirtualChannel.ChunkSize != 1600 {
		t.Fatalf("unexpected virtual channel capability: %#v", info.Capabilities.VirtualChannel)
	}
	if !info.Capabilities.LargePointer.Present || info.Capabilities.LargePointer.Flags != 0x0001 {
		t.Fatalf("unexpected large pointer capability: %#v", info.Capabilities.LargePointer)
	}
}

func TestParseConfirmActiveRejectsTruncatedCapability(t *testing.T) {
	broken := append([]byte{byte(capTypeBitmap), byte(capTypeBitmap >> 8), 0x10, 0x00}, []byte{0x20, 0x00, 0x01}...)
	pdu := buildTestConfirmActiveWithCapabilities(defaultShareID, defaultMCSUserID, broken)
	if _, err := parseConfirmActive(pdu); err == nil {
		t.Fatal("expected truncated capability error")
	}
}

func TestBuildMCSSendDataIndication(t *testing.T) {
	body := buildMCSSendDataIndication(serverChannelID, globalChannelID, []byte{1, 2, 3})
	if len(body) < 9 || body[0] != 0 || body[1] != 1 || body[4] != 0x70 {
		t.Fatalf("unexpected send data indication body: %x", body)
	}
}

func buildTestConfirmActiveWithCapabilities(shareID uint32, userID uint16, capabilitySets ...[]byte) []byte {
	source := []byte("TEST")
	caps := make([]byte, 0)
	for _, cap := range capabilitySets {
		caps = append(caps, cap...)
	}
	combinedCapsLen := 4 + len(caps)
	totalLength := 6 + 4 + 2 + 2 + 2 + len(source) + combinedCapsLen
	buf := make([]byte, 0, totalLength)
	pdu := appendShareControlHeader(buf, totalLength, pduTypeConfirmActive, userID)
	pdu = appendLE32(pdu, shareID)
	pdu = appendLE16(pdu, serverChannelID)
	pdu = appendLE16(pdu, uint16(len(source)))
	pdu = appendLE16(pdu, uint16(combinedCapsLen))
	pdu = append(pdu, source...)
	pdu = appendLE16(pdu, uint16(len(capabilitySets)))
	pdu = appendLE16(pdu, 0)
	pdu = append(pdu, caps...)
	return pdu
}

func buildTestOrderCapabilityPayload(orderFlags, supportExFlags uint16, desktopSaveSize uint32) []byte {
	payload := make([]byte, 84)
	binary.LittleEndian.PutUint16(payload[30:32], orderFlags)
	binary.LittleEndian.PutUint16(payload[66:68], supportExFlags)
	binary.LittleEndian.PutUint32(payload[72:76], desktopSaveSize)
	return payload
}

func buildTestVirtualChannelCapabilityPayload(flags, chunkSize uint32) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload[0:4], flags)
	binary.LittleEndian.PutUint32(payload[4:8], chunkSize)
	return payload
}
