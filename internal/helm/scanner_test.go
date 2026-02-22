package helm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindCharts(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-charts-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	chart1 := filepath.Join(tempDir, "chart1", "Chart.yaml")
	chart2 := filepath.Join(tempDir, "sub", "chart2", "Chart.yml")
	nonChart := filepath.Join(tempDir, "not-a-chart.txt")

	os.MkdirAll(filepath.Dir(chart1), 0755)
	os.MkdirAll(filepath.Dir(chart2), 0755)

	os.WriteFile(chart1, []byte("name: chart1"), 0644)
	os.WriteFile(chart2, []byte("name: chart2"), 0644)
	os.WriteFile(nonChart, []byte("hello"), 0644)

	charts, err := FindCharts(tempDir)
	if err != nil {
		t.Fatalf("FindCharts failed: %v", err)
	}

	if len(charts) != 2 {
		t.Errorf("Expected 2 charts, found %d", len(charts))
	}
}

func TestParseChart(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-parse-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	chartPath := filepath.Join(tempDir, "Chart.yaml")
	content := `
apiVersion: v2
name: test-chart
version: 1.0.0
dependencies:
  - name: dep1
    version: 0.1.0
    repository: https://example.com
`
	os.WriteFile(chartPath, []byte(content), 0644)

	chart, err := ParseChart(chartPath)
	if err != nil {
		t.Fatalf("ParseChart failed: %v", err)
	}

	if chart.Name != "test-chart" {
		t.Errorf("Expected name 'test-chart', got '%s'", chart.Name)
	}

	if len(chart.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(chart.Dependencies))
	}

	if chart.Dependencies[0].Name != "dep1" {
		t.Errorf("Expected dependency name 'dep1', got '%s'", chart.Dependencies[0].Name)
	}
}
