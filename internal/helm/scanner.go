package helm

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"yk-update-checker/internal/models"

	"gopkg.in/yaml.v3"
)

// FindCharts searches for Chart.yaml files in the given directory and its subdirectories.
func FindCharts(root string) ([]string, error) {
	var charts []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.EqualFold(d.Name(), "Chart.yaml") || strings.EqualFold(d.Name(), "Chart.yml")) {
			charts = append(charts, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}
	return charts, nil
}

// ParseChart reads and unmarshals a Chart.yaml file.
func ParseChart(filePath string) (*models.Chart, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read chart file: %w", err)
	}

	var chart models.Chart
	if err := yaml.Unmarshal(data, &chart); err != nil {
		return nil, fmt.Errorf("failed to unmarshal chart: %w", err)
	}

	return &chart, nil
}
