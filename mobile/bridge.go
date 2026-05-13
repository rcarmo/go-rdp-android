// Package mobile exposes a gomobile-friendly API for the Android shell.
package mobile

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
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
	addr     string
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
	s.frames.Drain()
	srv, err := rdpserver.New(rdpserver.Config{Addr: addr, Width: 1280, Height: 720, Authenticator: auth}, s.frames, s.input)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.frames.Drain()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel
	s.server = srv
	s.addr = ln.Addr().String()
	done := make(chan error, 1)
	s.done = done
	go func() { done <- srv.Serve(ctx, ln) }()
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
	s.addr = ""
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
		s.frames.Drain()
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	case <-time.After(2 * time.Second):
		// Android lifecycle shutdown must not hang the UI/service teardown path.
		// The listener has been canceled/closed above; let the goroutine drain asynchronously.
		s.frames.Drain()
	}
	return nil
}

// SubmitFrame queues an Android RGBA_8888 frame for the RDP server.
func (s *Server) SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error {
	if width <= 0 || height <= 0 {
		return errors.New("frame dimensions must be positive")
	}
	if pixelStride != 4 {
		return fmt.Errorf("unsupported pixel stride %d", pixelStride)
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/pixelStride {
		return errors.New("frame row stride overflows")
	}
	minRowStride := width * pixelStride
	if rowStride < minRowStride {
		return fmt.Errorf("row stride %d is smaller than minimum %d", rowStride, minRowStride)
	}
	if height > 1 && rowStride > (maxInt-minRowStride)/(height-1) {
		return errors.New("frame byte size overflows")
	}
	minBytes := minRowStride
	if height > 1 {
		minBytes += rowStride * (height - 1)
	}
	if len(data) < minBytes {
		return fmt.Errorf("frame data too short: got %d bytes, need at least %d", len(data), minBytes)
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
	return s.addr
}

// TLSFingerprintSHA256 returns the current TLS certificate fingerprint when running.
func (s *Server) TLSFingerprintSHA256() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return ""
	}
	return s.server.TLSFingerprintSHA256()
}

// ActiveConnections returns the number of active accepted TCP sessions.
func (s *Server) ActiveConnections() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.ActiveConnections()
}

// AcceptedConnections returns the total number of accepted TCP sessions for the running server.
func (s *Server) AcceptedConnections() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.AcceptedConnections()
}

// HandshakeFailures returns the total number of pre-auth handshake failures for the running server.
func (s *Server) HandshakeFailures() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.HandshakeFailures()
}

// AuthFailures returns the total number of auth failures for the running server.
func (s *Server) AuthFailures() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.AuthFailures()
}

// InputEvents returns the total number of decoded input callbacks for the running server.
func (s *Server) InputEvents() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.InputEvents()
}

// RDPEIContacts returns the total number of decoded RDPEI touch contacts for the running server.
func (s *Server) RDPEIContacts() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.RDPEIContacts()
}

// FramesSent returns the total number of bitmap frame update batches sent by the running server.
func (s *Server) FramesSent() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.FramesSent()
}

// BitmapBytes returns the total number of bitmap update payload bytes sent by the running server.
func (s *Server) BitmapBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.BitmapBytes()
}

// DVCFragments returns the dynamic virtual channel fragment PDU count for the running server.
func (s *Server) DVCFragments() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.DVCFragments()
}

// SubmittedFrames returns the number of frames accepted by the bounded queue.
func (s *Server) SubmittedFrames() int64 { return s.frames.Submitted() }

// DroppedFrames returns the number of queued frames dropped in favor of newer frames.
func (s *Server) DroppedFrames() int64 { return s.frames.Dropped() }

// QueuedFrames returns the current bounded queue depth.
func (s *Server) QueuedFrames() int64 { return s.frames.Depth() }

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

// TLSFingerprintSHA256 returns the default server TLS cert fingerprint when running.
func TLSFingerprintSHA256() string { return defaultServer.TLSFingerprintSHA256() }

// Addr returns the default singleton server listen address when running.
func Addr() string { return defaultServer.Addr() }

// ActiveConnections returns the active connection count for the default server.
func ActiveConnections() int64 { return defaultServer.ActiveConnections() }

// AcceptedConnections returns the accepted connection count for the default server.
func AcceptedConnections() int64 { return defaultServer.AcceptedConnections() }

// HandshakeFailures returns the handshake failure count for the default server.
func HandshakeFailures() int64 { return defaultServer.HandshakeFailures() }

// AuthFailures returns the auth failure count for the default server.
func AuthFailures() int64 { return defaultServer.AuthFailures() }

// InputEvents returns the input event count for the default server.
func InputEvents() int64 { return defaultServer.InputEvents() }

// RDPEIContacts returns the RDPEI contact count for the default server.
func RDPEIContacts() int64 { return defaultServer.RDPEIContacts() }

// FramesSent returns the sent bitmap frame count for the default server.
func FramesSent() int64 { return defaultServer.FramesSent() }

// BitmapBytes returns the bitmap update payload bytes sent by the default server.
func BitmapBytes() int64 { return defaultServer.BitmapBytes() }

// DVCFragments returns the dynamic virtual channel fragment PDU count for the default server.
func DVCFragments() int64 { return defaultServer.DVCFragments() }

// SubmittedFrames returns the number of frames accepted by the default server queue.
func SubmittedFrames() int64 { return defaultServer.SubmittedFrames() }

// DroppedFrames returns the number of queued frames dropped by the default server queue.
func DroppedFrames() int64 { return defaultServer.DroppedFrames() }

// QueuedFrames returns the current default server queue depth.
func QueuedFrames() int64 { return defaultServer.QueuedFrames() }

// FrameQueue is a bounded latest-frame queue implementing frame.Source.
type FrameQueue struct {
	mu        sync.Mutex
	frames    chan frame.Frame
	closed    bool
	submitted atomic.Int64
	dropped   atomic.Int64
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
	q.submitted.Add(1)
	select {
	case q.frames <- f:
		return nil
	default:
		select {
		case <-q.frames:
			q.dropped.Add(1)
		default:
		}
		q.frames <- f
		return nil
	}
}

// Frames returns the receive side consumed by the RDP server.
func (q *FrameQueue) Frames() <-chan frame.Frame { return q.frames }

// Submitted returns the number of successfully accepted Submit calls.
func (q *FrameQueue) Submitted() int64 { return q.submitted.Load() }

// Dropped returns the number of queued frames discarded to make room for newer frames.
func (q *FrameQueue) Dropped() int64 { return q.dropped.Load() }

// Depth returns the current number of queued frames.
func (q *FrameQueue) Depth() int64 { return int64(len(q.frames)) }

// Drain discards queued frames without closing the queue, allowing reuse across Android service restarts.
func (q *FrameQueue) Drain() int64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	var drained int64
	for {
		select {
		case _, ok := <-q.frames:
			if !ok {
				return drained
			}
			drained++
		default:
			return drained
		}
	}
}

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
	PointerWheel(x int, y int, delta int, horizontal bool)
	Key(scancode int, down bool)
	Unicode(codepoint int)
	TouchFrameStart(contactCount int)
	TouchContact(contactID int, x int, y int, flags int)
	TouchFrameEnd()
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

func (s *mobileInputSink) PointerWheel(x, y int, delta int, horizontal bool) error {
	if handler := s.getHandler(); handler != nil {
		handler.PointerWheel(x, y, delta, horizontal)
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

func (s *mobileInputSink) TouchFrame(contacts []input.TouchContact) error {
	if handler := s.getHandler(); handler != nil {
		handler.TouchFrameStart(len(contacts))
		for _, contact := range contacts {
			handler.TouchContact(int(contact.ID), contact.X, contact.Y, int(contact.Flags))
		}
		handler.TouchFrameEnd()
	}
	return nil
}
