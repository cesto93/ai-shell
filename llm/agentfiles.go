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

// AgentFileInfo describes an AGENTS.md customization file read for the agent.
type AgentFileInfo struct {
	Path    string
	Content string
}

// GetAgentFileInfo returns the global (~/.config/ai-shell/AGENTS.md) and
// repo-level (./AGENTS.md) customization files that exist and are non-empty.
// Returns nil when support is disabled or when neither file exists.
func GetAgentFileInfo(enabled bool) []AgentFileInfo {
	if !enabled {
		return nil
	}

	var files []AgentFileInfo

	if uc, err := agentFilesUserConfigDir(); err == nil {
		global := filepath.Join(uc, "ai-shell", agentFileName)
		if content, ok := readAgentFile(global); ok {
			files = append(files, AgentFileInfo{Path: global, Content: content})
		}
	}

	if cwd, err := agentFilesGetwd(); err == nil {
		repo := filepath.Join(cwd, agentFileName)
		if content, ok := readAgentFile(repo); ok {
			files = append(files, AgentFileInfo{Path: repo, Content: content})
		}
	}

	return files
}

// GetAgentFiles returns the combined contents of the global
// (~/.config/ai-shell/AGENTS.md) and repo-level (./AGENTS.md) customization
// files, formatted as additional instructions. Returns "" when support is
// disabled or when neither file exists.
func GetAgentFiles(enabled bool) string {
	var parts []string
	for _, f := range GetAgentFileInfo(enabled) {
		parts = append(parts, fmt.Sprintf("# Instructions from %s\n%s", f.Path, f.Content))
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
