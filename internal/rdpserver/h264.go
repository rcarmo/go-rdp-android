package rdpserver

import (
	"encoding/binary"
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

func h264ForcedFromEnv() bool {
	if !h264EnabledFromEnv() {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_FORCE_H264"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
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
	annexB, ok := h264NormalizeAnnexB(unit.Data)
	if !ok {
		return h264AccessUnit{}, false
	}
	unit.Data = annexB
	if !unit.KeyFrame && h264AnnexBContainsNALType(unit.Data, 5) {
		unit.KeyFrame = true
	}
	if unit.CodecConfig {
		if h264AnnexBContainsNALType(unit.Data, 7) {
			s.codecConfig = append(s.codecConfig[:0], unit.Data...)
		} else {
			if len(s.codecConfig) > h264MaxAccessUnitLen-len(unit.Data) {
				return h264AccessUnit{}, false
			}
			s.codecConfig = append(s.codecConfig, unit.Data...)
		}
		if !unit.KeyFrame {
			return h264AccessUnit{}, false
		}
	}
	if unit.KeyFrame {
		s.seenKey = true
		if len(s.codecConfig) > 0 && !unit.CodecConfig {
			if len(s.codecConfig) > h264MaxAccessUnitLen-len(unit.Data) {
				return h264AccessUnit{}, false
			}
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

func h264NormalizeAnnexB(data []byte) ([]byte, bool) {
	if len(data) == 0 {
		return nil, false
	}
	if h264HasStartCode(data) {
		return append([]byte(nil), data...), true
	}
	return h264LengthPrefixedToAnnexB(data)
}

func h264HasStartCode(data []byte) bool {
	return len(data) >= 4 && data[0] == 0 && data[1] == 0 && (data[2] == 1 || data[2] == 0 && data[3] == 1)
}

func h264LengthPrefixedToAnnexB(data []byte) ([]byte, bool) {
	out := make([]byte, 0, len(data)+16)
	if cap(out) > h264MaxAccessUnitLen {
		out = make([]byte, 0, h264MaxAccessUnitLen)
	}
	for offset := 0; offset < len(data); {
		if len(data)-offset < 4 {
			return nil, false
		}
		nalLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		offset += 4
		if nalLen <= 0 || nalLen > len(data)-offset {
			return nil, false
		}
		if len(out) > h264MaxAccessUnitLen-4-nalLen {
			return nil, false
		}
		out = append(out, 0, 0, 0, 1)
		out = append(out, data[offset:offset+nalLen]...)
		offset += nalLen
	}
	return out, len(out) > 0
}

func h264AnnexBContainsNALType(data []byte, nalType byte) bool {
	for offset := 0; offset < len(data); {
		start, prefixLen, ok := h264FindStartCode(data, offset)
		if !ok {
			return false
		}
		nalStart := start + prefixLen
		if nalStart < len(data) && data[nalStart]&0x1f == nalType {
			return true
		}
		offset = nalStart + 1
	}
	return false
}

func h264FindStartCode(data []byte, offset int) (start int, prefixLen int, ok bool) {
	for i := offset; i+3 <= len(data); i++ {
		if data[i] == 0 && data[i+1] == 0 {
			if data[i+2] == 1 {
				return i, 3, true
			}
			if i+4 <= len(data) && data[i+2] == 0 && data[i+3] == 1 {
				return i, 4, true
			}
		}
	}
	return 0, 0, false
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
