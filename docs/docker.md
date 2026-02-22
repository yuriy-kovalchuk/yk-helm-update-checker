# Docker Usage Guide

This guide explains how to build and run the **yk-update-checker** using Docker.

## Building the Image

Build the production-ready multi-stage image from the root of the project:

```bash
docker build -t yk-update-checker .
```

## Running Locally

### 1. Basic Web Mode
Start the web server on port 8080:

```bash
docker run -p 8080:8080 yk-update-checker
```

### 2. Monitoring a Private Repo (SSH)
If you are using a repository URL that starts with `git@github.com:...`, you must mount your local SSH credentials into the container. 

**Note:** The container runs as `appuser` (UID 10001). Ensure your local `.ssh` directory has the appropriate permissions.

```bash
docker run -p 8080:8080 
  -v ~/.ssh:/home/appuser/.ssh:ro 
  yk-update-checker 
  -repo git@github.com:your-org/your-repo.git 
  -scan-interval 1h
```

### 3. Using a Custom Port
```bash
docker run -p 9000:9000 yk-update-checker -port 9000
```

## Configuration via Docker

You can pass any of the [Command Line Flags](flags.md) as arguments to the `docker run` command:

- `-repo`: The default repository to scan.
- `-path`: The sub-directory to scan for charts.
- `-scan-interval`: How often to perform background scans (e.g., `1h`, `30m`).
- `-update-type`: The scope of updates (`all`, `major`, `minor`, `patch`).

## Troubleshooting

### SSH Known Hosts
If you encounter "Host key verification failed" errors, you may need to ensure your local `~/.ssh/known_hosts` file contains the GitHub host keys, or mount an updated `known_hosts` file into the container.

### Permissions
If the container cannot read your mounted `.ssh` keys, ensure they are readable by the user inside the container (UID 10001).
