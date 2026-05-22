package rdpserver

import (
	"testing"

	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

func TestNegotiatedJPEGCodecIDRequiresEnvAndCapability(t *testing.T) {
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{{GUID: rdpcodec.JPEGCodecGUID, ID: 6, Name: rdpcodec.BitmapCodecNameJPEG}}}}
	if id, ok := negotiatedJPEGCodecID(caps); ok || id != 0 {
		t.Fatalf("negotiatedJPEGCodecID without env = %d,%t", id, ok)
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_JPEG_CODEC", "1")
	if id, ok := negotiatedJPEGCodecID(confirmActiveCapabilities{}); ok || id != 0 {
		t.Fatalf("negotiatedJPEGCodecID without caps = %d,%t", id, ok)
	}
	if id, ok := negotiatedJPEGCodecID(caps); !ok || id != 6 {
		t.Fatalf("negotiatedJPEGCodecID with caps = %d,%t", id, ok)
	}
}
