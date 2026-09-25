package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func stripANSI(s string) string {
	return ansi.Strip(s)
}

func TestWrapShellTextFitsWidth(t *testing.T) {
	text := "L'ambientazione unisce elementi di un futuro distopico/tecnologico " +
		"a tracce di un passato recente sepolto sotto strati di mistero e meraviglia"
	for _, width := range []int{40, 80, 100} {
		wrapped := wrapShellText(text, width)
		for _, line := range strings.Split(wrapped, "\n") {
			if utf8.RuneCountInString(line) > width {
				t.Errorf("width %d: line exceeds width: %q", width, line)
			}
		}
	}
}

func TestWrapShellTextPreservesNewlines(t *testing.T) {
	text := "### 1. Punti di Forza\n\n* item one\n* item two"
	wrapped := wrapShellText(text, 80)
	if !strings.Contains(wrapped, "\n\n") {
		t.Errorf("expected blank line preserved, got %q", wrapped)
	}
}

func TestWrapShellTextWithPrefixFitsWidth(t *testing.T) {
	text := strings.Repeat("parola ", 40)
	for _, tc := range []struct{ prefix string }{{"AI: "}, {"You: "}, {"Error: "}} {
		lines := wrapShellTextWithPrefix(tc.prefix, text, 80)
		for _, line := range lines {
			if utf8.RuneCountInString(line) > 80 {
				t.Errorf("prefix %q: line exceeds width: %q", tc.prefix, line)
			}
		}
		if !strings.HasPrefix(lines[0], tc.prefix) {
			t.Errorf("prefix %q: first line missing prefix: %q", tc.prefix, lines[0])
		}
	}
}

func TestShellViewNoLineExceedsWidth(t *testing.T) {
	m := &ShellModel{width: 80}
	long := strings.Repeat("Questa è una frase molto lunga da mandare a capo. ", 20)
	m.messages = []Message{
		{role: "assistant", content: long},
		{role: "user", content: long},
		{role: "system", content: long},
		{role: "error", content: long},
		{role: "tool", content: long},
	}
	view := stripANSI(m.View())
	for _, line := range strings.Split(view, "\n") {
		if utf8.RuneCountInString(line) > 80 {
			t.Errorf("view line exceeds width 80: %q", line)
		}
	}
}
