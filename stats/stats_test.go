package stats

import (
	"path/filepath"
	"testing"
)

func TestRecordAndGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	dbPathFunc = func() (string, error) { return filepath.Join(tmpDir, "usage.db"), nil }
	defer func() { dbPathFunc = getDBPath }()

	RecordUsage("ollama", "granite4:3b-h", Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	})
	RecordUsage("ollama", "granite4:3b-h", Usage{
		PromptTokens:     200,
		CompletionTokens: 40,
		TotalTokens:      240,
		CachedTokens:     10,
		ReasoningTokens:  5,
	})
	RecordUsage("openrouter", "some/model", Usage{
		PromptTokens:     30,
		CompletionTokens: 20,
		TotalTokens:      50,
		Cost:             0.001,
	})

	entries, err := GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("GetStats() returned %d entries, want 2", len(entries))
	}

	ollama := entries[0]
	if ollama.Provider != "ollama" || ollama.Model != "granite4:3b-h" {
		t.Errorf("unexpected first entry: %+v", ollama)
	}
	if ollama.Calls != 2 {
		t.Errorf("Calls = %d, want 2", ollama.Calls)
	}
	if ollama.PromptTokens != 300 {
		t.Errorf("PromptTokens = %d, want 300", ollama.PromptTokens)
	}
	if ollama.CompletionTokens != 90 {
		t.Errorf("CompletionTokens = %d, want 90", ollama.CompletionTokens)
	}
	if ollama.TotalTokens != 390 {
		t.Errorf("TotalTokens = %d, want 390", ollama.TotalTokens)
	}
	if ollama.CachedTokens != 10 {
		t.Errorf("CachedTokens = %d, want 10", ollama.CachedTokens)
	}
	if ollama.ReasoningTokens != 5 {
		t.Errorf("ReasoningTokens = %d, want 5", ollama.ReasoningTokens)
	}

	openrouter := entries[1]
	if openrouter.Provider != "openrouter" {
		t.Errorf("expected openrouter entry second (sorted), got %+v", openrouter)
	}
	if openrouter.Cost != 0.001 {
		t.Errorf("Cost = %v, want 0.001", openrouter.Cost)
	}
}

func TestGetStatsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPathFunc = func() (string, error) { return filepath.Join(tmpDir, "usage.db"), nil }
	defer func() { dbPathFunc = getDBPath }()

	entries, err := GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("GetStats() returned %d entries, want 0", len(entries))
	}
}

func TestReset(t *testing.T) {
	tmpDir := t.TempDir()
	dbPathFunc = func() (string, error) { return filepath.Join(tmpDir, "usage.db"), nil }
	defer func() { dbPathFunc = getDBPath }()

	RecordUsage("ollama", "granite4:3b-h", Usage{PromptTokens: 10, TotalTokens: 10})
	if err := Reset(); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}

	entries, err := GetStats()
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("GetStats() after Reset returned %d entries, want 0", len(entries))
	}
}
