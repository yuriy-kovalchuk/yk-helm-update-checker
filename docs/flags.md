# Command Line Flags

The **yk-update-checker** can be configured using the following flags. These flags control the execution mode (CLI vs. Web), repository targets, and background scanning behavior.

## Core Flags

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-repo` | `string` | `""` | The GitHub repository URL to scan. Supports HTTPS and SSH (`git@...`) formats. |
| `-web` | `bool` | `false` | If set, the application starts as a Web Server with a dashboard and Prometheus metrics. |
| `-verbose` | `bool` | `false` | Enables Debug level logging for troubleshooting. |

## Web Mode Flags
*These flags are primarily used when `-web` is enabled.*

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-port` | `string` | `"8080"` | The port on which the web server will listen. |
| `-scan-interval` | `duration` | `0` | The frequency for background scans (e.g., `1h`, `30m`, `15s`). Requires `-repo` to be set. Set to `0` to disable background scanning. |

## Scanning & Logic Flags
*These flags affect how Helm charts are discovered and how updates are calculated.*

| Flag | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `-path` | `string` | `"."` | The sub-directory within the repository to scan for `Chart.yaml` files. |
| `-update-type` | `string` | `"all"` | The scope of the update check. Options: `all`, `major`, `minor`, `patch`. |
| `-temp-dir` | `string` | `""` | Overrides the default system temporary directory used for cloning repositories. |

---

## Usage Examples

### 1. Simple CLI Scan
Check a repository once and output results to the terminal:
```bash
./yk-update-checker -repo https://github.com/helm/examples -path charts
```

### 2. Web Dashboard with Default Repo
Start the server and hide the repo input in the UI:
```bash
./yk-update-checker -web -repo https://github.com/my-org/infra-charts
```

### 3. Continuous Monitoring (Best for Docker/Helm)
Run as a background service that updates Prometheus metrics every 30 minutes:
```bash
./yk-update-checker -web -repo git@github.com:my-org/charts.git -scan-interval 30m
```

### 4. Patch-Only Updates
Only look for safe, non-breaking patch updates:
```bash
./yk-update-checker -repo https://github.com/my-org/charts -update-type patch
```
