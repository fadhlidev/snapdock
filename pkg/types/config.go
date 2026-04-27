package types

// Config represents the top-level snapdock.yaml structure.
type Config struct {
	Jobs []JobConfig `yaml:"jobs"`
}

// JobConfig defines a single scheduled snapshot job.
type JobConfig struct {
	Name      string          `yaml:"name"`
	Container string          `yaml:"container"`
	Schedule  string          `yaml:"schedule"`
	Output    string          `yaml:"output,omitempty"`
	Options   JobOptions      `yaml:"options,omitempty"`
	Retention RetentionConfig `yaml:"retention,omitempty"`
}

// JobOptions mirrors SnapOptions but for the config file.
type JobOptions struct {
	WithVolumes bool `yaml:"with_volumes"`
	Encrypt     bool `yaml:"encrypt"`
}

// RetentionConfig defines how many snapshots to keep.
type RetentionConfig struct {
	KeepLast    int    `yaml:"keep_last,omitempty"`
	DeleteAfter string `yaml:"delete_after,omitempty"` // e.g. "30d"
}
