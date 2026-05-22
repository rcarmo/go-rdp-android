package rdpserver

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBuildPNGSurfaceBitsCommandRGBA(t *testing.T) {
	fr := frame.Frame{Width: 2, Height: 1, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: []byte{
		0xff, 0x00, 0x00, 0xff,
		0x00, 0xff, 0x00, 0x80,
	}}
	cmd, ok := buildPNGSurfaceBitsCommand(fr, 9)
	if !ok {
		t.Fatal("buildPNGSurfaceBitsCommand() ok = false")
	}
	cmdType, codecID, width, height, bitmapLen, ok := parseSurfaceBitsCommandHeaderForTest(cmd)
	if !ok {
		t.Fatal("parseSurfaceBitsCommandHeaderForTest() ok = false")
	}
	if cmdType != surfaceCmdSetSurfaceBits || codecID != 9 || width != 2 || height != 1 {
		t.Fatalf("unexpected header: cmd=0x%04x codec=%d size=%dx%d", cmdType, codecID, width, height)
	}
	img, err := png.Decode(bytes.NewReader(cmd[22 : 22+bitmapLen]))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	if got := img.Bounds().Dx(); got != 2 {
		t.Fatalf("decoded width = %d", got)
	}
	if got := img.Bounds().Dy(); got != 1 {
		t.Fatalf("decoded height = %d", got)
	}
}

func TestBuildPNGSurfaceBitsCommandRejectsInvalid(t *testing.T) {
	fr := frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatBGRA8888, Data: []byte{0, 0, 0, 0xff}}
	if cmd, ok := buildPNGSurfaceBitsCommand(fr, 0); ok || cmd != nil {
		t.Fatalf("zero codec ID built command len=%d ok=%t", len(cmd), ok)
	}
	if cmd, ok := buildPNGSurfaceBitsCommand(frame.Frame{Width: 1, Height: 1, Stride: 3, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3}}, 1); ok || cmd != nil {
		t.Fatalf("invalid frame built command len=%d ok=%t", len(cmd), ok)
	}
}
