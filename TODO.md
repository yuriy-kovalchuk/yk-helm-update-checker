# yk-helm-update-checker — TODO

## High — production readiness

- [ ] **Unit tests** — no tests exist; start with `internal/extractor/`, `internal/version/`, and `internal/scan/`
- [ ] **Private OCI registry auth** — version resolution falls back to `authn.DefaultKeychain`; add explicit token/basic-auth config for private OCI chart registries (same auth model as repos)

## Medium — quality and UX

- [ ] **Per-repo scan progress** — surface per-repo state in `/api/status` so the UI can show which repos are still being cloned/scanned
- [ ] **Native chart version checking** — only `dependencies[]` in `Chart.yaml` are checked; add support for checking the chart's own version against its upstream source repo
- [ ] **HTTP mock tests** — test the HTTPS version-check path against a recorded `index.yaml` without hitting real registries
- [ ] **OCI mock tests** — test the OCI version-check path using `go-containerregistry`'s in-process registry test helper

## Low — features and deployment

- [ ] **Prometheus metrics** — expose `/metrics` with gauges for total charts, updates available, and last scan duration
- [ ] **Notification webhooks** — POST to Slack/Discord/generic URL when updates are detected that weren't present in the previous scan
- [ ] **Grafana dashboard** — reference JSON for update counts and scan duration (requires Prometheus metrics)
- [ ] **HTTPRoute support** — add a Helm template for Gateway API `HTTPRoute` as an alternative to `Ingress`
- [ ] **Image tag scanning** — extend extractors to find container image tags in `values.yaml` and check for newer versions/digests
