package main

import (
	"bufio"
	"os"
	"strings"
)

// GetDistro returns the Linux distribution name from /etc/os-release
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

// GetShell returns the current shell from the SHELL environment variable
func GetShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "Unknown Shell"
	}
	return shell
}
