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
	units := splitAnnexBH264AccessUnits(data)
	if len(units) == 0 {
		units = [][]byte{data}
	} else {
		units = coalesceH264FixtureConfigUnits(units)
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
		index := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				unit := units[index%len(units)]
				index++
				keyFrame := h264FixtureContainsNALType(unit, 5)
				codecConfig := !keyFrame && (h264FixtureContainsNALType(unit, 7) || h264FixtureContainsNALType(unit, 8))
				frame := rdpserver.H264Frame{PresentationTimeUS: pts, KeyFrame: keyFrame, CodecConfig: codecConfig, Data: unit}
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

func splitAnnexBH264AccessUnits(data []byte) [][]byte {
	starts := h264FixtureStartCodeOffsets(data)
	if len(starts) == 0 || starts[0] != 0 {
		return nil
	}
	units := make([][]byte, 0, len(starts))
	for i, start := range starts {
		end := len(data)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if end > start {
			units = append(units, append([]byte(nil), data[start:end]...))
		}
	}
	return units
}

func coalesceH264FixtureConfigUnits(units [][]byte) [][]byte {
	out := make([][]byte, 0, len(units))
	var config []byte
	flushConfig := func() {
		if len(config) == 0 {
			return
		}
		out = append(out, append([]byte(nil), config...))
		config = nil
	}
	for _, unit := range units {
		isConfig := h264FixtureContainsNALType(unit, 7) || h264FixtureContainsNALType(unit, 8)
		isKeyFrame := h264FixtureContainsNALType(unit, 5)
		if isConfig && !isKeyFrame {
			config = append(config, unit...)
			continue
		}
		flushConfig()
		out = append(out, unit)
	}
	flushConfig()
	return out
}

func h264FixtureStartCodeOffsets(data []byte) []int {
	var out []int
	for i := 0; i+3 <= len(data); i++ {
		if data[i] == 0 && data[i+1] == 0 && (data[i+2] == 1 || i+4 <= len(data) && data[i+2] == 0 && data[i+3] == 1) {
			out = append(out, i)
			if data[i+2] == 1 {
				i += 2
			} else {
				i += 3
			}
		}
	}
	return out
}

func h264FixtureContainsNALType(data []byte, nalType byte) bool {
	starts := h264FixtureStartCodeOffsets(data)
	for _, start := range starts {
		nal := start + 3
		if start+3 < len(data) && data[start+2] == 0 {
			nal = start + 4
		}
		if nal < len(data) && data[nal]&0x1f == nalType {
			return true
		}
	}
	return false
}

func h264FixtureContainsIDR(data []byte) bool {
	return h264FixtureContainsNALType(data, 5)
}

func (s *fileH264Source) H264Frames() <-chan rdpserver.H264Frame { return s.frames }
