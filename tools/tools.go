package tools

import (
	"bytes"
	"fmt"
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
