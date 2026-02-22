# Project Roadmap 🚀

This document outlines the planned features and future direction for **yk-update-checker**.

## 🟢 Phase 1: Observability & Deployment (Short-term)
- [ ] **Grafana Dashboard**: Create a reference JSON dashboard for visualizing Helm updates and scan performance.
- [ ] **Docker Hub Image**: Automate the publishing of the Docker image.
- [ ] **Helm Chart**: Create a Helm chart to deploy this app easily into any Kubernetes cluster.
- [ ] **Graceful Shutdown**: Implement better handling of OS signals (SIGINT/SIGTERM) for cleaner server exits.

## 🟡 Phase 2: Enhanced Scanning (Mid-term)
- [ ] **Image Tag Scanning**: Expand the scanner to look at `values.yaml` and check for newer container image tags.
- [ ] **Remote Index Caching**: Implement an internal cache for `index.yaml` files to improve speed and reduce rate-limiting risks.
- [ ] **Smart Git Sync**: Optimize the downloader to perform `git pull` instead of a full re-clone on existing directories.

## 🟠 Phase 3: Notifications & History (Long-term)
- [ ] **Notification Webhooks**: Support sending alerts to Slack, Discord, or generic Webhooks when new updates are found.
- [ ] **SQLite Persistence**: Store scan history in a local database to track when updates were first detected.
- [ ] **Summary API**: An endpoint that returns a high-level summary (e.g., "3 critical updates, 5 patches available").

---
*Note: This roadmap is subject to change based on project needs.*
