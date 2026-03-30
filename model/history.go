package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// HistoryStore manages speed test result persistence and in-memory history.
type HistoryStore struct {
	Results *SpeedTestResult
	Entries []*SpeedTestResult
	config  HistoryConfig
}

// NewHistoryStore creates a HistoryStore with the given configuration.
func NewHistoryStore(cfg HistoryConfig) *HistoryStore {
	return &HistoryStore{
		Entries: make([]*SpeedTestResult, 0),
		config:  cfg,
	}
}

// Append adds a result to the history and sets Results to the latest entry.
func (h *HistoryStore) Append(result *SpeedTestResult) {
	h.Entries = append(h.Entries, result)
	h.Results = result
}

// historyPath returns the file path for the history JSON file.
func (h *HistoryStore) historyPath() string {
	if h.config.Path != "" {
		return h.config.Path
	}
	return defaultHistoryPath()
}

// Load reads and parses the history file. Returns nil if the file does not exist.
// If the main file is corrupted, attempts recovery from the backup (.bak).
func (h *HistoryStore) Load() error {
	path := h.historyPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read history file: %v", err)
	}

	if err := json.Unmarshal(data, &h.Entries); err != nil {
		// Main file is corrupted — attempt recovery from backup
		bakData, bakErr := os.ReadFile(path + ".bak")
		if bakErr != nil {
			return fmt.Errorf("failed to parse history file: %v", err)
		}
		if json.Unmarshal(bakData, &h.Entries) != nil {
			return fmt.Errorf("failed to parse history file (backup also corrupt): %v", err)
		}
	}

	if len(h.Entries) > 0 {
		h.Results = h.Entries[len(h.Entries)-1]
	}

	return nil
}

// Save writes the history to disk, enforcing the max-entries cap.
// Uses atomic write (temp file + rename) to prevent corruption from
// interrupted writes. Backs up the current file before overwriting.
func (h *HistoryStore) Save() error {
	path := h.historyPath()

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create history directory: %v", err)
	}

	maxEntries := defaultMaxEntries
	if h.config.MaxEntries > 0 {
		maxEntries = h.config.MaxEntries
	}
	if len(h.Entries) > maxEntries {
		h.Entries = h.Entries[len(h.Entries)-maxEntries:]
	}

	data, err := json.MarshalIndent(h.Entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize history: %v", err)
	}

	// Back up current file before overwriting (best-effort)
	if src, readErr := os.ReadFile(path); readErr == nil {
		_ = os.WriteFile(path+".bak", src, 0600)
	}

	// Atomic write: temp file + rename prevents corruption from interrupted writes.
	// 0600: history contains the user's IP address (PII).
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %v", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit history file: %v", err)
	}

	return nil
}
