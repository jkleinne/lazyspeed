package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jkleinne/lazyspeed/model"
)

const exportTimestampFormat = "20060102_150405_000000000"

// ExportResult writes result to a file named lazyspeed_<timestamp>.<ext> in dir.
// format must be "json" or "csv". It returns the full path of the written file.
func ExportResult(result *model.SpeedTestResult, format string, dir string) (path string, err error) {
	timestampStr := result.Timestamp.Format(exportTimestampFormat)
	switch format {
	case formatJSON:
		path = filepath.Join(dir, fmt.Sprintf("lazyspeed_%s.json", timestampStr))
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to serialise result: %v", err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return "", fmt.Errorf("failed to write file: %v", err)
		}
		return path, nil

	case formatCSV:
		path = filepath.Join(dir, fmt.Sprintf("lazyspeed_%s.csv", timestampStr))
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return "", fmt.Errorf("failed to create file: %v", err)
		}
		defer func() {
			if cerr := f.Close(); cerr != nil && err == nil {
				err = fmt.Errorf("failed to close export file: %v", cerr)
			}
		}()
		csvWriter := csv.NewWriter(f)
		_ = csvWriter.Write(model.SpeedTestCSVHeader())
		_ = csvWriter.Write(result.CSVRow())
		csvWriter.Flush()
		if err = csvWriter.Error(); err != nil {
			return "", fmt.Errorf("failed to flush CSV writer: %v", err)
		}
		return path, nil

	default:
		return "", fmt.Errorf("unknown format %q: must be \"json\" or \"csv\"", format)
	}
}
