package cmd

import (
	"fmt"
	"strings"

	"ai-shell/config"
	"ai-shell/llm"

	"github.com/spf13/cobra"
)

var showPrompt, showAgents, showSkills bool

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Show the context provided to the agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runContext()
	},
}

func init() {
	contextCmd.Flags().BoolVar(&showPrompt, "prompt", false, "print the full system prompt of the selected agent")
	contextCmd.Flags().BoolVar(&showAgents, "agents", false, "print the full text of each AGENTS.md file")
	contextCmd.Flags().BoolVar(&showSkills, "skills", false, "print the skills index sent to the agent")
	rootCmd.AddCommand(contextCmd)
}

func runContext() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	initLogger(cfg)

	if !cfg.AgentFiles {
		fmt.Println("Note: agent_files is disabled in config; AGENTS.md context is not sent to the agent.")
		fmt.Println()
	}

	if !cfg.Skills {
		fmt.Println("Note: skills is disabled in config; skills are not sent to the agent.")
		fmt.Println()
	}

	agent := llm.NewAgentFor(cfg.Agent, cfg.LLM.Model, cfg.LLM.Provider, cfg.Tools)
	prompt := agent.Prompt
	fmt.Printf("Agent %q system prompt (%d words, ~%d tokens)\n",
		cfg.Agent, len(strings.Fields(prompt)), estimateTokens(prompt))
	if showPrompt {
		fmt.Println("---")
		fmt.Println(prompt)
		fmt.Println("---")
	}
	fmt.Println()

	files := llm.GetAgentFileInfo(cfg.AgentFiles)
	if len(files) == 0 {
		fmt.Println("No AGENTS.md context files found.")
	} else {
		var totalWords, totalTokens int
		for _, f := range files {
			words := len(strings.Fields(f.Content))
			tokens := estimateTokens(f.Content)
			totalWords += words
			totalTokens += tokens
			fmt.Printf("%s (%d words, ~%d tokens)\n", f.Path, words, tokens)
			if showAgents {
				fmt.Println("---")
				fmt.Println(f.Content)
				fmt.Println("---")
			}
		}

		if len(files) > 1 {
			fmt.Printf("\nTotal: %d words, ~%d tokens\n", totalWords, totalTokens)
		}
	}
	fmt.Println()

	skillsIndex := llm.GetSkillsPrompt(cfg.Skills)
	skills := llm.GetSkills(cfg.Skills)
	if len(skills) == 0 {
		fmt.Println("No skills found (~/.agents/skills, ./skills).")
		return nil
	}

	fmt.Printf("Skills index (%d skills, ~%d tokens)\n", len(skills), estimateTokens(skillsIndex))
	for _, s := range skills {
		fmt.Printf("- %s: %s\n", s.Name, s.Path)
		if showSkills {
			fmt.Printf("  %s\n", s.Description)
		}
	}
	if showSkills {
		fmt.Println("---")
		fmt.Println(skillsIndex)
		fmt.Println("---")
	}

	return nil
}

// estimateTokens approximates token count using ~4 characters per token,
// a common heuristic for English text.
func estimateTokens(s string) int {
	return (len(s) + 3) / 4
}
