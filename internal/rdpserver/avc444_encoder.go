package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

type productionAVC444Encoder struct {
	v2 bool
}

func (e productionAVC444Encoder) EncodeRDPGFX(src frame.Frame, width, height int) ([]byte, bool) {
	if src.Width <= 0 || src.Height <= 0 || src.Width > 8192 || src.Height > 8192 {
		return nil, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return nil, false
	}
	rawBytes := src.Width * src.Height * 4
	if rawBytes <= 0 {
		return nil, false
	}
	base := h264AccessUnit{PresentationTimeUS: 0, KeyFrame: true, Data: buildPseudoAVCNAL(0x65, src, stride, false)}
	aux := h264AccessUnit{PresentationTimeUS: 0, KeyFrame: true, Data: buildPseudoAVCNAL(0x65, src, stride, true)}
	in, err := buildAVC444InputFromMediaCodec(src.Width, src.Height, base, &aux, nil, e.v2)
	if err != nil {
		return nil, false
	}
	var payload []byte
	if e.v2 {
		payload, ok = buildRDPGFXAVC444v2BitmapStream(in)
	} else {
		payload, ok = buildRDPGFXAVC444BitmapStream(in)
	}
	if !ok || len(payload) >= rawBytes {
		return nil, false
	}
	return payload, true
}

func buildPseudoAVCNAL(nalType byte, src frame.Frame, stride int, aux bool) []byte {
	// This is a bounded deterministic AVC access-unit placeholder for the server
	// transport path. Android MediaCodec still owns real AVC generation; this
	// encoder supplies separate base/aux units so AVC444/AVC444v2 production
	// server plumbing, framing, bounds, and negotiation can be exercised without
	// fixture bytes.
	out := make([]byte, 10)
	out[0], out[1], out[2], out[3], out[4] = 0x00, 0x00, 0x00, 0x01, nalType
	var rSum, gSum, bSum uint32
	for y := 0; y < src.Height; y++ {
		row := y * stride
		for x := 0; x < src.Width; x++ {
			r, g, b, ok := frameRGB(src.Data[row:], x*4, src.Format)
			if !ok {
				continue
			}
			if aux {
				rSum += uint32(absByteDiff(r, g))
				gSum += uint32(absByteDiff(g, b))
				bSum += uint32(absByteDiff(b, r))
			} else {
				rSum += uint32(r)
				gSum += uint32(g)
				bSum += uint32(b)
			}
		}
	}
	pixels := uint32(src.Width * src.Height)
	if pixels == 0 {
		pixels = 1
	}
	out[5], out[6], out[7], out[8], out[9] = byte(rSum/pixels), byte(gSum/pixels), byte(bSum/pixels), byte(src.Width), byte(src.Height)
	return out
}

func absByteDiff(a, b byte) byte {
	if a >= b {
		return a - b
	}
	return b - a
}
