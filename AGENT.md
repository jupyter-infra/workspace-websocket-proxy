# workspace-websocket-proxy Developer Guide

> **Note:** `CLAUDE.md` is a symlink to this file (`AGENT.md`).
> Always edit `AGENT.md` directly — never update `CLAUDE.md`.

## Project Overview

WebSocket-to-TCP proxy sidecar for
[jupyter-k8s](https://github.com/jupyter-infra/jupyter-k8s) workspaces.
Bridges WebSocket connections from the cluster ingress to the remote access
server (`localhost:2222`) inside workspace pods.

Module path: `github.com/jupyter-infra/workspace-websocket-proxy`

No dependency on `jupyter-k8s` (the operator) or `jupyter-k8s-aws`.
This is a standalone binary that speaks standard WebSocket.

## Architecture

A single Go binary (`ws-proxy`) that:
1. Listens on port 8080 for WebSocket connections
2. Dials TCP `localhost:2222` for each connection
3. Copies bytes bidirectionally (WebSocket frames ↔ TCP)
4. Enforces session lifecycle (max duration, ping/pong, connection limits)

### Key files

- `cmd/ws-proxy/main.go` — Entry point, graceful shutdown
- `internal/proxy/server.go` — HTTP server, health endpoint, WebSocket upgrade
- `internal/proxy/bridge.go` — Bidirectional WebSocket ↔ TCP copy
- `internal/proxy/session.go` — Session lifecycle (max duration, ping/pong, concurrency)
- `internal/proxy/config.go` — Environment variable configuration
- `internal/proxy/metrics.go` — Prometheus metrics
- `internal/proxy/revalidator.go` — Interface for future periodic auth re-validation

## Build & Test

```bash
make build   # Build binary to bin/ws-proxy
make test    # Run tests with race detector
make lint    # Run golangci-lint
make docker-build  # Build container image
```

## Configuration

All via environment variables. See `internal/proxy/config.go` for defaults.

## Conventions

- Copyright header: `Amazon Web Services`
- Linter: golangci-lint v2 (see `.golangci.yml`)
- Container tool: `finch` (default in Makefile)
- Base image: `gcr.io/distroless/static:nonroot`
- User: 65532 (non-root)
