package rdpserver

import "testing"

func TestBuildAVC444InputFromMediaCodec(t *testing.T) {
	base := h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x65, 0x88, 0x84}, KeyFrame: true}
	aux := h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x61, 0x00, 0x01}}
	in, err := buildAVC444InputFromMediaCodec(64, 64, base, &aux, nil, false)
	if err != nil {
		t.Fatalf("buildAVC444InputFromMediaCodec(valid): %v", err)
	}
	if len(in.RegionRects) != 1 || in.RegionRects[0].Right != 64 || in.RegionRects[0].Bottom != 64 {
		t.Fatalf("default rects = %#v", in.RegionRects)
	}
	if _, err := buildAVC444InputFromMediaCodec(64, 64, base, nil, nil, false); err == nil {
		t.Fatal("buildAVC444InputFromMediaCodec unexpectedly accepted nil aux layer")
	}
	pBase := base
	pBase.KeyFrame = false
	if _, err := buildAVC444InputFromMediaCodec(64, 64, pBase, &aux, nil, false); err == nil {
		t.Fatal("buildAVC444InputFromMediaCodec unexpectedly accepted non-keyframe base layer")
	}
}

func TestValidateAVC444EncoderInput(t *testing.T) {
	base := h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x65, 0x88, 0x84}, KeyFrame: true}
	aux := h264AccessUnit{PresentationTimeUS: 1, Data: []byte{0, 0, 0, 1, 0x61, 0x00, 0x01}}
	valid := avc444EncoderInput{
		Width:       64,
		Height:      64,
		BaseLayer:   base,
		AuxLayer:    aux,
		RegionRects: []avc444RegionRect{{Left: 0, Top: 0, Right: 64, Bottom: 64}},
	}
	if err := validateAVC444EncoderInput(valid); err != nil {
		t.Fatalf("validateAVC444EncoderInput(valid): %v", err)
	}

	cases := []struct {
		name string
		in   avc444EncoderInput
	}{
		{name: "bad dimensions", in: func() avc444EncoderInput { c := valid; c.Width = 0; return c }()},
		{name: "bad base", in: func() avc444EncoderInput { c := valid; c.BaseLayer.Data = nil; return c }()},
		{name: "bad aux", in: func() avc444EncoderInput { c := valid; c.AuxLayer.Data = nil; return c }()},
		{name: "base not keyframe", in: func() avc444EncoderInput { c := valid; c.BaseLayer.KeyFrame = false; return c }()},
		{name: "no rects", in: func() avc444EncoderInput { c := valid; c.RegionRects = nil; return c }()},
		{name: "empty rect", in: func() avc444EncoderInput {
			c := valid
			c.RegionRects = []avc444RegionRect{{Left: 8, Top: 8, Right: 8, Bottom: 16}}
			return c
		}()},
		{name: "out of bounds rect", in: func() avc444EncoderInput {
			c := valid
			c.RegionRects = []avc444RegionRect{{Left: 0, Top: 0, Right: 128, Bottom: 64}}
			return c
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateAVC444EncoderInput(tc.in); err == nil {
				t.Fatalf("validateAVC444EncoderInput(%s) unexpectedly succeeded", tc.name)
			}
		})
	}
}
