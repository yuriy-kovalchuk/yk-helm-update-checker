package models

import "time"

type CustomIndex struct {
	APIVersion string                          `yaml:"apiVersion"`
	Entries    map[string][]CustomChartVersion `yaml:"entries"`
}

type CustomChartVersion struct {
	Name        string    `yaml:"name"`
	Version     string    `yaml:"version"`
	AppVersion  string    `yaml:"appVersion"`
	Description string    `yaml:"description"`
	Digest      string    `yaml:"digest"`
	Type        string    `yaml:"type"`
	Home        string    `yaml:"home"`
	Icon        string    `yaml:"icon"`
	URLs        []string  `yaml:"urls"`
	Sources     []string  `yaml:"sources"`
	Created     time.Time `yaml:"created"`
}
