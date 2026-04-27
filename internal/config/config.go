package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Build metadata — set via -ldflags at build time.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// RepoAuth holds credentials for a single repository.
// Type selects the mechanism: "token", "basic", or "ssh".
// File variants (TokenFile, PasswordFile) point to files mounted from Secrets.
type RepoAuth struct {
	Type         string `yaml:"type"`
	Token        string `yaml:"token"`
	TokenFile    string `yaml:"token_file"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	PasswordFile string `yaml:"password_file"`
	SSHKeyPath   string `yaml:"ssh_key_path"`
}

type Repo struct {
	Name string   `yaml:"name"`
	URL  string   `yaml:"repo"`
	Path string   `yaml:"path"`
	Auth RepoAuth `yaml:"auth"`
}

type Config struct {
	Repos          []Repo        `yaml:"repos"`
	UpdateType     string        `yaml:"update_type"`
	ParallelChecks int           `yaml:"parallel_checks"`
	GitCacheDir    string        `yaml:"git_cache_dir"`
	ScanInterval   time.Duration `yaml:"scan_interval"`
	StartupScan    bool          `yaml:"startup_scan"`
	StartupDelay   time.Duration `yaml:"startup_delay"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.UpdateType == "" {
		cfg.UpdateType = "all"
	}
	if cfg.ParallelChecks <= 0 {
		cfg.ParallelChecks = 5
	}
	return &cfg, nil
}
