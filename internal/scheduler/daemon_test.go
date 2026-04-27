package scheduler

import (
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1536, "1.5 KB"},
	}

	for _, tt := range tests {
		result := formatSize(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatSize(%d): expected %s, got %s", tt.bytes, tt.expected, result)
		}
	}
}
