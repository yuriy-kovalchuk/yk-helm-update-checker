# Application Architecture

The **YK-Update-Checker** is a modular Go application designed to scan Helm charts within GitHub repositories and identify available updates for their dependencies.

## Design Principles
- **Separation of Concerns**: Logic is divided into specialized packages (api, server, helm, github).
- **Concurrency**: Dependency checks are performed in parallel using Go routines and semaphores.
- **Observability**: Built-in structured logging (slog) and Prometheus metrics.
- **Portability**: All static assets (HTML, icons) are embedded into a single binary.

## Core Components

### 1. Delivery Layer (`internal/server`)
Responsible for the HTTP lifecycle.
- Configures the `http.ServeMux`.
- Embeds static assets using `go:embed`.
- Applies middleware (logging).
- Serves the `/metrics` endpoint for Prometheus.

### 2. Handler Layer (`internal/api`)
Contains the request/response logic.
- `Index`: Renders the terminal-themed UI.
- `CheckUpdates`: Coordinates the scan process and returns JSON or records metrics.
- Uses a `sync.Mutex` to prevent concurrent scans.

### 3. Logic Layer (`internal/helm`, `internal/github`)
- **github**: Handles repository cloning using the system `git` binary.
- **helm**: 
    - Scans filesystems for `Chart.yaml`.
    - Parses YAML metadata into Go structs.
    - Implements update logic for HTTPS and OCI repositories.

### 4. Metrics Layer (`internal/metrics`)
- Defines Prometheus counters and gauges.
- Tracks scan performance and update findings.

## Data Flow
1. **User Request**: User triggers a scan via UI (browser) or CLI.
2. **Cloning**: The `github` package clones the repo to a temporary directory.
3. **Scanning**: The `helm` package locates all charts and their dependencies.
4. **Validation**: The app queries remote HTTPS/OCI repositories for the latest stable SemVer.
5. **Output**: Results are returned to the UI, logged to the terminal, and updated in Prometheus gauges.
6. **Cleanup**: Temporary directories are removed after each scan.
