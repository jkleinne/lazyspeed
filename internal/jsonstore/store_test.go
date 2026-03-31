package jsonstore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type testEntry struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestStore_LoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	s := New[testEntry](path, 0, 0600)

	entries, err := s.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(entries))
	}
}

func TestStore_SaveAndLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := New[testEntry](path, 0, 0600)

	entries := []*testEntry{
		{Name: "a", Value: 1},
		{Name: "b", Value: 2},
	}
	if err := s.Save(entries); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(loaded))
	}
	if loaded[0].Name != "a" || loaded[1].Value != 2 {
		t.Errorf("unexpected values: %+v, %+v", loaded[0], loaded[1])
	}
}

func TestStore_SaveCreatesDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "data.json")
	s := New[testEntry](path, 0, 0600)

	if err := s.Save([]*testEntry{{Name: "x", Value: 1}}); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to be created at %s", path)
	}
}

func TestStore_SaveEnforcesMaxEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := New[testEntry](path, 3, 0600)

	var entries []*testEntry
	for i := range 10 {
		entries = append(entries, &testEntry{Name: "e", Value: i})
	}
	if err := s.Save(entries); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded))
	}
	if loaded[0].Value != 7 {
		t.Errorf("expected oldest kept Value 7, got %d", loaded[0].Value)
	}
}

func TestStore_SaveNoTruncationWhenMaxEntriesZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := New[testEntry](path, 0, 0600)

	var entries []*testEntry
	for i := range 100 {
		entries = append(entries, &testEntry{Name: "e", Value: i})
	}
	if err := s.Save(entries); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 100 {
		t.Errorf("expected 100 entries (no truncation), got %d", len(loaded))
	}
}

func TestStore_LoadBackupRecovery(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	bakPath := path + ".bak"
	s := New[testEntry](path, 0, 0600)

	entries := []*testEntry{{Name: "recovered", Value: 42}}

	// Two saves: first creates file, second creates .bak
	if err := s.Save(entries); err != nil {
		t.Fatalf("first save failed: %v", err)
	}
	if err := s.Save(entries); err != nil {
		t.Fatalf("second save failed: %v", err)
	}

	// Corrupt main + valid backup — should recover
	_ = os.WriteFile(path, []byte("invalid json"), 0600)
	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("expected backup recovery, got: %v", err)
	}
	if len(loaded) != 1 || loaded[0].Value != 42 {
		t.Errorf("unexpected recovered data: %+v", loaded)
	}

	// Corrupt main + no backup — should error
	_ = os.Remove(bakPath)
	_, err = s.Load()
	if err == nil {
		t.Error("expected error with corrupt file and no backup")
	}

	// Corrupt main + corrupt backup — should error
	_ = os.WriteFile(bakPath, []byte("also bad"), 0600)
	_, err = s.Load()
	if err == nil {
		t.Error("expected error when both files corrupt")
	}
}

func TestStore_SaveSkipsCorruptBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	bakPath := path + ".bak"
	s := New[testEntry](path, 0, 0600)

	if err := s.Save([]*testEntry{{Name: "valid", Value: 1}}); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Corrupt main file
	_ = os.WriteFile(path, []byte("corrupt"), 0600)

	// Save new data — should NOT back up the corrupt file
	if err := s.Save([]*testEntry{{Name: "new", Value: 2}}); err != nil {
		t.Fatalf("save over corrupt failed: %v", err)
	}

	if bakData, err := os.ReadFile(bakPath); err == nil {
		if string(bakData) == "corrupt" {
			t.Error("backup contains corrupt data — json.Valid guard failed")
		}
	}
}

func TestStore_SavePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := New[testEntry](path, 0, 0600)

	if err := s.Save([]*testEntry{{Name: "x", Value: 1}}); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("permissions = %o, want 0600", perm)
	}
}

func TestStore_SaveNilSlice(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := New[testEntry](path, 0, 0600)

	if err := s.Save(nil); err != nil {
		t.Fatalf("save nil failed: %v", err)
	}

	loaded, err := s.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected 0 entries for nil save, got %d", len(loaded))
	}
}

func TestStore_SaveIndented(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	s := New[testEntry](path, 0, 0600)

	if err := s.Save([]*testEntry{{Name: "x", Value: 1}}); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	var indented json.RawMessage
	if err := json.Unmarshal(data, &indented); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	expected, _ := json.MarshalIndent([]*testEntry{{Name: "x", Value: 1}}, "", "  ")
	if string(data) != string(expected) {
		t.Errorf("expected indented output:\n%s\ngot:\n%s", expected, data)
	}
}
