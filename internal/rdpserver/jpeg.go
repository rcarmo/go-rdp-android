package rdpserver

import (
	"image/jpeg"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

const bitmapCodecJPEGDefaultID byte = 2

type jpegSurfaceBitsWriter struct {
	buf []byte
}

func newJPEGSurfaceBitsWriter(capacity int) jpegSurfaceBitsWriter {
	if capacity < 0 {
		capacity = 0
	}
	return jpegSurfaceBitsWriter{buf: make([]byte, 0, capacity)}
}

func (w *jpegSurfaceBitsWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *jpegSurfaceBitsWriter) WriteByte(b byte) error {
	w.buf = append(w.buf, b)
	return nil
}

func (w *jpegSurfaceBitsWriter) Flush() error { return nil }

func (w *jpegSurfaceBitsWriter) Bytes() []byte { return w.buf }

func buildJPEGSurfaceBitsCommand(src frame.Frame, codecID byte, quality int) ([]byte, bool) {
	if codecID == 0 {
		codecID = bitmapCodecJPEGDefaultID
	}
	if quality <= 0 || quality > 100 {
		quality = 75
	}
	img, ok := frameToRGBAImage(src)
	if !ok {
		return nil, false
	}
	buf := newJPEGSurfaceBitsWriter(surfaceBitsHeaderLen + jpegSurfaceBitsCapacityHint(src.Width, src.Height))
	_, _ = buf.Write(emptySurfaceBitsHeader[:])
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, false
	}
	out := buf.Bytes()
	encodedLen := len(out) - surfaceBitsHeaderLen
	if !validSurfaceBitsCommand(src.Width, src.Height, codecID, encodedLen) {
		return nil, false
	}
	writeSurfaceBitsHeader(out[:surfaceBitsHeaderLen], src.Width, src.Height, codecID, encodedLen)
	return out, true
}

func jpegSurfaceBitsCapacityHint(width, height int) int {
	if width <= 0 || height <= 0 {
		return 4096
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/height {
		return 4096
	}
	hint := width * height / 4
	if hint < 4096 {
		return 4096
	}
	return hint
}
