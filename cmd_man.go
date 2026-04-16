package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var manDir string

var manCmd = &cobra.Command{
	Use:    "man",
	Short:  "Generate man pages for lazyspeed",
	Hidden: true,
	RunE: func(_ *cobra.Command, _ []string) error {
		if manDir == "" {
			return errors.New("--dir is required")
		}
		if err := os.MkdirAll(manDir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %v", err) //nolint:errorlint // project convention: %v not %w
		}
		header := &doc.GenManHeader{
			Title:   "LAZYSPEED",
			Section: "1",
		}
		if err := doc.GenManTree(rootCmd, header, manDir); err != nil {
			return fmt.Errorf("failed to generate man pages: %v", err) //nolint:errorlint // project convention: %v not %w
		}
		fmt.Printf("Man pages written to %s\n", manDir)
		return nil
	},
}

func init() {
	manCmd.Flags().StringVar(&manDir, "dir", "", "Output directory for man pages (required)")
	rootCmd.AddCommand(manCmd)
}
