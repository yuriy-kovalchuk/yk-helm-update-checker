# yk-helm-update-checker — TODO

## High — production readiness

- [ ] **Unit tests** — no tests exist for any package; start with `extractor/`, `version/engine.go`, and `scan/scanner.go`
- [ ] **Private OCI registry auth** — relies on `authn.DefaultKeychain`; add explicit token/basic-auth option in config
- [ ] **Scheduled scans** — add a `scan_interval` config option (e.g. `6h`) to re-scan automatically without external tooling
- [ ] **Startup scan** — auto-trigger the first scan on startup so the UI is not empty on first load

## Medium — quality and UX

- [ ] **HTTP mock tests** — test the HTTPS version check path against a recorded `index.yaml` without hitting real registries
- [ ] **OCI mock tests** — test the OCI version check path using `go-containerregistry`'s in-process registry test helper
- [ ] **Per-repo scan progress** — surface per-repo state in `/api/status` so the UI can show a progress list
- [ ] **Update-available filter in UI** — add a one-click toggle to show only charts with pending updates
- [ ] **JSON output for CLI** — add `--output json` flag alongside the default table so results can be piped to other tools
- [ ] **Rate limiting** — add a simple token or IP-based rate limit to `/api/scan`
- [ ] **HTTPRoute support** — add a Helm chart template for Gateway API `HTTPRoute` as an alternative to `Ingress`

## Low — features and deployment

- [ ] **Scan result persistence** — results live only in memory; write to a local JSON file or SQLite between scans
- [ ] **Prometheus metrics** — expose `/metrics` with gauges for total charts, updates available, and last scan duration
- [ ] **Notification webhooks** — POST to Slack/Discord/generic URL when new updates are detected vs the previous scan
- [ ] **Grafana dashboard** — reference JSON dashboard for update counts and scan duration (requires Prometheus metrics)
- [ ] **Image tag scanning** — extend the extractor to pull container image tags from `values.yaml` and check for newer digests
