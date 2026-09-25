package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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

func TestShellViewTurnBlocks(t *testing.T) {
	m := &ShellModel{width: 80}
	m.messages = []Message{
		{role: "user", content: "ciao mondo"},
		{role: "assistant", content: "ciao a te"},
	}
	view := stripANSI(m.View())
	if !strings.Contains(view, "You") {
		t.Errorf("expected user block header, got:\n%s", view)
	}
	if !strings.Contains(view, "AI") {
		t.Errorf("expected AI block header, got:\n%s", view)
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
		t.Errorf("expected rounded block borders, got:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if utf8.RuneCountInString(line) > 80 {
			t.Errorf("block line exceeds width 80: %q", line)
		}
	}
}

func TestShellViewportScroll(t *testing.T) {
	m := &ShellModel{width: 80, height: 24, followOutput: true}
	m.viewport = viewport.New(80, 20)
	m.ready = true
	for i := 0; i < 50; i++ {
		m.messages = append(m.messages, Message{
			role:    "assistant",
			content: strings.Repeat("riga di testo ", 10),
		})
	}
	m.View()
	if !m.viewport.AtBottom() {
		t.Fatalf("expected viewport at bottom after render")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	m = updated.(*ShellModel)
	if m.followOutput {
		t.Errorf("expected follow detached after PgUp")
	}
	if m.viewport.AtBottom() {
		t.Errorf("expected viewport scrolled up after PgUp")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	m = updated.(*ShellModel)
	if !m.followOutput {
		t.Errorf("expected follow re-attached after End")
	}
	if !m.viewport.AtBottom() {
		t.Errorf("expected viewport at bottom after End")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftUp})
	m = updated.(*ShellModel)
	if m.followOutput {
		t.Errorf("expected follow detached after Shift+Up")
	}
	if m.viewport.AtBottom() {
		t.Errorf("expected viewport scrolled up after Shift+Up")
	}
}

func TestShellScrollHint(t *testing.T) {
	m := &ShellModel{width: 80, height: 24, followOutput: true}
	m.viewport = viewport.New(80, 20)
	m.ready = true
	for i := 0; i < 50; i++ {
		m.messages = append(m.messages, Message{
			role:    "assistant",
			content: strings.Repeat("riga di testo ", 10),
		})
	}
	m.View() // first render fills the viewport
	footer := stripANSI(m.renderFooter())
	if !strings.Contains(footer, "PgUp") {
		t.Errorf("expected scroll hint on overflow, got %q", footer)
	}

	small := &ShellModel{width: 80, height: 24, followOutput: true}
	small.viewport = viewport.New(80, 20)
	small.ready = true
	small.messages = []Message{{role: "user", content: "ciao"}}
	small.View()
	if hint := stripANSI(small.renderFooter()); strings.Contains(hint, "PgUp") {
		t.Errorf("expected no scroll hint when content fits, got %q", hint)
	}
}
