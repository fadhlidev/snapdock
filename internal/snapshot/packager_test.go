package snapshot

import (
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my/container", "my-container"},
		{"my:tag", "my-tag"},
		{"my container", "my_container"},
		{"clean-name", "clean-name"},
		{"complex/name:with spaces", "complex-name-with_spaces"},
	}

	for _, tt := range tests {
		result := sanitizeName(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeName(%s): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}
