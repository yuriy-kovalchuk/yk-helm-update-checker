package extractor

import (
	"bytes"
	"sync"

	"gopkg.in/yaml.v3"
)

// objectMeta mirrors the Kubernetes ObjectMeta fields we care about.
type objectMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// crossNamespaceRef is the sourceRef / chartRef block inside a HelmRelease.
type crossNamespaceRef struct {
	Kind      string `yaml:"kind"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// fluxSourceSpec is the spec block of a HelmRepository or OCIRepository.
type fluxSourceSpec struct {
	URL string     `yaml:"url"`
	Ref fluxOCIRef `yaml:"ref"`
}

// fluxOCIRef is the spec.ref block of an OCIRepository.
type fluxOCIRef struct {
	Tag string `yaml:"tag"`
}

// fluxSourceResource is used during Prepare to parse HelmRepository and
// OCIRepository manifests.
type fluxSourceResource struct {
	Kind     string         `yaml:"kind"`
	Metadata objectMeta     `yaml:"metadata"`
	Spec     fluxSourceSpec `yaml:"spec"`
}

// helmChartSpec is the spec.chart.spec block of a HelmRelease.
type helmChartSpec struct {
	Chart     string             `yaml:"chart"`
	Version   string             `yaml:"version"`
	RepoURL   string             `yaml:"repoURL"`
	SourceRef *crossNamespaceRef `yaml:"sourceRef"`
}

// helmChartWrapper is the spec.chart block of a HelmRelease.
type helmChartWrapper struct {
	Spec helmChartSpec `yaml:"spec"`
}

// helmReleaseSpec is the spec block of a HelmRelease.
type helmReleaseSpec struct {
	ChartRef *crossNamespaceRef `yaml:"chartRef"`
	Chart    helmChartWrapper   `yaml:"chart"`
}

// helmReleaseResource is used during Extract to parse a HelmRelease manifest.
type helmReleaseResource struct {
	Metadata objectMeta      `yaml:"metadata"`
	Spec     helmReleaseSpec `yaml:"spec"`
}

// repoEntry is a resolved registry record built from a HelmRepository or
// OCIRepository resource during the Prepare phase.
type repoEntry struct {
	protocol  string
	repoURL   string // bare URL, scheme stripped
	chartName string // derived from OCI URL last segment; empty for HTTPS
	version   string // pinned version from OCIRepository spec.ref.tag
}

// FluxCD extracts chart refs from FluxCD HelmRelease manifests.
//
// It implements Contextual: the scanner calls Prepare with every YAML file
// in the scan root before any Extract calls. Prepare collects all
// HelmRepository and OCIRepository resources so that HelmRelease
// sourceRef/chartRef cross-file lookups can succeed.
type FluxCD struct {
	mu   sync.RWMutex
	helm map[string]repoEntry // keyed by "namespace/name"
	oci  map[string]repoEntry // keyed by "namespace/name"
}

func NewFluxCD() *FluxCD {
	return &FluxCD{
		helm: make(map[string]repoEntry),
		oci:  make(map[string]repoEntry),
	}
}

func (*FluxCD) Type() string { return "fluxcd" }

func (*FluxCD) Match(_ string, content []byte) bool {
	return bytes.Contains(content, []byte("HelmRelease"))
}

// PrepareFile processes a single YAML file, collecting any HelmRepository and
// OCIRepository resources it contains so that HelmRelease sourceRef/chartRef
// lookups work across files. It is called once per file during the pre-pass.
func (f *FluxCD) PrepareFile(_ string, content []byte) error {
	if !bytes.Contains(content, []byte("HelmRepository")) &&
		!bytes.Contains(content, []byte("OCIRepository")) {
		return nil
	}

	dec := yaml.NewDecoder(bytes.NewReader(content))

	f.mu.Lock()
	defer f.mu.Unlock()

	for {
		var doc fluxSourceResource
		if err := dec.Decode(&doc); err != nil {
			break
		}

		key := doc.Metadata.Namespace + "/" + doc.Metadata.Name

		switch doc.Kind {
		case "HelmRepository":
			protocol, bare := ParseProtocol(doc.Spec.URL)
			f.helm[key] = repoEntry{protocol: protocol, repoURL: bare}

		case "OCIRepository":
			protocol, bare := ParseProtocol(doc.Spec.URL)
			// OCI URL encodes the chart name as its last path segment:
			//   ghcr.io/org/charts/chartname → repo=ghcr.io/org/charts  chart=chartname
			repo, chart := SplitOCIRef(bare)
			f.oci[key] = repoEntry{
				protocol:  protocol,
				repoURL:   repo,
				chartName: chart,
				version:   doc.Spec.Ref.Tag,
			}
		}
	}

	return nil
}

// Extract handles multi-document YAML files. Each document that contains a
// HelmRelease is processed independently via three patterns:
//  1. Inline repoURL  — spec.chart.spec.repoURL (older/simpler pattern)
//  2. sourceRef       — spec.chart.spec.sourceRef → HelmRepository
//  3. chartRef        — spec.chartRef → OCIRepository
//
// ChartRef.Chart is set to the HelmRelease name so the scanner can attribute
// each ref to the correct release even when a file contains multiple releases.
func (f *FluxCD) Extract(_ string, content []byte) (string, []ChartRef, error) {
	dec := yaml.NewDecoder(bytes.NewReader(content))
	var all []ChartRef
	for {
		var hr helmReleaseResource
		if err := dec.Decode(&hr); err != nil {
			break // io.EOF or malformed document — stop iterating
		}
		if hr.Metadata.Name == "" {
			continue // not a HelmRelease (or an empty document)
		}
		all = append(all, f.refsFromRelease(hr)...)
	}
	return "", all, nil
}

// refsFromRelease extracts ChartRefs from a single HelmRelease document.
func (f *FluxCD) refsFromRelease(hr helmReleaseResource) []ChartRef {
	releaseName := hr.Metadata.Name
	releaseNS := hr.Metadata.Namespace

	// ── Pattern 3: chartRef → OCIRepository ──────────────────────────────
	if cr := hr.Spec.ChartRef; cr != nil && cr.Kind == "OCIRepository" {
		ns := cr.Namespace
		if ns == "" {
			ns = releaseNS
		}
		key := ns + "/" + cr.Name

		f.mu.RLock()
		entry, ok := f.oci[key]
		f.mu.RUnlock()

		if !ok || entry.chartName == "" || entry.version == "" {
			return nil
		}
		return []ChartRef{{
			Name:           entry.chartName,
			Chart:          releaseName,
			Protocol:       entry.protocol,
			Repository:     entry.repoURL,
			CurrentVersion: entry.version,
		}}
	}

	cs := hr.Spec.Chart.Spec

	// ── Pattern 2: sourceRef → HelmRepository ────────────────────────────
	if sr := cs.SourceRef; sr != nil && sr.Kind == "HelmRepository" {
		ns := sr.Namespace
		if ns == "" {
			ns = releaseNS
		}
		key := ns + "/" + sr.Name

		f.mu.RLock()
		entry, ok := f.helm[key]
		f.mu.RUnlock()

		if !ok || cs.Chart == "" || cs.Version == "" {
			return nil
		}
		return []ChartRef{{
			Name:           cs.Chart,
			Chart:          releaseName,
			Protocol:       entry.protocol,
			Repository:     entry.repoURL,
			CurrentVersion: cs.Version,
		}}
	}

	// ── Pattern 1: inline repoURL ─────────────────────────────────────────
	if cs.RepoURL == "" || cs.Chart == "" || cs.Version == "" {
		return nil
	}
	protocol, repo := ParseProtocol(cs.RepoURL)
	return []ChartRef{{
		Name:           cs.Chart,
		Chart:          releaseName,
		Protocol:       protocol,
		Repository:     repo,
		CurrentVersion: cs.Version,
	}}
}
