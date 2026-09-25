package llm

import "testing"

func TestParseThinkEffort(t *testing.T) {
	cases := []struct {
		in      string
		want    ThinkEffort
		wantErr bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"none", ThinkEffortNone, false},
		{"minimal", ThinkEffortMinimal, false},
		{"low", ThinkEffortLow, false},
		{"medium", ThinkEffortMedium, false},
		{"high", ThinkEffortHigh, false},
		{"xhigh", ThinkEffortXHigh, false},
		{"max", ThinkEffortMax, false},
		{" Low ", ThinkEffortLow, false},
		{"HIGH", ThinkEffortHigh, false},
		{"ultra", "", true},
		{"off", "", true},
	}
	for _, tt := range cases {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseThinkEffort(tt.in)
			if tt.wantErr && err == nil {
				t.Fatalf("ParseThinkEffort(%q) = %q, want error", tt.in, got)
			}
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("ParseThinkEffort(%q) error: %v", tt.in, err)
				}
				if got != tt.want {
					t.Errorf("ParseThinkEffort(%q) = %q, want %q", tt.in, got, tt.want)
				}
			}
		})
	}
}
