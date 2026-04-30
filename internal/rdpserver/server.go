// Package rdpserver contains the server-side RDP skeleton.
package rdpserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	"github.com/rcarmo/go-rdp-android/internal/input"
)

// Config controls the RDP server.
type Config struct {
	Addr   string
	Width  int
	Height int
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
	s.mu.Lock()
	s.ln = ln
	s.mu.Unlock()
	defer ln.Close()

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
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	_, _ = fmt.Fprintf(conn, "go-rdp-android: RDP server handshake not implemented yet\n")
}

// Close stops the listener.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}
