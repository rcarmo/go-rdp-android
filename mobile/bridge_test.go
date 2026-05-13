package mobile

import (
	"net"
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	"github.com/rcarmo/go-rdp-android/internal/input"
)

func TestFrameQueueDropsOldest(t *testing.T) {
	q := NewFrameQueue(1)
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3, 4}}); err != nil {
		t.Fatal(err)
	}
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{5, 6, 7, 8}}); err != nil {
		t.Fatal(err)
	}
	if q.Submitted() != 2 || q.Dropped() != 1 || q.Depth() != 1 {
		t.Fatalf("unexpected queue stats submitted=%d dropped=%d depth=%d", q.Submitted(), q.Dropped(), q.Depth())
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

func TestFrameQueueValidationDrainAndClose(t *testing.T) {
	q := NewFrameQueue(0)
	if err := q.Submit(frame.Frame{}); err == nil {
		t.Fatal("expected invalid frame error")
	}
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3, 4}}); err != nil {
		t.Fatal(err)
	}
	if drained := q.Drain(); drained != 1 || q.Depth() != 0 {
		t.Fatalf("unexpected drain result drained=%d depth=%d", drained, q.Depth())
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
	if drained := q.Drain(); drained != 0 {
		t.Fatalf("expected closed queue drain to return 0, got %d", drained)
	}
}

func TestFrameQueueDrainAfterCloseDoesNotHang(t *testing.T) {
	q := NewFrameQueue(1)
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	done := make(chan int64, 1)
	go func() { done <- q.Drain() }()
	select {
	case drained := <-done:
		if drained != 0 {
			t.Fatalf("expected zero drained frames, got %d", drained)
		}
	case <-time.After(time.Second):
		t.Fatal("drain hung after close")
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
	wheels  []struct {
		x, y       int
		delta      int
		horizontal bool
	}
	touchFrameStarts []int
	touchFrameEnds   int
	touches          []struct {
		contactID int
		x, y      int
		flags     int
	}
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
func (h *recordingMobileInputHandler) PointerWheel(x int, y int, delta int, horizontal bool) {
	h.wheels = append(h.wheels, struct {
		x, y       int
		delta      int
		horizontal bool
	}{x: x, y: y, delta: delta, horizontal: horizontal})
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
func (h *recordingMobileInputHandler) TouchFrameStart(contactCount int) {
	h.touchFrameStarts = append(h.touchFrameStarts, contactCount)
}
func (h *recordingMobileInputHandler) TouchContact(contactID int, x int, y int, flags int) {
	h.touches = append(h.touches, struct {
		contactID int
		x, y      int
		flags     int
	}{contactID: contactID, x: x, y: y, flags: flags})
}
func (h *recordingMobileInputHandler) TouchFrameEnd() {
	h.touchFrameEnds++
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
	if err := srv.input.PointerWheel(10, 20, -120, false); err != nil {
		t.Fatal(err)
	}
	if err := srv.input.Key(0x1e, true); err != nil {
		t.Fatal(err)
	}
	if err := srv.input.Unicode('A'); err != nil {
		t.Fatal(err)
	}
	if err := srv.input.TouchFrame([]input.TouchContact{{ID: 3, X: 30, Y: 40, Flags: input.TouchDown | input.TouchInRange | input.TouchInContact}}); err != nil {
		t.Fatal(err)
	}
	if len(handler.moves) != 1 || handler.moves[0] != ([2]int{10, 20}) {
		t.Fatalf("unexpected moves: %#v", handler.moves)
	}
	if len(handler.buttons) != 1 || handler.buttons[0].buttons != 1 || !handler.buttons[0].down {
		t.Fatalf("unexpected buttons: %#v", handler.buttons)
	}
	if len(handler.wheels) != 1 || handler.wheels[0].delta != -120 || handler.wheels[0].horizontal {
		t.Fatalf("unexpected wheels: %#v", handler.wheels)
	}
	if len(handler.keys) != 1 || handler.keys[0].scancode != 0x1e || !handler.keys[0].down {
		t.Fatalf("unexpected keys: %#v", handler.keys)
	}
	if len(handler.unicode) != 1 || handler.unicode[0] != 'A' {
		t.Fatalf("unexpected unicode: %#v", handler.unicode)
	}
	if len(handler.touchFrameStarts) != 1 || handler.touchFrameStarts[0] != 1 || handler.touchFrameEnds != 1 {
		t.Fatalf("unexpected touch frame markers starts=%#v ends=%d", handler.touchFrameStarts, handler.touchFrameEnds)
	}
	if len(handler.touches) != 1 || handler.touches[0].contactID != 3 || handler.touches[0].x != 30 || handler.touches[0].y != 40 || handler.touches[0].flags&int(input.TouchDown) == 0 {
		t.Fatalf("unexpected touches: %#v", handler.touches)
	}

	srv.SetInputHandler(nil)
	if err := srv.input.PointerMove(1, 2); err != nil {
		t.Fatal(err)
	}
}

func TestMobileServerCredentials(t *testing.T) {
	srv := NewServer()
	srv.SetCredentials("rui", "secret")
	if srv.username != "rui" || srv.password != "secret" {
		t.Fatalf("credentials not stored: %#v", srv)
	}
}

func TestMobileServerSubmitFrameValidation(t *testing.T) {
	srv := NewServer()
	cases := []struct {
		name        string
		width       int
		height      int
		pixelStride int
		rowStride   int
		data        []byte
	}{
		{name: "zero width", width: 0, height: 1, pixelStride: 4, rowStride: 4, data: []byte{1, 2, 3, 4}},
		{name: "unsupported pixel stride", width: 1, height: 1, pixelStride: 2, rowStride: 2, data: []byte{1, 2}},
		{name: "short row stride", width: 2, height: 1, pixelStride: 4, rowStride: 4, data: make([]byte, 8)},
		{name: "short data", width: 2, height: 2, pixelStride: 4, rowStride: 8, data: make([]byte, 15)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := srv.SubmitFrame(tc.width, tc.height, tc.pixelStride, tc.rowStride, tc.data); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if srv.SubmittedFrames() != 0 || srv.QueuedFrames() != 0 {
		t.Fatalf("invalid frames should not be queued submitted=%d queued=%d", srv.SubmittedFrames(), srv.QueuedFrames())
	}
}

func TestMobileServerStartReportsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	srv := NewServer()
	if err := srv.Start(port); err == nil {
		t.Fatal("expected listen error for occupied port")
	}
	if srv.Addr() != "" {
		t.Fatalf("expected no address after failed start, got %q", srv.Addr())
	}
	if err := srv.SubmitFrame(1, 1, 4, 4, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if srv.QueuedFrames() != 1 {
		t.Fatalf("expected queue to remain reusable after failed start, got depth %d", srv.QueuedFrames())
	}
}

func TestMobileServerRejectsInvalidPort(t *testing.T) {
	srv := NewServer()
	if err := srv.Start(-1); err == nil {
		t.Fatal("expected invalid low port error")
	}
	if err := srv.Start(65536); err == nil {
		t.Fatal("expected invalid high port error")
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
	if srv.SubmittedFrames() != 1 || srv.DroppedFrames() != 0 || srv.QueuedFrames() != 1 {
		t.Fatalf("unexpected server frame stats submitted=%d dropped=%d queued=%d", srv.SubmittedFrames(), srv.DroppedFrames(), srv.QueuedFrames())
	}
	conn, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	deadline = time.Now().Add(time.Second)
	for srv.AcceptedConnections() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.AcceptedConnections() != 1 {
		t.Fatalf("expected accepted connection counter, got %d", srv.AcceptedConnections())
	}
	if err := srv.SubmitFrame(1, 1, 2, 2, []byte{1, 2}); err == nil {
		t.Fatal("expected unsupported pixel stride")
	}
	if err := srv.Stop(); err != nil {
		t.Fatal(err)
	}
	if srv.QueuedFrames() != 0 {
		t.Fatalf("expected queued frames to be drained on stop, got %d", srv.QueuedFrames())
	}
	if srv.Addr() != "" {
		t.Fatal("expected empty address after stop")
	}
	if err := srv.Stop(); err != nil {
		t.Fatal(err)
	}
}
