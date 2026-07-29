package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"ai-shell/config"

	"github.com/spf13/cobra"
)

var commandsCmd = &cobra.Command{
	Use:   "commands",
	Short: "List all available custom commands",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCommands()
	},
}

func init() {
	rootCmd.AddCommand(commandsCmd)
}

func runCommands() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".ai-shell", "commands")
	cmds, err := config.LoadCommandsFromDir(dir)
	if err != nil {
		return fmt.Errorf("failed to load commands: %w", err)
	}

	if len(cmds) == 0 {
		fmt.Println("No commands found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "COMMAND\tDESCRIPTION")
	fmt.Fprintln(w, "-------\t-----------")
	for _, c := range cmds {
		desc := c.Description
		if desc == "" {
			desc = "-"
		}
		fmt.Fprintf(w, "%s\t%s\n", c.Name, desc)
	}
	w.Flush()

	return nil
}
