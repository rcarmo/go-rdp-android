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
	mu                   sync.Mutex
	ctx                  context.Context
	cancel               context.CancelFunc
	done                 chan error
	server               *rdpserver.Server
	addr                 string
	frames               *FrameQueue
	encodedFrames        *EncodedFrameQueue
	input                *mobileInputSink
	username             string
	password             string
	securityMode         rdpserver.SecurityMode
	failedAuthLimit      int
	failedAuthBackoff    time.Duration
	failedAuthBackoffMax time.Duration
}

// NewServer creates a mobile bridge server instance.
func NewServer() *Server {
	return &Server{
		frames:        NewFrameQueue(2),
		encodedFrames: NewEncodedFrameQueue(2),
		input:         &mobileInputSink{},
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
	s.encodedFrames.Drain()
	srv, err := rdpserver.New(rdpserver.Config{
		Addr:          addr,
		Width:         1280,
		Height:        720,
		Authenticator: auth,
		Policy: rdpserver.AccessPolicy{
			SecurityMode:         s.securityMode,
			FailedAuthLimit:      s.failedAuthLimit,
			FailedAuthBackoff:    s.failedAuthBackoff,
			FailedAuthBackoffMax: s.failedAuthBackoffMax,
		},
		H264: s.encodedFrames,
	}, s.frames, s.input)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.frames.Drain()
		s.encodedFrames.Drain()
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
		s.encodedFrames.Drain()
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	case <-time.After(2 * time.Second):
		// Android lifecycle shutdown must not hang the UI/service teardown path.
		// The listener has been canceled/closed above; let the goroutine drain asynchronously.
		s.frames.Drain()
		s.encodedFrames.Drain()
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

// SubmitH264Frame queues an encoded H.264/AVC access unit for future transport wiring.
func (s *Server) SubmitH264Frame(presentationTimeUs int64, keyFrame bool, codecConfig bool, data []byte) error {
	return s.encodedFrames.Submit(EncodedFrame{
		PresentationTimeUS: presentationTimeUs,
		KeyFrame:           keyFrame,
		CodecConfig:        codecConfig,
		Data:               data,
	})
}

// SetCredentials configures a simple username/password authenticator for future sessions.
func (s *Server) SetCredentials(username, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.username = username
	s.password = password
}

// SetSecurityMode configures the accepted RDP security mode for future sessions.
func (s *Server) SetSecurityMode(mode string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch rdpserver.SecurityMode(mode) {
	case "", rdpserver.SecurityModeNegotiate:
		s.securityMode = rdpserver.SecurityModeNegotiate
	case rdpserver.SecurityModeRDPOnly, rdpserver.SecurityModeTLSOnly, rdpserver.SecurityModeNLARequired:
		s.securityMode = rdpserver.SecurityMode(mode)
	default:
		return fmt.Errorf("unsupported security mode %q", mode)
	}
	return nil
}

// SetFailedAuthPolicy configures host-level failed-auth backoff for future sessions.
func (s *Server) SetFailedAuthPolicy(limit int, backoffMillis int, backoffMaxMillis int) error {
	if limit < 0 || backoffMillis < 0 || backoffMaxMillis < 0 {
		return errors.New("failed auth policy values must be non-negative")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	backoff := time.Duration(backoffMillis) * time.Millisecond
	backoffMax := time.Duration(backoffMaxMillis) * time.Millisecond
	if backoffMax < backoff {
		backoffMax = backoff
	}
	s.failedAuthLimit = limit
	s.failedAuthBackoff = backoff
	s.failedAuthBackoffMax = backoffMax
	return nil
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

// BitmapRLEFrames returns the total number of bitmap batches containing compressed RLE rectangles.
func (s *Server) BitmapRLEFrames() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.BitmapRLEFrames()
}

// BitmapRLEBytes returns the total number of compressed bitmap RLE payload bytes sent by the running server.
func (s *Server) BitmapRLEBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.BitmapRLEBytes()
}

// BitmapRLESavedBytes returns the estimated bytes saved by bitmap RLE versus uncompressed rectangles.
func (s *Server) BitmapRLESavedBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.BitmapRLESavedBytes()
}

// NSCodecFrames returns the total number of experimental NSCodec update batches sent by the running server.
func (s *Server) NSCodecFrames() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.NSCodecFrames()
}

// NSCodecBytes returns the total number of experimental NSCodec SurfaceBits command bytes sent by the running server.
func (s *Server) NSCodecBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.NSCodecBytes()
}

// JPEGCodecFrames returns the total number of experimental JPEG bitmap-codec update batches sent by the running server.
func (s *Server) JPEGCodecFrames() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.JPEGCodecFrames()
}

// JPEGCodecBytes returns the total number of experimental JPEG bitmap-codec SurfaceBits command bytes sent by the running server.
func (s *Server) JPEGCodecBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.JPEGCodecBytes()
}

// PNGCodecFrames returns the total number of experimental PNG bitmap-codec update batches sent by the running server.
func (s *Server) PNGCodecFrames() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.PNGCodecFrames()
}

// PNGCodecBytes returns the total number of experimental PNG bitmap-codec SurfaceBits command bytes sent by the running server.
func (s *Server) PNGCodecBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.PNGCodecBytes()
}

// RDPGFXFrames returns the total number of RDPGFX frame batches sent by the running server.
func (s *Server) RDPGFXFrames() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.RDPGFXFrames()
}

// RDPGFXBytes returns the total number of RDPGFX dynamic-channel payload bytes sent by the running server.
func (s *Server) RDPGFXBytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.RDPGFXBytes()
}

// H264Frames returns the total number of H.264/AVC frame batches sent by the running server.
func (s *Server) H264Frames() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.H264Frames()
}

// H264Bytes returns the total number of H.264/AVC payload bytes sent by the running server.
func (s *Server) H264Bytes() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return 0
	}
	return s.server.H264Bytes()
}

// H264Status returns the latest H.264 capability/emission status reason.
func (s *Server) H264Status() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return "stopped"
	}
	return s.server.H264Status()
}

// GraphicsPath returns the active/last observed graphics transport path.
func (s *Server) GraphicsPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.server == nil {
		return "stopped"
	}
	return s.server.GraphicsPath()
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

// SubmittedFrames returns the number of frames accepted by the bounded RGBA queue.
func (s *Server) SubmittedFrames() int64 { return s.frames.Submitted() }

// DroppedFrames returns the number of queued RGBA frames dropped in favor of newer frames.
func (s *Server) DroppedFrames() int64 { return s.frames.Dropped() }

// QueuedFrames returns the current bounded RGBA queue depth.
func (s *Server) QueuedFrames() int64 { return s.frames.Depth() }

// H264SubmittedFrames returns the number of encoded H.264 access units accepted by the bounded queue.
func (s *Server) H264SubmittedFrames() int64 { return s.encodedFrames.Submitted() }

// H264DroppedFrames returns the number of encoded H.264 access units dropped in favor of newer frames.
func (s *Server) H264DroppedFrames() int64 { return s.encodedFrames.Dropped() }

// H264QueuedFrames returns the current bounded H.264 access-unit queue depth.
func (s *Server) H264QueuedFrames() int64 { return s.encodedFrames.Depth() }

// StartServer starts the default singleton server. It mirrors the current Kotlin stub shape.
func StartServer(port int) error { return defaultServer.Start(port) }

// StopServer stops the default singleton server.
func StopServer() error { return defaultServer.Stop() }

// SubmitFrame queues a frame on the default singleton server.
func SubmitFrame(width, height, pixelStride, rowStride int, data []byte) error {
	return defaultServer.SubmitFrame(width, height, pixelStride, rowStride, data)
}

// SubmitH264Frame queues an encoded H.264/AVC access unit on the default singleton server.
func SubmitH264Frame(presentationTimeUs int64, keyFrame bool, codecConfig bool, data []byte) error {
	return defaultServer.SubmitH264Frame(presentationTimeUs, keyFrame, codecConfig, data)
}

// SetCredentials configures simple username/password authentication for future sessions.
func SetCredentials(username, password string) { defaultServer.SetCredentials(username, password) }

// SetSecurityMode configures the accepted RDP security mode for future sessions.
func SetSecurityMode(mode string) error { return defaultServer.SetSecurityMode(mode) }

// SetFailedAuthPolicy configures failed-auth backoff for future sessions.
func SetFailedAuthPolicy(limit int, backoffMillis int, backoffMaxMillis int) error {
	return defaultServer.SetFailedAuthPolicy(limit, backoffMillis, backoffMaxMillis)
}

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

// BitmapRLEFrames returns the bitmap RLE frame-batch count for the default server.
func BitmapRLEFrames() int64 { return defaultServer.BitmapRLEFrames() }

// BitmapRLEBytes returns the compressed bitmap RLE payload bytes for the default server.
func BitmapRLEBytes() int64 { return defaultServer.BitmapRLEBytes() }

// BitmapRLESavedBytes returns estimated bytes saved by bitmap RLE for the default server.
func BitmapRLESavedBytes() int64 { return defaultServer.BitmapRLESavedBytes() }

// NSCodecFrames returns the experimental NSCodec update-batch count for the default server.
func NSCodecFrames() int64 { return defaultServer.NSCodecFrames() }

// NSCodecBytes returns the experimental NSCodec SurfaceBits command bytes for the default server.
func NSCodecBytes() int64 { return defaultServer.NSCodecBytes() }

// JPEGCodecFrames returns the experimental JPEG bitmap-codec update count for the default server.
func JPEGCodecFrames() int64 { return defaultServer.JPEGCodecFrames() }

// JPEGCodecBytes returns the experimental JPEG bitmap-codec SurfaceBits command bytes for the default server.
func JPEGCodecBytes() int64 { return defaultServer.JPEGCodecBytes() }

// PNGCodecFrames returns the experimental PNG bitmap-codec update count for the default server.
func PNGCodecFrames() int64 { return defaultServer.PNGCodecFrames() }

// PNGCodecBytes returns the experimental PNG bitmap-codec SurfaceBits command bytes for the default server.
func PNGCodecBytes() int64 { return defaultServer.PNGCodecBytes() }

// RDPGFXFrames returns the RDPGFX frame count for the default server.
func RDPGFXFrames() int64 { return defaultServer.RDPGFXFrames() }

// RDPGFXBytes returns the RDPGFX dynamic-channel payload bytes for the default server.
func RDPGFXBytes() int64 { return defaultServer.RDPGFXBytes() }

// H264Frames returns the H.264/AVC frame count for the default server.
func H264Frames() int64 { return defaultServer.H264Frames() }

// H264Bytes returns the H.264/AVC payload bytes for the default server.
func H264Bytes() int64 { return defaultServer.H264Bytes() }

// H264Status returns the latest H.264 capability/emission status reason for the default server.
func H264Status() string { return defaultServer.H264Status() }

// GraphicsPath returns the active/last observed graphics transport path for the default server.
func GraphicsPath() string { return defaultServer.GraphicsPath() }

// DVCFragments returns the dynamic virtual channel fragment PDU count for the default server.
func DVCFragments() int64 { return defaultServer.DVCFragments() }

// SubmittedFrames returns the number of frames accepted by the default server queue.
func SubmittedFrames() int64 { return defaultServer.SubmittedFrames() }

// DroppedFrames returns the number of queued frames dropped by the default server queue.
func DroppedFrames() int64 { return defaultServer.DroppedFrames() }

// QueuedFrames returns the current default server RGBA queue depth.
func QueuedFrames() int64 { return defaultServer.QueuedFrames() }

// H264SubmittedFrames returns the number of encoded H.264 access units accepted by the default server queue.
func H264SubmittedFrames() int64 { return defaultServer.H264SubmittedFrames() }

// H264DroppedFrames returns the number of encoded H.264 access units dropped by the default server queue.
func H264DroppedFrames() int64 { return defaultServer.H264DroppedFrames() }

// H264QueuedFrames returns the current default server H.264 access-unit queue depth.
func H264QueuedFrames() int64 { return defaultServer.H264QueuedFrames() }

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
