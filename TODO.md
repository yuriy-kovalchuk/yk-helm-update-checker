# yk-helm-update-checker — TODO

## Critical — bugs and correctness

- [x] **HTTP timeout** — replaced bare `http.Get` with a package-level `http.Client{Timeout: 30s}`; requests also respect the caller's context via `http.NewRequestWithContext`
- [x] **Multi-document YAML** — `extractor/fluxcd.go` now uses `yaml.Decoder` in both `Prepare` and `Extract` to iterate through all `---`-separated documents; each HelmRelease sets `ChartRef.Chart` to its own release name so the scanner can attribute results correctly
- [x] **Context propagation** — `Runner.Run`, `Scanner.ScanDir`, `version.Latest`, and the underlying HTTP/OCI/git calls all accept and propagate `context.Context`; cancelling the context aborts in-flight clones and version checks
- [x] **FluxCD cross-file reference resolution** — `HelmRelease` now resolves `sourceRef → HelmRepository` and `chartRef → OCIRepository` via a pre-pass (`Contextual` interface); all three patterns supported: inline `repoURL`, `sourceRef`, and `chartRef`

## High — production readiness

- [ ] **Graceful shutdown** — web server has no signal handling; SIGTERM/SIGINT should drain in-flight requests before exit
- [ ] **Unit tests** — no tests exist for any package; start with `extractor/`, `version/semver.go`, and `scan/format.go` as they have pure, easily testable logic
- [ ] **index.yaml caching** — a repo with many dependencies on the same registry fires one HTTP request per dependency; cache responses in-memory for the duration of a scan
- [ ] **Private repository support** — git clone uses anonymous HTTPS only; add SSH key and GitHub token support for private repos
- [ ] **Private OCI registry auth** — `version/oci.go` relies on `authn.DefaultKeychain`; document how to configure credentials and add an explicit token/basic-auth option in config

## Medium — quality and UX

- [ ] **golangci-lint** — add to Makefile and CI; at minimum enable `errcheck`, `staticcheck`, `gosec`
- [ ] **HTTP mock tests** — test `version/https.go` against a recorded `index.yaml` response without hitting real registries
- [ ] **OCI mock tests** — test `version/oci.go` using `go-containerregistry`'s in-process registry test helper
- [ ] **Rate limiting** — `/api/scan` can be triggered by anyone; add a simple token or IP-based rate limit
- [ ] **TLS** — web server is HTTP-only; add `--tls-cert` / `--tls-key` flags or a Let's Encrypt option
- [ ] **JSON output for CLI** — add `--output json` flag alongside the default table so results can be piped to other tools
- [ ] **Per-repo scan progress** — the web `/api/status` only exposes `scanning: true/false`; surface per-repo state so the UI can show a progress list
- [ ] **Smart git sync** — re-clone on every scan; use `git fetch --depth=1` + reset if the directory already exists to avoid redundant full clones

## Low — features and deployment

- [ ] **Scan result persistence** — results live only in memory; a server restart wipes them; consider writing to a local JSON file or SQLite between scans
- [ ] **Notification webhooks** — POST to a Slack/Discord/generic URL when new updates are detected compared to the previous scan
- [ ] **Helm chart for self-hosting** — Kubernetes manifest / Helm chart to deploy the web server into a cluster
- [ ] **Secrets in config** — document that `config.yaml` may contain tokens and should not be committed; add a `.gitignore` entry
- [ ] **Prometheus metrics** — expose `/metrics` with gauges for total charts, updates available, and last scan duration
- [ ] **Grafana dashboard** — reference JSON dashboard for update counts and scan duration
- [ ] **Image tag scanning** — extend the extractor interface to also pull container image tags from `values.yaml` and check registries for newer digests

---

## Done

- [x] Feature-folder Go project structure
- [x] `Extractor` interface — `helmchart` and `fluxcd` implementations are decoupled from the version engine
- [x] Protocol-aware version engine — `https` and `oci` dispatch in `version/engine.go`
- [x] Parallel repo cloning and parallel version checks
- [x] Scope filtering — `patch` / `minor` / `major` / `all`
- [x] Web UI — Material Design dark theme, sortable/filterable table, stat cards
- [x] `/health` endpoint for container orchestration
- [x] Structured logging via `slog`
- [x] Config file support (`config.yaml`)
- [x] Docker image
