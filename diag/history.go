package diag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadHistory reads and parses the diagnostics history file. Returns an empty
// slice if the file does not exist. If the main file is corrupted, attempts
// recovery from the backup (.bak).
func LoadHistory(path string) ([]*DiagResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []*DiagResult{}, nil
		}
		return nil, fmt.Errorf("failed to read diagnostics history: %v", err)
	}

	var results []*DiagResult
	if err := json.Unmarshal(data, &results); err != nil {
		// Main file is corrupted — attempt recovery from backup
		bakData, bakErr := os.ReadFile(path + ".bak")
		if bakErr != nil {
			return nil, fmt.Errorf("failed to parse diagnostics history: %v", err)
		}
		if json.Unmarshal(bakData, &results) != nil {
			return nil, fmt.Errorf("failed to parse diagnostics history (backup also corrupt): %v", err)
		}
	}
	return results, nil
}

// AppendHistory loads existing history, appends the result, and saves back.
func AppendHistory(path string, result *DiagResult, maxEntries int) error {
	history, err := LoadHistory(path)
	if err != nil {
		return fmt.Errorf("failed to load history for append: %v", err)
	}
	history = append(history, result)
	return SaveHistory(path, history, maxEntries)
}

// SaveHistory writes diagnostics history to disk. Uses atomic write (temp file +
// rename) to prevent corruption from interrupted writes. Backs up the current
// file before overwriting.
func SaveHistory(path string, results []*DiagResult, maxEntries int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("failed to create diagnostics directory: %v", err)
	}

	if len(results) > maxEntries {
		results = results[len(results)-maxEntries:]
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize diagnostics history: %v", err)
	}

	// Back up current file before overwriting (best-effort)
	if src, readErr := os.ReadFile(path); readErr == nil {
		_ = os.WriteFile(path+".bak", src, 0600)
	}

	// Atomic write: temp file + rename prevents corruption from interrupted writes
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write diagnostics history: %v", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit diagnostics history: %v", err)
	}
	return nil
}
