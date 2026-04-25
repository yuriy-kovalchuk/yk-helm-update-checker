package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Build metadata — set via -ldflags at build time.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

type Repo struct {
	Name string `yaml:"name"`
	URL  string `yaml:"repo"`
	Path string `yaml:"path"`
}

type Config struct {
	Repos          []Repo `yaml:"repos"`
	UpdateType     string `yaml:"update_type"`
	ParallelChecks int    `yaml:"parallel_checks"`
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
		cfg.ParallelChecks = 20
	}
	return &cfg, nil
}
