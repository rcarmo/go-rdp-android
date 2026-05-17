package rdpserver

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestParseRDPGFXCapsAdvertise(t *testing.T) {
	payload := []byte{0x02, 0x00}
	payload = appendLE32Bytes(payload, rdpgfxCapsVersion8)
	payload = appendLE32Bytes(payload, 4)
	payload = appendLE32Bytes(payload, 0)
	payload = appendLE32Bytes(payload, rdpgfxCapsVersion106)
	payload = appendLE32Bytes(payload, 4)
	payload = appendLE32Bytes(payload, 0x00000003)
	pduBytes := buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, payload)

	pdu, err := parseRDPGFXPDU(pduBytes)
	if err != nil {
		t.Fatal(err)
	}
	if pdu.CmdID != rdpgfxCmdCapsAdvertise || len(pdu.Caps) != 2 {
		t.Fatalf("unexpected pdu: cmd=0x%x caps=%d", pdu.CmdID, len(pdu.Caps))
	}
	best, ok := negotiateRDPGFXCapability(pdu.Caps)
	if !ok {
		t.Fatal("expected negotiated capability")
	}
	if best.Version != rdpgfxCapsVersion106 || best.Flags != 0x00000003 {
		t.Fatalf("unexpected negotiated capability: version=0x%08x flags=0x%08x", best.Version, best.Flags)
	}
}

func TestParseRDPGFXRejectsMalformedPDUs(t *testing.T) {
	cases := map[string][]byte{
		"short header": {0x01, 0x00},
		"length mismatch": func() []byte {
			p := buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{1, 0, 0, 0, 0, 0, 0, 0})
			binary.LittleEndian.PutUint32(p[4:8], uint32(len(p)+1))
			return p
		}(),
		"empty caps": buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{0, 0}),
		"short caps": buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{2, 0, 0, 0, 0, 0, 0, 0}),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseRDPGFXPDU(data); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestBuildRDPGFXCapsConfirmPDU(t *testing.T) {
	pdu := buildRDPGFXCapsConfirmPDU(rdpgfxCapabilitySet{Version: rdpgfxCapsVersion10, Flags: 7})
	parsed, err := parseRDPGFXPDU(pdu)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.CmdID != rdpgfxCmdCapsConfirm {
		t.Fatalf("unexpected cmd 0x%x", parsed.CmdID)
	}
	if binary.LittleEndian.Uint32(pdu[8:12]) != rdpgfxCapsVersion10 || binary.LittleEndian.Uint32(pdu[12:16]) != 4 || binary.LittleEndian.Uint32(pdu[16:20]) != 7 {
		t.Fatalf("unexpected caps confirm payload: %x", pdu[8:])
	}
}

func TestBuildRDPGFXSurfaceAndFramePDUs(t *testing.T) {
	create, ok := buildRDPGFXCreateSurfacePDU(1, 640, 480)
	if !ok {
		t.Fatal("expected create surface PDU")
	}
	if pdu, err := parseRDPGFXPDU(create); err != nil || pdu.CmdID != rdpgfxCmdCreateSurface {
		t.Fatalf("unexpected create pdu=%#v err=%v", pdu, err)
	}
	mapped, ok := buildRDPGFXMapSurfaceToOutputPDU(1, 0, 0)
	if !ok {
		t.Fatal("expected map surface PDU")
	}
	if pdu, err := parseRDPGFXPDU(mapped); err != nil || pdu.CmdID != rdpgfxCmdMapSurfaceToOutput {
		t.Fatalf("unexpected map pdu=%#v err=%v", pdu, err)
	}

	src := frame.Frame{
		Width:     16,
		Height:    1,
		Stride:    64,
		Format:    frame.PixelFormatRGBA8888,
		Timestamp: time.Now(),
		Data:      repeatRGBAForTest(16, 0x11, 0x22, 0x33, 0xff),
	}
	pdus, ok := buildRDPGFXPlanarFramePDUs(1, 42, src, 16, 1)
	if !ok || len(pdus) != 3 {
		t.Fatalf("expected three frame PDUs, ok=%t len=%d", ok, len(pdus))
	}
	for i, want := range []uint16{rdpgfxCmdStartFrame, rdpgfxCmdWireToSurface1, rdpgfxCmdEndFrame} {
		pdu, err := parseRDPGFXPDU(pdus[i])
		if err != nil {
			t.Fatalf("parse frame pdu %d: %v", i, err)
		}
		if pdu.CmdID != want {
			t.Fatalf("frame pdu %d cmd=0x%x want=0x%x", i, pdu.CmdID, want)
		}
	}
	wire := pdus[1]
	if codec := binary.LittleEndian.Uint16(wire[10:12]); codec != rdpgfxCodecPlanar {
		t.Fatalf("codec=0x%x", codec)
	}
	payload := wire[25:]
	if len(payload) == 0 || payload[0] != 0x30 {
		t.Fatalf("expected no-alpha RLE planar payload, got=%x", payload)
	}
	if len(payload) >= 64 {
		t.Fatalf("expected compact planar payload smaller than raw XRGB, got %d bytes", len(payload))
	}
}

func repeatRGBAForTest(count int, r, g, b, a byte) []byte {
	out := make([]byte, 0, count*4)
	for i := 0; i < count; i++ {
		out = append(out, r, g, b, a)
	}
	return out
}

func FuzzParseRDPGFXPDU(f *testing.F) {
	f.Add(buildRDPGFXPDU(rdpgfxCmdCapsAdvertise, 0, []byte{1, 0, 4, 0, 8, 0, 0, 0, 0, 0}))
	f.Add([]byte{0x01, 0x00})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseRDPGFXPDU(data)
	})
}

func TestBuildPlanarRLEPayloadRoundTrip(t *testing.T) {
	const width, height = 48, 4
	data := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := (y*width + x) * 4
			// Mix repeated spans, zero deltas across rows, and positive/negative deltas.
			if x < 18 {
				data[i+0] = 0x20
				data[i+1] = 0x40
				data[i+2] = 0x60
			} else if x < 34 {
				data[i+0] = byte(0x70 + y)
				data[i+1] = byte(0x50 - y)
				data[i+2] = byte(0x30 + x - 18)
			} else {
				data[i+0] = byte((x*7 + y*3) & 0xff)
				data[i+1] = byte((x*5 - y*2) & 0xff)
				data[i+2] = byte((x*3 + y*11) & 0xff)
			}
			data[i+3] = 0xff
		}
	}
	fr := frame.Frame{Width: width, Height: height, Stride: width * 4, Format: frame.PixelFormatRGBA8888, Data: data}
	payload, ok := buildPlanarRLEPayload(fr, fr.Stride)
	if !ok {
		t.Fatal("expected planar payload")
	}
	got, err := decodePlanarRLEPayloadForTest(payload, width, height)
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			si := (y*width + x) * 4
			gi := (y*width + x) * 3
			want := data[si : si+3]
			if string(got[gi:gi+3]) != string(want) {
				t.Fatalf("pixel %d,%d got RGB=%x want=%x", x, y, got[gi:gi+3], want)
			}
		}
	}
}

func decodePlanarRLEPayloadForTest(payload []byte, width, height int) ([]byte, error) {
	if len(payload) == 0 || payload[0] != 0x30 {
		return nil, errTestPlanar("unexpected planar header")
	}
	offset := 1
	planes := make([][]byte, 3)
	for i := range planes {
		plane, used, err := decodePlanarRLEPlaneForTest(payload[offset:], width, height)
		if err != nil {
			return nil, err
		}
		offset += used
		planes[i] = plane
	}
	if offset != len(payload) {
		return nil, errTestPlanar("trailing planar bytes")
	}
	out := make([]byte, width*height*3)
	for i := 0; i < width*height; i++ {
		out[i*3+0] = planes[0][i]
		out[i*3+1] = planes[1][i]
		out[i*3+2] = planes[2][i]
	}
	return out, nil
}

func decodePlanarRLEPlaneForTest(data []byte, width, height int) ([]byte, int, error) {
	out := make([]byte, width*height)
	offset := 0
	for y := 0; y < height; y++ {
		pixel := byte(0)
		for x := 0; x < width; {
			if offset >= len(data) {
				return nil, 0, errTestPlanar("short RLE plane")
			}
			control := data[offset]
			offset++
			runLen := int(control & 0x0f)
			rawLen := int((control >> 4) & 0x0f)
			if runLen == 1 {
				runLen = rawLen + 16
				rawLen = 0
			} else if runLen == 2 {
				runLen = rawLen + 32
				rawLen = 0
			}
			if x+rawLen+runLen > width || offset+rawLen > len(data) {
				return nil, 0, errTestPlanar("invalid RLE scanline")
			}
			for ; rawLen > 0; rawLen-- {
				pixel = data[offset]
				offset++
				out[y*width+x] = decodePlanarRLEValueForTest(pixel, out, width, x, y)
				x++
			}
			for ; runLen > 0; runLen-- {
				out[y*width+x] = decodePlanarRLEValueForTest(pixel, out, width, x, y)
				x++
			}
		}
	}
	return out, offset, nil
}

func decodePlanarRLEValueForTest(encoded byte, plane []byte, width, x, y int) byte {
	if y == 0 {
		return encoded
	}
	delta := int(encoded >> 1)
	if encoded&1 != 0 {
		delta = -(delta + 1)
	}
	return byte(int(plane[(y-1)*width+x]) + delta)
}

type errTestPlanar string

func (e errTestPlanar) Error() string { return string(e) }
