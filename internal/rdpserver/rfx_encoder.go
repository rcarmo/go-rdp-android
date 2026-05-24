package rdpserver

import "github.com/rcarmo/go-rdp-android/internal/frame"

type RFXEncoder interface {
	EncodeRFX(frame.Frame, int, int) ([]byte, bool)
}

type rfxEncoderFunc func(frame.Frame, int, int) ([]byte, bool)

func (f rfxEncoderFunc) EncodeRFX(src frame.Frame, width, height int) ([]byte, bool) {
	return f(src, width, height)
}
