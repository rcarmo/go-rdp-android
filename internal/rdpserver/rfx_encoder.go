package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

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
	return buildRFXMessageSingleTile(src, width, height, 1, 0, 0, nil)
}
