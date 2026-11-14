package httpx

import "testing"

// Vectors produced by @kastov/uuid-color 0.1.0.
func TestColorsMatchUpstream(t *testing.T) {
	for id, want := range map[uint64]string{1: "#0b05e5", 42: "#727622", 123456789: "#3ffd60"} {
		if got := ColorFromID(id); got != want {
			t.Errorf("ColorFromID(%d) = %s, want %s", id, got, want)
		}
	}
	if got := ColorFromUUID("1b34af8e-0d2d-42d7-8fc7-f9524d984583"); got != "#984583" {
		t.Errorf("ColorFromUUID = %s", got)
	}
}
