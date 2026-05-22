package rdpserver

import (
	"encoding/binary"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBuildRDPGFXFrameUpdatePDUsSelectsPlanarByDefault(t *testing.T) {
	fr := frame.Frame{Width: 2, Height: 1, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: []byte{
		0xff, 0x00, 0x00, 0xff,
		0x00, 0xff, 0x00, 0xff,
	}}
	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(4, 9, fr, 2, 1)
	if !ok || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXFrameUpdatePDUs len=%d path=%q ok=%t", len(pdus), path, ok)
	}
	if path != "rdpgfx-planar" {
		t.Fatalf("path = %q", path)
	}
	payload := pdus[1][8:]
	if surfaceID := binary.LittleEndian.Uint16(payload[0:2]); surfaceID != 4 {
		t.Fatalf("surfaceID = %d", surfaceID)
	}
	if codec := binary.LittleEndian.Uint16(payload[2:4]); codec != rdpgfxCodecPlanar {
		t.Fatalf("codec = 0x%04x", codec)
	}
}

func TestBuildRDPGFXFrameUpdatePDUsSelectsUncompressedWhenEnabled(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_RDPGFX_UNCOMPRESSED", "1")
	fr := frame.Frame{Width: 2, Height: 1, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: []byte{
		0xff, 0x00, 0x00, 0xff,
		0x00, 0xff, 0x00, 0xff,
	}}
	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(4, 9, fr, 2, 1)
	if !ok || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXFrameUpdatePDUs len=%d path=%q ok=%t", len(pdus), path, ok)
	}
	if path != "rdpgfx-uncompressed" {
		t.Fatalf("path = %q", path)
	}
	payload := pdus[1][8:]
	if codec := binary.LittleEndian.Uint16(payload[2:4]); codec != rdpgfxCodecUncompressed {
		t.Fatalf("codec = 0x%04x", codec)
	}
}

func TestBuildRDPGFXFrameUpdatePDUsRejectsInvalidFrame(t *testing.T) {
	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(0, 1, frame.Frame{Width: 1, Height: 1, Stride: 3, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3}}, 1, 1)
	if ok || pdus != nil || path != "" {
		t.Fatalf("invalid frame built pdus=%d path=%q ok=%t", len(pdus), path, ok)
	}
}
