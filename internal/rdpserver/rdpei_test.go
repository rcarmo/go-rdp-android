package rdpserver

import (
	"encoding/binary"
	"testing"
)

func TestRDPEIVariableIntegerExamples(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		read func(*rdpeiCursor) (any, error)
		want any
	}{
		{
			name: "two byte unsigned one byte",
			data: []byte{0x7f},
			read: func(c *rdpeiCursor) (any, error) { return c.readVarUint16() },
			want: uint16(0x7f),
		},
		{
			name: "two byte unsigned spec example",
			data: []byte{0x9a, 0x1b},
			read: func(c *rdpeiCursor) (any, error) { return c.readVarUint16() },
			want: uint16(0x1a1b),
		},
		{
			name: "two byte signed negative one byte",
			data: []byte{0x42},
			read: func(c *rdpeiCursor) (any, error) { return c.readVarInt16() },
			want: int16(-2),
		},
		{
			name: "two byte signed spec example",
			data: []byte{0xda, 0x1b},
			read: func(c *rdpeiCursor) (any, error) { return c.readVarInt16() },
			want: int16(-0x1a1b),
		},
		{
			name: "four byte unsigned spec example",
			data: []byte{0x9a, 0x1b, 0x1c},
			read: func(c *rdpeiCursor) (any, error) { return c.readVarUint32() },
			want: uint32(0x001a1b1c),
		},
		{
			name: "four byte signed spec example",
			data: []byte{0xba, 0x1b, 0x1c},
			read: func(c *rdpeiCursor) (any, error) { return c.readVarInt32() },
			want: int32(-0x001a1b1c),
		},
		{
			name: "eight byte unsigned spec example",
			data: []byte{0xda, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x2a},
			read: func(c *rdpeiCursor) (any, error) { return c.readVarUint64() },
			want: uint64(0x001a1b1c1d1e1f2a),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cur := &rdpeiCursor{data: tt.data}
			got, err := tt.read(cur)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %#v, want %#v", got, tt.want)
			}
			if cur.remaining() != 0 {
				t.Fatalf("remaining = %d, want 0", cur.remaining())
			}
		})
	}
}

func TestParseRDPEICSReady(t *testing.T) {
	payload := make([]byte, 10)
	binary.LittleEndian.PutUint32(payload[0:4], rdpeiCSReadyShowTouchVisuals|rdpeiCSReadyDisableTimestampInjection)
	binary.LittleEndian.PutUint32(payload[4:8], rdpeiProtocolV300)
	binary.LittleEndian.PutUint16(payload[8:10], 10)
	pdu, err := parseRDPEIPDU(withRDPEIHeader(rdpeiEventCSReady, payload))
	if err != nil {
		t.Fatalf("parseRDPEIPDU: %v", err)
	}
	if pdu.Header.EventID != rdpeiEventCSReady || pdu.CSReady == nil {
		t.Fatalf("unexpected parsed PDU: %#v", pdu)
	}
	if pdu.CSReady.Flags != rdpeiCSReadyShowTouchVisuals|rdpeiCSReadyDisableTimestampInjection {
		t.Fatalf("flags = 0x%x", pdu.CSReady.Flags)
	}
	if pdu.CSReady.ProtocolVersion != rdpeiProtocolV300 || pdu.CSReady.MaxTouchContacts != 10 {
		t.Fatalf("unexpected CS_READY: %#v", pdu.CSReady)
	}
}

func TestParseRDPEITouchEventSingleContact(t *testing.T) {
	payload := []byte{}
	payload = append(payload, rdpeiTestVarUint32(0)...)
	payload = append(payload, rdpeiTestVarUint16(1)...)
	payload = append(payload, rdpeiTestVarUint16(1)...)
	payload = append(payload, rdpeiTestVarUint64(0)...)
	payload = append(payload, 7)                         // contactId
	payload = append(payload, rdpeiTestVarUint16(0)...)  // fieldsPresent
	payload = append(payload, rdpeiTestVarInt32(320)...) // x
	payload = append(payload, rdpeiTestVarInt32(240)...) // y
	payload = append(payload, rdpeiTestVarUint32(rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact)...)

	pdu, err := parseRDPEIPDU(withRDPEIHeader(rdpeiEventTouch, payload))
	if err != nil {
		t.Fatalf("parseRDPEIPDU: %v", err)
	}
	if pdu.TouchEvent == nil || len(pdu.TouchEvent.Frames) != 1 {
		t.Fatalf("unexpected touch event: %#v", pdu.TouchEvent)
	}
	frame := pdu.TouchEvent.Frames[0]
	if frame.FrameOffset != 0 || len(frame.Contacts) != 1 {
		t.Fatalf("unexpected frame: %#v", frame)
	}
	contact := frame.Contacts[0]
	if contact.ContactID != 7 || contact.X != 320 || contact.Y != 240 {
		t.Fatalf("unexpected contact coordinates: %#v", contact)
	}
	if contact.Flags != rdpeiContactFlagDown|rdpeiContactFlagInRange|rdpeiContactFlagInContact {
		t.Fatalf("unexpected contact flags: 0x%x", contact.Flags)
	}
	if contact.ContactRect != nil || contact.Orientation != nil || contact.Pressure != nil {
		t.Fatalf("unexpected optional fields: %#v", contact)
	}
}

func TestParseRDPEITouchEventOptionalFields(t *testing.T) {
	payload := []byte{}
	payload = append(payload, rdpeiTestVarUint32(5)...)
	payload = append(payload, rdpeiTestVarUint16(1)...)
	payload = append(payload, rdpeiTestVarUint16(1)...)
	payload = append(payload, rdpeiTestVarUint64(1000)...)
	payload = append(payload, 9)
	payload = append(payload, rdpeiTestVarUint16(rdpeiTouchContactRectPresent|rdpeiTouchContactOrientationPresent|rdpeiTouchContactPressurePresent)...)
	payload = append(payload, rdpeiTestVarInt32(-10)...)
	payload = append(payload, rdpeiTestVarInt32(20)...)
	payload = append(payload, rdpeiTestVarUint32(rdpeiContactFlagUpdate|rdpeiContactFlagInRange|rdpeiContactFlagInContact)...)
	payload = append(payload, rdpeiTestVarInt16(-4)...)
	payload = append(payload, rdpeiTestVarInt16(-5)...)
	payload = append(payload, rdpeiTestVarInt16(6)...)
	payload = append(payload, rdpeiTestVarInt16(7)...)
	payload = append(payload, rdpeiTestVarUint32(45)...)
	payload = append(payload, rdpeiTestVarUint32(512)...)

	pdu, err := parseRDPEIPDU(withRDPEIHeader(rdpeiEventTouch, payload))
	if err != nil {
		t.Fatalf("parseRDPEIPDU: %v", err)
	}
	contact := pdu.TouchEvent.Frames[0].Contacts[0]
	if contact.ContactID != 9 || contact.X != -10 || contact.Y != 20 {
		t.Fatalf("unexpected contact: %#v", contact)
	}
	if contact.ContactRect == nil || *contact.ContactRect != (rdpeiTouchContactRect{Left: -4, Top: -5, Right: 6, Bottom: 7}) {
		t.Fatalf("unexpected rect: %#v", contact.ContactRect)
	}
	if contact.Orientation == nil || *contact.Orientation != 45 {
		t.Fatalf("unexpected orientation: %#v", contact.Orientation)
	}
	if contact.Pressure == nil || *contact.Pressure != 512 {
		t.Fatalf("unexpected pressure: %#v", contact.Pressure)
	}
}

func TestParseRDPEIDismissHoveringTouchContact(t *testing.T) {
	pdu, err := parseRDPEIPDU(withRDPEIHeader(rdpeiEventDismissHoveringTouchContact, []byte{0x2a}))
	if err != nil {
		t.Fatalf("parseRDPEIPDU: %v", err)
	}
	if pdu.DismissTouch == nil || pdu.DismissTouch.ContactID != 0x2a {
		t.Fatalf("unexpected dismiss PDU: %#v", pdu.DismissTouch)
	}
}

func TestBuildRDPEISCReadyPDU(t *testing.T) {
	features := uint32(1)
	pdu := buildRDPEISCReadyPDU(rdpeiProtocolV300, &features)
	parsed, err := parseRDPEIPDU(pdu)
	if err != nil {
		t.Fatalf("parse built SC_READY: %v", err)
	}
	if parsed.Header.EventID != rdpeiEventSCReady || parsed.Header.Length != uint32(len(pdu)) {
		t.Fatalf("unexpected header: %#v", parsed.Header)
	}
	if binary.LittleEndian.Uint32(pdu[6:10]) != rdpeiProtocolV300 || binary.LittleEndian.Uint32(pdu[10:14]) != features {
		t.Fatalf("unexpected SC_READY payload: %x", pdu)
	}
}

func TestParseRDPEIPDUErrors(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "short header", data: []byte{0x01}},
		{name: "oversized pdu", data: withOversizedRDPEIHeaderForTest(rdpeiMaxPDUSize + 1)},
		{name: "short declared length", data: []byte{0x01, 0, 0x05, 0, 0, 0}},
		{name: "length mismatch", data: []byte{0x02, 0, 0x10, 0, 0, 0}},
		{name: "unsupported event", data: withRDPEIHeader(0xffff, nil)},
		{name: "truncated CS_READY", data: withRDPEIHeader(rdpeiEventCSReady, []byte{1, 2, 3})},
		{name: "truncated touch varint", data: withRDPEIHeader(rdpeiEventTouch, []byte{0xff})},
		{name: "too many touch frames", data: withRDPEIHeader(rdpeiEventTouch, append(rdpeiTestVarUint32(0), rdpeiTestVarUint16(rdpeiMaxTouchFrames+1)...))},
		{name: "too many contacts", data: withRDPEIHeader(rdpeiEventTouch, rdpeiTouchCountBoundsPayloadForTest(1, rdpeiMaxContactsPerFrame+1))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := parseRDPEIPDU(tt.data); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func FuzzParseRDPEIPDU(f *testing.F) {
	f.Add(withRDPEIHeader(rdpeiEventCSReady, append(append(append([]byte{}, le32(rdpeiCSReadyShowTouchVisuals)...), le32(rdpeiProtocolV300)...), le16(10)...)))
	f.Add(withRDPEIHeader(rdpeiEventDismissHoveringTouchContact, []byte{0x2a}))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseRDPEIPDU(data)
	})
}

func withRDPEIHeader(eventID uint16, payload []byte) []byte {
	out := make([]byte, 6+len(payload))
	binary.LittleEndian.PutUint16(out[0:2], eventID)
	binary.LittleEndian.PutUint32(out[2:6], uint32(len(out))) // #nosec G115 -- test data is bounded.
	copy(out[6:], payload)
	return out
}

func withOversizedRDPEIHeaderForTest(length int) []byte {
	out := make([]byte, length)
	binary.LittleEndian.PutUint16(out[0:2], rdpeiEventTouch)
	binary.LittleEndian.PutUint32(out[2:6], uint32(length)) // #nosec G115 -- test data is bounded.
	return out
}

func rdpeiTouchCountBoundsPayloadForTest(frameCount, contactCount uint16) []byte {
	payload := []byte{}
	payload = append(payload, rdpeiTestVarUint32(0)...)
	payload = append(payload, rdpeiTestVarUint16(frameCount)...)
	payload = append(payload, rdpeiTestVarUint16(contactCount)...)
	payload = append(payload, rdpeiTestVarUint64(0)...)
	return payload
}

func le16(v uint16) []byte {
	out := make([]byte, 2)
	binary.LittleEndian.PutUint16(out, v)
	return out
}

func le32(v uint32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, v)
	return out
}

func rdpeiTestVarUint16(v uint16) []byte {
	if v <= 0x7f {
		return []byte{byte(v)}
	}
	return []byte{0x80 | byte(v>>8), byte(v)}
}

func rdpeiTestVarInt16(v int16) []byte {
	neg := v < 0
	mag := uint16(v)
	if neg {
		mag = uint16(-v)
	}
	if mag <= 0x3f {
		b := byte(mag)
		if neg {
			b |= 0x40
		}
		return []byte{b}
	}
	b := 0x80 | byte(mag>>8)
	if neg {
		b |= 0x40
	}
	return []byte{b, byte(mag)}
}

func rdpeiTestVarUint32(v uint32) []byte {
	switch {
	case v <= 0x3f:
		return []byte{byte(v)}
	case v <= 0x3fff:
		return []byte{0x40 | byte(v>>8), byte(v)}
	case v <= 0x3fffff:
		return []byte{0x80 | byte(v>>16), byte(v >> 8), byte(v)}
	default:
		return []byte{0xc0 | byte(v>>24), byte(v >> 16), byte(v >> 8), byte(v)}
	}
}

func rdpeiTestVarInt32(v int32) []byte {
	neg := v < 0
	mag := uint32(v)
	if neg {
		mag = uint32(-v)
	}
	sign := byte(0)
	if neg {
		sign = 0x20
	}
	switch {
	case mag <= 0x1f:
		return []byte{sign | byte(mag)}
	case mag <= 0x1fff:
		return []byte{0x40 | sign | byte(mag>>8), byte(mag)}
	case mag <= 0x1fffff:
		return []byte{0x80 | sign | byte(mag>>16), byte(mag >> 8), byte(mag)}
	default:
		return []byte{0xc0 | sign | byte(mag>>24), byte(mag >> 16), byte(mag >> 8), byte(mag)}
	}
}

func rdpeiTestVarUint64(v uint64) []byte {
	bytesNeeded := 1
	limits := []uint64{0x1f, 0x1fff, 0x1fffff, 0x1fffffff, 0x1fffffffff, 0x1fffffffffff, 0x1fffffffffffff, 0x1fffffffffffffff}
	for bytesNeeded < len(limits) && v > limits[bytesNeeded-1] {
		bytesNeeded++
	}
	out := make([]byte, bytesNeeded)
	out[0] = byte(bytesNeeded-1) << 5
	shift := uint((bytesNeeded - 1) * 8)
	out[0] |= byte(v >> shift)
	for i := 1; i < bytesNeeded; i++ {
		shift -= 8
		out[i] = byte(v >> shift)
	}
	return out
}
