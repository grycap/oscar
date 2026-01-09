# Project Context

## Purpose
OSCAR is an open-source platform for event-driven, serverless data-processing applications.
It manages services and storage on Kubernetes clusters and integrates with serverless backends
to run containerized workloads on demand.

## Tech Stack
- Go (module `github.com/grycap/oscar/v3`, go 1.25)
- Gin HTTP framework for REST API
- Kubernetes client-go and metrics APIs
- Knative Serving for synchronous serverless execution
- MinIO/S3-compatible object storage clients
- OAuth2/OIDC + JWT auth helpers
- Docker + Kubernetes for deployment
- MkDocs (Material theme) for documentation

## Project Conventions

### Code Style
- Go code is gofmt-formatted and idiomatic
- Packages live under `pkg/` with short lowercase names (e.g., `pkg/handlers`, `pkg/backends`)
- Configuration is read from environment via `pkg/types`

### Architecture Patterns
- Main entrypoint in `main.go` wires config, Kubernetes clients, and HTTP routes
- HTTP handlers in `pkg/handlers` delegate to backends and resource managers
- Serverless backends are abstracted behind interfaces in `pkg/backends`
- Object storage operations integrate with MinIO/S3-compatible services

### Testing Strategy
- Run `go test ./...` for touched Go packages when feasible
- Use test helpers from `pkg/testsupport` where available

### Git Workflow
- Discuss changes before submitting PRs (issues/email/maintainers)
- Update README/docs when interfaces change
- Follow SemVer for version bumps in examples and README
- PRs require two reviewer sign-offs per CONTRIBUTING.md

## Domain Context
- Event-driven serverless execution for data processing
- Services define container images and scripts invoked on object storage events
- Supports async jobs (`/job/:serviceName`) and sync runs (`/run/:serviceName`)
- Storage backends include MinIO and other S3-compatible systems

## Important Constraints
- Preserve existing behavior unless explicitly requested
- Do not introduce new dependencies without approval
- Do not modify `dashboard/dist` unless the UI source is updated and rebuilt
- Do not change CI/CD workflows or licenses without explicit approval

## External Dependencies
- Kubernetes (cluster, API server, metrics)
- Knative Serving
- MinIO or S3-compatible object storage
- Optional external providers: Amazon S3, Onedata, dCache
- OAuth2/OIDC providers for auth
