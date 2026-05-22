package rdpserver

import (
	"encoding/binary"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBuildRDPGFXUncompressedFramePDUs(t *testing.T) {
	fr := frame.Frame{Width: 2, Height: 1, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: []byte{
		0xff, 0x00, 0x00, 0xff,
		0x00, 0xff, 0x00, 0xff,
	}}
	pdus, ok := buildRDPGFXUncompressedFramePDUs(3, 7, fr, 2, 1)
	if !ok || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXUncompressedFramePDUs len=%d ok=%t", len(pdus), ok)
	}
	if cmd := binary.LittleEndian.Uint16(pdus[1][0:2]); cmd != rdpgfxCmdWireToSurface1 {
		t.Fatalf("wire cmd = 0x%04x", cmd)
	}
	payload := pdus[1][8:]
	if surfaceID := binary.LittleEndian.Uint16(payload[0:2]); surfaceID != 3 {
		t.Fatalf("surfaceID = %d", surfaceID)
	}
	if codec := binary.LittleEndian.Uint16(payload[2:4]); codec != rdpgfxCodecUncompressed {
		t.Fatalf("codec = 0x%04x", codec)
	}
	if pixelFormat := payload[4]; pixelFormat != rdpgfxPixelFormatXRGB8888 {
		t.Fatalf("pixel format = 0x%02x", pixelFormat)
	}
	bitmapLen := binary.LittleEndian.Uint32(payload[13:17])
	if bitmapLen != 8 {
		t.Fatalf("bitmapLen = %d", bitmapLen)
	}
	bitmap := payload[17:]
	want := []byte{0x00, 0x00, 0xff, 0xff, 0x00, 0xff, 0x00, 0xff}
	for i := range want {
		if bitmap[i] != want[i] {
			t.Fatalf("bitmap[%d]=0x%02x want 0x%02x", i, bitmap[i], want[i])
		}
	}
}

func TestRDPGFXUncompressedEnvGate(t *testing.T) {
	if rdpgfxUncompressedEnabledFromEnv() {
		t.Fatal("uncompressed RDPGFX should be disabled by default")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_RDPGFX_UNCOMPRESSED", "1")
	if !rdpgfxUncompressedEnabledFromEnv() {
		t.Fatal("uncompressed RDPGFX should be enabled by env")
	}
}
