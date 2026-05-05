package rdpserver

import (
	"encoding/binary"
	"fmt"
)

const (
	rdpeiDynamicChannelName = "Microsoft::Windows::RDS::Input"

	rdpeiEventSCReady                     uint16 = 0x0001
	rdpeiEventCSReady                     uint16 = 0x0002
	rdpeiEventTouch                       uint16 = 0x0003
	rdpeiEventSuspendInput                uint16 = 0x0004
	rdpeiEventResumeInput                 uint16 = 0x0005
	rdpeiEventDismissHoveringTouchContact uint16 = 0x0006
	rdpeiEventPen                         uint16 = 0x0008

	rdpeiProtocolV100 uint32 = 0x00010000
	rdpeiProtocolV101 uint32 = 0x00010001
	rdpeiProtocolV200 uint32 = 0x00020000
	rdpeiProtocolV300 uint32 = 0x00030000

	rdpeiCSReadyShowTouchVisuals          uint32 = 0x00000001
	rdpeiCSReadyDisableTimestampInjection uint32 = 0x00000002
	rdpeiCSReadyEnableMultipenInjection   uint32 = 0x00000004

	rdpeiTouchContactRectPresent        uint16 = 0x0001
	rdpeiTouchContactOrientationPresent uint16 = 0x0002
	rdpeiTouchContactPressurePresent    uint16 = 0x0004

	rdpeiContactFlagDown      uint32 = 0x0001
	rdpeiContactFlagUpdate    uint32 = 0x0002
	rdpeiContactFlagUp        uint32 = 0x0004
	rdpeiContactFlagInRange   uint32 = 0x0008
	rdpeiContactFlagInContact uint32 = 0x0010
	rdpeiContactFlagCanceled  uint32 = 0x0020

	rdpeiMaxPDUSize          = 256 * 1024
	rdpeiMaxTouchFrames      = 256
	rdpeiMaxContactsPerFrame = 64
)

type rdpeiPDU struct {
	Header       rdpeiHeader
	SCReady      *rdpeiSCReadyPDU
	CSReady      *rdpeiCSReadyPDU
	TouchEvent   *rdpeiTouchEventPDU
	DismissTouch *rdpeiDismissHoveringTouchContactPDU
}

type rdpeiHeader struct {
	EventID uint16
	Length  uint32
}

type rdpeiSCReadyPDU struct {
	ProtocolVersion   uint32
	SupportedFeatures *uint32
}

type rdpeiCSReadyPDU struct {
	Flags            uint32
	ProtocolVersion  uint32
	MaxTouchContacts uint16
}

type rdpeiTouchEventPDU struct {
	EncodeTime uint32
	Frames     []rdpeiTouchFrame
}

type rdpeiTouchFrame struct {
	FrameOffset uint64
	Contacts    []rdpeiTouchContact
}

type rdpeiTouchContact struct {
	ContactID     uint8
	FieldsPresent uint16
	X             int32
	Y             int32
	Flags         uint32

	ContactRect *rdpeiTouchContactRect
	Orientation *uint32
	Pressure    *uint32
}

type rdpeiTouchContactRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

type rdpeiDismissHoveringTouchContactPDU struct {
	ContactID uint8
}

type rdpeiCursor struct {
	data []byte
	off  int
}

func parseRDPEIPDU(data []byte) (*rdpeiPDU, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("RDPEI PDU too short: %d", len(data))
	}
	if len(data) > rdpeiMaxPDUSize {
		return nil, fmt.Errorf("RDPEI PDU length %d exceeds maximum %d", len(data), rdpeiMaxPDUSize)
	}
	header := rdpeiHeader{
		EventID: binary.LittleEndian.Uint16(data[0:2]),
		Length:  binary.LittleEndian.Uint32(data[2:6]),
	}
	if header.Length < 6 {
		return nil, fmt.Errorf("RDPEI PDU length below header: %d", header.Length)
	}
	if int(header.Length) != len(data) {
		return nil, fmt.Errorf("RDPEI PDU length mismatch: header=%d actual=%d", header.Length, len(data))
	}
	cur := &rdpeiCursor{data: data[6:]}
	pdu := &rdpeiPDU{Header: header}
	var err error
	switch header.EventID {
	case rdpeiEventSCReady:
		pdu.SCReady, err = parseRDPEISCReady(cur)
	case rdpeiEventCSReady:
		pdu.CSReady, err = parseRDPEICSReady(cur)
	case rdpeiEventTouch:
		pdu.TouchEvent, err = parseRDPEITouchEvent(cur)
	case rdpeiEventDismissHoveringTouchContact:
		pdu.DismissTouch, err = parseRDPEIDismissHoveringTouchContact(cur)
	case rdpeiEventSuspendInput, rdpeiEventResumeInput, rdpeiEventPen:
		// The server-side path currently only consumes client-to-server touch PDUs.
		// Preserve the header so higher layers can safely ignore unsupported events.
	default:
		return nil, fmt.Errorf("unsupported RDPEI event ID 0x%04x", header.EventID)
	}
	if err != nil {
		return nil, err
	}
	if cur.remaining() != 0 {
		return nil, fmt.Errorf("RDPEI PDU has %d trailing bytes", cur.remaining())
	}
	return pdu, nil
}

func parseRDPEISCReady(cur *rdpeiCursor) (*rdpeiSCReadyPDU, error) {
	version, err := cur.readUint32LE()
	if err != nil {
		return nil, fmt.Errorf("read RDPEI SC_READY protocol version: %w", err)
	}
	ready := &rdpeiSCReadyPDU{ProtocolVersion: version}
	if cur.remaining() >= 4 {
		features, err := cur.readUint32LE()
		if err != nil {
			return nil, fmt.Errorf("read RDPEI SC_READY supported features: %w", err)
		}
		ready.SupportedFeatures = &features
	}
	return ready, nil
}

func parseRDPEICSReady(cur *rdpeiCursor) (*rdpeiCSReadyPDU, error) {
	flags, err := cur.readUint32LE()
	if err != nil {
		return nil, fmt.Errorf("read RDPEI CS_READY flags: %w", err)
	}
	version, err := cur.readUint32LE()
	if err != nil {
		return nil, fmt.Errorf("read RDPEI CS_READY protocol version: %w", err)
	}
	maxContacts, err := cur.readUint16LE()
	if err != nil {
		return nil, fmt.Errorf("read RDPEI CS_READY max touch contacts: %w", err)
	}
	return &rdpeiCSReadyPDU{Flags: flags, ProtocolVersion: version, MaxTouchContacts: maxContacts}, nil
}

func parseRDPEITouchEvent(cur *rdpeiCursor) (*rdpeiTouchEventPDU, error) {
	encodeTime, err := cur.readVarUint32()
	if err != nil {
		return nil, fmt.Errorf("read RDPEI touch encode time: %w", err)
	}
	frameCount, err := cur.readVarUint16()
	if err != nil {
		return nil, fmt.Errorf("read RDPEI touch frame count: %w", err)
	}
	if frameCount > rdpeiMaxTouchFrames {
		return nil, fmt.Errorf("RDPEI touch frame count %d exceeds maximum %d", frameCount, rdpeiMaxTouchFrames)
	}
	frames := make([]rdpeiTouchFrame, 0, frameCount)
	for i := 0; i < int(frameCount); i++ {
		frame, err := parseRDPEITouchFrame(cur)
		if err != nil {
			return nil, fmt.Errorf("read RDPEI touch frame %d: %w", i, err)
		}
		frames = append(frames, frame)
	}
	return &rdpeiTouchEventPDU{EncodeTime: encodeTime, Frames: frames}, nil
}

func parseRDPEITouchFrame(cur *rdpeiCursor) (rdpeiTouchFrame, error) {
	contactCount, err := cur.readVarUint16()
	if err != nil {
		return rdpeiTouchFrame{}, fmt.Errorf("read contact count: %w", err)
	}
	if contactCount > rdpeiMaxContactsPerFrame {
		return rdpeiTouchFrame{}, fmt.Errorf("RDPEI touch contact count %d exceeds maximum %d", contactCount, rdpeiMaxContactsPerFrame)
	}
	frameOffset, err := cur.readVarUint64()
	if err != nil {
		return rdpeiTouchFrame{}, fmt.Errorf("read frame offset: %w", err)
	}
	contacts := make([]rdpeiTouchContact, 0, contactCount)
	for i := 0; i < int(contactCount); i++ {
		contact, err := parseRDPEITouchContact(cur)
		if err != nil {
			return rdpeiTouchFrame{}, fmt.Errorf("read contact %d: %w", i, err)
		}
		contacts = append(contacts, contact)
	}
	return rdpeiTouchFrame{FrameOffset: frameOffset, Contacts: contacts}, nil
}

func parseRDPEITouchContact(cur *rdpeiCursor) (rdpeiTouchContact, error) {
	contactID, err := cur.readByte()
	if err != nil {
		return rdpeiTouchContact{}, fmt.Errorf("read contact ID: %w", err)
	}
	fieldsPresent, err := cur.readVarUint16()
	if err != nil {
		return rdpeiTouchContact{}, fmt.Errorf("read fieldsPresent: %w", err)
	}
	x, err := cur.readVarInt32()
	if err != nil {
		return rdpeiTouchContact{}, fmt.Errorf("read x: %w", err)
	}
	y, err := cur.readVarInt32()
	if err != nil {
		return rdpeiTouchContact{}, fmt.Errorf("read y: %w", err)
	}
	flags, err := cur.readVarUint32()
	if err != nil {
		return rdpeiTouchContact{}, fmt.Errorf("read flags: %w", err)
	}
	contact := rdpeiTouchContact{ContactID: contactID, FieldsPresent: fieldsPresent, X: x, Y: y, Flags: flags}
	if fieldsPresent&rdpeiTouchContactRectPresent != 0 {
		left, err := cur.readVarInt16()
		if err != nil {
			return rdpeiTouchContact{}, fmt.Errorf("read contactRectLeft: %w", err)
		}
		top, err := cur.readVarInt16()
		if err != nil {
			return rdpeiTouchContact{}, fmt.Errorf("read contactRectTop: %w", err)
		}
		right, err := cur.readVarInt16()
		if err != nil {
			return rdpeiTouchContact{}, fmt.Errorf("read contactRectRight: %w", err)
		}
		bottom, err := cur.readVarInt16()
		if err != nil {
			return rdpeiTouchContact{}, fmt.Errorf("read contactRectBottom: %w", err)
		}
		contact.ContactRect = &rdpeiTouchContactRect{Left: left, Top: top, Right: right, Bottom: bottom}
	}
	if fieldsPresent&rdpeiTouchContactOrientationPresent != 0 {
		orientation, err := cur.readVarUint32()
		if err != nil {
			return rdpeiTouchContact{}, fmt.Errorf("read orientation: %w", err)
		}
		contact.Orientation = &orientation
	}
	if fieldsPresent&rdpeiTouchContactPressurePresent != 0 {
		pressure, err := cur.readVarUint32()
		if err != nil {
			return rdpeiTouchContact{}, fmt.Errorf("read pressure: %w", err)
		}
		contact.Pressure = &pressure
	}
	return contact, nil
}

func parseRDPEIDismissHoveringTouchContact(cur *rdpeiCursor) (*rdpeiDismissHoveringTouchContactPDU, error) {
	contactID, err := cur.readByte()
	if err != nil {
		return nil, fmt.Errorf("read RDPEI dismiss contact ID: %w", err)
	}
	return &rdpeiDismissHoveringTouchContactPDU{ContactID: contactID}, nil
}

func buildRDPEISCReadyPDU(protocolVersion uint32, supportedFeatures *uint32) []byte {
	payloadLen := 4
	if supportedFeatures != nil {
		payloadLen += 4
	}
	out := make([]byte, 6+payloadLen)
	binary.LittleEndian.PutUint16(out[0:2], rdpeiEventSCReady)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out))) // #nosec G115 -- bounded by allocation above.
	binary.LittleEndian.PutUint32(out[6:10], protocolVersion)
	if supportedFeatures != nil {
		binary.LittleEndian.PutUint32(out[10:14], *supportedFeatures)
	}
	return out
}

func (c *rdpeiCursor) remaining() int { return len(c.data) - c.off }

func (c *rdpeiCursor) readByte() (byte, error) {
	if c.remaining() < 1 {
		return 0, fmt.Errorf("buffer exhausted")
	}
	b := c.data[c.off]
	c.off++
	return b, nil
}

func (c *rdpeiCursor) readUint16LE() (uint16, error) {
	if c.remaining() < 2 {
		return 0, fmt.Errorf("buffer exhausted")
	}
	v := binary.LittleEndian.Uint16(c.data[c.off : c.off+2])
	c.off += 2
	return v, nil
}

func (c *rdpeiCursor) readUint32LE() (uint32, error) {
	if c.remaining() < 4 {
		return 0, fmt.Errorf("buffer exhausted")
	}
	v := binary.LittleEndian.Uint32(c.data[c.off : c.off+4])
	c.off += 4
	return v, nil
}

func (c *rdpeiCursor) readVarUint16() (uint16, error) {
	v, err := c.readVarUint(1, 7)
	return uint16(v), err // #nosec G115 -- readVarUint(1, 7) returns at most 0x7fff.
}

func (c *rdpeiCursor) readVarInt16() (int16, error) {
	v, err := c.readVarInt(1, 6)
	return int16(v), err // #nosec G115 -- readVarInt(1, 6) returns within signed 15-bit range.
}

func (c *rdpeiCursor) readVarUint32() (uint32, error) {
	v, err := c.readVarUint(2, 6)
	return uint32(v), err // #nosec G115 -- readVarUint(2, 6) returns at most 0x3fffffff.
}

func (c *rdpeiCursor) readVarInt32() (int32, error) {
	v, err := c.readVarInt(2, 5)
	return int32(v), err // #nosec G115 -- readVarInt(2, 5) returns within signed 30-bit range.
}

func (c *rdpeiCursor) readVarUint64() (uint64, error) {
	return c.readVarUint(3, 5)
}

func (c *rdpeiCursor) readVarUint(countBits, valueBits uint) (uint64, error) {
	first, err := c.readByte()
	if err != nil {
		return 0, err
	}
	count := int(first >> (8 - countBits))
	totalBytes := count + 1
	if c.remaining() < totalBytes-1 {
		return 0, fmt.Errorf("variable unsigned integer needs %d bytes, have %d", totalBytes, c.remaining()+1)
	}
	valueMask := byte((1 << valueBits) - 1)
	value := uint64(first & valueMask)
	for i := 1; i < totalBytes; i++ {
		b, _ := c.readByte()
		value = (value << 8) | uint64(b)
	}
	return value, nil
}

func (c *rdpeiCursor) readVarInt(countBits, valueBits uint) (int64, error) {
	first, err := c.readByte()
	if err != nil {
		return 0, err
	}
	count := int(first >> (8 - countBits))
	signBit := byte(1 << (7 - countBits))
	totalBytes := count + 1
	if c.remaining() < totalBytes-1 {
		return 0, fmt.Errorf("variable signed integer needs %d bytes, have %d", totalBytes, c.remaining()+1)
	}
	valueMask := byte((1 << valueBits) - 1)
	value := uint64(first & valueMask)
	for i := 1; i < totalBytes; i++ {
		b, _ := c.readByte()
		value = (value << 8) | uint64(b)
	}
	if first&signBit != 0 {
		return -int64(value), nil // #nosec G115 -- protocol signed integers are at most 30-bit magnitude here.
	}
	return int64(value), nil // #nosec G115 -- protocol signed integers are at most 30-bit magnitude here.
}
