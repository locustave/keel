package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"keel/internal/session"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate reports from the session event ledger",
}

var reportSessionCmd = &cobra.Command{
	Use:   "session [session-id]",
	Short: "Print a summary report for the active (or named) session",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		var sessDir string
		if len(args) > 0 {
			// Explicit session ID — resolve to directory.
			sessDir = repo + "/.agent/sessions/" + args[0]
		} else {
			var err error
			sessDir, err = session.FindCurrentSessionDir(repo)
			if err != nil {
				return fmt.Errorf("no active session: %w", err)
			}
		}
		return session.SessionReport(sessDir, os.Stdout)
	},
}

var reportPhaseCmd = &cobra.Command{
	Use:   "phase <phase-id>",
	Short: "Print a report for a specific phase within the active session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		sessDir, err := session.FindCurrentSessionDir(repo)
		if err != nil {
			return fmt.Errorf("no active session: %w", err)
		}
		return session.PhaseReport(sessDir, args[0], os.Stdout)
	},
}

var reportTokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Print a token usage report for the active session",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		groupBy, _ := cmd.Flags().GetString("group-by")
		sessDir, err := session.FindCurrentSessionDir(repo)
		if err != nil {
			return fmt.Errorf("no active session: %w", err)
		}
		return session.TokenReport(sessDir, groupBy, os.Stdout)
	},
}

var reportToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Print a tool usage report for the active session",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		sessDir, err := session.FindCurrentSessionDir(repo)
		if err != nil {
			return fmt.Errorf("no active session: %w", err)
		}
		return session.ToolReport(sessDir, os.Stdout)
	},
}

var reportDebugCmd = &cobra.Command{
	Use:   "debug",
	Short: "Print a debug-loop report for the active session",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		sessDir, err := session.FindCurrentSessionDir(repo)
		if err != nil {
			return fmt.Errorf("no active session: %w", err)
		}
		return session.DebugReport(sessDir, os.Stdout)
	},
}

func init() {
	for _, sub := range []*cobra.Command{
		reportSessionCmd, reportPhaseCmd, reportTokensCmd, reportToolsCmd, reportDebugCmd,
	} {
		sub.Flags().String("repo", ".", "Path to repository root")
	}
	reportTokensCmd.Flags().String("group-by", "", "Group tokens by: phase, implementer, or actor")

	reportCmd.AddCommand(reportSessionCmd, reportPhaseCmd, reportTokensCmd, reportToolsCmd, reportDebugCmd)
	rootCmd.AddCommand(reportCmd)
}
