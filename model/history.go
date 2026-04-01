package model

import (
	"github.com/jkleinne/lazyspeed/internal/jsonstore"
)

const historyFilePerm = 0600

// HistoryStore manages speed test result persistence and in-memory history.
type HistoryStore struct {
	Results    *SpeedTestResult
	Entries    []*SpeedTestResult
	store      *jsonstore.Store[SpeedTestResult]
	maxEntries int
}

// NewHistoryStore creates a HistoryStore with the given configuration.
func NewHistoryStore(cfg HistoryConfig) *HistoryStore {
	maxEntries := defaultMaxEntries
	if cfg.MaxEntries > 0 {
		maxEntries = cfg.MaxEntries
	}
	path := cfg.Path
	if path == "" {
		path = defaultHistoryPath()
	}
	return &HistoryStore{
		Entries:    make([]*SpeedTestResult, 0),
		store:      jsonstore.New[SpeedTestResult](path, maxEntries, historyFilePerm),
		maxEntries: maxEntries,
	}
}

// Append adds a result to the history and sets Results to the latest entry.
func (h *HistoryStore) Append(result *SpeedTestResult) {
	h.Entries = append(h.Entries, result)
	h.Results = result
}

// Load reads and parses the history file. Returns nil if the file does not exist.
// If the main file is corrupted, attempts recovery from the backup (.bak).
func (h *HistoryStore) Load() error {
	entries, err := h.store.Load()
	if err != nil {
		return err
	}
	h.Entries = entries
	if len(h.Entries) > 0 {
		h.Results = h.Entries[len(h.Entries)-1]
	}
	return nil
}

// Save writes the history to disk, enforcing the max-entries cap.
// Uses atomic write (temp file + rename) to prevent corruption from
// interrupted writes. Backs up the current file before overwriting.
func (h *HistoryStore) Save() error {
	if err := h.store.Save(h.Entries); err != nil {
		return err
	}
	// Sync in-memory slice to match what was written (truncated to maxEntries)
	if h.maxEntries > 0 && len(h.Entries) > h.maxEntries {
		h.Entries = h.Entries[len(h.Entries)-h.maxEntries:]
	}
	return nil
}
