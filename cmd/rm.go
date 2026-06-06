package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <model-name>",
	Short: "Remove a downloaded model",
	Long: `Delete a GGUF model from disk and remove it from models.ini.

The model name is the name shown by 'ai-shell list' (without
the .gguf extension and without the mmproj- prefix).

Examples:
  ai-shell rm mistral-7b-instruct-v0.1.Q4_K_M
  ai-shell rm qwen3-asr`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		modelName := args[0]

		modelsDir, err := getModelsDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		entries, err := os.ReadDir(modelsDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: model %q not found\n", modelName)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		var removed []string
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gguf") {
				continue
			}
			if modelNameFromFile(entry.Name()) == modelName {
				path := filepath.Join(modelsDir, entry.Name())
				if err := os.Remove(path); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", entry.Name(), err)
					continue
				}
				removed = append(removed, entry.Name())
			}
		}

		if len(removed) == 0 {
			fmt.Fprintf(os.Stderr, "Error: model %q not found\n", modelName)
			os.Exit(1)
		}

		for _, name := range removed {
			fmt.Printf("Removed %s\n", name)
		}

		if err := updateModelsIni(modelsDir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to update models.ini: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(rmCmd)
}
