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
	"sync/atomic"
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
	H264          H264Source
	RFX           RFXEncoder
	ClearCodec    RDPGFXFrameEncoder
	Progressive   RDPGFXFrameEncoder
	AVC444        RDPGFXFrameEncoder
	AVC444v2      RDPGFXFrameEncoder
	ProgressiveV2 RDPGFXFrameEncoder
}

// Server is the native Android RDP server core.
type Server struct {
	cfg    Config
	frames frame.Source
	input  input.Sink

	tlsConfig              *tls.Config
	tlsFingerprint         string
	authLimiter            *authBackoffLimiter
	activeConns            atomic.Int64
	acceptedConns          atomic.Int64
	handshakeFailures      atomic.Int64
	authFailures           atomic.Int64
	inputEvents            atomic.Int64
	rdpeiContacts          atomic.Int64
	framesSent             atomic.Int64
	bitmapBytes            atomic.Int64
	bitmapRLEFrames        atomic.Int64
	bitmapRLEBytes         atomic.Int64
	bitmapRLESavedBytes    atomic.Int64
	nsCodecFrames          atomic.Int64
	nsCodecBytes           atomic.Int64
	nsCodecRawBytes        atomic.Int64
	nsCodecSavedBytes      atomic.Int64
	jpegCodecFrames        atomic.Int64
	jpegCodecBytes         atomic.Int64
	jpegCodecRawBytes      atomic.Int64
	jpegCodecSavedBytes    atomic.Int64
	pngCodecFrames         atomic.Int64
	pngCodecBytes          atomic.Int64
	pngCodecRawBytes       atomic.Int64
	pngCodecSavedBytes     atomic.Int64
	rfxCodecFrames         atomic.Int64
	rfxCodecBytes          atomic.Int64
	rfxCodecRawBytes       atomic.Int64
	rfxCodecSavedBytes     atomic.Int64
	bitmapCodecStreamStops atomic.Int64
	rdpgfxFrames           atomic.Int64
	rdpgfxBytes            atomic.Int64
	rdpgfxStreamStops      atomic.Int64
	rdpgfxPath             atomic.Value
	h264Frames             atomic.Int64
	h264Bytes              atomic.Int64
	dvcFragments           atomic.Int64
	h264Status             atomic.Value
	rfxEncoder             RFXEncoder
	clearCodecEncoder      RDPGFXFrameEncoder
	progressiveEncoder     RDPGFXFrameEncoder
	avc444Encoder          RDPGFXFrameEncoder
	avc444v2Encoder        RDPGFXFrameEncoder
	progressiveV2Encoder   RDPGFXFrameEncoder

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
	rfxEncoder := cfg.RFX
	if rfxEncoder == nil {
		rfxEncoder = productionRFXEncoder{}
	}
	clearEnc := cfg.ClearCodec
	if clearEnc == nil {
		clearEnc = clearCodecEncoder{}
	}
	progressiveEnc := cfg.Progressive
	if progressiveEnc == nil {
		progressiveEnc = productionProgressiveEncoder{}
	}
	progressiveV2Enc := cfg.ProgressiveV2
	if progressiveV2Enc == nil {
		progressiveV2Enc = productionProgressiveV2Encoder{}
	}
	return &Server{cfg: cfg, frames: frames, input: sink, tlsConfig: tlsConfig, tlsFingerprint: fingerprint, authLimiter: newAuthBackoffLimiter(cfg.Policy), rfxEncoder: rfxEncoder, clearCodecEncoder: clearEnc, progressiveEncoder: progressiveEnc, avc444Encoder: cfg.AVC444, avc444v2Encoder: cfg.AVC444v2, progressiveV2Encoder: progressiveV2Enc}, nil
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
	ctxWatcherDone := make(chan struct{})
	defer close(ctxWatcherDone)
	defer func() {
		_ = ln.Close()
		s.mu.Lock()
		if s.ln == ln {
			s.ln = nil
		}
		s.mu.Unlock()
	}()

	go func() {
		select {
		case <-ctx.Done():
			_ = ln.Close()
		case <-ctxWatcherDone:
		}
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

// ActiveConnections returns the number of currently accepted TCP sessions.
func (s *Server) ActiveConnections() int64 { return s.activeConns.Load() }

// AcceptedConnections returns the total number of accepted TCP sessions.
func (s *Server) AcceptedConnections() int64 { return s.acceptedConns.Load() }

// HandshakeFailures returns the total number of pre-auth handshake failures.
func (s *Server) HandshakeFailures() int64 { return s.handshakeFailures.Load() }

// AuthFailures returns the total number of authentication or auth-policy failures.
func (s *Server) AuthFailures() int64 { return s.authFailures.Load() }

// InputEvents returns the total number of decoded input callbacks sent to the sink.
func (s *Server) InputEvents() int64 { return s.inputEvents.Load() }

// RDPEIContacts returns the total number of decoded RDPEI touch contacts sent to the sink.
func (s *Server) RDPEIContacts() int64 { return s.rdpeiContacts.Load() }

// FramesSent returns the total number of frame update batches sent to clients.
func (s *Server) FramesSent() int64 { return s.framesSent.Load() }

// BitmapBytes returns the total number of bitmap update payload bytes sent to clients.
func (s *Server) BitmapBytes() int64 { return s.bitmapBytes.Load() }

// BitmapRLEFrames returns the total number of bitmap frame batches containing compressed RLE rectangles.
func (s *Server) BitmapRLEFrames() int64 { return s.bitmapRLEFrames.Load() }

// BitmapRLEBytes returns the total number of compressed bitmap RLE payload bytes sent to clients.
func (s *Server) BitmapRLEBytes() int64 { return s.bitmapRLEBytes.Load() }

// BitmapRLESavedBytes returns the estimated bytes saved versus uncompressed bitmap rectangles.
func (s *Server) BitmapRLESavedBytes() int64 { return s.bitmapRLESavedBytes.Load() }

// NSCodecFrames returns the total number of experimental NSCodec update batches sent to clients.
func (s *Server) NSCodecFrames() int64 { return s.nsCodecFrames.Load() }

// NSCodecBytes returns the total number of experimental NSCodec SurfaceBits command bytes sent to clients.
func (s *Server) NSCodecBytes() int64 { return s.nsCodecBytes.Load() }

// NSCodecRawBytes returns the estimated raw source bytes represented by experimental NSCodec SurfaceBits commands.
func (s *Server) NSCodecRawBytes() int64 { return s.nsCodecRawBytes.Load() }

// NSCodecSavedBytes returns the estimated bytes saved by experimental NSCodec SurfaceBits commands.
func (s *Server) NSCodecSavedBytes() int64 { return s.nsCodecSavedBytes.Load() }

// NSCodecSavedPercent returns the estimated percentage saved by experimental NSCodec SurfaceBits commands.
func (s *Server) NSCodecSavedPercent() float64 {
	return savedPercent(s.NSCodecRawBytes(), s.NSCodecSavedBytes())
}

// JPEGCodecFrames returns the total number of experimental JPEG bitmap-codec update batches sent to clients.
func (s *Server) JPEGCodecFrames() int64 { return s.jpegCodecFrames.Load() }

// JPEGCodecBytes returns the total number of experimental JPEG bitmap-codec SurfaceBits command bytes sent to clients.
func (s *Server) JPEGCodecBytes() int64 { return s.jpegCodecBytes.Load() }

// JPEGCodecRawBytes returns the estimated raw source bytes represented by experimental JPEG SurfaceBits commands.
func (s *Server) JPEGCodecRawBytes() int64 { return s.jpegCodecRawBytes.Load() }

// JPEGCodecSavedBytes returns the estimated bytes saved by experimental JPEG SurfaceBits commands.
func (s *Server) JPEGCodecSavedBytes() int64 { return s.jpegCodecSavedBytes.Load() }

// JPEGCodecSavedPercent returns the estimated percentage saved by experimental JPEG SurfaceBits commands.
func (s *Server) JPEGCodecSavedPercent() float64 {
	return savedPercent(s.JPEGCodecRawBytes(), s.JPEGCodecSavedBytes())
}

// PNGCodecFrames returns the total number of experimental PNG bitmap-codec update batches sent to clients.
func (s *Server) PNGCodecFrames() int64 { return s.pngCodecFrames.Load() }

// PNGCodecBytes returns the total number of experimental PNG bitmap-codec SurfaceBits command bytes sent to clients.
func (s *Server) PNGCodecBytes() int64 { return s.pngCodecBytes.Load() }

// PNGCodecRawBytes returns the estimated raw source bytes represented by experimental PNG SurfaceBits commands.
func (s *Server) PNGCodecRawBytes() int64 { return s.pngCodecRawBytes.Load() }

// PNGCodecSavedBytes returns the estimated bytes saved by experimental PNG SurfaceBits commands.
func (s *Server) PNGCodecSavedBytes() int64 { return s.pngCodecSavedBytes.Load() }

// PNGCodecSavedPercent returns the estimated percentage saved by experimental PNG SurfaceBits commands.
func (s *Server) PNGCodecSavedPercent() float64 {
	return savedPercent(s.PNGCodecRawBytes(), s.PNGCodecSavedBytes())
}

// RFXCodecFrames returns the total number of experimental RemoteFX update batches sent to clients.
func (s *Server) RFXCodecFrames() int64 { return s.rfxCodecFrames.Load() }

// RFXCodecBytes returns the total number of experimental RemoteFX SurfaceBits command bytes sent to clients.
func (s *Server) RFXCodecBytes() int64 { return s.rfxCodecBytes.Load() }

// RFXCodecRawBytes returns the estimated raw source bytes represented by experimental RemoteFX SurfaceBits commands.
func (s *Server) RFXCodecRawBytes() int64 { return s.rfxCodecRawBytes.Load() }

// RFXCodecSavedBytes returns the estimated bytes saved by experimental RemoteFX SurfaceBits commands.
func (s *Server) RFXCodecSavedBytes() int64 { return s.rfxCodecSavedBytes.Load() }

// RFXCodecSavedPercent returns the estimated percentage saved by experimental RemoteFX SurfaceBits commands.
func (s *Server) RFXCodecSavedPercent() float64 {
	return savedPercent(s.RFXCodecRawBytes(), s.RFXCodecSavedBytes())
}

// BitmapCodecStreamStops returns the total number of experimental SurfaceBits bitmap-codec stream write stops observed.
func (s *Server) BitmapCodecStreamStops() int64 { return s.bitmapCodecStreamStops.Load() }

// RDPGFXFrames returns the total number of RDPGFX frame update batches sent to clients.
func (s *Server) RDPGFXFrames() int64 { return s.rdpgfxFrames.Load() }

// RDPGFXBytes returns the total number of RDPGFX dynamic-channel payload bytes sent to clients.
func (s *Server) RDPGFXBytes() int64 { return s.rdpgfxBytes.Load() }

// RDPGFXStreamStops returns the total number of RDPGFX frame-stream write stops observed.
func (s *Server) RDPGFXStreamStops() int64 { return s.rdpgfxStreamStops.Load() }

// H264Frames returns the total number of H.264/AVC frame update batches sent to clients.
func (s *Server) H264Frames() int64 { return s.h264Frames.Load() }

// H264Bytes returns the total number of H.264/AVC payload bytes sent to clients.
func (s *Server) H264Bytes() int64 { return s.h264Bytes.Load() }

// H264Status returns the latest H.264 capability/emission status reason observed for diagnostics.
func (s *Server) H264Status() string {
	if status, ok := s.h264Status.Load().(string); ok && status != "" {
		return status
	}
	return "not-observed"
}

// GraphicsPath returns the active/last observed graphics transport path.
func (s *Server) GraphicsPath() string {
	if s.h264Frames.Load() > 0 {
		return h264GraphicsPathName
	}
	if s.rdpgfxFrames.Load() > 0 {
		if path, ok := s.rdpgfxPath.Load().(string); ok && path != "" {
			return path
		}
		return "rdpgfx-planar"
	}
	if s.nsCodecFrames.Load() > 0 {
		return "nscodec"
	}
	if s.jpegCodecFrames.Load() > 0 {
		return "jpeg-codec"
	}
	if s.pngCodecFrames.Load() > 0 {
		return "png-codec"
	}
	if s.rfxCodecFrames.Load() > 0 {
		return "rfx-codec"
	}
	if s.bitmapRLEFrames.Load() > 0 {
		return "bitmap-rle"
	}
	if s.bitmapBytes.Load() > 0 || s.framesSent.Load() > 0 {
		return "bitmap-fallback"
	}
	return "pending"
}

// DVCFragments returns the total number of dynamic virtual channel fragment PDUs observed.
func (s *Server) DVCFragments() int64 { return s.dvcFragments.Load() }

func (s *Server) handleConn(conn net.Conn) {
	s.activeConns.Add(1)
	s.acceptedConns.Add(1)
	defer s.activeConns.Add(-1)
	defer conn.Close()
	if !s.cfg.Policy.remoteAllowed(conn.RemoteAddr()) {
		s.authFailures.Add(1)
		log.Printf("rdp connection denied by CIDR policy from %s", conn.RemoteAddr())
		return
	}
	info, secureConn, err := performInitialHandshakeWithModeAndTLS(conn, s.cfg.Policy.SecurityMode, s.tlsConfig)
	if err != nil {
		s.handshakeFailures.Add(1)
		log.Printf("rdp initial handshake failed from %s: %v", conn.RemoteAddr(), err)
		return
	}
	conn = secureConn
	log.Printf("rdp initial handshake from %s: requested=0x%08x selected=0x%08x cookie=%q tls_fp=%s", conn.RemoteAddr(), info.RequestedProtocols, info.SelectedProtocol, sanitizeForLog(info.Cookie, 80), s.tlsFingerprint)
	if info.SelectedProtocol == protocolHybrid {
		remote := remoteHost(conn.RemoteAddr())
		if wait := s.authLimiter.lockoutRemaining(remote, ""); wait > 0 {
			s.authFailures.Add(1)
			log.Printf("rdp NLA/CredSSP locked out from %s: retry in %s", conn.RemoteAddr(), wait.Round(time.Second))
			return
		}
		clientInfo, err := performCredSSPWithBindings(conn, s.cfg.Authenticator, info.TLSPublicKeyCandidates)
		if err != nil {
			s.authFailures.Add(1)
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
			s.authFailures.Add(1)
			log.Printf("rdp NLA/CredSSP denied from %s: user=%q err=%v", conn.RemoteAddr(), sanitizeForLog(clientInfo.UserName, 64), err)
			return
		}
		log.Printf("rdp NLA/CredSSP authenticated from %s: user=%q domain=%q", conn.RemoteAddr(), sanitizeForLog(clientInfo.UserName, 64), sanitizeForLog(clientInfo.Domain, 64))
	}

	mcsInfo, err := readMCSConnectInitial(conn)
	if err != nil {
		s.handshakeFailures.Add(1)
		log.Printf("rdp MCS Connect-Initial failed from %s: %v", conn.RemoteAddr(), err)
		return
	}
	sessionWidth, sessionHeight := chooseSessionDesktopSize(s.cfg.Width, s.cfg.Height, mcsInfo.ClientDisplay.DesktopWidth, mcsInfo.ClientDisplay.DesktopHeight)
	log.Printf(
		"rdp MCS Connect-Initial from %s: appTag=%d payload=%d userData=%d channels=%d channelNames=%q clientDesktop=%dx%d monitorLayout=%t monitorCount=%d sessionDesktop=%dx%d",
		conn.RemoteAddr(),
		mcsInfo.ApplicationTag,
		mcsInfo.PayloadLength,
		mcsInfo.UserDataLength,
		len(mcsInfo.ClientChannels),
		clientChannelNames(mcsInfo.ClientChannels),
		mcsInfo.ClientDisplay.DesktopWidth,
		mcsInfo.ClientDisplay.DesktopHeight,
		mcsInfo.ClientDisplay.MonitorLayoutPresent,
		mcsInfo.ClientDisplay.MonitorCount,
		sessionWidth,
		sessionHeight,
	)
	if err := writeMCSConnectResponse(conn, info.SelectedProtocol, mcsInfo.ClientChannels); err != nil {
		s.handshakeFailures.Add(1)
		log.Printf("rdp MCS Connect-Response failed to %s: %v", conn.RemoteAddr(), err)
		return
	}
	log.Printf("rdp MCS Connect-Response sent to %s", conn.RemoteAddr())
	countingSink := &countingInputSink{sink: s.input, inputEvents: &s.inputEvents, rdpeiContacts: &s.rdpeiContacts}
	metrics := s.metrics()
	if err := handleMCSDomainSequence(conn, s.frames, s.cfg.H264, countingSink, sessionWidth, sessionHeight, s.cfg.Authenticator, s.cfg.Policy, s.authLimiter, info.SelectedProtocol, mcsInfo.ClientChannels, metrics); err != nil {
		if errors.Is(err, errAuthFailure) {
			s.authFailures.Add(1)
		} else {
			s.handshakeFailures.Add(1)
		}
		log.Printf("rdp MCS domain sequence failed from %s: %v", conn.RemoteAddr(), err)
		return
	}
	log.Printf("rdp MCS domain sequence finished for %s", conn.RemoteAddr())
	// The next phase is Security Exchange / Client Info handling.
}

func clientChannelNames(channels []clientChannel) []string {
	names := make([]string, 0, len(channels))
	for _, ch := range channels {
		names = append(names, ch.Name)
	}
	return names
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
