package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

// newSkillsRepoFixture builds a fake cloned skill repo with the standard
// layout: skills/<name>/SKILL.md plus an unrelated root file.
func newSkillsRepoFixture(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, "skills", "alpha", "SKILL.md"),
		"---\nname: Alpha\ndescription: first skill\n---\nbody\n", 0644)
	writeTestFile(t, filepath.Join(repo, "skills", "beta", "SKILL.md"),
		"---\nname: Beta\ndescription: second skill\n---\nbody\n", 0644)
	writeTestFile(t, filepath.Join(repo, "skills", "beta", "scripts", "x.sh"),
		"#!/bin/sh\n", 0755)
	writeTestFile(t, filepath.Join(repo, "README.md"), "readme\n", 0644)
	return repo
}

func TestFindSkillDirs(t *testing.T) {
	root := t.TempDir()
	skill := "---\nname: x\ndescription: y\n---\nbody\n"
	writeTestFile(t, filepath.Join(root, "skills", "foo", "SKILL.md"), skill, 0644)
	writeTestFile(t, filepath.Join(root, "skills", "foo", "extra.txt"), "data", 0644)
	writeTestFile(t, filepath.Join(root, "skills", "bar", "SKILL.md"), skill, 0644)
	writeTestFile(t, filepath.Join(root, "standalone", "SKILL.md"), skill, 0644)
	writeTestFile(t, filepath.Join(root, "not-a-skill", "README.md"), "readme", 0644)
	writeTestFile(t, filepath.Join(root, ".git", "nested", "SKILL.md"), skill, 0644)

	tests := []struct {
		name string
		root string
		want []string
	}{
		{
			name: "finds skills at any depth",
			root: root,
			want: []string{
				filepath.Join(root, "skills", "bar"),
				filepath.Join(root, "skills", "foo"),
				filepath.Join(root, "standalone"),
			},
		},
		{
			name: "missing root",
			root: filepath.Join(root, "nope"),
			want: nil,
		},
		{
			name: "empty root",
			root: t.TempDir(),
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findSkillDirs(tt.root)
			if err != nil {
				t.Fatalf("findSkillDirs() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("findSkillDirs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("findSkillDirs()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestPullSkills(t *testing.T) {
	tests := []struct {
		name       string
		repo       func(t *testing.T) string
		preInstall func(t *testing.T, destRoot string)
		cloneFails bool
		wantErr    string
		check      func(t *testing.T, destRoot string)
	}{
		{
			name: "installs skills into global dir",
			repo: newSkillsRepoFixture,
			check: func(t *testing.T, destRoot string) {
				alpha, err := os.ReadFile(filepath.Join(destRoot, "alpha", "SKILL.md"))
				if err != nil {
					t.Fatalf("alpha SKILL.md not installed: %v", err)
				}
				if !strings.Contains(string(alpha), "name: Alpha") {
					t.Errorf("unexpected alpha content: %q", alpha)
				}
				script, err := os.Stat(filepath.Join(destRoot, "beta", "scripts", "x.sh"))
				if err != nil {
					t.Fatalf("nested file not copied: %v", err)
				}
				if script.Mode().Perm() != 0755 {
					t.Errorf("script mode = %v, want 0755", script.Mode().Perm())
				}
				if _, err := os.Stat(filepath.Join(destRoot, "README.md")); err == nil {
					t.Error("repo README should not be installed")
				}
			},
		},
		{
			name: "updates existing skill in place",
			repo: func(t *testing.T) string {
				repo := newSkillsRepoFixture(t)
				writeTestFile(t, filepath.Join(repo, "skills", "alpha", "SKILL.md"),
					"---\nname: Alpha\ndescription: updated\n---\nnew body\n", 0644)
				return repo
			},
			check: func(t *testing.T, destRoot string) {
				data, err := os.ReadFile(filepath.Join(destRoot, "alpha", "SKILL.md"))
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(data), "updated") {
					t.Errorf("skill not updated: %q", data)
				}
				if _, err := os.Stat(filepath.Join(destRoot, "alpha", "stale.txt")); err == nil {
					t.Error("stale file should have been removed with the old skill dir")
				}
			},
			preInstall: func(t *testing.T, destRoot string) {
				writeTestFile(t, filepath.Join(destRoot, "alpha", "SKILL.md"),
					"---\nname: Alpha\ndescription: old\n---\nold body\n", 0644)
				writeTestFile(t, filepath.Join(destRoot, "alpha", "stale.txt"), "junk", 0644)
			},
		},
		{
			name: "no skills in repo",
			repo: func(t *testing.T) string {
				repo := t.TempDir()
				writeTestFile(t, filepath.Join(repo, "README.md"), "x", 0644)
				return repo
			},
			wantErr: "no skills found",
		},
		{
			name:       "clone failure",
			repo:       newSkillsRepoFixture,
			cloneFails: true,
			wantErr:    "git clone failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			destRoot := filepath.Join(home, ".agents", "skills")
			fixture := tt.repo(t)

			origHome, origExec := skillsHomeDir, execCommand
			defer func() { skillsHomeDir, execCommand = origHome, origExec }()
			skillsHomeDir = func() (string, error) { return home, nil }
			execCommand = func(command string, args ...string) *exec.Cmd {
				if command == "git" && len(args) >= 2 && args[0] == "clone" {
					if tt.cloneFails {
						return exec.Command("sh", "-c", "echo 'fatal: boom' >&2; false")
					}
					return exec.Command("cp", "-a", fixture+"/.", args[len(args)-1])
				}
				return exec.Command(command, args...)
			}

			if tt.preInstall != nil {
				tt.preInstall(t, destRoot)
			}

			err := pullSkills("https://example.com/org/skills.git")
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("pullSkills() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("pullSkills() error = %v", err)
			}
			if tt.check != nil {
				tt.check(t, destRoot)
			}
		})
	}
}

func TestGlobalSkillsDir(t *testing.T) {
	origHome := skillsHomeDir
	defer func() { skillsHomeDir = origHome }()
	skillsHomeDir = func() (string, error) { return "/home/tester", nil }

	got, err := globalSkillsDir()
	if err != nil {
		t.Fatalf("globalSkillsDir() error = %v", err)
	}
	if want := filepath.Join("/home/tester", ".agents", "skills"); got != want {
		t.Errorf("globalSkillsDir() = %q, want %q", got, want)
	}
}
