package main

import (
	"os"

	"github.com/spf13/cobra"

	"keel/internal/merge"
)

var mergeFeatureCmd = &cobra.Command{
	Use:   "merge-feature <slug>",
	Short: "Merge a feature's phase plan into the project BUILD_MANIFEST.yaml",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		confirm, _ := cmd.Flags().GetBool("confirm")
		code := merge.Run(merge.Options{
			Slug:    args[0],
			Confirm: confirm,
			Out:     os.Stdout,
			Err:     os.Stderr,
		})
		if code != 0 {
			os.Exit(code)
		}
		return nil
	},
}

func init() {
	mergeFeatureCmd.Flags().Bool("confirm", false, "Write changes (default: dry run)")
	rootCmd.AddCommand(mergeFeatureCmd)
}
