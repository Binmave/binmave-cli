package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/Binmave/binmave-cli/internal/api"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage agents",
	Long:  `List and manage Binmave agents deployed on endpoints.`,
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	Long:  `Display a list of all registered agents with their status.`,
	RunE:  runAgentsList,
}

var agentsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show agent statistics",
	Long:  `Display statistics about agents including online/offline counts.`,
	RunE:  runAgentsStats,
}

func init() {
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsStatsCmd)

	// Make 'agents' without subcommand run 'agents list'
	agentsCmd.RunE = runAgentsList
}

func runAgentsList(cmd *cobra.Command, args []string) error {
	client, err := api.NewClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	agents, err := client.ListAgents(ctx)
	if err != nil {
		return fmt.Errorf("failed to list agents: %w", err)
	}

	if IsJSONOutput() {
		return printJSON(agents)
	}

	if len(agents) == 0 {
		fmt.Println("No agents found.")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tNAME\tOS\tVERSION\tLAST SEEN\tIP")
	fmt.Fprintln(w, "------\t----\t--\t-------\t---------\t--")

	for _, agent := range agents {
		status := formatAgentStatus(agent.AgentStatus)
		lastSeen := formatTimeAgo(agent.LastConnectionEstablished)
		os := truncateString(agent.OperatingSystem, 25)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			status,
			agent.MachineName,
			os,
			agent.AgentVersion,
			lastSeen,
			agent.LastIP,
		)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d agents\n", len(agents))

	return nil
}

func runAgentsStats(cmd *cobra.Command, args []string) error {
	client, err := api.NewClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats, err := client.GetAgentStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get agent stats: %w", err)
	}

	if IsJSONOutput() {
		return printJSON(stats)
	}

	fmt.Println("Agent Statistics")
	fmt.Println("================")
	fmt.Printf("Total:    %d\n", stats.Total)
	fmt.Printf("Online:   %d ●\n", stats.Online)
	fmt.Printf("Offline:  %d ○\n", stats.Offline)
	fmt.Printf("Expired:  %d ✗\n", stats.Expired)
	fmt.Printf("Expiring: %d !\n", stats.Expiring)

	if len(stats.ByOperatingSystem) > 0 {
		fmt.Println("\nBy Operating System:")
		for os, count := range stats.ByOperatingSystem {
			fmt.Printf("  %s: %d\n", os, count)
		}
	}

	return nil
}

func formatAgentStatus(status string) string {
	switch strings.ToLower(status) {
	case "online":
		return "● Online"
	case "offline":
		return "○ Offline"
	case "expired":
		return "✗ Expired"
	case "expiring":
		return "! Expiring"
	default:
		return status
	}
}

func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "Never"
	}

	duration := time.Since(t)

	switch {
	case duration < time.Minute:
		return fmt.Sprintf("%ds ago", int(duration.Seconds()))
	case duration < time.Hour:
		return fmt.Sprintf("%dm ago", int(duration.Minutes()))
	case duration < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(duration.Hours()))
	default:
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func printJSON(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}
