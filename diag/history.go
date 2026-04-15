package diag

import (
	"fmt"

	"github.com/jkleinne/lazyspeed/internal/jsonstore"
)

const historyFilePerm = 0600

// LoadHistory reads and parses the diagnostics history file. Returns an empty
// slice if the file does not exist. If the main file is corrupted, attempts
// recovery from the backup (.bak).
func LoadHistory(path string) ([]*Result, error) {
	s := jsonstore.New[Result](path, 0, historyFilePerm)
	return s.Load()
}

// AppendHistory loads existing history, appends the result, and saves back.
func AppendHistory(path string, result *Result, maxEntries int) error {
	s := jsonstore.New[Result](path, maxEntries, historyFilePerm)
	history, err := s.Load()
	if err != nil {
		return fmt.Errorf("failed to load history for append: %v", err) //nolint:errorlint // project convention: %v not %w
	}
	history = append(history, result)
	return s.Save(history)
}
