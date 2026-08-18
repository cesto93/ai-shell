package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const agentFileName = "AGENTS.md"

var (
	agentFilesUserConfigDir = os.UserConfigDir
	agentFilesGetwd         = os.Getwd
)

// GetAgentFiles returns the combined contents of the global
// (~/.config/ai-shell/AGENTS.md) and repo-level (./AGENTS.md) customization
// files, formatted as additional instructions. Returns "" when support is
// disabled or when neither file exists.
func GetAgentFiles(enabled bool) string {
	if !enabled {
		return ""
	}

	var parts []string

	if uc, err := agentFilesUserConfigDir(); err == nil {
		global := filepath.Join(uc, "ai-shell", agentFileName)
		if content, ok := readAgentFile(global); ok {
			parts = append(parts, fmt.Sprintf("# Instructions from %s\n%s", global, content))
		}
	}

	if cwd, err := agentFilesGetwd(); err == nil {
		repo := filepath.Join(cwd, agentFileName)
		if content, ok := readAgentFile(repo); ok {
			parts = append(parts, fmt.Sprintf("# Instructions from %s\n%s", agentFileName, content))
		}
	}

	return strings.Join(parts, "\n\n")
}

func readAgentFile(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	content := strings.TrimSpace(string(data))
	return content, content != ""
}
