# Local Runbook

This document covers day-to-day operations for developers working on healthd.

## Prerequisites

- Go 1.26 or newer
- Make
- Docker and Docker Compose (for container workflows)
- gh CLI (for release and PR workflows)

## Local Development

```bash
# Run unit tests with race detection and shuffling
make test

# Build binary into ./bin/healthd
make build

# Run binary locally on port 8080
make run

# Query health endpoint
curl -i http://localhost:8080/health
```

## Local Container Workflow

```bash
# Build image tagged ghcr.io/neyfua/healthd:latest
make docker

# Run container locally on port 8080
make docker-run

# Or use Compose for a reproducible stack
docker compose up --build -d
curl http://localhost:8080/health
docker compose down
```

## CI Pipeline (GitHub Actions)

Two workflows run on every push and PR:

- **ci.yml** — Go tests (race, shuffle), go vet, go mod tidy check
- **container.yml** — Build container, Trivy scan for CRITICAL/HIGH fixable vulnerabilities

On tag pushes matching `v*.*.*`, container.yml also:
- Logs in to ghcr.io
- Generates semver and git-SHA tags
- Publishes multi-tagged images to GitHub Container Registry

## Creating a Release

```bash
# Tag and push
git tag v0.1.0
git push origin v0.1.0
```

The release workflow publishes images tagged:
- `vX.Y.Z` (exact version)
- `vX.Y` (moving minor tag)
- `sha-<short-sha>` (git commit)

Pull the published image:

```bash
docker pull ghcr.io/neyfua/healthd:v0.1.0
```

## Rollback

If a published release has issues, delete the tag locally and on GitHub, then create a new tag from a known-good commit:

```bash
git tag -d v0.1.0
git push origin --delete v0.1.0
git tag v0.1.1 <good-commit>
git push origin v0.1.1
```

The failed image remains in GHCR but is no longer pointed to by the version tag.

## Troubleshooting

| Symptom | Check |
|---------|-------|
| `make test` fails with race warning | Run `go test -race -shuffle=on ./...` locally to reproduce |
| Trivy scan fails in CI | Run `trivy image ghcr.io/neyfua/healthd:scan` locally after `make docker` |
| Container won't start | `docker run --rm ghcr.io/neyfua/healthd:latest` — logs show startup or port binding issues |
| Port already in use | Change `PORT` env var or stop the conflicting process |