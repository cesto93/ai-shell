package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorReset  = "\033[0m"
	ColorBold   = "\033[1m"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "get-config":
			PrintConfig()
			return
		}
	}

	fmt.Printf("%s%sStarting AI Shell...%s\n", ColorBold, ColorCyan, ColorReset)
	fmt.Printf("%sType 'exit' or 'quit' to leave, 'get-config' to show settings, or start with '!' to run a shell command.%s\n", ColorYellow, ColorReset)

	scanner := bufio.NewScanner(os.Stdin)
	ctx := context.Background()

	for {
		fmt.Printf("%s%sai-shell > %s", ColorBold, ColorGreen, ColorReset)
		if !scanner.Scan() {
			break
		}

		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			continue
		}

		if trimmed == "exit" || trimmed == "quit" {
			break
		}

		if trimmed == "get-config" {
			PrintConfig()
			continue
		}

		// Handle shell commands
		if strings.HasPrefix(trimmed, "!") {
			executeShellCommand(trimmed[1:])
			continue
		}

		fmt.Printf("%sLLM Response:%s\n", ColorBlue, ColorReset)
		err := CallOllama(ctx, trimmed)
		if err != nil {
			fmt.Printf("%sError: %v%s\n", ColorYellow, err, ColorReset)
		}
	}

	fmt.Printf("%sGoodbye!%s\n", ColorCyan, ColorReset)
}

func executeShellCommand(commandLine string) {
	commandLine = strings.TrimSpace(commandLine)
	if commandLine == "" {
		return
	}

	// For simple commands, we can just split by space.
	// For more complex ones with pipes, we might need to run via /bin/sh
	cmd := exec.Command("bash", "-c", commandLine)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		fmt.Printf("%sCommand failed: %v%s\n", ColorYellow, err, ColorReset)
	}
}
