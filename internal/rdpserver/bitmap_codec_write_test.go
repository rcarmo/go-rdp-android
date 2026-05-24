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
	for _, tc := range []struct {
		name      string
		cmd       bitmapCodecCommand
		metrics   func(frames, bytes, rawBytes, savedBytes *atomic.Int64) serverMetrics
		wantBytes int64
		wantRaw   int64
		wantSaved int64
	}{
		{
			name: "nscodec",
			cmd:  bitmapCodecCommand{Name: "nscodec", CodecID: 2, Command: []byte{1, 2, 3}, Trace: "nscodec", RawBytes: 10},
			metrics: func(frames, bytes, rawBytes, savedBytes *atomic.Int64) serverMetrics {
				return serverMetrics{nsCodecFrames: frames, nsCodecBytes: bytes, nsCodecRawBytes: rawBytes, nsCodecSavedBytes: savedBytes}
			},
			wantBytes: 3,
			wantRaw:   10,
			wantSaved: 7,
		},
		{
			name: "jpeg",
			cmd:  bitmapCodecCommand{Name: "jpeg-codec", CodecID: 3, Command: []byte{1, 2, 3, 4}, Trace: "jpeg_codec", RawBytes: 16, Quality: 80},
			metrics: func(frames, bytes, rawBytes, savedBytes *atomic.Int64) serverMetrics {
				return serverMetrics{jpegCodecFrames: frames, jpegCodecBytes: bytes, jpegCodecRawBytes: rawBytes, jpegCodecSavedBytes: savedBytes}
			},
			wantBytes: 4,
			wantRaw:   16,
			wantSaved: 12,
		},
		{
			name: "png",
			cmd:  bitmapCodecCommand{Name: "png-codec", CodecID: 9, Command: []byte{1, 2, 3, 4, 5}, Trace: "png_codec", RawBytes: 20},
			metrics: func(frames, bytes, rawBytes, savedBytes *atomic.Int64) serverMetrics {
				return serverMetrics{pngCodecFrames: frames, pngCodecBytes: bytes, pngCodecRawBytes: rawBytes, pngCodecSavedBytes: savedBytes}
			},
			wantBytes: 5,
			wantRaw:   20,
			wantSaved: 15,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := &captureConn{}
			var frames atomic.Int64
			var bytes atomic.Int64
			var rawBytes atomic.Int64
			var savedBytes atomic.Int64
			if err := writeExperimentalBitmapCodecUpdate(conn, tc.metrics(&frames, &bytes, &rawBytes, &savedBytes), tc.cmd); err != nil {
				t.Fatalf("writeExperimentalBitmapCodecUpdate: %v", err)
			}
			if conn.writes.Load() <= 0 {
				t.Fatalf("writes = %d, want positive", conn.writes.Load())
			}
			if frames.Load() != 1 || bytes.Load() != tc.wantBytes || rawBytes.Load() != tc.wantRaw || savedBytes.Load() != tc.wantSaved {
				t.Fatalf("metrics = %d/%d/%d/%d, want 1/%d/%d/%d", frames.Load(), bytes.Load(), rawBytes.Load(), savedBytes.Load(), tc.wantBytes, tc.wantRaw, tc.wantSaved)
			}
		})
	}
}
