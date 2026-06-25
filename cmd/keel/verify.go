package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"keel/internal/verify"
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify keel consistency in the target repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		phase, _ := cmd.Flags().GetInt("phase")
		if err := verify.Run(repo, phase); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("Repository verification passed.")
		return nil
	},
}

func init() {
	verifyCmd.Flags().String("repo", ".", "Path to the target repository (default: current directory)")
	verifyCmd.Flags().Int("phase", -1, "Additionally verify the gate for a specific phase")
	rootCmd.AddCommand(verifyCmd)
}
