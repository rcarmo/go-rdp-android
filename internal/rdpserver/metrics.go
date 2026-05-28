package rdpserver

import "sync/atomic"

type serverMetrics struct {
	framesSent             *atomic.Int64
	bitmapBytes            *atomic.Int64
	bitmapRLEFrames        *atomic.Int64
	bitmapRLEBytes         *atomic.Int64
	bitmapRLESavedBytes    *atomic.Int64
	nsCodecFrames          *atomic.Int64
	nsCodecBytes           *atomic.Int64
	nsCodecRawBytes        *atomic.Int64
	nsCodecSavedBytes      *atomic.Int64
	jpegCodecFrames        *atomic.Int64
	jpegCodecBytes         *atomic.Int64
	jpegCodecRawBytes      *atomic.Int64
	jpegCodecSavedBytes    *atomic.Int64
	pngCodecFrames         *atomic.Int64
	pngCodecBytes          *atomic.Int64
	pngCodecRawBytes       *atomic.Int64
	pngCodecSavedBytes     *atomic.Int64
	rfxCodecFrames         *atomic.Int64
	rfxCodecBytes          *atomic.Int64
	rfxCodecRawBytes       *atomic.Int64
	rfxCodecSavedBytes     *atomic.Int64
	bitmapCodecStreamStops *atomic.Int64
	rdpgfxFrames           *atomic.Int64
	rdpgfxBytes            *atomic.Int64
	rdpgfxStreamStops      *atomic.Int64
	rdpgfxPath             *atomic.Value
	h264Frames             *atomic.Int64
	h264Bytes              *atomic.Int64
	dvcFragments           *atomic.Int64
	h264Status             *atomic.Value
	rfxEncoder             RFXEncoder
	clearCodecEncoder      RDPGFXFrameEncoder
	progressiveEncoder     RDPGFXFrameEncoder
	progressiveV2Encoder   RDPGFXFrameEncoder
	avc444Encoder          RDPGFXFrameEncoder
	avc444v2Encoder        RDPGFXFrameEncoder
}

func (m serverMetrics) recordH264Status(status string) {
	if m.h264Status != nil && status != "" {
		m.h264Status.Store(status)
	}
}

func (m serverMetrics) recordDVCFragment() {
	if m.dvcFragments != nil {
		m.dvcFragments.Add(1)
	}
}

func (m serverMetrics) recordBitmapFrame(updates [][]byte) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.bitmapBytes != nil {
		m.bitmapBytes.Add(totalPayloadBytes(updates))
	}
	rleCount, rleBytes, savedBytes := bitmapRLEStatsFromUpdates(updates)
	if rleCount > 0 {
		if m.bitmapRLEFrames != nil {
			m.bitmapRLEFrames.Add(1)
		}
		if m.bitmapRLEBytes != nil {
			m.bitmapRLEBytes.Add(rleBytes)
		}
		if m.bitmapRLESavedBytes != nil {
			m.bitmapRLESavedBytes.Add(savedBytes)
		}
	}
}

func (m serverMetrics) recordNSCodecFrame(commands [][]byte) {
	m.recordNSCodecFrameSavings(commands, 0, 0)
}

func (m serverMetrics) recordNSCodecFrameSavings(commands [][]byte, rawBytes, savedBytes int64) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.nsCodecFrames != nil {
		m.nsCodecFrames.Add(1)
	}
	if m.nsCodecBytes != nil {
		m.nsCodecBytes.Add(totalPayloadBytes(commands))
	}
	if rawBytes > 0 && m.nsCodecRawBytes != nil {
		m.nsCodecRawBytes.Add(rawBytes)
	}
	if savedBytes > 0 && m.nsCodecSavedBytes != nil {
		m.nsCodecSavedBytes.Add(savedBytes)
	}
}

func (m serverMetrics) recordJPEGCodecFrame(commands [][]byte) {
	m.recordJPEGCodecFrameSavings(commands, 0, 0)
}

func (m serverMetrics) recordJPEGCodecFrameSavings(commands [][]byte, rawBytes, savedBytes int64) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.jpegCodecFrames != nil {
		m.jpegCodecFrames.Add(1)
	}
	if m.jpegCodecBytes != nil {
		m.jpegCodecBytes.Add(totalPayloadBytes(commands))
	}
	if rawBytes > 0 && m.jpegCodecRawBytes != nil {
		m.jpegCodecRawBytes.Add(rawBytes)
	}
	if savedBytes > 0 && m.jpegCodecSavedBytes != nil {
		m.jpegCodecSavedBytes.Add(savedBytes)
	}
}

func (m serverMetrics) recordPNGCodecFrame(commands [][]byte) {
	m.recordPNGCodecFrameSavings(commands, 0, 0)
}

func (m serverMetrics) recordPNGCodecFrameSavings(commands [][]byte, rawBytes, savedBytes int64) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.pngCodecFrames != nil {
		m.pngCodecFrames.Add(1)
	}
	if m.pngCodecBytes != nil {
		m.pngCodecBytes.Add(totalPayloadBytes(commands))
	}
	if rawBytes > 0 && m.pngCodecRawBytes != nil {
		m.pngCodecRawBytes.Add(rawBytes)
	}
	if savedBytes > 0 && m.pngCodecSavedBytes != nil {
		m.pngCodecSavedBytes.Add(savedBytes)
	}
}

func (m serverMetrics) recordRFXCodecFrame(commands [][]byte, rawBytes, savedBytes int64) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.rfxCodecFrames != nil {
		m.rfxCodecFrames.Add(1)
	}
	if m.rfxCodecBytes != nil {
		m.rfxCodecBytes.Add(totalPayloadBytes(commands))
	}
	if rawBytes > 0 && m.rfxCodecRawBytes != nil {
		m.rfxCodecRawBytes.Add(rawBytes)
	}
	if savedBytes > 0 && m.rfxCodecSavedBytes != nil {
		m.rfxCodecSavedBytes.Add(savedBytes)
	}
}

func (m serverMetrics) recordRDPGFXFrame(pdus [][]byte) {
	m.recordRDPGFXFramePath(pdus, "rdpgfx-planar")
}

func (m serverMetrics) recordBitmapCodecStreamStop() {
	if m.bitmapCodecStreamStops != nil {
		m.bitmapCodecStreamStops.Add(1)
	}
}

func (m serverMetrics) recordRDPGFXStreamStop() {
	if m.rdpgfxStreamStops != nil {
		m.rdpgfxStreamStops.Add(1)
	}
}

func (m serverMetrics) recordRDPGFXFramePath(pdus [][]byte, path string) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.rdpgfxFrames != nil {
		m.rdpgfxFrames.Add(1)
	}
	if m.rdpgfxBytes != nil {
		m.rdpgfxBytes.Add(totalPayloadBytes(pdus))
	}
	if m.rdpgfxPath != nil && path != "" {
		m.rdpgfxPath.Store(path)
	}
}

func (m serverMetrics) recordH264Frame(accessUnits [][]byte) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.h264Frames != nil {
		m.h264Frames.Add(1)
	}
	if m.h264Bytes != nil {
		m.h264Bytes.Add(totalPayloadBytes(accessUnits))
	}
}

func totalPayloadBytes(payloads [][]byte) int64 {
	var bytes int64
	for _, payload := range payloads {
		bytes += int64(len(payload))
	}
	return bytes
}
