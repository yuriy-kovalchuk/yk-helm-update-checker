package models

type Chart struct {
	Name         string       `yaml:"name"`
	APIVersion   string       `yaml:"apiVersion"`
	Description  string       `yaml:"description"`
	Type         string       `yaml:"type"`
	Version      string       `yaml:"version"`
	AppVersion   string       `yaml:"appVersion"`
	Dependencies []Dependency `yaml:"dependencies"`
}

type Dependency struct {
	Name       string `yaml:"name"`
	Version    string `yaml:"version"`
	Repository string `yaml:"repository"`
}

type UpdateResult struct {
	ChartName     string `json:"chart_name"`
	Dependency    string `json:"dependency"`
	RepoType      string `json:"repo_type"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	UpdateType    string `json:"update_type"`
}
