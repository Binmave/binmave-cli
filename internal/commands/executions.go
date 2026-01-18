package commands

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/Binmave/binmave-cli/internal/api"
)

var (
	executionsLimit int
)

var executionsCmd = &cobra.Command{
	Use:   "executions",
	Short: "Manage script executions",
	Long:  `List and view script execution history and results.`,
}

var executionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent executions",
	Long:  `Display a list of recent script executions.`,
	RunE:  runExecutionsList,
}

var executionsShowCmd = &cobra.Command{
	Use:   "show <execution-id>",
	Short: "Show execution details",
	Long:  `Display detailed information about a specific execution.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runExecutionsShow,
}

var executionsResultsCmd = &cobra.Command{
	Use:   "results <execution-id>",
	Short: "Show execution results",
	Long:  `Display results from agents for a specific execution.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runExecutionsResults,
}

func init() {
	executionsCmd.AddCommand(executionsListCmd)
	executionsCmd.AddCommand(executionsShowCmd)
	executionsCmd.AddCommand(executionsResultsCmd)

	executionsListCmd.Flags().IntVarP(&executionsLimit, "limit", "n", 20, "Number of executions to show")

	// Make 'executions' without subcommand run 'executions list'
	executionsCmd.RunE = runExecutionsList
}

func runExecutionsList(cmd *cobra.Command, args []string) error {
	client, err := api.NewClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	executions, err := client.ListRecentExecutions(ctx, executionsLimit)
	if err != nil {
		return fmt.Errorf("failed to list executions: %w", err)
	}

	if IsJSONOutput() {
		return printJSON(executions)
	}

	if len(executions) == 0 {
		fmt.Println("No executions found.")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tID\tSCRIPT\tPROGRESS\tCREATED\tBY")
	fmt.Fprintln(w, "------\t--\t------\t--------\t-------\t--")

	for _, exec := range executions {
		status := formatExecutionStatus(exec.State, exec.Errors)
		progress := fmt.Sprintf("%d/%d", exec.Received, exec.Expected)
		if exec.Errors > 0 {
			progress += fmt.Sprintf(" (%d err)", exec.Errors)
		}
		created := formatTimeAgo(exec.Created)
		scriptName := truncateString(exec.ScriptName, 25)
		shortID := exec.ExecutionID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			status,
			shortID,
			scriptName,
			progress,
			created,
			exec.CreatedBy,
		)
	}
	w.Flush()

	fmt.Printf("\nShowing %d most recent executions\n", len(executions))

	return nil
}

func runExecutionsShow(cmd *cobra.Command, args []string) error {
	executionID := args[0]

	client, err := api.NewClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	execution, err := client.GetExecution(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	status, err := client.GetExecutionStatus(ctx, executionID)
	if err != nil {
		return fmt.Errorf("failed to get execution status: %w", err)
	}

	if IsJSONOutput() {
		return printJSON(map[string]interface{}{
			"execution": execution,
			"status":    status,
		})
	}

	fmt.Printf("Execution Details\n")
	fmt.Printf("=================\n")
	fmt.Printf("ID:        %s\n", execution.ExecutionID)
	fmt.Printf("Script:    %s (ID: %d)\n", execution.ScriptName, execution.ScriptID)
	fmt.Printf("Created:   %s\n", execution.Created.Format("2006-01-02 15:04:05"))
	fmt.Printf("By:        %s\n", execution.CreatedBy)
	fmt.Printf("Timeout:   %s\n", execution.ScriptTimeout)

	fmt.Printf("\nStatus\n")
	fmt.Printf("------\n")
	fmt.Printf("State:     %s\n", formatExecutionStatus(status.State, status.Errors))
	fmt.Printf("Progress:  %d/%d agents\n", status.Received, status.Expected)
	if status.Errors > 0 {
		fmt.Printf("Errors:    %d\n", status.Errors)
	}

	// Calculate progress bar
	if status.Expected > 0 {
		pct := float64(status.Received) / float64(status.Expected) * 100
		bar := createProgressBar(int(pct), 30)
		fmt.Printf("           %s %.0f%%\n", bar, pct)
	}

	if len(execution.Inputs) > 0 {
		fmt.Printf("\nInputs\n")
		fmt.Printf("------\n")
		for _, input := range execution.Inputs {
			fmt.Printf("  %s: %s\n", input.Key, input.Value)
		}
	}

	fmt.Printf("\nUse 'binmave executions results %s' to view agent results\n", executionID)
	fmt.Printf("Use 'binmave watch %s' for live monitoring\n", executionID)

	return nil
}

func runExecutionsResults(cmd *cobra.Command, args []string) error {
	executionID := args[0]

	client, err := api.NewClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := client.GetExecutionResults(ctx, executionID, 1, 100)
	if err != nil {
		return fmt.Errorf("failed to get execution results: %w", err)
	}

	if IsJSONOutput() {
		return printJSON(results)
	}

	if len(results.Results) == 0 {
		fmt.Println("No results yet.")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tAGENT\tTIME\tRESULT")
	fmt.Fprintln(w, "------\t-----\t----\t------")

	for _, result := range results.Results {
		status := "✓"
		if result.HasError {
			status = "✗"
		}
		execTime := fmt.Sprintf("%.1fs", float64(result.ExecutionTimeSeconds))

		// Parse result preview
		resultPreview := truncateString(result.AnswerJSON, 40)
		if result.HasError && result.RawStdError != "" {
			resultPreview = truncateString(result.RawStdError, 40)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			status,
			result.AgentName,
			execTime,
			resultPreview,
		)
	}
	w.Flush()

	fmt.Printf("\nShowing %d of %d results\n", len(results.Results), results.TotalCount)

	return nil
}

func formatExecutionStatus(state string, errors int) string {
	switch state {
	case "Completed":
		if errors > 0 {
			return "⚠ Partial"
		}
		return "✓ Done"
	case "Running":
		return "⟳ Running"
	case "Pending":
		return "○ Pending"
	case "Failed":
		return "✗ Failed"
	default:
		return state
	}
}

func createProgressBar(pct int, width int) string {
	filled := pct * width / 100
	if filled > width {
		filled = width
	}
	empty := width - filled
	return "[" + repeat("█", filled) + repeat("░", empty) + "]"
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
