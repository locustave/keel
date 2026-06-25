package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"keel/internal/rollback"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback <phase>",
	Short: "Roll back a build phase and all downstream phases",
	Long: `Roll back phase N and any downstream phases that have rollback DAGs.

By default this is a dry run — it prints what would happen without making changes.
Pass --confirm to execute the rollback.

The rollback DAG for each phase is written to .agent/snapshots/phase_N.rollback.json
when the phase gate is recorded. If that file is missing, rollback cannot proceed.

Example:
  keel rollback 3            # dry run
  keel rollback 3 --confirm  # execute`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase := 0
		if _, err := fmt.Sscanf(args[0], "%d", &phase); err != nil || phase < 0 {
			fmt.Fprintln(os.Stderr, "error: phase must be a non-negative integer")
			os.Exit(1)
		}
		confirm, _ := cmd.Flags().GetBool("confirm")
		repo, _ := cmd.Flags().GetString("repo")
		code := rollback.Run(rollback.Options{
			Phase:    phase,
			RepoPath: repo,
			Confirm:  confirm,
			Out:      os.Stdout,
		})
		if code != 0 {
			os.Exit(code)
		}
		return nil
	},
}

var rollbackWriteDAGCmd = &cobra.Command{
	Use:   "write-dag",
	Short: "Derive and write a rollback DAG from a passed gate file",
	Long: `Derive a rollback DAG from an existing gate file and write it to
.agent/snapshots/phase_N.rollback.json.

Add this to the Phase Completion Checklist immediately after the gate file
is written. It can also retroactively backfill DAGs for phases completed
before this step was added.

Example:
  keel rollback write-dag --phase 3
  keel rollback write-dag --phase 3 --repo /path/to/project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		phase, _ := cmd.Flags().GetInt("phase")
		repo, _ := cmd.Flags().GetString("repo")
		if phase < 0 {
			return fmt.Errorf("--phase is required and must be >= 0")
		}
		dag, err := rollback.DeriveFromGate(repo, phase)
		if err != nil {
			return err
		}
		if err := rollback.WriteDAG(repo, *dag); err != nil {
			return fmt.Errorf("write DAG: %w", err)
		}
		fmt.Fprintf(os.Stdout, "Rollback DAG written: .agent/snapshots/phase_%d.rollback.json\n", phase)
		return nil
	},
}

func init() {
	rollbackCmd.Flags().Bool("confirm", false, "Execute the rollback (default: dry run)")
	rollbackCmd.Flags().String("repo", ".", "Path to the target repository")
	rollbackWriteDAGCmd.Flags().Int("phase", -1, "Phase number to write a rollback DAG for")
	rollbackWriteDAGCmd.Flags().String("repo", ".", "Path to the target repository")
	rollbackCmd.AddCommand(rollbackWriteDAGCmd)
	rootCmd.AddCommand(rollbackCmd)
}
