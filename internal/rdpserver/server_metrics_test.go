package rdpserver

import "testing"

func TestServerMetricsIncludesBitmapCodecSavedByteCounters(t *testing.T) {
	srv, err := New(Config{Addr: "127.0.0.1:0", Width: 2, Height: 2}, nil, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	metrics := srv.metrics()

	metrics.recordNSCodecFrameSavings([][]byte{[]byte("ns")}, 100, 25)
	metrics.recordJPEGCodecFrameSavings([][]byte{[]byte("jpg")}, 200, 50)
	metrics.recordPNGCodecFrameSavings([][]byte{[]byte("png")}, 400, 100)
	metrics.recordRFXCodecFrame([][]byte{[]byte("rfx")}, 800, 200)

	if got := srv.NSCodecRawBytes(); got != 100 {
		t.Fatalf("NSCodecRawBytes = %d, want 100", got)
	}
	if got := srv.NSCodecSavedBytes(); got != 25 {
		t.Fatalf("NSCodecSavedBytes = %d, want 25", got)
	}
	if got := srv.NSCodecSavedPercent(); got != 25 {
		t.Fatalf("NSCodecSavedPercent = %f, want 25", got)
	}
	if got := srv.JPEGCodecRawBytes(); got != 200 {
		t.Fatalf("JPEGCodecRawBytes = %d, want 200", got)
	}
	if got := srv.JPEGCodecSavedBytes(); got != 50 {
		t.Fatalf("JPEGCodecSavedBytes = %d, want 50", got)
	}
	if got := srv.JPEGCodecSavedPercent(); got != 25 {
		t.Fatalf("JPEGCodecSavedPercent = %f, want 25", got)
	}
	if got := srv.PNGCodecRawBytes(); got != 400 {
		t.Fatalf("PNGCodecRawBytes = %d, want 400", got)
	}
	if got := srv.PNGCodecSavedBytes(); got != 100 {
		t.Fatalf("PNGCodecSavedBytes = %d, want 100", got)
	}
	if got := srv.PNGCodecSavedPercent(); got != 25 {
		t.Fatalf("PNGCodecSavedPercent = %f, want 25", got)
	}
	if got := srv.RFXCodecRawBytes(); got != 800 {
		t.Fatalf("RFXCodecRawBytes = %d, want 800", got)
	}
	if got := srv.RFXCodecSavedBytes(); got != 200 {
		t.Fatalf("RFXCodecSavedBytes = %d, want 200", got)
	}
	if got := srv.RFXCodecSavedPercent(); got != 25 {
		t.Fatalf("RFXCodecSavedPercent = %f, want 25", got)
	}
}
