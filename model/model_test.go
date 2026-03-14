package model

import (
	"testing"
)

func TestNewModel(t *testing.T) {
	m := NewModel()

	if m.Results != nil {
		t.Errorf("Expected Results to be nil, got %v", m.Results)
	}

	if m.TestHistory == nil {
		t.Errorf("Expected TestHistory to not be nil")
	} else if len(m.TestHistory) != 0 {
		t.Errorf("Expected TestHistory to be empty, got length %d", len(m.TestHistory))
	}

	if m.Testing != false {
		t.Errorf("Expected Testing to be false, got %t", m.Testing)
	}

	if m.Progress != 0 {
		t.Errorf("Expected Progress to be 0, got %f", m.Progress)
	}

	if m.CurrentPhase != "" {
		t.Errorf("Expected CurrentPhase to be empty, got %s", m.CurrentPhase)
	}

	if m.ShowHelp != true {
		t.Errorf("Expected ShowHelp to be true, got %t", m.ShowHelp)
	}

	if m.SelectingServer != false {
		t.Errorf("Expected SelectingServer to be false, got %t", m.SelectingServer)
	}

	if m.Cursor != 0 {
		t.Errorf("Expected Cursor to be 0, got %d", m.Cursor)
	}
}
