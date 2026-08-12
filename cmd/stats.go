package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"ai-shell/config"
	"ai-shell/stats"

	"github.com/spf13/cobra"
)

var statsReset bool

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show token usage stats by model and provider",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		initLogger(cfg)

		if statsReset {
			if err := stats.Reset(); err != nil {
				return fmt.Errorf("failed to reset stats: %w", err)
			}
			fmt.Println("Usage stats cleared.")
			return nil
		}

		entries, err := stats.GetStats()
		if err != nil {
			return fmt.Errorf("failed to load stats: %w", err)
		}
		if len(entries) == 0 {
			fmt.Println("No usage stats recorded yet.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "\tPROVIDER\tMODEL\tCALLS\tINPUT\tOUTPUT\tCACHED\tREASONING\tTOTAL\tCOST")
		fmt.Fprintln(w, "\t--------\t-----\t-----\t-----\t------\t------\t---------\t-----\t----")
		var totals stats.Entry
		for _, e := range entries {
			fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%g\n",
				e.Provider, e.Model, e.Calls,
				e.PromptTokens, e.CompletionTokens, e.CachedTokens, e.ReasoningTokens,
				e.TotalTokens, e.Cost)
			totals.Calls += e.Calls
			totals.PromptTokens += e.PromptTokens
			totals.CompletionTokens += e.CompletionTokens
			totals.CachedTokens += e.CachedTokens
			totals.ReasoningTokens += e.ReasoningTokens
			totals.TotalTokens += e.TotalTokens
			totals.Cost += e.Cost
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%d\t%g\n",
			"TOTAL", "", totals.Calls,
			totals.PromptTokens, totals.CompletionTokens, totals.CachedTokens, totals.ReasoningTokens,
			totals.TotalTokens, totals.Cost)
		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().BoolVar(&statsReset, "reset", false, "clear all recorded usage stats")
}
