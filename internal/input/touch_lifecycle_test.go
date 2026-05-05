package input

import "testing"

func TestTouchLifecycleCoalescerDownUpdateUp(t *testing.T) {
	c := NewTouchLifecycleCoalescer()

	out := c.ApplyFrame([]TouchContact{{ID: 1, X: 10, Y: 20, Flags: TouchDown | TouchInRange | TouchInContact}})
	if len(out) != 1 || out[0].Flags&TouchDown == 0 || c.ActiveCount() != 1 {
		t.Fatalf("unexpected down output=%#v active=%d", out, c.ActiveCount())
	}

	out = c.ApplyFrame([]TouchContact{{ID: 1, X: 15, Y: 25, Flags: TouchUpdate | TouchInRange | TouchInContact}})
	if len(out) != 1 || out[0].X != 15 || out[0].Flags&TouchUpdate == 0 || c.ActiveCount() != 1 {
		t.Fatalf("unexpected update output=%#v active=%d", out, c.ActiveCount())
	}

	out = c.ApplyFrame([]TouchContact{{ID: 1, X: 20, Y: 30, Flags: TouchUp | TouchInRange}})
	if len(out) != 1 || out[0].Flags&TouchUp == 0 || c.ActiveCount() != 0 {
		t.Fatalf("unexpected up output=%#v active=%d", out, c.ActiveCount())
	}
}

func TestTouchLifecycleCoalescerDropsStrayEvents(t *testing.T) {
	c := NewTouchLifecycleCoalescer()
	out := c.ApplyFrame([]TouchContact{
		{ID: 1, X: 10, Y: 20, Flags: TouchUpdate | TouchInRange | TouchInContact},
		{ID: 2, X: 30, Y: 40, Flags: TouchUp},
		{ID: 3, X: 50, Y: 60, Flags: TouchCanceled},
	})
	if len(out) != 0 || c.ActiveCount() != 0 {
		t.Fatalf("stray events should be dropped output=%#v active=%d", out, c.ActiveCount())
	}
}

func TestTouchLifecycleCoalescerDuplicateDownReplacesActiveContact(t *testing.T) {
	c := NewTouchLifecycleCoalescer()
	out := c.ApplyFrame([]TouchContact{{ID: 7, X: 10, Y: 20, Flags: TouchDown | TouchInContact}})
	if len(out) != 1 || c.ActiveCount() != 1 {
		t.Fatalf("unexpected first down output=%#v active=%d", out, c.ActiveCount())
	}

	out = c.ApplyFrame([]TouchContact{{ID: 7, X: 100, Y: 200, Flags: TouchDown | TouchInContact}})
	if len(out) != 1 || out[0].X != 100 || out[0].Y != 200 || c.ActiveCount() != 1 {
		t.Fatalf("duplicate down should restart contact output=%#v active=%d", out, c.ActiveCount())
	}

	out = c.ApplyFrame([]TouchContact{{ID: 7, X: 101, Y: 201, Flags: TouchUp}})
	if len(out) != 1 || c.ActiveCount() != 0 {
		t.Fatalf("up should finish restarted contact output=%#v active=%d", out, c.ActiveCount())
	}
}

func TestTouchLifecycleCoalescerCancelRemovesActiveContact(t *testing.T) {
	c := NewTouchLifecycleCoalescer()
	_ = c.ApplyFrame([]TouchContact{{ID: 4, X: 10, Y: 20, Flags: TouchDown | TouchInContact}})
	out := c.ApplyFrame([]TouchContact{{ID: 4, X: 11, Y: 21, Flags: TouchCanceled}})
	if len(out) != 1 || out[0].Flags&TouchCanceled == 0 || c.ActiveCount() != 0 {
		t.Fatalf("cancel should finish contact output=%#v active=%d", out, c.ActiveCount())
	}

	out = c.ApplyFrame([]TouchContact{{ID: 4, X: 11, Y: 21, Flags: TouchUp}})
	if len(out) != 0 {
		t.Fatalf("up after cancel should be dropped: %#v", out)
	}
}

func TestTouchLifecycleCoalescerPreservesOptionalMetadata(t *testing.T) {
	c := NewTouchLifecycleCoalescer()
	orientation := uint32(90)
	pressure := uint32(512)
	rect := &TouchRect{Left: -1, Top: -2, Right: 3, Bottom: 4}
	out := c.ApplyFrame([]TouchContact{{ID: 5, X: 10, Y: 20, Flags: TouchDown | TouchInContact, Rect: rect, Orientation: &orientation, Pressure: &pressure}})
	if len(out) != 1 {
		t.Fatalf("expected metadata contact, got %#v", out)
	}
	got := out[0]
	if got.Rect == nil || *got.Rect != *rect || got.Orientation == nil || *got.Orientation != orientation || got.Pressure == nil || *got.Pressure != pressure {
		t.Fatalf("optional metadata was not preserved: %#v", got)
	}

	updatedPressure := uint32(768)
	out = c.ApplyFrame([]TouchContact{{ID: 5, X: 11, Y: 21, Flags: TouchUpdate | TouchInContact, Pressure: &updatedPressure}})
	if len(out) != 1 || out[0].Pressure == nil || *out[0].Pressure != updatedPressure {
		t.Fatalf("updated optional metadata was not preserved: %#v", out)
	}
}

func TestTouchLifecycleCoalescerPreservesMultiContactFrame(t *testing.T) {
	c := NewTouchLifecycleCoalescer()
	out := c.ApplyFrame([]TouchContact{
		{ID: 1, X: 10, Y: 20, Flags: TouchDown | TouchInContact},
		{ID: 2, X: 30, Y: 40, Flags: TouchDown | TouchInContact},
	})
	if len(out) != 2 || c.ActiveCount() != 2 {
		t.Fatalf("unexpected multi-contact down output=%#v active=%d", out, c.ActiveCount())
	}

	out = c.ApplyFrame([]TouchContact{
		{ID: 1, X: 11, Y: 21, Flags: TouchUpdate | TouchInContact},
		{ID: 2, X: 31, Y: 41, Flags: TouchUp},
	})
	if len(out) != 2 || out[0].ID != 1 || out[1].ID != 2 || c.ActiveCount() != 1 {
		t.Fatalf("unexpected multi-contact update/up output=%#v active=%d", out, c.ActiveCount())
	}
}
