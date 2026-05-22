package rdpserver

import (
	"bytes"
	"image/jpeg"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBuildJPEGSurfaceBitsCommandBGRA(t *testing.T) {
	fr := frame.Frame{Width: 2, Height: 2, Stride: 8, Format: frame.PixelFormatBGRA8888, Data: []byte{
		0x00, 0x00, 0xff, 0xff, 0x00, 0xff, 0x00, 0xff,
		0xff, 0x00, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff,
	}}
	cmd, ok := buildJPEGSurfaceBitsCommand(fr, 7, 90)
	if !ok {
		t.Fatal("buildJPEGSurfaceBitsCommand() ok = false")
	}
	cmdType, codecID, width, height, bitmapLen, ok := parseSurfaceBitsCommandHeaderForTest(cmd)
	if !ok {
		t.Fatal("parseSurfaceBitsCommandHeaderForTest() ok = false")
	}
	if cmdType != surfaceCmdSetSurfaceBits || codecID != 7 || width != 2 || height != 2 {
		t.Fatalf("unexpected header: cmd=0x%04x codec=%d size=%dx%d", cmdType, codecID, width, height)
	}
	img, err := jpeg.Decode(bytes.NewReader(cmd[22 : 22+bitmapLen]))
	if err != nil {
		t.Fatalf("jpeg.Decode: %v", err)
	}
	if got := img.Bounds().Dx(); got != 2 {
		t.Fatalf("decoded width = %d", got)
	}
	if got := img.Bounds().Dy(); got != 2 {
		t.Fatalf("decoded height = %d", got)
	}
}

func TestBuildJPEGSurfaceBitsCommandDefaultsAndRejectsInvalid(t *testing.T) {
	fr := frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{0xff, 0x00, 0x00, 0xff}}
	cmd, ok := buildJPEGSurfaceBitsCommand(fr, 0, 0)
	if !ok {
		t.Fatal("buildJPEGSurfaceBitsCommand() ok = false")
	}
	_, codecID, width, height, _, ok := parseSurfaceBitsCommandHeaderForTest(cmd)
	if !ok || codecID != bitmapCodecJPEGDefaultID || width != 1 || height != 1 {
		t.Fatalf("unexpected default header: codec=%d size=%dx%d ok=%t", codecID, width, height, ok)
	}
	if cmd, ok := buildJPEGSurfaceBitsCommand(frame.Frame{Width: 1, Height: 1, Stride: 3, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3}}, 1, 75); ok || cmd != nil {
		t.Fatalf("invalid frame built command len=%d ok=%t", len(cmd), ok)
	}
}
