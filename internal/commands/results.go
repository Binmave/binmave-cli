package commands

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/Binmave/binmave-cli/internal/api"
	"github.com/Binmave/binmave-cli/internal/ui/models"
)

var resultsCmd = &cobra.Command{
	Use:   "results <execution-id>",
	Short: "View execution results in interactive TUI",
	Long: `View execution results with multiple view modes.

The interactive TUI provides three view modes:
  - Table: Flat list of results per agent
  - Tree: Hierarchical data grouped by agent
  - Aggregated: Merged tree with counts and anomaly detection

Keyboard shortcuts:
  Tab        Switch between Results/Errors tabs
  1/2/3      Switch view mode (Table/Tree/Aggregated)
  Up/Down    Navigate
  Enter      Expand/collapse nodes (tree views)
  e          Expand all nodes
  c          Collapse all nodes
  a          Toggle "anomalies only" (aggregated view)
  q          Quit

Examples:
  # View results for an execution
  binmave results a1b2c3d4-e5f6-7890-abcd-ef1234567890

  # Start in tree view
  binmave results a1b2c3d4 --view tree

  # Start in aggregated view with anomalies filter
  binmave results a1b2c3d4 --view aggregated --anomalies`,
	Args: cobra.ExactArgs(1),
	RunE: runResults,
}

var (
	resultsViewMode      string
	resultsAnomaliesOnly bool
)

func init() {
	resultsCmd.Flags().StringVarP(&resultsViewMode, "view", "v", "table", "Initial view mode: table, tree, or aggregated")
	resultsCmd.Flags().BoolVarP(&resultsAnomaliesOnly, "anomalies", "a", false, "Show only anomalies (aggregated view)")
}

func runResults(cmd *cobra.Command, args []string) error {
	executionID := args[0]

	// Create API client
	client, err := api.NewClient()
	if err != nil {
		return err
	}

	// Validate execution exists
	_, err = client.GetExecution(cmd.Context(), executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	// Create TUI model
	model := models.NewResultsModel(executionID, client)

	// Set initial view mode
	switch resultsViewMode {
	case "tree":
		model.SetInitialViewMode(models.TreeView)
	case "aggregated", "agg":
		model.SetInitialViewMode(models.AggregatedView)
		if resultsAnomaliesOnly {
			model.SetAnomaliesOnly(true)
		}
	}

	// Run TUI
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
