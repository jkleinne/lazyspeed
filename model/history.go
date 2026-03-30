package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	backupSuffix = ".bak"
	tmpSuffix    = ".tmp"
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
		bakData, bakErr := os.ReadFile(path + backupSuffix)
		if bakErr != nil {
			return fmt.Errorf("failed to parse history file: %v", err)
		}
		if bakUnmarshalErr := json.Unmarshal(bakData, &h.Entries); bakUnmarshalErr != nil {
			return fmt.Errorf("failed to parse history file (backup also corrupt): main: %v, backup: %v", err, bakUnmarshalErr)
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

	// Atomic write: temp file first, then backup, then rename.
	// This ordering ensures the main file stays intact until the final rename.
	// 0600: history contains the user's IP address (PII).
	tmpPath := path + tmpSuffix
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %v", err)
	}

	// Back up current file before overwriting (best-effort, only if valid JSON)
	if src, readErr := os.ReadFile(path); readErr == nil && json.Valid(src) {
		_ = os.WriteFile(path+backupSuffix, src, 0600)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit history file: %v", err)
	}

	return nil
}
