// Package mobile exposes a gomobile-friendly API for the Android shell.
package mobile

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	"github.com/rcarmo/go-rdp-android/internal/input"
	"github.com/rcarmo/go-rdp-android/internal/rdpserver"
)

var defaultServer = NewServer()

// Server is a gomobile-friendly wrapper around the RDP server core.
type Server struct {
	mu       sync.Mutex
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan error
	server   *rdpserver.Server
	frames   *FrameQueue
	input    *mobileInputSink
	username string
	password string
}

// NewServer creates a mobile bridge server instance.
func NewServer() *Server {
	return &Server{
		frames: NewFrameQueue(2),
		input:  &mobileInputSink{},
	}
}

// Start begins listening on the given TCP port.
func (s *Server) Start(port int) error {
	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil
	}
	addr := fmt.Sprintf(":%d", port)
	var auth rdpserver.Authenticator
	if s.username != "" || s.password != "" {
		auth = rdpserver.StaticCredentials{Username: s.username, Password: s.password}
	}
	srv, err := rdpserver.New(rdpserver.Config{Addr: addr, Width: 1280, Height: 720, Authenticator: auth}, s.frames, s.input)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel
	s.server = srv
	done := make(chan error, 1)
	s.done = done
	go func() { done <- srv.Listen(ctx) }()
	return nil
}

// Stop terminates the server and releases queued frames.
func (s *Server) Stop() error {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	srv := s.server
	s.cancel = nil
	s.done = nil
	s.server = nil
	s.ctx = nil
	s.mu.Unlock()

	if cancel == nil {
		return nil
	}
	cancel()
	if srv != nil {
		_ = srv.Close()
	}
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	case <-time.After(2 * time.Second):
		// Android lifecycle shutdown must not hang the UI/service teardown path.
		// The listener has been canceled/closed above; let the goroutine drain asynchronously.
	}
	return nil
}

// SubmitFrame queues an Android RGBA_8888 frame for the RDP server.
func (s *Server) SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error {
	if pixelStride != 4 {
		return fmt.Errorf("unsupported pixel stride %d", pixelStride)
	}
	return s.frames.Submit(frame.Frame{
		Width:     width,
		Height:    height,
		Stride:    rowStride,
		Format:    frame.PixelFormatRGBA8888,
		Timestamp: time.Now(),
		Data:      append([]byte(nil), data...),
	})
}

// SetCredentials configures a simple username/password authenticator for future sessions.
func (s *Server) SetCredentials(username, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.username = username
	s.password = password
}

// SetInputHandler installs the callback target for decoded RDP input events.
func (s *Server) SetInputHandler(handler InputHandler) {
	s.input.SetHandler(handler)
}

// Addr returns the active listen address, or an empty string if the server is stopped.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil || s.server.Addr() == nil {
		return ""
	}
	return s.server.Addr().String()
}

// StartServer starts the default singleton server. It mirrors the current Kotlin stub shape.
func StartServer(port int) error { return defaultServer.Start(port) }

// StopServer stops the default singleton server.
func StopServer() error { return defaultServer.Stop() }

// SubmitFrame queues a frame on the default singleton server.
func SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error {
	return defaultServer.SubmitFrame(width, height, pixelStride, rowStride, data)
}

// SetCredentials configures simple username/password authentication for future sessions.
func SetCredentials(username, password string) { defaultServer.SetCredentials(username, password) }

// SetInputHandler installs the callback target on the default singleton server.
func SetInputHandler(handler InputHandler) { defaultServer.SetInputHandler(handler) }

// FrameQueue is a bounded latest-frame queue implementing frame.Source.
type FrameQueue struct {
	mu     sync.Mutex
	frames chan frame.Frame
	closed bool
}

// NewFrameQueue creates a bounded frame queue. Capacity values below one are rounded up.
func NewFrameQueue(capacity int) *FrameQueue {
	if capacity < 1 {
		capacity = 1
	}
	return &FrameQueue{frames: make(chan frame.Frame, capacity)}
}

// Submit enqueues a frame, dropping the oldest frame when the queue is full.
func (q *FrameQueue) Submit(f frame.Frame) error {
	if f.Width <= 0 || f.Height <= 0 {
		return errors.New("frame dimensions must be positive")
	}
	if len(f.Data) == 0 {
		return errors.New("frame data is empty")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return errors.New("frame queue is closed")
	}
	select {
	case q.frames <- f:
		return nil
	default:
		select {
		case <-q.frames:
		default:
		}
		q.frames <- f
		return nil
	}
}

// Frames returns the receive side consumed by the RDP server.
func (q *FrameQueue) Frames() <-chan frame.Frame { return q.frames }

// Close closes the queue.
func (q *FrameQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		close(q.frames)
		q.closed = true
	}
	return nil
}

// InputHandler is implemented by the gomobile/Kotlin side to receive decoded RDP input.
type InputHandler interface {
	PointerMove(x int, y int)
	PointerButton(x int, y int, buttons int, down bool)
	Key(scancode int, down bool)
	Unicode(codepoint int)
}

type mobileInputSink struct {
	mu      sync.RWMutex
	handler InputHandler
}

func (s *mobileInputSink) SetHandler(handler InputHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
}

func (s *mobileInputSink) getHandler() InputHandler {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.handler
}

func (s *mobileInputSink) PointerMove(x, y int) error {
	if handler := s.getHandler(); handler != nil {
		handler.PointerMove(x, y)
	}
	return nil
}

func (s *mobileInputSink) PointerButton(x, y int, buttons input.ButtonState, down bool) error {
	if handler := s.getHandler(); handler != nil {
		handler.PointerButton(x, y, int(buttons), down)
	}
	return nil
}

func (s *mobileInputSink) Key(scancode uint16, down bool) error {
	if handler := s.getHandler(); handler != nil {
		handler.Key(int(scancode), down)
	}
	return nil
}

func (s *mobileInputSink) Unicode(r rune) error {
	if handler := s.getHandler(); handler != nil {
		handler.Unicode(int(r))
	}
	return nil
}
