package rdpserver

import (
	"encoding/binary"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBuildClassicBitmapPlanarUpdateRoundTrip(t *testing.T) {
	fr := solidRGBAFrameForClassicPlanarTest(16, 16, 0x10, 0x20, 0x30)
	update, ok := buildClassicBitmapPlanarUpdate(fr)
	if !ok {
		t.Fatal("buildClassicBitmapPlanarUpdate() ok = false")
	}
	if rects, err := parseBitmapUpdateHeader(update); err != nil || rects != 1 {
		t.Fatalf("parseBitmapUpdateHeader() rects=%d err=%v", rects, err)
	}
	if bpp := le16ForTest(update[4+12 : 4+14]); bpp != 32 {
		t.Fatalf("bpp = %d, want 32", bpp)
	}
	if flags := le16ForTest(update[4+14 : 4+16]); flags != bitmapCompressionFlag|noBitmapCompressionHeader {
		t.Fatalf("flags = 0x%04x", flags)
	}
	payloadLen := int(le16ForTest(update[4+16 : 4+18]))
	payload := update[4+18:]
	if payloadLen != len(payload) {
		t.Fatalf("payloadLen = %d, actual=%d", payloadLen, len(payload))
	}
	got, err := decodeClassicPlanarPayloadForTest(payload, fr.Width, fr.Height)
	if err != nil {
		t.Fatalf("decodeClassicPlanarPayloadForTest: %v", err)
	}
	want := fr.Data
	if string(got) != string(want) {
		t.Fatalf("decoded = %x, want %x", got, want)
	}
}

func solidRGBAFrameForClassicPlanarTest(width, height int, r, g, b byte) frame.Frame {
	data := make([]byte, width*height*4)
	for i := 0; i < len(data); i += 4 {
		data[i+0] = r
		data[i+1] = g
		data[i+2] = b
		data[i+3] = 0xff
	}
	return frame.Frame{Width: width, Height: height, Stride: width * 4, Format: frame.PixelFormatRGBA8888, Data: data}
}

func TestBuildClassicBitmapPlanarUpdateRejectsInvalid(t *testing.T) {
	if update, ok := buildClassicBitmapPlanarUpdate(frame.Frame{}); ok || update != nil {
		t.Fatal("expected empty frame rejection")
	}
	if update, ok := buildClassicBitmapPlanarUpdate(frame.Frame{Width: 1, Height: 1, Stride: 3, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3}}); ok || update != nil {
		t.Fatal("expected short stride/data rejection")
	}
}

func decodeClassicPlanarPayloadForTest(payload []byte, width, height int) ([]byte, error) {
	if len(payload) == 0 || payload[0] != 0x30 {
		return nil, errTestPlanar("unexpected planar header")
	}
	offset := 1
	planes := make([][]byte, 3)
	for i := 0; i < 3; i++ {
		plane, used, err := decodePlanarRLEPlaneForTest(payload[offset:], width, height)
		if err != nil {
			return nil, err
		}
		planes[i] = plane
		offset += used
	}
	if offset != len(payload) {
		return nil, errTestPlanar("trailing planar bytes")
	}
	out := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		srcY := height - 1 - y
		for x := 0; x < width; x++ {
			si := srcY*width + x
			di := (y*width + x) * 4
			out[di+0] = planes[0][si]
			out[di+1] = planes[1][si]
			out[di+2] = planes[2][si]
			out[di+3] = 0xff
		}
	}
	return out, nil
}

func TestBuildClassicBitmapPlanarUpdateRejectsExpansion(t *testing.T) {
	fr := frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3, 0xff}}
	if update, ok := buildClassicBitmapPlanarUpdate(fr); ok || update != nil {
		t.Fatal("expected tiny planar payload that expands raw 32bpp to be rejected")
	}
}

func TestClassicBitmapPlanarUpdateHeaderBounds(t *testing.T) {
	fr := frameOfSolidBGRAForRLETest(16, 16)
	update, ok := buildClassicBitmapPlanarUpdate(fr)
	if !ok {
		t.Fatal("buildClassicBitmapPlanarUpdate() ok = false")
	}
	if got := binary.LittleEndian.Uint16(update[4+8 : 4+10]); got != 16 {
		t.Fatalf("width = %d, want 16", got)
	}
	if got := binary.LittleEndian.Uint16(update[4+10 : 4+12]); got != 16 {
		t.Fatalf("height = %d, want 16", got)
	}
}
