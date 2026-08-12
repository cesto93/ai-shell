package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"ai-shell/config"
	"ai-shell/llm"

	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List all available agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAgents()
	},
}

func init() {
	rootCmd.AddCommand(agentsCmd)
}

func runAgents() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	initLogger(cfg)

	defs := llm.GetAgentDefs()
	if len(defs) == 0 {
		fmt.Println("No agents found.")
		return nil
	}

	sorted := make([]llm.AgentDef, len(defs))
	copy(sorted, defs)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "\tAGENT\tDESCRIPTION\tTOOLS")
	fmt.Fprintln(w, "\t-----\t-----------\t-----")
	for _, def := range sorted {
		marker := ""
		if def.Name == cfg.Agent {
			marker = "*"
		}
		var toolNames []string
		for name, enabled := range def.Tools {
			if enabled {
				toolNames = append(toolNames, name)
			}
		}
		sort.Strings(toolNames)
		tools := strings.Join(toolNames, ", ")
		if tools == "" {
			tools = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", marker, def.Name, def.Description, tools)
	}
	w.Flush()

	return nil
}
