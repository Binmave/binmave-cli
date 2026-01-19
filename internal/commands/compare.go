package commands

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/Binmave/binmave-cli/internal/api"
	"github.com/Binmave/binmave-cli/internal/ui/models"
)

var compareCmd = &cobra.Command{
	Use:   "compare <execution-id> --baseline <baseline-id>",
	Short: "Compare execution results against a baseline",
	Long: `Compare the results of an execution against a baseline execution.

This is useful for detecting changes in your environment over time:
  - New items that appeared (potential threats or misconfigurations)
  - Removed items that disappeared (potential cleanup or issues)
  - Modified items (changes in configuration or behavior)

Keyboard shortcuts:
  Up/Down    Navigate
  d          Show only differences
  a          Show all items
  q          Quit

Examples:
  # Compare current execution against a baseline
  binmave compare a1b2c3d4 --baseline b5c6d7e8

  # Using short flag
  binmave compare a1b2c3d4 -b b5c6d7e8`,
	Args: cobra.ExactArgs(1),
	RunE: runCompare,
}

var compareBaselineID string

func init() {
	compareCmd.Flags().StringVarP(&compareBaselineID, "baseline", "b", "", "Baseline execution ID to compare against (required)")
	compareCmd.MarkFlagRequired("baseline")
}

func runCompare(cmd *cobra.Command, args []string) error {
	executionID := args[0]

	if compareBaselineID == "" {
		return fmt.Errorf("baseline execution ID is required (--baseline)")
	}

	// Create API client
	client, err := api.NewClient()
	if err != nil {
		return err
	}

	// Validate both executions exist
	_, err = client.GetExecution(cmd.Context(), executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	_, err = client.GetExecution(cmd.Context(), compareBaselineID)
	if err != nil {
		return fmt.Errorf("failed to get baseline execution: %w", err)
	}

	// Create TUI model
	model := models.NewCompareModel(executionID, compareBaselineID, client)

	// Run TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
