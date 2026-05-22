package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

func TestBuildNSCodecSurfaceBitsCommandBGRA(t *testing.T) {
	fr := frameOfSolidBGRAForRLETest(4, 3)
	cmd, ok := buildNSCodecSurfaceBitsCommand(fr, 7)
	if !ok {
		t.Fatal("buildNSCodecSurfaceBitsCommand ok = false")
	}
	cmdType, codecID, width, height, bitmapLen, ok := parseSurfaceBitsCommandHeaderForTest(cmd)
	if !ok {
		t.Fatalf("parse surface bits header failed: %x", cmd)
	}
	if cmdType != surfaceCmdSetSurfaceBits || codecID != 7 || width != 4 || height != 3 {
		t.Fatalf("unexpected header: cmd=0x%04x codec=%d size=%dx%d", cmdType, codecID, width, height)
	}
	if int(bitmapLen) != len(cmd)-22 {
		t.Fatalf("bitmap len = %d, payload = %d", bitmapLen, len(cmd)-22)
	}
	decoded, err := rdpcodec.DecodeNSCodec(cmd[22:], int(width), int(height))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != int(width)*int(height)*4 {
		t.Fatalf("decoded len = %d", len(decoded))
	}
}

func TestBuildNSCodecSurfaceBitsCommandRGBA(t *testing.T) {
	data := []byte{
		0x20, 0x40, 0x60, 0xff,
		0x10, 0x30, 0x50, 0xff,
	}
	fr := frame.Frame{Width: 2, Height: 1, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: data}
	cmd, ok := buildNSCodecSurfaceBitsCommand(fr, 0)
	if !ok {
		t.Fatal("buildNSCodecSurfaceBitsCommand ok = false")
	}
	_, codecID, width, height, _, ok := parseSurfaceBitsCommandHeaderForTest(cmd)
	if !ok || codecID != bitmapCodecNSCodecDefaultID || width != 2 || height != 1 {
		t.Fatalf("unexpected header ok=%t codec=%d size=%dx%d", ok, codecID, width, height)
	}
}

func TestBuildNSCodecSurfaceBitsCommandRejectsInvalid(t *testing.T) {
	if _, ok := buildNSCodecSurfaceBitsCommand(frame.Frame{}, 1); ok {
		t.Fatal("expected empty frame to be rejected")
	}
	if _, ok := buildNSCodecSurfaceBitsCommand(frame.Frame{Width: 1, Height: 1, Stride: 3, Format: frame.PixelFormatBGRA8888, Data: []byte{1, 2, 3}}, 1); ok {
		t.Fatal("expected short stride to be rejected")
	}
}
