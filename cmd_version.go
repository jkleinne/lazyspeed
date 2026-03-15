package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of lazyspeed",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println(GetVersionInfo())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
