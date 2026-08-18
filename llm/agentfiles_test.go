package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetAgentFilesDisabled(t *testing.T) {
	if got := GetAgentFiles(false); got != "" {
		t.Errorf("GetAgentFiles(false) = %q, want empty string", got)
	}
}

func TestGetAgentFilesNoneFound(t *testing.T) {
	tmp := t.TempDir()

	origUserConfigDir := agentFilesUserConfigDir
	origGetwd := agentFilesGetwd
	agentFilesUserConfigDir = func() (string, error) { return tmp, nil }
	agentFilesGetwd = func() (string, error) { return tmp, nil }
	defer func() {
		agentFilesUserConfigDir = origUserConfigDir
		agentFilesGetwd = origGetwd
	}()

	if got := GetAgentFiles(true); got != "" {
		t.Errorf("GetAgentFiles(true) = %q, want empty string", got)
	}
}

func TestGetAgentFilesGlobalAndRepo(t *testing.T) {
	tmp := t.TempDir()

	globalDir := filepath.Join(tmp, "ai-shell")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}
	globalContent := "Global instructions for the agent."
	if err := os.WriteFile(filepath.Join(globalDir, "AGENTS.md"), []byte(globalContent), 0644); err != nil {
		t.Fatal(err)
	}

	repoDir := filepath.Join(tmp, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	repoContent := "Repo-level instructions for the agent."
	if err := os.WriteFile(filepath.Join(repoDir, "AGENTS.md"), []byte(repoContent), 0644); err != nil {
		t.Fatal(err)
	}

	origUserConfigDir := agentFilesUserConfigDir
	origGetwd := agentFilesGetwd
	agentFilesUserConfigDir = func() (string, error) { return tmp, nil }
	agentFilesGetwd = func() (string, error) { return repoDir, nil }
	defer func() {
		agentFilesUserConfigDir = origUserConfigDir
		agentFilesGetwd = origGetwd
	}()

	got := GetAgentFiles(true)
	if !strings.Contains(got, globalContent) {
		t.Errorf("GetAgentFiles() missing global content, got: %q", got)
	}
	if !strings.Contains(got, repoContent) {
		t.Errorf("GetAgentFiles() missing repo content, got: %q", got)
	}
	if !strings.Contains(got, "AGENTS.md") {
		t.Errorf("GetAgentFiles() missing file labels, got: %q", got)
	}
}

func TestGetAgentFilesEmptyFileIgnored(t *testing.T) {
	tmp := t.TempDir()

	globalDir := filepath.Join(tmp, "ai-shell")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "AGENTS.md"), []byte("   \n  "), 0644); err != nil {
		t.Fatal(err)
	}

	origUserConfigDir := agentFilesUserConfigDir
	origGetwd := agentFilesGetwd
	agentFilesUserConfigDir = func() (string, error) { return tmp, nil }
	agentFilesGetwd = func() (string, error) { return tmp, nil }
	defer func() {
		agentFilesUserConfigDir = origUserConfigDir
		agentFilesGetwd = origGetwd
	}()

	if got := GetAgentFiles(true); got != "" {
		t.Errorf("GetAgentFiles(true) with empty file = %q, want empty string", got)
	}
}
