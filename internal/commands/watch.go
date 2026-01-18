package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/Binmave/binmave-cli/internal/api"
)

var (
	watchInterval time.Duration
)

var watchCmd = &cobra.Command{
	Use:   "watch <execution-id>",
	Short: "Watch execution progress in real-time",
	Long: `Monitor a script execution in real-time, showing progress updates
as agents complete their work.

Press Ctrl+C to stop watching (the execution continues in the background).`,
	Args: cobra.ExactArgs(1),
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().DurationVarP(&watchInterval, "interval", "i", 2*time.Second, "Polling interval")
}

func runWatch(cmd *cobra.Command, args []string) error {
	executionID := args[0]

	client, err := api.NewClient()
	if err != nil {
		return err
	}

	// Get initial execution details
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	execution, err := client.GetExecution(ctx, executionID)
	cancel()
	if err != nil {
		return fmt.Errorf("failed to get execution: %w", err)
	}

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	fmt.Printf("Watching execution %s\n", executionID)
	fmt.Printf("Script: %s\n", execution.ScriptName)
	fmt.Printf("Press Ctrl+C to stop watching\n\n")

	ticker := time.NewTicker(watchInterval)
	defer ticker.Stop()

	lastReceived := -1
	startTime := time.Now()

	for {
		select {
		case <-sigChan:
			fmt.Println("\nStopped watching. Execution continues in background.")
			return nil
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			status, err := client.GetExecutionStatus(ctx, executionID)
			cancel()

			if err != nil {
				fmt.Printf("\r⚠ Error getting status: %v", err)
				continue
			}

			// Only redraw if something changed
			if status.Received != lastReceived {
				lastReceived = status.Received
				printWatchStatus(execution.ScriptName, status, startTime)
			}

			// Check if complete
			if status.State == "Completed" || status.State == "Failed" {
				fmt.Println()
				printFinalSummary(executionID, status)
				return nil
			}
		}
	}
}

func printWatchStatus(scriptName string, status *api.ExecutionStatus, startTime time.Time) {
	// Clear line and move cursor to beginning
	fmt.Print("\033[2K\r")

	elapsed := time.Since(startTime).Round(time.Second)
	pct := 0
	if status.Expected > 0 {
		pct = status.Received * 100 / status.Expected
	}

	// Progress bar
	bar := createProgressBar(pct, 25)

	statusIcon := "⟳"
	switch status.State {
	case "Completed":
		statusIcon = "✓"
	case "Failed":
		statusIcon = "✗"
	case "Pending":
		statusIcon = "○"
	}

	errStr := ""
	if status.Errors > 0 {
		errStr = fmt.Sprintf(" (%d errors)", status.Errors)
	}

	fmt.Printf("%s %s %d/%d %s %d%% [%s]%s",
		statusIcon,
		bar,
		status.Received,
		status.Expected,
		status.State,
		pct,
		elapsed,
		errStr,
	)
}

func printFinalSummary(executionID string, status *api.ExecutionStatus) {
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	if status.State == "Completed" && status.Errors == 0 {
		fmt.Println("✓ Execution completed successfully!")
	} else if status.State == "Completed" && status.Errors > 0 {
		fmt.Printf("⚠ Execution completed with %d errors\n", status.Errors)
	} else {
		fmt.Printf("✗ Execution failed (%d/%d completed)\n", status.Received, status.Expected)
	}

	fmt.Printf("\nResults: %d/%d agents responded\n", status.Received, status.Expected)
	if status.Errors > 0 {
		fmt.Printf("Errors:  %d agents had errors\n", status.Errors)
	}

	fmt.Printf("\nView results: binmave executions results %s\n", executionID)
}
