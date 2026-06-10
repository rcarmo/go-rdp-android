package rdpserver

import (
	"encoding/binary"
	"fmt"
	"net"

	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
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

	capTypeGeneral         = 0x0001
	capTypeBitmap          = 0x0002
	capTypeOrder           = 0x0003
	capTypePointer         = 0x0008
	capTypeShare           = 0x0009
	capTypeInput           = 0x000d
	capTypeFont            = 0x000e
	capTypeVirtualChannel  = 0x0014
	capTypeLargePointer    = 0x001b
	capTypeSurfaceCommands = 0x001c
	capTypeBitmapCodecs    = 0x001d

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
	Bitmap          bitmapCapabilityInfo
	Input           inputCapabilityInfo
	Order           orderCapabilityInfo
	VirtualChannel  virtualChannelCapabilityInfo
	LargePointer    largePointerCapabilityInfo
	SurfaceCommands surfaceCommandsCapabilityInfo
	BitmapCodecs    bitmapCodecsCapabilityInfo
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

type surfaceCommandsCapabilityInfo struct {
	Present bool
	Flags   uint32
}

type bitmapCodecsCapabilityInfo struct {
	Present bool
	Codecs  []bitmapCodecInfo
}

type bitmapCodecInfo struct {
	GUID           [16]byte
	ID             byte
	Name           string
	PropertiesSize uint16
}

func (c bitmapCodecsCapabilityInfo) codecIDByName(name string) (byte, bool) {
	for _, codec := range c.Codecs {
		if codec.Name == name && codec.ID != 0 {
			return codec.ID, true
		}
	}
	return 0, false
}

func (c bitmapCodecsCapabilityInfo) nsCodecID() (byte, bool) {
	return c.codecIDByName(rdpcodec.BitmapCodecNameNSCodec)
}

func (c bitmapCodecsCapabilityInfo) jpegCodecID() (byte, bool) {
	return c.codecIDByName(rdpcodec.BitmapCodecNameJPEG)
}

func (c bitmapCodecsCapabilityInfo) remoteFXCodecID() (byte, bool) {
	return c.codecIDByName(rdpcodec.BitmapCodecNameRemoteFX)
}

func (c bitmapCodecsCapabilityInfo) remoteFXImageCodecID() (byte, bool) {
	return c.codecIDByName(rdpcodec.BitmapCodecNameRemoteFXImage)
}

func writeDemandActive(conn net.Conn, width, height int) error {
	pduLen := demandActivePDULen()
	perLen := encodedPERLengthSize(pduLen)
	bodyLen := 2 + 2 + 1 + perLen + pduLen
	totalLen := 4 + 3 + 1 + bodyLen
	out := make([]byte, totalLen)
	writeTPKTX224MCSHeader(out, mcsSendDataIndicationApp, bodyLen)
	binary.BigEndian.PutUint16(out[8:10], serverChannelID-defaultMCSUserID)
	binary.BigEndian.PutUint16(out[10:12], globalChannelID)
	out[12] = 0x70
	pduOff := 13 + writePERLength(out[13:], pduLen)
	writeDemandActivePDUAt(out[pduOff:pduOff+pduLen], width, height)
	_, err := conn.Write(out)
	if err == nil && traceEnabled {
		tracef("tpkt_write", "payload_len=%d", totalLen-4)
	}
	return err
}

func buildDemandActivePDU(width, height int) []byte {
	out := make([]byte, demandActivePDULen())
	writeDemandActivePDUAt(out, width, height)
	return out
}

func demandActivePDULen() int {
	return 6 + 4 + 2 + 2 + len(serverSourceName) + 4 + serverCapabilitySetsLen() + 4
}

func writeDemandActivePDUAt(out []byte, width, height int) {
	combinedCapsLen := 4 + serverCapabilitySetsLen()
	writeShareControlHeaderAt(out, demandActivePDULen(), pduTypeDemandActive, serverChannelID)
	binary.LittleEndian.PutUint32(out[6:10], defaultShareID)
	binary.LittleEndian.PutUint16(out[10:12], uint16(len(serverSourceName)))
	binary.LittleEndian.PutUint16(out[12:14], uint16(combinedCapsLen))
	copy(out[14:], serverSourceName)
	off := 14 + len(serverSourceName)
	binary.LittleEndian.PutUint16(out[off:off+2], 6) // capability count
	off += 4                                         // pad2Octets is already zero
	writeServerCapabilitySetsAt(out[off:off+serverCapabilitySetsLen()], width, height)
}

func buildServerCapabilitySets(width, height int) []byte {
	out := make([]byte, serverCapabilitySetsLen())
	writeServerCapabilitySetsAt(out, width, height)
	return out
}

func serverCapabilitySetsLen() int {
	return 4 + len(generalCapability) + 4 + 24 + 4 + len(pointerCapability) + 4 + len(inputCapability) + 4 + len(fontCapability) + 4 + len(shareCapability)
}

func writeServerCapabilitySetsAt(out []byte, width, height int) {
	off := 0
	off += writeCapabilitySetAt(out[off:], capTypeGeneral, generalCapability[:])
	off += writeBitmapCapabilitySetAt(out[off:], width, height)
	off += writeCapabilitySetAt(out[off:], capTypePointer, pointerCapability[:])
	off += writeCapabilitySetAt(out[off:], capTypeInput, inputCapability[:])
	off += writeCapabilitySetAt(out[off:], capTypeFont, fontCapability[:])
	writeCapabilitySetAt(out[off:], capTypeShare, shareCapability[:])
}

func capabilitySet(capType uint16, payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	writeCapabilitySetAt(out, capType, payload)
	return out
}

func writeCapabilitySetAt(out []byte, capType uint16, payload []byte) int {
	binary.LittleEndian.PutUint16(out[0:2], capType)
	binary.LittleEndian.PutUint16(out[2:4], uint16(4+len(payload))) // #nosec G115 -- capability payloads are fixed/bounded.
	copy(out[4:], payload)
	return 4 + len(payload)
}

func writeBitmapCapabilitySetAt(out []byte, width, height int) int {
	binary.LittleEndian.PutUint16(out[0:2], capTypeBitmap)
	binary.LittleEndian.PutUint16(out[2:4], 28)
	writeBitmapCapabilityAt(out[4:28], width, height)
	return 28
}

var (
	generalCapability = [...]byte{
		1, 0, // osMajorType: OSMAJORTYPE_WINDOWS
		3, 0, // osMinorType: OSMINORTYPE_WINDOWS_NT
		0, 2, // protocolVersion
		0, 0, // pad2octetsA
		0, 0, // generalCompressionTypes
		0, 0, // extraFlags
		0, 0, // updateCapabilityFlag
		0, 0, // remoteUnshareFlag
		0, 0, // generalCompressionLevel
		0, // refreshRectSupport
		0, // suppressOutputSupport
	}
	pointerCapability = [...]byte{0, 0, 24, 0, 24, 0}
	inputCapability   = buildInputCapabilityTemplate()
	fontCapability    = [...]byte{1, 0, 0, 0}
	shareCapability   = [...]byte{byte(serverChannelID & 0xff), byte((serverChannelID >> 8) & 0xff), 0, 0}
)

func buildInputCapabilityTemplate() [88]byte {
	var out [88]byte
	binary.LittleEndian.PutUint16(out[0:2], 0x0001|0x0004|0x0008) // keyboard + unicode + mouseX
	binary.LittleEndian.PutUint32(out[8:12], 4)                   // keyboardType
	binary.LittleEndian.PutUint32(out[16:20], 12)
	return out
}

func buildGeneralCapability() []byte {
	return generalCapability[:]
}

func buildBitmapCapability(width, height int) []byte {
	out := make([]byte, 24)
	writeBitmapCapabilityAt(out, width, height)
	return out
}

func writeBitmapCapabilityAt(out []byte, width, height int) {
	binary.LittleEndian.PutUint16(out[0:2], 32) // preferredBitsPerPixel
	binary.LittleEndian.PutUint16(out[2:4], 1)  // receive1BitPerPixel
	binary.LittleEndian.PutUint16(out[4:6], 1)  // receive4BitsPerPixel
	binary.LittleEndian.PutUint16(out[6:8], 1)  // receive8BitsPerPixel
	binary.LittleEndian.PutUint16(out[8:10], uint16(width))
	binary.LittleEndian.PutUint16(out[10:12], uint16(height))
	binary.LittleEndian.PutUint16(out[14:16], 1) // desktopResizeFlag
	binary.LittleEndian.PutUint16(out[16:18], 1) // bitmapCompressionFlag
	binary.LittleEndian.PutUint16(out[20:22], 1) // multipleRectangleSupport
}

func buildPointerCapability() []byte {
	return pointerCapability[:]
}

func buildInputCapability() []byte {
	return inputCapability[:]
}

func buildFontCapability() []byte {
	return fontCapability[:]
}

func buildShareCapability() []byte {
	return shareCapability[:]
}

func buildMCSSendDataIndication(initiator, channelID uint16, data []byte) []byte {
	perLen := encodedPERLengthSize(len(data))
	body := make([]byte, 2+2+1+perLen+len(data))
	binary.BigEndian.PutUint16(body[0:2], initiator-defaultMCSUserID)
	binary.BigEndian.PutUint16(body[2:4], channelID)
	body[4] = 0x70
	off := 5 + writePERLength(body[5:], len(data))
	copy(body[off:], data)
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
		case capTypeSurfaceCommands:
			parseSurfaceCommandsCapability(payload, &caps)
		case capTypeBitmapCodecs:
			parseBitmapCodecsCapability(payload, &caps)
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

func parseSurfaceCommandsCapability(payload []byte, caps *confirmActiveCapabilities) {
	if len(payload) < 4 {
		return
	}
	caps.SurfaceCommands.Present = true
	caps.SurfaceCommands.Flags = binary.LittleEndian.Uint32(payload[0:4])
}

func parseBitmapCodecsCapability(payload []byte, caps *confirmActiveCapabilities) {
	upstreamCodecs, err := rdpcodec.ParseBitmapCodecCapabilities(payload)
	if err != nil || len(upstreamCodecs) == 0 {
		return
	}
	codecs := make([]bitmapCodecInfo, 0, len(upstreamCodecs))
	for _, codec := range upstreamCodecs {
		codecs = append(codecs, bitmapCodecInfo{GUID: codec.GUID, ID: codec.ID, Name: codec.Name, PropertiesSize: uint16(len(codec.Properties))}) // #nosec G115 -- upstream parser bounds property slices by payload length.
	}
	caps.BitmapCodecs.Present = true
	caps.BitmapCodecs.Codecs = codecs
}

func writeShareControlHeaderAt(out []byte, totalLength int, pduType uint16, source uint16) {
	binary.LittleEndian.PutUint16(out[0:2], uint16(totalLength)) // #nosec G115 -- share PDUs are bounded by allocation.
	binary.LittleEndian.PutUint16(out[2:4], pduType)
	binary.LittleEndian.PutUint16(out[4:6], source)
}
