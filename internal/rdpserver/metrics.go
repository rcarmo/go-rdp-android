package rdpserver

import "sync/atomic"

type serverMetrics struct {
	framesSent   *atomic.Int64
	bitmapBytes  *atomic.Int64
	rdpgfxFrames *atomic.Int64
	rdpgfxBytes  *atomic.Int64
	dvcFragments *atomic.Int64
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

func totalPayloadBytes(payloads [][]byte) int64 {
	var bytes int64
	for _, payload := range payloads {
		bytes += int64(len(payload))
	}
	return bytes
}
