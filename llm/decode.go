package llm

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// decodeDataURL decodes a `data:<mime>;base64,<payload>` URL into bytes.
func decodeDataURL(url string) ([]byte, error) {
	i := strings.Index(url, ",")
	if i == -1 {
		return nil, fmt.Errorf("invalid data URL")
	}
	return base64.StdEncoding.DecodeString(url[i+1:])
}
