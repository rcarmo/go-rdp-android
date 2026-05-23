package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

func tinyCodecFrame() frame.Frame {
	return frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{0x33, 0x66, 0x99, 0xff}}
}

func TestBuildExperimentalBitmapCodecCommandPrefersNSCodec(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_NSCODEC", "1")
	t.Setenv("GO_RDP_ANDROID_ENABLE_JPEG_CODEC", "1")
	t.Setenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID", "9")
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{
		{GUID: rdpcodec.JPEGCodecGUID, ID: 3, Name: rdpcodec.BitmapCodecNameJPEG},
		{GUID: rdpcodec.NSCodecGUID, ID: 2, Name: rdpcodec.BitmapCodecNameNSCodec},
	}}}
	cmd, ok := buildExperimentalBitmapCodecCommand(tinyCodecFrame(), caps)
	if !ok || cmd.Name != "nscodec" || cmd.CodecID != 2 || len(cmd.Command) == 0 {
		t.Fatalf("command = %#v ok=%t", cmd, ok)
	}
}

func TestBuildExperimentalBitmapCodecCommandFallsBackToJPEG(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_JPEG_CODEC", "1")
	t.Setenv("GO_RDP_ANDROID_JPEG_QUALITY", "55")
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{{GUID: rdpcodec.JPEGCodecGUID, ID: 3, Name: rdpcodec.BitmapCodecNameJPEG}}}}
	cmd, ok := buildExperimentalBitmapCodecCommand(tinyCodecFrame(), caps)
	if !ok || cmd.Name != "jpeg-codec" || cmd.CodecID != 3 || cmd.Quality != 55 || len(cmd.Command) == 0 {
		t.Fatalf("command = %#v ok=%t", cmd, ok)
	}
}

func TestBuildExperimentalBitmapCodecCommandUsesPNGOverride(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID", "9")
	cmd, ok := buildExperimentalBitmapCodecCommand(tinyCodecFrame(), confirmActiveCapabilities{})
	if !ok || cmd.Name != "png-codec" || cmd.CodecID != 9 || len(cmd.Command) == 0 {
		t.Fatalf("command = %#v ok=%t", cmd, ok)
	}
}

func TestBuildExperimentalBitmapCodecCommandNoSelection(t *testing.T) {
	cmd, ok := buildExperimentalBitmapCodecCommand(tinyCodecFrame(), confirmActiveCapabilities{})
	if ok || cmd.Command != nil || cmd.Name != "" {
		t.Fatalf("command = %#v ok=%t", cmd, ok)
	}
}
