package rdpserver

import (
	"bytes"
	"image/png"
	"sync"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

var pngSurfaceBitsBufferPool pooledPNGEncoderBufferPool

type pooledPNGEncoderBufferPool struct {
	pool sync.Pool
}

func (p *pooledPNGEncoderBufferPool) Get() *png.EncoderBuffer {
	if v := p.pool.Get(); v != nil {
		if b, ok := v.(*png.EncoderBuffer); ok {
			return b
		}
	}
	return new(png.EncoderBuffer)
}

func (p *pooledPNGEncoderBufferPool) Put(b *png.EncoderBuffer) {
	if b != nil {
		p.pool.Put(b)
	}
}

func buildPNGSurfaceBitsCommand(src frame.Frame, codecID byte) ([]byte, bool) {
	if codecID == 0 {
		return nil, false
	}
	img, ok := frameToRGBAImage(src)
	if !ok {
		return nil, false
	}
	var buf bytes.Buffer
	buf.Grow(surfaceBitsHeaderLen + pngSurfaceBitsCapacityHint(src.Width, src.Height))
	_, _ = buf.Write(emptySurfaceBitsHeader[:])
	encoder := png.Encoder{CompressionLevel: pngCompressionLevelFromEnv(), BufferPool: &pngSurfaceBitsBufferPool}
	if err := encoder.Encode(&buf, img); err != nil {
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

func pngSurfaceBitsCapacityHint(width, height int) int {
	if width <= 0 || height <= 0 {
		return 4096
	}
	maxInt := int(^uint(0) >> 1)
	if width > maxInt/height {
		return 4096
	}
	// PNG is diagnostic/operator-only here; prefer avoiding repeated buffer growth
	// for typical compressible screen content without pessimistically reserving a
	// full raw frame.
	hint := width * height / 8
	if hint < 4096 {
		return 4096
	}
	return hint
}
