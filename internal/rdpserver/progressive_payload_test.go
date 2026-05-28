package rdpserver

import "testing"

func TestBuildAndParseProgressivePayload(t *testing.T) {
	in := progressivePayload{
		Width:      64,
		Height:     64,
		LayerCount: 2,
		Quant:      6,
		RegionRects: []progressiveRect{
			{Left: 0, Top: 0, Right: 64, Bottom: 64},
		},
		Data: []byte{1, 2, 3, 4, 5},
	}
	wire, ok := buildProgressivePayload(in)
	if !ok || len(wire) == 0 {
		t.Fatalf("buildProgressivePayload len=%d ok=%t", len(wire), ok)
	}
	decoded, ok := parseProgressivePayload(wire)
	if !ok {
		t.Fatal("parseProgressivePayload failed on built payload")
	}
	if decoded.Width != in.Width || decoded.Height != in.Height || decoded.LayerCount != in.LayerCount || decoded.Quant != in.Quant || len(decoded.RegionRects) != 1 || len(decoded.Data) != len(in.Data) {
		t.Fatalf("decoded mismatch: %#v", decoded)
	}
}

func TestBuildProgressivePayloadRejectsInvalid(t *testing.T) {
	bad := progressivePayload{}
	if _, ok := buildProgressivePayload(bad); ok {
		t.Fatal("buildProgressivePayload accepted invalid payload")
	}
}

func TestParseProgressivePayloadRejectsMalformed(t *testing.T) {
	if _, ok := parseProgressivePayload([]byte{1, 2, 3}); ok {
		t.Fatal("parseProgressivePayload accepted short payload")
	}
}

func FuzzParseProgressivePayload(f *testing.F) {
	seed := progressivePayload{
		Width:      64,
		Height:     64,
		LayerCount: 1,
		Quant:      5,
		RegionRects: []progressiveRect{
			{Left: 0, Top: 0, Right: 64, Bottom: 64},
		},
		Data: []byte{9, 8, 7, 6, 5, 4},
	}
	wire, ok := buildProgressivePayload(seed)
	if ok {
		f.Add(wire)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = parseProgressivePayload(data)
	})
}
