# yk-helm-update-checker — TODO

## High — production readiness

- [ ] **Unit tests** — no tests exist for any package; start with `extractor/`, `version/engine.go`, and `scan/scanner.go` as they have pure, easily testable logic; the streaming refactor and FluxCD extractor changes are untested
- [ ] **index.yaml response caching** — a repo with many dependencies on the same registry fires one HTTP request per dependency; cache the parsed index in-memory keyed by registry URL for the duration of a scan
- [ ] **Graceful shutdown** — web server has no signal handling; SIGTERM/SIGINT should drain in-flight requests before exit
- [ ] **Private repository support** — git clone uses anonymous HTTPS only; add SSH key and GitHub token support for private repos
- [ ] **Private OCI registry auth** — `version/engine.go` relies on `authn.DefaultKeychain`; document how to configure credentials and add an explicit token/basic-auth option in config

## Medium — quality and UX

- [ ] **HTTP mock tests** — test `version/engine.go` HTTPS path against a recorded `index.yaml` response without hitting real registries
- [ ] **OCI mock tests** — test `version/engine.go` OCI path using `go-containerregistry`'s in-process registry test helper
- [x] **Stream-parse Helm index.yaml** — `version/engine.go` buffers the entire registry response via `io.ReadAll` before unmarshalling; switch to a streaming YAML decoder to avoid holding multiple large responses in memory during concurrent checks
- [ ] **Replace go-git with shell git** — `go-git/go-git` carries ~20–50 MB of in-memory overhead per clone even at depth 1; shelling out to system `git clone --depth=1` offloads that allocation outside the process heap
- [ ] **Smart git sync** — currently re-clones on every scan; use `git fetch --depth=1` + reset if the directory already exists to avoid redundant full clones
- [ ] **Per-repo scan progress** — `/api/status` only exposes `scanning: true/false`; surface per-repo state so the UI can show a progress list
- [ ] **JSON output for CLI** — add `--output json` flag alongside the default table so results can be piped to other tools
- [ ] **Rate limiting** — `/api/scan` can be triggered by anyone; add a simple token or IP-based rate limit
- [ ] **TLS** — web server is HTTP-only; add `--tls-cert` / `--tls-key` flags or a Let's Encrypt option

## Low — features and deployment

- [ ] **Scan result persistence** — results live only in memory; a server restart wipes them; consider writing to a local JSON file or SQLite between scans
- [ ] **Prometheus metrics** — expose `/metrics` with gauges for total charts, updates available, and last scan duration
- [ ] **Notification webhooks** — POST to a Slack/Discord/generic URL when new updates are detected compared to the previous scan
- [ ] **Grafana dashboard** — reference JSON dashboard for update counts and scan duration (requires Prometheus metrics)
- [ ] **Helm chart for self-hosting** — Kubernetes manifest / Helm chart to deploy the web server into a cluster
- [ ] **Image tag scanning** — extend the extractor interface to also pull container image tags from `values.yaml` and check registries for newer digests
- [ ] **Secrets in config** — document that `config.yaml` may contain tokens and should not be committed; add a `.gitignore` entry

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
- [x] Docker image with multi-arch build
- [x] HTTP timeout — package-level `http.Client{Timeout: 30s}`; requests respect caller context
- [x] Multi-document YAML — `extractor/fluxcd.go` uses `yaml.Decoder` to iterate `---`-separated documents
- [x] Context propagation — `Runner.Run`, `Scanner.ScanDir`, `version.Latest`, and underlying calls all propagate `context.Context`
- [x] FluxCD cross-file reference resolution — `HelmRelease` resolves `sourceRef → HelmRepository` and `chartRef → OCIRepository` via `Contextual` interface pre-pass
- [x] golangci-lint — added to CI
- [x] Streaming YAML file reads — eliminated `allFiles map[string][]byte`; replaced with two `WalkDir` passes so only one file's bytes are live at a time
- [x] Configurable parallel version checks — `parallel_checks` field in `config.yaml` (default 20)
