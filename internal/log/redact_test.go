package log

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		contains    string
		notContains string
	}{
		{
			name:        "redacts account ID in ARN",
			input:       "Role: arn:aws:iam::111111111111:role/DevRole",
			contains:    "***",
			notContains: "111111111111",
		},
		{
			name:        "redacts standalone account ID",
			input:       "Account 123456789012 accessed",
			contains:    "************",
			notContains: "123456789012",
		},
		{
			name:        "redacts secrets",
			input:       "secret: abc123xyz",
			contains:    "secret:",
			notContains: "abc123xyz",
		},
		{
			name:     "preserves normal text",
			input:    "Processing role DevRole",
			contains: "Processing role DevRole",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Redact(tt.input)
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, result)
			}
			if tt.notContains != "" && strings.Contains(result, tt.notContains) {
				t.Errorf("expected result not to contain %q, got %q", tt.notContains, result)
			}
		})
	}
}
