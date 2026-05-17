package mobile

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/rcarmo/go-rdp-android/internal/rdpserver"
)

const maxEncodedFrameBytes = 1024 * 1024

// EncodedFrame is a bounded copy of one encoded H.264/AVC access unit.
type EncodedFrame = rdpserver.H264Frame

// EncodedFrameQueue keeps the latest encoded access units for future transport wiring.
type EncodedFrameQueue struct {
	mu        sync.Mutex
	frames    chan EncodedFrame
	closed    bool
	submitted atomic.Int64
	dropped   atomic.Int64
}

func NewEncodedFrameQueue(capacity int) *EncodedFrameQueue {
	if capacity <= 0 {
		capacity = 1
	}
	return &EncodedFrameQueue{frames: make(chan EncodedFrame, capacity)}
}

func (q *EncodedFrameQueue) Submit(frame EncodedFrame) error {
	if len(frame.Data) == 0 {
		return errors.New("encoded frame data is empty")
	}
	if len(frame.Data) > maxEncodedFrameBytes {
		return errors.New("encoded frame exceeds maximum size")
	}
	copyFrame := frame
	copyFrame.Data = append([]byte(nil), frame.Data...)

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return errors.New("encoded frame queue closed")
	}
	select {
	case q.frames <- copyFrame:
		q.submitted.Add(1)
		return nil
	default:
		select {
		case <-q.frames:
			q.dropped.Add(1)
		default:
		}
		q.frames <- copyFrame
		q.submitted.Add(1)
		return nil
	}
}

func (q *EncodedFrameQueue) Drain() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for {
		select {
		case <-q.frames:
		default:
			return
		}
	}
}

func (q *EncodedFrameQueue) H264Frames() <-chan rdpserver.H264Frame { return q.frames }
func (q *EncodedFrameQueue) Depth() int64                           { return int64(len(q.frames)) }
func (q *EncodedFrameQueue) Submitted() int64                       { return q.submitted.Load() }
func (q *EncodedFrameQueue) Dropped() int64                         { return q.dropped.Load() }
