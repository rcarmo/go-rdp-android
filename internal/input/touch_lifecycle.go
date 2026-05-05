package input

// TouchLifecycleCoalescer tracks active RDPEI contacts and only forwards
// lifecycle-consistent touch contacts to downstream sinks. It deliberately
// stays small and allocation-light so protocol handlers can use it as a safety
// guard before events cross the gomobile/Android boundary.
type TouchLifecycleCoalescer struct {
	active map[uint8]TouchContact
}

// NewTouchLifecycleCoalescer returns an empty touch lifecycle coalescer.
func NewTouchLifecycleCoalescer() *TouchLifecycleCoalescer {
	return &TouchLifecycleCoalescer{active: make(map[uint8]TouchContact)}
}

// Reset drops all remembered active contacts.
func (c *TouchLifecycleCoalescer) Reset() {
	if c == nil {
		return
	}
	clear(c.active)
}

// ActiveCount returns the number of active contacts currently tracked.
func (c *TouchLifecycleCoalescer) ActiveCount() int {
	if c == nil {
		return 0
	}
	return len(c.active)
}

// ApplyFrame normalizes one RDPEI touch frame. Stray UPDATE/UP/CANCELED events
// are ignored, duplicate DOWN events replace the previous active contact for
// the same ID, and terminal UP/CANCELED contacts remove active state.
func (c *TouchLifecycleCoalescer) ApplyFrame(contacts []TouchContact) []TouchContact {
	if c == nil || len(contacts) == 0 {
		return nil
	}
	if c.active == nil {
		c.active = make(map[uint8]TouchContact)
	}
	out := make([]TouchContact, 0, len(contacts))
	for _, contact := range contacts {
		flags := contact.Flags
		isDown := flags&TouchDown != 0
		isUpdate := flags&TouchUpdate != 0
		isUp := flags&TouchUp != 0
		isCanceled := flags&TouchCanceled != 0
		_, wasActive := c.active[contact.ID]

		switch {
		case isDown:
			c.active[contact.ID] = contact
			out = append(out, contact)
			if isUp || isCanceled {
				delete(c.active, contact.ID)
			}
		case isUpdate || isUp || isCanceled:
			if !wasActive {
				continue
			}
			out = append(out, contact)
			if isUp || isCanceled {
				delete(c.active, contact.ID)
			} else {
				c.active[contact.ID] = contact
			}
		case wasActive:
			// Some peers may omit explicit UPDATE while still sending in-contact
			// coordinates. Preserve active lifecycle and forward the position.
			c.active[contact.ID] = contact
			out = append(out, contact)
		}
	}
	return out
}
