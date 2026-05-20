package rdpserver

import "sync/atomic"

type serverMetrics struct {
	framesSent          *atomic.Int64
	bitmapBytes         *atomic.Int64
	bitmapRLEFrames     *atomic.Int64
	bitmapRLEBytes      *atomic.Int64
	bitmapRLESavedBytes *atomic.Int64
	rdpgfxFrames        *atomic.Int64
	rdpgfxBytes         *atomic.Int64
	h264Frames          *atomic.Int64
	h264Bytes           *atomic.Int64
	dvcFragments        *atomic.Int64
	h264Status          *atomic.Value
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

func (m serverMetrics) recordRDPGFXFrame(pdus [][]byte) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.rdpgfxFrames != nil {
		m.rdpgfxFrames.Add(1)
	}
	if m.rdpgfxBytes != nil {
		m.rdpgfxBytes.Add(totalPayloadBytes(pdus))
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
