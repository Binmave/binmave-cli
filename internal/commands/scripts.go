package commands

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/Binmave/binmave-cli/internal/api"
)

var scriptsCmd = &cobra.Command{
	Use:   "scripts",
	Short: "Manage scripts",
	Long:  `List and manage Binmave scripts for execution on endpoints.`,
}

var scriptsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all scripts",
	Long:  `Display a list of all available scripts.`,
	RunE:  runScriptsList,
}

var scriptsShowCmd = &cobra.Command{
	Use:   "show <script-id>",
	Short: "Show script details",
	Long:  `Display detailed information about a specific script.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runScriptsShow,
}

func init() {
	scriptsCmd.AddCommand(scriptsListCmd)
	scriptsCmd.AddCommand(scriptsShowCmd)

	// Make 'scripts' without subcommand run 'scripts list'
	scriptsCmd.RunE = runScriptsList
}

func runScriptsList(cmd *cobra.Command, args []string) error {
	client, err := api.NewClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scripts, err := client.ListScripts(ctx)
	if err != nil {
		return fmt.Errorf("failed to list scripts: %w", err)
	}

	if IsJSONOutput() {
		return printJSON(scripts)
	}

	if len(scripts) == 0 {
		fmt.Println("No scripts found.")
		return nil
	}

	// Print table
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tTYPE\tVERSION\tTAGS\tTIMEOUT")
	fmt.Fprintln(w, "--\t----\t----\t-------\t----\t-------")

	for _, script := range scripts {
		tags := strings.Join(script.Tags, ", ")
		if len(tags) > 30 {
			tags = tags[:27] + "..."
		}
		name := truncateString(script.Name, 40)

		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s\t%s\n",
			script.ScriptID,
			name,
			script.ScriptType,
			script.Version,
			tags,
			script.ScriptTimeout,
		)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d scripts\n", len(scripts))

	return nil
}

func runScriptsShow(cmd *cobra.Command, args []string) error {
	var scriptID int
	if _, err := fmt.Sscanf(args[0], "%d", &scriptID); err != nil {
		return fmt.Errorf("invalid script ID: %s", args[0])
	}

	client, err := api.NewClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	script, err := client.GetScript(ctx, scriptID)
	if err != nil {
		return fmt.Errorf("failed to get script: %w", err)
	}

	if IsJSONOutput() {
		return printJSON(script)
	}

	fmt.Printf("Script Details\n")
	fmt.Printf("==============\n")
	fmt.Printf("ID:          %d\n", script.ScriptID)
	fmt.Printf("Name:        %s\n", script.Name)
	fmt.Printf("Description: %s\n", script.Description)
	fmt.Printf("Type:        %s\n", script.ScriptType)
	fmt.Printf("Output:      %s\n", script.OutputType)
	fmt.Printf("Version:     %d\n", script.Version)
	fmt.Printf("Timeout:     %s\n", script.ScriptTimeout)
	fmt.Printf("Repository:  %s (%s)\n", script.RepoName, script.RepoType)

	if len(script.Tags) > 0 {
		fmt.Printf("Tags:        %s\n", strings.Join(script.Tags, ", "))
	}

	return nil
}
