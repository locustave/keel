package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"keel/internal/banner"
)

// version is set at build time via -ldflags "-X main.version=v0.1.0".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "keel",
	Short: "Keel CLI — bootstrap and govern AI agent phase execution",
	Run: func(cmd *cobra.Command, args []string) {
		// Show banner + help when run with no subcommand.
		banner.Print(version)
		cmd.Help()
	},
}

// bannerCommands lists commands that show the logo banner.
// All other commands run quietly.
var bannerCommands = map[string]bool{
	"init":        true,
	"plan-phases": true,
	"help":        true,
}

func init() {
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if bannerCommands[cmd.Name()] {
			banner.Print(version)
		}
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
