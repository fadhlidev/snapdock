package cmd

import (
	"testing"
)

func TestMaskValue(t *testing.T) {
	tests := []struct {
		key      string
		value    string
		expected string
	}{
		{"DB_PASSWORD", "secret123", "***"},
		{"API_TOKEN", "abc-token", "***"},
		{"PORT", "8080", "8080"},
		{"USERNAME", "admin", "admin"},
		{"AWS_SECRET_ACCESS_KEY", "aws-secret", "***"},
	}

	for _, tt := range tests {
		result := maskValue(tt.key, tt.value)
		if result != tt.expected {
			t.Errorf("maskValue(%s, %s): expected %s, got %s", tt.key, tt.value, tt.expected, result)
		}
	}
}
