package rdpserver

import "fmt"

const avc444MaxRegionRects = 256

type avc444RegionRect struct {
	Left   uint16
	Top    uint16
	Right  uint16
	Bottom uint16
}

type avc444EncoderInput struct {
	Width  int
	Height int

	BaseLayer h264AccessUnit
	AuxLayer  h264AccessUnit

	RegionRects []avc444RegionRect
	UseV2       bool
}

func buildAVC444InputFromMediaCodec(width, height int, base h264AccessUnit, aux *h264AccessUnit, rects []avc444RegionRect, useV2 bool) (avc444EncoderInput, error) {
	in := avc444EncoderInput{Width: width, Height: height, BaseLayer: base, UseV2: useV2}
	if len(rects) == 0 && width > 0 && height > 0 {
		in.RegionRects = []avc444RegionRect{{Left: 0, Top: 0, Right: uint16(width), Bottom: uint16(height)}} // #nosec G115 -- width/height checked by validateAVC444EncoderInput.
	} else {
		in.RegionRects = append([]avc444RegionRect(nil), rects...)
	}
	if aux == nil || len(aux.Data) == 0 {
		return avc444EncoderInput{}, fmt.Errorf("AVC444 auxiliary plane access unit is required; current MediaCodec path only provides base layer")
	}
	in.AuxLayer = *aux
	if err := validateAVC444EncoderInput(in); err != nil {
		return avc444EncoderInput{}, err
	}
	return in, nil
}

func validateAVC444EncoderInput(in avc444EncoderInput) error {
	if in.Width <= 0 || in.Height <= 0 || in.Width > 8192 || in.Height > 8192 {
		return fmt.Errorf("invalid AVC444 dimensions %dx%d", in.Width, in.Height)
	}
	if err := validateH264AccessUnit(in.BaseLayer); err != nil {
		return fmt.Errorf("invalid AVC444 base layer: %w", err)
	}
	if err := validateH264AccessUnit(in.AuxLayer); err != nil {
		return fmt.Errorf("invalid AVC444 auxiliary layer: %w", err)
	}
	if len(in.RegionRects) == 0 {
		return fmt.Errorf("AVC444 requires at least one region rect")
	}
	if len(in.RegionRects) > avc444MaxRegionRects {
		return fmt.Errorf("AVC444 region rect count %d exceeds maximum %d", len(in.RegionRects), avc444MaxRegionRects)
	}
	for i, r := range in.RegionRects {
		if r.Right <= r.Left || r.Bottom <= r.Top {
			return fmt.Errorf("AVC444 region rect %d is empty", i)
		}
		if int(r.Right) > in.Width || int(r.Bottom) > in.Height {
			return fmt.Errorf("AVC444 region rect %d exceeds frame bounds", i)
		}
	}
	return nil
}
