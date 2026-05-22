package rdpserver

import (
	"testing"

	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

func TestNegotiatedNSCodecIDRequiresEnvAndCapability(t *testing.T) {
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{{GUID: rdpcodec.NSCodecGUID, ID: 9, Name: rdpcodec.BitmapCodecNameNSCodec}}}}
	if id, ok := negotiatedNSCodecID(caps); ok || id != 0 {
		t.Fatalf("negotiatedNSCodecID without env = %d,%t", id, ok)
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_NSCODEC", "1")
	if id, ok := negotiatedNSCodecID(confirmActiveCapabilities{}); ok || id != 0 {
		t.Fatalf("negotiatedNSCodecID without caps = %d,%t", id, ok)
	}
	if id, ok := negotiatedNSCodecID(caps); !ok || id != 9 {
		t.Fatalf("negotiatedNSCodecID with caps = %d,%t", id, ok)
	}
}

func TestNegotiatedNSCodecIDIgnoresZeroCodecID(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_NSCODEC", "1")
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{{GUID: rdpcodec.NSCodecGUID, ID: 0, Name: rdpcodec.BitmapCodecNameNSCodec}}}}
	if id, ok := negotiatedNSCodecID(caps); ok || id != 0 {
		t.Fatalf("negotiatedNSCodecID zero id = %d,%t", id, ok)
	}
}
