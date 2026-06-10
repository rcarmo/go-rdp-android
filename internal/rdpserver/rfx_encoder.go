package rdpserver

import (
	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

type RFXEncoder interface {
	EncodeRFX(frame.Frame, int, int) ([]byte, bool)
}

type rfxEncoderFunc func(frame.Frame, int, int) ([]byte, bool)

func (f rfxEncoderFunc) EncodeRFX(src frame.Frame, width, height int) ([]byte, bool) {
	return f(src, width, height)
}

type productionRFXEncoder struct{}

func (productionRFXEncoder) EncodeRFX(src frame.Frame, width, height int) ([]byte, bool) {
	if width < rfxTileSize || height < rfxTileSize || src.Width < rfxTileSize || src.Height < rfxTileSize {
		return nil, false
	}
	stride, ok := normalizedFrameStride(src)
	if !ok {
		return nil, false
	}
	format, ok := planarPixelFormat(src.Format)
	if !ok {
		return nil, false
	}
	payload, err := rdpcodec.EncodeRFXSingleTileFrame(rdpcodec.BitmapInput{Pixels: src.Data, Width: src.Width, Height: src.Height, Stride: stride, Format: format}, width, height, 1, 0, 0, nil)
	if err != nil {
		return nil, false
	}
	return payload, true
}
