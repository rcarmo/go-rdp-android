package frame

import (
	"testing"
	"time"
)

func TestTestPatternSource(t *testing.T) {
	s := NewTestPatternSource(8, 4, 30)
	defer s.Close()
	select {
	case f := <-s.Frames():
		if f.Width != 8 || f.Height != 4 || f.Stride != 32 || f.Format != PixelFormatRGBA8888 || len(f.Data) != 8*4*4 {
			t.Fatalf("unexpected frame: %#v len=%d", f, len(f.Data))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for frame")
	}
}
