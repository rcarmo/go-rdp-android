// Package rdpserver contains the server-side RDP skeleton.
package rdpserver

import (
	"context"
	"errors"
	"log"
	"net"
	"sync"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	"github.com/rcarmo/go-rdp-android/internal/input"
)

// Config controls the RDP server.
type Config struct {
	Addr          string
	Width         int
	Height        int
	Authenticator Authenticator
}

// Server is the native Android RDP server core.
type Server struct {
	cfg    Config
	frames frame.Source
	input  input.Sink

	mu sync.Mutex
	ln net.Listener
}

// New creates a Server.
func New(cfg Config, frames frame.Source, sink input.Sink) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = ":3390"
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, errors.New("width and height must be positive")
	}
	return &Server{cfg: cfg, frames: frames, input: sink}, nil
}

// Listen starts accepting TCP connections.
// The protocol implementation is intentionally a stub until the server-side
// X.224/MCS/GCC handshake is extracted from go-rdp.
func (s *Server) Listen(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ctx, ln)
}

// Serve accepts connections from an existing listener.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	defer func() {
		_ = ln.Close()
		s.mu.Lock()
		if s.ln == ln {
			s.ln = nil
		}
		s.mu.Unlock()
	}()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go s.handleConn(conn)
	}
}

// Addr returns the listener address if the server is running.
func (s *Server) Addr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln == nil {
		return nil
	}
	return s.ln.Addr()
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	info, secureConn, err := performInitialHandshake(conn)
	if err != nil {
		log.Printf("rdp initial handshake failed from %s: %v", conn.RemoteAddr(), err)
		return
	}
	conn = secureConn
	log.Printf("rdp initial handshake from %s: requested=0x%08x selected=0x%08x cookie=%q", conn.RemoteAddr(), info.RequestedProtocols, info.SelectedProtocol, info.Cookie)
	if info.SelectedProtocol == protocolHybrid {
		clientInfo, err := performCredSSPWithBindings(conn, s.cfg.Authenticator, info.TLSPublicKeyCandidates)
		if err != nil {
			log.Printf("rdp NLA/CredSSP failed from %s: %v", conn.RemoteAddr(), err)
			return
		}
		log.Printf("rdp NLA/CredSSP authenticated from %s: user=%q domain=%q", conn.RemoteAddr(), clientInfo.UserName, clientInfo.Domain)
	}

	mcsInfo, err := readMCSConnectInitial(conn)
	if err != nil {
		log.Printf("rdp MCS Connect-Initial failed from %s: %v", conn.RemoteAddr(), err)
		return
	}
	sessionWidth, sessionHeight := chooseSessionDesktopSize(s.cfg.Width, s.cfg.Height, mcsInfo.ClientDisplay.DesktopWidth, mcsInfo.ClientDisplay.DesktopHeight)
	log.Printf(
		"rdp MCS Connect-Initial from %s: appTag=%d payload=%d userData=%d channels=%d clientDesktop=%dx%d monitorLayout=%t monitorCount=%d sessionDesktop=%dx%d",
		conn.RemoteAddr(),
		mcsInfo.ApplicationTag,
		mcsInfo.PayloadLength,
		mcsInfo.UserDataLength,
		len(mcsInfo.ClientChannels),
		mcsInfo.ClientDisplay.DesktopWidth,
		mcsInfo.ClientDisplay.DesktopHeight,
		mcsInfo.ClientDisplay.MonitorLayoutPresent,
		mcsInfo.ClientDisplay.MonitorCount,
		sessionWidth,
		sessionHeight,
	)
	if err := writeMCSConnectResponse(conn, info.SelectedProtocol, mcsInfo.ClientChannels); err != nil {
		log.Printf("rdp MCS Connect-Response failed to %s: %v", conn.RemoteAddr(), err)
		return
	}
	log.Printf("rdp MCS Connect-Response sent to %s", conn.RemoteAddr())
	if err := handleMCSDomainSequence(conn, s.frames, s.input, sessionWidth, sessionHeight, s.cfg.Authenticator, info.SelectedProtocol, mcsInfo.ClientChannels); err != nil {
		log.Printf("rdp MCS domain sequence failed from %s: %v", conn.RemoteAddr(), err)
		return
	}
	log.Printf("rdp MCS domain sequence finished for %s", conn.RemoteAddr())
	// The next phase is Security Exchange / Client Info handling.
}

// Close stops the listener.
func chooseSessionDesktopSize(defaultWidth, defaultHeight int, clientWidth, clientHeight uint16) (int, int) {
	width := defaultWidth
	height := defaultHeight
	if clientWidth > 0 {
		width = clampDesktopDimension(int(clientWidth), width)
	}
	if clientHeight > 0 {
		height = clampDesktopDimension(int(clientHeight), height)
	}
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	return width, height
}

func clampDesktopDimension(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value < 64 {
		return 64
	}
	if value > 8192 {
		return 8192
	}
	return value
}

func (s *Server) Close() error {
	s.mu.Lock()
	ln := s.ln
	s.ln = nil
	s.mu.Unlock()
	if ln != nil {
		return ln.Close()
	}
	return nil
}
