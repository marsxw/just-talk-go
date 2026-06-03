package hotkey

import (
	"testing"
	"time"
)

func TestStandardComboKeyUpFiresWhenModifierReleasesFirst(t *testing.T) {
	tracker := NewKeyStateTracker()
	ch := make(chan Event, 4)
	combo := Combo{Mods: ModAlt, Key: KeyR}
	tracker.Watch(combo, ch)

	events := tracker.KeyDown(KeyAlt, time.Now())
	if len(events) != 0 {
		t.Fatalf("Alt down emitted %d events, want 0", len(events))
	}

	events = tracker.KeyDown(KeyR, time.Now())
	if len(events) != 1 || events[0].Combo != combo || events[0].Type != KeyDown {
		t.Fatalf("R down emitted %#v, want one KeyDown for %s", events, combo)
	}

	events = tracker.KeyUp(KeyAlt, time.Now())
	if len(events) != 1 || events[0].Combo != combo || events[0].Type != KeyUp {
		t.Fatalf("Alt up emitted %#v, want one KeyUp for %s", events, combo)
	}

	events = tracker.KeyUp(KeyR, time.Now())
	if len(events) != 0 {
		t.Fatalf("R up emitted %d events, want 0", len(events))
	}
}

func TestStandardComboKeyDownFiresWhenModifierPressedAfterKey(t *testing.T) {
	tracker := NewKeyStateTracker()
	ch := make(chan Event, 4)
	combo := Combo{Mods: ModAlt, Key: KeyR}
	tracker.Watch(combo, ch)

	events := tracker.KeyDown(KeyR, time.Now())
	if len(events) != 0 {
		t.Fatalf("R down emitted %d events, want 0", len(events))
	}

	events = tracker.KeyDown(KeyAlt, time.Now())
	if len(events) != 1 || events[0].Combo != combo || events[0].Type != KeyDown {
		t.Fatalf("Alt down emitted %#v, want one KeyDown for %s", events, combo)
	}

	events = tracker.KeyUp(KeyR, time.Now())
	if len(events) != 1 || events[0].Combo != combo || events[0].Type != KeyUp {
		t.Fatalf("R up emitted %#v, want one KeyUp for %s", events, combo)
	}

	events = tracker.KeyUp(KeyAlt, time.Now())
	if len(events) != 0 {
		t.Fatalf("Alt up emitted %d events, want 0", len(events))
	}
}
