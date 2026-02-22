package helm

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"yk-update-checker/internal/models"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"gopkg.in/yaml.v3"
)

// UpdateType defines the level of updates to check for.
type UpdateType string

const (
	UpdateAll   UpdateType = "all"
	UpdateMajor UpdateType = "major"
	UpdateMinor UpdateType = "minor"
	UpdatePatch UpdateType = "patch"
)

// CheckUpdates checks for updates for all dependencies of a chart and returns results.
func CheckUpdates(chart *models.Chart, updateType UpdateType) []models.UpdateResult {
	var results []models.UpdateResult
	for i := range chart.Dependencies {
		dependency := &chart.Dependencies[i]
		if dependency.Repository == "" {
			continue
		}

		var latestVersion string
		var err error
		var repoType string

		switch {
		case strings.HasPrefix(dependency.Repository, "https://"):
			repoType = "https"
			latestVersion, err = checkHttp(dependency, updateType)
		case strings.HasPrefix(dependency.Repository, "oci://"):
			repoType = "oci"
			latestVersion, err = checkOci(dependency, updateType)
		default:
			slog.Warn("Unsupported protocol", "repository", dependency.Repository)
			continue
		}

		if err != nil {
			slog.Error("Failed to check for updates", "dependency", dependency.Name, "error", err)
			continue
		}

		if latestVersion != "" && latestVersion != dependency.Version {
			results = append(results, models.UpdateResult{
				ChartName:      chart.Name,
				Dependency:     dependency.Name,
				RepoType:       repoType,
				CurrentVersion: dependency.Version,
				LatestVersion:  latestVersion,
				UpdateType:     string(updateType),
			})
			slog.Info("New version found",
				"chart", chart.Name,
				"dependency", dependency.Name,
				"type", repoType,
				"current", dependency.Version,
				"latest", latestVersion,
				"update_type", updateType)
		}
	}
	return results
}

func checkHttp(dependency *models.Dependency, updateType UpdateType) (string, error) {
	url := strings.TrimSuffix(dependency.Repository, "/") + "/index.yaml"
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch index: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var index models.CustomIndex
	if err := yaml.Unmarshal(body, &index); err != nil {
		return "", err
	}

	versions, ok := index.Entries[dependency.Name]
	if !ok || len(versions) == 0 {
		return "", fmt.Errorf("chart %s not found in repository", dependency.Name)
	}

	var tags []string
	for _, v := range versions {
		tags = append(tags, v.Version)
	}

	return getLatestVersion(dependency.Version, tags, updateType)
}

func checkOci(dependency *models.Dependency, updateType UpdateType) (string, error) {
	baseRepo := strings.TrimPrefix(dependency.Repository, "oci://")
	repoName := baseRepo + "/" + dependency.Name

	repository, err := name.NewRepository(repoName)
	if err != nil {
		return "", err
	}

	tags, err := remote.List(repository, remote.WithAuth(authn.Anonymous))
	if err != nil {
		return "", err
	}

	return getLatestVersion(dependency.Version, tags, updateType)
}

func getLatestVersion(currentStr string, tags []string, updateType UpdateType) (string, error) {
	if len(tags) == 0 {
		return "", errors.New("no tags found")
	}

	current, err := semver.NewVersion(currentStr)
	if err != nil {
		// If current is not valid semver, we can't do scoped updates, fall back to "all"
		updateType = UpdateAll
	}

	var eligibleVersions []*semver.Version
	for _, tag := range tags {
		sv, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if sv.Prerelease() != "" {
			continue
		}

		// Skip if older or equal to current (unless current was invalid)
		if current != nil && !sv.GreaterThan(current) {
			continue
		}

		if current != nil {
			switch updateType {
			case UpdatePatch:
				if sv.Major() == current.Major() && sv.Minor() == current.Minor() {
					eligibleVersions = append(eligibleVersions, sv)
				}
			case UpdateMinor:
				if sv.Major() == current.Major() {
					eligibleVersions = append(eligibleVersions, sv)
				}
			case UpdateMajor:
				// All stable versions > current are eligible for "major" (which includes all updates)
				// or strictly only if major changed? Usually "major" means "check everything".
				eligibleVersions = append(eligibleVersions, sv)
			case UpdateAll:
				eligibleVersions = append(eligibleVersions, sv)
			}
		} else {
			eligibleVersions = append(eligibleVersions, sv)
		}
	}

	if len(eligibleVersions) == 0 {
		return "", nil
	}

	sort.Sort(semver.Collection(eligibleVersions))
	return eligibleVersions[len(eligibleVersions)-1].String(), nil
}
