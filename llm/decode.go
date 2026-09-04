package llm

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// decodeDataURL decodes a `data:<mime>;base64,<payload>` URL into bytes.
func decodeDataURL(url string) ([]byte, error) {
	if !strings.HasPrefix(url, "data:") {
		return nil, fmt.Errorf("invalid data URL: missing data: prefix")
	}
	i := strings.Index(url, ",")
	if i == -1 {
		return nil, fmt.Errorf("invalid data URL")
	}
	meta := url[:i]
	if !strings.Contains(meta, ";base64") {
		return nil, fmt.Errorf("invalid data URL: missing ;base64 marker")
	}
	payload := strings.TrimSpace(url[i+1:])
	// Try standard encoding first, fallback to RawStdEncoding for unpadded input.
	if data, err := base64.StdEncoding.DecodeString(payload); err == nil {
		return data, nil
	}
	return base64.RawStdEncoding.DecodeString(payload)
}
