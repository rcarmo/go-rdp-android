package rdpserver

import rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"

const avc444MaxBitmapStreamLen = rdpcodec.AVCMaxBitmapStreamLen

func buildRDPGFXAVC444BitmapStream(in avc444EncoderInput) ([]byte, bool) {
	payload, err := rdpcodec.BuildAVC444BitmapStream(upstreamAVC444Input(in))
	if err != nil {
		return nil, false
	}
	return payload, true
}

func avc444BitmapStreamLen(in avc444EncoderInput) int {
	payload, ok := buildRDPGFXAVC444BitmapStream(in)
	if !ok {
		return 0
	}
	return len(payload)
}

func writeRDPGFXAVC444BitmapStream(out []byte, in avc444EncoderInput) bool {
	payload, ok := buildRDPGFXAVC444BitmapStream(in)
	if !ok || len(payload) > len(out) {
		return false
	}
	copy(out, payload)
	return true
}

func upstreamAVC444Input(in avc444EncoderInput) rdpcodec.AVC444Input {
	rects := make([]rdpcodec.ProgressiveRect, 0, len(in.RegionRects))
	for _, r := range in.RegionRects {
		rects = append(rects, rdpcodec.ProgressiveRect{Left: r.Left, Top: r.Top, Right: r.Right, Bottom: r.Bottom})
	}
	return rdpcodec.AVC444Input{
		Width:       in.Width,
		Height:      in.Height,
		BaseLayer:   rdpcodec.H264AccessUnit{PresentationTimeUS: in.BaseLayer.PresentationTimeUS, KeyFrame: in.BaseLayer.KeyFrame, CodecConfig: in.BaseLayer.CodecConfig, Data: in.BaseLayer.Data},
		AuxLayer:    rdpcodec.H264AccessUnit{PresentationTimeUS: in.AuxLayer.PresentationTimeUS, KeyFrame: in.AuxLayer.KeyFrame, CodecConfig: in.AuxLayer.CodecConfig, Data: in.AuxLayer.Data},
		RegionRects: rects,
		UseV2:       in.UseV2,
	}
}

func buildRDPGFXAVC444v2BitmapStream(in avc444EncoderInput) ([]byte, bool) {
	in.UseV2 = true
	return buildRDPGFXAVC444BitmapStream(in)
}
