package llm

import (
	"testing"
)

func TestMaybeNoThink(t *testing.T) {
	thinkingTemplate := "{% if message %}<think>reasoning</think>{% endif %}"
	plainTemplate := "{{ bos_token }}[INST]{{ content }}[/INST]"
	prompt := "<|im_start|>assistant\n"

	tests := []struct {
		name     string
		noThink  bool
		template string
		prompt   string
		want     string
	}{
		{"disabled returns unchanged", false, thinkingTemplate, prompt, prompt},
		{"plain template unchanged", true, plainTemplate, prompt, prompt},
		{"empty template unchanged", true, "", prompt, prompt},
		{
			"thinking template gets empty block",
			true, thinkingTemplate, prompt, prompt + emptyThinkBlock,
		},
		{
			"existing block not doubled",
			true, thinkingTemplate, prompt + emptyThinkBlock, prompt + emptyThinkBlock,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &LlamacppCaller{NoThink: tt.noThink, template: tt.template}
			if got := l.maybeNoThink(tt.prompt); got != tt.want {
				t.Errorf("maybeNoThink() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMaybeNoThinkEffortNone(t *testing.T) {
	thinkingTemplate := "{% if message %}<think>reasoning</think>{% endif %}"
	prompt := "<|im_start|>assistant\n"
	l := &LlamacppCaller{ThinkEffort: ThinkEffortNone, template: thinkingTemplate}
	if got := l.maybeNoThink(prompt); got != prompt+emptyThinkBlock {
		t.Errorf("maybeNoThink() with effort none = %q, want block appended", got)
	}
	l = &LlamacppCaller{ThinkEffort: ThinkEffortLow, template: thinkingTemplate}
	if got := l.maybeNoThink(prompt); got != prompt {
		t.Errorf("maybeNoThink() with effort low = %q, want unchanged", got)
	}
}

func TestMaxTokens(t *testing.T) {
	if got := (&LlamacppCaller{}).maxTokens(); got != llamacppDefaultMaxTokens {
		t.Errorf("maxTokens() default = %d, want %d", got, llamacppDefaultMaxTokens)
	}
	if got := (&LlamacppCaller{MaxTokens: 128}).maxTokens(); got != 128 {
		t.Errorf("maxTokens() override = %d, want 128", got)
	}
	if got := (&LlamacppCaller{MaxTokens: -1}).maxTokens(); got != llamacppDefaultMaxTokens {
		t.Errorf("maxTokens() negative = %d, want default %d", got, llamacppDefaultMaxTokens)
	}
}
