package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"ai-shell/config"

	"github.com/spf13/cobra"
)

var modelsSet string

var modelsCmd = &cobra.Command{
	Use:   "models",
	Short: "List all available models from all providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runModels(cmd)
	},
}

func init() {
	rootCmd.AddCommand(modelsCmd)
	modelsCmd.Flags().StringVarP(&modelsSet, "set", "s", "", "Set the current model")
}

func runModels(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	initLogger(cfg)

	if cmd.Flags().Changed("set") {
		info := config.LookupModelInfo(modelsSet)
		if info == nil {
			return fmt.Errorf("model %q not found", modelsSet)
		}
		if err := config.SaveModelWithProvider(modelsSet, info.Provider); err != nil {
			return fmt.Errorf("failed to set model: %w", err)
		}
		fmt.Printf("Model set to %s (provider: %s)\n", modelsSet, info.Provider)
		return nil
	}

	models := config.GetAllAvailableModels()
	if len(models) == 0 {
		fmt.Println("No models found.")
		return nil
	}

	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].Name < models[j].Name
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "\tMODEL\tPROVIDER\tSIZE\tINPUT TYPES")
	fmt.Fprintln(w, "\t-----\t--------\t----\t-----------")
	for _, m := range models {
		marker := ""
		if m.Name == cfg.LLM.Model && m.Provider == cfg.LLM.Provider {
			marker = "*"
		}
		size := m.Size
		if size == "" {
			size = "-"
		}
		inputTypes := strings.Join(m.InputTypes, ", ")
		if inputTypes == "" {
			inputTypes = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", marker, m.Name, m.Provider, size, inputTypes)
	}
	w.Flush()

	return nil
}
