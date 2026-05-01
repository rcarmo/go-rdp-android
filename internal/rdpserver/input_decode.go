package rdpserver

import (
	"encoding/binary"
	"fmt"

	"github.com/rcarmo/go-rdp-android/internal/input"
)

const (
	pduType2Input = 0x1c

	slowInputSync     = 0x0000
	slowInputScanCode = 0x0004
	slowInputUnicode  = 0x0005
	slowInputMouse    = 0x8001
	slowInputMouseX   = 0x8002

	slowKeyboardFlagRelease = 0x8000

	slowPointerFlagMove    = 0x0800
	slowPointerFlagDown    = 0x8000
	slowPointerFlagButton1 = 0x1000
	slowPointerFlagButton2 = 0x2000
	slowPointerFlagButton3 = 0x4000
)

type decodedInputEvent struct {
	MessageType uint16
	Flags       uint16
	Code        uint16
	X           uint16
	Y           uint16
}

func dispatchSlowPathInput(payload []byte, sink input.Sink) error {
	events, err := parseSlowPathInput(payload)
	if err != nil {
		return err
	}
	if sink == nil {
		return nil
	}
	for _, event := range events {
		switch event.MessageType {
		case slowInputScanCode:
			if err := sink.Key(event.Code, event.Flags&slowKeyboardFlagRelease == 0); err != nil {
				return err
			}
		case slowInputUnicode:
			if event.Flags&slowKeyboardFlagRelease == 0 {
				if err := sink.Unicode(rune(event.Code)); err != nil {
					return err
				}
			}
		case slowInputMouse:
			if event.Flags&slowPointerFlagMove != 0 {
				if err := sink.PointerMove(int(event.X), int(event.Y)); err != nil {
					return err
				}
			}
			buttons := pointerButtons(event.Flags)
			if buttons != 0 {
				if err := sink.PointerButton(int(event.X), int(event.Y), buttons, event.Flags&slowPointerFlagDown != 0); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func parseSlowPathInput(payload []byte) ([]decodedInputEvent, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("short Input PDU")
	}
	count := int(binary.LittleEndian.Uint16(payload[0:2]))
	offset := 4
	events := make([]decodedInputEvent, 0, count)
	for i := 0; i < count; i++ {
		if len(payload)-offset < 12 {
			return nil, fmt.Errorf("short Input event")
		}
		messageType := binary.LittleEndian.Uint16(payload[offset+4 : offset+6])
		event := decodedInputEvent{MessageType: messageType}
		switch messageType {
		case slowInputSync:
			event.Flags = binary.LittleEndian.Uint16(payload[offset+6 : offset+8])
		case slowInputScanCode, slowInputUnicode:
			event.Flags = binary.LittleEndian.Uint16(payload[offset+6 : offset+8])
			event.Code = binary.LittleEndian.Uint16(payload[offset+8 : offset+10])
		case slowInputMouse, slowInputMouseX:
			event.Flags = binary.LittleEndian.Uint16(payload[offset+6 : offset+8])
			event.X = binary.LittleEndian.Uint16(payload[offset+8 : offset+10])
			event.Y = binary.LittleEndian.Uint16(payload[offset+10 : offset+12])
		default:
			// Preserve unknown event type but do not dispatch it.
		}
		events = append(events, event)
		offset += 12
	}
	return events, nil
}

func pointerButtons(flags uint16) input.ButtonState {
	var buttons input.ButtonState
	if flags&slowPointerFlagButton1 != 0 {
		buttons |= input.ButtonPrimary
	}
	if flags&slowPointerFlagButton2 != 0 {
		buttons |= input.ButtonSecondary
	}
	if flags&slowPointerFlagButton3 != 0 {
		buttons |= input.ButtonMiddle
	}
	return buttons
}

func buildSlowPathInputEvent(messageType, flags, codeOrX, y uint16) []byte {
	out := appendLE32Bytes(nil, 0) // eventTime
	out = appendLE16Bytes(out, messageType)
	out = appendLE16Bytes(out, flags)
	out = appendLE16Bytes(out, codeOrX)
	out = appendLE16Bytes(out, y)
	return out
}

func buildSlowPathInputPDU(events ...[]byte) []byte {
	out := appendLE16Bytes(nil, uint16(len(events)))
	out = appendLE16Bytes(out, 0)
	for _, event := range events {
		out = append(out, event...)
	}
	return out
}
