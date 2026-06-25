package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"keel/internal/gate"
	"keel/internal/session"
)

var phaseCloseCmd = &cobra.Command{
	Use:   "close <N>",
	Short: "Write all phase-closing artifacts (gate, rollback DAG, ledger, audit)",
	Long: `Close phase N by writing every required phase-closing artifact:

  1. .agent/phase_gates/phase_N.gate.json   — canonical gate record
  2. .agent/snapshots/phase_N.rollback.json — rollback DAG
  3. docs/build-ledger/phase_N_build.md     — human-readable build ledger
  4. docs/audit/phase_N.log                 — human-readable audit log
  5. .agent/audit.jsonl                     — append audit event
  6. .agent/run_log.jsonl                   — append run log event
  7. Session event (phase_completed)        — session ledger

Exit criteria can be passed via --criteria-json (JSON array) or will default
to an empty list. The gate status is derived: if all criteria pass, the gate
is "passed"; if any criterion fails, the gate is "failed".

Examples:
  keel phase close 1 --summary "Built core API"
  keel phase close 2 --agent claude --model opus --criteria-json '[{"criterion":"tests pass","passed":true}]'`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		phase, err := strconv.Atoi(args[0])
		if err != nil || phase < 0 {
			return fmt.Errorf("phase must be a non-negative integer")
		}

		repo, _ := cmd.Flags().GetString("repo")
		agent, _ := cmd.Flags().GetString("agent")
		model, _ := cmd.Flags().GetString("model")
		summary, _ := cmd.Flags().GetString("summary")
		gitBefore, _ := cmd.Flags().GetString("git-sha-before")
		gitAfter, _ := cmd.Flags().GetString("git-sha-after")
		criteriaJSON, _ := cmd.Flags().GetString("criteria-json")

		var criteria []gate.ExitCriterionResult
		if criteriaJSON != "" {
			if err := json.Unmarshal([]byte(criteriaJSON), &criteria); err != nil {
				return fmt.Errorf("invalid --criteria-json: %w", err)
			}
		}

		in := gate.CloseInput{
			Phase:               phase,
			RepoPath:            repo,
			Agent:               agent,
			Model:               model,
			Summary:             summary,
			GitSHABefore:        gitBefore,
			GitSHAAfter:         gitAfter,
			ExitCriteriaResults: criteria,
		}

		result, err := gate.Close(in)
		if err != nil {
			return err
		}

		// Emit session event.
		m := session.NewManager(repo)
		sess := m.EnsureSession(
			session.CommandUser{UserID: "agent", Name: "agent"},
			session.UnknownImplementer,
		)
		if sess != nil {
			sess.WriteEvent(
				session.EvtPhaseCompleted,
				session.HarnessCLIActor,
				map[string]interface{}{
					"phase":     phase,
					"gate_path": result.GatePath,
				},
				session.WithPhase(fmt.Sprintf("phase_%d", phase)),
			)
		}

		fmt.Fprintf(os.Stdout, "Phase %d closed.\n", phase)
		fmt.Fprintf(os.Stdout, "  Gate:     %s\n", result.GatePath)
		if result.RollbackPath != "" {
			fmt.Fprintf(os.Stdout, "  Rollback: %s\n", result.RollbackPath)
		}
		fmt.Fprintf(os.Stdout, "  Ledger:   %s\n", result.LedgerPath)
		fmt.Fprintf(os.Stdout, "  Audit:    %s\n", result.AuditLogPath)
		return nil
	},
}

func init() {
	phaseCloseCmd.Flags().String("repo", ".", "Path to repository root")
	phaseCloseCmd.Flags().String("agent", "", "Agent identifier (e.g. claude)")
	phaseCloseCmd.Flags().String("model", "", "Model used (e.g. opus)")
	phaseCloseCmd.Flags().String("summary", "", "Brief summary of what was built")
	phaseCloseCmd.Flags().String("git-sha-before", "", "Git SHA before phase started (auto-detected if omitted)")
	phaseCloseCmd.Flags().String("git-sha-after", "", "Git SHA after phase completed (auto-detected if omitted)")
	phaseCloseCmd.Flags().String("criteria-json", "", `Exit criteria as JSON array: [{"criterion":"...","passed":true}]`)
	phaseCmd.AddCommand(phaseCloseCmd)
}
