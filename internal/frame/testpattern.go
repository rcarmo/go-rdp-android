package frame

import (
	"context"
	"sync"
	"time"
)

// TestPatternSource generates synthetic RGBA frames for desktop/CI testing.
type TestPatternSource struct {
	ctx    context.Context
	cancel context.CancelFunc
	frames chan Frame
	once   sync.Once
}

// NewTestPatternSource starts a synthetic frame source.
func NewTestPatternSource(width, height, fps int) *TestPatternSource {
	if width <= 0 {
		width = 320
	}
	if height <= 0 {
		height = 240
	}
	if fps <= 0 {
		fps = 5
	}
	// #nosec G118 -- cancel is retained on TestPatternSource and invoked in Close().
	ctx, cancel := context.WithCancel(context.Background())
	s := &TestPatternSource{ctx: ctx, cancel: cancel, frames: make(chan Frame, 2)}
	go s.run(width, height, fps)
	return s
}

func (s *TestPatternSource) run(width, height, fps int) {
	period := time.Second / time.Duration(fps)
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	frameNo := 0
	for {
		select {
		case <-s.ctx.Done():
			close(s.frames)
			return
		case now := <-ticker.C:
			f := buildPatternFrame(width, height, frameNo, now)
			frameNo++
			select {
			case s.frames <- f:
			default:
				select {
				case <-s.frames:
				default:
				}
				s.frames <- f
			}
		}
	}
}

// Frames implements Source.
func (s *TestPatternSource) Frames() <-chan Frame { return s.frames }

// Close implements Source.
func (s *TestPatternSource) Close() error {
	s.once.Do(s.cancel)
	return nil
}

func buildPatternFrame(width, height, frameNo int, ts time.Time) Frame {
	stride := width * 4
	data := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*stride + x*4
			data[i+0] = byte((x + frameNo*7) % 256)     // R
			data[i+1] = byte((y + frameNo*5) % 256)     // G
			data[i+2] = byte((x + y + frameNo*3) % 256) // B
			data[i+3] = 0xff
		}
	}
	return Frame{Width: width, Height: height, Stride: stride, Format: PixelFormatRGBA8888, Timestamp: ts, Data: data}
}
