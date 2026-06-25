package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"keel/internal/session"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage keel sessions",
}

var sessionStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new tracking session",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		name, _ := cmd.Flags().GetString("name")
		email, _ := cmd.Flags().GetString("email")
		implName, _ := cmd.Flags().GetString("implementer")
		implType, _ := cmd.Flags().GetString("implementer-type")

		m := session.NewManager(repo)
		if !m.IsEnabled() {
			fmt.Fprintln(os.Stderr, "session tracking is disabled")
			os.Exit(1)
		}

		owner := session.ResolveIdentity(session.ResolveOptions{Name: name, Email: email})
		impl := session.UnknownImplementer
		if implName != "" {
			impl = session.MakeImplementer(implName, implType)
		}

		sess, err := m.StartSession(owner, owner.ToCommandUser(), impl)
		if err != nil {
			return fmt.Errorf("start session: %w", err)
		}
		fmt.Fprintf(os.Stdout, "Session started: %s\n", sess.ID)
		fmt.Fprintf(os.Stdout, "Directory: %s\n", sess.Dir)
		return nil
	},
}

var sessionCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Print the current active session ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		m := session.NewManager(repo)
		id := m.CurrentSessionID()
		if id == "" {
			fmt.Fprintln(os.Stderr, "no active session")
			os.Exit(1)
		}
		fmt.Println(id)
		return nil
	},
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions (newest first)",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		m := session.NewManager(repo)
		ids, err := m.ListSessions()
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}
		current := m.CurrentSessionID()
		for _, id := range ids {
			if id == current {
				fmt.Printf("* %s (active)\n", id)
			} else {
				fmt.Printf("  %s\n", id)
			}
		}
		return nil
	},
}

var sessionEndCmd = &cobra.Command{
	Use:   "end [session-id]",
	Short: "End the active session (or a specific session by ID)",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		id := ""
		if len(args) > 0 {
			id = args[0]
		}
		m := session.NewManager(repo)
		if err := m.EndSession(id); err != nil {
			return fmt.Errorf("end session: %w", err)
		}
		fmt.Fprintln(os.Stdout, "Session ended.")
		return nil
	},
}

func init() {
	for _, sub := range []*cobra.Command{sessionStartCmd, sessionCurrentCmd, sessionListCmd, sessionEndCmd} {
		sub.Flags().String("repo", ".", "Path to repository root")
	}
	sessionStartCmd.Flags().String("name", "", "Identity name (overrides auto-detection)")
	sessionStartCmd.Flags().String("email", "", "Identity email (overrides auto-detection)")
	sessionStartCmd.Flags().String("implementer", "", "Implementer name (e.g. claude-code)")
	sessionStartCmd.Flags().String("implementer-type", "agent", "Implementer type (agent|human|automation)")

	sessionCmd.AddCommand(sessionStartCmd, sessionCurrentCmd, sessionListCmd, sessionEndCmd)
	rootCmd.AddCommand(sessionCmd)
}
