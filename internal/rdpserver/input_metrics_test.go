package rdpserver

import (
	"sync/atomic"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/input"
)

type metricsRecordingSink struct {
	wheels  int
	touches int
}

func (s *metricsRecordingSink) PointerMove(_, _ int) error { return nil }
func (s *metricsRecordingSink) PointerButton(_, _ int, _ input.ButtonState, _ bool) error { return nil }
func (s *metricsRecordingSink) Key(_ uint16, _ bool) error { return nil }
func (s *metricsRecordingSink) Unicode(_ rune) error { return nil }
func (s *metricsRecordingSink) PointerWheel(_, _ int, _ int, _ bool) error {
	s.wheels++
	return nil
}
func (s *metricsRecordingSink) TouchFrame(contacts []input.TouchContact) error {
	s.touches += len(contacts)
	return nil
}

func TestCountingInputSink(t *testing.T) {
	var events atomic.Int64
	var contacts atomic.Int64
	recording := &metricsRecordingSink{}
	sink := &countingInputSink{sink: recording, inputEvents: &events, rdpeiContacts: &contacts}

	calls := []func() error{
		func() error { return sink.PointerMove(1, 2) },
		func() error { return sink.PointerButton(1, 2, input.ButtonPrimary, true) },
		func() error { return sink.Key(0x1e, true) },
		func() error { return sink.Unicode('A') },
		func() error { return sink.PointerWheel(1, 2, -120, false) },
		func() error { return sink.TouchFrame([]input.TouchContact{{ID: 1}, {ID: 2}}) },
	}
	for _, call := range calls {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	if events.Load() != int64(len(calls)) {
		t.Fatalf("unexpected event count %d", events.Load())
	}
	if contacts.Load() != 2 {
		t.Fatalf("unexpected RDPEI contact count %d", contacts.Load())
	}
	if recording.wheels != 1 || recording.touches != 2 {
		t.Fatalf("sink forwarding failed wheels=%d touches=%d", recording.wheels, recording.touches)
	}
}
