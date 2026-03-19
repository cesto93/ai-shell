package main

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"
)

type completer struct{}

func (c *completer) Do(line []rune, pos int) ([][]rune, int) {
	currentLine := string(line[:pos])
	lastAt := strings.LastIndex(currentLine, "@")

	if lastAt == -1 {
		return nil, pos
	}

	partial := currentLine[lastAt+1:]

	if strings.Contains(partial, "/") {
		dir := filepath.Dir(partial)
		if !strings.HasSuffix(dir, "/") && dir != "." {
			dir += "/"
		}
		base := filepath.Base(partial)
		matches := c.completeInDir(dir, base)
		return matches, lastAt + 1
	}

	matches := c.completeFiles(partial)
	return matches, lastAt + 1
}

func (c *completer) completeFiles(prefix string) [][]rune {
	var results [][]rune
	cwd, err := os.Getwd()
	if err != nil {
		return results
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		return results
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefix) {
			if entry.IsDir() {
				results = append(results, []rune(name+"/"))
			} else {
				results = append(results, []rune(name))
			}
		}
	}

	return results
}

func (c *completer) completeInDir(dir, prefix string) [][]rune {
	var results [][]rune

	absDir := dir
	if !filepath.IsAbs(dir) {
		cwd, _ := os.Getwd()
		absDir = filepath.Join(cwd, dir)
	}

	absDir = filepath.Clean(absDir)

	entries, err := os.ReadDir(absDir)
	if err != nil {
		homeDir, _ := os.UserHomeDir()
		if strings.HasPrefix(dir, "~") {
			absDir = filepath.Join(homeDir, dir[1:])
			absDir = filepath.Clean(absDir)
			entries, err = os.ReadDir(absDir)
		}
		if err != nil {
			return results
		}
	}

	prefixBase := filepath.Base(prefix)
	prefixDir := filepath.Dir(prefix)
	if prefixDir == "." {
		prefixDir = ""
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, prefixBase) {
			fullPath := filepath.Join(dir, name)
			if entry.IsDir() {
				results = append(results, []rune(fullPath+"/"))
			} else {
				results = append(results, []rune(fullPath))
			}
		}
	}

	return results
}

func isInCompletion(line []rune, pos int) (bool, int) {
	currentLine := string(line[:pos])
	lastAt := strings.LastIndex(currentLine, "@")
	return lastAt != -1, lastAt
}

func getCompletions(line []rune, pos int) [][]rune {
	c := &completer{}
	matches, _ := c.Do(line, pos)
	return matches
}

func NewReadline() (*readline.Instance, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          ColorBold + ColorGreen + "ai-shell > " + ColorReset,
		HistoryFile:     getHistoryFile(),
		AutoComplete:    &completer{},
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	return rl, err
}

func getHistoryFile() string {
	usr, err := user.Current()
	if err != nil {
		return ""
	}
	return filepath.Join(usr.HomeDir, ".ai-shell-history")
}
