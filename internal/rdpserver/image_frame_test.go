package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestFrameToRGBAImageConvertsBGRA(t *testing.T) {
	fr := frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatBGRA8888, Data: []byte{0x11, 0x22, 0x33, 0x44}}
	img, ok := frameToRGBAImage(fr)
	if !ok {
		t.Fatal("frameToRGBAImage() ok = false")
	}
	got := img.RGBAAt(0, 0)
	if got.R != 0x33 || got.G != 0x22 || got.B != 0x11 || got.A != 0x44 {
		t.Fatalf("RGBA = %#v", got)
	}
}

func TestFrameToRGBAImageConvertsRGBA(t *testing.T) {
	fr := frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{0x33, 0x22, 0x11, 0x44}}
	img, ok := frameToRGBAImage(fr)
	if !ok {
		t.Fatal("frameToRGBAImage() ok = false")
	}
	got := img.RGBAAt(0, 0)
	if got.R != 0x33 || got.G != 0x22 || got.B != 0x11 || got.A != 0x44 {
		t.Fatalf("RGBA = %#v", got)
	}
}

func TestFrameToRGBAImageRejectsInvalid(t *testing.T) {
	if img, ok := frameToRGBAImage(frame.Frame{Width: 1, Height: 1, Stride: 3, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3}}); ok || img != nil {
		t.Fatalf("invalid frame converted: %#v ok=%t", img, ok)
	}
}
