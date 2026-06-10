package rdpserver

import rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"

const progressiveMaxRects = rdpcodec.ProgressiveMaxRects

type progressiveRect struct {
	Left   uint16
	Top    uint16
	Right  uint16
	Bottom uint16
}

type progressivePayload struct {
	Width       int
	Height      int
	LayerCount  uint8
	Quant       uint8
	RegionRects []progressiveRect
	Data        []byte
}

func validateProgressivePayload(in progressivePayload) bool {
	return rdpcodec.ValidateProgressivePayload(upstreamProgressivePayload(in)) == nil
}

func buildProgressivePayload(in progressivePayload) ([]byte, bool) {
	out, err := rdpcodec.MarshalProgressivePayload(upstreamProgressivePayload(in))
	if err != nil {
		return nil, false
	}
	return out, true
}

func parseProgressivePayload(data []byte) (progressivePayload, bool) {
	parsed, err := rdpcodec.ParseProgressivePayloadAlias(data)
	if err != nil {
		return progressivePayload{}, false
	}
	return localProgressivePayload(parsed), true
}

func upstreamProgressivePayload(in progressivePayload) rdpcodec.ProgressivePayload {
	rects := make([]rdpcodec.ProgressiveRect, 0, len(in.RegionRects))
	for _, r := range in.RegionRects {
		rects = append(rects, rdpcodec.ProgressiveRect{Left: r.Left, Top: r.Top, Right: r.Right, Bottom: r.Bottom})
	}
	return rdpcodec.ProgressivePayload{Width: in.Width, Height: in.Height, LayerCount: in.LayerCount, Quant: in.Quant, RegionRects: rects, EncodedData: in.Data}
}

func localProgressivePayload(in rdpcodec.ProgressivePayload) progressivePayload {
	rects := make([]progressiveRect, 0, len(in.RegionRects))
	for _, r := range in.RegionRects {
		rects = append(rects, progressiveRect{Left: r.Left, Top: r.Top, Right: r.Right, Bottom: r.Bottom})
	}
	return progressivePayload{Width: in.Width, Height: in.Height, LayerCount: in.LayerCount, Quant: in.Quant, RegionRects: rects, Data: in.EncodedData}
}
