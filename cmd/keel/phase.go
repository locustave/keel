package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"keel/internal/session"
	"keel/internal/snapshot"
)

var phaseCmd = &cobra.Command{
	Use:   "phase",
	Short: "Emit phase lifecycle events to the active session ledger",
}

var phaseStartCmd = &cobra.Command{
	Use:   "start <N>",
	Short: "Emit a phase_started event for phase N",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase, err := strconv.Atoi(args[0])
		if err != nil || phase < 0 {
			return fmt.Errorf("phase must be a non-negative integer")
		}
		repo, _ := cmd.Flags().GetString("repo")

		// Capture pre-phase file manifest for rollback.
		manifest, err := snapshot.Capture(repo, phase, "pre")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not capture pre-phase manifest: %v\n", err)
		} else {
			if err := snapshot.Write(repo, manifest); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not write pre-phase manifest: %v\n", err)
			} else {
				fmt.Fprintf(os.Stdout, "Manifest: .agent/snapshots/phase_%d.pre.manifest.json (%d files)\n", phase, len(manifest.Files))
			}
		}

		// Stash all files for git-free rollback restore.
		if manifest != nil {
			if err := snapshot.Stash(repo, manifest); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not stash files for rollback: %v\n", err)
			} else {
				fmt.Fprintf(os.Stdout, "Stash: .agent/snapshots/stash_%d/ (%d files)\n", phase, len(manifest.Files))
			}
		}

		m := session.NewManager(repo)
		sess := m.EnsureSession(session.CommandUser{UserID: "agent", Name: "agent"}, session.UnknownImplementer)
		if sess == nil {
			return nil // tracking disabled — fail open
		}
		eid := sess.WriteEvent(
			session.EvtPhaseStarted,
			session.HarnessCLIActor,
			map[string]interface{}{"phase": phase},
			session.WithPhase(fmt.Sprintf("phase_%d", phase)),
		)
		if eid != "" {
			fmt.Fprintln(os.Stdout, eid)
		}
		return nil
	},
}

var phaseCompleteCmd = &cobra.Command{
	Use:   "complete <N>",
	Short: "Emit a phase_completed event for phase N",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase, err := strconv.Atoi(args[0])
		if err != nil || phase < 0 {
			return fmt.Errorf("phase must be a non-negative integer")
		}
		repo, _ := cmd.Flags().GetString("repo")

		m := session.NewManager(repo)
		sess := m.EnsureSession(session.CommandUser{UserID: "agent", Name: "agent"}, session.UnknownImplementer)
		if sess == nil {
			return nil
		}
		eid := sess.WriteEvent(
			session.EvtPhaseCompleted,
			session.HarnessCLIActor,
			map[string]interface{}{"phase": phase},
			session.WithPhase(fmt.Sprintf("phase_%d", phase)),
		)
		if eid != "" {
			fmt.Fprintln(os.Stdout, eid)
		}
		return nil
	},
}

var phaseFailedCmd = &cobra.Command{
	Use:   "failed <N>",
	Short: "Emit a phase_failed event for phase N",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase, err := strconv.Atoi(args[0])
		if err != nil || phase < 0 {
			return fmt.Errorf("phase must be a non-negative integer")
		}
		repo, _ := cmd.Flags().GetString("repo")
		reason, _ := cmd.Flags().GetString("reason")

		m := session.NewManager(repo)
		sess := m.EnsureSession(session.CommandUser{UserID: "agent", Name: "agent"}, session.UnknownImplementer)
		if sess == nil {
			return nil
		}
		eid := sess.WriteEvent(
			session.EvtPhaseFailed,
			session.HarnessCLIActor,
			map[string]interface{}{"phase": phase, "reason": reason},
			session.WithPhase(fmt.Sprintf("phase_%d", phase)),
		)
		if eid != "" {
			fmt.Fprintln(os.Stdout, eid)
		}
		return nil
	},
}

func init() {
	for _, sub := range []*cobra.Command{phaseStartCmd, phaseCompleteCmd, phaseFailedCmd} {
		sub.Flags().String("repo", ".", "Path to repository root")
	}
	phaseFailedCmd.Flags().String("reason", "", "Reason for failure (optional)")
	phaseCmd.AddCommand(phaseStartCmd, phaseCompleteCmd, phaseFailedCmd)
	rootCmd.AddCommand(phaseCmd)
}
