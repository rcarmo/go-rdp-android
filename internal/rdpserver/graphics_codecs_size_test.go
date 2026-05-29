package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func solidCodecFrame(width, height int, rgba [4]byte) frame.Frame {
	stride := width * 4
	data := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*stride + x*4
			copy(data[i:i+4], rgba[:])
		}
	}
	return frame.Frame{Width: width, Height: height, Stride: stride, Format: frame.PixelFormatRGBA8888, Data: data}
}

func TestRDPGFXPlanarBuilderAllocationSmoke(t *testing.T) {
	fr := benchmarkCodecFrame(320, 240)
	allocs := testing.AllocsPerRun(5, func() {
		pdus, ok := buildRDPGFXPlanarFramePDUs(0, 1, fr, fr.Width, fr.Height)
		if !ok || len(pdus) != 3 {
			t.Fatalf("buildRDPGFXPlanarFramePDUs len=%d ok=%t", len(pdus), ok)
		}
	})
	if allocs > 20 {
		t.Fatalf("RDPGFX Planar allocations = %.1f, want <= 20", allocs)
	}
}

func TestGraphicsCodecBuilderSizeSmoke(t *testing.T) {
	fr := solidCodecFrame(64, 64, [4]byte{0x33, 0x66, 0x99, 0xff})
	uncompressedBytes := fr.Width * fr.Height * 4

	planar, ok := buildRDPGFXPlanarFramePDUs(0, 1, fr, fr.Width, fr.Height)
	if !ok {
		t.Fatal("buildRDPGFXPlanarFramePDUs() ok = false")
	}
	planarBytes := int(totalPayloadBytes(planar))
	if planarBytes <= 0 || planarBytes >= uncompressedBytes {
		t.Fatalf("planarBytes = %d, want 0 < size < raw %d", planarBytes, uncompressedBytes)
	}

	uncompressed, ok := buildRDPGFXUncompressedFramePDUs(0, 1, fr, fr.Width, fr.Height)
	if !ok {
		t.Fatal("buildRDPGFXUncompressedFramePDUs() ok = false")
	}
	uncompressedPDUBytes := int(totalPayloadBytes(uncompressed))
	if uncompressedPDUBytes <= uncompressedBytes {
		t.Fatalf("uncompressedPDUBytes = %d, want protocol overhead above raw %d", uncompressedPDUBytes, uncompressedBytes)
	}
	if planarBytes >= uncompressedPDUBytes {
		t.Fatalf("planarBytes = %d, want less than uncompressed PDU bytes %d", planarBytes, uncompressedPDUBytes)
	}

	ns, ok := buildNSCodecSurfaceBitsCommand(fr, bitmapCodecNSCodecDefaultID)
	if !ok {
		t.Fatal("buildNSCodecSurfaceBitsCommand() ok = false")
	}
	if len(ns) <= 22 || len(ns) >= uncompressedBytes {
		t.Fatalf("NSCodec command bytes = %d, want header and less than raw %d", len(ns), uncompressedBytes)
	}

	jpegCmd, ok := buildJPEGSurfaceBitsCommand(fr, bitmapCodecJPEGDefaultID, 80)
	if !ok {
		t.Fatal("buildJPEGSurfaceBitsCommand() ok = false")
	}
	if len(jpegCmd) <= 22 || len(jpegCmd) >= uncompressedBytes {
		t.Fatalf("JPEG command bytes = %d, want header and less than raw %d", len(jpegCmd), uncompressedBytes)
	}

	pngCmd, ok := buildPNGSurfaceBitsCommand(fr, 10)
	if !ok {
		t.Fatal("buildPNGSurfaceBitsCommand() ok = false")
	}
	if len(pngCmd) <= 22 || len(pngCmd) >= uncompressedBytes {
		t.Fatalf("PNG command bytes = %d, want header and less than raw %d", len(pngCmd), uncompressedBytes)
	}
}
