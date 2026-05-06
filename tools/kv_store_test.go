package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKVStore(t *testing.T) {
	// Setup temporary DB
	tmpDir, err := os.MkdirTemp("", "ai-shell-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPathOverride = filepath.Join(tmpDir, "test_kv.db")
	defer func() { dbPathOverride = "" }()

	// Test KVSet
	msg, err := KVSet("testKey", "testValue")
	if err != nil {
		t.Errorf("KVSet failed: %v", err)
	}
	if !strings.Contains(msg, "Successfully saved") {
		t.Errorf("Unexpected KVSet message: %s", msg)
	}

	// Test KVGet
	val, err := KVGet("testKey")
	if err != nil {
		t.Errorf("KVGet failed: %v", err)
	}
	if val != "testValue" {
		t.Errorf("Expected 'testValue', got '%s'", val)
	}

	// Test KVGet non-existent
	_, err = KVGet("nonExistent")
	if err == nil {
		t.Error("Expected error for non-existent key, got nil")
	}

	// Test KVList
	_, _ = KVSet("anotherKey", "anotherValue")
	list, err := KVList()
	if err != nil {
		t.Errorf("KVList failed: %v", err)
	}
	if !strings.Contains(list, "testKey") || !strings.Contains(list, "anotherKey") {
		t.Errorf("KVList output missing keys: %s", list)
	}

	// Test empty list (new DB)
	dbPathOverride = filepath.Join(tmpDir, "empty_kv.db")
	list, err = KVList()
	if err != nil {
		t.Errorf("KVList failed on empty DB: %v", err)
	}
	if list != "No keys found in KV store" {
		t.Errorf("Expected 'No keys found in KV store', got '%s'", list)
	}
}
