package rdpserver

import (
	"net"
	"sync/atomic"
	"testing"
	"time"
)

type captureConn struct {
	writes atomic.Int64
}

func (c *captureConn) Read(_ []byte) (int, error)         { return 0, net.ErrClosed }
func (c *captureConn) Write(p []byte) (int, error)        { c.writes.Add(1); return len(p), nil }
func (c *captureConn) Close() error                       { return nil }
func (c *captureConn) LocalAddr() net.Addr                { return nil }
func (c *captureConn) RemoteAddr() net.Addr               { return nil }
func (c *captureConn) SetDeadline(_ time.Time) error      { return nil }
func (c *captureConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *captureConn) SetWriteDeadline(_ time.Time) error { return nil }

func TestWriteExperimentalBitmapCodecUpdateRecordsMetrics(t *testing.T) {
	conn := &captureConn{}
	var nsFrames atomic.Int64
	var nsBytes atomic.Int64
	metrics := serverMetrics{nsCodecFrames: &nsFrames, nsCodecBytes: &nsBytes}
	cmd := bitmapCodecCommand{Name: "nscodec", CodecID: 2, Command: []byte{1, 2, 3}, Trace: "nscodec"}
	if err := writeExperimentalBitmapCodecUpdate(conn, metrics, cmd); err != nil {
		t.Fatalf("writeExperimentalBitmapCodecUpdate: %v", err)
	}
	if conn.writes.Load() <= 0 {
		t.Fatalf("writes = %d, want positive", conn.writes.Load())
	}
	if nsFrames.Load() != 1 || nsBytes.Load() != 3 {
		t.Fatalf("metrics = %d/%d, want 1/3", nsFrames.Load(), nsBytes.Load())
	}
}
