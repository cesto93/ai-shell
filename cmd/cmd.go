package cmd

import (
	"fmt"
	"os"

	"ai-shell/config"

	"github.com/spf13/cobra"
)

var debug bool

var rootCmd = &cobra.Command{
	Use:   "ai-shell",
	Short: "AI Shell is an interactive shell powered by AI",
	Long:  `An interactive shell powered by AI that can help you with commands and explanations.`,
	Example: `  ai-shell
  ai-shell get-config
  ai-shell commit
  ai-shell models
  ai-shell extract notes.txt schema.json
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
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug logging (temporarily overrides the configured log level)")
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(commitCmd)
	rootCmd.AddCommand(extractCmd)
}

// initLogger configures the global slog level, temporarily forcing debug mode
// when the --debug flag is passed. The configured log level is not modified.
func initLogger(cfg *config.Config) {
	if debug {
		cfg.LogLevel = "debug"
	}
	config.InitLogger(cfg.LogLevel)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
