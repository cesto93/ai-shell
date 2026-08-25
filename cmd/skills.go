package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"ai-shell/config"
	"ai-shell/llm"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "List all available skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSkills()
	},
}

func init() {
	rootCmd.AddCommand(skillsCmd)
}

func runSkills() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	initLogger(cfg)

	if !cfg.Skills {
		fmt.Println("Note: skills is disabled in config; skills are not sent to the agent.")
	}

	skills := llm.GetSkills(cfg.Skills)
	if len(skills) == 0 {
		fmt.Println("No skills found (~/.agents/skills, ./skills).")
		return nil
	}

	nameW := utf8.RuneCountInString("NAME")
	for _, s := range skills {
		if l := utf8.RuneCountInString(s.Name); l > nameW {
			nameW = l
		}
	}
	nameW += 3

	width := 80
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > nameW+10 {
		width = w
	}
	descW := width - nameW

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION")
	fmt.Fprintln(w, "----\t-----------")
	for _, s := range skills {
		desc := s.Description
		if desc == "" {
			desc = "-"
		}
		for i, line := range wrapText(desc, descW) {
			name := ""
			if i == 0 {
				name = s.Name
			}
			fmt.Fprintf(w, "%s\t%s\n", name, line)
		}
		for _, line := range wrapText(s.Path, descW) {
			fmt.Fprintf(w, "\t%s\n", line)
		}
		fmt.Fprintln(w)
	}
	w.Flush()

	return nil
}

// wrapText wraps text to the given display width, breaking on spaces and
// hard-splitting words that are longer than the width itself.
func wrapText(text string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		for utf8.RuneCountInString(word) > width {
			if line != "" {
				lines = append(lines, line)
				line = ""
			}
			r := []rune(word)
			lines = append(lines, string(r[:width]))
			word = string(r[width:])
		}
		switch {
		case line == "":
			line = word
		case utf8.RuneCountInString(line)+1+utf8.RuneCountInString(word) <= width:
			line += " " + word
		default:
			lines = append(lines, line)
			line = word
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
