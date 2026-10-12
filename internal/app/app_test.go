package app

import "testing"

// TestWindowRemembersItsSize pins the state key, which is the whole of the
// remembering: with it the framework restores the size the user left the
// window at, and without it the app opens at the default size every time,
// with nothing failing and nothing to notice but the window being the wrong
// size.
func TestWindowRemembersItsSize(t *testing.T) {
	o := windowOptions(nil)
	if o.StateKey == "" {
		t.Error("the window has no state key, so it forgets the size it was left at")
	}
	if o.MinWidth > o.Width || o.MinHeight > o.Height {
		t.Errorf("the window opens at %dx%d, smaller than the %dx%d it is held to",
			o.Width, o.Height, o.MinWidth, o.MinHeight)
	}
}
