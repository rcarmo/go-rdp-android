package rdpserver

import (
	"encoding/binary"
	"fmt"

	"github.com/rcarmo/go-rdp-android/internal/input"
)

const (
	pduType2Input = 0x1c

	fastPathInputActionFastPath = 0x00

	fastPathInputEventScanCode = 0x00
	fastPathInputEventMouse    = 0x01
	fastPathInputEventMouseX   = 0x02
	fastPathInputEventSync     = 0x03
	fastPathInputEventUnicode  = 0x04

	fastPathKeyboardFlagRelease = 0x01

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
	FastPath    bool
}

func dispatchFastPathInput(header byte, payload []byte, sink input.Sink) error {
	action := header & 0x03
	numEvents := int((header >> 2) & 0x0f)
	flags := (header >> 6) & 0x03
	if action != fastPathInputActionFastPath {
		tracef("fastpath_input_skip", "action=%d flags=0x%02x payload_len=%d", action, flags, len(payload))
		return nil
	}
	if flags != 0 {
		return fmt.Errorf("unsupported Fast-Path input flags 0x%02x", flags)
	}
	if numEvents == 0 {
		if len(payload) == 0 {
			return fmt.Errorf("short Fast-Path input event count")
		}
		numEvents = int(payload[0])
		payload = payload[1:]
	}
	events, err := parseFastPathInputEvents(payload, numEvents)
	if err != nil {
		return err
	}
	tracef("fastpath_input", "events=%d payload_len=%d", len(events), len(payload))
	return dispatchDecodedInputEvents(events, sink)
}

func parseFastPathInputEvents(payload []byte, count int) ([]decodedInputEvent, error) {
	events := make([]decodedInputEvent, 0, count)
	offset := 0
	for i := 0; i < count; i++ {
		if offset >= len(payload) {
			return nil, fmt.Errorf("short Fast-Path input event")
		}
		eventHeader := payload[offset]
		offset++
		eventCode := uint16((eventHeader >> 5) & 0x07)
		eventFlags := uint16(eventHeader & 0x1f)
		event := decodedInputEvent{Flags: eventFlags, FastPath: true}
		switch eventCode {
		case fastPathInputEventScanCode:
			event.MessageType = slowInputScanCode
			if len(payload)-offset < 1 {
				return nil, fmt.Errorf("short Fast-Path scancode event")
			}
			event.Code = uint16(payload[offset])
			offset++
		case fastPathInputEventMouse, fastPathInputEventMouseX:
			if eventCode == fastPathInputEventMouseX {
				event.MessageType = slowInputMouseX
			} else {
				event.MessageType = slowInputMouse
			}
			if len(payload)-offset < 6 {
				return nil, fmt.Errorf("short Fast-Path mouse event")
			}
			event.Flags = binary.LittleEndian.Uint16(payload[offset : offset+2])
			event.X = binary.LittleEndian.Uint16(payload[offset+2 : offset+4])
			event.Y = binary.LittleEndian.Uint16(payload[offset+4 : offset+6])
			offset += 6
		case fastPathInputEventSync:
			event.MessageType = slowInputSync
			// Flags are carried in the event header; no extra payload.
		case fastPathInputEventUnicode:
			event.MessageType = slowInputUnicode
			if len(payload)-offset < 2 {
				return nil, fmt.Errorf("short Fast-Path unicode event")
			}
			event.Code = binary.LittleEndian.Uint16(payload[offset : offset+2])
			offset += 2
		default:
			return nil, fmt.Errorf("unsupported Fast-Path input event code %d", eventCode)
		}
		events = append(events, event)
	}
	return events, nil
}

func dispatchSlowPathInput(payload []byte, sink input.Sink) error {
	events, err := parseSlowPathInput(payload)
	if err != nil {
		return err
	}
	return dispatchDecodedInputEvents(events, sink)
}

func dispatchDecodedInputEvents(events []decodedInputEvent, sink input.Sink) error {
	if sink == nil {
		return nil
	}
	for _, event := range events {
		switch event.MessageType {
		case slowInputScanCode:
			released := event.Flags&slowKeyboardFlagRelease != 0
			if event.FastPath {
				released = event.Flags&fastPathKeyboardFlagRelease != 0
			}
			if err := sink.Key(event.Code, !released); err != nil {
				return err
			}
		case slowInputUnicode:
			released := event.Flags&slowKeyboardFlagRelease != 0
			if event.FastPath {
				released = event.Flags&fastPathKeyboardFlagRelease != 0
			}
			if !released {
				if err := sink.Unicode(rune(event.Code)); err != nil {
					return err
				}
			}
		case slowInputMouse, slowInputMouseX:
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
