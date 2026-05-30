package rdpserver

import (
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func benchmarkCodecFrame(width, height int) frame.Frame {
	stride := width * 4
	data := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*stride + x*4
			data[i+0] = byte((x * 3) & 0xff)
			data[i+1] = byte((y * 5) & 0xff)
			data[i+2] = byte((x + y) & 0xff)
			data[i+3] = 0xff
		}
	}
	return frame.Frame{Width: width, Height: height, Stride: stride, Format: frame.PixelFormatRGBA8888, Timestamp: time.Unix(0, 0), Data: data}
}

func BenchmarkBuildRDPGFXPlanarFramePDUs_320x240(b *testing.B) {
	fr := benchmarkCodecFrame(320, 240)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pdus, ok := buildRDPGFXPlanarFramePDUs(0, uint32(i+1), fr, fr.Width, fr.Height)
		if !ok || len(pdus) != 3 {
			b.Fatalf("buildRDPGFXPlanarFramePDUs len=%d ok=%t", len(pdus), ok)
		}
		b.SetBytes(totalPayloadBytes(pdus))
	}
}

func BenchmarkBuildRFXProductionEncoder_320x240(b *testing.B) {
	fr := benchmarkCodecFrame(320, 240)
	enc := productionRFXEncoder{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		payload, ok := enc.EncodeRFX(fr, fr.Width, fr.Height)
		if !ok || len(payload) == 0 {
			b.Fatalf("EncodeRFX len=%d ok=%t", len(payload), ok)
		}
		b.SetBytes(int64(len(payload)))
	}
}

func BenchmarkBuildClearCodecEncoder_320x240(b *testing.B) {
	fr := benchmarkCodecFrame(320, 240)
	enc := clearCodecEncoder{}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		payload, ok := enc.EncodeRDPGFX(fr, fr.Width, fr.Height)
		if !ok || len(payload) == 0 {
			b.Fatalf("ClearCodec len=%d ok=%t", len(payload), ok)
		}
		b.SetBytes(int64(len(payload)))
	}
}

func BenchmarkBuildRDPGFXH264FramePDUs_320x240(b *testing.B) {
	unit := h264AccessUnit{PresentationTimeUS: 42, KeyFrame: true, Data: make([]byte, 4096)}
	for i := range unit.Data {
		unit.Data[i] = byte(i)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pdus, ok := buildRDPGFXH264FramePDUs(0, uint32(i+1), unit, 320, 240)
		if !ok || len(pdus) != 3 {
			b.Fatalf("buildRDPGFXH264FramePDUs len=%d ok=%t", len(pdus), ok)
		}
		b.SetBytes(totalPayloadBytes(pdus))
	}
}

func BenchmarkBuildRDPGFXUncompressedFramePDUs_320x240(b *testing.B) {
	fr := benchmarkCodecFrame(320, 240)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		pdus, ok := buildRDPGFXUncompressedFramePDUs(0, uint32(i+1), fr, fr.Width, fr.Height)
		if !ok || len(pdus) != 3 {
			b.Fatalf("buildRDPGFXUncompressedFramePDUs len=%d ok=%t", len(pdus), ok)
		}
		b.SetBytes(totalPayloadBytes(pdus))
	}
}

func BenchmarkBuildNSCodecSurfaceBitsCommand_320x240(b *testing.B) {
	fr := benchmarkCodecFrame(320, 240)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cmd, ok := buildNSCodecSurfaceBitsCommand(fr, bitmapCodecNSCodecDefaultID)
		if !ok || len(cmd) == 0 {
			b.Fatalf("buildNSCodecSurfaceBitsCommand len=%d ok=%t", len(cmd), ok)
		}
		b.SetBytes(int64(len(cmd)))
	}
}

func BenchmarkBuildJPEGSurfaceBitsCommand_320x240(b *testing.B) {
	fr := benchmarkCodecFrame(320, 240)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cmd, ok := buildJPEGSurfaceBitsCommand(fr, bitmapCodecJPEGDefaultID, 80)
		if !ok || len(cmd) == 0 {
			b.Fatalf("buildJPEGSurfaceBitsCommand len=%d ok=%t", len(cmd), ok)
		}
		b.SetBytes(int64(len(cmd)))
	}
}

func BenchmarkBuildPNGSurfaceBitsCommand_320x240(b *testing.B) {
	fr := benchmarkCodecFrame(320, 240)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cmd, ok := buildPNGSurfaceBitsCommand(fr, 10)
		if !ok || len(cmd) == 0 {
			b.Fatalf("buildPNGSurfaceBitsCommand len=%d ok=%t", len(cmd), ok)
		}
		b.SetBytes(int64(len(cmd)))
	}
}
