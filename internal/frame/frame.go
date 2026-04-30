package frame

import "time"

// PixelFormat describes the pixel layout of a captured frame.
type PixelFormat string

const (
	PixelFormatRGBA8888 PixelFormat = "rgba8888"
	PixelFormatBGRA8888 PixelFormat = "bgra8888"
)

// Frame is a single screen frame ready for RDP encoding.
type Frame struct {
	Width     int
	Height    int
	Stride    int
	Format    PixelFormat
	Timestamp time.Time
	Data      []byte
}

// Source yields frames from Android MediaProjection or from a desktop mock.
type Source interface {
	Frames() <-chan Frame
	Close() error
}
