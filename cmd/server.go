package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var serverPort int

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start llama-server with models.ini",
	Long: `Launch llama-server with the models.ini configuration from ~/.ai-shell.
The server runs in the foreground; press Ctrl+C to stop it.`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		workDir := filepath.Join(home, ".ai-shell")
		iniPath := filepath.Join(workDir, "models.ini")

		if _, err := os.Stat(iniPath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: %s not found\n", iniPath)
			os.Exit(1)
		}

		llamaCmd := exec.Command("llama-server",
			"--models-preset", "models.ini",
			"--port", fmt.Sprintf("%d", serverPort),
		)
		llamaCmd.Dir = workDir
		llamaCmd.Stdout = os.Stdout
		llamaCmd.Stderr = os.Stderr
		llamaCmd.Stdin = os.Stdin

		fmt.Printf("Starting llama-server on port %d (working dir: %s)\n", serverPort, workDir)
		if err := llamaCmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "llama-server exited: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	serverCmd.Flags().IntVarP(&serverPort, "port", "p", 8080, "Port to listen on")
	rootCmd.AddCommand(serverCmd)
}
