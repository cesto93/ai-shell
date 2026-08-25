package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	skillFileName = "SKILL.md"
	skillDirName  = "skills"
)

var (
	skillsUserHomeDir = os.UserHomeDir
	skillsGetwd       = os.Getwd
)

// SkillInfo describes a discovered skill: a <dir>/SKILL.md file under the
// global ~/.agents/skills/ or repo-level ./skills/ directory.
type SkillInfo struct {
	Name        string
	Description string
	Path        string
}

// GetSkills returns the skills discovered in the global (~/.agents/skills)
// and repo-level (./skills) directories, sorted by name within each
// location. Returns nil when support is disabled or when no skill exists.
func GetSkills(enabled bool) []SkillInfo {
	if !enabled {
		return nil
	}

	var skills []SkillInfo

	if home, err := skillsUserHomeDir(); err == nil {
		skills = append(skills, discoverSkills(filepath.Join(home, ".agents", skillDirName))...)
	}

	if cwd, err := skillsGetwd(); err == nil {
		skills = append(skills, discoverSkills(filepath.Join(cwd, skillDirName))...)
	}

	return skills
}

func discoverSkills(dir string) []SkillInfo {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var skills []SkillInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), skillFileName)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		name, description := parseSkillFrontmatter(content)
		if name == "" {
			name = e.Name()
		}
		skills = append(skills, SkillInfo{Name: name, Description: description, Path: path})
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills
}

// parseSkillFrontmatter extracts the name and description fields from the
// YAML frontmatter of a SKILL.md file. Returns "" for missing or malformed
// frontmatter so callers can fall back to the directory name.
func parseSkillFrontmatter(content string) (name, description string) {
	if !strings.HasPrefix(content, "---") {
		return "", ""
	}
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", ""
	}

	var fm struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(rest[:end]), &fm); err != nil {
		return "", ""
	}
	return strings.TrimSpace(fm.Name), strings.TrimSpace(fm.Description)
}

// GetSkillsPrompt renders the discovered skills as a compact index for
// inclusion in the system prompt. Full SKILL.md contents are deliberately
// excluded; the agent reads them on demand via ReadFile when a task matches.
// Returns "" when support is disabled or no skills are found.
func GetSkillsPrompt(enabled bool) string {
	skills := GetSkills(enabled)
	if len(skills) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("# Skills\n")
	b.WriteString("The following skills provide specialized instructions and workflows. When a task matches one of these skills, read its SKILL.md file before proceeding:\n")
	for _, s := range skills {
		description := strings.ReplaceAll(strings.TrimSpace(s.Description), "\n", " ")
		fmt.Fprintf(&b, "- **%s**: %s (read %s)\n", s.Name, description, s.Path)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
