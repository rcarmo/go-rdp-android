package rdpserver

import (
	"sync/atomic"

	"github.com/rcarmo/go-rdp-android/internal/input"
)

type countingInputSink struct {
	sink          input.Sink
	inputEvents   *atomic.Int64
	rdpeiContacts *atomic.Int64
}

func (s *countingInputSink) PointerMove(x, y int) error {
	s.inputEvents.Add(1)
	if s.sink == nil {
		return nil
	}
	return s.sink.PointerMove(x, y)
}

func (s *countingInputSink) PointerButton(x, y int, buttons input.ButtonState, down bool) error {
	s.inputEvents.Add(1)
	if s.sink == nil {
		return nil
	}
	return s.sink.PointerButton(x, y, buttons, down)
}

func (s *countingInputSink) Key(scancode uint16, down bool) error {
	s.inputEvents.Add(1)
	if s.sink == nil {
		return nil
	}
	return s.sink.Key(scancode, down)
}

func (s *countingInputSink) Unicode(r rune) error {
	s.inputEvents.Add(1)
	if s.sink == nil {
		return nil
	}
	return s.sink.Unicode(r)
}

func (s *countingInputSink) PointerWheel(x, y int, delta int, horizontal bool) error {
	s.inputEvents.Add(1)
	if s.sink == nil {
		return nil
	}
	if wheelSink, ok := s.sink.(input.WheelSink); ok {
		return wheelSink.PointerWheel(x, y, delta, horizontal)
	}
	return nil
}

func (s *countingInputSink) TouchFrame(contacts []input.TouchContact) error {
	s.inputEvents.Add(1)
	s.rdpeiContacts.Add(int64(len(contacts)))
	if s.sink == nil {
		return nil
	}
	if touchSink, ok := s.sink.(input.TouchSink); ok {
		return touchSink.TouchFrame(contacts)
	}
	return nil
}
