package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

func loadSchema(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema: %w", err)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}
	return raw, nil
}

func structuredResponseFormat(schemaRaw any) map[string]any {
	return map[string]any{
		"type": "json_schema",
		"json_schema": map[string]any{
			"name":   "extracted_data",
			"strict": true,
			"schema": schemaRaw,
		},
	}
}

func prettyJSONOrRaw(s string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err == nil {
		return buf.String()
	}
	return s
}
