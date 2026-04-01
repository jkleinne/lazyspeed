package jsonstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	backupSuffix = ".bak"
	tmpSuffix    = ".tmp"
	dirPerm      = 0700
)

// Store[T] manages atomic JSON file persistence with backup and recovery.
type Store[T any] struct {
	path       string
	maxEntries int
	filePerm   os.FileMode
}

// New creates a Store configured with path, max entries cap, and file permissions.
// If maxEntries <= 0, Save does not truncate.
func New[T any](path string, maxEntries int, perm os.FileMode) *Store[T] {
	return &Store[T]{
		path:       path,
		maxEntries: maxEntries,
		filePerm:   perm,
	}
}

// Load reads and unmarshals the JSON file into a slice of *T.
// Returns an empty slice if the file does not exist.
// If the main file is corrupt, attempts recovery from the .bak file.
func (s *Store[T]) Load() ([]*T, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*T{}, nil
		}
		return nil, fmt.Errorf("failed to read data file: %v", err)
	}

	var entries []*T
	if err := json.Unmarshal(data, &entries); err != nil {
		bakData, bakErr := os.ReadFile(s.path + backupSuffix)
		if bakErr != nil {
			return nil, fmt.Errorf("failed to parse data file (backup unreadable): %v", err)
		}
		if bakUnmarshalErr := json.Unmarshal(bakData, &entries); bakUnmarshalErr != nil {
			return nil, fmt.Errorf("failed to parse data file (backup also corrupt): main: %v, backup: %v", err, bakUnmarshalErr)
		}
	}
	if entries == nil {
		entries = []*T{}
	}
	return entries, nil
}

// Save marshals the slice to JSON, enforces the maxEntries cap (keeping the
// most recent), and writes atomically (temp file -> backup current -> rename).
// Skips backup if the current file contains invalid JSON.
func (s *Store[T]) Save(entries []*T) error {
	if err := os.MkdirAll(filepath.Dir(s.path), dirPerm); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	if s.maxEntries > 0 && len(entries) > s.maxEntries {
		entries = entries[len(entries)-s.maxEntries:]
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize data: %v", err)
	}

	tmpPath := s.path + tmpSuffix
	if err := os.WriteFile(tmpPath, data, s.filePerm); err != nil {
		return fmt.Errorf("failed to write data file: %v", err)
	}

	// Back up current file before overwriting (best-effort, only if valid JSON)
	if src, readErr := os.ReadFile(s.path); readErr == nil && json.Valid(src) {
		_ = os.WriteFile(s.path+backupSuffix, src, s.filePerm)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit data file: %v", err)
	}

	return nil
}
