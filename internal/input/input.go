package input

// ButtonState represents pointer button state.
type ButtonState uint16

const (
	ButtonPrimary ButtonState = 1 << iota
	ButtonSecondary
	ButtonMiddle
)

// TouchFlags represents RDPEI touch contact lifecycle flags.
type TouchFlags uint32

const (
	TouchDown TouchFlags = 1 << iota
	TouchUpdate
	TouchUp
	TouchInRange
	TouchInContact
	TouchCanceled
)

// TouchContact is a decoded true RDP touch contact, separate from mouse/pointer events.
type TouchContact struct {
	ID    uint8
	X     int
	Y     int
	Flags TouchFlags
}

// Sink receives decoded RDP input events and forwards them to Android.
type Sink interface {
	PointerMove(x, y int) error
	PointerButton(x, y int, buttons ButtonState, down bool) error
	Key(scancode uint16, down bool) error
	Unicode(r rune) error
}

// TouchSink is optionally implemented by sinks that can receive true RDPEI
// touch contacts separately from classic pointer events.
type TouchSink interface {
	TouchFrame(contacts []TouchContact) error
}
