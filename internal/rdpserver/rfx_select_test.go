package rdpserver

import (
	"testing"

	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

func TestNegotiatedRemoteFXCodecIDRequiresEnvAndCapability(t *testing.T) {
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{{GUID: rdpcodec.RemoteFXGUID, ID: 4, Name: rdpcodec.BitmapCodecNameRemoteFX}}}}
	if id, ok := negotiatedRemoteFXCodecID(caps); ok || id != 0 {
		t.Fatalf("negotiatedRemoteFXCodecID without env = %d,%t", id, ok)
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_RFX_CODEC", "1")
	if id, ok := negotiatedRemoteFXCodecID(confirmActiveCapabilities{}); ok || id != 0 {
		t.Fatalf("negotiatedRemoteFXCodecID without caps = %d,%t", id, ok)
	}
	if id, ok := negotiatedRemoteFXCodecID(caps); !ok || id != 4 {
		t.Fatalf("negotiatedRemoteFXCodecID with caps = %d,%t", id, ok)
	}
}

func TestNegotiatedRemoteFXCodecIDAcceptsImageCodec(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_RFX_CODEC", "1")
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{{GUID: rdpcodec.RemoteFXImageGUID, ID: 5, Name: rdpcodec.BitmapCodecNameRemoteFXImage}}}}
	if id, ok := negotiatedRemoteFXCodecID(caps); !ok || id != 5 {
		t.Fatalf("negotiatedRemoteFXCodecID image = %d,%t", id, ok)
	}
}
