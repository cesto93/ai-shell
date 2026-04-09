package tools

import (
	"bufio"
	"os"
	"strings"
)

func GetDistro() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "Unknown Linux Distro"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "Linux"
}

func GetShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "Unknown Shell"
	}
	return shell
}
