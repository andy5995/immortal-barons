package game

import "testing"

func TestDefaultMaxConcurrentSessions(t *testing.T) {
	if c := DefaultConfig(); c.MaxConcurrentSessions != 4 {
		t.Errorf("MaxConcurrentSessions = %d, want 4", c.MaxConcurrentSessions)
	}
}
