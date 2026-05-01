package mobile

import (
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestFrameQueueDropsOldest(t *testing.T) {
	q := NewFrameQueue(1)
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3, 4}}); err != nil {
		t.Fatal(err)
	}
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{5, 6, 7, 8}}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-q.Frames():
		if got.Data[0] != 5 {
			t.Fatalf("expected newest frame, got %#v", got.Data)
		}
	default:
		t.Fatal("expected queued frame")
	}
}

func TestFrameQueueValidationAndClose(t *testing.T) {
	q := NewFrameQueue(0)
	if err := q.Submit(frame.Frame{}); err == nil {
		t.Fatal("expected invalid frame error")
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Data: []byte{1}}); err == nil {
		t.Fatal("expected closed queue error")
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
}

type recordingMobileInputHandler struct {
	moves   [][2]int
	buttons []struct {
		x, y    int
		buttons int
		down    bool
	}
	keys []struct {
		scancode int
		down     bool
	}
	unicode []int
}

func (h *recordingMobileInputHandler) PointerMove(x int, y int) {
	h.moves = append(h.moves, [2]int{x, y})
}
func (h *recordingMobileInputHandler) PointerButton(x int, y int, buttons int, down bool) {
	h.buttons = append(h.buttons, struct {
		x, y    int
		buttons int
		down    bool
	}{x: x, y: y, buttons: buttons, down: down})
}
func (h *recordingMobileInputHandler) Key(scancode int, down bool) {
	h.keys = append(h.keys, struct {
		scancode int
		down     bool
	}{scancode: scancode, down: down})
}
func (h *recordingMobileInputHandler) Unicode(codepoint int) {
	h.unicode = append(h.unicode, codepoint)
}

func TestMobileInputHandler(t *testing.T) {
	srv := NewServer()
	handler := &recordingMobileInputHandler{}
	srv.SetInputHandler(handler)
	if err := srv.input.PointerMove(10, 20); err != nil {
		t.Fatal(err)
	}
	if err := srv.input.PointerButton(10, 20, 1, true); err != nil {
		t.Fatal(err)
	}
	if err := srv.input.Key(0x1e, true); err != nil {
		t.Fatal(err)
	}
	if err := srv.input.Unicode('A'); err != nil {
		t.Fatal(err)
	}
	if len(handler.moves) != 1 || handler.moves[0] != ([2]int{10, 20}) {
		t.Fatalf("unexpected moves: %#v", handler.moves)
	}
	if len(handler.buttons) != 1 || handler.buttons[0].buttons != 1 || !handler.buttons[0].down {
		t.Fatalf("unexpected buttons: %#v", handler.buttons)
	}
	if len(handler.keys) != 1 || handler.keys[0].scancode != 0x1e || !handler.keys[0].down {
		t.Fatalf("unexpected keys: %#v", handler.keys)
	}
	if len(handler.unicode) != 1 || handler.unicode[0] != 'A' {
		t.Fatalf("unexpected unicode: %#v", handler.unicode)
	}

	srv.SetInputHandler(nil)
	if err := srv.input.PointerMove(1, 2); err != nil {
		t.Fatal(err)
	}
}

func TestMobileServerLifecycleAndSubmitFrame(t *testing.T) {
	srv := NewServer()
	if err := srv.Start(0); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for srv.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		t.Fatal("server did not expose address")
	}
	if err := srv.SubmitFrame(1, 1, 4, 4, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if err := srv.SubmitFrame(1, 1, 2, 2, []byte{1, 2}); err == nil {
		t.Fatal("expected unsupported pixel stride")
	}
	if err := srv.Stop(); err != nil {
		t.Fatal(err)
	}
	if srv.Addr() != "" {
		t.Fatal("expected empty address after stop")
	}
	if err := srv.Stop(); err != nil {
		t.Fatal(err)
	}
}
