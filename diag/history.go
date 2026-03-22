package diag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

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
		return nil, fmt.Errorf("failed to parse diagnostics history: %v", err)
	}
	return results, nil
}

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

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write diagnostics history: %v", err)
	}
	return nil
}
