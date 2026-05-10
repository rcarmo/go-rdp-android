// Package rdpserver contains the server-side RDP skeleton.
package rdpserver

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	"github.com/rcarmo/go-rdp-android/internal/input"
)

// Config controls the RDP server.
type Config struct {
	Addr          string
	Width         int
	Height        int
	Authenticator Authenticator
	Policy        AccessPolicy
	TLS           TLSSettings
}

// Server is the native Android RDP server core.
type Server struct {
	cfg    Config
	frames frame.Source
	input  input.Sink

	tlsConfig      *tls.Config
	tlsFingerprint string
	authLimiter    *authBackoffLimiter

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
	policy, err := normalizeAccessPolicy(cfg.Policy)
	if err != nil {
		return nil, err
	}
	cfg.Policy = policy
	tlsConfig, fingerprint, err := resolveTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}
	return &Server{cfg: cfg, frames: frames, input: sink, tlsConfig: tlsConfig, tlsFingerprint: fingerprint, authLimiter: newAuthBackoffLimiter(cfg.Policy)}, nil
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

// TLSFingerprintSHA256 returns the current server-certificate fingerprint.
func (s *Server) TLSFingerprintSHA256() string { return s.tlsFingerprint }

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	if !s.cfg.Policy.remoteAllowed(conn.RemoteAddr()) {
		log.Printf("rdp connection denied by CIDR policy from %s", conn.RemoteAddr())
		return
	}
	info, secureConn, err := performInitialHandshakeWithModeAndTLS(conn, s.cfg.Policy.SecurityMode, s.tlsConfig)
	if err != nil {
		log.Printf("rdp initial handshake failed from %s: %v", conn.RemoteAddr(), err)
		return
	}
	conn = secureConn
	log.Printf("rdp initial handshake from %s: requested=0x%08x selected=0x%08x cookie=%q tls_fp=%s", conn.RemoteAddr(), info.RequestedProtocols, info.SelectedProtocol, sanitizeForLog(info.Cookie, 80), s.tlsFingerprint)
	if info.SelectedProtocol == protocolHybrid {
		remote := remoteHost(conn.RemoteAddr())
		if wait := s.authLimiter.lockoutRemaining(remote, ""); wait > 0 {
			log.Printf("rdp NLA/CredSSP locked out from %s: retry in %s", conn.RemoteAddr(), wait.Round(time.Second))
			return
		}
		clientInfo, err := performCredSSPWithBindings(conn, s.cfg.Authenticator, info.TLSPublicKeyCandidates)
		if err != nil {
			wait := s.authLimiter.recordFailure(remote, "")
			if wait > 0 {
				log.Printf("rdp NLA/CredSSP failed from %s: %v (retry in %s)", conn.RemoteAddr(), err, wait.Round(time.Second))
			} else {
				log.Printf("rdp NLA/CredSSP failed from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}
		s.authLimiter.recordSuccess(remote, "")
		if err := s.checkAndRecordAuthPolicy(remote, clientInfo.UserName, s.cfg.Policy.userAllowed(clientInfo.UserName)); err != nil {
			log.Printf("rdp NLA/CredSSP denied from %s: user=%q err=%v", conn.RemoteAddr(), sanitizeForLog(clientInfo.UserName, 64), err)
			return
		}
		log.Printf("rdp NLA/CredSSP authenticated from %s: user=%q domain=%q", conn.RemoteAddr(), sanitizeForLog(clientInfo.UserName, 64), sanitizeForLog(clientInfo.Domain, 64))
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
	if err := handleMCSDomainSequence(conn, s.frames, s.input, sessionWidth, sessionHeight, s.cfg.Authenticator, s.cfg.Policy, s.authLimiter, info.SelectedProtocol, mcsInfo.ClientChannels); err != nil {
		log.Printf("rdp MCS domain sequence failed from %s: %v", conn.RemoteAddr(), err)
		return
	}
	log.Printf("rdp MCS domain sequence finished for %s", conn.RemoteAddr())
	// The next phase is Security Exchange / Client Info handling.
}

func (s *Server) checkAndRecordAuthPolicy(remote, username string, userAllowed bool) error {
	if wait := s.authLimiter.lockoutRemaining(remote, username); wait > 0 {
		return fmt.Errorf("auth temporarily locked, retry in %s", wait.Round(time.Second))
	}
	if !userAllowed {
		s.authLimiter.recordFailure(remote, username)
		return fmt.Errorf("user not allowed by policy")
	}
	s.authLimiter.recordSuccess(remote, username)
	return nil
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
