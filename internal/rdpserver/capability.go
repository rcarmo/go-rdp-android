package rdpserver

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

const (
	pduTypeDemandActive  = 0x0011
	pduTypeConfirmActive = 0x0013
	pduTypeDeactivateAll = 0x0016
	pduTypeData          = 0x0017

	pduType2Synchronize = 0x1f
	pduType2Control     = 0x14
	pduType2FontList    = 0x27
	pduType2FontMap     = 0x28

	capTypeGeneral        = 0x0001
	capTypeBitmap         = 0x0002
	capTypeOrder          = 0x0003
	capTypePointer        = 0x0008
	capTypeShare          = 0x0009
	capTypeInput          = 0x000d
	capTypeFont           = 0x000e
	capTypeVirtualChannel = 0x0014
	capTypeLargePointer   = 0x001b

	defaultShareID   = 0x000103ea
	serverChannelID  = 1002
	globalChannelID  = 1003
	serverSourceName = "MSTSC"
)

type shareControlPDU struct {
	TotalLength uint16
	PDUType     uint16
	PDUSource   uint16
	Payload     []byte
}

type confirmActiveInfo struct {
	ShareID            uint32
	OriginatorID       uint16
	SourceDescriptor   string
	CapabilitySetCount uint16
	Capabilities       confirmActiveCapabilities
}

type confirmActiveCapabilities struct {
	Bitmap         bitmapCapabilityInfo
	Input          inputCapabilityInfo
	Order          orderCapabilityInfo
	VirtualChannel virtualChannelCapabilityInfo
	LargePointer   largePointerCapabilityInfo
}

type bitmapCapabilityInfo struct {
	Present               bool
	PreferredBitsPerPixel uint16
	DesktopWidth          uint16
	DesktopHeight         uint16
	DesktopResize         bool
}

type inputCapabilityInfo struct {
	Present bool
	Flags   uint16
}

type orderCapabilityInfo struct {
	Present         bool
	Flags           uint16
	SupportExFlags  uint16
	DesktopSaveSize uint32
}

type virtualChannelCapabilityInfo struct {
	Present   bool
	Flags     uint32
	ChunkSize uint32
}

type largePointerCapabilityInfo struct {
	Present bool
	Flags   uint16
}

func writeDemandActive(conn net.Conn, width, height int) error {
	pdu := buildDemandActivePDU(width, height)
	body := buildMCSSendDataIndication(serverChannelID, globalChannelID, pdu)
	return writeMCSDomainPDU(conn, mcsSendDataIndicationApp, body)
}

func buildDemandActivePDU(width, height int) []byte {
	caps := buildServerCapabilitySets(width, height)
	combinedCapsLen := 4 + len(caps)
	source := []byte(serverSourceName)
	totalLength := 6 + 4 + 2 + 2 + len(source) + combinedCapsLen + 4

	buf := new(bytes.Buffer)
	writeShareControlHeader(buf, totalLength, pduTypeDemandActive, serverChannelID)
	_ = binary.Write(buf, binary.LittleEndian, uint32(defaultShareID))
	_ = binary.Write(buf, binary.LittleEndian, uint16(len(source)))
	_ = binary.Write(buf, binary.LittleEndian, uint16(combinedCapsLen))
	buf.Write(source)
	_ = binary.Write(buf, binary.LittleEndian, uint16(6)) // capability count
	_ = binary.Write(buf, binary.LittleEndian, uint16(0)) // pad2Octets
	buf.Write(caps)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // sessionId
	return buf.Bytes()
}

func buildServerCapabilitySets(width, height int) []byte {
	buf := new(bytes.Buffer)
	buf.Write(capabilitySet(capTypeGeneral, buildGeneralCapability()))
	buf.Write(capabilitySet(capTypeBitmap, buildBitmapCapability(width, height)))
	buf.Write(capabilitySet(capTypePointer, buildPointerCapability()))
	buf.Write(capabilitySet(capTypeInput, buildInputCapability()))
	buf.Write(capabilitySet(capTypeFont, buildFontCapability()))
	buf.Write(capabilitySet(capTypeShare, buildShareCapability()))
	return buf.Bytes()
}

func capabilitySet(capType uint16, payload []byte) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, capType)
	_ = binary.Write(buf, binary.LittleEndian, uint16(4+len(payload)))
	buf.Write(payload)
	return buf.Bytes()
}

func buildGeneralCapability() []byte {
	buf := new(bytes.Buffer)
	for _, v := range []uint16{
		1,      // osMajorType: OSMAJORTYPE_WINDOWS
		3,      // osMinorType: OSMINORTYPE_WINDOWS_NT
		0x0200, // protocolVersion
		0,      // pad2octetsA
		0,      // generalCompressionTypes
		0,      // extraFlags
		0,      // updateCapabilityFlag
		0,      // remoteUnshareFlag
		0,      // generalCompressionLevel
	} {
		_ = binary.Write(buf, binary.LittleEndian, v)
	}
	_ = binary.Write(buf, binary.LittleEndian, uint8(0)) // refreshRectSupport
	_ = binary.Write(buf, binary.LittleEndian, uint8(0)) // suppressOutputSupport
	return buf.Bytes()
}

func buildBitmapCapability(width, height int) []byte {
	buf := new(bytes.Buffer)
	for _, v := range []uint16{
		32,             // preferredBitsPerPixel
		1,              // receive1BitPerPixel
		1,              // receive4BitsPerPixel
		1,              // receive8BitsPerPixel
		uint16(width),  // desktopWidth
		uint16(height), // desktopHeight
		0,              // pad2octets
		1,              // desktopResizeFlag
		1,              // bitmapCompressionFlag
	} {
		_ = binary.Write(buf, binary.LittleEndian, v)
	}
	_ = binary.Write(buf, binary.LittleEndian, uint8(0))  // highColorFlags
	_ = binary.Write(buf, binary.LittleEndian, uint8(0))  // drawingFlags
	_ = binary.Write(buf, binary.LittleEndian, uint16(1)) // multipleRectangleSupport
	_ = binary.Write(buf, binary.LittleEndian, uint16(0)) // pad2octetsB
	return buf.Bytes()
}

func buildPointerCapability() []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))  // colorPointerFlag
	_ = binary.Write(buf, binary.LittleEndian, uint16(24)) // colorPointerCacheSize
	_ = binary.Write(buf, binary.LittleEndian, uint16(24)) // pointerCacheSize
	return buf.Bytes()
}

func buildInputCapability() []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint16(0x0001|0x0004|0x0008)) // keyboard + unicode + mouseX
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // keyboardLayout
	_ = binary.Write(buf, binary.LittleEndian, uint32(4)) // keyboardType
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // keyboardSubType
	_ = binary.Write(buf, binary.LittleEndian, uint32(12))
	buf.Write(make([]byte, 64)) // imeFileName
	return buf.Bytes()
}

func buildFontCapability() []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	return buf.Bytes()
}

func buildShareCapability() []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint16(serverChannelID))
	_ = binary.Write(buf, binary.LittleEndian, uint16(0))
	return buf.Bytes()
}

func buildMCSSendDataIndication(initiator, channelID uint16, data []byte) []byte {
	body := encodePERInteger16(initiator, defaultMCSUserID)
	body = append(body, encodePERInteger16(channelID, 0)...)
	body = append(body, 0x70)
	body = append(body, encodePERLength(len(data))...)
	body = append(body, data...)
	return body
}

func parseShareControlPDU(data []byte) (*shareControlPDU, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("short share control PDU")
	}
	pdu := &shareControlPDU{
		TotalLength: binary.LittleEndian.Uint16(data[0:2]),
		PDUType:     binary.LittleEndian.Uint16(data[2:4]),
		PDUSource:   binary.LittleEndian.Uint16(data[4:6]),
	}
	if int(pdu.TotalLength) > len(data) {
		return nil, fmt.Errorf("share control length %d exceeds available %d", pdu.TotalLength, len(data))
	}
	pdu.Payload = data[6:pdu.TotalLength]
	return pdu, nil
}

func parseConfirmActive(data []byte) (*confirmActiveInfo, error) {
	share, err := parseShareControlPDU(data)
	if err != nil {
		return nil, err
	}
	if share.PDUType != pduTypeConfirmActive {
		return nil, fmt.Errorf("not Confirm Active PDU: 0x%04x", share.PDUType)
	}
	if len(share.Payload) < 10 {
		return nil, fmt.Errorf("short Confirm Active payload")
	}
	info := &confirmActiveInfo{
		ShareID:      binary.LittleEndian.Uint32(share.Payload[0:4]),
		OriginatorID: binary.LittleEndian.Uint16(share.Payload[4:6]),
	}
	sourceLen := int(binary.LittleEndian.Uint16(share.Payload[6:8]))
	combinedCapsLen := int(binary.LittleEndian.Uint16(share.Payload[8:10]))
	if len(share.Payload) < 10+sourceLen+4 || len(share.Payload) < 10+sourceLen+combinedCapsLen {
		return nil, fmt.Errorf("short Confirm Active variable payload")
	}
	sourceEnd := 10 + sourceLen
	capsEnd := 10 + sourceLen + combinedCapsLen
	info.SourceDescriptor = string(share.Payload[10:sourceEnd])
	info.CapabilitySetCount = binary.LittleEndian.Uint16(share.Payload[sourceEnd : sourceEnd+2])
	caps, err := parseConfirmActiveCapabilities(share.Payload[sourceEnd+4:capsEnd], info.CapabilitySetCount)
	if err != nil {
		return nil, err
	}
	info.Capabilities = caps
	return info, nil
}

func parseConfirmActiveCapabilities(data []byte, declaredCount uint16) (confirmActiveCapabilities, error) {
	var caps confirmActiveCapabilities
	if declaredCount == 0 {
		return caps, nil
	}
	off := 0
	parsed := 0
	for off+4 <= len(data) && parsed < int(declaredCount) {
		capType := binary.LittleEndian.Uint16(data[off : off+2])
		capLen := int(binary.LittleEndian.Uint16(data[off+2 : off+4]))
		if capLen < 4 {
			return caps, fmt.Errorf("invalid Confirm Active capability length %d for type 0x%04x", capLen, capType)
		}
		if off+capLen > len(data) {
			return caps, fmt.Errorf("truncated Confirm Active capability type 0x%04x length=%d remaining=%d", capType, capLen, len(data)-off)
		}
		payload := data[off+4 : off+capLen]
		switch capType {
		case capTypeBitmap:
			parseBitmapCapability(payload, &caps)
		case capTypeInput:
			parseInputCapability(payload, &caps)
		case capTypeOrder:
			parseOrderCapability(payload, &caps)
		case capTypeVirtualChannel:
			parseVirtualChannelCapability(payload, &caps)
		case capTypeLargePointer:
			parseLargePointerCapability(payload, &caps)
		}
		off += capLen
		parsed++
	}
	if parsed < int(declaredCount) {
		return caps, fmt.Errorf("short Confirm Active capability payload: parsed=%d declared=%d", parsed, declaredCount)
	}
	return caps, nil
}

func parseBitmapCapability(payload []byte, caps *confirmActiveCapabilities) {
	if len(payload) < 16 {
		return
	}
	caps.Bitmap.Present = true
	caps.Bitmap.PreferredBitsPerPixel = binary.LittleEndian.Uint16(payload[0:2])
	caps.Bitmap.DesktopWidth = binary.LittleEndian.Uint16(payload[8:10])
	caps.Bitmap.DesktopHeight = binary.LittleEndian.Uint16(payload[10:12])
	caps.Bitmap.DesktopResize = binary.LittleEndian.Uint16(payload[14:16]) != 0
}

func parseInputCapability(payload []byte, caps *confirmActiveCapabilities) {
	if len(payload) < 2 {
		return
	}
	caps.Input.Present = true
	caps.Input.Flags = binary.LittleEndian.Uint16(payload[0:2])
}

func parseOrderCapability(payload []byte, caps *confirmActiveCapabilities) {
	if len(payload) < 32 {
		return
	}
	caps.Order.Present = true
	caps.Order.Flags = binary.LittleEndian.Uint16(payload[30:32])
	if len(payload) >= 68 {
		caps.Order.SupportExFlags = binary.LittleEndian.Uint16(payload[66:68])
	}
	if len(payload) >= 76 {
		caps.Order.DesktopSaveSize = binary.LittleEndian.Uint32(payload[72:76])
	}
}

func parseVirtualChannelCapability(payload []byte, caps *confirmActiveCapabilities) {
	if len(payload) < 4 {
		return
	}
	caps.VirtualChannel.Present = true
	caps.VirtualChannel.Flags = binary.LittleEndian.Uint32(payload[0:4])
	if len(payload) >= 8 {
		caps.VirtualChannel.ChunkSize = binary.LittleEndian.Uint32(payload[4:8])
	}
}

func parseLargePointerCapability(payload []byte, caps *confirmActiveCapabilities) {
	if len(payload) < 2 {
		return
	}
	caps.LargePointer.Present = true
	caps.LargePointer.Flags = binary.LittleEndian.Uint16(payload[0:2])
}

func writeShareControlHeader(buf *bytes.Buffer, totalLength int, pduType uint16, source uint16) {
	_ = binary.Write(buf, binary.LittleEndian, uint16(totalLength))
	_ = binary.Write(buf, binary.LittleEndian, pduType)
	_ = binary.Write(buf, binary.LittleEndian, source)
}
