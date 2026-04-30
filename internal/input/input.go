package input

// ButtonState represents pointer button state.
type ButtonState uint16

const (
	ButtonPrimary ButtonState = 1 << iota
	ButtonSecondary
	ButtonMiddle
)

// Sink receives decoded RDP input events and forwards them to Android.
type Sink interface {
	PointerMove(x, y int) error
	PointerButton(x, y int, buttons ButtonState, down bool) error
	Key(scancode uint16, down bool) error
	Unicode(r rune) error
}
