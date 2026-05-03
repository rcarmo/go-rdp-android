package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/input"
)

type recordingInputSink struct {
	moves   [][2]int
	buttons []struct {
		x, y int
		b    input.ButtonState
		down bool
	}
	keys []struct {
		scancode uint16
		down     bool
	}
	unicode []rune
}

func (s *recordingInputSink) PointerMove(x, y int) error {
	s.moves = append(s.moves, [2]int{x, y})
	return nil
}
func (s *recordingInputSink) PointerButton(x, y int, b input.ButtonState, down bool) error {
	s.buttons = append(s.buttons, struct {
		x, y int
		b    input.ButtonState
		down bool
	}{x: x, y: y, b: b, down: down})
	return nil
}
func (s *recordingInputSink) Key(scancode uint16, down bool) error {
	s.keys = append(s.keys, struct {
		scancode uint16
		down     bool
	}{scancode: scancode, down: down})
	return nil
}
func (s *recordingInputSink) Unicode(r rune) error {
	s.unicode = append(s.unicode, r)
	return nil
}

func TestParseSlowPathInput(t *testing.T) {
	payload := buildSlowPathInputPDU(
		buildSlowPathInputEvent(slowInputScanCode, 0, 0x1e, 0),
		buildSlowPathInputEvent(slowInputMouse, slowPointerFlagMove, 10, 20),
	)
	events, err := parseSlowPathInput(payload)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Code != 0x1e || events[1].X != 10 || events[1].Y != 20 {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestParseFastPathInputEvents(t *testing.T) {
	payload := []byte{
		byte(fastPathInputEventScanCode<<5) | 0x00, 0x1e,
		byte(fastPathInputEventMouse << 5),
	}
	payload = appendLE16Bytes(payload, slowPointerFlagMove)
	payload = appendLE16Bytes(payload, 10)
	payload = appendLE16Bytes(payload, 20)
	events, err := parseFastPathInputEvents(payload, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].MessageType != slowInputScanCode || events[0].Code != 0x1e || events[1].X != 10 || events[1].Y != 20 {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestDispatchFastPathInput(t *testing.T) {
	sink := &recordingInputSink{}
	payload := []byte{
		byte(fastPathInputEventScanCode<<5) | 0x00, 0x1e,
		byte(fastPathInputEventScanCode<<5) | fastPathKeyboardFlagRelease, 0x1e,
		byte(fastPathInputEventUnicode << 5), 'A', 0,
		byte(fastPathInputEventMouse << 5),
	}
	payload = appendLE16Bytes(payload, slowPointerFlagMove)
	payload = appendLE16Bytes(payload, 12)
	payload = appendLE16Bytes(payload, 34)
	payload = append(payload, byte(fastPathInputEventMouse<<5))
	payload = appendLE16Bytes(payload, slowPointerFlagDown|slowPointerFlagButton1)
	payload = appendLE16Bytes(payload, 12)
	payload = appendLE16Bytes(payload, 34)
	header := byte(len([]int{1, 2, 3, 4, 5}) << 2)
	if err := dispatchFastPathInput(header, payload, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.keys) != 2 || !sink.keys[0].down || sink.keys[1].down {
		t.Fatalf("unexpected keys: %#v", sink.keys)
	}
	if len(sink.unicode) != 1 || sink.unicode[0] != 'A' {
		t.Fatalf("unexpected unicode: %#v", sink.unicode)
	}
	if len(sink.moves) != 1 || sink.moves[0] != ([2]int{12, 34}) {
		t.Fatalf("unexpected moves: %#v", sink.moves)
	}
	if len(sink.buttons) != 1 || sink.buttons[0].b != input.ButtonPrimary || !sink.buttons[0].down {
		t.Fatalf("unexpected buttons: %#v", sink.buttons)
	}
}

func TestDispatchFastPathInputExtendedEventCount(t *testing.T) {
	sink := &recordingInputSink{}
	payload := []byte{1, byte(fastPathInputEventScanCode << 5), 0x47}
	if err := dispatchFastPathInput(0, payload, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.keys) != 1 || sink.keys[0].scancode != 0x47 || !sink.keys[0].down {
		t.Fatalf("unexpected keys: %#v", sink.keys)
	}
}

func TestDispatchSlowPathInput(t *testing.T) {
	sink := &recordingInputSink{}
	payload := buildSlowPathInputPDU(
		buildSlowPathInputEvent(slowInputScanCode, 0, 0x1e, 0),
		buildSlowPathInputEvent(slowInputScanCode, slowKeyboardFlagRelease, 0x1e, 0),
		buildSlowPathInputEvent(slowInputUnicode, 0, 'A', 0),
		buildSlowPathInputEvent(slowInputMouse, slowPointerFlagMove, 12, 34),
		buildSlowPathInputEvent(slowInputMouse, slowPointerFlagDown|slowPointerFlagButton1, 12, 34),
	)
	if err := dispatchSlowPathInput(payload, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.keys) != 2 || !sink.keys[0].down || sink.keys[1].down {
		t.Fatalf("unexpected keys: %#v", sink.keys)
	}
	if len(sink.unicode) != 1 || sink.unicode[0] != 'A' {
		t.Fatalf("unexpected unicode: %#v", sink.unicode)
	}
	if len(sink.moves) != 1 || sink.moves[0] != ([2]int{12, 34}) {
		t.Fatalf("unexpected moves: %#v", sink.moves)
	}
	if len(sink.buttons) != 1 || sink.buttons[0].b != input.ButtonPrimary || !sink.buttons[0].down {
		t.Fatalf("unexpected buttons: %#v", sink.buttons)
	}
}
