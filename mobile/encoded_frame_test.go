package mobile

import "testing"

func TestEncodedFrameQueueSubmitCopiesAndDropsOldest(t *testing.T) {
	q := NewEncodedFrameQueue(1)
	data := []byte{1, 2, 3}
	if err := q.Submit(EncodedFrame{PresentationTimeUS: 1, KeyFrame: true, Data: data}); err != nil {
		t.Fatalf("Submit first: %v", err)
	}
	data[0] = 9
	if err := q.Submit(EncodedFrame{PresentationTimeUS: 2, Data: []byte{4}}); err != nil {
		t.Fatalf("Submit second: %v", err)
	}
	if got := q.Submitted(); got != 2 {
		t.Fatalf("Submitted() = %d, want 2", got)
	}
	if got := q.Dropped(); got != 1 {
		t.Fatalf("Dropped() = %d, want 1", got)
	}
	if got := q.Depth(); got != 1 {
		t.Fatalf("Depth() = %d, want 1", got)
	}
}

func TestEncodedFrameQueueRejectsInvalidPayloads(t *testing.T) {
	q := NewEncodedFrameQueue(1)
	if err := q.Submit(EncodedFrame{}); err == nil {
		t.Fatal("Submit empty payload error = nil, want error")
	}
	if err := q.Submit(EncodedFrame{Data: make([]byte, maxEncodedFrameBytes+1)}); err == nil {
		t.Fatal("Submit oversize payload error = nil, want error")
	}
}

func TestServerSubmitH264FrameQueuesEncodedFrame(t *testing.T) {
	s := NewServer()
	if err := s.SubmitH264Frame(123, true, false, []byte{1, 2}); err != nil {
		t.Fatalf("SubmitH264Frame() error = %v", err)
	}
	if got := s.H264SubmittedFrames(); got != 1 {
		t.Fatalf("H264SubmittedFrames() = %d, want 1", got)
	}
	if got := s.H264QueuedFrames(); got != 1 {
		t.Fatalf("H264QueuedFrames() = %d, want 1", got)
	}
}
