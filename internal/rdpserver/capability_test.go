package rdpserver

import (
	"encoding/binary"
	"testing"

	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

func BenchmarkBuildServerCapabilityLeaves(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if len(buildGeneralCapability()) != 20 || len(buildPointerCapability()) != 6 || len(buildInputCapability()) != 88 || len(buildFontCapability()) != 4 || len(buildShareCapability()) != 4 {
			b.Fatal("bad capability length")
		}
	}
}

func BenchmarkBuildDemandActivePDU(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if len(buildDemandActivePDU(1280, 720)) != demandActivePDULen() {
			b.Fatal("bad Demand Active length")
		}
	}
}

func BenchmarkWriteDemandActive(b *testing.B) {
	conn := discardConn{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := writeDemandActive(conn, 1280, 720); err != nil {
			b.Fatal(err)
		}
	}
}

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
	surfaceCommands := capabilitySet(capTypeSurfaceCommands, buildTestSurfaceCommandsCapabilityPayload(0x00000052))
	bitmapCodecs := capabilitySet(capTypeBitmapCodecs, buildTestBitmapCodecsCapabilityPayload())
	pdu := buildTestConfirmActiveWithCapabilities(defaultShareID, defaultMCSUserID, bitmap, input, order, virtualChannel, largePointer, surfaceCommands, bitmapCodecs)

	info, err := parseConfirmActive(pdu)
	if err != nil {
		t.Fatal(err)
	}
	if info.CapabilitySetCount != 7 {
		t.Fatalf("capability count = %d, want 7", info.CapabilitySetCount)
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
	if !info.Capabilities.SurfaceCommands.Present || info.Capabilities.SurfaceCommands.Flags != 0x00000052 {
		t.Fatalf("unexpected surface commands capability: %#v", info.Capabilities.SurfaceCommands)
	}
	if !info.Capabilities.BitmapCodecs.Present || len(info.Capabilities.BitmapCodecs.Codecs) != 2 {
		t.Fatalf("unexpected bitmap codecs capability: %#v", info.Capabilities.BitmapCodecs)
	}
	if got := info.Capabilities.BitmapCodecs.Codecs[0]; got.Name != rdpcodec.BitmapCodecNameNSCodec || got.ID != 1 || got.PropertiesSize != 3 {
		t.Fatalf("unexpected NSCodec entry: %#v", got)
	}
	if got := info.Capabilities.BitmapCodecs.Codecs[1]; got.Name != rdpcodec.BitmapCodecNameJPEG || got.ID != 2 || got.PropertiesSize != 0 {
		t.Fatalf("unexpected JPEG entry: %#v", got)
	}
	if id, ok := info.Capabilities.BitmapCodecs.nsCodecID(); !ok || id != 1 {
		t.Fatalf("nsCodecID() = %d,%t", id, ok)
	}
	if id, ok := info.Capabilities.BitmapCodecs.jpegCodecID(); !ok || id != 2 {
		t.Fatalf("jpegCodecID() = %d,%t", id, ok)
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

func TestParseConfirmActiveCapabilitiesBitmapCodecsExtended(t *testing.T) {
	bitmapCodecs := capabilitySet(capTypeBitmapCodecs, buildTestBitmapCodecsCapabilityPayloadExtended())
	pdu := buildTestConfirmActiveWithCapabilities(defaultShareID+2, defaultMCSUserID+2, bitmapCodecs)
	info, err := parseConfirmActive(pdu)
	if err != nil {
		t.Fatalf("parseConfirmActive: %v", err)
	}
	if !info.Capabilities.BitmapCodecs.Present || len(info.Capabilities.BitmapCodecs.Codecs) != 5 {
		t.Fatalf("unexpected bitmap codecs capability: %#v", info.Capabilities.BitmapCodecs)
	}
	if id, ok := info.Capabilities.BitmapCodecs.nsCodecID(); !ok || id != 0x11 {
		t.Fatalf("nsCodecID = %d,%t, want 17,true", id, ok)
	}
	if id, ok := info.Capabilities.BitmapCodecs.jpegCodecID(); !ok || id != 0x12 {
		t.Fatalf("jpegCodecID = %d,%t, want 18,true", id, ok)
	}
	if id, ok := info.Capabilities.BitmapCodecs.remoteFXCodecID(); !ok || id != 0x13 {
		t.Fatalf("remoteFXCodecID = %d,%t, want 19,true", id, ok)
	}
	if id, ok := info.Capabilities.BitmapCodecs.remoteFXImageCodecID(); !ok || id != 0x15 {
		t.Fatalf("remoteFXImageCodecID = %d,%t, want 21,true", id, ok)
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

func buildTestSurfaceCommandsCapabilityPayload(flags uint32) []byte {
	payload := make([]byte, 8)
	binary.LittleEndian.PutUint32(payload[0:4], flags)
	return payload
}

func buildTestBitmapCodecsCapabilityPayload() []byte {
	payload := []byte{0x02}
	payload = append(payload, rdpcodec.NSCodecGUID[:]...)
	payload = append(payload, 0x01)
	payload = appendLE16(payload, 3)
	payload = append(payload, 1, 1, 3)
	payload = append(payload, rdpcodec.JPEGCodecGUID[:]...)
	payload = append(payload, 0x02)
	payload = appendLE16(payload, 0)
	return payload
}

func buildTestBitmapCodecsCapabilityPayloadExtended() []byte {
	payload := []byte{0x05}
	payload = append(payload, rdpcodec.NSCodecGUID[:]...)
	payload = append(payload, 0x11)
	payload = appendLE16(payload, 0)
	payload = append(payload, rdpcodec.JPEGCodecGUID[:]...)
	payload = append(payload, 0x12)
	payload = appendLE16(payload, 0)
	payload = append(payload, rdpcodec.RemoteFXGUID[:]...)
	payload = append(payload, 0x13)
	payload = appendLE16(payload, 0)
	// No known PNG bitmap-codec GUID in this implementation; keep operator override-only.
	unknown := [16]byte{0xaa, 0xbb, 0xcc, 0xdd, 0x10, 0x32, 0x54, 0x76, 0x98, 0xba, 0xdc, 0xfe, 0x11, 0x22, 0x33, 0x44}
	payload = append(payload, unknown[:]...)
	payload = append(payload, 0x14)
	payload = appendLE16(payload, 0)
	payload = append(payload, rdpcodec.RemoteFXImageGUID[:]...)
	payload = append(payload, 0x15)
	payload = appendLE16(payload, 0)
	return payload
}
