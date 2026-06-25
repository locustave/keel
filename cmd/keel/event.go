package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"keel/internal/session"
)

var eventCmd = &cobra.Command{
	Use:   "event",
	Short: "Manage keel events",
}

var eventAppendCmd = &cobra.Command{
	Use:   "append",
	Short: "Append an event to the active session ledger (used by hook scripts)",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		evtType, _ := cmd.Flags().GetString("type")
		actorType, _ := cmd.Flags().GetString("actor-type")
		actorName, _ := cmd.Flags().GetString("actor-name")
		phaseID, _ := cmd.Flags().GetString("phase-id")
		toolName, _ := cmd.Flags().GetString("tool-name")
		metadataStr, _ := cmd.Flags().GetString("metadata")
		tokenUsageStr, _ := cmd.Flags().GetString("token-usage")

		if evtType == "" {
			return fmt.Errorf("--type is required")
		}

		m := session.NewManager(repo)
		if !m.IsEnabled() {
			// Fail silently — hooks must not block agent commands.
			return nil
		}

		sess := m.EnsureSession(session.CommandUser{UserID: "hook", Name: "hook"}, session.UnknownImplementer)
		if sess == nil {
			return nil
		}

		actor := session.Actor{
			ActorType: actorType,
			ActorName: actorName,
			ActorID:   actorType + ":" + actorName,
		}

		var opts []session.EventOpt
		if phaseID != "" {
			opts = append(opts, session.WithPhase(phaseID))
		}
		if toolName != "" {
			opts = append(opts, session.WithTool(toolName))
		}
		if tokenUsageStr != "" {
			var tu session.TokenUsage
			if err := json.Unmarshal([]byte(tokenUsageStr), &tu); err == nil && tu.TotalTokens > 0 {
				opts = append(opts, session.WithTokenUsage(tu))
			}
		}

		var metadata interface{}
		if metadataStr != "" {
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(metadataStr), &m); err == nil {
				metadata = m
			} else {
				metadata = metadataStr
			}
		}

		eid := sess.WriteEvent(evtType, actor, metadata, opts...)
		if eid != "" {
			fmt.Fprintln(os.Stdout, eid)
		}
		return nil
	},
}

func init() {
	eventAppendCmd.Flags().String("repo", ".", "Path to repository root")
	eventAppendCmd.Flags().String("type", "", "Event type (e.g. tool_call_completed)")
	eventAppendCmd.Flags().String("actor-type", "keel", "Actor type (keel|tool|hook|agent|human)")
	eventAppendCmd.Flags().String("actor-name", "keel-cli", "Actor name")
	eventAppendCmd.Flags().String("phase-id", "", "Current phase ID (optional)")
	eventAppendCmd.Flags().String("tool-name", "", "Tool name (optional)")
	eventAppendCmd.Flags().String("metadata", "", "JSON metadata string (optional)")
	eventAppendCmd.Flags().String("token-usage", "", "JSON token usage (optional, e.g. {\"input_tokens\":100,\"output_tokens\":50,\"total_tokens\":150})")

	eventCmd.AddCommand(eventAppendCmd)
	rootCmd.AddCommand(eventCmd)
}
