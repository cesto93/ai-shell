package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ai-shell",
	Short: "AI Shell is an interactive shell powered by AI",
	Long:  `An interactive shell powered by AI (Ollama) that can help you with commands and explanations.`,
	Example: `  ai-shell
  ai-shell get-config
  echo "how do I list files?" | ai-shell`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := RunShell(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

var configCmd = &cobra.Command{
	Use:   "get-config",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		PrintConfig()
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
