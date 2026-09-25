package llm

import (
	"fmt"
	"strings"
)

// ThinkEffort is the unified reasoning-effort level sent to all providers.
// Empty means unset: omit the field and use the provider/model default.
type ThinkEffort string

const (
	ThinkEffortNone    ThinkEffort = "none"
	ThinkEffortMinimal ThinkEffort = "minimal"
	ThinkEffortLow     ThinkEffort = "low"
	ThinkEffortMedium  ThinkEffort = "medium"
	ThinkEffortHigh    ThinkEffort = "high"
	ThinkEffortXHigh   ThinkEffort = "xhigh"
	ThinkEffortMax     ThinkEffort = "max"
)

// ParseThinkEffort normalizes and validates a think-effort value.
// Empty (after trim) means unset and returns "" with no error.
func ParseThinkEffort(s string) (ThinkEffort, error) {
	v := ThinkEffort(strings.ToLower(strings.TrimSpace(s)))
	if v == "" {
		return "", nil
	}
	switch v {
	case ThinkEffortNone, ThinkEffortMinimal, ThinkEffortLow, ThinkEffortMedium,
		ThinkEffortHigh, ThinkEffortXHigh, ThinkEffortMax:
		return v, nil
	default:
		return "", fmt.Errorf("invalid think effort %q: want one of "+
			"none, minimal, low, medium, high, xhigh, max (or empty)", s)
	}
}
