package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

type fixtureFrameEncoder struct {
	path string
	data []byte
}

func newFixtureFrameEncoder(path string) (*fixtureFrameEncoder, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path) // #nosec G304 -- operator-provided fixture path for local codec transport experiments.
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("empty codec fixture %q", path)
	}
	return &fixtureFrameEncoder{path: path, data: append([]byte(nil), data...)}, nil
}

func (e *fixtureFrameEncoder) EncodeRFX(frame.Frame, int, int) ([]byte, bool) {
	if e == nil || len(e.data) == 0 {
		return nil, false
	}
	return append([]byte(nil), e.data...), true
}

func (e *fixtureFrameEncoder) EncodeRDPGFX(frame.Frame, int, int) ([]byte, bool) {
	if e == nil || len(e.data) == 0 {
		return nil, false
	}
	return append([]byte(nil), e.data...), true
}
