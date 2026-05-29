package rdpserver

import (
	"sync/atomic"
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

func TestRecordExperimentalBitmapCodecFrame(t *testing.T) {
	var nsFrames atomic.Int64
	var nsBytes atomic.Int64
	var nsRaw atomic.Int64
	var nsSaved atomic.Int64
	var jpegFrames atomic.Int64
	var jpegBytes atomic.Int64
	var jpegRaw atomic.Int64
	var jpegSaved atomic.Int64
	var pngFrames atomic.Int64
	var pngBytes atomic.Int64
	var pngRaw atomic.Int64
	var pngSaved atomic.Int64
	var rfxFrames atomic.Int64
	var rfxBytes atomic.Int64
	var rfxRaw atomic.Int64
	var rfxSaved atomic.Int64
	metrics := serverMetrics{nsCodecFrames: &nsFrames, nsCodecBytes: &nsBytes, nsCodecRawBytes: &nsRaw, nsCodecSavedBytes: &nsSaved, jpegCodecFrames: &jpegFrames, jpegCodecBytes: &jpegBytes, jpegCodecRawBytes: &jpegRaw, jpegCodecSavedBytes: &jpegSaved, pngCodecFrames: &pngFrames, pngCodecBytes: &pngBytes, pngCodecRawBytes: &pngRaw, pngCodecSavedBytes: &pngSaved, rfxCodecFrames: &rfxFrames, rfxCodecBytes: &rfxBytes, rfxCodecRawBytes: &rfxRaw, rfxCodecSavedBytes: &rfxSaved}

	if !recordExperimentalBitmapCodecFrame(metrics, bitmapCodecCommand{Name: "nscodec", Command: []byte{1, 2}, RawBytes: 6}) {
		t.Fatal("record nscodec = false")
	}
	if !recordExperimentalBitmapCodecFrame(metrics, bitmapCodecCommand{Name: "jpeg-codec", Command: []byte{1, 2, 3}, RawBytes: 8}) {
		t.Fatal("record jpeg = false")
	}
	if !recordExperimentalBitmapCodecFrame(metrics, bitmapCodecCommand{Name: "png-codec", Command: []byte{1, 2, 3, 4}, RawBytes: 10}) {
		t.Fatal("record png = false")
	}
	if !recordExperimentalBitmapCodecFrame(metrics, bitmapCodecCommand{Name: "rfx-codec", Command: []byte{1, 2, 3, 4, 5}, RawBytes: 12}) {
		t.Fatal("record rfx = false")
	}
	if recordExperimentalBitmapCodecFrame(metrics, bitmapCodecCommand{Name: "unknown", Command: []byte{1}}) {
		t.Fatal("record unknown = true")
	}
	if nsFrames.Load() != 1 || nsBytes.Load() != 2 || nsRaw.Load() != 6 || nsSaved.Load() != 4 || jpegFrames.Load() != 1 || jpegBytes.Load() != 3 || jpegRaw.Load() != 8 || jpegSaved.Load() != 5 || pngFrames.Load() != 1 || pngBytes.Load() != 4 || pngRaw.Load() != 10 || pngSaved.Load() != 6 || rfxFrames.Load() != 1 || rfxBytes.Load() != 5 || rfxRaw.Load() != 12 || rfxSaved.Load() != 7 {
		t.Fatalf("unexpected metrics ns=%d/%d/%d/%d jpeg=%d/%d/%d/%d png=%d/%d/%d/%d rfx=%d/%d/%d/%d", nsFrames.Load(), nsBytes.Load(), nsRaw.Load(), nsSaved.Load(), jpegFrames.Load(), jpegBytes.Load(), jpegRaw.Load(), jpegSaved.Load(), pngFrames.Load(), pngBytes.Load(), pngRaw.Load(), pngSaved.Load(), rfxFrames.Load(), rfxBytes.Load(), rfxRaw.Load(), rfxSaved.Load())
	}
}
