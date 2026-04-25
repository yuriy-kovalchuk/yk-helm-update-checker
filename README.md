# yk-update-checker

Scans one or more GitOps repositories for outdated Helm chart dependencies and FluxCD HelmRelease resources. Runs as a one-shot CLI tool or as a persistent web server with a dark-themed UI.

## What it does

- Clones configured Git repositories (shallow, depth 1)
- Walks every YAML file looking for Helm `Chart.yaml` dependencies and FluxCD `HelmRelease` manifests
- Resolves cross-file FluxCD references (`HelmRepository`, `OCIRepository`)
- Queries the upstream Helm index or OCI registry for the latest stable version
- Filters results by upgrade scope: `patch`, `minor`, `major`, or `all`
- Outputs a formatted table (CLI) or serves a sortable/filterable web UI (web mode)

## Installation

**Binary**

```bash
make build
./bin/yk-update-checker -config config.yaml
```

**Docker**

```bash
docker run --rm \
  -v $(pwd)/config.yaml:/etc/yk-update-checker/config.yaml \
  ghcr.io/yuriy-kovalchuk/yk-helm-update-checker:latest \
  -config /etc/yk-update-checker/config.yaml
```

```bash
docker run --rm \
  -v $(pwd)/config.yaml:/etc/yk-update-checker/config.yaml -p 8080:8080 \
  ghcr.io/yuriy-kovalchuk/yk-helm-update-checker:latest \
  -config /etc/yk-update-checker/config.yaml -web
```

**Helm**

```bash
helm install yk-update-checker \
  oci://ghcr.io/yuriy-kovalchuk/charts/yk-update-checker \
  --set config.repos[0].name=my-gitops \
  --set config.repos[0].repo=https://github.com/example/gitops
```

## Configuration

Copy `config.example.yaml` and edit it:

```yaml
update_type: all   # all | major | minor | patch

repos:
  - name: homelab
    repo: https://github.com/example/my-gitops-repo
    path: kubernetes/apps   # optional sub-path to scan
```

## Usage

```
yk-update-checker [flags]

  -config  string   path to config file (default "config.yaml")
  -scope   string   override update_type from config
  -web              start web server instead of CLI scan
  -port    string   web server port (default "8080")
  -verbose          enable debug logging
```

**CLI scan**

```bash
./bin/yk-update-checker -config config.yaml -scope minor
```

**Web server**

```bash
./bin/yk-update-checker -web -config config.yaml
# open http://localhost:8080
```

## Supported sources

| Source | Pattern |
|---|---|
| Helm `Chart.yaml` | `dependencies[].repository` (https and oci) |
| FluxCD `HelmRelease` | inline `repoURL`, `sourceRef → HelmRepository`, `chartRef → OCIRepository` |

Multi-document YAML files (`---` separated) are fully supported.

## Development

```bash
make build    # compile binary to bin/
make test     # run tests with race detector
make clean    # remove build artifacts
```

CI runs on every push and pull request to `master`. Releases are triggered by pushing a `v*` tag, which builds and pushes the Docker image and Helm chart to GHCR.

## License

MIT
