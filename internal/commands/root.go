package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/Binmave/binmave-cli/internal/config"
)

var (
	// Version is set at build time
	Version = "dev"

	rootCmd = &cobra.Command{
		Use:   "binmave",
		Short: "Binmave CLI - Endpoint Management and Security",
		Long: `Binmave CLI provides a command-line interface for managing
endpoints, executing scripts, and monitoring your security infrastructure.

Use 'binmave login' to authenticate, then explore agents, scripts, and executions.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip config init for completion commands
			if cmd.Name() == "completion" || cmd.Name() == "__complete" {
				return nil
			}
			return config.Init()
		},
	}

	// Global flags
	serverFlag string
	jsonOutput bool
)

func init() {
	rootCmd.PersistentFlags().StringVar(&serverFlag, "server", "", "Override the server URL")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")

	// Add subcommands
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)
	rootCmd.AddCommand(agentsCmd)
	rootCmd.AddCommand(scriptsCmd)
	rootCmd.AddCommand(executionsCmd)
	rootCmd.AddCommand(watchCmd)
	rootCmd.AddCommand(resultsCmd)
	rootCmd.AddCommand(compareCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}

// IsJSONOutput returns true if JSON output is requested
func IsJSONOutput() bool {
	return jsonOutput
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("binmave version %s\n", Version)
	},
}
