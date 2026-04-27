package snapshot

import (
	"encoding/json"
	"testing"

	"github.com/fadhlidev/snapdock/internal/docker"
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

func TestExportableSnapshot(t *testing.T) {
	snap := &docker.ContainerSnapshot{
		ID:   "test-id",
		Name: "test-name",
		Env:  []string{"K=V"},
	}

	result := exportableSnapshot(snap)
	if result == nil {
		t.Fatal("exportableSnapshot returned nil")
	}

	// We can't easily check fields of anonymous struct without reflection or JSON marshaling
	// But we can check that it's not nil and maybe do a quick check if it can be marshaled
	_, err := json.Marshal(result)
	if err != nil {
		t.Errorf("failed to marshal exportableSnapshot result: %v", err)
	}
}

