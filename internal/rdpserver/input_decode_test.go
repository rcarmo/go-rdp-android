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
	wheels  []struct {
		x, y       int
		delta      int
		horizontal bool
	}
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
func (s *recordingInputSink) PointerWheel(x, y int, delta int, horizontal bool) error {
	s.wheels = append(s.wheels, struct {
		x, y       int
		delta      int
		horizontal bool
	}{x: x, y: y, delta: delta, horizontal: horizontal})
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

func TestDispatchFastPathAndSlowPathInputEquivalence(t *testing.T) {
	slowSink := &recordingInputSink{}
	fastSink := &recordingInputSink{}
	slowPayload := buildSlowPathInputPDU(
		buildSlowPathInputEvent(slowInputScanCode, 0, 0x1e, 0),
		buildSlowPathInputEvent(slowInputScanCode, slowKeyboardFlagRelease, 0x1e, 0),
		buildSlowPathInputEvent(slowInputUnicode, 0, 'A', 0),
		buildSlowPathInputEvent(slowInputMouse, slowPointerFlagMove, 12, 34),
		buildSlowPathInputEvent(slowInputMouse, slowPointerFlagDown|slowPointerFlagButton1, 12, 34),
	)
	fastPayload := []byte{
		byte(fastPathInputEventScanCode<<5) | 0x00, 0x1e,
		byte(fastPathInputEventScanCode<<5) | fastPathKeyboardFlagRelease, 0x1e,
		byte(fastPathInputEventUnicode << 5), 'A', 0,
		byte(fastPathInputEventMouse << 5),
	}
	fastPayload = appendLE16Bytes(fastPayload, slowPointerFlagMove)
	fastPayload = appendLE16Bytes(fastPayload, 12)
	fastPayload = appendLE16Bytes(fastPayload, 34)
	fastPayload = append(fastPayload, byte(fastPathInputEventMouse<<5))
	fastPayload = appendLE16Bytes(fastPayload, slowPointerFlagDown|slowPointerFlagButton1)
	fastPayload = appendLE16Bytes(fastPayload, 12)
	fastPayload = appendLE16Bytes(fastPayload, 34)

	if err := dispatchSlowPathInput(slowPayload, slowSink); err != nil {
		t.Fatalf("slow path: %v", err)
	}
	if err := dispatchFastPathInput(byte(5<<2), fastPayload, fastSink); err != nil {
		t.Fatalf("fast path: %v", err)
	}
	assertInputSinksEqual(t, slowSink, fastSink)
}

func TestDispatchPointerWheelInput(t *testing.T) {
	sink := &recordingInputSink{}
	payload := buildSlowPathInputPDU(
		buildSlowPathInputEvent(slowInputMouse, slowPointerFlagWheel|0x0078, 20, 30),
		buildSlowPathInputEvent(slowInputMouse, slowPointerFlagHWheel|0x0188, 21, 31),
	)
	if err := dispatchSlowPathInput(payload, sink); err != nil {
		t.Fatal(err)
	}
	if len(sink.wheels) != 2 {
		t.Fatalf("unexpected wheels: %#v", sink.wheels)
	}
	if sink.wheels[0].x != 20 || sink.wheels[0].y != 30 || sink.wheels[0].delta != 120 || sink.wheels[0].horizontal {
		t.Fatalf("unexpected vertical wheel: %#v", sink.wheels[0])
	}
	if sink.wheels[1].x != 21 || sink.wheels[1].y != 31 || sink.wheels[1].delta != -120 || !sink.wheels[1].horizontal {
		t.Fatalf("unexpected horizontal wheel: %#v", sink.wheels[1])
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

func assertInputSinksEqual(t *testing.T, a, b *recordingInputSink) {
	t.Helper()
	if len(a.moves) != len(b.moves) {
		t.Fatalf("move count mismatch: %#v != %#v", a.moves, b.moves)
	}
	for i := range a.moves {
		if a.moves[i] != b.moves[i] {
			t.Fatalf("move %d mismatch: %#v != %#v", i, a.moves[i], b.moves[i])
		}
	}
	if len(a.buttons) != len(b.buttons) {
		t.Fatalf("button count mismatch: %#v != %#v", a.buttons, b.buttons)
	}
	for i := range a.buttons {
		if a.buttons[i] != b.buttons[i] {
			t.Fatalf("button %d mismatch: %#v != %#v", i, a.buttons[i], b.buttons[i])
		}
	}
	if len(a.keys) != len(b.keys) {
		t.Fatalf("key count mismatch: %#v != %#v", a.keys, b.keys)
	}
	for i := range a.keys {
		if a.keys[i] != b.keys[i] {
			t.Fatalf("key %d mismatch: %#v != %#v", i, a.keys[i], b.keys[i])
		}
	}
	if len(a.unicode) != len(b.unicode) {
		t.Fatalf("unicode count mismatch: %#v != %#v", a.unicode, b.unicode)
	}
	for i := range a.unicode {
		if a.unicode[i] != b.unicode[i] {
			t.Fatalf("unicode %d mismatch: %#v != %#v", i, a.unicode[i], b.unicode[i])
		}
	}
	if len(a.wheels) != len(b.wheels) {
		t.Fatalf("wheel count mismatch: %#v != %#v", a.wheels, b.wheels)
	}
	for i := range a.wheels {
		if a.wheels[i] != b.wheels[i] {
			t.Fatalf("wheel %d mismatch: %#v != %#v", i, a.wheels[i], b.wheels[i])
		}
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
