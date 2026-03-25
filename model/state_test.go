package model

import "testing"

func TestModelStateValues(t *testing.T) {
	var s ModelState
	if s != StateIdle {
		t.Errorf("Expected zero value to be StateIdle, got %d", s)
	}

	states := []ModelState{StateIdle, StateAwaitingServers, StateSelectingServer, StateTesting, StateExporting}
	seen := make(map[ModelState]bool)
	for _, st := range states {
		if seen[st] {
			t.Errorf("Duplicate ModelState value: %d", st)
		}
		seen[st] = true
	}
}
