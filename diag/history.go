package diag

import (
	"fmt"

	"github.com/jkleinne/lazyspeed/internal/jsonstore"
)

const historyFilePerm = 0600

// LoadHistory reads and parses the diagnostics history file. Returns an empty
// slice if the file does not exist. If the main file is corrupted, attempts
// recovery from the backup (.bak).
func LoadHistory(path string) ([]*DiagResult, error) {
	s := jsonstore.New[DiagResult](path, 0, historyFilePerm)
	return s.Load()
}

// AppendHistory loads existing history, appends the result, and saves back.
func AppendHistory(path string, result *DiagResult, maxEntries int) error {
	s := jsonstore.New[DiagResult](path, maxEntries, historyFilePerm)
	history, err := s.Load()
	if err != nil {
		return fmt.Errorf("failed to load history for append: %v", err)
	}
	history = append(history, result)
	return s.Save(history)
}

// saveHistory writes diagnostics history to disk. Uses atomic write (temp file +
// rename) to prevent corruption from interrupted writes. Backs up the current
// file before overwriting.
func saveHistory(path string, results []*DiagResult, maxEntries int) error {
	s := jsonstore.New[DiagResult](path, maxEntries, historyFilePerm)
	return s.Save(results)
}
