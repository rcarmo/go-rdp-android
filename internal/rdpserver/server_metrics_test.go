package rdpserver

import "testing"

func TestServerMetricsIncludesBitmapCodecSavedByteCounters(t *testing.T) {
	srv, err := New(Config{Addr: "127.0.0.1:0", Width: 2, Height: 2}, nil, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	metrics := srv.metrics()

	metrics.recordNSCodecFrameSavings([][]byte{[]byte("ns")}, 101, 11)
	metrics.recordJPEGCodecFrameSavings([][]byte{[]byte("jpg")}, 103, 13)
	metrics.recordPNGCodecFrameSavings([][]byte{[]byte("png")}, 107, 17)

	if got := srv.NSCodecRawBytes(); got != 101 {
		t.Fatalf("NSCodecRawBytes = %d, want 101", got)
	}
	if got := srv.NSCodecSavedBytes(); got != 11 {
		t.Fatalf("NSCodecSavedBytes = %d, want 11", got)
	}
	if got := srv.JPEGCodecRawBytes(); got != 103 {
		t.Fatalf("JPEGCodecRawBytes = %d, want 103", got)
	}
	if got := srv.JPEGCodecSavedBytes(); got != 13 {
		t.Fatalf("JPEGCodecSavedBytes = %d, want 13", got)
	}
	if got := srv.PNGCodecRawBytes(); got != 107 {
		t.Fatalf("PNGCodecRawBytes = %d, want 107", got)
	}
	if got := srv.PNGCodecSavedBytes(); got != 17 {
		t.Fatalf("PNGCodecSavedBytes = %d, want 17", got)
	}
}
