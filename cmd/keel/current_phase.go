package main

import (
	"os"

	"github.com/spf13/cobra"

	"keel/internal/currentphase"
)

var currentPhaseCmd = &cobra.Command{
	Use:   "current-phase",
	Short: "Report the current phase state of the target repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		code := currentphase.Run(repo, os.Stdout)
		if code != 0 {
			os.Exit(code)
		}
		return nil
	},
}

func init() {
	currentPhaseCmd.Flags().String("repo", ".", "Path to the target repository (default: current directory)")
	rootCmd.AddCommand(currentPhaseCmd)
}
