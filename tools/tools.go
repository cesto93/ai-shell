package tools

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func RunCommand(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return out.String(), fmt.Errorf("command failed: %w", err)
	}

	return out.String(), nil
}

func WriteFile(path string, content string) (string, error) {
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}
	return fmt.Sprintf("File written successfully to %s", path), nil
}

func ReadFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}
