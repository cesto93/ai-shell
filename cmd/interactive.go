package cmd

import (
	"fmt"

	"ai-shell/config"

	"github.com/spf13/cobra"
)

const (
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
	Prompt      = ColorBold + ColorGreen + "ai-shell > " + ColorReset
)

var getConfigCmd = &cobra.Command{
	Use:   "get-config",
	Short: "Show current configuration",
	Run: func(cmd *cobra.Command, args []string) {
		PrintConfig()
	},
}

func PrintConfig() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("%sError loading config: %v%s\n", ColorYellow, err, ColorReset)
		return
	}

	fmt.Printf("%sCurrent Configuration:%s\n", ColorBold+ColorCyan, ColorReset)
	fmt.Printf("Provider: %s%s%s\n", ColorGreen, cfg.LLM.Provider, ColorReset)
	fmt.Printf("Model: %s%s%s\n", ColorGreen, cfg.LLM.Model, ColorReset)
	fmt.Printf("Confirm Commands: %s%v%s\n", ColorGreen, cfg.Shell.Confirm, ColorReset)
	fmt.Printf("Allowed Commands: %s%s%s\n", ColorGreen, cfg.Shell.AllowedCommands, ColorReset)

	if cfg.ConfigFile != "" {
		fmt.Printf("Config file: %s%s%s\n", ColorBlue, cfg.ConfigFile, ColorReset)
	} else {
		fmt.Printf("Config file: %sNone (using defaults)%s\n", ColorYellow, ColorReset)
	}
}
