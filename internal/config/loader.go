package config

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/fadhlidev/snapdock/pkg/types"
)

// Load reads and parses a snapdock.yaml file.
func Load(path string) (*types.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg types.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save writes a Config to a YAML file.
func Save(path string, cfg *types.Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
