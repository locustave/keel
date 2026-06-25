package main

import (
	"os"

	"github.com/spf13/cobra"

	"keel/internal/planner"
)

var planPhasesCmd = &cobra.Command{
	Use:   "plan-phases",
	Short: "Derive a phase plan from the TDD Deliverables DAG",
	RunE: func(cmd *cobra.Command, args []string) error {
		tdd, _ := cmd.Flags().GetString("tdd")
		confirm, _ := cmd.Flags().GetBool("confirm")
		startPhase, _ := cmd.Flags().GetInt("start-phase")
		existingGates, _ := cmd.Flags().GetString("existing-gates")
		featureSlug, _ := cmd.Flags().GetString("feature-slug")
		title, _ := cmd.Flags().GetString("title")

		code := planner.Run(planner.Options{
			TDDPath:       tdd,
			Confirm:       confirm,
			StartPhase:    startPhase,
			ExistingGates: existingGates,
			FeatureSlug:   featureSlug,
			Title:         title,
			Out:           os.Stdout,
		})
		if code != 0 {
			os.Exit(code)
		}
		return nil
	},
}

func init() {
	planPhasesCmd.Flags().String("tdd", "docs/TDD.md", "Path to the TDD file containing a ## Deliverables section")
	planPhasesCmd.Flags().Bool("confirm", false, "Write BUILD_MANIFEST.yaml (default: dry run)")
	planPhasesCmd.Flags().Int("start-phase", 0, "Starting phase number")
	planPhasesCmd.Flags().String("existing-gates", "", "Path to phase gates directory")
	planPhasesCmd.Flags().String("feature-slug", "", "Feature slug (writes to docs/features/{slug}/BUILD_MANIFEST.yaml)")
	planPhasesCmd.Flags().String("title", "", "Title for the generated manifest")
	rootCmd.AddCommand(planPhasesCmd)
}
