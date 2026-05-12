package rdpserver

import "sync/atomic"

type serverMetrics struct {
	framesSent  *atomic.Int64
	bitmapBytes *atomic.Int64
}

func (m serverMetrics) recordBitmapFrame(updates [][]byte) {
	if m.framesSent != nil {
		m.framesSent.Add(1)
	}
	if m.bitmapBytes != nil {
		var bytes int64
		for _, update := range updates {
			bytes += int64(len(update))
		}
		m.bitmapBytes.Add(bytes)
	}
}
