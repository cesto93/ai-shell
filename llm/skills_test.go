package llm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name, skillFileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func mockSkillDirs(t *testing.T, home, cwd string) {
	t.Helper()
	origHome := skillsUserHomeDir
	origGetwd := skillsGetwd
	skillsUserHomeDir = func() (string, error) { return home, nil }
	skillsGetwd = func() (string, error) { return cwd, nil }
	t.Cleanup(func() {
		skillsUserHomeDir = origHome
		skillsGetwd = origGetwd
	})
}

func TestGetSkillsDisabled(t *testing.T) {
	if got := GetSkills(false); got != nil {
		t.Errorf("GetSkills(false) = %v, want nil", got)
	}
	if got := GetSkillsPrompt(false); got != "" {
		t.Errorf("GetSkillsPrompt(false) = %q, want empty string", got)
	}
}

func TestGetSkillsNoneFound(t *testing.T) {
	tmp := t.TempDir()
	mockSkillDirs(t, tmp, tmp)

	if got := GetSkills(true); got != nil {
		t.Errorf("GetSkills(true) = %v, want nil", got)
	}
	if got := GetSkillsPrompt(true); got != "" {
		t.Errorf("GetSkillsPrompt(true) = %q, want empty string", got)
	}
}

func TestGetSkillsGlobalAndRepo(t *testing.T) {
	tmp := t.TempDir()
	repo := t.TempDir()
	mockSkillDirs(t, tmp, repo)

	writeSkill(t, filepath.Join(tmp, ".agents", "skills"), "global-skill",
		"---\nname: global-skill\ndescription: A global skill.\n---\n\n# Global Skill\n")
	writeSkill(t, filepath.Join(repo, "skills"), "repo-skill",
		"---\nname: repo-skill\ndescription: A repo skill.\n---\n\n# Repo Skill\n")

	skills := GetSkills(true)
	if len(skills) != 2 {
		t.Fatalf("GetSkills() returned %d skills, want 2", len(skills))
	}

	globalPath := filepath.Join(tmp, ".agents", "skills", "global-skill", "SKILL.md")
	repoPath := filepath.Join(repo, "skills", "repo-skill", "SKILL.md")
	if skills[0].Name != "global-skill" || skills[0].Description != "A global skill." || skills[0].Path != globalPath {
		t.Errorf("global skill mismatch: got %+v, want name=global-skill path=%s", skills[0], globalPath)
	}
	if skills[1].Name != "repo-skill" || skills[1].Description != "A repo skill." || skills[1].Path != repoPath {
		t.Errorf("repo skill mismatch: got %+v, want name=repo-skill path=%s", skills[1], repoPath)
	}
}

func TestGetSkillsSortedByName(t *testing.T) {
	tmp := t.TempDir()
	mockSkillDirs(t, tmp, tmp)

	globalDir := filepath.Join(tmp, ".agents", "skills")
	writeSkill(t, globalDir, "zulu", "---\nname: zulu\ndescription: Z.\n---\nbody")
	writeSkill(t, globalDir, "alpha", "---\nname: alpha\ndescription: A.\n---\nbody")
	writeSkill(t, globalDir, "mike", "no frontmatter here")

	skills := GetSkills(true)
	if len(skills) != 3 {
		t.Fatalf("GetSkills() returned %d skills, want 3", len(skills))
	}
	want := []string{"alpha", "mike", "zulu"}
	for i, s := range skills {
		if s.Name != want[i] {
			t.Errorf("skills[%d].Name = %q, want %q", i, s.Name, want[i])
		}
	}
}

func TestParseSkillFrontmatterFallbacks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		wantName string
		wantDesc string
	}{
		{name: "valid frontmatter", content: "---\nname: my-skill\ndescription: Does things.\n---\n# Body", wantName: "my-skill", wantDesc: "Does things."},
		{name: "no frontmatter", content: "# Just a body", wantName: "", wantDesc: ""},
		{name: "unclosed fence", content: "---\nname: broken\nbody forever", wantName: "", wantDesc: ""},
		{name: "extra fields ignored", content: "---\nname: x\nlicense: MIT\nmetadata:\n  version: \"1\"\ndescription: D.\n---\nb", wantName: "x", wantDesc: "D."},
		{name: "multiline description folded", content: "---\nname: m\ndescription: line one\n  continued\n---\nb", wantName: "m", wantDesc: "line one continued"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotDesc := parseSkillFrontmatter(tt.content)
			if gotName != tt.wantName || gotDesc != tt.wantDesc {
				t.Errorf("parseSkillFrontmatter() = (%q, %q), want (%q, %q)", gotName, gotDesc, tt.wantName, tt.wantDesc)
			}
		})
	}
}

func TestGetSkillsEmptyFileIgnored(t *testing.T) {
	tmp := t.TempDir()
	mockSkillDirs(t, tmp, tmp)

	writeSkill(t, filepath.Join(tmp, ".agents"), "empty", "   \n  ")

	if got := GetSkills(true); got != nil {
		t.Errorf("GetSkills(true) with empty SKILL.md = %v, want nil", got)
	}
}

func TestGetSkillsPrompt(t *testing.T) {
	tmp := t.TempDir()
	repo := t.TempDir()
	mockSkillDirs(t, tmp, repo)

	writeSkill(t, filepath.Join(tmp, ".agents", "skills"), "docskill",
		"---\nname: docskill\ndescription: Reads documents.\n---\nbody")

	got := GetSkillsPrompt(true)
	if !strings.HasPrefix(got, "# Skills") {
		t.Errorf("GetSkillsPrompt() missing header, got: %q", got)
	}
	if !strings.Contains(got, "**docskill**: Reads documents.") {
		t.Errorf("GetSkillsPrompt() missing skill entry, got: %q", got)
	}
	path := filepath.Join(tmp, ".agents", "skills", "docskill", "SKILL.md")
	if !strings.Contains(got, "(read "+path+")") {
		t.Errorf("GetSkillsPrompt() missing read instruction with path %s, got: %q", path, got)
	}
	if strings.Contains(got, "body") {
		t.Errorf("GetSkillsPrompt() should not include full skill contents, got: %q", got)
	}
}
