package rdpserver

import (
	"fmt"
	"os"
	"strings"
)

const (
	h264GraphicsPathName = "h264-avc"
	h264MaxAccessUnitLen = 1024 * 1024
	h264MaxAccessUnits   = 4
)

// H264Frame is one encoded H.264/AVC access unit submitted by the Android encoder.
type H264Frame struct {
	PresentationTimeUS int64
	KeyFrame           bool
	CodecConfig        bool
	Data               []byte
}

// H264Source exposes encoded H.264/AVC access units to the RDPGFX transport.
type H264Source interface {
	H264Frames() <-chan H264Frame
}

type h264AccessUnit = H264Frame

func h264EnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_DISABLE_H264"))) {
	case "1", "true", "yes", "on":
		return false
	default:
		return true
	}
}

func validateH264AccessUnit(unit h264AccessUnit) error {
	if len(unit.Data) == 0 {
		return fmt.Errorf("empty H.264 access unit")
	}
	if len(unit.Data) > h264MaxAccessUnitLen {
		return fmt.Errorf("H.264 access unit length %d exceeds maximum %d", len(unit.Data), h264MaxAccessUnitLen)
	}
	if unit.PresentationTimeUS < 0 {
		return fmt.Errorf("negative H.264 presentation timestamp %d", unit.PresentationTimeUS)
	}
	return nil
}

type h264StreamState struct {
	codecConfig []byte
	seenKey     bool
}

func (s *h264StreamState) prepareForWire(unit h264AccessUnit) (h264AccessUnit, bool) {
	if err := validateH264AccessUnit(unit); err != nil {
		return h264AccessUnit{}, false
	}
	if unit.CodecConfig {
		s.codecConfig = append(s.codecConfig[:0], unit.Data...)
		if !unit.KeyFrame {
			return h264AccessUnit{}, false
		}
	}
	if unit.KeyFrame {
		s.seenKey = true
		if len(s.codecConfig) > 0 && !unit.CodecConfig {
			combined := make([]byte, 0, len(s.codecConfig)+len(unit.Data))
			combined = append(combined, s.codecConfig...)
			combined = append(combined, unit.Data...)
			unit.Data = combined
		}
		return unit, true
	}
	if !s.seenKey {
		return h264AccessUnit{}, false
	}
	return unit, true
}

func validateH264AccessUnitBatch(units []h264AccessUnit) error {
	if len(units) == 0 {
		return fmt.Errorf("empty H.264 access-unit batch")
	}
	if len(units) > h264MaxAccessUnits {
		return fmt.Errorf("H.264 access-unit batch size %d exceeds maximum %d", len(units), h264MaxAccessUnits)
	}
	for i, unit := range units {
		if err := validateH264AccessUnit(unit); err != nil {
			return fmt.Errorf("H.264 access unit %d: %w", i, err)
		}
	}
	return nil
}
