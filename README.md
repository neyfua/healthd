# healthd

[![CI](https://github.com/neyfua/healthd/actions/workflows/ci.yml/badge.svg)](https://github.com/neyfua/healthd/actions/workflows/ci.yml)
[![Container Security & Release](https://github.com/neyfua/healthd/actions/workflows/container.yml/badge.svg)](https://github.com/neyfua/healthd/actions/workflows/container.yml)

A minimal Go HTTP service that demonstrates an automated container delivery pipeline: code testing, multi-stage compilation, security scanning, and container registry publishing.

Originally built as a quick DevOps portfolio piece, it's now evolving into a small synthetic healthcare service to showcase more realistic operational concerns — readiness probes, structured logging, metrics, and local observability stack.

## Project Scope

healthd simulates a tiny clinic backend with in-memory appointment data. It is intentionally small: no database, no authentication, no real patient data. All data is synthetic and generated at startup.

Goals:

- Show a complete Go + Docker + GitHub Actions pipeline
- Exercise operational patterns (probes, logging, metrics) without cloud dependencies
- Stay runnable locally with `make` and `docker compose`

Non-goals:

- Production healthcare compliance (HIPAA, etc.)
- Real user management or data persistence
- Kubernetes, cloud providers, or service mesh

**AI assistance note**: AI helped with some scaffolding and implementation suggestions. I review and test changes before keeping them, and document the decisions and trade-offs as the project grows.

## Architecture

```
Git Push / PR
      │
      ▼
┌─────────────────────┐
│  GitHub Actions CI  │
│  - Go test (race)   │
│  - go vet           │
│  - go mod tidy      │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  Container Build    │
│  - Multi-stage      │
│  - Distroless base  │
│  - Nonroot user     │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│  Trivy Scan         │
│  - CRITICAL/HIGH    │
│  - Ignores unfixed  │
└─────────┬───────────┘
          │
    ┌─────┴─────┐
    ▼           ▼
 PR checks   Tag push → GHCR
```

## Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Liveness probe — returns `{"status":"ok"}` |
| GET | `/` | Service metadata — name, version, build info |

Additional endpoints will appear as features land (readiness, appointments, metrics).

## Local Development

See [Local Runbook](docs/runbook.md) for the full workflow.

Quick start:

```bash
# Run tests
make test

# Build and run locally
make run

# Check health
curl http://localhost:8080/health
```

With Docker Compose:

```bash
docker compose up --build -d
curl http://localhost:8080/health
docker compose down
```

## Continuous Integration & Delivery

1. **Pull Requests** (`ci.yml` and `container.yml`):
   - Unit tests with race detection and shuffling
   - `go vet` and `go mod tidy` verification
   - Container build and Trivy vulnerability scan (CRITICAL/HIGH, fixable only)

2. **Releases** (`container.yml` on `v*.*.*` tags):
   - Semantic version and git-SHA image tags
   - Publish to `ghcr.io/neyfua/healthd`
   - Images published with semantic-version and commit-SHA tags

## Creating a Release

```bash
git tag v0.1.0
git push origin v0.1.0
```

Images are then available at:

```bash
docker pull ghcr.io/neyfua/healthd:v0.1.0
```

## Limitations & Known Gaps

- Single binary, no horizontal scaling story yet
- In-memory only — data lost on restart
- No authentication, authorization, or audit logging
- Metrics and readiness probes not yet implemented (planned)
- Distroless base limits debugging tools in production image

## License

MIT — see LICENSE file.