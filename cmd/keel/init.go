package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"keel/internal/bootstrap"
	"keel/internal/detect"
	"keel/internal/prd"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap a keel governance structure into the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		forceTemplate, _ := cmd.Flags().GetBool("force-template")
		tracking, _ := cmd.Flags().GetString("tracking")
		skipDetect, _ := cmd.Flags().GetBool("skip-detect")
		prdPath, _ := cmd.Flags().GetString("prd")

		// --- PRD discovery / creation ---
		if prdPath != "" {
			// Explicit --prd flag: copy it to docs/PRD.md.
			if err := prd.CopyToDocsPRD(repo, prdPath); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not copy PRD to docs/PRD.md: %v\n", err)
			}
		} else {
			found := prd.FindPRDs(repo)
			if len(found) == 1 {
				fmt.Fprintf(os.Stdout, "\n  Using PRD: %s\n", found[0])
				if err := prd.CopyToDocsPRD(repo, found[0]); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: could not copy PRD to docs/PRD.md: %v\n", err)
				}
			} else if len(found) > 1 {
				selected := prd.PromptSelect(found, os.Stdin, os.Stdout)
				if selected != "" {
					if err := prd.CopyToDocsPRD(repo, selected); err != nil {
						fmt.Fprintf(os.Stderr, "Warning: could not copy PRD to docs/PRD.md: %v\n", err)
					}
				}
			} else {
				// No PRD found — prompt user to create one.
				if _, err := prd.PromptCreate(repo, os.Stdin, os.Stdout); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
				}
			}
		}

		// Detect and confirm tech stack before writing any files.
		var stack detect.Stack
		if !skipDetect {
			stack = detect.Scan(repo)
			stack = detect.Confirm(stack, os.Stdin, os.Stdout)
		}

		if err := bootstrap.Run(repo, bootstrap.Options{
			ForceTemplate: forceTemplate,
			Tracking:      tracking,
			Out:           os.Stdout,
		}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// Write confirmed stack to agent-rules.md if anything was detected.
		if !stack.IsEmpty() {
			if err := detect.ApplyToAgentRules(repo, stack); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not update agent-rules.md tech stack: %v\n", err)
			}
		}

		return nil
	},
}

func init() {
	initCmd.Flags().String("repo", ".", "Path to the target repository (default: current directory)")
	initCmd.Flags().Bool("force-template", false, "Overwrite existing keel template files")
	initCmd.Flags().String("tracking", "local", "Session tracking mode: local or off")
	initCmd.Flags().Bool("skip-detect", false, "Skip tech stack detection and confirmation")
	initCmd.Flags().String("prd", "", "Path to PRD file (searches automatically if not provided)")
	rootCmd.AddCommand(initCmd)
}
