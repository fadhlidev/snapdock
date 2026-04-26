package audit

import (
	"regexp"

	"github.com/fadhlidev/snapdock/pkg/types"
)

// RiskLevel defines the severity of a finding.
type RiskLevel string

const (
	RiskCritical RiskLevel = "Critical"
	RiskWarning  RiskLevel = "Warning"
	RiskInfo     RiskLevel = "Info"
)

// Finding represents a security risk found in a snapshot.
type Finding struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"` // masked
	Pattern   string    `json:"pattern"`
	Risk      RiskLevel `json:"risk"`
	Description string  `json:"description"`
}

// Scanner handles the identification of sensitive data.
type Scanner struct {
	rules []rule
}

type rule struct {
	name        string
	regex       *regexp.Regexp
	risk        RiskLevel
	description string
}

func NewScanner() *Scanner {
	return &Scanner{
		rules: []rule{
			{
				name:        "Generic Password",
				regex:       regexp.MustCompile(`(?i)(password|passwd|pwd|passphrase)$`),
				risk:        RiskCritical,
				description: "Contains a password or passphrase in environment variables.",
			},
			{
				name:        "Secret Key",
				regex:       regexp.MustCompile(`(?i)(secret|private_key|api_key|token|auth_token)$`),
				risk:        RiskCritical,
				description: "Contains a secret key or access token.",
			},
			{
				name:        "Database Connection",
				regex:       regexp.MustCompile(`(?i)(db_url|database_url|connection_string|dsn)$`),
				risk:        RiskWarning,
				description: "Contains a database connection string which may include credentials.",
			},
			{
				name:        "AWS Credentials",
				regex:       regexp.MustCompile(`(?i)(aws_access_key_id|aws_secret_access_key)$`),
				risk:        RiskCritical,
				description: "Contains AWS credentials.",
			},
			{
				name:        "Certificate/Key",
				regex:       regexp.MustCompile(`(?i)(cert_path|key_path|ssh_key)$`),
				risk:        RiskWarning,
				description: "Reference to a certificate or key file.",
			},
		},
	}
}

// Scan evaluates environment variables and returns findings.
func (s *Scanner) Scan(envVars []types.EnvVar) []Finding {
	var findings []Finding

	for _, env := range envVars {
		for _, rule := range s.rules {
			if rule.regex.MatchString(env.Key) {
				findings = append(findings, Finding{
					Key:         env.Key,
					Value:       maskValue(env.Value),
					Pattern:     rule.name,
					Risk:        rule.risk,
					Description: rule.description,
				})
			}
		}
	}

	return findings
}

func maskValue(val string) string {
	if len(val) <= 4 {
		return "****"
	}
	return val[:2] + "..." + val[len(val)-2:]
}
