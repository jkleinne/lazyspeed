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
func (h *HistoryStore) Load() error {
	data, err := os.ReadFile(h.historyPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read history file: %v", err)
	}

	if err := json.Unmarshal(data, &h.Entries); err != nil {
		return fmt.Errorf("failed to parse history file: %v", err)
	}

	if len(h.Entries) > 0 {
		h.Results = h.Entries[len(h.Entries)-1]
	}

	return nil
}

// Save writes the history to disk, enforcing the max-entries cap.
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

	// 0600: history contains the user's IP address (PII)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write history file: %v", err)
	}

	return nil
}
