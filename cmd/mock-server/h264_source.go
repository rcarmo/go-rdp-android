package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/rdpserver"
)

type fileH264Source struct {
	frames chan rdpserver.H264Frame
}

func newFileH264Source(ctx context.Context, path string, fps int) (*fileH264Source, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- explicit operator-provided mock-server fixture path for H.264 protocol experiments.
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("H.264 file %q is empty", path)
	}
	if fps <= 0 {
		fps = 1
	}
	s := &fileH264Source{frames: make(chan rdpserver.H264Frame, 2)}
	interval := time.Second / time.Duration(fps)
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		defer close(s.frames)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		pts := int64(0)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				frame := rdpserver.H264Frame{PresentationTimeUS: pts, KeyFrame: true, Data: data}
				pts += interval.Microseconds()
				select {
				case s.frames <- frame:
				default:
					select {
					case <-s.frames:
					default:
					}
					s.frames <- frame
				}
			}
		}
	}()
	return s, nil
}

func (s *fileH264Source) H264Frames() <-chan rdpserver.H264Frame { return s.frames }
