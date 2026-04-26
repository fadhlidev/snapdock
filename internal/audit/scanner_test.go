package audit

import (
	"testing"

	"github.com/fadhlidev/snapdock/pkg/types"
)

func TestScanner(t *testing.T) {
	scanner := NewScanner()

	envVars := []types.EnvVar{
		{Key: "DB_PASSWORD", Value: "secret123"},
		{Key: "API_TOKEN", Value: "abc123def456"},
		{Key: "PORT", Value: "8080"},
		{Key: "AWS_ACCESS_KEY_ID", Value: "AKIA1234567890"},
	}

	findings := scanner.Scan(envVars)

	if len(findings) != 3 {
		t.Errorf("expected 3 findings, got %d", len(findings))
	}

	for _, f := range findings {
		t.Logf("Found: %s (%s) - %s", f.Key, f.Risk, f.Pattern)
		if f.Key == "PORT" {
			t.Errorf("PORT should not be flagged as sensitive")
		}
	}
}
